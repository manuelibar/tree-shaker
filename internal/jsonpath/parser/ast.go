package parser

// ast.go defines the Abstract Syntax Tree (AST) for parsed JSONPath expressions.
//
// A parsed JSONPath is a [Path] containing an ordered sequence of [Segment]s,
// each holding one or more [Selector]s. The Descendant flag on a [Segment]
// represents the ".." (recursive descent) operator: selectors in that segment
// may match at any depth in the JSON tree.

// Segment is a single step in a JSONPath expression.
//
// Each segment contains one or more selectors (multi-selector brackets like [a,b]
// produce multiple selectors in the same segment) and an optional Descendant flag
// that indicates the ".." (recursive descent) operator preceded this segment.
type Segment struct {
	Selectors  []Selector
	Descendant bool
}

// WithoutDescendant returns a copy of the segment with Descendant set to false.
func (s Segment) WithoutDescendant() Segment {
	return Segment{Selectors: s.Selectors, Descendant: false}
}

// Path is a parsed JSONPath expression (the AST root).
//
// Raw preserves the original string for error messages and debugging.
// Segments is the ordered sequence of steps from root ($) to the target node.
type Path struct {
	Segments []Segment
	Raw      string
}

// IsAbsolute reports whether the path starts with "$" (root reference).
func (p Path) IsAbsolute() bool {
	return len(p.Raw) > 0 && p.Raw[0] == '$'
}

// Prepend returns a new Path with the prefix's segments before this path's segments.
func (p Path) Prepend(prefix Path) Path {
	segments := make([]Segment, 0, len(prefix.Segments)+len(p.Segments))
	segments = append(segments, prefix.Segments...)
	segments = append(segments, p.Segments...)
	return Path{Segments: segments, Raw: prefix.Raw + p.Raw}
}
