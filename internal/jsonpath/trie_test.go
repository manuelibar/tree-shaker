package jsonpath

import "testing"

func TestTrieSinglePath(t *testing.T) {
	p, _ := parsePath("$.name")
	trie := buildTrie([]*Path{p})

	child := trie.match("name")
	if child == nil {
		t.Fatal("expected match for 'name'")
	}
	if !child.terminal {
		t.Error("expected terminal node")
	}

	if trie.match("age") != nil {
		t.Error("should not match 'age'")
	}
}

func TestTrieMultiplePaths(t *testing.T) {
	p1, _ := parsePath("$.name")
	p2, _ := parsePath("$.email")
	trie := buildTrie([]*Path{p1, p2})

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
	p1, _ := parsePath("$.data.name")
	p2, _ := parsePath("$.data.email")
	trie := buildTrie([]*Path{p1, p2})

	data := trie.match("data")
	if data == nil {
		t.Fatal("expected match for 'data'")
	}
	if data.terminal {
		t.Error("'data' should not be terminal")
	}

	name := data.match("name")
	if name == nil || !name.terminal {
		t.Error("expected terminal 'name' under 'data'")
	}
	email := data.match("email")
	if email == nil || !email.terminal {
		t.Error("expected terminal 'email' under 'data'")
	}
}

func TestTrieWildcard(t *testing.T) {
	p, _ := parsePath("$.users[*].name")
	trie := buildTrie([]*Path{p})

	users := trie.match("users")
	if users == nil {
		t.Fatal("expected match for 'users'")
	}
	if users.wildcard == nil {
		t.Fatal("expected wildcard child")
	}
	name := users.wildcard.match("name")
	if name == nil || !name.terminal {
		t.Error("expected terminal 'name' under wildcard")
	}
}

func TestTrieIndex(t *testing.T) {
	p, _ := parsePath("$.items[0].title")
	trie := buildTrie([]*Path{p})

	items := trie.match("items")
	if items == nil {
		t.Fatal("expected match for 'items'")
	}

	child := items.matchIndex(0, 5)
	if child == nil {
		t.Fatal("expected match for index 0")
	}

	title := child.match("title")
	if title == nil || !title.terminal {
		t.Error("expected terminal 'title'")
	}

	if items.matchIndex(1, 5) != nil {
		t.Error("should not match index 1")
	}
}

func TestTrieDescendant(t *testing.T) {
	p, _ := parsePath("$..name")
	trie := buildTrie([]*Path{p})

	if trie.descendant == nil {
		t.Fatal("expected descendant node")
	}

	name := trie.descendant.match("name")
	if name == nil || !name.terminal {
		t.Error("expected terminal 'name' in descendant")
	}
}

func TestTrieMatchIndex(t *testing.T) {
	p, _ := parsePath("$[0,2,4]")
	trie := buildTrie([]*Path{p})

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
