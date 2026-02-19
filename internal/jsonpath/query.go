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
// in a single query. Applied when [Limits].MaxPathCount is nil.
const MaxPathCount = 1000

type (
	ParseError = parser.ParseError
)

const MaxPathLength = parser.MaxPathLength

// Limits configures safety limits for JSON tree shaking.
//
// A nil field means "use the default constant" ([MaxDepth], [MaxPathLength],
// or [MaxPathCount]). The zero value of Limits therefore applies sensible
// defaults — safe by design.
//
// To explicitly disable a limit, set the field to a pointer to 0:
//
//	shaker.Limits{MaxDepth: shaker.Ptr(0)} // no depth limit
//
// Use [NoLimits] to disable all limits at once.
type Limits struct {
	MaxDepth      *int // Maximum JSON nesting depth (nil = MaxDepth default; 0 = no limit)
	MaxPathLength *int // Maximum byte length of a single JSONPath (nil = MaxPathLength default; 0 = no limit)
	MaxPathCount  *int // Maximum number of paths in a query (nil = MaxPathCount default; 0 = no limit)
}

// ptr returns a pointer to v.
func ptr[T any](v T) *T { return &v }

// DefaultLimits returns the default safety limits with each field set
// explicitly to its package-level constant. This is equivalent to the
// zero-value Limits{} but makes the values visible for inspection or logging.
func DefaultLimits() Limits {
	return Limits{
		MaxDepth:      ptr(MaxDepth),
		MaxPathLength: ptr(MaxPathLength),
		MaxPathCount:  ptr(MaxPathCount),
	}
}

// NoLimits returns a Limits value that explicitly disables all safety checks.
// Use this only when you fully trust both the JSON input and the JSONPath
// expressions — for example, in tests or internal pipelines.
//
// In production code that handles untrusted data, prefer the zero-value
// Limits{} (which applies sensible defaults) or [DefaultLimits].
func NoLimits() Limits {
	return Limits{
		MaxDepth:      ptr(0),
		MaxPathLength: ptr(0),
		MaxPathCount:  ptr(0),
	}
}

// Query describes a set of JSONPath expressions and a mode (include or exclude).
// Queries are lazy: paths are stored as strings until Compile() or first Shake().
type Query struct {
	mode     Mode
	paths    []string
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

// WithLimits returns a copy of the query with the given safety limits.
// Nil fields in l fall back to default constants. To disable all limits,
// pass [NoLimits].
func (q Query) WithLimits(l Limits) Query {
	return Query{
		mode:   q.mode,
		paths:  q.paths,
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
	maxDepth := MaxDepth // default when nil
	if q.limits.MaxDepth != nil {
		maxDepth = *q.limits.MaxDepth // 0 means no limit
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
	maxPathCount := MaxPathCount // default when nil
	if q.limits.MaxPathCount != nil {
		maxPathCount = *q.limits.MaxPathCount // 0 means no limit
	}
	if maxPathCount > 0 && len(q.paths) > maxPathCount {
		return nil, fmt.Errorf("query exceeds maximum path count of %d", maxPathCount)
	}

	maxPathLen := MaxPathLength // default when nil
	if q.limits.MaxPathLength != nil {
		maxPathLen = *q.limits.MaxPathLength // 0 means no limit
	}
	var parseOpts []parser.ParseOption
	if maxPathLen > 0 {
		parseOpts = append(parseOpts, parser.WithMaxLength(maxPathLen))
	}

	var errs []error
	var paths []*parser.Path

	for _, raw := range q.paths {
		p, err := parser.ParsePath(raw, parseOpts...)
		if err != nil {
			errs = append(errs, err)
			continue
		}
		paths = append(paths, p)
	}

	if len(errs) > 0 {
		return nil, errors.Join(errs...)
	}

	return buildTrie(paths), nil
}
