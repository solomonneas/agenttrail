package sources

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/escoffier-labs/stationtrail/internal/adapter"
)

func TestStableIDIsDeterministicAndScoped(t *testing.T) {
	a := StableID("x", "y", "z")
	b := StableID("x", "y", "z")
	if a != b {
		t.Fatalf("StableID not deterministic: %q != %q", a, b)
	}
	if len(a) != 24 {
		t.Fatalf("StableID length = %d, want 24", len(a))
	}
	// Null-byte separation must distinguish differently-split inputs.
	if StableID("ab", "c") == StableID("a", "bc") {
		t.Fatalf("StableID collided across different part boundaries")
	}
}

func TestKindFromEvent(t *testing.T) {
	cases := []struct {
		eventType string
		text      string
		want      string
	}{
		{"exec_command", "", "command"},
		{"function_call", "", "tool_call"},
		{"file_edit", "", "file_edit"},
		{"", "operation failed with error", "error"},
		{"", "screenshot artifact captured", "artifact"},
		{"", "made a decision", "decision"},
		{"", "user prompt", "message"},
		{"random", "nothing notable", "event"},
	}
	for _, tc := range cases {
		if got := KindFromEvent(tc.eventType, tc.text); got != tc.want {
			t.Fatalf("KindFromEvent(%q,%q) = %q, want %q", tc.eventType, tc.text, got, tc.want)
		}
	}
}

func TestActorFromRole(t *testing.T) {
	cases := []struct {
		role      string
		eventType string
		wantType  string
	}{
		{"user", "", "human"},
		{"human", "", "human"},
		{"assistant", "", "assistant"},
		{"tool", "", "tool"},
		{"agent", "", "agent"},
		{"system", "", "system"},
		{"", "model_change", "assistant"},
		{"", "tool_call", "tool"},
		{"reviewer", "", "agent"},
	}
	for _, tc := range cases {
		actor := ActorFromRole("codex", tc.role, tc.eventType)
		if actor == nil {
			t.Fatalf("nil actor for role %q", tc.role)
		}
		if actor.Type != tc.wantType {
			t.Fatalf("ActorFromRole(%q,%q).Type = %q, want %q", tc.role, tc.eventType, actor.Type, tc.wantType)
		}
		if !strings.HasPrefix(actor.ExternalID, "codex:") {
			t.Fatalf("actor external id not scoped: %q", actor.ExternalID)
		}
	}
}

func TestParseSince(t *testing.T) {
	if _, has, err := ParseSince(""); err != nil || has {
		t.Fatalf("empty since should be no-op: has=%v err=%v", has, err)
	}
	if _, has, err := ParseSince("2026-06-01"); err != nil || !has {
		t.Fatalf("date should parse: has=%v err=%v", has, err)
	}
	if _, _, err := ParseSince("not-a-date"); err == nil {
		t.Fatalf("expected error for invalid since")
	}
}

func TestKeepTimestamp(t *testing.T) {
	since, _, err := ParseSince("2026-06-03")
	if err != nil {
		t.Fatal(err)
	}
	if !KeepTimestamp("2026-06-04T00:00:00Z", since, true) {
		t.Fatalf("later timestamp should be kept")
	}
	if KeepTimestamp("2026-06-01T00:00:00Z", since, true) {
		t.Fatalf("earlier timestamp should be dropped")
	}
	if !KeepTimestamp("", since, true) {
		t.Fatalf("empty timestamp should be kept")
	}
	if !KeepTimestamp("garbage", since, true) {
		t.Fatalf("unparseable timestamp should be kept")
	}
}

func TestTextFromAnyTruncates(t *testing.T) {
	long := strings.Repeat("z", 100)
	out := TextFromAny(long, 10)
	if !strings.HasPrefix(out, strings.Repeat("z", 10)) || !strings.Contains(out, "[truncated]") {
		t.Fatalf("expected truncation marker, got %q", out)
	}
	nested := map[string]any{"content": []any{"a", map[string]any{"text": "b"}}}
	if got := TextFromAny(nested, 0); !strings.Contains(got, "a") || !strings.Contains(got, "b") {
		t.Fatalf("nested text extraction failed: %q", got)
	}
}

func TestRedactPath(t *testing.T) {
	if got := RedactPath("/etc/secret/config.yaml"); got != "[redacted-path]/config.yaml" {
		t.Fatalf("absolute path redaction = %q", got)
	}
	if got := RedactPath("relative/file.txt"); got != "[redacted-path]/file.txt" {
		t.Fatalf("relative path redaction = %q", got)
	}
	if got := RedactPath("bare"); got != "bare" {
		t.Fatalf("bare token should be untouched: %q", got)
	}
}

func TestRedactTextProfiles(t *testing.T) {
	opts := Options{RedactSecrets: true, RedactEmails: true, RedactURLs: true, RedactHostnames: true}
	text := "token=abc123 slack xoxb-1234567890-abcdef email demo@example.com url https://host.example.com/p host build.internal"
	out := RedactText(text, opts)
	for _, forbidden := range []string{"abc123", "xoxb-1234567890-abcdef", "demo@example.com", "https://host.example.com/p", "build.internal"} {
		if strings.Contains(out, forbidden) {
			t.Fatalf("redaction leaked %q in %q", forbidden, out)
		}
	}
}

func TestApplyRedactionRedactsRawPathAndText(t *testing.T) {
	rec := adapter.Record{
		Item: adapter.Item{Text: "contact demo@example.com"},
		Raw:  adapter.RawRef{Path: "/home/demo/secret.jsonl"},
	}
	ApplyRedaction(&rec, Options{RedactPaths: true, RedactEmails: true})
	if strings.Contains(rec.Raw.Path, "secret.jsonl") && !strings.Contains(rec.Raw.Path, "[redacted") {
		t.Fatalf("raw path not redacted: %q", rec.Raw.Path)
	}
	if strings.Contains(rec.Item.Text, "demo@example.com") {
		t.Fatalf("item text email not redacted: %q", rec.Item.Text)
	}
}

func TestWalkJSONLSkipsExcludedDirs(t *testing.T) {
	root := t.TempDir()
	mustWrite := func(rel, content string) {
		p := filepath.Join(root, rel)
		if err := os.MkdirAll(filepath.Dir(p), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	mustWrite("keep/a.jsonl", `{"k":1}`+"\n")
	mustWrite("backup/b.jsonl", `{"k":2}`+"\n")
	mustWrite("deleted/c.jsonl", `{"k":3}`+"\n")

	var seen []string
	err := WalkJSONL(root, DefaultInclude, func(ev RawEvent) error {
		seen = append(seen, ev.Path)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(seen) != 1 || !strings.Contains(seen[0], filepath.Join("keep", "a.jsonl")) {
		t.Fatalf("expected only keep/a.jsonl, got %v", seen)
	}
}

func TestScanJSONLEmitsMalformedWarning(t *testing.T) {
	root := t.TempDir()
	p := filepath.Join(root, "x.jsonl")
	if err := os.WriteFile(p, []byte("not json\n{\"ok\":true}\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	var warnings, objects int
	err := WalkJSONL(root, DefaultInclude, func(ev RawEvent) error {
		if w, _ := ev.Object["_warning"].(string); w != "" {
			warnings++
		} else {
			objects++
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if warnings != 1 || objects != 1 {
		t.Fatalf("warnings=%d objects=%d, want 1 and 1", warnings, objects)
	}
}

func TestDefaultIncludeFilters(t *testing.T) {
	cases := map[string]bool{
		"session.jsonl":          true,
		"session.metadata.jsonl": false,
		"session.sidecar.jsonl":  false,
		"backup.jsonl":           false,
		"notes.txt":              false,
		"deleted.jsonl":          false,
	}
	for name, want := range cases {
		if got := DefaultInclude(filepath.Join("/root", name)); got != want {
			t.Fatalf("DefaultInclude(%q) = %v, want %v", name, got, want)
		}
	}
}

func TestWriteRecordFillsEmptyArrays(t *testing.T) {
	var buf bytes.Buffer
	rec := adapter.Record{
		Schema:     adapter.SchemaV1,
		Source:     adapter.Source{Kind: "codex"},
		Collection: adapter.Collection{ExternalID: "c", Kind: "agent_session"},
		Item:       adapter.Item{ExternalID: "i", Kind: "message"},
	}
	if err := WriteRecord(&buf, rec); err != nil {
		t.Fatal(err)
	}
	var decoded map[string]json.RawMessage
	if err := json.Unmarshal(bytes.TrimSpace(buf.Bytes()), &decoded); err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{"artifacts", "links", "relations"} {
		if string(decoded[key]) != "[]" {
			t.Fatalf("%s = %s, want []", key, decoded[key])
		}
	}
}

func TestExtractArtifacts(t *testing.T) {
	m := map[string]any{
		"file_path": "/tmp/out.txt",
		"command":   "go test ./...",
		"stdout":    "ok",
		"artifacts": []any{map[string]any{"kind": "image", "url": "https://x/y.png"}},
	}
	arts := ExtractArtifacts("item-1", m)
	kinds := map[string]bool{}
	for _, a := range arts {
		kinds[a.Kind] = true
		if a.ExternalID == "" || a.Hash == "" {
			t.Fatalf("artifact missing id or hash: %#v", a)
		}
	}
	for _, want := range []string{"file", "command", "log", "image"} {
		if !kinds[want] {
			t.Fatalf("missing %q artifact, got kinds %v", want, kinds)
		}
	}
}
