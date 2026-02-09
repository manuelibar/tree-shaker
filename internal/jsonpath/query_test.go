package jsonpath

import (
	"errors"
	"testing"
)

func TestIncludeQuery(t *testing.T) {
	q := Include("$.name", "$.email")
	if q.mode != ModeInclude {
		t.Error("expected include mode")
	}
	if len(q.rawPaths) != 2 {
		t.Errorf("expected 2 paths, got %d", len(q.rawPaths))
	}
}

func TestExcludeQuery(t *testing.T) {
	q := Exclude("$.password")
	if q.mode != ModeExclude {
		t.Error("expected exclude mode")
	}
}

func TestWithPrefix(t *testing.T) {
	q := Include(".name", ".email").WithPrefix("$.data")
	if q.prefix != "$.data" {
		t.Errorf("expected prefix '$.data', got %q", q.prefix)
	}
	// Compile and verify paths were scoped
	compiled, err := q.Compile()
	if err != nil {
		t.Fatal(err)
	}
	if compiled.compiled == nil {
		t.Error("expected compiled trie")
	}
}

func TestWithPrefixAbsolutePath(t *testing.T) {
	// Absolute paths (starting with $) should not be prefixed
	q := Include("$.name", ".email").WithPrefix("$.data")
	compiled, err := q.Compile()
	if err != nil {
		t.Fatal(err)
	}
	if compiled.compiled == nil {
		t.Error("expected compiled trie")
	}
}

func TestCompileErrors(t *testing.T) {
	q := Include("$.valid", "$.invalid[", "$[bad")
	_, err := q.Compile()
	if err == nil {
		t.Fatal("expected error")
	}

	// Should contain 2 errors joined
	var pe *ParseError
	unwrapped := errors.Unwrap(err)
	// errors.Join wraps, so we check for ParseError in the chain
	if !errors.As(err, &pe) {
		t.Errorf("expected ParseError in chain, got %T: %v", unwrapped, err)
	}
}

func TestCompileIdempotent(t *testing.T) {
	q := Include("$.name")
	q1, err := q.Compile()
	if err != nil {
		t.Fatal(err)
	}
	q2, err := q1.Compile()
	if err != nil {
		t.Fatal(err)
	}
	if q2.compiled != q1.compiled {
		t.Error("double compile should reuse trie")
	}
}

func TestMaxPathCount(t *testing.T) {
	paths := make([]string, MaxPathCount+1)
	for i := range paths {
		paths[i] = "$.name"
	}
	q := Include(paths...)
	_, err := q.Compile()
	if err == nil {
		t.Error("expected error for exceeding MaxPathCount")
	}
}

func TestShakeRequestToQueryInclude(t *testing.T) {
	r := ShakeRequest{
		Mode:  "include",
		Paths: []string{".name", ".email"},
	}
	q, err := r.ToQuery()
	if err != nil {
		t.Fatal(err)
	}
	if q.mode != ModeInclude {
		t.Error("expected include mode")
	}
}

func TestShakeRequestToQueryExclude(t *testing.T) {
	r := ShakeRequest{
		Mode:  "exclude",
		Paths: []string{".password"},
	}
	q, err := r.ToQuery()
	if err != nil {
		t.Fatal(err)
	}
	if q.mode != ModeExclude {
		t.Error("expected exclude mode")
	}
}


func TestShakeRequestToQueryInvalidMode(t *testing.T) {
	r := ShakeRequest{Mode: "invalid", Paths: []string{".name"}}
	_, err := r.ToQuery()
	if err == nil {
		t.Error("expected error for invalid mode")
	}
}

func TestShakeRequestToQueryEmptyPaths(t *testing.T) {
	r := ShakeRequest{Mode: "include", Paths: nil}
	_, err := r.ToQuery()
	if err == nil {
		t.Error("expected error for empty paths")
	}
}

func TestShakeRequestToQueryEmptyMode(t *testing.T) {
	r := ShakeRequest{Mode: "", Paths: []string{".name"}}
	_, err := r.ToQuery()
	if err == nil {
		t.Error("expected error for empty mode")
	}
}
