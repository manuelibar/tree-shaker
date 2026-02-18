# 🧪 Integration Examples

tree-shaker is transport-agnostic. These guides show common integration patterns.

| Pattern | Description |
|---------|-------------|
| [🖥️ Standalone CLI](standalone-cli.md) | Prune JSON from the terminal, shell scripts, or CI pipelines — no Go code required |
| [🌐 REST Middleware](rest-middleware.md) | Capture HTTP responses and prune fields based on a client-provided shake query |
| [🤖 MCP Integration](mcp-integration.md) | Prune MCP tool results via `_meta.shake` before returning to the LLM |
| [🔗 Composition](composition.md) | Layer server policy (strip secrets) and client hints (narrow fields) |

---

## General Approach

All patterns follow the same shape:

1. **Receive** a shake query from the caller (request body, `_meta`, query parameter, config, etc.)
2. **Execute** the normal operation to get the full result
3. **Apply** `shaker.Shake(result, query)` to prune the output
4. **Return** the pruned result

The `ShakeRequest` type handles JSON deserialisation automatically — embed it in your request struct and the query is built on `Unmarshal`.
