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
// # Pre-compiled Queries
//
// Compile a query once and reuse it across many documents. A compiled [Query]
// is immutable and safe for concurrent use.
//
//	q, err := shaker.Include("$.name").Compile()
//	out, err := shaker.Shake(doc, q)
//
// # Security — Safe by Default
//
// Shaker ships with sensible safety limits enabled out of the box.
// A zero-value [Limits] (i.e. no call to [Query.WithLimits] at all) enforces:
//
//   - [MaxDepth] (1 000) — prevents stack exhaustion from deeply nested
//     JSON documents (a.k.a. "JSON bombs").
//   - [MaxPathLength] (10 000) — caps parser work per JSONPath string,
//     preventing memory and CPU abuse from oversized expressions.
//   - [MaxPathCount] (1 000) — bounds trie memory, preventing path-flooding
//     attacks that submit millions of JSONPath expressions.
//
// You can tighten these limits for your use-case:
//
//	q := shaker.Include("$.name").WithLimits(shaker.Limits{
//	    MaxDepth:      ptr(200),
//	    MaxPathLength: ptr(4096),
//	    MaxPathCount:  ptr(100),
//	})
//
// To explicitly disable all limits (e.g. in trusted internal pipelines or
// tests), opt in with [NoLimits]:
//
//	q := shaker.Include("$.name").WithLimits(shaker.NoLimits())
package shaker

import (
	"bytes"
	"encoding/json"
	"errors"
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
// Safety limits ([MaxDepth], [MaxPathLength], [MaxPathCount]) are applied by
// default. Use [Query.WithLimits] to customise them or [NoLimits] to
// disable them.
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
	// A nil field means "use the default constant" — safe by default.
	// To explicitly disable a check, set the field to ptr(0).
	// Use [NoLimits] to disable all limits at once.
	//
	// See the package-level Security section for details.
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

	var errs []error

	if len(raw.Paths) == 0 {
		errs = append(errs, fmt.Errorf("shake request: paths must not be empty"))
	}

	switch raw.Mode {
	case "include":
		raw.Query = Include(raw.Paths...)
	case "exclude":
		raw.Query = Exclude(raw.Paths...)
	default:
		errs = append(errs, fmt.Errorf("shake request: invalid mode %q (expected \"include\" or \"exclude\")", raw.Mode))
	}

	if err := errors.Join(errs...); err != nil {
		return err
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

// DefaultLimits returns the default safety limits with each field set
// explicitly to its package-level constant. This is equivalent to the
// zero-value Limits{} but makes the values visible for inspection or logging.
//
// Current defaults:
//   - MaxDepth:      1 000
//   - MaxPathLength: 10 000
//   - MaxPathCount:  1 000
func DefaultLimits() Limits { return jsonpath.DefaultLimits() }

// NoLimits returns a [Limits] value that explicitly disables all safety
// checks. Use this only when you fully trust both the JSON input and the
// JSONPath expressions — for example, in tests or internal pipelines.
func NoLimits() Limits { return jsonpath.NoLimits() }

// Include returns an include-mode [Query] for the given JSONPath expressions.
func Include(paths ...string) Query { return jsonpath.Include(paths...) }

// Exclude returns an exclude-mode [Query] for the given JSONPath expressions.
func Exclude(paths ...string) Query { return jsonpath.Exclude(paths...) }

// MustCompile is like [Query.Compile] but panics on error.
func MustCompile(q Query) Query { return jsonpath.MustCompile(q) }
