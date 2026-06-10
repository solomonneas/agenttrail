# Roadmap

StationTrail is usable now as a local scanner and exporter that normalizes agent-session logs into `miseledger.adapter.v1` JSONL. It reads local files, normalizes them, and writes adapter records. It makes no network calls and owns no storage.

## Usable Now

- Export Codex session JSONL from `~/.codex/sessions` or an explicit path.
- Export Claude project JSONL from `~/.claude/projects` or an explicit path.
- Export OpenClaw agent sessions and trajectory JSONL from `~/.openclaw/agents` or an explicit path.
- Export Hermes `session_*.json` snapshots and trajectory JSONL from `~/.hermes/sessions` or an explicit path.
- Run `stationtrail all` to export Codex, Claude, OpenClaw, and Hermes default roots in one pass.
- Normalize messages, tool calls, artifacts, actors, relations, and raw references into one contract.
- Apply `--since`, `--limit`, and `safe/none/paths/secrets/emails/urls/hostnames/all` redaction per export.
- Inspect structure without printing transcript text via `discover`, `doctor`, `doctor --live`, `inspect`, and `--dry-run --json`.
- Emit JSON summaries with record counts, warnings, and per-file manifests.
- Tolerate malformed input: truncated JSON lines, wrong types, missing fields, empty files, and oversized lines warn and keep going instead of aborting the export.

## OpenCode Adapter Maturity

OpenCode support is the least mature source and is intentionally explicit-only.

- There is no default root. `stationtrail all` does not scan OpenCode, and discovery reports OpenCode as `blocked_on_samples` until a real sanitized export is supplied.
- The reliable path is a sanitized export file or directory produced by `opencode export <session-id> --sanitize`. Passing a bare session ID makes StationTrail shell out to the local `opencode` binary, which must be installed and on `PATH`.
- The parser currently reads `text`, `tool`, and `reasoning` parts into searchable item text. Other part types are skipped, and `tool` plus `step-finish` parts become artifacts.
- Timestamps are read as unix milliseconds. Only a single sanitized export shape is exercised by the current fixture.
- Treat OpenCode export records as best-effort until more real sanitized export shapes are observed.

## Later

- Add more real redacted fixture shapes for each harness, especially OpenCode sanitized exports and Hermes trajectory variants.
- Harden the OpenCode adapter against additional part types and export schema versions as real samples appear.
- Direct Hermes `state.db` support only after real redacted samples and a stable schema need exist. `state.db` is observed today but not parsed.
- Native support for any future harness only after observed local samples exist.
- Clearer diagnostics when a required external tool (such as `opencode`) is missing.

## Non-Goals

- No archive storage, SQLite, search, relations backfill, or evidence bundles. Those belong in MiseLedger.
- No crawler adapters or general note and file harvesting. Those belong in SourceHarvest.
- No GUI and no server requirement.
- No network calls from any command.
- No imported text treated as instructions. Generated item text is untrusted evidence.
- No parser parity chase with full session-browser tools.
