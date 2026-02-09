package jsonpath

// trieNode is an internal node in the compiled path trie.
type trieNode struct {
	names      map[string]*trieNode // exact name children
	indices    map[int]*trieNode    // exact index children
	wildcard   *trieNode            // * selector child
	slices     []*sliceChild        // slice selector children
	descendant *trieNode            // .. recursive descent child
	terminal   bool                 // a path ends here (include/exclude entire subtree)

	// Pre-merged children: namesMerged[k] = merge(names[k], wildcard).
	// Populated by finalize() when both names and wildcard are present.
	// Eliminates heap allocations on the hot match() path.
	namesMerged map[string]*trieNode
	finalized   bool
}

type sliceChild struct {
	sel  SliceSelector
	node *trieNode
}

func newTrieNode() *trieNode {
	return &trieNode{}
}

// buildTrie compiles multiple parsed paths into a prefix trie.
func buildTrie(paths []*Path) *trieNode {
	root := newTrieNode()
	for _, p := range paths {
		root.insert(p.Segments, 0)
	}
	root.finalize()
	return root
}

// finalize pre-computes merged children for nodes that have both names and wildcard.
// This is a compile-time DP step: for each named child, the merge with wildcard is
// deterministic and can be memoized, eliminating heap allocations on the hot match() path.
func (n *trieNode) finalize() {
	if n.finalized {
		return
	}
	n.finalized = true

	// Recurse into all children first (post-order ensures children are finalized before merge).
	for _, child := range n.names {
		child.finalize()
	}
	for _, child := range n.indices {
		child.finalize()
	}
	if n.wildcard != nil {
		n.wildcard.finalize()
	}
	for _, sc := range n.slices {
		sc.node.finalize()
	}
	if n.descendant != nil {
		n.descendant.finalize()
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
func (n *trieNode) insert(segments []Segment, segIdx int) {
	if segIdx >= len(segments) {
		n.terminal = true
		return
	}

	seg := segments[segIdx]

	if seg.Descendant {
		// Create or reuse descendant node
		if n.descendant == nil {
			n.descendant = newTrieNode()
		}
		// Insert the rest of this segment (without the descendant flag) into the descendant node
		nonDescSeg := Segment{Selectors: seg.Selectors, Descendant: false}
		remaining := make([]Segment, 0, len(segments)-segIdx)
		remaining = append(remaining, nonDescSeg)
		remaining = append(remaining, segments[segIdx+1:]...)
		n.descendant.insert(remaining, 0)
		return
	}

	// For each selector in this segment, branch into the trie
	for _, sel := range seg.Selectors {
		child := n.getOrCreateChild(sel)
		child.insert(segments, segIdx+1)
	}
}

// getOrCreateChild returns the child node for the given selector, creating it if needed.
func (n *trieNode) getOrCreateChild(sel Selector) *trieNode {
	switch s := sel.(type) {
	case NameSelector:
		if n.names == nil {
			n.names = make(map[string]*trieNode)
		}
		if child, ok := n.names[s.Name]; ok {
			return child
		}
		child := newTrieNode()
		n.names[s.Name] = child
		return child

	case IndexSelector:
		if n.indices == nil {
			n.indices = make(map[int]*trieNode)
		}
		if child, ok := n.indices[s.Index]; ok {
			return child
		}
		child := newTrieNode()
		n.indices[s.Index] = child
		return child

	case WildcardSelector:
		if n.wildcard == nil {
			n.wildcard = newTrieNode()
		}
		return n.wildcard

	case SliceSelector:
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

// match finds the child trie node matching a string key (object property).
// It returns nil if no match. Results from multiple matching branches are merged.
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

// matchIndex finds the child trie node matching an array index.
func (n *trieNode) matchIndex(idx, arrLen int) *trieNode {
	// Fast path: no wildcard and no slices — direct index lookup only
	if n.wildcard == nil && len(n.slices) == 0 {
		if n.indices == nil {
			return nil
		}
		child := n.indices[idx]
		negIdx := idx - arrLen
		if negIdx != idx {
			if negChild := n.indices[negIdx]; negChild != nil {
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

	if n.indices != nil {
		if child, ok := n.indices[idx]; ok {
			matches = append(matches, child)
		}
		negIdx := idx - arrLen
		if negIdx != idx {
			if child, ok := n.indices[negIdx]; ok {
				matches = append(matches, child)
			}
		}
	}

	if n.wildcard != nil {
		matches = append(matches, n.wildcard)
	}

	for _, sc := range n.slices {
		if sc.sel.Match(idx, arrLen) {
			matches = append(matches, sc.node)
		}
	}

	return mergeNodes(matches)
}

// mergeNodes combines multiple trie nodes into a single logical node.
// Returns nil if no matches. Returns the single node if only one match.
// For multiple matches, creates a merged node containing all children.
func mergeNodes(nodes []*trieNode) *trieNode {
	switch len(nodes) {
	case 0:
		return nil
	case 1:
		return nodes[0]
	}

	merged := newTrieNode()
	for _, node := range nodes {
		if node.terminal {
			merged.terminal = true
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
		for k, v := range node.indices {
			if merged.indices == nil {
				merged.indices = make(map[int]*trieNode)
			}
			if existing, ok := merged.indices[k]; ok {
				merged.indices[k] = mergeNodes([]*trieNode{existing, v})
			} else {
				merged.indices[k] = v
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
		if node.descendant != nil {
			if merged.descendant != nil {
				merged.descendant = mergeNodes([]*trieNode{merged.descendant, node.descendant})
			} else {
				merged.descendant = node.descendant
			}
		}
	}
	return merged
}

// sliceEqual reports whether two SliceSelectors are equivalent.
func sliceEqual(a, b SliceSelector) bool {
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
