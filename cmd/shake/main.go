package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/mibar/tree-shaker/pkg/shaker"
)

func ptr[T any](v T) *T { return &v }

func main() {
	mode := flag.String("mode", "include", `"include" or "exclude"`)
	paths := flag.String("paths", "", "comma-separated JSONPath expressions")
	file := flag.String("file", "", "path to input JSON file (default: read from stdin)")
	output := flag.String("output", "", "path to output JSON file (default: write to stdout)")
	maxDepth := flag.Int("max-depth", 0, fmt.Sprintf("maximum JSON nesting depth (default: %d, -1 = no limit)", shaker.MaxDepth))
	maxPathLength := flag.Int("max-path-length", 0, fmt.Sprintf("maximum byte length per JSONPath expression (default: %d, -1 = no limit)", shaker.MaxPathLength))
	maxPathCount := flag.Int("max-path-count", 0, fmt.Sprintf("maximum number of JSONPath expressions (default: %d, -1 = no limit)", shaker.MaxPathCount))
	pretty := flag.Bool("pretty", false, "pretty-print the JSON output")
	flag.Parse()

	if *paths == "" {
		fmt.Fprintln(os.Stderr, "usage: shake -mode include -paths '$.name,$.email' [-file input.json | < input.json] [-output result.json]")
		os.Exit(1)
	}

	var r io.Reader = os.Stdin
	if *file != "" {
		f, err := os.Open(*file)
		if err != nil {
			fmt.Fprintf(os.Stderr, "open file: %v\n", err)
			os.Exit(1)
		}
		defer f.Close()
		r = f
	}

	input, err := io.ReadAll(r)
	if err != nil {
		fmt.Fprintf(os.Stderr, "read input: %v\n", err)
		os.Exit(1)
	}

	pathList := strings.Split(*paths, ",")

	var q shaker.Query
	switch *mode {
	case "include":
		q = shaker.Include(pathList...)
	case "exclude":
		q = shaker.Exclude(pathList...)
	default:
		fmt.Fprintf(os.Stderr, "invalid mode: %q (expected \"include\" or \"exclude\")\n", *mode)
		os.Exit(1)
	}

	// Limits: 0 = use default, >0 = custom, <0 = no limit (Ptr(0)).
	var limits shaker.Limits
	if *maxDepth > 0 {
		limits.MaxDepth = maxDepth
	} else if *maxDepth < 0 {
		limits.MaxDepth = ptr(0)
	}
	if *maxPathLength > 0 {
		limits.MaxPathLength = maxPathLength
	} else if *maxPathLength < 0 {
		limits.MaxPathLength = ptr(0)
	}
	if *maxPathCount > 0 {
		limits.MaxPathCount = maxPathCount
	} else if *maxPathCount < 0 {
		limits.MaxPathCount = ptr(0)
	}
	q = q.WithLimits(limits)

	out, err := shaker.Shake(input, q)
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
