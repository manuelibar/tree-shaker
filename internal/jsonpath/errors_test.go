package jsonpath

import (
	"testing"
)

func TestParseErrorMessage(t *testing.T) {
	err := &ParseError{Path: "$.foo[", Pos: 5, Message: "unclosed '['"}
	got := err.Error()
	want := `parse error at position 5 in "$.foo[": unclosed '['`
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestDepthErrorMessage(t *testing.T) {
	err := &DepthError{Depth: 1001}
	got := err.Error()
	if got == "" {
		t.Error("expected non-empty error message")
	}
}

func TestConstants(t *testing.T) {
	if MaxDepth != 1000 {
		t.Errorf("MaxDepth = %d, want 1000", MaxDepth)
	}
	if MaxPathLength != 10000 {
		t.Errorf("MaxPathLength = %d, want 10000", MaxPathLength)
	}
	if MaxPathCount != 1000 {
		t.Errorf("MaxPathCount = %d, want 1000", MaxPathCount)
	}
}
