package hermes

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
		if rec.Source.Kind != "hermes" {
			t.Fatalf("source kind = %q, want hermes", rec.Source.Kind)
		}
		if rec.Collection.Kind != "agent_session" {
			t.Fatalf("collection kind = %q, want agent_session", rec.Collection.Kind)
		}
		records = append(records, rec)
	}
	return records, result
}

// writeTemp writes content under a fresh temp dir using a name that Include accepts.
func writeTemp(t *testing.T, name, content string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestGenerateSnapshotFixture(t *testing.T) {
	records, result := generate(t, fixturePath("session_hermes-demo.fixture.json"), sources.Options{})
	if len(records) == 0 {
		t.Fatalf("no records produced")
	}
	if result.Records != len(records) {
		t.Fatalf("result.Records = %d, emitted = %d", result.Records, len(records))
	}
	var foundToolCall, foundResultRelation bool
	for _, rec := range records {
		if rec.Collection.ExternalID != "hermes:session:hermes-demo" {
			t.Fatalf("collection external id = %q", rec.Collection.ExternalID)
		}
		if rec.Item.ExternalID == "hermes:tool_call:hermes-tool-1" {
			foundToolCall = true
		}
		for _, rel := range rec.Relations {
			if rel.TargetExternalID == "hermes:tool_call:hermes-tool-1" && rel.Type == "result_of" {
				foundResultRelation = true
			}
		}
	}
	if !foundToolCall {
		t.Fatalf("missing hermes:tool_call:hermes-tool-1 item")
	}
	if !foundResultRelation {
		t.Fatalf("missing result_of relation to hermes tool call")
	}
}

func TestGenerateTrajectoryFixture(t *testing.T) {
	records, _ := generate(t, fixturePath("hermes-trajectory.fixture.jsonl"), sources.Options{})
	if len(records) != 2 {
		t.Fatalf("trajectory records = %d, want 2", len(records))
	}
	if records[1].Actor == nil || records[1].Actor.Type != "assistant" {
		t.Fatalf("expected gpt role mapped to assistant actor: %#v", records[1].Actor)
	}
}

func TestGenerateLimit(t *testing.T) {
	records, _ := generate(t, fixturePath("session_hermes-demo.fixture.json"), sources.Options{Limit: 1})
	if len(records) != 1 {
		t.Fatalf("limited records = %d, want 1", len(records))
	}
}

func TestGenerateMalformedSnapshot(t *testing.T) {
	cases := []struct {
		name        string
		file        string
		content     string
		wantRecords bool
		wantWarning bool
	}{
		{
			name:        "truncated snapshot json",
			file:        "session_bad.json",
			content:     `{"session_id":"bad","messages":[{"role":"user","content":"hi"`,
			wantRecords: false,
			wantWarning: true,
		},
		{
			name:        "snapshot with no messages",
			file:        "session_empty.json",
			content:     `{"session_id":"empty","model":"m"}`,
			wantRecords: false,
			wantWarning: true,
		},
		{
			name:        "snapshot message wrong type",
			file:        "session_wrong.json",
			content:     `{"session_id":"w","messages":["not-an-object"]}`,
			wantRecords: false,
			wantWarning: true,
		},
		{
			name:        "empty snapshot file",
			file:        "session_blank.json",
			content:     "",
			wantRecords: false,
			wantWarning: true,
		},
		{
			name:        "truncated trajectory jsonl line",
			file:        "trajectory_bad.jsonl",
			content:     `{"conversations":[{"from":"human","value":"hi"` + "\n",
			wantRecords: false,
			wantWarning: true,
		},
		{
			name:        "trajectory blank lines only",
			file:        "trajectory_blank.jsonl",
			content:     "\n  \n",
			wantRecords: false,
			wantWarning: false,
		},
		{
			name:        "malformed then valid trajectory line",
			file:        "trajectory_mixed.jsonl",
			content:     "broken\n" + `{"conversations":[{"from":"human","value":"after malformed"}]}` + "\n",
			wantRecords: true,
			wantWarning: true,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			path := writeTemp(t, tc.file, tc.content)
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

func TestGenerateHugeSnapshotMessage(t *testing.T) {
	big := strings.Repeat("D", 256*1024)
	content := `{"session_id":"big","messages":[{"role":"user","content":"` + big + `"}]}`
	path := writeTemp(t, "session_big.json", content)
	records, _ := generate(t, path, sources.Options{})
	if len(records) != 1 {
		t.Fatalf("huge snapshot records = %d, want 1", len(records))
	}
	if len(records[0].Item.Text) >= len(big) {
		t.Fatalf("expected huge text to be truncated, got len %d", len(records[0].Item.Text))
	}
}

func TestIncludeFiltersUnsupportedNames(t *testing.T) {
	cases := map[string]bool{
		"session_demo.json":          true,
		"trajectory_samples.jsonl":   true,
		"failed_trajectories.jsonl":  true,
		"some.trajectory.jsonl":      true,
		"request_dump_1.jsonl":       false,
		"notes.metadata.jsonl":       false,
		"session_backup.json":        false,
		"random.txt":                 false,
		"session_deleted-thing.json": false,
	}
	for name, want := range cases {
		if got := Include(filepath.Join("/root", name)); got != want {
			t.Fatalf("Include(%q) = %v, want %v", name, got, want)
		}
	}
}

func TestGenerateMissingRootIsError(t *testing.T) {
	_, err := Generate(filepath.Join(t.TempDir(), "does-not-exist"), sources.Options{}, &bytes.Buffer{})
	if err == nil {
		t.Fatalf("expected error for missing root")
	}
}
