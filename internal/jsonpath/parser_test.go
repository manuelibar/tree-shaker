package jsonpath

import (
	"testing"
)

func TestParseSimpleName(t *testing.T) {
	p, err := parsePath("$.foo")
	if err != nil {
		t.Fatal(err)
	}
	if len(p.Segments) != 1 {
		t.Fatalf("expected 1 segment, got %d", len(p.Segments))
	}
	if p.Segments[0].Selectors[0].(NameSelector).Name != "foo" {
		t.Error("expected name 'foo'")
	}
}

func TestParseNestedPath(t *testing.T) {
	p, err := parsePath("$.data.users")
	if err != nil {
		t.Fatal(err)
	}
	if len(p.Segments) != 2 {
		t.Fatalf("expected 2 segments, got %d", len(p.Segments))
	}
	if p.Segments[0].Selectors[0].(NameSelector).Name != "data" {
		t.Error("expected 'data'")
	}
	if p.Segments[1].Selectors[0].(NameSelector).Name != "users" {
		t.Error("expected 'users'")
	}
}

func TestParseWithoutDollar(t *testing.T) {
	p, err := parsePath(".foo.bar")
	if err != nil {
		t.Fatal(err)
	}
	if len(p.Segments) != 2 {
		t.Fatalf("expected 2 segments, got %d", len(p.Segments))
	}
}

func TestParseWildcard(t *testing.T) {
	p, err := parsePath("$.users.*")
	if err != nil {
		t.Fatal(err)
	}
	if len(p.Segments) != 2 {
		t.Fatalf("expected 2 segments, got %d", len(p.Segments))
	}
	if _, ok := p.Segments[1].Selectors[0].(WildcardSelector); !ok {
		t.Error("expected wildcard selector")
	}
}

func TestParseRecursiveDescent(t *testing.T) {
	p, err := parsePath("$..name")
	if err != nil {
		t.Fatal(err)
	}
	if len(p.Segments) != 1 {
		t.Fatalf("expected 1 segment, got %d", len(p.Segments))
	}
	if !p.Segments[0].Descendant {
		t.Error("expected descendant flag")
	}
	if p.Segments[0].Selectors[0].(NameSelector).Name != "name" {
		t.Error("expected name 'name'")
	}
}

func TestParseBracketIndex(t *testing.T) {
	p, err := parsePath("$.items[0]")
	if err != nil {
		t.Fatal(err)
	}
	if len(p.Segments) != 2 {
		t.Fatalf("expected 2 segments, got %d", len(p.Segments))
	}
	idx, ok := p.Segments[1].Selectors[0].(IndexSelector)
	if !ok {
		t.Fatal("expected index selector")
	}
	if idx.Index != 0 {
		t.Errorf("expected index 0, got %d", idx.Index)
	}
}

func TestParseNegativeIndex(t *testing.T) {
	p, err := parsePath("$.items[-1]")
	if err != nil {
		t.Fatal(err)
	}
	idx := p.Segments[1].Selectors[0].(IndexSelector)
	if idx.Index != -1 {
		t.Errorf("expected -1, got %d", idx.Index)
	}
}

func TestParseBracketWildcard(t *testing.T) {
	p, err := parsePath("$.items[*]")
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := p.Segments[1].Selectors[0].(WildcardSelector); !ok {
		t.Error("expected wildcard selector")
	}
}

func TestParseStringSelector(t *testing.T) {
	p, err := parsePath("$['special-key']")
	if err != nil {
		t.Fatal(err)
	}
	name := p.Segments[0].Selectors[0].(NameSelector)
	if name.Name != "special-key" {
		t.Errorf("expected 'special-key', got %q", name.Name)
	}
}

func TestParseDoubleQuotedString(t *testing.T) {
	p, err := parsePath(`$["special-key"]`)
	if err != nil {
		t.Fatal(err)
	}
	name := p.Segments[0].Selectors[0].(NameSelector)
	if name.Name != "special-key" {
		t.Errorf("expected 'special-key', got %q", name.Name)
	}
}

func TestParseSlice(t *testing.T) {
	p, err := parsePath("$[0:5]")
	if err != nil {
		t.Fatal(err)
	}
	s, ok := p.Segments[0].Selectors[0].(SliceSelector)
	if !ok {
		t.Fatal("expected slice selector")
	}
	if s.Start == nil || *s.Start != 0 {
		t.Error("expected start=0")
	}
	if s.End == nil || *s.End != 5 {
		t.Error("expected end=5")
	}
	if s.Step != nil {
		t.Error("expected step=nil")
	}
}

func TestParseSliceWithStep(t *testing.T) {
	p, err := parsePath("$[::2]")
	if err != nil {
		t.Fatal(err)
	}
	s := p.Segments[0].Selectors[0].(SliceSelector)
	if s.Start != nil {
		t.Error("expected start=nil")
	}
	if s.End != nil {
		t.Error("expected end=nil")
	}
	if s.Step == nil || *s.Step != 2 {
		t.Error("expected step=2")
	}
}

func TestParseMultiSelector(t *testing.T) {
	p, err := parsePath("$[0,1,2]")
	if err != nil {
		t.Fatal(err)
	}
	if len(p.Segments[0].Selectors) != 3 {
		t.Fatalf("expected 3 selectors, got %d", len(p.Segments[0].Selectors))
	}
	for i := 0; i < 3; i++ {
		idx := p.Segments[0].Selectors[i].(IndexSelector)
		if idx.Index != i {
			t.Errorf("selector %d: expected %d, got %d", i, i, idx.Index)
		}
	}
}

func TestParseMultiStringSelector(t *testing.T) {
	p, err := parsePath("$['a','b']")
	if err != nil {
		t.Fatal(err)
	}
	if len(p.Segments[0].Selectors) != 2 {
		t.Fatalf("expected 2 selectors, got %d", len(p.Segments[0].Selectors))
	}
	if p.Segments[0].Selectors[0].(NameSelector).Name != "a" {
		t.Error("expected 'a'")
	}
	if p.Segments[0].Selectors[1].(NameSelector).Name != "b" {
		t.Error("expected 'b'")
	}
}

func TestParseRecursiveDescentWildcard(t *testing.T) {
	p, err := parsePath("$..*")
	if err != nil {
		t.Fatal(err)
	}
	if len(p.Segments) != 1 {
		t.Fatalf("expected 1 segment, got %d", len(p.Segments))
	}
	if !p.Segments[0].Descendant {
		t.Error("expected descendant")
	}
	if _, ok := p.Segments[0].Selectors[0].(WildcardSelector); !ok {
		t.Error("expected wildcard")
	}
}

func TestParseRecursiveDescentBracket(t *testing.T) {
	p, err := parsePath("$..[0]")
	if err != nil {
		t.Fatal(err)
	}
	if !p.Segments[0].Descendant {
		t.Error("expected descendant")
	}
	idx := p.Segments[0].Selectors[0].(IndexSelector)
	if idx.Index != 0 {
		t.Errorf("expected 0, got %d", idx.Index)
	}
}

func TestParseErrorEmpty(t *testing.T) {
	_, err := parsePath("$")
	if err == nil {
		t.Error("expected error for empty path")
	}
}

func TestParseErrorUnclosedBracket(t *testing.T) {
	_, err := parsePath("$.foo[")
	if err == nil {
		t.Error("expected error for unclosed bracket")
	}
	pe, ok := err.(*ParseError)
	if !ok {
		t.Fatal("expected ParseError")
	}
	if pe.Pos != 6 {
		t.Errorf("expected pos 6, got %d", pe.Pos)
	}
}

func TestParseErrorTrailingDot(t *testing.T) {
	_, err := parsePath("$.foo.")
	if err == nil {
		t.Error("expected error for trailing dot")
	}
}

func TestParseErrorInvalidChar(t *testing.T) {
	_, err := parsePath("$.foo!")
	if err == nil {
		t.Error("expected error for invalid character")
	}
}

func TestParsePathTooLong(t *testing.T) {
	long := "$." + string(make([]byte, MaxPathLength))
	_, err := parsePath(long)
	if err == nil {
		t.Error("expected error for path too long")
	}
}
