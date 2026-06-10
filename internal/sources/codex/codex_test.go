package codex

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

// generate runs the parser against an explicit file path and returns the
// adapter records plus the scan result. Every record is validated against the
// miseledger.adapter.v1 contract the exporter relies on.
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
		if rec.Source.Kind != "codex" {
			t.Fatalf("source kind = %q, want codex", rec.Source.Kind)
		}
		if rec.Collection.Kind != "agent_session" {
			t.Fatalf("collection kind = %q, want agent_session", rec.Collection.Kind)
		}
		records = append(records, rec)
	}
	return records, result
}

// writeTemp writes content to a .jsonl file inside a fresh temp dir and returns
// the path, so each case scans an isolated directory.
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
	records, result := generate(t, fixturePath("codex-session.fixture.jsonl"), sources.Options{})
	if len(records) == 0 {
		t.Fatalf("no records produced from fixture")
	}
	if result.Records != len(records) {
		t.Fatalf("result.Records = %d, emitted = %d", result.Records, len(records))
	}
	if len(result.Files) != 1 {
		t.Fatalf("result.Files = %d, want 1", len(result.Files))
	}
	for _, rec := range records {
		if rec.Collection.ExternalID != "codex:session:codex-demo" {
			t.Fatalf("collection external id = %q", rec.Collection.ExternalID)
		}
		if rec.Item.ExternalID == "" || rec.Item.Kind == "" {
			t.Fatalf("item missing required fields: %#v", rec.Item)
		}
		if rec.Raw.Format != "json" || rec.Raw.Ordinal == nil {
			t.Fatalf("raw ref incomplete: %#v", rec.Raw)
		}
	}
}

func TestGenerateLinksFunctionCallResult(t *testing.T) {
	records, _ := generate(t, fixturePath("codex-session.fixture.jsonl"), sources.Options{})
	var foundCall, foundResultRelation bool
	for _, rec := range records {
		if rec.Item.ExternalID == "codex:call:call-123" {
			foundCall = true
		}
		for _, rel := range rec.Relations {
			if rel.TargetExternalID == "codex:call:call-123" && rel.Type == "result_of" {
				foundResultRelation = true
			}
		}
	}
	if !foundCall {
		t.Fatalf("missing codex:call:call-123 item")
	}
	if !foundResultRelation {
		t.Fatalf("missing result_of relation to codex:call:call-123")
	}
}

func TestGenerateLimit(t *testing.T) {
	records, result := generate(t, fixturePath("codex-session.fixture.jsonl"), sources.Options{Limit: 2})
	if len(records) != 2 {
		t.Fatalf("limited records = %d, want 2", len(records))
	}
	if result.Records != 2 {
		t.Fatalf("result.Records = %d, want 2", result.Records)
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
			content:     `{"type":"event_msg","payload":{"session_id":"x","role":"user","message":"hi"` + "\n",
			wantRecords: false,
			wantWarning: true,
		},
		{
			name:        "wrong type for payload",
			content:     `{"type":"event_msg","payload":"not-an-object","message":"recoverable text"}` + "\n",
			wantRecords: true,
			wantWarning: false,
		},
		{
			name:        "missing fields no text",
			content:     `{"type":"response_item","payload":{"session_id":"x"}}` + "\n",
			wantRecords: false,
			wantWarning: true,
		},
		{
			name:        "empty file",
			content:     "",
			wantRecords: false,
			wantWarning: false,
		},
		{
			name:        "blank lines only",
			content:     "\n   \n\t\n",
			wantRecords: false,
			wantWarning: false,
		},
		{
			name:        "malformed then valid keeps going",
			content:     "not json\n" + `{"type":"event_msg","payload":{"session_id":"x","role":"user","message":"after malformed"}}` + "\n",
			wantRecords: true,
			wantWarning: true,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			path := writeTemp(t, "codex.jsonl", tc.content)
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
	// A line well over the 64KB initial scanner buffer must still parse.
	big := strings.Repeat("A", 256*1024)
	content := `{"type":"event_msg","payload":{"session_id":"big","role":"user","message":"` + big + `"}}` + "\n"
	path := writeTemp(t, "codex.jsonl", content)
	records, _ := generate(t, path, sources.Options{})
	if len(records) != 1 {
		t.Fatalf("huge line records = %d, want 1", len(records))
	}
	// Item text is capped by the parser, so it must be shorter than the raw input.
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
