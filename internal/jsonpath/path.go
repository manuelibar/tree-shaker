package jsonpath

// Segment is a single step in a JSONPath expression.
// It contains one or more selectors and an optional descendant flag (.. operator).
type Segment struct {
	Selectors  []Selector
	Descendant bool
}

// WithoutDescendant returns a copy of the segment with Descendant set to false.
func (s Segment) WithoutDescendant() Segment {
	return Segment{Selectors: s.Selectors, Descendant: false}
}

// Path is a parsed JSONPath expression.
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
