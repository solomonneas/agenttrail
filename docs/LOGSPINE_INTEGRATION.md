# StationTrail and Logspine

StationTrail exports local agent-session logs. Logspine archives and indexes evidence.

The boundary is `logspine.adapter.v1` JSONL:

```bash
stationtrail all --out - --redact safe | spine import adapter -
stationtrail codex ~/.codex/sessions --out - | spine import adapter -
stationtrail claude ~/.claude/projects --out - | spine import adapter -
stationtrail openclaw ~/.openclaw/agents --out - | spine import adapter -
stationtrail opencode ./opencode-export.json --out - | spine import adapter -
stationtrail hermes ~/.hermes/sessions --out - | spine import adapter -
```

Logspine also has a wrapper command when `stationtrail` is installed on `PATH`:

```bash
spine import stationtrail codex ~/.codex/sessions --json
spine import stationtrail claude ~/.claude/projects --json
spine import stationtrail openclaw ~/.openclaw/agents --json
spine import stationtrail opencode ./opencode-export.json --json
spine import stationtrail hermes ~/.hermes/sessions --json
```

The wrapper streams StationTrail output into adapter ingest and records StationTrail scan manifests when StationTrail writes a summary.

For mixed-source imports, prefer the pipe form with `stationtrail all`. The adapter records preserve their own `source.kind`, while Logspine keeps archive, FTS, relation, and evidence behavior centralized.

Use StationTrail for source-specific harness parsing. Use Logspine for SQLite storage, FTS search, relations, scan manifests, and evidence bundles.
