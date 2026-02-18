package jsonpath

// trie.go implements a compiled prefix trie for efficient multi-pattern matching
// of JSONPath expressions.
//
// # Automata-Theoretic Model
//
// The trie is modeled as a Nondeterministic Finite Automaton (NFA):
//
//   - Each [trieNode] is a *state* in the automaton.
//   - Each JSON key or index consumed during traversal is an *input symbol*.
//   - Child pointers (names, indexes, wildcard, slices) are *transitions* —
//     they map an input symbol to the next state.
//   - [trieNode.accepting] marks *accepting states* — when reached, the entire
//     subtree below that JSON node is included (or excluded).
//   - [trieNode.epsilon] is an *ε-transition* (epsilon transition) — a transition
//     that does NOT consume an input symbol. It models the JSONPath ".."
//     (recursive descent) operator, which can match at *any* depth.
//
// The automaton is nondeterministic because a single input symbol (JSON key)
// can match multiple transitions simultaneously:
//
//   - A key "name" may match both names["name"] and wildcard (*).
//   - With recursive descent (..), the same key is tested against both the
//     direct children and the ε-transition's children.
//
// When multiple transitions match, [mergeNodes] performs an on-the-fly
// *subset construction* — combining the destination states into a single
// composite state that tracks all active NFA branches simultaneously.
//
// # Pre-computation (finalize)
//
// After the trie is built, [trieNode.finalize] walks it recursively and
// pre-computes the merge of names[k] and wildcard for every named key k.
// The results are cached in [trieNode.namesMerged], turning the hot-path
// match(key) call into a single map lookup with zero heap allocations.
//
// This is a memoization / dynamic programming optimization, NOT a full
// NFA→DFA conversion. The ε-transitions and runtime index matching still
// require on-the-fly merging at walk time.
//
// # Patterns
//
//   - Trie (prefix tree): multiple paths share common prefixes.
//   - NFA with ε-transitions: models recursive descent (..) operator.
//   - Subset construction (partial): mergeNodes() combines overlapping states.
//   - Memoization: finalize() pre-computes merged transitions to avoid
//     hot-path allocations.

import (
	"github.com/mibar/tree-shaker/internal/jsonpath/parser"
)

// trieNode is a state in the NFA-based path automaton.
//
// Each node represents a position in the JSONPath matching process.
// The node's children (names, indexes, wildcard, slices) are transitions
// to the next state, consumed by matching a JSON key or index.
//
// The automaton is nondeterministic: a single JSON key can activate
// multiple transitions (e.g., names["x"] + wildcard), producing a
// composite next-state via [mergeNodes].
//
// Example trie for paths ["$.user.name", "$.user.email", "$.*"]:
//
//	root
//	├── names["user"] ──► node
//	│                     ├── names["name"]  ──► (accepting)
//	│                     └── names["email"] ──► (accepting)
//	└── wildcard ──► (accepting)
//
// When match("user") is called on root, both names["user"] and wildcard
// match. mergeNodes combines them into a composite state that knows about
// both {name, email} children AND the accepting wildcard.
type trieNode struct {
	names     map[string]*trieNode // transitions on exact object keys
	indexes   map[int]*trieNode    // transitions on exact array indices
	wildcard  *trieNode            // transition on any key/index (JSONPath *)
	slices    []*sliceChild        // transitions on array slice ranges
	epsilon   *trieNode            // ε-transition for recursive descent (..)
	accepting bool                 // accepting state: a full path ends here

	// namesMerged is a pre-computed cache: namesMerged[k] = merge(names[k], wildcard).
	// Populated by finalize() when both names and wildcard exist on this node.
	// Turns the hot-path match(key) into a single map lookup (zero allocations).
	namesMerged map[string]*trieNode
	finalized   bool // guards against redundant finalize() calls on shared nodes
}

type sliceChild struct {
	sel  parser.SliceSelector
	node *trieNode
}

func newTrieNode() *trieNode {
	return &trieNode{}
}

// buildTrie compiles multiple parsed JSONPath ASTs into a single prefix trie (NFA).
// Shared prefixes are merged automatically — inserting "$.data.name" and "$.data.email"
// creates one "data" node with two children, not two separate branches.
// After insertion, finalize() pre-computes merged transitions for the hot path.
func buildTrie(paths []*parser.Path) *trieNode {
	root := newTrieNode()
	for _, p := range paths {
		root.insert(p.Segments, 0)
	}
	root.finalize()
	return root
}

// finalize walks the trie in post-order and pre-computes merged transitions.
//
// For every node that has both named children and a wildcard child, finalize()
// creates a namesMerged map where:
//
//	namesMerged[k] = mergeNodes([names[k], wildcard])
//
// This is a memoization optimization: the merge result for a given
// (named_child, wildcard) pair is deterministic and can be computed once
// at compile time instead of on every JSON key lookup at walk time.
//
// After finalize(), match(key) becomes a single map lookup into namesMerged
// with zero heap allocations on the hottest code path.
//
// Note: this is NOT a full NFA→DFA determinization. The ε-transitions
// (recursive descent) and runtime index matching (where array length is
// needed) still require on-the-fly merging in the walker.
func (n *trieNode) finalize() {
	if n.finalized {
		return
	}
	n.finalized = true

	// Recurse into all children first (post-order ensures children are finalized before merge).
	for _, child := range n.names {
		child.finalize()
	}
	for _, child := range n.indexes {
		child.finalize()
	}
	if n.wildcard != nil {
		n.wildcard.finalize()
	}
	for _, sc := range n.slices {
		sc.node.finalize()
	}
	if n.epsilon != nil {
		n.epsilon.finalize()
	}

	// Pre-merge: if both names and wildcard exist, compute namesMerged[k] = merge(names[k], wildcard).
	if len(n.names) > 0 && n.wildcard != nil {
		n.namesMerged = make(map[string]*trieNode, len(n.names))
		for k, named := range n.names {
			n.namesMerged[k] = mergeNodes([]*trieNode{named, n.wildcard})
		}
	}
}

// insert adds a path's segments into the trie starting from segIdx.
//
// For descendant segments (the ".." operator), the segment's selectors are
// inserted into the epsilon sub-trie rather than the direct children. This
// models the ε-transition: the walker will propagate the epsilon sub-trie
// at every depth during traversal, allowing matches at any nesting level.
//
// For regular segments, each selector in the segment creates a branch in
// the trie. Multi-selectors like [a,b] create multiple branches from the
// same parent node, all continuing with the remaining segments.
func (n *trieNode) insert(segments []parser.Segment, segIdx int) {
	if segIdx >= len(segments) {
		n.accepting = true
		return
	}

	seg := segments[segIdx]

	if seg.Descendant {
		// Create or reuse ε-transition node
		if n.epsilon == nil {
			n.epsilon = newTrieNode()
		}
		// Insert the rest of this segment (without the descendant flag) into the ε-transition node
		nonDescSeg := seg.WithoutDescendant()
		remaining := make([]parser.Segment, 0, len(segments)-segIdx)
		remaining = append(remaining, nonDescSeg)
		remaining = append(remaining, segments[segIdx+1:]...)
		n.epsilon.insert(remaining, 0)
		return
	}

	// For each selector in this segment, branch into the trie
	for _, sel := range seg.Selectors {
		child := n.getOrCreateChild(sel)
		child.insert(segments, segIdx+1)
	}
}

func (n *trieNode) getOrCreateChild(sel parser.Selector) *trieNode {
	switch s := sel.(type) {
	case parser.NameSelector:
		if n.names == nil {
			n.names = make(map[string]*trieNode)
		}
		if child, ok := n.names[s.Name]; ok {
			return child
		}
		child := newTrieNode()
		n.names[s.Name] = child
		return child

	case parser.IndexSelector:
		if n.indexes == nil {
			n.indexes = make(map[int]*trieNode)
		}
		if child, ok := n.indexes[s.Index]; ok {
			return child
		}
		child := newTrieNode()
		n.indexes[s.Index] = child
		return child

	case parser.WildcardSelector:
		if n.wildcard == nil {
			n.wildcard = newTrieNode()
		}
		return n.wildcard

	case parser.SliceSelector:
		for _, sc := range n.slices {
			if sliceEqual(sc.sel, s) {
				return sc.node
			}
		}
		child := newTrieNode()
		n.slices = append(n.slices, &sliceChild{sel: s, node: child})
		return child
	}

	// Shouldn't reach here with well-typed selectors
	return newTrieNode()
}

// match finds the next state for a given object key (string input symbol).
//
// Returns nil if no transition matches. When multiple transitions match
// (e.g., names["key"] + wildcard), the destinations are merged via
// [mergeNodes] into a composite state (subset construction).
//
// After finalize(), the common case (names + wildcard both present) is a
// single map lookup into namesMerged — zero allocations.
func (n *trieNode) match(key string) *trieNode {
	// Fast path: pre-merged map computed by finalize() — zero allocations.
	if n.namesMerged != nil {
		if child, ok := n.namesMerged[key]; ok {
			return child
		}
		return n.wildcard // key not in names, wildcard only
	}

	// Unfinalized path: names-only, wildcard-only, or ephemeral merged nodes.
	named := n.names[key] // nil if names is nil or key absent
	if n.wildcard == nil {
		return named
	}
	if named == nil {
		return n.wildcard
	}
	return mergeNodes([]*trieNode{named, n.wildcard})
}

// matchIndex finds the next state for a given array index (integer input symbol).
//
// Unlike match(), this cannot be fully pre-computed by finalize() because
// slice matching depends on the runtime array length (e.g., [-1] resolves
// to a different absolute index depending on array size). Negative index
// resolution and slice evaluation happen here at walk time.
//
// When multiple sources match (exact index, negative index, wildcard, slices),
// all matching destinations are merged via [mergeNodes].
func (n *trieNode) matchIndex(idx, arrLen int) *trieNode {
	// Fast path: no wildcard and no slices — direct index lookup only
	if n.wildcard == nil && len(n.slices) == 0 {
		if n.indexes == nil {
			return nil
		}
		child := n.indexes[idx]
		negIdx := idx - arrLen
		if negIdx != idx {
			if negChild := n.indexes[negIdx]; negChild != nil {
				if child == nil {
					return negChild
				}
				return mergeNodes([]*trieNode{child, negChild})
			}
		}
		return child
	}

	// Slow path: multiple match sources
	var matches []*trieNode

	if n.indexes != nil {
		if child, ok := n.indexes[idx]; ok {
			matches = append(matches, child)
		}
		negIdx := idx - arrLen
		if negIdx != idx {
			if child, ok := n.indexes[negIdx]; ok {
				matches = append(matches, child)
			}
		}
	}

	if n.wildcard != nil {
		matches = append(matches, n.wildcard)
	}

	for _, sc := range n.slices {
		if sc.sel.Matches(idx, arrLen) {
			matches = append(matches, sc.node)
		}
	}

	return mergeNodes(matches)
}

// mergePair merges exactly two states, with fast nil checks.
// Used by the walker to combine a direct trie match with an ε-transition match
// without allocating a slice when one or both sides are nil.
func mergePair(a, b *trieNode) *trieNode {
	if a == nil {
		return b
	}
	if b == nil {
		return a
	}
	return mergeNodes([]*trieNode{a, b})
}

// mergeNodes performs on-the-fly subset construction: it combines multiple
// NFA states into a single composite state that represents "being in all
// of these states simultaneously."
//
// This is the core of the NFA simulation. When a JSON key matches both
// names["x"] and wildcard, the walker can't follow just one — it must
// track both branches. mergeNodes creates a new trieNode whose children
// are the union of all input nodes' children, with overlapping keys
// merged recursively.
//
// Returns nil if nodes is empty. Returns the single node if len == 1
// (avoids unnecessary allocation). For len >= 2, allocates a merged node.
//
// The merged node is ephemeral — it is not part of the compiled trie and
// will be garbage collected after the walk. The exception is namesMerged,
// where merged nodes are cached by finalize() to avoid repeated allocation.
func mergeNodes(nodes []*trieNode) *trieNode {
	switch len(nodes) {
	case 0:
		return nil
	case 1:
		return nodes[0]
	}

	merged := newTrieNode()
	for _, node := range nodes {
		if node.accepting {
			merged.accepting = true
		}
		for k, v := range node.names {
			if merged.names == nil {
				merged.names = make(map[string]*trieNode)
			}
			if existing, ok := merged.names[k]; ok {
				merged.names[k] = mergeNodes([]*trieNode{existing, v})
			} else {
				merged.names[k] = v
			}
		}
		for k, v := range node.indexes {
			if merged.indexes == nil {
				merged.indexes = make(map[int]*trieNode)
			}
			if existing, ok := merged.indexes[k]; ok {
				merged.indexes[k] = mergeNodes([]*trieNode{existing, v})
			} else {
				merged.indexes[k] = v
			}
		}
		if node.wildcard != nil {
			if merged.wildcard != nil {
				merged.wildcard = mergeNodes([]*trieNode{merged.wildcard, node.wildcard})
			} else {
				merged.wildcard = node.wildcard
			}
		}
		merged.slices = append(merged.slices, node.slices...)
		if node.epsilon != nil {
			if merged.epsilon != nil {
				merged.epsilon = mergeNodes([]*trieNode{merged.epsilon, node.epsilon})
			} else {
				merged.epsilon = node.epsilon
			}
		}
	}
	return merged
}

// sliceEqual reports whether two SliceSelectors are equivalent.
func sliceEqual(a, b parser.SliceSelector) bool {
	return intPtrEqual(a.Start, b.Start) && intPtrEqual(a.End, b.End) && intPtrEqual(a.Step, b.Step)
}

func intPtrEqual(a, b *int) bool {
	if a == nil && b == nil {
		return true
	}
	if a == nil || b == nil {
		return false
	}
	return *a == *b
}
