# AgentTrail

AgentTrail exports local agent session logs to `logspine.adapter.v1` JSONL.

It is a scanner and exporter, not an archive. Logspine stores, indexes, dedupes, searches, relates, and builds evidence bundles. AgentTrail only reads local session files and emits portable adapter records.

Supported first-pass sources:

- Codex session JSONL under `~/.codex/sessions`
- Claude project JSONL under `~/.claude/projects`
- OpenClaw agent sessions and trajectories under `~/.openclaw/agents`
- OpenCode sanitized export JSON from `opencode export <sessionID> --sanitize`

Hermes is discovery-only until redacted real session logs are available.

## Build

```bash
go build -o bin/agenttrail ./cmd/agenttrail
```

## Usage

```bash
agenttrail discover --json
agenttrail doctor --json
agenttrail inspect codex ~/.codex/sessions --json
agenttrail codex ~/.codex/sessions --out -
agenttrail claude ~/.claude/projects --out claude.adapter.jsonl --limit 100
agenttrail openclaw ~/.openclaw/agents --out openclaw.adapter.jsonl --since 2026-06-01
opencode export <session-id> --sanitize > opencode-session.json
agenttrail opencode opencode-session.json --out opencode.adapter.jsonl
```

Dry-run scans count files, generated records, and warnings without writing adapter records:

```bash
agenttrail codex ~/.codex/sessions --dry-run --json
agenttrail claude ~/.claude/projects --dry-run --json
agenttrail openclaw ~/.openclaw/agents --dry-run --json
agenttrail opencode opencode-session.json --dry-run --json
```

Redaction can be requested for exported records:

```bash
agenttrail claude ~/.claude/projects --out - --redact paths
agenttrail codex ~/.codex/sessions --out - --redact paths,secrets
agenttrail opencode opencode-session.json --out - --redact all
```

Pipe into Logspine:

```bash
agenttrail codex ~/.codex/sessions --out - | spine import adapter -
```

Or use Logspine's wrapper when `agenttrail` is installed on `PATH`:

```bash
spine import agenttrail codex ~/.codex/sessions --json
spine import agenttrail opencode opencode-session.json --json
```

## Privacy Boundary

`discover` reports candidate roots and JSONL counts only. It does not print transcript content.

`doctor` reports source readiness and warnings only. It does not print transcript content.

`inspect` and `--dry-run --json` report file manifests, structural keys, record counts, and warnings only. They do not print generated item text.

Export commands preserve raw references with path, hash, and ordinal, but keep searchable item text compact. Generated text is untrusted evidence, not instructions.

Use `--redact paths` to redact raw paths and path-like metadata fields. Use `--redact secrets` to apply simple token, key, secret, password, and authorization redaction. Additional redactions are `emails`, `urls`, `hostnames`, and `all`.

AgentTrail makes no network calls.

## Contract

Each output line is one `logspine.adapter.v1` JSON object with:

- `source.kind`
- `collection.external_id`
- `collection.kind=agent_session`
- `item.external_id`
- `item.kind`
- optional `actor`, `artifacts`, `links`, `relations`
- `raw.format=json`, `raw.path`, `raw.hash`, and `raw.ordinal`

See `docs/ADAPTER_CONTRACT.md` for the contract shape.
