package jsonpath

import (
	"testing"

	"github.com/mibar/tree-shaker/internal/jsonpath/parser"
)

func TestTrieSinglePath(t *testing.T) {
	p, _ := parser.ParsePath("$.name")
	trie := buildTrie([]*parser.Path{p})

	child := trie.match("name")
	if child == nil {
		t.Fatal("expected match for 'name'")
	}
	if !child.accepting {
		t.Error("expected accepting node")
	}

	if trie.match("age") != nil {
		t.Error("should not match 'age'")
	}
}

func TestTrieMultiplePaths(t *testing.T) {
	p1, _ := parser.ParsePath("$.name")
	p2, _ := parser.ParsePath("$.email")
	trie := buildTrie([]*parser.Path{p1, p2})

	if trie.match("name") == nil {
		t.Error("expected match for 'name'")
	}
	if trie.match("email") == nil {
		t.Error("expected match for 'email'")
	}
	if trie.match("phone") != nil {
		t.Error("should not match 'phone'")
	}
}

func TestTrieSharedPrefix(t *testing.T) {
	p1, _ := parser.ParsePath("$.data.name")
	p2, _ := parser.ParsePath("$.data.email")
	trie := buildTrie([]*parser.Path{p1, p2})

	data := trie.match("data")
	if data == nil {
		t.Fatal("expected match for 'data'")
	}
	if data.accepting {
		t.Error("'data' should not be accepting")
	}

	name := data.match("name")
	if name == nil || !name.accepting {
		t.Error("expected accepting 'name' under 'data'")
	}
	email := data.match("email")
	if email == nil || !email.accepting {
		t.Error("expected accepting 'email' under 'data'")
	}
}

func TestTrieWildcard(t *testing.T) {
	p, _ := parser.ParsePath("$.users[*].name")
	trie := buildTrie([]*parser.Path{p})

	users := trie.match("users")
	if users == nil {
		t.Fatal("expected match for 'users'")
	}
	if users.wildcard == nil {
		t.Fatal("expected wildcard child")
	}
	name := users.wildcard.match("name")
	if name == nil || !name.accepting {
		t.Error("expected accepting 'name' under wildcard")
	}
}

func TestTrieIndex(t *testing.T) {
	p, _ := parser.ParsePath("$.items[0].title")
	trie := buildTrie([]*parser.Path{p})

	items := trie.match("items")
	if items == nil {
		t.Fatal("expected match for 'items'")
	}

	child := items.matchIndex(0, 5)
	if child == nil {
		t.Fatal("expected match for index 0")
	}

	title := child.match("title")
	if title == nil || !title.accepting {
		t.Error("expected accepting 'title'")
	}

	if items.matchIndex(1, 5) != nil {
		t.Error("should not match index 1")
	}
}

func TestTrieEpsilon(t *testing.T) {
	p, _ := parser.ParsePath("$..name")
	trie := buildTrie([]*parser.Path{p})

	if trie.epsilon == nil {
		t.Fatal("expected epsilon node")
	}

	name := trie.epsilon.match("name")
	if name == nil || !name.accepting {
		t.Error("expected accepting 'name' in epsilon")
	}
}

func TestTrieMatchIndex(t *testing.T) {
	p, _ := parser.ParsePath("$[0,2,4]")
	trie := buildTrie([]*parser.Path{p})

	if trie.matchIndex(0, 5) == nil {
		t.Error("expected match for 0")
	}
	if trie.matchIndex(2, 5) == nil {
		t.Error("expected match for 2")
	}
	if trie.matchIndex(4, 5) == nil {
		t.Error("expected match for 4")
	}
	if trie.matchIndex(1, 5) != nil {
		t.Error("should not match 1")
	}
}
