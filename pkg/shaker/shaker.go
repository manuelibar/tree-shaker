// Package shaker provides JSON tree shaking — selecting or removing fields
// from JSON documents using JSONPath queries.
//
// Given a JSON document and a set of JSONPath expressions, shaker returns a
// new document containing only the requested fields (include mode) or
// everything except the specified fields (exclude mode).
//
// Zero dependencies beyond the standard library.
//
// # Usage
//
//	out, err := shaker.Shake(input, shaker.Include("$.name", "$.address.city"))
//
// # Fluent API
//
//	out, err := shaker.From(input).Include("$.name", "$.items[*].id").Shake()
//
// # Pre-compiled Queries
//
// Compile a query once and reuse it across many documents. A compiled [Query]
// is immutable and safe for concurrent use.
//
//	q, err := shaker.Include("$.name").Compile()
//	out, err := shaker.Shake(doc, q)
//
// # Security
//
// By default, shaker imposes no limits on JSON depth, path count, or path
// length. This is by design: it keeps the library simple for trusted
// environments where inputs are known to be well-formed.
//
// However, when processing untrusted input, the absence of limits exposes
// your application to several denial-of-service vectors:
//
//   - JSON bombs / deeply nested payloads — a document with thousands of
//     nesting levels causes deep recursion, potentially exhausting the
//     goroutine stack.
//   - Path flooding — a query with millions of JSONPath expressions
//     consumes unbounded memory during trie construction.
//   - Oversized paths — extremely long JSONPath strings waste CPU and
//     memory in the parser.
//
// To mitigate these risks, use [Query.WithLimits] or [DefaultLimits]:
//
//	// Apply recommended safe limits.
//	q := shaker.Include("$.name").WithLimits(shaker.DefaultLimits())
//
//	// Or set custom limits for your use-case.
//	q := shaker.Include("$.name").WithLimits(shaker.Limits{
//	    MaxDepth:      shaker.Ptr(200),
//	    MaxPathLength: shaker.Ptr(4096),
//	    MaxPathCount:  shaker.Ptr(100),
//	})
//
// If you are building an HTTP API or any service that accepts user-supplied
// JSONPath expressions or JSON documents, always apply limits.
package shaker

import (
	"bytes"
	"encoding/json"
	"fmt"

	"github.com/mibar/tree-shaker/internal/jsonpath"
)

// Shake prunes the input JSON according to the query.
//
// In include mode, only matched paths are kept.
// In exclude mode, matched paths are removed.
//
// All path parse errors are aggregated into a single error via [errors.Join].
// No partial application occurs — if any path is invalid, the entire operation fails.
//
// WARNING: by default, no safety limits are applied. If the query was not
// configured with [Query.WithLimits], deeply nested documents or adversarial
// inputs may cause excessive resource consumption. See the package-level
// Security section and [DefaultLimits].
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

// MustShake is like [Shake] but panics on error.
func MustShake(input []byte, q Query) []byte {
	out, err := Shake(input, q)
	if err != nil {
		panic(err)
	}
	return out
}

type (
	// Query describes a set of JSONPath expressions and a mode (include or exclude).
	Query = jsonpath.Query
	// Mode selects between include and exclude behaviour.
	Mode = jsonpath.Mode
	// ParseError describes a syntax error in a JSONPath expression.
	ParseError = jsonpath.ParseError
	// DepthError is returned when a JSON document exceeds the configured maximum depth.
	DepthError = jsonpath.DepthError
	// Limits configures safety limits for JSON tree shaking.
	//
	// A nil field means "no restriction". This is the zero-value default,
	// which means a plain [Query] runs without any limits. While convenient
	// for trusted inputs, this leaves the caller vulnerable to denial-of-service
	// attacks (JSON bombs, stack exhaustion, memory flooding) when processing
	// untrusted data. See the package-level Security section for details.
	//
	// Use [DefaultLimits] to obtain a recommended safe baseline, or set
	// individual fields with [Ptr]:
	//
	//	q = q.WithLimits(shaker.Limits{MaxDepth: shaker.Ptr(500)})
	Limits = jsonpath.Limits
)

// ShakeRequest is a wire-friendly representation of a shake query.
//
// Embed it in request types for transport-agnostic JSON field selection.
// When unmarshalled from JSON, the Query field is automatically populated
// from Mode and Paths — no extra conversion step is needed.
//
//	type APIRequest struct {
//	    Payload map[string]any      `json:"payload"`
//	    Shake   *shaker.ShakeRequest `json:"shake,omitempty"`
//	}
type ShakeRequest struct {
	Query Query    `json:"-"`
	Mode  string   `json:"mode"`  // "include" or "exclude"
	Paths []string `json:"paths"` // JSONPath expressions
}

// UnmarshalJSON implements [json.Unmarshaler].
//
// It decodes Mode and Paths from the JSON payload and builds the [Query]
// automatically. Returns an error if mode is invalid or paths is empty.
func (r *ShakeRequest) UnmarshalJSON(data []byte) error {
	// Alias avoids infinite recursion on UnmarshalJSON.
	type aux ShakeRequest
	var raw aux
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}

	if len(raw.Paths) == 0 {
		return fmt.Errorf("shake request: paths must not be empty")
	}

	switch raw.Mode {
	case "include":
		raw.Query = Include(raw.Paths...)
	case "exclude":
		raw.Query = Exclude(raw.Paths...)
	default:
		return fmt.Errorf("shake request: invalid mode %q (expected \"include\" or \"exclude\")", raw.Mode)
	}

	*r = ShakeRequest(raw)
	return nil
}

const (
	ModeInclude = jsonpath.ModeInclude
	ModeExclude = jsonpath.ModeExclude
)

const (
	MaxDepth      = jsonpath.MaxDepth
	MaxPathLength = jsonpath.MaxPathLength
	MaxPathCount  = jsonpath.MaxPathCount
)

// DefaultLimits returns the recommended safety limits for untrusted input.
//
// Current defaults:
//   - MaxDepth:      1 000 — prevents stack exhaustion from deeply nested JSON.
//   - MaxPathLength: 10 000 — caps parser work per JSONPath string.
//   - MaxPathCount:  1 000 — bounds trie memory for large query sets.
//
// Always apply these (or stricter) limits when parsing user-supplied JSON or
// JSONPath expressions. Omitting limits on untrusted input may allow
// denial-of-service attacks such as JSON bombs or path flooding.
func DefaultLimits() Limits { return jsonpath.DefaultLimits() }

// Ptr returns a pointer to v. It is a convenience helper for constructing [Limits]:
//
//	shaker.Limits{MaxDepth: shaker.Ptr(500)}
func Ptr[T any](v T) *T { return &v }

// Include returns an include-mode [Query] for the given JSONPath expressions.
func Include(paths ...string) Query { return jsonpath.Include(paths...) }

// Exclude returns an exclude-mode [Query] for the given JSONPath expressions.
func Exclude(paths ...string) Query { return jsonpath.Exclude(paths...) }

// MustCompile is like [Query.Compile] but panics on error.
func MustCompile(q Query) Query { return jsonpath.MustCompile(q) }
