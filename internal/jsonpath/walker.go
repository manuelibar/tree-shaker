package jsonpath

// walker.go implements a trie-guided tree walker for JSON tree pruning.
//
// # How It Works
//
// The walker traverses a JSON tree (from encoding/json.Unmarshal) and the
// compiled trie simultaneously — in lockstep. "Lockstep" means that for
// every step down into the JSON tree (entering an object value or array
// element), the walker also steps down into the trie (following the
// matching transition). The trie acts as a guide: it tells the walker
// which branches of the JSON tree to keep, skip, or recurse into.
//
// At each JSON node, the walker queries the trie:
//   - If the trie node is nil → no path reaches here (include: skip, exclude: keep).
//   - If the trie node is accepting → a full path ends here (include: keep subtree, exclude: remove subtree).
//   - Otherwise → recurse deeper into both the JSON tree and the trie.
//
// # NFA Simulation at Walk Time
//
// When the trie has an ε-transition (from the ".." operator), the walker
// simulates an NFA: at each JSON node, it is effectively in *two states*
// simultaneously — the direct trie state and the ε-transition state.
// Both are tested against each key, and results are merged.
//
// The ε-transition state is *propagated* to every descendant — this is
// the ε-closure. Unlike direct transitions that are consumed (one key =
// one step), the ε-transition persists across depths, which is what makes
// "$..name" match "name" at ANY nesting level.
//
// # Include vs Exclude
//
// Behavior is selected by a single `include` bool set at construction:
//   - Include mode: builds a new tree containing only matched subtrees.
//     Uses [walkSearchEpsilon] for ε-closure (finds and collects matches).
//   - Exclude mode: clones the tree, omitting matched subtrees.
//     Uses [walkFilterEpsilon] for ε-closure (removes matches from result).
//
// # Patterns
//
//   - Tree Walker / Catamorphism: recursive fold over a JSON tree guided by trie.
//   - NFA simulation: tracks multiple active states via trie + ε-transition.
//   - ε-closure propagation: recursive descent matches at every depth.
//   - Strategy pattern via bool: include/exclude selected once, never changes.

import "fmt"

// ---------------------------------------------------------------------------
// Depth limit
// ---------------------------------------------------------------------------

// MaxDepth is the default maximum recursion depth the walker will traverse.
// Applied by [DefaultLimits]; ignored when [Limits].MaxDepth is nil.
//
// Deeply nested JSON documents can cause stack exhaustion. Always set a depth
// limit when processing untrusted input.
const MaxDepth = 1000

// DepthError is returned when the walker exceeds the configured maximum depth.
type DepthError struct {
	Depth    int
	MaxDepth int
}

func (e *DepthError) Error() string {
	return fmt.Sprintf("maximum JSON depth %d exceeded at depth %d", e.MaxDepth, e.Depth)
}

// ---------------------------------------------------------------------------
// Walker
// ---------------------------------------------------------------------------

// walker traverses a JSON tree guided by a compiled trie, producing a pruned copy.
// The `include` flag selects between include mode (keep only matched paths) and
// exclude mode (remove matched paths, keep everything else).
type walker struct {
	include  bool
	maxDepth int
}

func newWalker(mode Mode, maxDepth int) walker {
	return walker{include: mode == ModeInclude, maxDepth: maxDepth}
}

// walk dispatches to the appropriate handler based on node type.
func (w walker) walk(node any, trie *trieNode, depth int) (any, error) {
	if trie == nil {
		if w.include {
			return nil, nil
		}
		return node, nil
	}
	if w.maxDepth > 0 && depth > w.maxDepth {
		return nil, &DepthError{Depth: depth, MaxDepth: w.maxDepth}
	}
	if trie.accepting {
		if w.include {
			return node, nil
		}
		return nil, nil
	}

	switch v := node.(type) {
	case map[string]any:
		return w.walkObject(v, trie, depth)
	case []any:
		return w.walkArray(v, trie, depth)
	default:
		if w.include {
			return nil, nil
		}
		return node, nil
	}
}

// walkObject iterates over object keys, matches each against the trie inline,
// resolves the result, and applies ε-closure propagation.
func (w walker) walkObject(obj map[string]any, trie *trieNode, depth int) (any, error) {
	eps := trie.epsilon
	var result map[string]any
	if w.include {
		result = make(map[string]any)
	} else {
		result = make(map[string]any, len(obj))
	}

	for key, val := range obj {
		// Inline trie matching — no iterator closure, no trieMatch struct.
		child := trie.match(key)
		if eps != nil {
			child = mergePair(child, eps.match(key))
		}

		r, err := w.resolveMatch(val, child, depth)
		if err != nil {
			return nil, err
		}

		// ε-closure propagation
		if eps != nil {
			if w.include {
				found, ferr := w.walkSearchEpsilon(val, eps, depth+1)
				if ferr != nil {
					return nil, ferr
				}
				r = mergeValues(r, found)
			} else if r != nil {
				r, err = w.walkFilterEpsilon(r, eps, depth+1)
				if err != nil {
					return nil, err
				}
			}
		}

		if r != nil {
			result[key] = r
		}
	}

	if w.include && len(result) == 0 {
		return nil, nil
	}
	return result, nil
}

// walkArray iterates over array elements, matches each index against the trie inline,
// resolves the result, and applies ε-closure propagation.
func (w walker) walkArray(arr []any, trie *trieNode, depth int) (any, error) {
	eps := trie.epsilon
	arrLen := len(arr)
	var result []any
	if !w.include {
		result = make([]any, 0, arrLen)
	}

	for idx, val := range arr {
		child := trie.matchIndex(idx, arrLen)
		if eps != nil {
			child = mergePair(child, eps.matchIndex(idx, arrLen))
		}

		r, err := w.resolveMatch(val, child, depth)
		if err != nil {
			return nil, err
		}

		// ε-closure propagation
		if eps != nil {
			if w.include {
				found, ferr := w.walkSearchEpsilon(val, eps, depth+1)
				if ferr != nil {
					return nil, ferr
				}
				r = mergeValues(r, found)
			} else if r != nil {
				r, err = w.walkFilterEpsilon(r, eps, depth+1)
				if err != nil {
					return nil, err
				}
			}
		}

		if r != nil {
			result = append(result, r)
		}
	}

	if w.include && len(result) == 0 {
		return nil, nil
	}
	return result, nil
}

// resolveMatch determines the value for a key/index based on the effective trie.
func (w walker) resolveMatch(val any, child *trieNode, depth int) (any, error) {
	if child == nil {
		if w.include {
			return nil, nil
		}
		return val, nil
	}
	if child.accepting {
		if w.include {
			return val, nil
		}
		return nil, nil
	}
	return w.walk(val, child, depth+1)
}

// ---------------------------------------------------------------------------
// ε-closure propagation strategies
// ---------------------------------------------------------------------------

// walkSearchEpsilon recursively searches for ε-transition matches in the
// original value and builds a partial result containing only matched paths.
// Used by include mode. Analogous to regex findAll.
func (w walker) walkSearchEpsilon(node any, epsTrie *trieNode, depth int) (any, error) {
	if w.maxDepth > 0 && depth > w.maxDepth {
		return nil, &DepthError{Depth: depth, MaxDepth: w.maxDepth}
	}

	switch v := node.(type) {
	case map[string]any:
		result := make(map[string]any)
		for key, val := range v {
			childTrie := epsTrie.match(key)
			if childTrie != nil {
				childResult, err := w.walk(val, childTrie, depth+1)
				if err != nil {
					return nil, err
				}
				if childResult != nil {
					result[key] = childResult
				}
			}
			// Always continue ε-closure
			epsResult, err := w.walkSearchEpsilon(val, epsTrie, depth+1)
			if err != nil {
				return nil, err
			}
			if epsResult != nil {
				if existing, ok := result[key]; ok {
					result[key] = mergeValues(existing, epsResult)
				} else {
					result[key] = epsResult
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
			childTrie := epsTrie.matchIndex(idx, arrLen)
			var childResult any
			var err error
			if childTrie != nil {
				childResult, err = w.walk(val, childTrie, depth+1)
				if err != nil {
					return nil, err
				}
			}
			epsResult, err := w.walkSearchEpsilon(val, epsTrie, depth+1)
			if err != nil {
				return nil, err
			}
			merged := mergeValues(childResult, epsResult)
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

// walkFilterEpsilon recursively walks an already-filtered result and removes
// any additional matches found by the ε-transition trie at every level.
// Used by exclude mode. Analogous to regex replaceAll.
func (w walker) walkFilterEpsilon(node any, epsTrie *trieNode, depth int) (any, error) {
	if w.maxDepth > 0 && depth > w.maxDepth {
		return nil, &DepthError{Depth: depth, MaxDepth: w.maxDepth}
	}

	switch v := node.(type) {
	case map[string]any:
		result := make(map[string]any, len(v))
		for key, val := range v {
			childTrie := epsTrie.match(key)
			if childTrie != nil && childTrie.accepting {
				continue
			}
			if childTrie != nil {
				childResult, err := w.walk(val, childTrie, depth+1)
				if err != nil {
					return nil, err
				}
				epsResult, err := w.walkFilterEpsilon(childResult, epsTrie, depth+1)
				if err != nil {
					return nil, err
				}
				if epsResult != nil {
					result[key] = epsResult
				}
			} else {
				epsResult, err := w.walkFilterEpsilon(val, epsTrie, depth+1)
				if err != nil {
					return nil, err
				}
				result[key] = epsResult
			}
		}
		return result, nil

	case []any:
		result := make([]any, 0, len(v))
		arrLen := len(v)
		for idx, val := range v {
			childTrie := epsTrie.matchIndex(idx, arrLen)
			if childTrie != nil && childTrie.accepting {
				continue
			}
			if childTrie != nil {
				childResult, err := w.walk(val, childTrie, depth+1)
				if err != nil {
					return nil, err
				}
				epsResult, err := w.walkFilterEpsilon(childResult, epsTrie, depth+1)
				if err != nil {
					return nil, err
				}
				result = append(result, epsResult)
			} else {
				epsResult, err := w.walkFilterEpsilon(val, epsTrie, depth+1)
				if err != nil {
					return nil, err
				}
				result = append(result, epsResult)
			}
		}
		return result, nil
	}

	return node, nil
}

// ---------------------------------------------------------------------------
// Merge utility
// ---------------------------------------------------------------------------

// mergeValues merges two values (used when include + ε-closure both produce results).
// Neither argument is mutated; a new map is returned when both are objects.
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
		merged := make(map[string]any, len(aObj)+len(bObj))
		for k, v := range aObj {
			merged[k] = v
		}
		for k, v := range bObj {
			if existing, ok := merged[k]; ok {
				merged[k] = mergeValues(existing, v)
			} else {
				merged[k] = v
			}
		}
		return merged
	}

	// For non-objects, prefer a (direct match over ε-transition)
	return a
}
