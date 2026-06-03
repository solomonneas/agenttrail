# Adapter Contract

AgentTrail emits `logspine.adapter.v1` JSONL.

Each line is one JSON object. Required fields:

- `schema`: `logspine.adapter.v1`
- `source.kind`
- `collection.external_id`
- `collection.kind`
- `item.external_id`
- `item.kind`

Recommended fields:

- `item.created_at`
- `item.text`
- `item.metadata`
- `actor.external_id`, `actor.type`, and `actor.name`
- `artifacts`
- `links`
- `relations`
- `raw.format`, `raw.hash`, `raw.path`, and `raw.ordinal`

Example:

```json
{"schema":"logspine.adapter.v1","source":{"kind":"codex","name":"Codex Sessions"},"collection":{"external_id":"codex:session:demo","kind":"agent_session","name":"demo"},"item":{"external_id":"codex:demo-item","kind":"message","created_at":"2026-06-03T12:00:00Z","text":"example","tags":["agent-session","codex"]},"actor":{"external_id":"codex:human:human","type":"human","name":"human"},"artifacts":[],"links":[],"relations":[],"raw":{"format":"json","hash":"sha256:example","path":"session.jsonl","ordinal":1}}
```

Identity should be deterministic. If a source lacks stable IDs, AgentTrail creates external IDs from path, session ID, ordinal, event type, timestamp, call ID when available, and content hash.

## Scanner Behavior

AgentTrail scanners:

- Accept a file or directory.
- Walk relevant `.jsonl` files recursively, plus source-specific JSON files such as Hermes `session_*.json` snapshots.
- Skip obvious backups, deleted files, `skills-prompts`, and sidecar metadata.
- Preserve raw refs with `raw.format=json`, `raw.path`, `raw.hash`, and `raw.ordinal`.
- Emit warnings and keep going on malformed or unknown event shapes.
- Keep `item.text` searchable without dumping huge raw JSON blobs as text.
- Preserve useful non-secret source structure in `item.metadata`.
- Emit empty arrays for `artifacts`, `links`, and `relations` when no values are present.

`--dry-run --json` reports scan counts and warnings without writing adapter records. `discover` and `doctor` report source roots and counts only, not transcript content.

`--redact paths` redacts raw paths and path-like metadata fields. `--redact secrets` applies simple secret-pattern redaction to generated text and metadata before records are written. Additional redactions are `emails`, `urls`, `hostnames`, and `all`.

OpenCode support consumes sanitized export JSON from:

```bash
opencode export <sessionID> --sanitize
```

AgentTrail can also run the local export command when passed an OpenCode session ID. It does not parse OpenCode's private SQLite database directly.

Hermes support consumes:

- `~/.hermes/sessions/session_*.json` snapshots when Hermes has `sessions.write_json_snapshots: true`
- `trajectory_samples.jsonl`
- `failed_trajectories.jsonl`
- other trajectory-named JSONL files with ShareGPT-style `conversations`

It does not parse Hermes `state.db` directly.
