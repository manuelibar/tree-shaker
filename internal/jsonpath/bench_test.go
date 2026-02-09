package jsonpath

import (
	"encoding/json"
	"fmt"
	"testing"
)

// --- Helpers ---

func mustCompileQuery(mode Mode, paths ...string) Query {
	var q Query
	switch mode {
	case ModeInclude:
		q = Include(paths...)
	case ModeExclude:
		q = Exclude(paths...)
	}
	compiled, err := q.Compile()
	if err != nil {
		panic(err)
	}
	return compiled
}

func mustUnmarshal(data []byte) any {
	var v any
	if err := json.Unmarshal(data, &v); err != nil {
		panic(err)
	}
	return v
}

// --- Test Data ---

func flatObject(n int) []byte {
	m := make(map[string]any, n)
	for i := range n {
		m[fmt.Sprintf("field_%d", i)] = fmt.Sprintf("value_%d", i)
	}
	b, _ := json.Marshal(m)
	return b
}

func nestedObject() []byte {
	return []byte(`{
		"user": {
			"name": "Alice",
			"email": "alice@example.com",
			"address": {"city": "Buenos Aires", "country": "AR"},
			"tags": ["admin", "user"],
			"orders": [
				{"id": 1, "total": 100, "items": [{"name": "A", "price": 50}, {"name": "B", "price": 50}]},
				{"id": 2, "total": 200, "items": [{"name": "C", "price": 200}]}
			]
		},
		"meta": {"version": 1}
	}`)
}

func largeArray(n int) []byte {
	items := make([]map[string]any, n)
	for i := range n {
		items[i] = map[string]any{
			"id":    i,
			"name":  fmt.Sprintf("item_%d", i),
			"price": i * 10,
			"tags":  []string{"a", "b"},
		}
	}
	obj := map[string]any{"items": items}
	b, _ := json.Marshal(obj)
	return b
}

func deeplyNested(depth int) []byte {
	inner := map[string]any{"secret": "hidden", "public": "visible"}
	for i := range depth {
		inner = map[string]any{
			fmt.Sprintf("level_%d", depth-i): inner,
			"other": "data",
		}
	}
	b, _ := json.Marshal(inner)
	return b
}

// --- End-to-End Benchmarks (compile + walk) ---

func BenchmarkShakeIncludeFlat(b *testing.B) {
	data := flatObject(20)
	tree := mustUnmarshal(data)
	q := mustCompileQuery(ModeInclude, "$.field_0", "$.field_5", "$.field_10")

	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		q.Walk(tree)
	}
}

func BenchmarkShakeIncludeNested(b *testing.B) {
	data := nestedObject()
	tree := mustUnmarshal(data)
	q := mustCompileQuery(ModeInclude, "$.user.name", "$.user.orders[*].id")

	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		q.Walk(tree)
	}
}

func BenchmarkShakeExcludeFlat(b *testing.B) {
	data := flatObject(20)
	tree := mustUnmarshal(data)
	q := mustCompileQuery(ModeExclude, "$.field_3", "$.field_7")

	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		q.Walk(tree)
	}
}

func BenchmarkShakeExcludeDescendant(b *testing.B) {
	data := deeplyNested(5)
	tree := mustUnmarshal(data)
	q := mustCompileQuery(ModeExclude, "$..secret")

	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		q.Walk(tree)
	}
}

func BenchmarkShakeWildcardLargeArray(b *testing.B) {
	data := largeArray(1000)
	tree := mustUnmarshal(data)
	q := mustCompileQuery(ModeInclude, "$.items[*].name")

	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		q.Walk(tree)
	}
}

func BenchmarkShakeSlice(b *testing.B) {
	data := largeArray(1000)
	tree := mustUnmarshal(data)
	q := mustCompileQuery(ModeInclude, "$.items[0:100]")

	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		q.Walk(tree)
	}
}

func BenchmarkShakePrecompiled(b *testing.B) {
	data := nestedObject()
	tree := mustUnmarshal(data)
	q := mustCompileQuery(ModeInclude, "$.user.name", "$.user.email", "$.user.address.city")

	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		q.Walk(tree)
	}
}

// --- Isolated Benchmarks ---

func BenchmarkCompile(b *testing.B) {
	paths := []string{"$.user.name", "$.user.orders[*].items[0].price", "$..id", "$.meta.version"}

	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		q := Include(paths...)
		q.Compile()
	}
}

func BenchmarkTrieMatch(b *testing.B) {
	p1, _ := parsePath("$.user.name")
	p2, _ := parsePath("$.user.email")
	p3, _ := parsePath("$.*")
	trie := buildTrie([]*Path{p1, p2, p3})

	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		trie.match("user")
		trie.match("other")
		trie.match("name")
	}
}

func BenchmarkTrieMatchPremerged(b *testing.B) {
	// Same setup as BenchmarkTrieMatch but explicitly exercises the pre-merged path.
	// After finalize(), match() on a node with both names and wildcard should do
	// a single map lookup with zero allocations.
	p1, _ := parsePath("$.user.name")
	p2, _ := parsePath("$.user.email")
	p3, _ := parsePath("$.*")
	trie := buildTrie([]*Path{p1, p2, p3})

	// Verify pre-merge was applied.
	if trie.namesMerged == nil {
		b.Fatal("expected namesMerged to be populated after finalize()")
	}

	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		trie.match("user")
		trie.match("other")
		trie.match("name")
	}
}

func BenchmarkTrieMatchIndex(b *testing.B) {
	p1, _ := parsePath("$.items[0].name")
	p2, _ := parsePath("$.items[*].id")
	p3, _ := parsePath("$.items[0:10].price")
	trie := buildTrie([]*Path{p1, p2, p3})

	itemsTrie := trie.match("items")
	if itemsTrie == nil {
		b.Fatal("expected items trie")
	}

	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		itemsTrie.matchIndex(0, 100)
		itemsTrie.matchIndex(5, 100)
		itemsTrie.matchIndex(50, 100)
	}
}
