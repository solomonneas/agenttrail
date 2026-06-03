# Hermes Adapter Status

Hermes native export remains blocked on a real redacted log sample.

Current local state only showed Hermes configuration and plugins, not readable session logs. Do not invent a parser from configuration files.

To unblock support:

1. Recreate a Hermes run in a disposable environment.
2. Capture the smallest possible redacted session log sample.
3. Record only structural fields, event types, timestamps, actor/model identifiers, tool calls, artifacts, and raw references.
4. Add the sample under `testdata/harnesses/`.
5. Implement a source parser that emits `logspine.adapter.v1`.

Until then, `agenttrail discover --json` should report Hermes as blocked on samples.
