# shaker

Zero-dependency Go package for pruning JSON documents via JSONPath queries.

Inspired by GraphQL's field selection model: specify what you want (include mode) or what to remove (exclude mode). The package provides the engine; your application provides the policy.

```
go get github.com/mibar/shaker
```

Requires Go 1.21+.

---

## Why

APIs return more data than clients need. GraphQL solves this with field selection, but REST APIs don't have a standard mechanism. `shaker` brings field selection to any JSON payload — HTTP responses, MCP tool results, message queue payloads, config files — without coupling to a transport or framework.

The package is deliberately unopinionated: it takes `[]byte` in, returns `[]byte` out. How you expose the pruning query to your clients (query parameter, request body field, `_meta`, header) is your decision.

---

## Quick Start

```go
package main

import (
    "fmt"
    "log"

    "github.com/mibar/shaker"
)

func main() {
    s := shaker.New()

    input := []byte(`{
        "name": "John",
        "email": "john@example.com",
        "password": "s3cret",
        "age": 30
    }`)

    // Include: keep only what you ask for (everything else removed)
    out, err := s.Shake(input, shaker.Include("$.name", "$.email"))
    if err != nil {
        log.Fatal(err)
    }
    fmt.Println(string(out))
    // {"email":"john@example.com","name":"John"}

    // Exclude: remove what you specify (everything else kept)
    out, err = s.Shake(input, shaker.Exclude("$.password"))
    if err != nil {
        log.Fatal(err)
    }
    fmt.Println(string(out))
    // {"age":30,"email":"john@example.com","name":"John"}
}
```

---

## API

### Direct

```go
s := shaker.New()

// Include mode (GraphQL-like): specify what you want
out, err := s.Shake(json, shaker.Include("$.name", "$.email"))

// Exclude mode: specify what to remove
out, err := s.Shake(json, shaker.Exclude("$.password", "$..secret"))

// MustShake panics on error — useful in tests
out := s.MustShake(json, shaker.Include("$.name"))
```

### WithPrefix

Scope all paths under a common root. Relative paths (starting with `.`) are appended to the prefix. Absolute paths (starting with `$`) are left as-is.

```go
q := shaker.Exclude(".password", ".secret").WithPrefix("$.data")
out, err := s.Shake(json, q)
```

### Pre-compiled Query

Parse and build the path trie once, reuse across multiple documents:

```go
q, err := shaker.Include("$.name", "$.email").Compile()
if err != nil {
    log.Fatal(err)
}

for _, doc := range documents {
    out, _ := s.Shake(doc, q)
    // ...
}
```

### Fluent Builder

Type-safe builder that prevents mixing Include and Exclude at compile time:

```go
// Include
out, err := shaker.New().
    From(json).
    Prefix("$.data").
    Include(".name", ".email").
    Include(".age").                // adds to same include set
    Shake()

// Exclude
out, err := shaker.New().
    From(json).
    Exclude(".password").
    Shake()

// WON'T COMPILE — type system enforces mutual exclusivity:
// shaker.New().From(json).Include(".x").Exclude(".y")
```

### ShakeRequest (wire format)

`ShakeRequest` is a JSON-serializable struct that clients send over the wire. It maps 1:1 to a `Query` via `ToQuery()`. Embed it in your request types — the package handles validation and conversion.

```go
// Deserialize from any transport (HTTP body, MCP _meta, query params, etc.)
req := shaker.ShakeRequest{
    Mode:  "include",                    // "include" or "exclude"
    Paths: []string{"$.name", "$.email"},
}

q, err := req.ToQuery()
if err != nil {
    // invalid mode or empty paths
    return err
}

out, err := shaker.New().Shake(payload, q)
```

`ToQuery()` returns an error if:
- `Mode` is not `"include"` or `"exclude"`
- `Paths` is empty

### Composability

Output of one Shake feeds into the next:

```go
s := shaker.New()
out1, _ := s.Shake(json, shaker.Exclude("$.password"))
out2, _ := s.Shake(out1, shaker.Include("$.name", "$.age"))
```

### Error Handling

All invalid paths are reported in a single error via `errors.Join`. No partial application — if any path is invalid, the entire operation fails:

```go
out, err := s.Shake(json, shaker.Include("$.invalid[", "$[bad", "$.valid"))
// err contains 2 joined ParseErrors; valid paths are NOT applied partially

var pe *shaker.ParseError
if errors.As(err, &pe) {
    fmt.Println(pe.Path, pe.Pos, pe.Message)
}
```

---

## JSONPath Subset

| Feature | Syntax | Example |
|---|---|---|
| Root | `$` (optional) | `$.foo` or `.foo` |
| Name selector | `.name` or `['name']` | `$.users` |
| Index | `[0]`, `[-1]` | `$.items[0]` |
| Wildcard | `.*` or `[*]` | `$.users[*]` |
| Recursive descent | `..` | `$..name` |
| Slice | `[start:end:step]` | `$[0:5]`, `$[::2]` |
| Multi-selector | `[0,1,2]`, `['a','b']` | `$[0,2,4]` |
| Bracket notation | `['key']`, `["key"]` | `$['special-key']` |

Not supported: filters (`?@.price>10`), functions, script expressions.

---

## Integration Patterns

All patterns below use `ShakeRequest` as the wire format. The JSON representation is always:

```json
{
    "mode": "include",
    "paths": ["$.name", "$.email"]
}
```

### Pattern 1: REST API middleware

The client sends a `shake` field in the request body. The server captures the response, applies the shake, and returns the pruned payload.

```go
type APIRequest struct {
    UserID string              `json:"user_id"`
    Shake  *shaker.ShakeRequest `json:"shake,omitempty"`
}

func shakeMiddleware(next http.Handler) http.Handler {
    s := shaker.New()

    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        req := r.Context().Value(requestKey).(*APIRequest)
        if req.Shake == nil {
            next.ServeHTTP(w, r)
            return
        }

        q, err := req.Shake.ToQuery()
        if err != nil {
            http.Error(w, err.Error(), http.StatusBadRequest)
            return
        }

        rec := httptest.NewRecorder()
        next.ServeHTTP(rec, r)

        result, err := s.Shake(rec.Body.Bytes(), q)
        if err != nil {
            http.Error(w, err.Error(), http.StatusBadRequest)
            return
        }

        for k, v := range rec.Header() {
            w.Header()[k] = v
        }
        w.Header().Set("Content-Type", "application/json")
        w.WriteHeader(rec.Code)
        w.Write(result)
    })
}
```

**Request:**
```json
POST /api/users/123
{
    "shake": {
        "mode": "include",
        "paths": ["$.name", "$.email"]
    }
}
```

**Response (before shake):**
```json
{
    "name": "John",
    "email": "john@example.com",
    "password_hash": "$2b$...",
    "internal_id": "uuid-xxx"
}
```

**Response (after shake):**
```json
{
    "name": "John",
    "email": "john@example.com"
}
```

### Pattern 2: MCP server — client-controlled field selection via `_meta`

The [Model Context Protocol](https://modelcontextprotocol.io) uses JSON-RPC 2.0 where `params._meta` is an open map for arbitrary metadata. A client-server pair can pass a `ShakeRequest` inside `_meta` to let clients prune tool results before they're returned.

This is particularly useful for LLM-backed MCP clients where every token counts. A tool that returns 50KB of deployment logs can be pruned to just `name` and `status`.

```go
type CallToolParams struct {
    Meta      *Meta          `json:"_meta,omitempty"`
    Name      string         `json:"name"`
    Arguments map[string]any `json:"arguments,omitempty"`
}

type Meta struct {
    ProgressToken any                  `json:"progressToken,omitempty"`
    Shake         *shaker.ShakeRequest `json:"shake,omitempty"`
}

func handleToolCall(s *shaker.Shaker, params CallToolParams) (json.RawMessage, error) {
    result, err := executeTool(params.Name, params.Arguments)
    if err != nil {
        return nil, err
    }

    if params.Meta != nil && params.Meta.Shake != nil {
        q, err := params.Meta.Shake.ToQuery()
        if err != nil {
            return nil, fmt.Errorf("invalid shake hint: %w", err)
        }
        result, err = s.Shake(result, q)
        if err != nil {
            return nil, err
        }
    }

    return result, nil
}
```

**MCP request** — `tools/call` asking for only `name` and `status`:

```json
{
    "jsonrpc": "2.0",
    "id": 1,
    "method": "tools/call",
    "params": {
        "_meta": {
            "shake": {
                "mode": "include",
                "paths": ["$.name", "$.status"]
            }
        },
        "name": "get_deployment",
        "arguments": { "deployment_id": "deploy-abc" }
    }
}
```

**Tool result (raw):**
```json
{
    "name": "prod-api",
    "status": "running",
    "internal_ip": "10.0.0.42",
    "config": { "replicas": 3, "memory": "2Gi", "secrets": ["DB_PASS"] },
    "logs": "... 50KB of log output ..."
}
```

**Response (after shake):**
```json
{
    "name": "prod-api",
    "status": "running"
}
```

### Pattern 3: MCP server-side policy — composing server + client shaking

The server applies its own policy first (strip sensitive fields), then optionally applies the client's shake hint. Composability is the key: output of one `Shake` feeds into the next.

```go
// Pre-compiled at startup — zero per-request overhead
var serverPolicy shaker.Query

func init() {
    var err error
    serverPolicy, err = shaker.Exclude(
        "$..internal_ip",
        "$..secrets",
        "$..password",
        "$..token",
    ).Compile()
    if err != nil {
        log.Fatal(err)
    }
}

func handleToolCall(s *shaker.Shaker, params CallToolParams) (json.RawMessage, error) {
    result, err := executeTool(params.Name, params.Arguments)
    if err != nil {
        return nil, err
    }

    // 1. Server policy: always strip sensitive fields
    result, err = s.Shake(result, serverPolicy)
    if err != nil {
        return nil, err
    }

    // 2. Client hint: optional further pruning
    if params.Meta != nil && params.Meta.Shake != nil {
        q, err := params.Meta.Shake.ToQuery()
        if err != nil {
            return nil, fmt.Errorf("invalid shake hint: %w", err)
        }
        result, err = s.Shake(result, q)
        if err != nil {
            return nil, err
        }
    }

    return result, nil
}
```

### Pattern 4: Query parameter shorthand

For simple GET endpoints, parse the shake query from URL parameters:

```
GET /api/users/123?shake_mode=include&shake_paths=name,email
```

```go
func parseShakeFromQuery(r *http.Request) (*shaker.ShakeRequest, bool) {
    mode := r.URL.Query().Get("shake_mode")
    if mode == "" {
        return nil, false
    }
    return &shaker.ShakeRequest{
        Mode:  mode,
        Paths: strings.Split(r.URL.Query().Get("shake_paths"), ","),
    }, true
}

// Usage:
// req, ok := parseShakeFromQuery(r)
// if ok {
//     q, err := req.ToQuery()
//     ...
// }
```

---

## Behavior

| Scenario | Result |
|---|---|
| Include, no match | Empty container (`{}` or `[]`) |
| Exclude, no match | Unchanged JSON |
| Invalid path(s) | All errors aggregated, no partial application |
| Invalid JSON input | Returns unmarshal error |
| Nesting > 1000 levels | Returns `DepthError` |

---

## Internals

### Component Map

| File | Package | Responsibility | Key Types |
|------|---------|---------------|-----------|
| `pkg/shaker/shaker.go` | `shaker` | Entry point: unmarshal, walk, marshal | `Shaker` |
| `pkg/shaker/builder.go` | `shaker` | Fluent API with compile-time mode safety | `Builder`, `IncludeBuilder`, `ExcludeBuilder` |
| `pkg/shaker/exports.go` | `shaker` | Re-exports internal types as public API | type aliases, constants |
| `pkg/shaker/doc.go` | `shaker` | Package-level godoc | — |
| `internal/jsonpath/query.go` | `jsonpath` | Query lifecycle: create, prefix, compile, walk | `Query`, `Mode`, `ShakeRequest` |
| `internal/jsonpath/parser.go` | `jsonpath` | Recursive descent JSONPath parser | `parser` |
| `internal/jsonpath/path.go` | `jsonpath` | AST types for parsed paths | `Path`, `Segment` |
| `internal/jsonpath/selector.go` | `jsonpath` | Selector interface + concrete implementations | `Selector`, `NameSelector`, `IndexSelector`, `WildcardSelector`, `SliceSelector` |
| `internal/jsonpath/trie.go` | `jsonpath` | Prefix trie: build from paths, match keys/indices | `trieNode`, `sliceChild` |
| `internal/jsonpath/walker.go` | `jsonpath` | Include/exclude tree walk guided by trie | `walkInclude*`, `walkExclude*` |
| `internal/jsonpath/errors.go` | `jsonpath` | Error types and safety limits | `ParseError`, `DepthError`, `MaxDepth`, `MaxPathLength`, `MaxPathCount` |

### Data Flow

```
User Input
    │
    ▼
rawPaths []string ──► parsePath() ──► []*Path ──► buildTrie() ──► *trieNode
                      (parser.go)     (path.go)   (trie.go)      (compiled)
                                                                      │
                                                                      ▼
[]byte ──► json.Unmarshal ──► any (tree) ──► walkInclude/Exclude(tree, trie)
           (shaker.go)                        (walker.go)
                                                   │
                                                   ▼
                                            pruned any ──► json.Marshal ──► []byte
```

**Lazy compilation**: `Query` stores raw path strings until first `Walk()` call (or explicit `Compile()`). Once compiled, the trie pointer is set and reused.

### Parser

Single-pass recursive descent over each path string. O(P) time and space where P = total characters across all paths. Handles: dot notation (`.name`), bracket notation (`['name']`, `[0]`, `[0:10:2]`, `[*]`), recursive descent (`..`), and union (`[a,b]`).

### Path Trie

Each parsed path is inserted into a shared prefix trie. Segments with multiple selectors (unions like `[a,b]`) branch at that node. Descendant segments (`..`) are stored on a separate `descendant` pointer, decoupling them from direct children. Shared prefixes are merged automatically by the trie structure.

```
Input: ["$.data.users[*].name", "$.data.users[*].email", "$.data.posts[0].title"]

Trie:
  root
   └─ data
       ├─ users
       │   └─ [*]
       │       ├─ name  ✓
       │       └─ email ✓
       └─ posts
           └─ [0]
               └─ title ✓
```

### Compile-time Pre-merge (Finalization)

After `buildTrie()` constructs the prefix trie, `finalize()` walks it recursively and precomputes merged children for every node that has both `names` and `wildcard`. For each named child `k`, the result of `merge(names[k], wildcard)` is stored in a `namesMerged` map on the node.

At runtime, `match(key)` becomes a single map lookup into `namesMerged` — zero heap allocations on the hottest path. Keys not present in `names` fall through to the wildcard directly. The `finalized` flag prevents redundant work on shared nodes.

This is a DP/memoization optimization: the merge result for a given `(named_child, wildcard)` pair is deterministic and can be computed once at compile time instead of on every JSON key lookup.

### Walk (Include/Exclude)

Single-pass traversal of the `encoding/json`-unmarshalled tree. At each object key or array index, the walker queries the trie:

- **Include**: builds a new tree containing only matched subtrees
- **Exclude**: clones the tree, omitting matched subtrees

The trie guides pruning: if `match(key)` returns nil and no descendant is active, that entire subtree is skipped without allocation.

### Descendant (`..`)

When a trie node has a `descendant` pointer, the walker propagates it at every level. At each node, it checks both the direct trie children and the descendant trie, merging results when both match.

### Node Merging

When multiple trie branches match the same key (e.g., wildcard `*` and specific name both match), `mergeNodes()` creates a merged trie node containing children from all matching branches. After finalization, this only occurs for unfinalized ephemeral nodes (e.g., intermediate nodes created during descendant walks) and for index-based matches where runtime array length is needed.

### Complexity

| Operation | Time | Space | Notes |
|-----------|------|-------|-------|
| Parse (per path) | O(L) | O(L) | L = path string length |
| Trie build | O(P) | O(P) | P = total segments across all paths |
| Walk (no descendant) | O(N) | O(N) | N = JSON nodes; output tree allocation |
| Walk (with descendant) | O(N × K) | O(N) | K = descendant match cost per node |
| `match(key)` | O(1) amortized | O(1) | Single map lookup; pre-merged at compile time (0 allocs) |
| `matchIndex(idx)` | O(S) | O(S) | S = number of slice selectors on the node |
| `mergeNodes` | O(C₁ + C₂) | O(C₁ + C₂) | C = children in merged nodes; recursive for overlapping keys |
| `json.Unmarshal` | O(N) | O(N) | Dominated by `encoding/json` |
| `json.Marshal` | O(N) | O(N) | Dominated by `encoding/json` |

### Security Limits

Three constants in `errors.go` bound resource consumption:

| Constant | Value | Purpose |
|----------|-------|---------|
| `MaxDepth` | 1000 | Prevents stack overflow from deeply nested JSON |
| `MaxPathLength` | 10,000 | Prevents parser DoS from pathologically long path strings |
| `MaxPathCount` | 1,000 | Prevents trie explosion from queries with excessive paths |

All three are checked eagerly (before processing), so partial work is never performed on invalid input.

### Design Decisions

**Prefix trie over flat regexp list**: The trie shares prefixes across paths, making multi-path queries sublinear in the number of paths. A flat list would require O(P) comparisons per JSON key. The trie also enables early pruning — if no trie child matches, the entire subtree is skipped.

**`encoding/json` unmarshal (not streaming)**: Trading peak memory for simplicity. A streaming tokenizer would give 3-5x throughput for large payloads, but adds 10x code complexity and makes the walker significantly harder to reason about. The library's value is in the query semantics, not in being a JSON parser.

**Lazy compilation**: Users often construct queries in one place and use them in another. Lazy compilation means `Include("$.name")` is a cheap value copy. Explicit `Compile()` is available when you want to fail fast or ensure thread safety.

**Type-safe builder**: The fluent API (`From().Include().Shake()`) uses separate `IncludeBuilder`/`ExcludeBuilder` types so you can't mix include and exclude in the same query. This is enforced at compile time, not runtime.

**Descendant as separate pointer**: Instead of mixing `..` into the regular children, each trie node has a distinct `descendant` field. This keeps the normal match path (which is the hot path) free from descendant logic, and makes the recursive descent propagation explicit in the walker.

---

## Running the demo

```
go run ./cmd/demo
```

This runs 17 examples covering include, exclude, recursive descent, wildcards, slices, prefix scoping, composability, the fluent builder, pre-compiled queries, bracket notation, and error handling.

---

## License

MIT
