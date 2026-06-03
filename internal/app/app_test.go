package app

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/openclaw/agenttrail/internal/adapter"
	"github.com/openclaw/agenttrail/internal/sources"
)

func TestExportCommandsEmitValidAdapterRecords(t *testing.T) {
	cases := []struct {
		name    string
		fixture string
		want    string
	}{
		{"codex", "codex-session.fixture.jsonl", "codex"},
		{"claude", "claude-project.fixture.jsonl", "claude"},
		{"openclaw", "openclaw-session.fixture.jsonl", "openclaw"},
		{"openclaw", "openclaw-trajectory.fixture.jsonl", "openclaw"},
		{"opencode", "opencode-export.fixture.json", "opencode"},
		{"hermes", "session_hermes-demo.fixture.json", "hermes"},
		{"hermes", "hermes-trajectory.fixture.jsonl", "hermes"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			code := Run([]string{tc.name, fixturePath(tc.fixture), "--out", "-"}, &stdout, &stderr)
			if code != 0 {
				t.Fatalf("exit %d stderr=%s", code, stderr.String())
			}
			lines := strings.Split(strings.TrimSpace(stdout.String()), "\n")
			if len(lines) == 0 || lines[0] == "" {
				t.Fatalf("no records emitted")
			}
			for _, line := range lines {
				rec, err := adapter.Parse([]byte(line))
				if err != nil {
					t.Fatalf("invalid adapter record: %v\n%s", err, line)
				}
				if rec.Source.Kind != tc.want {
					t.Fatalf("source kind = %q, want %q", rec.Source.Kind, tc.want)
				}
				if rec.Collection.Kind != "agent_session" {
					t.Fatalf("collection kind = %q", rec.Collection.Kind)
				}
			}
		})
	}
}

func TestCodexLimitAndJSONSummary(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := Run([]string{"codex", fixturePath("codex-session.fixture.jsonl"), "--out", "-", "--limit", "1", "--json"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit %d stderr=%s", code, stderr.String())
	}
	lines := strings.Split(strings.TrimSpace(stdout.String()), "\n")
	if len(lines) != 1 {
		t.Fatalf("stdout records = %d, want 1", len(lines))
	}
	var summary map[string]any
	if err := json.Unmarshal(stderr.Bytes(), &summary); err != nil {
		t.Fatalf("summary was not JSON: %v\n%s", err, stderr.String())
	}
	if got := summary["records"].(float64); got != 1 {
		t.Fatalf("summary records = %v, want 1", got)
	}
}

func TestDryRunDoesNotEmitRecords(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := Run([]string{"codex", fixturePath("codex-session.fixture.jsonl"), "--dry-run", "--json"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit %d stderr=%s", code, stderr.String())
	}
	if stderr.Len() != 0 {
		t.Fatalf("dry-run wrote stderr: %s", stderr.String())
	}
	var summary map[string]any
	if err := json.Unmarshal(stdout.Bytes(), &summary); err != nil {
		t.Fatalf("summary was not JSON: %v\n%s", err, stdout.String())
	}
	if summary["dry_run"] != true {
		t.Fatalf("dry_run = %#v", summary["dry_run"])
	}
	if got := summary["records"].(float64); got == 0 {
		t.Fatalf("expected dry-run to count records")
	}
	if _, ok := summary["warnings"].([]any); !ok {
		t.Fatalf("warnings should be an array: %#v", summary["warnings"])
	}
	if strings.Contains(stdout.String(), "adapter contract") {
		t.Fatalf("dry-run summary leaked fixture content")
	}
}

func TestAllExportsDefaultRoots(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	copyFixture(t, "codex-session.fixture.jsonl", filepath.Join(home, ".codex", "sessions", "2026", "06", "03", "codex.jsonl"))
	copyFixture(t, "claude-project.fixture.jsonl", filepath.Join(home, ".claude", "projects", "project", "claude.jsonl"))
	copyFixture(t, "openclaw-session.fixture.jsonl", filepath.Join(home, ".openclaw", "agents", "demo", "sessions", "openclaw.jsonl"))
	copyFixture(t, "session_hermes-demo.fixture.json", filepath.Join(home, ".hermes", "sessions", "session_hermes-demo.json"))

	var stdout, stderr bytes.Buffer
	code := Run([]string{"all", "--out", "-", "--json", "--redact", "paths"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit %d stderr=%s", code, stderr.String())
	}
	records := parseRecords(t, stdout.String())
	kinds := map[string]bool{}
	for _, rec := range records {
		kinds[rec.Source.Kind] = true
		if strings.Contains(rec.Raw.Path, home) {
			t.Fatalf("raw path was not redacted: %s", rec.Raw.Path)
		}
	}
	for _, source := range []string{"codex", "claude", "openclaw", "hermes"} {
		if !kinds[source] {
			t.Fatalf("missing source %s in all export; got %v", source, kinds)
		}
	}
	var summary map[string]any
	if err := json.Unmarshal(stderr.Bytes(), &summary); err != nil {
		t.Fatalf("summary was not JSON: %v\n%s", err, stderr.String())
	}
	if summary["source"] != "all" || summary["records"].(float64) != float64(len(records)) {
		t.Fatalf("bad all summary: %v records=%d", summary, len(records))
	}
}

func TestDoctorLiveReportsCountsOnly(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	secret := "PRIVATE_LIVE_DOCTOR_CONTENT"
	path := filepath.Join(home, ".codex", "sessions", "2026", "06", "03", "codex.jsonl")
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(`{"type":"event_msg","timestamp":"2026-06-03T00:00:00Z","payload":{"session_id":"demo","role":"user","message":"`+secret+`"}}`+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	var stdout, stderr bytes.Buffer
	code := Run([]string{"doctor", "--live", "--json"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit %d stderr=%s", code, stderr.String())
	}
	if strings.Contains(stdout.String(), secret) || strings.Contains(stdout.String(), "event_msg") {
		t.Fatalf("doctor --live leaked content or event detail: %s", stdout.String())
	}
	var report DoctorReport
	if err := json.Unmarshal(stdout.Bytes(), &report); err != nil {
		t.Fatalf("invalid doctor JSON: %v", err)
	}
	found := false
	for _, check := range report.LiveChecks {
		if check.Source == "codex" {
			found = true
			if check.Records != 1 || check.Files != 1 {
				t.Fatalf("bad codex live check: %#v", check)
			}
		}
	}
	if !found {
		t.Fatalf("missing codex live check: %#v", report.LiveChecks)
	}
}

func TestRedactPathsAndSecrets(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := Run([]string{"codex", fixturePath("codex-session.fixture.jsonl"), "--out", "-", "--limit", "1", "--redact", "paths,secrets"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit %d stderr=%s", code, stderr.String())
	}
	rec, err := adapter.Parse(bytes.TrimSpace(stdout.Bytes()))
	if err != nil {
		t.Fatalf("invalid adapter record: %v", err)
	}
	if strings.Contains(rec.Raw.Path, "testdata") {
		t.Fatalf("raw path was not redacted: %s", rec.Raw.Path)
	}
	if rec.Artifacts == nil || rec.Links == nil || rec.Relations == nil {
		t.Fatalf("expected empty arrays, got artifacts=%#v links=%#v relations=%#v", rec.Artifacts, rec.Links, rec.Relations)
	}
}

func TestRedactAllIncludesEmailsURLsAndHosts(t *testing.T) {
	opts, err := parseRedactions("all")
	if err != nil {
		t.Fatal(err)
	}
	text := "token=abc123 email demo@example.com url https://private.example.com/path host build.internal"
	redacted := sourcesRedactText(text, opts)
	for _, forbidden := range []string{"abc123", "demo@example.com", "https://private.example.com/path", "build.internal"} {
		if strings.Contains(redacted, forbidden) {
			t.Fatalf("redaction leaked %q in %q", forbidden, redacted)
		}
	}
}

func TestRedactionProfiles(t *testing.T) {
	safe, err := parseRedactions("safe")
	if err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{"paths", "secrets", "emails"} {
		if !safe[key] {
			t.Fatalf("safe profile missing %s: %v", key, safe)
		}
	}
	if safe["urls"] || safe["hostnames"] {
		t.Fatalf("safe profile redacts too much: %v", safe)
	}
	none, err := parseRedactions("none")
	if err != nil {
		t.Fatal(err)
	}
	if len(none) != 0 {
		t.Fatalf("none profile = %v, want empty", none)
	}
}

func TestSummaryOutWritesManifest(t *testing.T) {
	outPath := filepath.Join(t.TempDir(), "summary.json")
	var stdout, stderr bytes.Buffer
	code := Run([]string{"codex", fixturePath("codex-session.fixture.jsonl"), "--out", "-", "--limit", "1", "--summary-out", outPath}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit %d stderr=%s", code, stderr.String())
	}
	var summary map[string]any
	if err := readJSONFile(outPath, &summary); err != nil {
		t.Fatal(err)
	}
	if summary["records"].(float64) != 1 {
		t.Fatalf("records = %v, want 1", summary["records"])
	}
	files := summary["files"].([]any)
	if len(files) != 1 {
		t.Fatalf("files = %d, want 1", len(files))
	}
}

func TestFixtureOutputIsDeterministic(t *testing.T) {
	first := exportFixture(t, "claude", "claude-project.fixture.jsonl")
	second := exportFixture(t, "claude", "claude-project.fixture.jsonl")
	if first != second {
		t.Fatalf("fixture output changed between runs")
	}
	sum := sha256.Sum256([]byte(first))
	if hex.EncodeToString(sum[:]) == "" {
		t.Fatalf("missing output hash")
	}
}

func TestToolRelationsArePreserved(t *testing.T) {
	codexRecords := parseRecords(t, exportFixture(t, "codex", "codex-session.fixture.jsonl"))
	if !hasRelation(codexRecords, "codex:call:", "result_of") {
		t.Fatalf("missing Codex call result relation")
	}
	claudeRecords := parseRecords(t, exportFixture(t, "claude", "claude-project.fixture.jsonl"))
	if !hasRelation(claudeRecords, "claude:tool_use:", "result_of") {
		t.Fatalf("missing Claude tool result relation")
	}
}

func TestGoldenFixtureFields(t *testing.T) {
	records := parseRecords(t, exportFixture(t, "opencode", "opencode-export.fixture.json"))
	if len(records) != 2 {
		t.Fatalf("OpenCode records = %d, want 2", len(records))
	}
	first := records[0]
	if first.Item.ExternalID != "opencode:message:msg_user" {
		t.Fatalf("external id = %q", first.Item.ExternalID)
	}
	if first.Actor == nil || first.Actor.Type != "human" {
		t.Fatalf("actor = %#v", first.Actor)
	}
	second := records[1]
	if len(second.Artifacts) == 0 || second.Artifacts[0].Kind != "command" {
		t.Fatalf("expected command artifact: %#v", second.Artifacts)
	}
	if second.Raw.Ordinal == nil || *second.Raw.Ordinal != 2 {
		t.Fatalf("raw ordinal = %#v", second.Raw.Ordinal)
	}
}

func TestHermesSnapshotAndTrajectoryFields(t *testing.T) {
	snapshot := parseRecords(t, exportFixture(t, "hermes", "session_hermes-demo.fixture.json"))
	if len(snapshot) != 4 {
		t.Fatalf("Hermes snapshot records = %d, want 4", len(snapshot))
	}
	if snapshot[0].Collection.ExternalID != "hermes:session:hermes-demo" {
		t.Fatalf("collection external id = %q", snapshot[0].Collection.ExternalID)
	}
	if snapshot[2].Item.ExternalID != "hermes:tool_call:hermes-tool-1" {
		t.Fatalf("tool call external id = %q", snapshot[2].Item.ExternalID)
	}
	if len(snapshot[2].Artifacts) == 0 || snapshot[2].Artifacts[0].Kind != "command" {
		t.Fatalf("tool call command artifact missing: %#v", snapshot[2].Artifacts)
	}
	if snapshot[3].Actor == nil || snapshot[3].Actor.Type != "tool" {
		t.Fatalf("tool actor = %#v", snapshot[3].Actor)
	}
	if snapshot[3].Item.Kind != "tool_call" {
		t.Fatalf("tool result kind = %q", snapshot[3].Item.Kind)
	}
	if !hasRelation(snapshot, "hermes:tool_call:hermes-tool-1", "result_of") {
		t.Fatalf("Hermes tool result relation missing")
	}
	trajectory := parseRecords(t, exportFixture(t, "hermes", "hermes-trajectory.fixture.jsonl"))
	if len(trajectory) != 2 {
		t.Fatalf("Hermes trajectory records = %d, want 2", len(trajectory))
	}
	if trajectory[1].Actor == nil || trajectory[1].Actor.Type != "assistant" {
		t.Fatalf("trajectory assistant actor = %#v", trajectory[1].Actor)
	}
}

func TestMalformedInputWarnsAndKeepsGoing(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := Run([]string{"codex", fixturePath("malformed-unknown.fixture.jsonl"), "--out", "-", "--json"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit %d stderr=%s", code, stderr.String())
	}
	if strings.TrimSpace(stdout.String()) == "" {
		t.Fatalf("expected at least one adapter record")
	}
	var summary map[string]any
	if err := json.Unmarshal(stderr.Bytes(), &summary); err != nil {
		t.Fatalf("summary was not JSON: %v\n%s", err, stderr.String())
	}
	warnings, ok := summary["warnings"].([]any)
	if !ok || len(warnings) == 0 {
		t.Fatalf("expected warnings in summary: %#v", summary["warnings"])
	}
}

func TestDoctorReportsNoContent(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := Run([]string{"doctor", "--json"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit %d stderr=%s", code, stderr.String())
	}
	if strings.Contains(stdout.String(), "adapter contract") || strings.Contains(stdout.String(), "Claude native import") {
		t.Fatalf("doctor output appears to include fixture or transcript content: %s", stdout.String())
	}
	var report DoctorReport
	if err := json.Unmarshal(stdout.Bytes(), &report); err != nil {
		t.Fatalf("invalid doctor JSON: %v", err)
	}
	if len(report.Sources) == 0 {
		t.Fatalf("expected doctor sources")
	}
	if report.Warnings == nil {
		t.Fatalf("warnings should be an empty array, not null")
	}
}

func TestInspectReportsStructureOnly(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := Run([]string{"inspect", "opencode", fixturePath("opencode-export.fixture.json"), "--json"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit %d stderr=%s", code, stderr.String())
	}
	if strings.Contains(stdout.String(), "OpenCode adapter contract fixture") {
		t.Fatalf("inspect leaked fixture text: %s", stdout.String())
	}
	var report InspectReport
	if err := json.Unmarshal(stdout.Bytes(), &report); err != nil {
		t.Fatalf("invalid inspect JSON: %v", err)
	}
	if report.Records != 2 || report.EventTypes["part:tool"] != 1 {
		t.Fatalf("unexpected inspect report: %#v", report)
	}
}

func TestInspectHermesReportsStructureOnly(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := Run([]string{"inspect", "hermes", fixturePath("session_hermes-demo.fixture.json"), "--json"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit %d stderr=%s", code, stderr.String())
	}
	if strings.Contains(stdout.String(), "Hermes snapshots can be normalized") {
		t.Fatalf("inspect leaked fixture text: %s", stdout.String())
	}
	var report InspectReport
	if err := json.Unmarshal(stdout.Bytes(), &report); err != nil {
		t.Fatalf("invalid inspect JSON: %v", err)
	}
	if report.Files != 1 {
		t.Fatalf("files = %d, want 1", report.Files)
	}
}

func TestDiscoverDoesNotPrintContent(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := Run([]string{"discover", "--json"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit %d stderr=%s", code, stderr.String())
	}
	if strings.Contains(stdout.String(), "adapter contract") || strings.Contains(stdout.String(), "Claude native import") {
		t.Fatalf("discover output appears to include fixture or transcript content: %s", stdout.String())
	}
	var discovery Discovery
	if err := json.Unmarshal(stdout.Bytes(), &discovery); err != nil {
		t.Fatalf("invalid discovery JSON: %v", err)
	}
	if len(discovery.Sources) == 0 {
		t.Fatalf("expected discovery sources")
	}
}

func fixturePath(name string) string {
	return filepath.Join("..", "..", "testdata", "harnesses", name)
}

func copyFixture(t *testing.T, name, to string) {
	t.Helper()
	b, err := os.ReadFile(fixturePath(name))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(to), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(to, b, 0o600); err != nil {
		t.Fatal(err)
	}
}

func readJSONFile(path string, v any) error {
	b, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	return json.Unmarshal(b, v)
}

func sourcesRedactText(text string, redactions map[string]bool) string {
	return sources.RedactText(text, sources.Options{
		RedactPaths:     redactions["paths"],
		RedactSecrets:   redactions["secrets"],
		RedactEmails:    redactions["emails"],
		RedactURLs:      redactions["urls"],
		RedactHostnames: redactions["hostnames"],
	})
}

func exportFixture(t *testing.T, command, fixture string) string {
	t.Helper()
	var stdout, stderr bytes.Buffer
	code := Run([]string{command, fixturePath(fixture), "--out", "-"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit %d stderr=%s", code, stderr.String())
	}
	return stdout.String()
}

func parseRecords(t *testing.T, output string) []adapter.Record {
	t.Helper()
	var records []adapter.Record
	for _, line := range strings.Split(strings.TrimSpace(output), "\n") {
		if line == "" {
			continue
		}
		rec, err := adapter.Parse([]byte(line))
		if err != nil {
			t.Fatalf("invalid adapter record: %v\n%s", err, line)
		}
		records = append(records, rec)
	}
	return records
}

func hasRelation(records []adapter.Record, targetPrefix, relType string) bool {
	for _, rec := range records {
		for _, rel := range rec.Relations {
			if strings.HasPrefix(rel.TargetExternalID, targetPrefix) && rel.Type == relType {
				return true
			}
		}
	}
	return false
}
