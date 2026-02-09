package jsonpath

import "fmt"

const (
	// MaxDepth is the maximum JSON nesting depth before returning DepthError.
	MaxDepth = 1000

	// MaxPathLength is the maximum byte length of a single JSONPath string.
	MaxPathLength = 10000

	// MaxPathCount is the maximum number of paths in a single query.
	MaxPathCount = 1000
)

// ParseError is returned when a JSONPath string cannot be parsed.
type ParseError struct {
	Path    string
	Pos     int
	Message string
}

func (e *ParseError) Error() string {
	return fmt.Sprintf("parse error at position %d in %q: %s", e.Pos, e.Path, e.Message)
}

// DepthError is returned when JSON nesting exceeds MaxDepth.
type DepthError struct {
	Depth int
}

func (e *DepthError) Error() string {
	return fmt.Sprintf("maximum JSON depth %d exceeded at depth %d", MaxDepth, e.Depth)
}
