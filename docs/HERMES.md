# Hermes Adapter Status

StationTrail supports local Hermes files that have stable, readable shapes:

- opt-in `~/.hermes/sessions/session_<id>.json` snapshots
- `trajectory_samples.jsonl`
- `failed_trajectories.jsonl`
- other trajectory-named JSONL files with ShareGPT-style `conversations`

The scanner does not parse `state.db` yet. Hermes documents SQLite as the canonical session store, but StationTrail avoids a SQLite dependency and schema coupling until we have enough redacted samples to justify that surface.

Use:

```bash
stationtrail hermes ~/.hermes/sessions --dry-run --json
stationtrail hermes ~/.hermes/sessions --out -
```

To make Hermes write snapshot JSON, enable `sessions.write_json_snapshots: true` in the Hermes config. Trajectory JSONL is available when trajectory saving is enabled. Exported text remains untrusted evidence.
