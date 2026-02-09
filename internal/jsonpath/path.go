package jsonpath

// Segment is a single step in a JSONPath, containing one or more selectors.
// If Descendant is true, this segment was preceded by ".." (recursive descent).
type Segment struct {
	Selectors  []Selector
	Descendant bool
}

// Path is a compiled JSONPath expression.
type Path struct {
	Segments []Segment
	Raw      string
}
