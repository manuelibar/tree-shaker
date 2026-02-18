# 🖥️ Standalone CLI

Use `shake` as a command-line tool to prune JSON from the terminal, shell scripts, or CI pipelines — no Go code required.

---

## Install

```bash
go install github.com/mibar/tree-shaker/cmd/shake@latest
```

Or build from source:

```bash
go build -o shake ./cmd/shake
```

---

## Usage

```
shake -paths <JSONPath,...> [-mode include|exclude]
      [-max-depth N] [-max-path-length N] [-max-path-count N]
      < input.json
```

`shake` reads JSON from **stdin** and writes the pruned result to **stdout**.

| Flag | Default | Description |
|------|---------|-------------|
| `-paths` | *(required)* | Comma-separated JSONPath expressions |
| `-mode` | `include` | `"include"` keeps only matched fields; `"exclude"` removes them |
| `-max-depth` | `0` (no limit) | Maximum JSON nesting depth (recommended: `1000`) |
| `-max-path-length` | `0` (no limit) | Maximum byte length per JSONPath expression (recommended: `10000`) |
| `-max-path-count` | `0` (no limit) | Maximum number of JSONPath expressions (recommended: `1000`) |

> **⚠️ Safety note:** When no limits are set, `shake` prints a warning to stderr. Always set limits when processing untrusted input to prevent denial-of-service (JSON bombs, stack exhaustion, path flooding).

---

## Examples

### Include mode (default)

Keep only the fields you ask for:

```bash
echo '{"name":"John","email":"john@example.com","password":"s3cret","age":30}' | \
    shake -paths '$.name,$.email'
```

```json
{"email":"john@example.com","name":"John"}
```

### Exclude mode

Remove specific fields, keep everything else:

```bash
echo '{"name":"John","email":"john@example.com","password":"s3cret","age":30}' | \
    shake -mode exclude -paths '$.password'
```

```json
{"age":30,"email":"john@example.com","name":"John"}
```

### Nested fields and wildcards

Extract deeply nested values or use wildcards to match across arrays:

```bash
cat <<'EOF' | shake -paths '$.users[*].name,$.users[*].email'
{
    "users": [
        {"name": "Alice", "email": "alice@example.com", "role": "admin"},
        {"name": "Bob",   "email": "bob@example.com",   "role": "viewer"}
    ],
    "total": 2
}
EOF
```

```json
{"users":[{"email":"alice@example.com","name":"Alice"},{"email":"bob@example.com","name":"Bob"}]}
```

### Recursive descent

Use `..` to match a field at any depth:

```bash
echo '{"a":{"secret":"x"},"b":{"nested":{"secret":"y"}},"name":"safe"}' | \
    shake -mode exclude -paths '$..secret'
```

```json
{"a":{},"b":{"nested":{}},"name":"safe"}
```

### With safety limits

Always apply limits when processing untrusted input:

```bash
curl -s https://api.example.com/data | \
    shake -paths '$.results[*].id,$.results[*].title' \
          -max-depth 1000 \
          -max-path-length 10000 \
          -max-path-count 1000
```

---

## Pipe-Friendly

`shake` composes naturally with other CLI tools:

```bash
# Fetch → prune → pretty-print
curl -s https://api.example.com/users/123 | \
    shake -paths '$.name,$.email' | \
    jq .

# Prune each line of newline-delimited JSON
cat events.ndjson | while IFS= read -r line; do
    echo "$line" | shake -mode exclude -paths '$..internal_id,$..trace'
done

# Use in a CI pipeline to extract deployment status
kubectl get deployment my-app -o json | \
    shake -paths '$.metadata.name,$.status.readyReplicas,$.status.replicas'
```

---

## `./run shake` Helper

The project includes a `./run` script that wraps the CLI with file-based input:

```bash
./run shake -file input.json -paths '$.name,$.email'
./run shake -file input.json -paths '$.password,$..secret' -mode exclude
```

| Flag | Description |
|------|-------------|
| `-file` | Path to the JSON file (required) |
| `-paths` | Comma-separated JSONPath expressions (required) |
| `-mode` | `"include"` (default) or `"exclude"` (optional) |

Under the hood this is equivalent to:

```bash
cat input.json | go run ./cmd/shake -paths '$.name,$.email'
```

---

<p align="center">
  <a href="rest-middleware.md">Next: 🌐 REST Middleware →</a>
</p>
