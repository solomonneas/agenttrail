# Record Examples

These examples show one compact `miseledger.adapter.v1` record per source. Field values are representative and redacted where useful.

## Codex

```json
{"schema":"miseledger.adapter.v1","source":{"kind":"codex","name":"Codex Sessions"},"collection":{"external_id":"codex:session:demo","kind":"agent_session","name":"demo"},"item":{"external_id":"codex:call:call_demo","kind":"command","created_at":"2026-06-03T12:00:00Z","text":"function_call\nexec_command\ncall_demo\n{\"cmd\":\"go test ./...\"}","tags":["agent-session","codex"]},"actor":{"external_id":"codex:tool:tool","type":"tool","name":"tool"},"artifacts":[{"external_id":"artifact-demo","kind":"command","text":"go test ./...","hash":"sha256:demo"}],"links":[],"relations":[],"raw":{"format":"json","hash":"sha256:demo","path":"[redacted-path]/codex-session.fixture.jsonl","ordinal":3}}
```

## Claude

```json
{"schema":"miseledger.adapter.v1","source":{"kind":"claude","name":"Claude Project Logs"},"collection":{"external_id":"claude:session:demo","kind":"agent_session","name":"demo"},"item":{"external_id":"claude:tool_result:tool_demo","kind":"tool_call","created_at":"2026-06-03T12:01:00Z","text":"tool_result\nCommand output captured","tags":["agent-session","claude"]},"actor":{"external_id":"claude:tool:tool","type":"tool","name":"tool"},"artifacts":[],"links":[],"relations":[{"target_external_id":"claude:tool_use:tool_demo","type":"result_of"}],"raw":{"format":"json","hash":"sha256:demo","path":"[redacted-path]/claude-project.fixture.jsonl","ordinal":4}}
```

## OpenClaw

```json
{"schema":"miseledger.adapter.v1","source":{"kind":"openclaw","name":"OpenClaw Agent Sessions"},"collection":{"external_id":"openclaw:session:demo","kind":"agent_session","name":"demo"},"item":{"external_id":"openclaw:session:demo","kind":"event","created_at":"2026-06-03T12:02:00Z","text":"session","tags":["agent-session","openclaw"]},"actor":{"external_id":"openclaw:system:system","type":"system","name":"system"},"artifacts":[],"links":[],"relations":[],"raw":{"format":"json","hash":"sha256:demo","path":"[redacted-path]/openclaw-session.fixture.jsonl","ordinal":1}}
```

## OpenCode

```json
{"schema":"miseledger.adapter.v1","source":{"kind":"opencode","name":"OpenCode Sessions"},"collection":{"external_id":"opencode:session:ses_fixture","kind":"agent_session","name":"Fixture session"},"item":{"external_id":"opencode:message:msg_assistant","kind":"command","created_at":"2026-06-03T12:03:00Z","text":"Collected evidence for the adapter contract.\ntool bash call_fixture","tags":["agent-session","opencode"]},"actor":{"external_id":"opencode:assistant:assistant","type":"assistant","name":"assistant"},"artifacts":[{"external_id":"artifact-demo","kind":"command","text":"go test ./...","hash":"sha256:demo"}],"links":[],"relations":[],"raw":{"format":"json","hash":"sha256:demo","path":"[redacted-path]/opencode-export.fixture.json","ordinal":2}}
```

## Hermes

```json
{"schema":"miseledger.adapter.v1","source":{"kind":"hermes","name":"Hermes Sessions"},"collection":{"external_id":"hermes:session:hermes-demo","kind":"agent_session","name":"hermes-demo"},"item":{"external_id":"hermes:demo","kind":"message","created_at":"2026-06-03T20:00:00Z","text":"Hermes snapshots can be normalized into agent-session records.","tags":["agent-session","hermes"]},"actor":{"external_id":"hermes:assistant:assistant","type":"assistant","name":"assistant"},"artifacts":[],"links":[],"relations":[],"raw":{"format":"json","hash":"sha256:demo","path":"[redacted-path]/session_hermes-demo.json","ordinal":2}}
```
