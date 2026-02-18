# 🧠 Writing a SKILL.md for `shake`

A **SKILL.md** is a plain-Markdown file that teaches an AI coding agent (Claude Code, Cursor, Copilot, etc.) how to use a tool. Drop it in `.claude/`, `.cursor/rules/`, or any context-loading path your agent supports, and the agent gains the skill automatically.

This guide shows how to write one for the `shake` CLI.

---

## Anatomy of a SKILL.md

A good skill file has four sections:

| Section | Purpose |
|---------|---------|
| **What** | One-sentence description of the tool |
| **When** | Situations where the agent should reach for it |
| **How** | Flag reference + invocation variants |
| **Examples** | Copy-pasteable commands covering common patterns |

---

## Example: `SKILL.md` for `shake`

Below is a complete, ready-to-use skill file. Copy it into your agent's context directory.

````markdown
# Skill: tree-shaker CLI (`shake`)

## What

`shake` prunes JSON — keep only the fields you need (include mode) or strip
fields you don't want (exclude mode). Operates on files or stdin/stdout.

## When to use

- Extracting a subset of fields from a large JSON payload
- Stripping sensitive fields (passwords, tokens, secrets) before logging or forwarding
- Narrowing API responses in CI pipelines or shell scripts
- Pre-processing JSON fixtures for tests

## Invocation

```
shake -paths <JSONPath,...> [-mode include|exclude]
      [-file input.json] [-output result.json]
      [-max-depth N] [-max-path-length N] [-max-path-count N]
```

Or via the project helper:

```
./run shake <same flags>
```

### Flags

| Flag               | Default       | Description                                          |
|--------------------|---------------|------------------------------------------------------|
| `-paths`           | *(required)*  | Comma-separated JSONPath expressions                 |
| `-mode`            | `include`     | `include` keeps matched fields; `exclude` removes them |
| `-file`            | *(stdin)*     | Path to input JSON file                              |
| `-output`          | *(stdout)*    | Path to output JSON file                             |
| `-max-depth`       | `0` (no limit)| Max JSON nesting depth                               |
| `-max-path-length` | `0` (no limit)| Max byte length per JSONPath expression              |
| `-max-path-count`  | `0` (no limit)| Max number of JSONPath expressions                   |

## Examples

### Include — keep specific fields

```bash
shake -file user.json -paths '$.name,$.email'
```

### Exclude — strip sensitive fields

```bash
shake -file response.json -paths '$.password,$..secret,$..token' -mode exclude
```

### File-to-file transformation

```bash
shake -file raw.json -paths '$.data[*].id,$.data[*].title' -output slim.json
```

### Stdin/stdout piping

```bash
curl -s https://api.example.com/users/1 | shake -paths '$.name,$.email'
```

### Wildcards and recursive descent

```bash
# All names inside an array
shake -file data.json -paths '$.users[*].name'

# Field at any depth
shake -file data.json -paths '$..email' -mode exclude
```

### With safety limits (untrusted input)

```bash
shake -file untrusted.json \
  -paths '$.id,$.name' \
  -max-depth 1000 \
  -max-path-length 10000 \
  -max-path-count 1000
```

### Chained with other tools

```bash
# Prune then pretty-print
shake -file data.json -paths '$.name,$.email' | jq .

# Prune Kubernetes output
kubectl get pod my-pod -o json | shake -paths '$.metadata.name,$.status.phase'
```

## Walkthrough with `example.json`

The repo includes `docs/examples/example.json` — a SaaS payload with users, billing,
nested settings, and sensitive fields at multiple depths. Use it to verify commands.

### Extract user names and emails

```bash
./run shake -file docs/examples/example.json -paths '$.users[*].name,$.users[*].email'
```

Output:

```json
{"users":[{"email":"alice@acme.io","name":"Alice Chen"},{"email":"bob@acme.io","name":"Bob Martinez"},{"email":"carol@acme.io","name":"Carol Nakamura"}]}
```

### Strip all secrets (recursive descent)

```bash
./run shake -file docs/examples/example.json \
    -paths '$..password,$..secret_key,$..token,$..internal_trace_id' \
    -mode exclude
```

### Billing summary

```bash
./run shake -file docs/examples/example.json \
    -paths '$.billing.plan,$.billing.seats,$.billing.invoices[*].id,$.billing.invoices[*].status'
```

Output:

```json
{"billing":{"invoices":[{"id":"INV-001","status":"paid"},{"id":"INV-002","status":"paid"},{"id":"INV-003","status":"pending"}],"plan":"enterprise","seats":50}}
```

### Save to file

```bash
./run shake -file docs/examples/example.json \
    -paths '$.users[*].name,$.users[*].role' \
    -output users-slim.json
```
````

---

## Variants

Depending on your agent platform, place the file differently:

| Platform | Path | Notes |
|----------|------|-------|
| **Claude Code** | `.claude/commands/shake.md` | Available as `/shake` slash command |
| **Claude Code** | `CLAUDE.md` (append) | Always in context, no slash command needed |
| **Cursor** | `.cursor/rules/shake.mdc` | Auto-loaded as project rule |
| **Generic** | `SKILL.md` or `docs/SKILL.md` | Referenced manually or via include |

### Claude Code slash command variant

If placed at `.claude/commands/shake.md`, the agent can invoke it with `/shake`. Add a parameter placeholder at the top:

```markdown
# Skill: shake

$ARGUMENTS

(rest of the skill file)
```

The agent will substitute `$ARGUMENTS` with whatever the user types after `/shake`.

### Cursor rules variant

For Cursor, rename to `.cursor/rules/shake.mdc` and prepend a frontmatter block:

```markdown
---
description: "Use the shake CLI to prune JSON files"
globs:
  - "*.json"
alwaysApply: false
---

(rest of the skill file)
```

This activates the rule automatically when working with JSON files.

---

## Tips

- **Be specific about flag names.** Agents follow literal instructions — `"-paths"` is clearer than "pass the paths".
- **Show the exact command.** Agents copy-paste from examples more reliably than they compose from descriptions.
- **Include error patterns.** If `shake` prints a warning about missing limits, mention it so the agent knows to add `-max-depth` etc.
- **Keep it under 200 lines.** Agents have context limits; a focused skill beats an exhaustive manual.

---

<p align="center">
  <a href="composition.md">Next: 🔗 Composition →</a>
</p>
