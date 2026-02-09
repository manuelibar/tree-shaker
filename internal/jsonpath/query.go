package jsonpath

import (
	"errors"
	"fmt"
)

// Mode specifies whether a query includes or excludes matched paths.
type Mode int

const (
	ModeInclude Mode = iota
	ModeExclude
)

// Query describes a set of JSONPath expressions and a mode (include or exclude).
// Queries are lazy: paths are stored as strings until Compile() or first Shake().
type Query struct {
	mode     Mode
	rawPaths []string
	prefix   string
	compiled *trieNode // nil until compiled
}

// Include creates a query that keeps only the matched paths (GraphQL-like).
func Include(paths ...string) Query {
	return Query{mode: ModeInclude, rawPaths: paths}
}

// Exclude creates a query that removes the matched paths (keep everything else).
func Exclude(paths ...string) Query {
	return Query{mode: ModeExclude, rawPaths: paths}
}

// WithPrefix returns a copy of the query with all paths scoped under prefix.
// Relative paths (starting with ".") are appended to the prefix.
// Paths starting with "$" are left as-is.
func (q Query) WithPrefix(prefix string) Query {
	return Query{
		mode:     q.mode,
		rawPaths: q.rawPaths,
		prefix:   prefix,
	}
}

// Compile eagerly parses all paths and builds the trie.
// All parse errors are aggregated via errors.Join.
// Returns a compiled copy of the query.
//
// A compiled Query is safe for concurrent use from multiple goroutines.
// An uncompiled Query is NOT safe for concurrent use because ensureCompiled
// mutates the receiver.
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
		rawPaths: q.rawPaths,
		prefix:   q.prefix,
		compiled: trie,
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
	switch q.mode {
	case ModeInclude:
		return walkInclude(tree, q.compiled, 0)
	case ModeExclude:
		return walkExclude(tree, q.compiled, 0)
	}
	return nil, nil
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
	if len(q.rawPaths) > MaxPathCount {
		return nil, fmt.Errorf("query exceeds maximum path count of %d", MaxPathCount)
	}

	var errs []error
	var paths []*Path

	for _, raw := range q.rawPaths {
		fullPath := q.applyPrefix(raw)
		p, err := parsePath(fullPath)
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

// applyPrefix prepends the prefix to a path if the path is relative.
func (q *Query) applyPrefix(raw string) string {
	if q.prefix == "" {
		return raw
	}
	// If path starts with "$", leave it as-is
	if len(raw) > 0 && raw[0] == '$' {
		return raw
	}
	// Append relative path to prefix
	return q.prefix + raw
}

// ShakeRequest is a wire-friendly representation of a shake query.
// Embed it in your request types for transport-agnostic JSON field selection.
//
//	type APIRequest struct {
//	    UserID string        `json:"user_id"`
//	    Shake  *ShakeRequest `json:"shake,omitempty"`
//	}
type ShakeRequest struct {
	Mode  string   `json:"mode"`  // "include" or "exclude"
	Paths []string `json:"paths"` // JSONPath expressions
}

// ToQuery converts a ShakeRequest into a Query ready for Shake().
// Returns an error if mode is not "include" or "exclude", or if paths is empty.
func (r ShakeRequest) ToQuery() (Query, error) {
	if len(r.Paths) == 0 {
		return Query{}, fmt.Errorf("shake request: paths must not be empty")
	}

	var q Query
	switch r.Mode {
	case "include":
		q = Include(r.Paths...)
	case "exclude":
		q = Exclude(r.Paths...)
	default:
		return Query{}, fmt.Errorf("shake request: invalid mode %q (expected \"include\" or \"exclude\")", r.Mode)
	}

	return q, nil
}
