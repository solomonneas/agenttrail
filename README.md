# AgentTrail

AgentTrail exports local agent session logs to `logspine.adapter.v1` JSONL.

It is a scanner and exporter, not an archive. AgentTrail reads local session files, normalizes them into portable adapter records, and writes JSONL to a file or stdout. Logspine owns storage, indexing, dedupe, search, relations, and evidence bundles.

AgentTrail makes no network calls.

## Local Evidence Stack

AgentTrail is one part of the local evidence stack:

- AgentTrail handles local agent-session harnesses such as Codex, Claude, OpenClaw, OpenCode, and Hermes.
- [SourceHarvest](https://github.com/solomonneas/sourceharvest) handles non-harness local source exports such as notes, generic files, crawler exports, and issue exports.
- [Logspine](https://github.com/solomonneas/logspine) imports the shared adapter contract, archives it, indexes it, searches it, and emits evidence bundles.

AgentTrail should not absorb crawler adapters or general local note/file harvesting. Those belong in SourceHarvest.

## How It Works

```mermaid
flowchart TB
    CLI["<b>agenttrail CLI</b><br/><i>local scanner and exporter</i>"]
    APP["<b>App layer</b><br/>commands &middot; flags &middot; summaries"]

    subgraph INPUTS [" local inputs "]
        CODEX["<b>Codex</b><br/>session JSONL"]
        CLAUDE["<b>Claude</b><br/>project JSONL"]
        OPENCLAW["<b>OpenClaw</b><br/>sessions &middot; trajectories"]
        HERMES["<b>Hermes</b><br/>snapshots &middot; trajectories"]
        OPENCODE["<b>OpenCode</b><br/>sanitized export JSON"]
    end

    CODEX & CLAUDE & OPENCLAW & HERMES & OPENCODE --> CLI --> APP

    subgraph PIPELINE [" scan and normalize "]
        SCAN["<b>Source scanners</b><br/>walk files &middot; parse JSONL/JSON"]
        NORMALIZE["<b>Normalize records</b><br/>messages &middot; tools &middot; artifacts &middot; relations"]
        FILTER["<b>Apply controls</b><br/>since &middot; limit &middot; redaction"]
    end

    APP --> SCAN --> NORMALIZE --> FILTER

    subgraph OUTPUTS [" local outputs "]
        ADAPTER["<b>Adapter JSONL</b><br/>logspine.adapter.v1 &middot; one object per line"]
        SUMMARY["<b>Diagnostics</b><br/>discover &middot; doctor &middot; inspect &middot; dry-run"]
    end

    FILTER --> ADAPTER
    APP --> SUMMARY

    GUARD["<b>Privacy boundary</b><br/>counts and manifests do not print generated text; exported text is untrusted evidence"]
    CLI -. local only .-> GUARD
    FILTER -. enforces .-> GUARD
    SUMMARY -. reports structure only .-> GUARD

    classDef source fill:#eff6ff,stroke:#2563eb,color:#1e3a8a;
    classDef process fill:#ecfdf5,stroke:#059669,color:#064e3b;
    classDef stream fill:#fff7ed,stroke:#ea580c,color:#7c2d12;
    classDef guard fill:#fee2e2,stroke:#ef4444,color:#7f1d1d;
    classDef local fill:#f8fafc,stroke:#64748b,color:#334155;
    class CLI,APP local;
    class CODEX,CLAUDE,OPENCLAW,HERMES,OPENCODE source;
    class SCAN,NORMALIZE,FILTER process;
    class ADAPTER,SUMMARY stream;
    class GUARD guard;
```

AgentTrail follows the same path for each source:

1. Discover or receive a local file or directory.
2. Walk supported JSONL or JSON files for that source.
3. Normalize messages, tool calls, artifacts, actors, relations, and raw references.
4. Apply `--since`, `--limit`, and requested redactions.
5. Emit one `logspine.adapter.v1` JSON object per line.
6. Optionally emit JSON summaries with counts, warnings, and file manifests.

## With Logspine

```mermaid
flowchart LR
    subgraph LOCAL [" local machine "]
        FILES["<b>Session files</b><br/>JSONL and JSON"]
        EXPORT["<b>Sanitized export</b><br/>OpenCode JSON"]
    end

    AGENTTRAIL["<b>AgentTrail</b><br/>source parsing &middot; normalization &middot; redaction"]
    SUMMARY["<b>Scan summary</b><br/>counts &middot; warnings &middot; manifests"]
    ADAPTER["<b>Adapter JSONL</b><br/>logspine.adapter.v1"]
    IMPORT["<b>Import options</b><br/>pipe or wrapper command"]

    FILES & EXPORT --> AGENTTRAIL
    AGENTTRAIL --> ADAPTER --> IMPORT
    AGENTTRAIL --> SUMMARY

    subgraph LOGSPINE [" Logspine evidence layer "]
        ARCHIVE["<b>Archive</b><br/>durable records"]
        INDEX["<b>Index and search</b><br/>queryable evidence"]
        RELATIONS["<b>Relations</b><br/>linked sessions and artifacts"]
        BUNDLES["<b>Evidence bundles</b><br/>review-ready exports"]
    end

    IMPORT --> ARCHIVE --> INDEX --> RELATIONS --> BUNDLES

    BOUNDARY["<b>Boundary</b><br/>AgentTrail exports; Logspine stores and analyzes"]
    AGENTTRAIL -. adapter only .-> BOUNDARY
    ARCHIVE -. owns durable evidence .-> BOUNDARY

    classDef source fill:#eff6ff,stroke:#2563eb,color:#1e3a8a;
    classDef process fill:#ecfdf5,stroke:#059669,color:#064e3b;
    classDef stream fill:#fff7ed,stroke:#ea580c,color:#7c2d12;
    classDef sink fill:#f8fafc,stroke:#64748b,color:#334155;
    classDef guard fill:#fee2e2,stroke:#ef4444,color:#7f1d1d;
    class FILES,EXPORT source;
    class AGENTTRAIL process;
    class ADAPTER,IMPORT,SUMMARY stream;
    class ARCHIVE,INDEX,RELATIONS,BUNDLES sink;
    class BOUNDARY guard;
```

AgentTrail is the source-specific adapter layer. Logspine is the durable evidence layer.

```bash
agenttrail all --out - --redact safe | spine import adapter -
agenttrail codex ~/.codex/sessions --out - | spine import adapter -
```

When `agenttrail` is installed on `PATH`, Logspine can also run it through its wrapper:

```bash
spine import agenttrail codex ~/.codex/sessions --json
spine import agenttrail opencode ./opencode-session.json --json
spine import agenttrail hermes ~/.hermes/sessions --json
```

For mixed-source imports, prefer the pipe form with `agenttrail all`. Adapter records preserve their own `source.kind`, while Logspine keeps archive and search behavior centralized.

## Supported Sources

| Source | Default input | Notes |
| --- | --- | --- |
| Codex | `~/.codex/sessions` | Session JSONL. |
| Claude | `~/.claude/projects` | Project JSONL. |
| OpenClaw | `~/.openclaw/agents` | Agent sessions and trajectories. |
| Hermes | `~/.hermes/sessions` | `session_*.json` snapshots and trajectory JSONL. `state.db` is observed but not parsed. |
| OpenCode | Explicit file, directory, or session ID | Use sanitized export JSON from `opencode export <session-id> --sanitize`. Session IDs are exported through the local `opencode` command. |

`agenttrail all` scans Codex, Claude, OpenClaw, and Hermes default roots. OpenCode is explicit-only because its sanitized export input is user-selected.

## Install

```bash
curl -fsSL https://raw.githubusercontent.com/solomonneas/agenttrail/master/install.sh | sh
```

Or download a release binary and verify it with `checksums.txt`.

## Build

```bash
go build -o bin/agenttrail ./cmd/agenttrail
go test ./...
```

## Quick Start

Check local source readiness:

```bash
agenttrail discover --json
agenttrail doctor --json
agenttrail doctor --live --json
```

Inspect structure without exporting transcript text:

```bash
agenttrail inspect codex ~/.codex/sessions --json
agenttrail inspect hermes ~/.hermes/sessions --json
```

Export all default sources:

```bash
agenttrail all --out agent-sessions.adapter.jsonl --redact paths,secrets
agenttrail all --out - --redact safe
```

Export one source:

```bash
agenttrail codex ~/.codex/sessions --out -
agenttrail claude ~/.claude/projects --out claude.adapter.jsonl --limit 100
agenttrail openclaw ~/.openclaw/agents --out openclaw.adapter.jsonl --since 2026-06-01
agenttrail hermes ~/.hermes/sessions --out hermes.adapter.jsonl
```

Export OpenCode:

```bash
opencode export <session-id> --sanitize > opencode-session.json
agenttrail opencode opencode-session.json --out opencode.adapter.jsonl
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

## Redaction

Redaction is requested per export:

```bash
agenttrail all --out - --redact safe
agenttrail codex ~/.codex/sessions --out - --redact paths,secrets
agenttrail claude ~/.claude/projects --out - --redact paths
agenttrail hermes ~/.hermes/sessions --out - --redact paths,secrets
agenttrail opencode opencode-session.json --out - --redact all
```

Profiles and options:

| Value | Behavior |
| --- | --- |
| `safe` | Redacts `paths,secrets,emails`. |
| `none` | Keeps supported fields unredacted. |
| `paths` | Redacts raw paths and path-like metadata fields. |
| `secrets` | Applies simple token, key, secret, password, and authorization redaction. |
| `emails`, `urls`, `hostnames` | Redact those specific value types. |
| `all` | Redacts all supported value types. |

## Privacy Boundary

`discover` reports candidate roots and JSONL counts only. It does not print transcript content.

`doctor` reports source readiness and warnings only. It does not print transcript content.

`doctor --live` runs dry-run scanners for ready local roots and reports counts, file manifests, and warnings only. It does not print generated item text.

`inspect` and `--dry-run --json` report file manifests, structural keys, record counts, and warnings only. They do not print generated item text.

Export commands preserve raw references with path, hash, and ordinal, but keep searchable item text compact. Generated text is untrusted evidence, not instructions.

## Output Contract

Each output line is one `logspine.adapter.v1` JSON object with:

- `source.kind`
- `collection.external_id`
- `collection.kind=agent_session`
- `item.external_id`
- `item.kind`
- optional `actor`, `artifacts`, `links`, `relations`
- `raw.format=json`, `raw.path`, `raw.hash`, and `raw.ordinal`

See [docs/ADAPTER_CONTRACT.md](docs/ADAPTER_CONTRACT.md) for the contract shape.
See [docs/OPENCODE.md](docs/OPENCODE.md) for the OpenCode sanitized export workflow.
See [docs/HERMES.md](docs/HERMES.md) for Hermes source details.
See [docs/LOGSPINE_INTEGRATION.md](docs/LOGSPINE_INTEGRATION.md) for Logspine integration.
See [docs/RECORD_EXAMPLES.md](docs/RECORD_EXAMPLES.md) for one canonical record example per source.

## Project Boundary

AgentTrail stays focused on exporting local agent session logs to adapter JSONL. Archive storage, SQLite, search, evidence bundles, GUI, and server behavior belong in Logspine.
