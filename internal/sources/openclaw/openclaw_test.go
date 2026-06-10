package openclaw

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/escoffier-labs/stationtrail/internal/adapter"
	"github.com/escoffier-labs/stationtrail/internal/sources"
)

func fixturePath(name string) string {
	return filepath.Join("..", "..", "..", "testdata", "harnesses", name)
}

func generate(t *testing.T, path string, opts sources.Options) ([]adapter.Record, sources.Result) {
	t.Helper()
	var buf bytes.Buffer
	result, err := Generate(path, opts, &buf)
	if err != nil {
		t.Fatalf("Generate(%s) error: %v", path, err)
	}
	var records []adapter.Record
	for _, line := range strings.Split(strings.TrimSpace(buf.String()), "\n") {
		if line == "" {
			continue
		}
		rec, err := adapter.Parse([]byte(line))
		if err != nil {
			t.Fatalf("emitted invalid adapter record: %v\n%s", err, line)
		}
		if rec.Source.Kind != "openclaw" {
			t.Fatalf("source kind = %q, want openclaw", rec.Source.Kind)
		}
		if rec.Collection.Kind != "agent_session" {
			t.Fatalf("collection kind = %q, want agent_session", rec.Collection.Kind)
		}
		records = append(records, rec)
	}
	return records, result
}

func writeTemp(t *testing.T, name, content string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestGenerateSessionFixture(t *testing.T) {
	records, result := generate(t, fixturePath("openclaw-session.fixture.jsonl"), sources.Options{})
	if len(records) == 0 {
		t.Fatalf("no records produced")
	}
	if result.Records != len(records) {
		t.Fatalf("result.Records = %d, emitted = %d", result.Records, len(records))
	}
	var foundSession bool
	for _, rec := range records {
		if rec.Collection.ExternalID != "openclaw:session:openclaw-demo" {
			t.Fatalf("collection external id = %q", rec.Collection.ExternalID)
		}
		if rec.Item.ExternalID == "openclaw:session:openclaw-demo" {
			foundSession = true
		}
	}
	if !foundSession {
		t.Fatalf("expected a session item with stable external id")
	}
}

func TestGenerateTrajectoryFixtureRelations(t *testing.T) {
	records, _ := generate(t, fixturePath("openclaw-trajectory.fixture.jsonl"), sources.Options{})
	if len(records) == 0 {
		t.Fatalf("no records produced")
	}
	var foundRunRelation, foundSessionRelation bool
	for _, rec := range records {
		for _, rel := range rec.Relations {
			if rel.TargetExternalID == "openclaw:run:run-1" && rel.Type == "belongs_to_run" {
				foundRunRelation = true
			}
			if rel.TargetExternalID == "openclaw:session:trajectory-demo" && rel.Type == "belongs_to_session" {
				foundSessionRelation = true
			}
		}
	}
	if !foundRunRelation {
		t.Fatalf("missing belongs_to_run relation to openclaw:run:run-1")
	}
	if !foundSessionRelation {
		t.Fatalf("missing belongs_to_session relation to openclaw:session:trajectory-demo")
	}
}

func TestGenerateLimit(t *testing.T) {
	records, _ := generate(t, fixturePath("openclaw-session.fixture.jsonl"), sources.Options{Limit: 2})
	if len(records) != 2 {
		t.Fatalf("limited records = %d, want 2", len(records))
	}
}

func TestGenerateMalformedInput(t *testing.T) {
	cases := []struct {
		name        string
		content     string
		wantRecords bool
		wantWarning bool
	}{
		{
			name:        "truncated json line",
			content:     `{"type":"message","session_id":"x","message":"hi"` + "\n",
			wantRecords: false,
			wantWarning: true,
		},
		{
			name:        "wrong type for data",
			content:     `{"type":"message","session_id":"x","data":"not-an-object","message":"recoverable"}` + "\n",
			wantRecords: true,
			wantWarning: false,
		},
		{
			name:        "missing type and text",
			content:     `{"timestamp":"2026-06-03T16:00:00Z","session_id":"x"}` + "\n",
			wantRecords: false,
			wantWarning: true,
		},
		{
			name:        "session event with no text still emits",
			content:     `{"type":"session","session_id":"x","workspace_dir":"/w"}` + "\n",
			wantRecords: true,
			wantWarning: false,
		},
		{
			name:        "empty file",
			content:     "",
			wantRecords: false,
			wantWarning: false,
		},
		{
			name:        "malformed then valid keeps going",
			content:     "broken\n" + `{"type":"message","session_id":"x","role":"human","message":"after malformed"}` + "\n",
			wantRecords: true,
			wantWarning: true,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			path := writeTemp(t, "openclaw.jsonl", tc.content)
			records, result := generate(t, path, sources.Options{})
			if tc.wantRecords && len(records) == 0 {
				t.Fatalf("expected records, got none (warnings=%v)", result.Warnings)
			}
			if !tc.wantRecords && len(records) != 0 {
				t.Fatalf("expected no records, got %d", len(records))
			}
			if tc.wantWarning && len(result.Warnings) == 0 {
				t.Fatalf("expected a warning")
			}
			if !tc.wantWarning && len(result.Warnings) != 0 {
				t.Fatalf("unexpected warnings: %v", result.Warnings)
			}
		})
	}
}

func TestGenerateHugeLine(t *testing.T) {
	big := strings.Repeat("C", 256*1024)
	content := `{"type":"message","session_id":"big","role":"human","message":"` + big + `"}` + "\n"
	path := writeTemp(t, "openclaw.jsonl", content)
	records, _ := generate(t, path, sources.Options{})
	if len(records) != 1 {
		t.Fatalf("huge line records = %d, want 1", len(records))
	}
	if len(records[0].Item.Text) >= len(big) {
		t.Fatalf("expected huge text to be truncated, got len %d", len(records[0].Item.Text))
	}
}

func TestGenerateMissingRootIsError(t *testing.T) {
	_, err := Generate(filepath.Join(t.TempDir(), "does-not-exist"), sources.Options{}, &bytes.Buffer{})
	if err == nil {
		t.Fatalf("expected error for missing root")
	}
}
