# OpenCode Workflow

AgentTrail treats OpenCode as an explicit sanitized-export source. It does not parse OpenCode's private SQLite database directly.

Use OpenCode's own export command first:

```bash
opencode export <session-id> --sanitize > opencode-session.json
agenttrail opencode opencode-session.json --out -
```

AgentTrail can also run the export command when passed a session ID:

```bash
agenttrail opencode <session-id> --out -
```

For Logspine:

```bash
opencode export <session-id> --sanitize > opencode-session.json
agenttrail opencode opencode-session.json --out - | spine import adapter -
spine import agenttrail opencode opencode-session.json --json
```

Privacy notes:

- Use `opencode export --sanitize`.
- Use `agenttrail opencode ... --redact paths,secrets` when exporting records.
- `agenttrail all` does not include OpenCode because OpenCode requires an explicit session ID or sanitized export path.
