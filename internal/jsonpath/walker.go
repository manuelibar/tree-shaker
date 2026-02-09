package jsonpath

// walkInclude builds a new tree containing only the paths matched by the trie.
func walkInclude(node any, trie *trieNode, depth int) (any, error) {
	if trie == nil {
		return nil, nil
	}
	if depth > MaxDepth {
		return nil, &DepthError{Depth: depth}
	}
	if trie.terminal {
		return node, nil
	}

	switch v := node.(type) {
	case map[string]any:
		return walkIncludeObject(v, trie, depth)
	case []any:
		return walkIncludeArray(v, trie, depth)
	default:
		// Scalar: not at a terminal, so no match
		return nil, nil
	}
}

func walkIncludeObject(obj map[string]any, trie *trieNode, depth int) (any, error) {
	result := make(map[string]any)

	for key, val := range obj {
		childTrie := trie.match(key)

		var descChild *trieNode
		if trie.descendant != nil {
			descChild = trie.descendant.match(key)
		}

		// Walk each branch independently, merge at value level.
		var r1, r2 any
		var err error

		if childTrie != nil {
			r1, err = walkInclude(val, childTrie, depth+1)
			if err != nil {
				return nil, err
			}
		}

		if descChild != nil {
			r2, err = walkInclude(val, descChild, depth+1)
			if err != nil {
				return nil, err
			}
		}

		merged := mergeValues(r1, r2)
		if merged != nil {
			result[key] = merged
		}

		// Propagate descendant to children even if no direct/descendant match
		if trie.descendant != nil && merged == nil {
			descResult, err := walkIncludeDescendant(val, trie.descendant, depth+1)
			if err != nil {
				return nil, err
			}
			if descResult != nil {
				result[key] = descResult
			}
		}
	}

	if len(result) == 0 {
		return nil, nil
	}
	return result, nil
}

func walkIncludeArray(arr []any, trie *trieNode, depth int) (any, error) {
	var result []any
	arrLen := len(arr)

	for idx, val := range arr {
		childTrie := trie.matchIndex(idx, arrLen)

		var descChild *trieNode
		if trie.descendant != nil {
			descChild = trie.descendant.matchIndex(idx, arrLen)
		}

		// Walk each branch independently, merge at value level.
		var r1, r2 any
		var err error

		if childTrie != nil {
			r1, err = walkInclude(val, childTrie, depth+1)
			if err != nil {
				return nil, err
			}
		}

		if descChild != nil {
			r2, err = walkInclude(val, descChild, depth+1)
			if err != nil {
				return nil, err
			}
		}

		merged := mergeValues(r1, r2)
		if merged != nil {
			result = append(result, merged)
		} else if trie.descendant != nil {
			descResult, err := walkIncludeDescendant(val, trie.descendant, depth+1)
			if err != nil {
				return nil, err
			}
			if descResult != nil {
				result = append(result, descResult)
			}
		}
	}

	if len(result) == 0 {
		return nil, nil
	}
	return result, nil
}

// walkIncludeDescendant handles recursive descent for include mode.
// It checks if the descendant trie matches the current node and recurses into children.
func walkIncludeDescendant(node any, descTrie *trieNode, depth int) (any, error) {
	if depth > MaxDepth {
		return nil, &DepthError{Depth: depth}
	}

	switch v := node.(type) {
	case map[string]any:
		result := make(map[string]any)
		for key, val := range v {
			childTrie := descTrie.match(key)
			if childTrie != nil {
				childResult, err := walkInclude(val, childTrie, depth+1)
				if err != nil {
					return nil, err
				}
				if childResult != nil {
					result[key] = childResult
				}
			}
			// Always continue descent
			descResult, err := walkIncludeDescendant(val, descTrie, depth+1)
			if err != nil {
				return nil, err
			}
			if descResult != nil {
				if existing, ok := result[key]; ok {
					result[key] = mergeValues(existing, descResult)
				} else {
					result[key] = descResult
				}
			}
		}
		if len(result) == 0 {
			return nil, nil
		}
		return result, nil

	case []any:
		var result []any
		arrLen := len(v)
		for idx, val := range v {
			childTrie := descTrie.matchIndex(idx, arrLen)
			var childResult any
			var err error
			if childTrie != nil {
				childResult, err = walkInclude(val, childTrie, depth+1)
				if err != nil {
					return nil, err
				}
			}
			descResult, err := walkIncludeDescendant(val, descTrie, depth+1)
			if err != nil {
				return nil, err
			}
			merged := mergeValues(childResult, descResult)
			if merged != nil {
				result = append(result, merged)
			}
		}
		if len(result) == 0 {
			return nil, nil
		}
		return result, nil
	}

	return nil, nil
}

// walkExclude clones the structure, removing paths matched by the trie.
func walkExclude(node any, trie *trieNode, depth int) (any, error) {
	if trie == nil {
		return node, nil
	}
	if depth > MaxDepth {
		return nil, &DepthError{Depth: depth}
	}
	if trie.terminal {
		return nil, nil
	}

	switch v := node.(type) {
	case map[string]any:
		return walkExcludeObject(v, trie, depth)
	case []any:
		return walkExcludeArray(v, trie, depth)
	default:
		return node, nil
	}
}

func walkExcludeObject(obj map[string]any, trie *trieNode, depth int) (any, error) {
	result := make(map[string]any, len(obj))

	for key, val := range obj {
		childTrie := trie.match(key)

		var descChild *trieNode
		if trie.descendant != nil {
			descChild = trie.descendant.match(key)
		}

		// Merge the two trie branches for exclude logic.
		effectiveTrie := mergeNodes(compact(childTrie, descChild))

		if effectiveTrie == nil {
			// Not in trie — check descendant
			if trie.descendant != nil {
				excResult, err := walkExcludeDescendant(val, trie.descendant, depth+1)
				if err != nil {
					return nil, err
				}
				result[key] = excResult
			} else {
				result[key] = val
			}
		} else if effectiveTrie.terminal {
			// Terminal — exclude this key entirely
			continue
		} else {
			// Partial match — recurse deeper
			childResult, err := walkExclude(val, effectiveTrie, depth+1)
			if err != nil {
				return nil, err
			}
			if childResult != nil {
				result[key] = childResult
			}
		}
	}

	return result, nil
}

func walkExcludeArray(arr []any, trie *trieNode, depth int) (any, error) {
	var result []any
	arrLen := len(arr)

	for idx, val := range arr {
		childTrie := trie.matchIndex(idx, arrLen)

		var descChild *trieNode
		if trie.descendant != nil {
			descChild = trie.descendant.matchIndex(idx, arrLen)
		}

		effectiveTrie := mergeNodes(compact(childTrie, descChild))

		if effectiveTrie == nil {
			if trie.descendant != nil {
				excResult, err := walkExcludeDescendant(val, trie.descendant, depth+1)
				if err != nil {
					return nil, err
				}
				result = append(result, excResult)
			} else {
				result = append(result, val)
			}
		} else if effectiveTrie.terminal {
			continue
		} else {
			childResult, err := walkExclude(val, effectiveTrie, depth+1)
			if err != nil {
				return nil, err
			}
			result = append(result, childResult)
		}
	}

	return result, nil
}

// walkExcludeDescendant handles recursive descent for exclude mode.
func walkExcludeDescendant(node any, descTrie *trieNode, depth int) (any, error) {
	if depth > MaxDepth {
		return nil, &DepthError{Depth: depth}
	}

	switch v := node.(type) {
	case map[string]any:
		result := make(map[string]any, len(v))
		for key, val := range v {
			childTrie := descTrie.match(key)
			if childTrie != nil && childTrie.terminal {
				// Exclude
				continue
			}
			if childTrie != nil {
				// Partial match + continue descent
				childResult, err := walkExclude(val, childTrie, depth+1)
				if err != nil {
					return nil, err
				}
				// Also apply descendant exclusion
				descResult, err := walkExcludeDescendant(childResult, descTrie, depth+1)
				if err != nil {
					return nil, err
				}
				if descResult != nil {
					result[key] = descResult
				}
			} else {
				// No match at this level — continue descent
				descResult, err := walkExcludeDescendant(val, descTrie, depth+1)
				if err != nil {
					return nil, err
				}
				result[key] = descResult
			}
		}
		return result, nil

	case []any:
		var result []any
		arrLen := len(v)
		for idx, val := range v {
			childTrie := descTrie.matchIndex(idx, arrLen)
			if childTrie != nil && childTrie.terminal {
				continue
			}
			if childTrie != nil {
				childResult, err := walkExclude(val, childTrie, depth+1)
				if err != nil {
					return nil, err
				}
				descResult, err := walkExcludeDescendant(childResult, descTrie, depth+1)
				if err != nil {
					return nil, err
				}
				result = append(result, descResult)
			} else {
				descResult, err := walkExcludeDescendant(val, descTrie, depth+1)
				if err != nil {
					return nil, err
				}
				result = append(result, descResult)
			}
		}
		return result, nil
	}

	return node, nil
}

// mergeValues merges two values (used when include + descendant both produce results).
func mergeValues(a, b any) any {
	if a == nil {
		return b
	}
	if b == nil {
		return a
	}

	aObj, aIsObj := a.(map[string]any)
	bObj, bIsObj := b.(map[string]any)
	if aIsObj && bIsObj {
		for k, v := range bObj {
			if existing, ok := aObj[k]; ok {
				aObj[k] = mergeValues(existing, v)
			} else {
				aObj[k] = v
			}
		}
		return aObj
	}

	// For non-objects, prefer a (direct match over descendant)
	return a
}

// compact filters nil entries from a list of trie nodes.
// Uses a stack-allocated buffer to avoid heap allocation in the common 2-arg case.
func compact(nodes ...*trieNode) []*trieNode {
	var buf [2]*trieNode
	out := buf[:0]
	for _, n := range nodes {
		if n != nil {
			out = append(out, n)
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}
