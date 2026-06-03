package app

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"

	"github.com/openclaw/agenttrail/internal/adapter"
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
