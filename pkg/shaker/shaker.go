// Package shaker provides JSON tree shaking — selecting or removing fields
// from JSON documents using JSONPath queries.
//
// Given a JSON document and a query (include or exclude), shaker returns a new
// document containing only the requested fields. Zero dependencies beyond the
// standard library.
//
// Basic usage:
//
//	s := shaker.New()
//	out, err := s.Shake(input, shaker.Include("$.name", "$.address.city"))
//
// Fluent API:
//
//	out, err := shaker.New().From(input).Include("$.name", "$.items[*].id").Shake()
//
// Pre-compiled queries for repeated use:
//
//	q, err := shaker.Include("$.name").Compile()
//	// q is now safe for concurrent use
//	out, err := shaker.New().Shake(input, q)
package shaker

import (
	"encoding/json"

	"github.com/mibar/tree-shaker/internal/jsonpath"
)

type Shaker struct{}

func New() *Shaker {
	return &Shaker{}
}

// Shake prunes the input JSON according to the query.
// In Include mode, only matched paths are kept.
// In Exclude mode, matched paths are removed.
//
// All path parse errors are aggregated into a single error via errors.Join.
// On parse error, no partial application occurs.
func (s *Shaker) Shake(input []byte, q Query) ([]byte, error) {
	var tree any
	if err := json.Unmarshal(input, &tree); err != nil {
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

func (s *Shaker) MustShake(input []byte, q Query) []byte {
	out, err := s.Shake(input, q)
	if err != nil {
		panic(err)
	}
	return out
}

func (s *Shaker) From(input []byte) *Builder {
	return &Builder{
		shaker: s,
		input:  input,
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
