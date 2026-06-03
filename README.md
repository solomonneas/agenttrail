# AgentTrail

AgentTrail exports local agent session logs to `logspine.adapter.v1` JSONL.

It is a scanner and exporter, not an archive. Logspine stores, indexes, dedupes, searches, relates, and builds evidence bundles. AgentTrail only reads local session files and emits portable adapter records.

Supported first-pass sources:

- Codex session JSONL under `~/.codex/sessions`
- Claude project JSONL under `~/.claude/projects`
- OpenClaw agent sessions and trajectories under `~/.openclaw/agents`
- OpenCode sanitized export JSON from `opencode export <sessionID> --sanitize`
- Hermes session snapshots and trajectory JSONL under `~/.hermes/sessions`

Hermes `state.db` is observed but not parsed yet. AgentTrail reads opt-in `session_*.json` snapshots and trajectory JSONL files.

## Build

```bash
go build -o bin/agenttrail ./cmd/agenttrail
```

## Install

```bash
curl -fsSL https://raw.githubusercontent.com/solomonneas/agenttrail/master/install.sh | sh
```

Or download a release binary and verify it with `checksums.txt`.

## Usage

```bash
agenttrail discover --json
agenttrail doctor --json
agenttrail doctor --live --json
agenttrail inspect codex ~/.codex/sessions --json
agenttrail all --out agent-sessions.adapter.jsonl --redact paths,secrets
agenttrail codex ~/.codex/sessions --out -
agenttrail claude ~/.claude/projects --out claude.adapter.jsonl --limit 100
agenttrail openclaw ~/.openclaw/agents --out openclaw.adapter.jsonl --since 2026-06-01
opencode export <session-id> --sanitize > opencode-session.json
agenttrail opencode opencode-session.json --out opencode.adapter.jsonl
agenttrail hermes ~/.hermes/sessions --out hermes.adapter.jsonl
```

Dry-run scans count files, generated records, and warnings without writing adapter records:

```bash
agenttrail all --dry-run --json
agenttrail codex ~/.codex/sessions --dry-run --json
agenttrail claude ~/.claude/projects --dry-run --json
agenttrail openclaw ~/.openclaw/agents --dry-run --json
agenttrail opencode opencode-session.json --dry-run --json
agenttrail hermes ~/.hermes/sessions --dry-run --json
```

Redaction can be requested for exported records:

```bash
agenttrail claude ~/.claude/projects --out - --redact paths
agenttrail codex ~/.codex/sessions --out - --redact paths,secrets
agenttrail opencode opencode-session.json --out - --redact all
agenttrail hermes ~/.hermes/sessions --out - --redact paths,secrets
```

Pipe into Logspine:

```bash
agenttrail all --out - --redact paths,secrets | spine import adapter -
agenttrail codex ~/.codex/sessions --out - | spine import adapter -
```

Or use Logspine's wrapper when `agenttrail` is installed on `PATH`:

```bash
spine import agenttrail codex ~/.codex/sessions --json
spine import agenttrail opencode opencode-session.json --json
spine import agenttrail hermes ~/.hermes/sessions --json
```

## Privacy Boundary

`discover` reports candidate roots and JSONL counts only. It does not print transcript content.

`doctor` reports source readiness and warnings only. It does not print transcript content.

`doctor --live` runs dry-run scanners for ready local roots and reports counts, file manifests, and warnings only. It does not print generated item text.

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
See `docs/OPENCODE.md` for the OpenCode sanitized export workflow.
See `docs/RECORD_EXAMPLES.md` for one canonical record example per source.
