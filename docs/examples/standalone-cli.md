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
      [-file input.json] [-output result.json]
      [-max-depth N] [-max-path-length N] [-max-path-count N]
      [-pretty]
```

`shake` reads JSON from **`-file`** or **stdin** and writes the pruned result to **`-output`** or **stdout**.

| Flag | Default | Description |
|------|---------|-------------|
| `-paths` | *(required)* | Comma-separated JSONPath expressions |
| `-mode` | `include` | `"include"` keeps only matched fields; `"exclude"` removes them |
| `-file` | *(stdin)* | Path to input JSON file |
| `-output` | *(stdout)* | Path to output JSON file |
| `-max-depth` | `0` (no limit) | Maximum JSON nesting depth (recommended: `1000`) |
| `-max-path-length` | `0` (no limit) | Maximum byte length per JSONPath expression (recommended: `10000`) |
| `-max-path-count` | `0` (no limit) | Maximum number of JSONPath expressions (recommended: `1000`) |
| `-pretty` | `false` | Pretty-print the JSON output with 2-space indentation |

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
    shake -paths '$.name,$.email' -pretty

# Prune each line of newline-delimited JSON
cat events.ndjson | while IFS= read -r line; do
    echo "$line" | shake -mode exclude -paths '$..internal_id,$..trace'
done

# Use in a CI pipeline to extract deployment status
kubectl get deployment my-app -o json | \
    shake -paths '$.metadata.name,$.status.readyReplicas,$.status.replicas'
```

---

## Try It — `example.json` Walkthrough

The repo ships with [`example.json`](example.json), a realistic SaaS payload with nested objects, arrays, and sensitive fields at multiple depths. Every command below uses it.

### 1. Extract company info

```bash
./run shake -file docs/examples/example.json -paths '$.company,$.founded,$.active'
```

```json
{"active":true,"company":"Acme Corp","founded":2019}
```

### 2. List user names and emails

```bash
./run shake -file docs/examples/example.json -paths '$.users[*].name,$.users[*].email'
```

```json
{"users":[{"email":"alice@acme.io","name":"Alice Chen"},{"email":"bob@acme.io","name":"Bob Martinez"},{"email":"carol@acme.io","name":"Carol Nakamura"}]}
```

### 3. Strip all sensitive fields (recursive descent)

Passwords, secret keys, and payment tokens live at different depths — `..` catches them all:

```bash
./run shake -file docs/examples/example.json \
    -paths '$..password,$..secret_key,$..token,$..internal_trace_id' \
    -mode exclude
```

### 4. Extract nested billing summary

```bash
./run shake -file docs/examples/example.json \
    -paths '$.billing.plan,$.billing.seats,$.billing.invoices[*].id,$.billing.invoices[*].status'
```

```json
{"billing":{"invoices":[{"id":"INV-001","status":"paid"},{"id":"INV-002","status":"paid"},{"id":"INV-003","status":"pending"}],"plan":"enterprise","seats":50}}
```

### 5. User profiles without settings

```bash
./run shake -file docs/examples/example.json \
    -paths '$.users[*].profile.settings' \
    -mode exclude
```

### 6. File-to-file — save pruned output

```bash
./run shake -file docs/examples/example.json \
    -paths '$.users[*].name,$.users[*].role' \
    -output /tmp/users-slim.json

cat /tmp/users-slim.json
```

```json
{"users":[{"name":"Alice Chen","role":"admin"},{"name":"Bob Martinez","role":"viewer"},{"name":"Carol Nakamura","role":"editor"}]}
```

### 7. Pretty-print the output

Use `-pretty` to get human-readable, indented JSON — no need to pipe through `jq`:

```bash
./run shake -file docs/examples/example.json -paths '$.headquarters' -pretty
```

```json
{
  "headquarters": {
    "city": "San Francisco",
    "state": "CA",
    "coordinates": {
      "lat": 37.7749,
      "lng": -122.4194
    }
  }
}
```

---

## `./run shake` Helper

The project includes a `./run` script that passes all flags through to `cmd/shake`:

```bash
# File input → stdout
./run shake -file input.json -paths '$.name,$.email'

# File input → file output
./run shake -file input.json -paths '$.name,$.email' -output result.json

# Exclude mode
./run shake -file input.json -paths '$.password,$..secret' -mode exclude

# Stdin still works
./run shake -paths '$.name' < input.json
```

All flags from the [Usage](#usage) table are supported — the wrapper simply runs `go run ./cmd/shake "$@"`.

---

<p align="center">
  <a href="rest-middleware.md">Next: 🌐 REST Middleware →</a>
</p>
