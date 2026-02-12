// Package shaker provides JSON tree shaking — selecting or removing fields
// from JSON documents using JSONPath queries.
//
// Given a JSON document and a query (include or exclude), shaker returns a new
// document containing only the requested fields. Zero dependencies beyond the
// standard library.
//
// Basic usage:
//
//	out, err := shaker.Shake(input, shaker.Include("$.name", "$.address.city"))
//
// Fluent API:
//
//	out, err := shaker.From(input).Include("$.name", "$.items[*].id").Shake()
//
// Pre-compiled queries for repeated use:
//
//	q, err := shaker.Include("$.name").Compile()
//	// q is now safe for concurrent use
//	out, err := shaker.Shake(input, q)
package shaker

import (
	"bytes"
	"encoding/json"

	"github.com/mibar/tree-shaker/internal/jsonpath"
)

// Shake prunes the input JSON according to the query.
// In Include mode, only matched paths are kept.
// In Exclude mode, matched paths are removed.
//
// All path parse errors are aggregated into a single error via errors.Join.
// On parse error, no partial application occurs.
func Shake(input []byte, q Query) ([]byte, error) {
	dec := json.NewDecoder(bytes.NewReader(input))
	dec.UseNumber()

	var tree any
	if err := dec.Decode(&tree); err != nil {
		return nil, err
	}

	result, err := q.Walk(tree)
	if err != nil {
		return nil, err
	}

	if result == nil {
		switch tree.(type) {
		case map[string]any:
			return []byte("{}"), nil
		case []any:
			return []byte("[]"), nil
		}
	}

	return json.Marshal(result)
}

// MustShake is like Shake but panics on error.
func MustShake(input []byte, q Query) []byte {
	out, err := Shake(input, q)
	if err != nil {
		panic(err)
	}
	return out
}

// From starts a fluent builder for the given input JSON.
func From(input []byte) *Builder {
	return &Builder{
		input: input,
	}
}

type (
	Query        = jsonpath.Query
	Mode         = jsonpath.Mode
	ParseError   = jsonpath.ParseError
	DepthError   = jsonpath.DepthError
	ShakeRequest = jsonpath.ShakeRequest
)

const (
	ModeInclude = jsonpath.ModeInclude
	ModeExclude = jsonpath.ModeExclude
)

const (
	MaxDepth      = jsonpath.MaxDepth
	MaxPathLength = jsonpath.MaxPathLength
	MaxPathCount  = jsonpath.MaxPathCount
)

func Include(paths ...string) Query { return jsonpath.Include(paths...) }

func Exclude(paths ...string) Query { return jsonpath.Exclude(paths...) }

func MustCompile(q Query) Query { return jsonpath.MustCompile(q) }
