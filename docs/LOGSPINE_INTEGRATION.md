# AgentTrail and Logspine

AgentTrail exports local agent-session logs. Logspine archives and indexes evidence.

The boundary is `logspine.adapter.v1` JSONL:

```bash
agenttrail all --out - --redact safe | spine import adapter -
agenttrail codex ~/.codex/sessions --out - | spine import adapter -
agenttrail claude ~/.claude/projects --out - | spine import adapter -
agenttrail openclaw ~/.openclaw/agents --out - | spine import adapter -
agenttrail opencode ./opencode-export.json --out - | spine import adapter -
agenttrail hermes ~/.hermes/sessions --out - | spine import adapter -
```

Logspine also has a wrapper command when `agenttrail` is installed on `PATH`:

```bash
spine import agenttrail codex ~/.codex/sessions --json
spine import agenttrail claude ~/.claude/projects --json
spine import agenttrail openclaw ~/.openclaw/agents --json
spine import agenttrail opencode ./opencode-export.json --json
spine import agenttrail hermes ~/.hermes/sessions --json
```

The wrapper streams AgentTrail output into adapter ingest and records AgentTrail scan manifests when AgentTrail writes a summary.

For mixed-source imports, prefer the pipe form with `agenttrail all`. The adapter records preserve their own `source.kind`, while Logspine keeps archive, FTS, relation, and evidence behavior centralized.

Use AgentTrail for source-specific harness parsing. Use Logspine for SQLite storage, FTS search, relations, scan manifests, and evidence bundles.
