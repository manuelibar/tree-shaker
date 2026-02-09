package tree

import "github.com/mibar/tree-shaker/internal/set"

type Node[T any] interface {
	ID() string
	Data() T
	Parent() Node[T]
	SetParent(p Node[T])
	Children() []Node[T]
	AddChild(children ...Node[T])
	RemoveChild(child Node[T])
	HasChild(child Node[T]) bool
	IsRoot() bool
	IsLeaf() bool
}

type node[T any] struct {
	id       string
	data     T
	parent   Node[T]
	children set.Set[Node[T]]
}

func NewNode[T any](id string, data T) Node[T] {
	return &node[T]{
		id:       id,
		data:     data,
		children: set.New[Node[T]](),
	}
}

func (n *node[T]) ID() string                   { return n.id }
func (n *node[T]) Data() T                      { return n.data }
func (n *node[T]) Parent() Node[T]              { return n.parent }
func (n *node[T]) SetParent(p Node[T])          { n.parent = p }
func (n *node[T]) Children() []Node[T]          { return n.children.Values() }
func (n *node[T]) IsRoot() bool                 { return n.parent == nil }
func (n *node[T]) IsLeaf() bool                 { return n.children.Len() == 0 }
func (n *node[T]) AddChild(children ...Node[T]) { n.children.Add(children...) }
func (n *node[T]) RemoveChild(child Node[T])    { n.children.Remove(child) }
func (n *node[T]) HasChild(child Node[T]) bool  { return n.children.Has(child) }
