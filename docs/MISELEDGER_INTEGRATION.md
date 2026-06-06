# StationTrail and MiseLedger

StationTrail exports local agent-session logs. MiseLedger archives and indexes evidence.

The boundary is `miseledger.adapter.v1` JSONL:

```bash
stationtrail all --out - --redact safe | miseledger import adapter -
stationtrail codex ~/.codex/sessions --out - | miseledger import adapter -
stationtrail claude ~/.claude/projects --out - | miseledger import adapter -
stationtrail openclaw ~/.openclaw/agents --out - | miseledger import adapter -
stationtrail opencode ./opencode-export.json --out - | miseledger import adapter -
stationtrail hermes ~/.hermes/sessions --out - | miseledger import adapter -
```

MiseLedger also has a wrapper command when `stationtrail` is installed on `PATH`:

```bash
miseledger import stationtrail codex ~/.codex/sessions --json
miseledger import stationtrail claude ~/.claude/projects --json
miseledger import stationtrail openclaw ~/.openclaw/agents --json
miseledger import stationtrail opencode ./opencode-export.json --json
miseledger import stationtrail hermes ~/.hermes/sessions --json
```

The wrapper streams StationTrail output into adapter ingest and records StationTrail scan manifests when StationTrail writes a summary.

For mixed-source imports, prefer the pipe form with `stationtrail all`. The adapter records preserve their own `source.kind`, while MiseLedger keeps archive, FTS, relation, and evidence behavior centralized.

Use StationTrail for source-specific harness parsing. Use MiseLedger for SQLite storage, FTS search, relations, scan manifests, and evidence bundles.
