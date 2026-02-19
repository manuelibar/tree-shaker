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
//	    MaxDepth:      shaker.Ptr(200),
//	    MaxPathLength: shaker.Ptr(4096),
//	    MaxPathCount:  shaker.Ptr(100),
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

// ShakeRequest is a wire-friendly representation of a shake query.
//
// Embed it in request types for transport-agnostic JSON field selection.
// Call [ShakeRequest.Query] to obtain the [Query] derived from Mode and Paths.
//
//	type APIRequest struct {
//	    Payload map[string]any      `json:"payload"`
//	    Shake   *shaker.ShakeRequest `json:"shake,omitempty"`
//	}
type ShakeRequest struct {
	Mode  string   `json:"mode"`  // "include" or "exclude"
	Paths []string `json:"paths"` // JSONPath expressions
}

// Query returns a [Query] derived from Mode and Paths.
//
// If Mode is "include", it returns [Include](Paths...).
// If Mode is "exclude", it returns [Exclude](Paths...).
// For any other Mode, it returns an empty include query.
func (r ShakeRequest) Query() Query {
	switch r.Mode {
	case "include":
		return Include(r.Paths...)
	case "exclude":
		return Exclude(r.Paths...)
	default:
		return Include()
	}
}

// UnmarshalJSON implements [json.Unmarshaler].
//
// It decodes Mode and Paths from the JSON payload and validates them.
// Returns an error if mode is invalid or paths is empty.
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
	case "include", "exclude":
		// valid
	default:
		errs = append(errs, fmt.Errorf("shake request: invalid mode %q (expected \"include\" or \"exclude\")", raw.Mode))
	}

	if err := errors.Join(errs...); err != nil {
		return err
	}

	*r = ShakeRequest(raw)
	return nil
}
