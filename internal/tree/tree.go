package tree

import (
	"fmt"
	"iter"

	"github.com/mibar/tree-shaker/internal/queue"
	"github.com/mibar/tree-shaker/internal/set"
)

// Edge represents a directed parent-to-child relationship.
type Edge struct{ From, To string }

// Levels maps depth (0 = root) to the nodes at that depth.
type Levels[T any] map[int][]Node[T]

// Tree is a single-parent tree with O(1) node lookup by ID.
type Tree[T any] interface {
	Root() Node[T]
	Attach(child Node[T], parent ...Node[T])
	Remove(n Node[T])
	Replace(n, replacement Node[T])
	Get(id string) Node[T]
	Nodes() []Node[T]
	DFS() iter.Seq[Node[T]]
	BFS() iter.Seq[Node[T]]
	Levels() Levels[T]
	Find(predicate func(Node[T]) bool) Node[T]
	Filter(keep func(Node[T]) bool) []Node[T]
	Leaves() []Node[T]
	Edges() []Edge
	IsCyclic() bool
}

// New creates a Tree rooted at the given node.
func New[T any](root Node[T]) Tree[T] {
	if root == nil {
		panic("cannot create a tree with a nil root")
	}
	t := &tree[T]{
		root:  root,
		nodes: make(map[string]Node[T]),
	}
	t.nodes[root.ID()] = root
	return t
}

type tree[T any] struct {
	root  Node[T]
	nodes map[string]Node[T]
}

func (t *tree[T]) Root() Node[T] { return t.root }

// Attach adds child to the tree under parent (defaults to root).
// Panics if more than one parent is provided (single-parent tree).
// If the child already has a parent, it is detached first (re-parent).
func (t *tree[T]) Attach(child Node[T], parent ...Node[T]) {
	if child == nil {
		panic("cannot attach a nil node")
	}
	if len(parent) > 1 {
		panic("single-parent tree: at most one parent allowed")
	}

	p := t.root
	if len(parent) == 1 {
		if parent[0] == nil {
			panic("cannot attach to a nil parent")
		}
		if _, ok := t.nodes[parent[0].ID()]; !ok {
			panic(fmt.Sprintf("parent %q does not exist in the tree", parent[0].ID()))
		}
		p = parent[0]
	}

	// Re-parent: detach from current parent if any.
	if child.Parent() != nil {
		child.Parent().RemoveChild(child)
	}

	child.SetParent(p)
	p.AddChild(child)
	t.nodes[child.ID()] = child
}

func (t *tree[T]) Remove(n Node[T]) {
	if n == nil {
		panic("cannot remove a nil node")
	}
	if n == t.root {
		panic("cannot remove the root node")
	}
	if _, ok := t.nodes[n.ID()]; !ok {
		panic(fmt.Sprintf("node %q does not exist in the tree", n.ID()))
	}

	q := queue.New[Node[T]]()
	q.Enqueue(n)

	for !q.IsEmpty() {
		cur, ok := q.Dequeue()
		if !ok {
			break
		}
		if _, exists := t.nodes[cur.ID()]; !exists {
			continue
		}

		// Detach from parent.
		if p := cur.Parent(); p != nil {
			p.RemoveChild(cur)
		}

		// Queue all children for cascade removal.
		for _, ch := range cur.Children() {
			q.Enqueue(ch)
		}

		cur.SetParent(nil)
		delete(t.nodes, cur.ID())
	}
}

// Replace swaps n with replacement, preserving parent and children links.
func (t *tree[T]) Replace(n, replacement Node[T]) {
	if n == nil || replacement == nil {
		panic("cannot replace with a nil node")
	}
	if _, ok := t.nodes[n.ID()]; !ok {
		panic(fmt.Sprintf("node %q does not exist in the tree", n.ID()))
	}
	if n == replacement {
		panic("cannot replace a node with itself")
	}

	// Redirect parent link.
	if p := n.Parent(); p != nil {
		p.RemoveChild(n)
		p.AddChild(replacement)
		replacement.SetParent(p)
	}

	// Transfer children.
	for _, ch := range n.Children() {
		ch.SetParent(replacement)
		replacement.AddChild(ch)
	}

	// Handle root replacement.
	if t.root == n {
		t.root = replacement
		replacement.SetParent(nil)
	}

	n.SetParent(nil)
	delete(t.nodes, n.ID())
	t.nodes[replacement.ID()] = replacement
}

func (t *tree[T]) Get(id string) Node[T] {
	return t.nodes[id]
}

func (t *tree[T]) Nodes() []Node[T] {
	out := make([]Node[T], 0, len(t.nodes))
	for _, n := range t.nodes {
		out = append(out, n)
	}
	return out
}

func (t *tree[T]) DFS() iter.Seq[Node[T]] {
	return func(yield func(Node[T]) bool) {
		if t.root == nil {
			return
		}
		stack := []Node[T]{t.root}
		visited := set.New[string]()

		for len(stack) > 0 {
			n := stack[len(stack)-1]
			stack = stack[:len(stack)-1]

			if visited.Has(n.ID()) {
				continue
			}
			visited.Add(n.ID())

			if !yield(n) {
				return
			}

			children := n.Children()
			for i := len(children) - 1; i >= 0; i-- {
				if !visited.Has(children[i].ID()) {
					stack = append(stack, children[i])
				}
			}
		}
	}
}

func (t *tree[T]) BFS() iter.Seq[Node[T]] {
	return func(yield func(Node[T]) bool) {
		if t.root == nil {
			return
		}
		q := queue.New[Node[T]]()
		q.Enqueue(t.root)
		visited := set.New[string]()

		for !q.IsEmpty() {
			n, ok := q.Dequeue()
			if !ok {
				break
			}
			if visited.Has(n.ID()) {
				continue
			}
			visited.Add(n.ID())

			if !yield(n) {
				return
			}

			for _, ch := range n.Children() {
				if !visited.Has(ch.ID()) {
					q.Enqueue(ch)
				}
			}
		}
	}
}

func (t *tree[T]) Levels() Levels[T] {
	levels := make(Levels[T])
	if t.root == nil {
		return levels
	}

	type item struct {
		node  Node[T]
		depth int
	}

	q := queue.New[item]()
	q.Enqueue(item{node: t.root, depth: 0})
	visited := set.New[string]()

	for !q.IsEmpty() {
		it, ok := q.Dequeue()
		if !ok {
			break
		}
		if visited.Has(it.node.ID()) {
			continue
		}
		visited.Add(it.node.ID())

		levels[it.depth] = append(levels[it.depth], it.node)

		for _, ch := range it.node.Children() {
			if !visited.Has(ch.ID()) {
				q.Enqueue(item{node: ch, depth: it.depth + 1})
			}
		}
	}
	return levels
}

func (t *tree[T]) Find(predicate func(Node[T]) bool) Node[T] {
	for n := range t.DFS() {
		if predicate(n) {
			return n
		}
	}
	return nil
}

func (t *tree[T]) Filter(keep func(Node[T]) bool) []Node[T] {
	var result []Node[T]
	for n := range t.DFS() {
		if keep(n) {
			result = append(result, n)
		}
	}
	return result
}

func (t *tree[T]) Leaves() []Node[T] {
	return t.Filter(func(n Node[T]) bool { return n.IsLeaf() })
}

func (t *tree[T]) Edges() []Edge {
	var edges []Edge
	for n := range t.BFS() {
		for _, ch := range n.Children() {
			edges = append(edges, Edge{From: n.ID(), To: ch.ID()})
		}
	}
	return edges
}

func (t *tree[T]) IsCyclic() bool {
	if t.root == nil {
		return false
	}
	visited := set.New[string]()
	recStack := set.New[string]()
	return detectCycle(t.root, visited, recStack)
}

func detectCycle[T any](n Node[T], visited, recStack set.Set[string]) bool {
	visited.Add(n.ID())
	recStack.Add(n.ID())

	for _, ch := range n.Children() {
		if !visited.Has(ch.ID()) {
			if detectCycle(ch, visited, recStack) {
				return true
			}
		} else if recStack.Has(ch.ID()) {
			return true
		}
	}

	recStack.Remove(n.ID())
	return false
}
