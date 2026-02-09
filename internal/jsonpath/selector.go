package jsonpath

import "fmt"

// Selector matches a JSON key (string for objects) or index (int for arrays).
type Selector interface {
	// Match returns true if the selector matches the given key.
	// key is string for object keys, int for array indices.
	// arrLen is the array length (used for negative index resolution); ignored for object keys.
	Match(key any, arrLen int) bool
	String() string
}

// NameSelector matches an exact object key.
type NameSelector struct {
	Name string
}

func (s NameSelector) Match(key any, _ int) bool {
	k, ok := key.(string)
	return ok && k == s.Name
}

func (s NameSelector) String() string {
	return s.Name
}

// IndexSelector matches an exact array index, supporting negative indices.
type IndexSelector struct {
	Index int
}

func (s IndexSelector) Match(key any, arrLen int) bool {
	idx, ok := key.(int)
	if !ok {
		return false
	}
	resolved := s.Index
	if resolved < 0 {
		resolved = arrLen + resolved
	}
	return idx == resolved
}

func (s IndexSelector) String() string {
	return fmt.Sprintf("[%d]", s.Index)
}

// WildcardSelector matches any key or index.
type WildcardSelector struct{}

func (s WildcardSelector) Match(_ any, _ int) bool {
	return true
}

func (s WildcardSelector) String() string {
	return "*"
}

// SliceSelector matches array indices within a range per RFC 9535 slice semantics.
// nil pointers use default values: Start=0, End=arrLen, Step=1.
type SliceSelector struct {
	Start *int
	End   *int
	Step  *int
}

func (s SliceSelector) Match(key any, arrLen int) bool {
	idx, ok := key.(int)
	if !ok {
		return false
	}

	step := 1
	if s.Step != nil {
		step = *s.Step
	}
	if step == 0 {
		return false
	}

	start, end := s.bounds(arrLen, step)

	if step > 0 {
		if idx < start || idx >= end {
			return false
		}
		return (idx-start)%step == 0
	}

	// step < 0
	if idx > start || idx <= end {
		return false
	}
	return (start-idx)%(-step) == 0
}

// bounds resolves start/end defaults and clamps per RFC 9535.
func (s SliceSelector) bounds(arrLen, step int) (int, int) {
	if step > 0 {
		start := 0
		if s.Start != nil {
			start = normalize(*s.Start, arrLen)
		}
		end := arrLen
		if s.End != nil {
			end = normalize(*s.End, arrLen)
		}
		return clamp(start, 0, arrLen), clamp(end, 0, arrLen)
	}

	// step < 0
	start := arrLen - 1
	if s.Start != nil {
		start = normalize(*s.Start, arrLen)
	}
	end := -1
	if s.End != nil {
		end = normalize(*s.End, arrLen)
	}
	return clamp(start, -1, arrLen-1), clamp(end, -1, arrLen-1)
}

func (s SliceSelector) String() string {
	var startStr, endStr, stepStr string
	if s.Start != nil {
		startStr = fmt.Sprintf("%d", *s.Start)
	}
	if s.End != nil {
		endStr = fmt.Sprintf("%d", *s.End)
	}
	if s.Step != nil {
		stepStr = fmt.Sprintf("%d", *s.Step)
		return fmt.Sprintf("[%s:%s:%s]", startStr, endStr, stepStr)
	}
	return fmt.Sprintf("[%s:%s]", startStr, endStr)
}

// normalize converts a potentially negative index to its absolute form.
func normalize(idx, arrLen int) int {
	if idx < 0 {
		return idx + arrLen
	}
	return idx
}

// clamp restricts val to [lo, hi].
func clamp(val, lo, hi int) int {
	if val < lo {
		return lo
	}
	if val > hi {
		return hi
	}
	return val
}
