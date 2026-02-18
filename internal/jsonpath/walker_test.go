package jsonpath

import (
	"encoding/json"
	"testing"

	"github.com/mibar/tree-shaker/internal/jsonpath/parser"
)

func mustParse(t *testing.T, paths ...string) *trieNode {
	t.Helper()
	var parsed []*parser.Path
	for _, raw := range paths {
		p, err := parser.ParsePath(raw)
		if err != nil {
			t.Fatalf("ParsePath(%q): %v", raw, err)
		}
		parsed = append(parsed, p)
	}
	return buildTrie(parsed)
}

func unmarshal(t *testing.T, data string) any {
	t.Helper()
	var v any
	if err := json.Unmarshal([]byte(data), &v); err != nil {
		t.Fatalf("json.Unmarshal: %v", err)
	}
	return v
}

func toJSON(t *testing.T, v any) string {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("json.Marshal: %v", err)
	}
	return string(b)
}

func walkInclude(tree any, trie *trieNode, depth int) (any, error) {
	return (walker{include: true}).walk(tree, trie, depth)
}

func walkExclude(tree any, trie *trieNode, depth int) (any, error) {
	return (walker{include: false}).walk(tree, trie, depth)
}

func TestWalkIncludeSingleField(t *testing.T) {
	tree := unmarshal(t, `{"name":"John","age":30,"email":"john@example.com"}`)
	trie := mustParse(t, "$.name")

	result, err := walkInclude(tree, trie, 0)
	if err != nil {
		t.Fatal(err)
	}
	got := toJSON(t, result)
	if got != `{"name":"John"}` {
		t.Errorf("got %s", got)
	}
}

func TestWalkIncludeMultipleFields(t *testing.T) {
	tree := unmarshal(t, `{"name":"John","age":30,"email":"john@example.com"}`)
	trie := mustParse(t, "$.name", "$.email")

	result, err := walkInclude(tree, trie, 0)
	if err != nil {
		t.Fatal(err)
	}
	obj := result.(map[string]any)
	if obj["name"] != "John" || obj["email"] != "john@example.com" {
		t.Errorf("got %v", obj)
	}
	if _, ok := obj["age"]; ok {
		t.Error("age should be excluded")
	}
}

func TestWalkIncludeNested(t *testing.T) {
	tree := unmarshal(t, `{"data":{"name":"John","age":30},"meta":"ignored"}`)
	trie := mustParse(t, "$.data.name")

	result, err := walkInclude(tree, trie, 0)
	if err != nil {
		t.Fatal(err)
	}
	got := toJSON(t, result)
	if got != `{"data":{"name":"John"}}` {
		t.Errorf("got %s", got)
	}
}

func TestWalkIncludeWildcard(t *testing.T) {
	tree := unmarshal(t, `{"users":[{"name":"A","age":1},{"name":"B","age":2}]}`)
	trie := mustParse(t, "$.users[*].name")

	result, err := walkInclude(tree, trie, 0)
	if err != nil {
		t.Fatal(err)
	}
	got := toJSON(t, result)
	if got != `{"users":[{"name":"A"},{"name":"B"}]}` {
		t.Errorf("got %s", got)
	}
}

func TestWalkIncludeIndex(t *testing.T) {
	tree := unmarshal(t, `[10,20,30,40,50]`)
	trie := mustParse(t, "$[0]", "$[2]")

	result, err := walkInclude(tree, trie, 0)
	if err != nil {
		t.Fatal(err)
	}
	got := toJSON(t, result)
	if got != `[10,30]` {
		t.Errorf("got %s", got)
	}
}

func TestWalkIncludeRecursiveDescent(t *testing.T) {
	tree := unmarshal(t, `{"a":{"name":"A","b":{"name":"B","c":{"name":"C"}}}}`)
	trie := mustParse(t, "$..name")

	result, err := walkInclude(tree, trie, 0)
	if err != nil {
		t.Fatal(err)
	}
	obj := result.(map[string]any)
	a := obj["a"].(map[string]any)
	if a["name"] != "A" {
		t.Error("expected A's name")
	}
	b := a["b"].(map[string]any)
	if b["name"] != "B" {
		t.Error("expected B's name")
	}
	c := b["c"].(map[string]any)
	if c["name"] != "C" {
		t.Error("expected C's name")
	}
}

func TestWalkIncludeSlice(t *testing.T) {
	tree := unmarshal(t, `[0,1,2,3,4,5]`)
	trie := mustParse(t, "$[0:3]")

	result, err := walkInclude(tree, trie, 0)
	if err != nil {
		t.Fatal(err)
	}
	got := toJSON(t, result)
	if got != `[0,1,2]` {
		t.Errorf("got %s", got)
	}
}

func TestWalkIncludeNoMatch(t *testing.T) {
	tree := unmarshal(t, `{"name":"John"}`)
	trie := mustParse(t, "$.nonexistent")

	result, err := walkInclude(tree, trie, 0)
	if err != nil {
		t.Fatal(err)
	}
	if result != nil {
		t.Errorf("expected nil, got %v", result)
	}
}

func TestWalkExcludeSingleField(t *testing.T) {
	tree := unmarshal(t, `{"name":"John","password":"secret","email":"john@example.com"}`)
	trie := mustParse(t, "$.password")

	result, err := walkExclude(tree, trie, 0)
	if err != nil {
		t.Fatal(err)
	}
	obj := result.(map[string]any)
	if _, ok := obj["password"]; ok {
		t.Error("password should be excluded")
	}
	if obj["name"] != "John" || obj["email"] != "john@example.com" {
		t.Error("other fields should be kept")
	}
}

func TestWalkExcludeNested(t *testing.T) {
	tree := unmarshal(t, `{"data":{"name":"John","secret":"xxx"},"meta":"kept"}`)
	trie := mustParse(t, "$.data.secret")

	result, err := walkExclude(tree, trie, 0)
	if err != nil {
		t.Fatal(err)
	}
	obj := result.(map[string]any)
	data := obj["data"].(map[string]any)
	if _, ok := data["secret"]; ok {
		t.Error("secret should be excluded")
	}
	if data["name"] != "John" {
		t.Error("name should be kept")
	}
	if obj["meta"] != "kept" {
		t.Error("meta should be kept")
	}
}

func TestWalkExcludeRecursiveDescent(t *testing.T) {
	tree := unmarshal(t, `{"a":{"secret":"x","b":{"secret":"y","name":"B"}}}`)
	trie := mustParse(t, "$..secret")

	result, err := walkExclude(tree, trie, 0)
	if err != nil {
		t.Fatal(err)
	}
	obj := result.(map[string]any)
	a := obj["a"].(map[string]any)
	if _, ok := a["secret"]; ok {
		t.Error("a.secret should be excluded")
	}
	b := a["b"].(map[string]any)
	if _, ok := b["secret"]; ok {
		t.Error("b.secret should be excluded")
	}
	if b["name"] != "B" {
		t.Error("b.name should be kept")
	}
}

func TestWalkExcludeNoMatch(t *testing.T) {
	tree := unmarshal(t, `{"name":"John"}`)
	trie := mustParse(t, "$.nonexistent")

	result, err := walkExclude(tree, trie, 0)
	if err != nil {
		t.Fatal(err)
	}
	got := toJSON(t, result)
	if got != `{"name":"John"}` {
		t.Errorf("got %s", got)
	}
}

func TestWalkExcludeWildcard(t *testing.T) {
	tree := unmarshal(t, `{"users":[{"name":"A","age":1},{"name":"B","age":2}]}`)
	trie := mustParse(t, "$.users[*].age")

	result, err := walkExclude(tree, trie, 0)
	if err != nil {
		t.Fatal(err)
	}
	got := toJSON(t, result)
	if got != `{"users":[{"name":"A"},{"name":"B"}]}` {
		t.Errorf("got %s", got)
	}
}

func TestWalkDepthLimit(t *testing.T) {
	const limit = 50
	inner := map[string]any{"leaf": true}
	for i := 0; i < limit+10; i++ {
		inner = map[string]any{"nested": inner}
	}

	trie := mustParse(t, "$..leaf")
	w := walker{include: true, maxDepth: limit}
	_, err := w.walk(inner, trie, 0)
	if err == nil {
		t.Error("expected DepthError")
	}
	de, ok := err.(*DepthError)
	if !ok {
		t.Fatalf("expected *DepthError, got %T", err)
	}
	if de.MaxDepth != limit {
		t.Errorf("DepthError.MaxDepth = %d, want %d", de.MaxDepth, limit)
	}
}

func TestWalkNoDepthLimitByDefault(t *testing.T) {
	// Build a structure deeper than the MaxDepth constant — should succeed
	// because the default is unrestricted.
	inner := map[string]any{"leaf": true}
	for i := 0; i < MaxDepth+10; i++ {
		inner = map[string]any{"nested": inner}
	}

	trie := mustParse(t, "$..leaf")
	result, err := walkInclude(inner, trie, 0)
	if err != nil {
		t.Fatalf("unexpected error with unrestricted depth: %v", err)
	}
	if result == nil {
		t.Error("expected non-nil result")
	}
}

// TestWalkIncludeDescendantWithDirectMatch verifies that ε-closure propagation
// is NOT dropped when a direct match also exists. This was a bug where
// the condition `&& merged == nil` prevented ε-closure search when a key
// already had a direct trie match.
func TestWalkIncludeDescendantWithDirectMatch(t *testing.T) {
	tree := unmarshal(t, `{"a":{"x":1,"name":"A","b":{"name":"B"}}}`)
	trie := mustParse(t, "$.a.x", "$..name")

	result, err := walkInclude(tree, trie, 0)
	if err != nil {
		t.Fatal(err)
	}
	obj := result.(map[string]any)
	a := obj["a"].(map[string]any)
	if a["x"] != float64(1) {
		t.Error("expected a.x = 1")
	}
	if a["name"] != "A" {
		t.Error("expected a.name = A (from ε-closure)")
	}
	b := a["b"].(map[string]any)
	if b["name"] != "B" {
		t.Error("expected a.b.name = B (from ε-closure)")
	}
}

// TestWalkExcludeDescendantWithDirectMatch verifies that ε-closure exclusion
// is applied even when a direct match also exists for a key.
func TestWalkExcludeDescendantWithDirectMatch(t *testing.T) {
	tree := unmarshal(t, `{"a":{"x":1,"secret":"hidden","b":{"secret":"also_hidden","name":"B"}}}`)
	trie := mustParse(t, "$.a.x", "$..secret")

	result, err := walkExclude(tree, trie, 0)
	if err != nil {
		t.Fatal(err)
	}
	obj := result.(map[string]any)
	a := obj["a"].(map[string]any)
	if _, ok := a["x"]; ok {
		t.Error("a.x should be excluded by $.a.x")
	}
	if _, ok := a["secret"]; ok {
		t.Error("a.secret should be excluded by $..secret")
	}
	b := a["b"].(map[string]any)
	if _, ok := b["secret"]; ok {
		t.Error("a.b.secret should be excluded by $..secret")
	}
	if b["name"] != "B" {
		t.Error("a.b.name should be kept")
	}
}

func TestDepthErrorMessage(t *testing.T) {
	err := &DepthError{Depth: 1001, MaxDepth: MaxDepth}
	got := err.Error()
	if got == "" {
		t.Error("expected non-empty error message")
	}
}

func TestMaxDepthConstant(t *testing.T) {
	if MaxDepth != 1000 {
		t.Errorf("MaxDepth = %d, want 1000", MaxDepth)
	}
}
