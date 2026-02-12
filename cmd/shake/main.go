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

	req := shaker.ShakeRequest{
		Mode:  *mode,
		Paths: strings.Split(*paths, ","),
	}

	q, err := req.ToQuery()
	if err != nil {
		fmt.Fprintf(os.Stderr, "invalid query: %v\n", err)
		os.Exit(1)
	}

	out, err := shaker.Shake(input, q)
	if err != nil {
		fmt.Fprintf(os.Stderr, "shake: %v\n", err)
		os.Exit(1)
	}

	fmt.Println(string(out))
}
