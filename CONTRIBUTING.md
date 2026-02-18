# Contributing

Thank you for your interest in contributing to **tree-shaker**.  
This guide covers the conventions we follow so every change stays consistent.

---

## 🏗️ Project Structure

```
tree-shaker/
├── cmd/shake/          # CLI entry point
├── pkg/shaker/         # Public API — thin wrappers over internal/
├── internal/jsonpath/  # Core engine (parser, trie, walker)
│   └── parser/         # Recursive descent JSONPath parser
└── docs/               # Concept-oriented documentation
```

- **`pkg/shaker`** is the only import path external consumers should use.
- **`internal/`** is invisible to external code by Go's module system.

---

## 🧑‍💻 Go Style

We follow [Effective Go](https://go.dev/doc/effective_go) and the
[Go Code Review Comments](https://go.dev/wiki/CodeReviewComments) wiki.
When in doubt, match the standard library.

### Doc Comments

Every exported symbol **must** have a doc comment.  
Every doc comment **must** earn its place — if it restates what the name and
signature already tell you, rewrite it or remove the noise.

| ✅ Good | ❌ Bad |
|---|---|
| `// Compile eagerly parses all paths and builds the trie.` | `// Compile compiles the query.` |
| `// Include returns an include-mode [Query] for the given JSONPath expressions.` | `// Include creates a [Query] that keeps only the matched paths.` |
| `// IsAbsolute reports whether the path starts with "$".` | `// IsAbsolute checks if the path is absolute.` |

**Rules of thumb:**

1. **Start with the symbol name** — Go convention: `// Foo does X.`
2. **Say _why_, not _what_** — unless the _what_ is non-obvious.
3. **No section-label comments** — `// Mode constants.` or `// Safety limits.`
   above a `const` block is noise. The code structure speaks for itself.
4. **No file-name references** — `// builder.go provides…` couples the comment
   to an artifact that may be renamed. Describe the concept, not the file.
5. **Bool-returning functions** — use `reports whether`:
   `// IsAbsolute reports whether the path starts with "$".`
6. **Must\* pattern** — `// MustCompile is like [Query.Compile] but panics on error.`
7. **Re-exports need their own comment** — `go doc` shows the local comment,
   not the one from the aliased package. Keep it minimal but non-redundant.
8. **Internal code can be richly commented** — explain design decisions,
   trade-offs, and algorithmic choices. These help future maintainers.

### Inline Comments

- **Explain _why_, not _what_:** `// Fast path: pre-merged map — zero allocations.`
  is useful; `// Resolve match` above `resolveMatch(…)` is noise.
- **Algorithm phase labels are OK** when they separate distinct logical sections
  inside a long function (e.g., `// ε-closure propagation`), but only if the
  label adds meaning the function name doesn't already convey.

### Naming

- Follow [Go naming conventions](https://go.dev/doc/effective_go#names).
- Prefer short, descriptive names. Avoid stuttering
  (`shaker.ShakerQuery` → `shaker.Query`).
- Unexported helpers don't need doc comments unless the logic is non-obvious.

### No Function Duplication for Optional Parameters

Never create two functions that differ only by an extra parameter (e.g.,
`DoThing()` + `DoThingWithLimit(limit int)`). This pattern splits
discoverability, doubles the documentation surface, and couples callers to
a default they may not want.

Instead, use **functional options** — the idiomatic Go pattern for optional
configuration:

| ✅ Good | ❌ Bad |
|---|---|
| `ParsePath(raw, WithMaxLength(n))` | `ParsePath(raw)` + `ParsePathWithLimit(raw, n)` |

```go
// ParseOption configures optional behaviour for ParsePath.
type ParseOption func(*parseConfig)

// WithMaxLength restricts the byte length of the input.
func WithMaxLength(n int) ParseOption {
    return func(c *parseConfig) { c.maxLength = n }
}

func ParsePath(raw string, opts ...ParseOption) (*Path, error) { … }
```

This gives callers a single entry point, keeps the zero-option call clean
(`ParsePath(raw)`), and lets us add future options without breaking any
existing signature.

---

## 📖 Documentation (`.md` files)

Markdown docs live in `docs/` and the project root `README.md`.

### Principles

1. **Concept-oriented, not code-specific.** Describe _roles_ and
   _responsibilities_, not variable names or function signatures. If someone
   renames a function, the docs shouldn't need a patch.
2. **Friendly to change.** Avoid hard-coding line numbers, struct field names,
   or implementation details that may shift.
3. **Narrative tone.** Write for a human scanning for understanding, not a
   compiler parsing for syntax.
4. **Mermaid diagrams** over ASCII art — they render on GitHub and are easier
   to maintain.
5. **Link, don't duplicate.** Cross-reference between docs; don't copy the
   same explanation into multiple files.

### Structure

| File | Purpose |
|---|---|
| `README.md` | User-facing: rationale, quick start, usage examples |
| `docs/architecture.md` | Internal design: pipeline stages, component map |
| `docs/algorithm.md` | Automata theory, trie model, walk semantics |
| `docs/examples/*.md` | Integration patterns (REST, MCP, composition) |
| `CONTRIBUTING.md` | This file — style guide and workflow |

---

## 🧪 Tests & Benchmarks

- **Test files live next to the code they test.** This is idiomatic Go — there
  is no top-level `test/` directory for unit tests or benchmarks. A top-level
  `test/` is reserved for end-to-end integration tests that exercise the
  compiled binary.
- **Benchmarks** go in `*_test.go` files in the same package, typically in a
  dedicated `bench_test.go` for discoverability.
- Prefer **table-driven tests** when testing many inputs against the same logic.
- Use `t.Helper()` in test helpers so failures report the caller's line.
- Benchmarks should call `b.ReportAllocs()` and `b.ResetTimer()` after setup.

```bash
# Run all tests
go test ./...

# Run benchmarks
go test -bench=. -benchmem ./internal/jsonpath/
```

---

## 🔄 Pull Request Checklist

- [ ] `go test ./...` passes.
- [ ] `go vet ./...` reports no issues.
- [ ] New exported symbols have doc comments that add insight.
- [ ] No section-label comments or file-name references in doc comments.
- [ ] No function duplication for optional parameters — use functional options.
- [ ] Markdown docs describe concepts, not code specifics.
- [ ] Benchmarks are included for performance-sensitive changes.

---

## 📝 Commit Messages

Use [Conventional Commits](https://www.conventionalcommits.org/) style:

```
feat: add slice selector support
fix: handle negative indices in matchIndex
docs: add algorithm deep-dive
refactor: rename determinize to finalize
test: add recursive descent edge cases
```

Keep the subject line under 72 characters. Use the body for _why_, not _what_.
