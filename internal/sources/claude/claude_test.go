package claude

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
		if rec.Source.Kind != "claude" {
			t.Fatalf("source kind = %q, want claude", rec.Source.Kind)
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

func TestGenerateFixtureProducesContractRecords(t *testing.T) {
	records, result := generate(t, fixturePath("claude-project.fixture.jsonl"), sources.Options{})
	if len(records) == 0 {
		t.Fatalf("no records produced")
	}
	if result.Records != len(records) {
		t.Fatalf("result.Records = %d, emitted = %d", result.Records, len(records))
	}
	for _, rec := range records {
		if rec.Collection.ExternalID != "claude:session:claude-demo" {
			t.Fatalf("collection external id = %q", rec.Collection.ExternalID)
		}
		if rec.Item.ExternalID == "" || rec.Item.Kind == "" {
			t.Fatalf("item missing required fields: %#v", rec.Item)
		}
	}
}

func TestGenerateToolUseAndResultRelation(t *testing.T) {
	records, _ := generate(t, fixturePath("claude-project.fixture.jsonl"), sources.Options{})
	var foundToolUse, foundResultRelation bool
	for _, rec := range records {
		if rec.Item.ExternalID == "claude:tool_use:toolu_1" {
			foundToolUse = true
		}
		for _, rel := range rec.Relations {
			if rel.TargetExternalID == "claude:tool_use:toolu_1" && rel.Type == "result_of" {
				foundResultRelation = true
			}
		}
	}
	if !foundToolUse {
		t.Fatalf("missing claude:tool_use:toolu_1 item")
	}
	if !foundResultRelation {
		t.Fatalf("missing result_of relation to claude:tool_use:toolu_1")
	}
}

func TestGenerateLimit(t *testing.T) {
	records, _ := generate(t, fixturePath("claude-project.fixture.jsonl"), sources.Options{Limit: 1})
	if len(records) != 1 {
		t.Fatalf("limited records = %d, want 1", len(records))
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
			content:     `{"type":"user","message":{"role":"user","content":"hi"` + "\n",
			wantRecords: false,
			wantWarning: true,
		},
		{
			name:        "wrong type for message",
			content:     `{"type":"user","message":"not-an-object","content":"recoverable text"}` + "\n",
			wantRecords: true,
			wantWarning: false,
		},
		{
			// Claude always has the "Claude" fallback label, so even a near-empty
			// object yields a low-signal but contract-valid record rather than a warning.
			name:        "missing fields uses fallback label",
			content:     `{"timestamp":"2026-06-03T19:00:00Z"}` + "\n",
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
			content:     "garbage\n" + `{"type":"user","message":{"role":"user","content":"after malformed"}}` + "\n",
			wantRecords: true,
			wantWarning: true,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			path := writeTemp(t, "claude.jsonl", tc.content)
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
	big := strings.Repeat("B", 256*1024)
	content := `{"type":"user","message":{"role":"user","content":"` + big + `"}}` + "\n"
	path := writeTemp(t, "claude.jsonl", content)
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
