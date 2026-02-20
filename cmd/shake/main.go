package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"

	"github.com/mibar/tree-shaker/pkg/shaker"
)

const usage = `usage: shake <include|exclude> [flags] <path> [path...]

Input sources (first match wins):
  -file <path>    Read from file
  -input <json>   Read from argument
  (default)       Read from stdin

Examples:
  shake include -input '{"a":1,"b":2}' '$.a'
  shake include -file data.json -pretty '$.name' '$.email'
  curl -s url | shake include '$.data[*].id'`

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, usage)
		os.Exit(1)
	}

	mode := os.Args[1]
	if mode != "include" && mode != "exclude" {
		fmt.Fprintf(os.Stderr, "invalid mode: %q (expected \"include\" or \"exclude\")\n\n%s\n", mode, usage)
		os.Exit(1)
	}

	fs := flag.NewFlagSet("shake", flag.ExitOnError)
	file := fs.String("file", "", "path to input JSON file")
	input := fs.String("input", "", "inline JSON string")
	output := fs.String("output", "", "path to output JSON file (default: stdout)")
	maxDepth := fs.Int("max-depth", 0, fmt.Sprintf("maximum JSON nesting depth (default: %d, -1 = no limit)", shaker.MaxDepth))
	maxPathLength := fs.Int("max-path-length", 0, fmt.Sprintf("maximum byte length per JSONPath expression (default: %d, -1 = no limit)", shaker.MaxPathLength))
	maxPathCount := fs.Int("max-path-count", 0, fmt.Sprintf("maximum number of JSONPath expressions (default: %d, -1 = no limit)", shaker.MaxPathCount))
	pretty := fs.Bool("pretty", false, "pretty-print the JSON output")
	fs.Usage = func() { fmt.Fprintln(os.Stderr, usage) }
	fs.Parse(os.Args[2:])

	paths := fs.Args()
	if len(paths) == 0 {
		fmt.Fprintln(os.Stderr, "error: at least one JSONPath expression is required")
		fmt.Fprintln(os.Stderr, usage)
		os.Exit(1)
	}

	var data []byte
	switch {
	case *file != "":
		f, err := os.Open(*file)
		if err != nil {
			fmt.Fprintf(os.Stderr, "open file: %v\n", err)
			os.Exit(1)
		}
		defer f.Close()
		data, err = io.ReadAll(f)
		if err != nil {
			fmt.Fprintf(os.Stderr, "read file: %v\n", err)
			os.Exit(1)
		}
	case *input != "":
		data = []byte(*input)
	default:
		var err error
		data, err = io.ReadAll(os.Stdin)
		if err != nil {
			fmt.Fprintf(os.Stderr, "read stdin: %v\n", err)
			os.Exit(1)
		}
	}

	var q shaker.Query
	switch mode {
	case "include":
		q = shaker.Include(paths...)
	case "exclude":
		q = shaker.Exclude(paths...)
	}

	var limits shaker.Limits
	if *maxDepth > 0 {
		limits.MaxDepth = maxDepth
	} else if *maxDepth < 0 {
		limits.MaxDepth = new(int)
	}
	if *maxPathLength > 0 {
		limits.MaxPathLength = maxPathLength
	} else if *maxPathLength < 0 {
		limits.MaxPathLength = new(int)
	}
	if *maxPathCount > 0 {
		limits.MaxPathCount = maxPathCount
	} else if *maxPathCount < 0 {
		limits.MaxPathCount = new(int)
	}
	q = q.WithLimits(limits)

	out, err := shaker.Shake(data, q)
	if err != nil {
		fmt.Fprintf(os.Stderr, "shake: %v\n", err)
		os.Exit(1)
	}

	if *pretty {
		var buf bytes.Buffer
		if err := json.Indent(&buf, out, "", "  "); err != nil {
			fmt.Fprintf(os.Stderr, "pretty-print: %v\n", err)
			os.Exit(1)
		}
		out = buf.Bytes()
	}

	if *output != "" {
		if err := os.WriteFile(*output, append(out, '\n'), 0o644); err != nil {
			fmt.Fprintf(os.Stderr, "write output: %v\n", err)
			os.Exit(1)
		}
	} else {
		fmt.Println(string(out))
	}
}
