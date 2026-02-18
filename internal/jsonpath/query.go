// Package jsonpath implements JSON tree shaking via JSONPath expressions.
//
// It compiles one or more JSONPath strings into a prefix trie (automaton),
// then walks a parsed JSON tree guided by that trie to produce a pruned copy.
// Two modes are supported:
//
//   - Include: build a new tree containing only matched paths.
//   - Exclude: clone the tree, omitting matched paths.
//
// # Pipeline
//
// Processing follows a three-stage pipeline:
//
//  1. Parse — JSONPath strings are parsed into an AST.
//  2. Compile — ASTs are merged into a shared prefix trie.
//  3. Walk — the JSON tree and trie are traversed in lockstep to produce the result.
//
// Stages 1–2 happen once (at Compile time or lazily on first Walk).
// Stage 3 runs per document.
//
// # Concurrency
//
// A compiled [Query] is immutable and safe for concurrent use.
// An uncompiled Query is NOT safe for concurrent use.
//
// This package is internal. The public API is in pkg/shaker.
package jsonpath

import (
	"errors"
	"fmt"

	"github.com/mibar/tree-shaker/internal/jsonpath/parser"
)

// Mode selects between include and exclude behaviour for a [Query].
type Mode int

const (
	ModeInclude Mode = iota // Keep only matched paths.
	ModeExclude             // Remove matched paths, keep everything else.
)

// MaxPathCount is the default maximum number of JSONPath expressions allowed
// in a single query. Applied by [DefaultLimits]; ignored when Limits is zero.
const MaxPathCount = 1000

type (
	ParseError = parser.ParseError
)

const MaxPathLength = parser.MaxPathLength

// Limits configures safety limits for JSON tree shaking.
//
// A nil field means "no restriction". The zero value of Limits is therefore
// fully unrestricted, which is appropriate for trusted inputs but dangerous
// for untrusted data (JSON bombs, stack exhaustion, memory flooding).
// Always set limits when processing external input.
//
// Use [DefaultLimits] for a recommended safe baseline.
type Limits struct {
	MaxDepth      *int // Maximum JSON nesting depth (nil = unrestricted)
	MaxPathLength *int // Maximum byte length of a single JSONPath (nil = unrestricted)
	MaxPathCount  *int // Maximum number of paths in a query (nil = unrestricted)
}

// ptr returns a pointer to v.
func ptr[T any](v T) *T { return &v }

// DefaultLimits returns the recommended safety limits for untrusted input.
// Each field is set to its corresponding package-level constant.
func DefaultLimits() Limits {
	return Limits{
		MaxDepth:      ptr(MaxDepth),
		MaxPathLength: ptr(MaxPathLength),
		MaxPathCount:  ptr(MaxPathCount),
	}
}

// Query describes a set of JSONPath expressions and a mode (include or exclude).
// Queries are lazy: paths are stored as strings until Compile() or first Shake().
type Query struct {
	mode     Mode
	paths    []string
	prefix   string
	compiled *trieNode // nil until compiled
	limits   Limits
}

// Include creates a query that keeps only the matched paths (GraphQL-like).
func Include(paths ...string) Query {
	return Query{mode: ModeInclude, paths: paths}
}

// Exclude creates a query that removes the matched paths (keep everything else).
func Exclude(paths ...string) Query {
	return Query{mode: ModeExclude, paths: paths}
}

// WithPrefix returns a copy of the query with all paths scoped under prefix.
// Relative paths (starting with ".") are appended to the prefix.
// Paths starting with "$" are left as-is.
func (q Query) WithPrefix(prefix string) Query {
	return Query{
		mode:   q.mode,
		paths:  q.paths,
		prefix: prefix,
		limits: q.limits,
	}
}

// WithLimits returns a copy of the query with the given safety limits.
// Nil fields in l mean "no restriction". To apply all recommended defaults,
// pass [DefaultLimits].
func (q Query) WithLimits(l Limits) Query {
	return Query{
		mode:   q.mode,
		paths:  q.paths,
		prefix: q.prefix,
		limits: l,
	}
}

// Compile eagerly parses all paths and builds the trie.
// All parse errors are aggregated via errors.Join.
// Returns a compiled copy of the query.
//
// A compiled Query is safe for concurrent use from multiple goroutines.
// An uncompiled Query is NOT safe for concurrent use.
func (q Query) Compile() (Query, error) {
	if q.compiled != nil {
		return q, nil
	}

	trie, err := q.buildTrie()
	if err != nil {
		return q, err
	}

	return Query{
		mode:     q.mode,
		paths:    q.paths,
		prefix:   q.prefix,
		compiled: trie,
		limits:   q.limits,
	}, nil
}

// MustCompile is like Compile but panics on error.
func MustCompile(q Query) Query {
	compiled, err := q.Compile()
	if err != nil {
		panic(err)
	}
	return compiled
}

// Walk applies the query to a parsed JSON tree.
func (q *Query) Walk(tree any) (any, error) {
	if err := q.compile(); err != nil {
		return nil, err
	}
	var maxDepth int
	if q.limits.MaxDepth != nil {
		maxDepth = *q.limits.MaxDepth
	}
	return newWalker(q.mode, maxDepth).walk(tree, q.compiled, 0)
}

func (q *Query) compile() error {
	if q.compiled != nil {
		return nil
	}
	trie, err := q.buildTrie()
	if err != nil {
		return err
	}
	q.compiled = trie
	return nil
}

// IsInclude reports whether the query is in include mode.
func (q Query) IsInclude() bool { return q.mode == ModeInclude }

// buildTrie parses all raw paths and builds the trie.
func (q *Query) buildTrie() (*trieNode, error) {
	if q.limits.MaxPathCount != nil && len(q.paths) > *q.limits.MaxPathCount {
		return nil, fmt.Errorf("query exceeds maximum path count of %d", *q.limits.MaxPathCount)
	}

	var parseOpts []parser.ParseOption
	if q.limits.MaxPathLength != nil {
		parseOpts = append(parseOpts, parser.WithMaxLength(*q.limits.MaxPathLength))
	}

	var prefix *parser.Path
	if q.prefix != "" {
		var err error
		prefix, err = parser.ParsePath(q.prefix, parseOpts...)
		if err != nil {
			return nil, err
		}
	}

	var errs []error
	var paths []*parser.Path

	for _, raw := range q.paths {
		p, err := parser.ParsePath(raw, parseOpts...)
		if err != nil {
			errs = append(errs, err)
			continue
		}
		if prefix != nil && !p.IsAbsolute() {
			scoped := p.Prepend(*prefix)
			p = &scoped
		}
		paths = append(paths, p)
	}

	if len(errs) > 0 {
		return nil, errors.Join(errs...)
	}

	return buildTrie(paths), nil
}
