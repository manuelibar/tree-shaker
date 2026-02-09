package tree

import (
	"slices"
	"testing"
)

// helper: build a tree
//
//	    root
//	   /    \
//	  a      b
//	 / \
//	c   d
func buildTree() (Tree[string], Node[string], Node[string], Node[string], Node[string], Node[string]) {
	root := NewNode("root", "root-data")
	t := New[string](root)

	a := NewNode("a", "a-data")
	b := NewNode("b", "b-data")
	c := NewNode("c", "c-data")
	d := NewNode("d", "d-data")

	t.Attach(a)           // under root
	t.Attach(b)           // under root
	t.Attach(c, a)        // under a
	t.Attach(d, a)        // under a

	return t, root, a, b, c, d
}

func TestNewTree(t *testing.T) {
	root := NewNode("r", "data")
	tr := New[string](root)
	if tr.Root() != root {
		t.Fatal("root mismatch")
	}
	if tr.Get("r") != root {
		t.Fatal("root not in nodes map")
	}
}

func TestNewTreeNilPanics(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("expected panic for nil root")
		}
	}()
	New[string](nil)
}

func TestAttachToRoot(t *testing.T) {
	root := NewNode("r", 0)
	tr := New[int](root)
	child := NewNode("c", 1)
	tr.Attach(child) // defaults to root

	if child.Parent() != root {
		t.Fatal("child parent should be root")
	}
	if len(root.Children()) != 1 || root.Children()[0] != child {
		t.Fatal("root should have child")
	}
}

func TestAttachToSpecificParent(t *testing.T) {
	tr, _, a, _, _, _ := buildTree()
	e := NewNode("e", "e-data")
	tr.Attach(e, a)

	if e.Parent() != a {
		t.Fatal("e's parent should be a")
	}
	if tr.Get("e") != e {
		t.Fatal("e should be in the tree")
	}
}

func TestAttachReparent(t *testing.T) {
	tr, root, a, b, _, _ := buildTree()
	// Move b under a (was under root).
	tr.Attach(b, a)

	if b.Parent() != a {
		t.Fatal("b's parent should be a after re-parent")
	}
	// b should no longer be a child of root.
	for _, ch := range root.Children() {
		if ch == b {
			t.Fatal("root should not have b as child after re-parent")
		}
	}
}

func TestAttachMultipleParentsPanics(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("expected panic for >1 parent")
		}
	}()
	tr, _, a, b, _, _ := buildTree()
	n := NewNode("x", "x")
	tr.Attach(n, a, b)
}

func TestRemoveCascade(t *testing.T) {
	tr, _, _, _, _, _ := buildTree()
	// Removing a should cascade-remove c and d.
	a := tr.Get("a")
	tr.Remove(a)

	if tr.Get("a") != nil {
		t.Fatal("a should be removed")
	}
	if tr.Get("c") != nil {
		t.Fatal("c should be cascade-removed")
	}
	if tr.Get("d") != nil {
		t.Fatal("d should be cascade-removed")
	}
	// root and b should remain.
	if tr.Get("root") == nil || tr.Get("b") == nil {
		t.Fatal("root and b should still exist")
	}
}

func TestRemoveRootPanics(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("expected panic for removing root")
		}
	}()
	tr, root, _, _, _, _ := buildTree()
	tr.Remove(root)
}

func TestReplace(t *testing.T) {
	tr, root, a, _, c, d := buildTree()
	replacement := NewNode("a2", "a2-data")
	tr.Replace(a, replacement)

	if tr.Get("a") != nil {
		t.Fatal("old node should be gone")
	}
	if tr.Get("a2") != replacement {
		t.Fatal("replacement should be in tree")
	}
	if replacement.Parent() != root {
		t.Fatal("replacement parent should be root")
	}
	// Children should be transferred.
	children := replacement.Children()
	if len(children) != 2 {
		t.Fatalf("replacement should have 2 children, got %d", len(children))
	}
	ids := []string{children[0].ID(), children[1].ID()}
	slices.Sort(ids)
	if ids[0] != "c" || ids[1] != "d" {
		t.Fatalf("expected children [c,d], got %v", ids)
	}
	if c.Parent() != replacement || d.Parent() != replacement {
		t.Fatal("children should point to replacement as parent")
	}
}

func TestReplaceRoot(t *testing.T) {
	tr, root, a, b, _, _ := buildTree()
	newRoot := NewNode("new-root", "nr")
	tr.Replace(root, newRoot)

	if tr.Root() != newRoot {
		t.Fatal("root should be replaced")
	}
	if newRoot.Parent() != nil {
		t.Fatal("new root should have nil parent")
	}
	// Original root's children should be under new root.
	children := newRoot.Children()
	ids := make([]string, len(children))
	for i, ch := range children {
		ids[i] = ch.ID()
	}
	slices.Sort(ids)
	if !slices.Contains(ids, a.ID()) || !slices.Contains(ids, b.ID()) {
		t.Fatalf("new root should have a and b as children, got %v", ids)
	}
}

func TestDFS(t *testing.T) {
	tr, _, _, _, _, _ := buildTree()
	var ids []string
	for n := range tr.DFS() {
		ids = append(ids, n.ID())
	}
	// DFS from root: root -> a -> c -> d -> b
	want := []string{"root", "a", "c", "d", "b"}
	if !slices.Equal(ids, want) {
		t.Fatalf("DFS order = %v, want %v", ids, want)
	}
}

func TestBFS(t *testing.T) {
	tr, _, _, _, _, _ := buildTree()
	var ids []string
	for n := range tr.BFS() {
		ids = append(ids, n.ID())
	}
	// BFS from root: root -> a,b -> c,d
	want := []string{"root", "a", "b", "c", "d"}
	if !slices.Equal(ids, want) {
		t.Fatalf("BFS order = %v, want %v", ids, want)
	}
}

func TestLevels(t *testing.T) {
	tr, _, _, _, _, _ := buildTree()
	levels := tr.Levels()

	if len(levels[0]) != 1 || levels[0][0].ID() != "root" {
		t.Fatal("level 0 should be [root]")
	}
	l1 := make([]string, len(levels[1]))
	for i, n := range levels[1] {
		l1[i] = n.ID()
	}
	slices.Sort(l1)
	if !slices.Equal(l1, []string{"a", "b"}) {
		t.Fatalf("level 1 = %v, want [a b]", l1)
	}
	l2 := make([]string, len(levels[2]))
	for i, n := range levels[2] {
		l2[i] = n.ID()
	}
	slices.Sort(l2)
	if !slices.Equal(l2, []string{"c", "d"}) {
		t.Fatalf("level 2 = %v, want [c d]", l2)
	}
}

func TestFind(t *testing.T) {
	tr, _, _, _, _, _ := buildTree()
	found := tr.Find(func(n Node[string]) bool { return n.ID() == "c" })
	if found == nil || found.ID() != "c" {
		t.Fatal("Find should return node c")
	}
	notFound := tr.Find(func(n Node[string]) bool { return n.ID() == "z" })
	if notFound != nil {
		t.Fatal("Find should return nil for non-existent")
	}
}

func TestFilter(t *testing.T) {
	tr, _, _, _, _, _ := buildTree()
	leaves := tr.Filter(func(n Node[string]) bool { return n.IsLeaf() })
	ids := make([]string, len(leaves))
	for i, n := range leaves {
		ids[i] = n.ID()
	}
	slices.Sort(ids)
	if !slices.Equal(ids, []string{"b", "c", "d"}) {
		t.Fatalf("filter leaves = %v, want [b c d]", ids)
	}
}

func TestLeaves(t *testing.T) {
	tr, _, _, _, _, _ := buildTree()
	leaves := tr.Leaves()
	ids := make([]string, len(leaves))
	for i, n := range leaves {
		ids[i] = n.ID()
	}
	slices.Sort(ids)
	if !slices.Equal(ids, []string{"b", "c", "d"}) {
		t.Fatalf("Leaves() = %v, want [b c d]", ids)
	}
}

func TestEdges(t *testing.T) {
	tr, _, _, _, _, _ := buildTree()
	edges := tr.Edges()

	type e = Edge
	want := []e{
		{From: "root", To: "a"},
		{From: "root", To: "b"},
		{From: "a", To: "c"},
		{From: "a", To: "d"},
	}

	slices.SortFunc(edges, func(a, b Edge) int {
		if a.From != b.From {
			if a.From < b.From {
				return -1
			}
			return 1
		}
		if a.To < b.To {
			return -1
		}
		if a.To > b.To {
			return 1
		}
		return 0
	})
	slices.SortFunc(want, func(a, b Edge) int {
		if a.From != b.From {
			if a.From < b.From {
				return -1
			}
			return 1
		}
		if a.To < b.To {
			return -1
		}
		if a.To > b.To {
			return 1
		}
		return 0
	})

	if !slices.Equal(edges, want) {
		t.Fatalf("edges = %v, want %v", edges, want)
	}
}

func TestIsCyclicFalse(t *testing.T) {
	tr, _, _, _, _, _ := buildTree()
	if tr.IsCyclic() {
		t.Fatal("well-formed tree should not be cyclic")
	}
}

func TestNodeProperties(t *testing.T) {
	_, root, a, b, c, _ := buildTree()

	if !root.IsRoot() {
		t.Fatal("root should be root")
	}
	if root.IsLeaf() {
		t.Fatal("root should not be leaf")
	}
	if a.IsRoot() {
		t.Fatal("a should not be root")
	}
	if a.IsLeaf() {
		t.Fatal("a should not be leaf (has children)")
	}
	if !b.IsLeaf() {
		t.Fatal("b should be leaf")
	}
	if !c.IsLeaf() {
		t.Fatal("c should be leaf")
	}
	if c.Data() != "c-data" {
		t.Fatalf("c.Data() = %q, want %q", c.Data(), "c-data")
	}
}

func TestNodes(t *testing.T) {
	tr, _, _, _, _, _ := buildTree()
	nodes := tr.Nodes()
	if len(nodes) != 5 {
		t.Fatalf("len(Nodes()) = %d, want 5", len(nodes))
	}
}

func TestGetNonExistent(t *testing.T) {
	tr, _, _, _, _, _ := buildTree()
	if n := tr.Get("nonexistent"); n != nil {
		t.Fatal("Get non-existent should return nil")
	}
}
