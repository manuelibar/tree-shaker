package shaker

import "github.com/mibar/tree-shaker/internal/jsonpath"

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
	// To explicitly disable a check, set the field to [Ptr](0).
	// Use [NoLimits] to disable all limits at once.
	//
	// See the package-level Security section for details.
	Limits = jsonpath.Limits
)

const (
	// ModeInclude selects include behaviour — keep only matched paths.
	ModeInclude = jsonpath.ModeInclude
	// ModeExclude selects exclude behaviour — remove matched paths, keep everything else.
	ModeExclude = jsonpath.ModeExclude
)

const (
	// MaxDepth is the default maximum JSON nesting depth (1 000).
	MaxDepth = jsonpath.MaxDepth
	// MaxPathLength is the default maximum byte length of a single JSONPath expression (10 000).
	MaxPathLength = jsonpath.MaxPathLength
	// MaxPathCount is the default maximum number of JSONPath expressions per query (1 000).
	MaxPathCount = jsonpath.MaxPathCount
)

// Ptr returns a pointer to v. Useful for constructing [Limits] with custom
// values inline:
//
//	shaker.Limits{MaxDepth: shaker.Ptr(200)}
func Ptr[T any](v T) *T { return &v }

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
