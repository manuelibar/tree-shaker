package main

import (
	"flag"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/mibar/tree-shaker/pkg/shaker"
)

func main() {
	mode := flag.String("mode", "include", `"include" or "exclude"`)
	paths := flag.String("paths", "", "comma-separated JSONPath expressions")
	maxDepth := flag.Int("max-depth", 0, fmt.Sprintf("maximum JSON nesting depth (0 = no limit; recommended: %d)", shaker.MaxDepth))
	maxPathLength := flag.Int("max-path-length", 0, fmt.Sprintf("maximum byte length per JSONPath expression (0 = no limit; recommended: %d)", shaker.MaxPathLength))
	maxPathCount := flag.Int("max-path-count", 0, fmt.Sprintf("maximum number of JSONPath expressions (0 = no limit; recommended: %d)", shaker.MaxPathCount))
	flag.Parse()

	if *paths == "" {
		fmt.Fprintln(os.Stderr, "usage: shake -mode include -paths '$.name,$.email' < input.json")
		os.Exit(1)
	}

	input, err := io.ReadAll(os.Stdin)
	if err != nil {
		fmt.Fprintf(os.Stderr, "read stdin: %v\n", err)
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

	var limits shaker.Limits
	if *maxDepth != 0 {
		limits.MaxDepth = maxDepth
	}
	if *maxPathLength != 0 {
		limits.MaxPathLength = maxPathLength
	}
	if *maxPathCount != 0 {
		limits.MaxPathCount = maxPathCount
	}

	if limits.MaxDepth == nil && limits.MaxPathLength == nil && limits.MaxPathCount == nil {
		fmt.Fprintln(os.Stderr, "warning: no safety limits configured; processing untrusted input may allow denial-of-service (JSON bombs, stack exhaustion). Consider setting -max-depth, -max-path-length, and -max-path-count.")
	}

	q = q.WithLimits(limits)

	out, err := shaker.Shake(input, q)
	if err != nil {
		fmt.Fprintf(os.Stderr, "shake: %v\n", err)
		os.Exit(1)
	}

	fmt.Println(string(out))
}
