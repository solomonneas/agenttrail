package sources

import (
	"bufio"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/openclaw/agenttrail/internal/adapter"
)

type Options struct {
	Limit           int
	Since           string
	RedactPaths     bool
	RedactSecrets   bool
	RedactEmails    bool
	RedactURLs      bool
	RedactHostnames bool
}

type Result struct {
	Records  int        `json:"records"`
	Warnings []string   `json:"warnings"`
	Files    []FileScan `json:"files,omitempty"`
}

type Generator func(path string, opts Options, w io.Writer) (Result, error)

type FileScan struct {
	Path        string `json:"path"`
	Size        int64  `json:"size"`
	MTime       string `json:"mtime"`
	ContentHash string `json:"content_hash"`
	Records     int    `json:"records_generated"`
	Warnings    int    `json:"warnings"`
}

type RawEvent struct {
	Path    string
	Ordinal int64
	Line    []byte
	Object  map[string]any
}

func WalkJSONL(root string, include func(string) bool, each func(RawEvent) error) error {
	files, err := ListJSONLFiles(root, include)
	if err != nil {
		return err
	}
	for _, path := range files {
		if err := scanJSONL(path, each); err != nil {
			return err
		}
	}
	return nil
}

func ListJSONLFiles(root string, include func(string) bool) ([]string, error) {
	var files []string
	info, err := os.Stat(root)
	if err != nil {
		return nil, err
	}
	if !info.IsDir() {
		if include(root) {
			files = append(files, root)
		}
	} else {
		if err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if d.IsDir() {
				name := strings.ToLower(d.Name())
				if name == "skills-prompts" || name == "deleted" || name == "backup" || name == "backups" {
					return filepath.SkipDir
				}
				return nil
			}
			if include(path) {
				files = append(files, path)
			}
			return nil
		}); err != nil {
			return nil, err
		}
	}
	sort.Strings(files)
	return files, nil
}

func scanJSONL(path string, each func(RawEvent) error) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 0, 64*1024), 10*1024*1024)
	var ordinal int64
	for scanner.Scan() {
		ordinal++
		line := append([]byte(nil), scanner.Bytes()...)
		if strings.TrimSpace(string(line)) == "" {
			continue
		}
		var obj map[string]any
		if err := json.Unmarshal(line, &obj); err != nil {
			if err := each(RawEvent{Path: path, Ordinal: ordinal, Line: line, Object: map[string]any{"_warning": "malformed json: " + err.Error()}}); err != nil {
				return err
			}
			continue
		}
		if err := each(RawEvent{Path: path, Ordinal: ordinal, Line: line, Object: obj}); err != nil {
			return err
		}
	}
	return scanner.Err()
}

func DefaultInclude(path string) bool {
	name := strings.ToLower(filepath.Base(path))
	if !strings.HasSuffix(name, ".jsonl") {
		return false
	}
	if strings.Contains(name, "backup") || strings.Contains(name, ".bak") || strings.Contains(name, "deleted") {
		return false
	}
	if strings.HasSuffix(name, ".metadata.jsonl") || strings.HasSuffix(name, ".sidecar.jsonl") {
		return false
	}
	return true
}

type FileScanSet struct {
	files map[string]*FileScan
}

func NewFileScanSet(root string, include func(string) bool) (*FileScanSet, error) {
	paths, err := ListJSONLFiles(root, include)
	if err != nil {
		return nil, err
	}
	set := &FileScanSet{files: map[string]*FileScan{}}
	for _, path := range paths {
		info, err := os.Stat(path)
		if err != nil {
			return nil, err
		}
		hash, err := FileHash(path)
		if err != nil {
			return nil, err
		}
		set.files[path] = &FileScan{
			Path:        path,
			Size:        info.Size(),
			MTime:       info.ModTime().UTC().Format(time.RFC3339Nano),
			ContentHash: "sha256:" + hash,
		}
	}
	return set, nil
}

func (s *FileScanSet) Record(path string) {
	if scan := s.files[path]; scan != nil {
		scan.Records++
	}
}

func (s *FileScanSet) Warning(path string) {
	if scan := s.files[path]; scan != nil {
		scan.Warnings++
	}
}

func (s *FileScanSet) List() []FileScan {
	out := make([]FileScan, 0, len(s.files))
	for _, scan := range s.files {
		out = append(out, *scan)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Path < out[j].Path })
	return out
}

func FileHash(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

func WriteRecord(w io.Writer, rec adapter.Record) error {
	if rec.Artifacts == nil {
		rec.Artifacts = []adapter.Artifact{}
	}
	if rec.Links == nil {
		rec.Links = []adapter.Link{}
	}
	if rec.Relations == nil {
		rec.Relations = []adapter.Relation{}
	}
	b, err := json.Marshal(rec)
	if err != nil {
		return err
	}
	_, err = fmt.Fprintf(w, "%s\n", b)
	return err
}

func ApplyRedaction(rec *adapter.Record, opts Options) {
	if rec == nil || !opts.HasRedactions() {
		return
	}
	if opts.RedactPaths {
		rec.Raw.Path = RedactPath(rec.Raw.Path)
		rec.Collection.Metadata = redactMetadata(rec.Collection.Metadata, opts)
		rec.Item.Metadata = redactMetadata(rec.Item.Metadata, opts)
		if rec.Actor != nil {
			rec.Actor.Metadata = redactMetadata(rec.Actor.Metadata, opts)
		}
		for i := range rec.Artifacts {
			rec.Artifacts[i].Path = RedactPath(rec.Artifacts[i].Path)
			rec.Artifacts[i].Metadata = redactMetadata(rec.Artifacts[i].Metadata, opts)
		}
	}
	if opts.RedactSecrets || opts.RedactEmails || opts.RedactURLs || opts.RedactHostnames {
		rec.Item.Text = RedactText(rec.Item.Text, opts)
		for i := range rec.Artifacts {
			rec.Artifacts[i].Text = RedactText(rec.Artifacts[i].Text, opts)
			rec.Artifacts[i].URL = RedactText(rec.Artifacts[i].URL, opts)
		}
		for i := range rec.Links {
			rec.Links[i].URL = RedactText(rec.Links[i].URL, opts)
			rec.Links[i].Text = RedactText(rec.Links[i].Text, opts)
		}
		rec.Collection.Metadata = redactMetadata(rec.Collection.Metadata, opts)
		rec.Item.Metadata = redactMetadata(rec.Item.Metadata, opts)
		if rec.Actor != nil {
			rec.Actor.Metadata = redactMetadata(rec.Actor.Metadata, opts)
		}
		for i := range rec.Artifacts {
			rec.Artifacts[i].Metadata = redactMetadata(rec.Artifacts[i].Metadata, opts)
		}
	}
}

func (o Options) HasRedactions() bool {
	return o.RedactPaths || o.RedactSecrets || o.RedactEmails || o.RedactURLs || o.RedactHostnames
}

func RedactPath(path string) string {
	if strings.TrimSpace(path) == "" {
		return path
	}
	home, err := os.UserHomeDir()
	if err == nil && home != "" && (path == home || strings.HasPrefix(path, home+string(filepath.Separator))) {
		return "[redacted-home]" + strings.TrimPrefix(path, home)
	}
	if filepath.IsAbs(path) {
		return "[redacted-path]/" + filepath.Base(path)
	}
	if strings.Contains(path, "/") || strings.Contains(path, string(filepath.Separator)) {
		return "[redacted-path]/" + filepath.Base(path)
	}
	return path
}

var secretPatterns = []*regexp.Regexp{
	regexp.MustCompile(`(?i)(api[_-]?key|token|secret|password|authorization|bearer)(["'\s:=]+)[^"'\s,}]+`),
	regexp.MustCompile(`(?i)sk-[A-Za-z0-9_-]{16,}`),
	regexp.MustCompile(`(?i)(xox[baprs]-[A-Za-z0-9-]+)`),
}

var (
	emailPattern    = regexp.MustCompile(`(?i)\b[A-Z0-9._%+\-]+@[A-Z0-9.\-]+\.[A-Z]{2,}\b`)
	urlPattern      = regexp.MustCompile(`(?i)\bhttps?://[^\s"'<>]+`)
	hostnamePattern = regexp.MustCompile(`(?i)\b([a-z0-9](?:[a-z0-9-]{0,61}[a-z0-9])?\.)+(local|internal|lan|corp|home|dev|test|example|invalid|localhost|[a-z]{2,})\b`)
)

func RedactText(text string, opts Options) string {
	out := text
	if opts.RedactSecrets {
		out = RedactSecrets(out)
	}
	if opts.RedactEmails {
		out = emailPattern.ReplaceAllString(out, "[redacted-email]")
	}
	if opts.RedactURLs {
		out = urlPattern.ReplaceAllString(out, "[redacted-url]")
	}
	if opts.RedactHostnames {
		out = hostnamePattern.ReplaceAllString(out, "[redacted-host]")
	}
	return out
}

func RedactSecrets(text string) string {
	if text == "" {
		return text
	}
	out := text
	for _, pattern := range secretPatterns {
		out = pattern.ReplaceAllString(out, "$1$2[redacted-secret]")
	}
	return out
}

func redactMetadata(raw json.RawMessage, opts Options) json.RawMessage {
	if len(raw) == 0 {
		return raw
	}
	var value any
	if err := json.Unmarshal(raw, &value); err != nil {
		if opts.RedactSecrets || opts.RedactEmails || opts.RedactURLs || opts.RedactHostnames {
			return json.RawMessage(strconvJSON(RedactText(string(raw), opts)))
		}
		return raw
	}
	value = redactAny(value, opts, "")
	b, err := json.Marshal(value)
	if err != nil {
		return raw
	}
	return b
}

func redactAny(value any, opts Options, key string) any {
	switch v := value.(type) {
	case map[string]any:
		out := make(map[string]any, len(v))
		for k, child := range v {
			out[k] = redactAny(child, opts, k)
		}
		return out
	case []any:
		out := make([]any, len(v))
		for i, child := range v {
			out[i] = redactAny(child, opts, key)
		}
		return out
	case string:
		out := v
		if opts.RedactPaths && pathLikeKey(key) {
			out = RedactPath(out)
		}
		if opts.RedactSecrets {
			out = RedactText(out, opts)
		}
		if opts.RedactEmails || opts.RedactURLs || opts.RedactHostnames {
			out = RedactText(out, opts)
		}
		return out
	default:
		return value
	}
}

func pathLikeKey(key string) bool {
	key = strings.ToLower(key)
	for _, part := range []string{"path", "cwd", "dir", "workspace", "file"} {
		if strings.Contains(key, part) {
			return true
		}
	}
	return false
}

func strconvJSON(s string) string {
	b, _ := json.Marshal(s)
	return string(b)
}

func RawRef(ev RawEvent) adapter.RawRef {
	ordinal := ev.Ordinal
	return adapter.RawRef{
		Format:  "json",
		Hash:    "sha256:" + HashBytes(ev.Line),
		Path:    ev.Path,
		Ordinal: &ordinal,
	}
}

func HashBytes(b []byte) string {
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}

func StableID(parts ...string) string {
	h := sha256.New()
	for _, p := range parts {
		_, _ = io.WriteString(h, p)
		_, _ = io.WriteString(h, "\x00")
	}
	return hex.EncodeToString(h.Sum(nil))[:24]
}

func Metadata(m map[string]any) json.RawMessage {
	b, _ := json.Marshal(m)
	return b
}

func String(m map[string]any, keys ...string) string {
	for _, key := range keys {
		if v, ok := m[key]; ok {
			if s, ok := v.(string); ok && s != "" {
				return s
			}
		}
	}
	return ""
}

func NestedString(v any, keys ...string) string {
	cur := v
	for _, key := range keys {
		m, ok := cur.(map[string]any)
		if !ok {
			return ""
		}
		cur = m[key]
	}
	if s, ok := cur.(string); ok {
		return s
	}
	return ""
}

func TextFromAny(v any, max int) string {
	if max <= 0 {
		max = 4000
	}
	text := strings.TrimSpace(textFromAny(v, 0))
	if len(text) > max {
		return text[:max] + "\n[truncated]"
	}
	return text
}

func textFromAny(v any, depth int) string {
	if depth > 4 || v == nil {
		return ""
	}
	switch t := v.(type) {
	case string:
		return t
	case []any:
		var parts []string
		for _, x := range t {
			if s := textFromAny(x, depth+1); s != "" {
				parts = append(parts, s)
			}
		}
		return strings.Join(parts, "\n")
	case map[string]any:
		for _, key := range []string{"text", "content", "message", "prompt", "output", "stdout", "stderr", "result", "summary", "reasoning", "title", "arguments", "name", "call_id"} {
			if s := textFromAny(t[key], depth+1); s != "" {
				return s
			}
		}
		var parts []string
		for _, key := range []string{"role", "type", "name"} {
			if s, ok := t[key].(string); ok && s != "" {
				parts = append(parts, s)
			}
		}
		return strings.Join(parts, " ")
	default:
		return ""
	}
}

func ParseSince(s string) (time.Time, bool, error) {
	if strings.TrimSpace(s) == "" {
		return time.Time{}, false, nil
	}
	for _, layout := range []string{time.RFC3339Nano, time.RFC3339, "2006-01-02"} {
		t, err := time.Parse(layout, s)
		if err == nil {
			return t, true, nil
		}
	}
	return time.Time{}, false, errors.New("invalid --since date")
}

func KeepTimestamp(ts string, since time.Time, hasSince bool) bool {
	if !hasSince || ts == "" {
		return true
	}
	t, err := time.Parse(time.RFC3339Nano, ts)
	if err != nil {
		t, err = time.Parse(time.RFC3339, ts)
	}
	if err != nil {
		return true
	}
	return !t.Before(since)
}

func KindFromEvent(eventType, text string) string {
	lower := strings.ToLower(eventType + " " + text)
	switch {
	case strings.Contains(lower, "shell") || strings.Contains(lower, "bash") || strings.Contains(lower, "exec_command") || strings.Contains(lower, "command"):
		return "command"
	case strings.Contains(lower, "tool") || strings.Contains(lower, "function_call"):
		return "tool_call"
	case strings.Contains(lower, "file") || strings.Contains(lower, "patch") || strings.Contains(lower, "edit"):
		return "file_edit"
	case strings.Contains(lower, "error") || strings.Contains(lower, "failed") || strings.Contains(lower, "exception"):
		return "error"
	case strings.Contains(lower, "artifact") || strings.Contains(lower, "screenshot") || strings.Contains(lower, "output"):
		return "artifact"
	case strings.Contains(lower, "decision"):
		return "decision"
	case strings.Contains(lower, "message") || strings.Contains(lower, "prompt"):
		return "message"
	default:
		return "event"
	}
}

func ActorFromRole(sourceKind, role, eventType string) *adapter.Actor {
	role = strings.ToLower(strings.TrimSpace(role))
	actorType := "system"
	name := "system"
	switch role {
	case "user", "human":
		actorType, name = "human", "human"
	case "assistant":
		actorType, name = "assistant", "assistant"
	case "tool", "function":
		actorType, name = "tool", "tool"
	case "agent":
		actorType, name = "agent", "agent"
	case "system", "":
		if strings.Contains(strings.ToLower(eventType), "model") {
			actorType, name = "assistant", "assistant"
		} else if strings.Contains(strings.ToLower(eventType), "tool") || strings.Contains(strings.ToLower(eventType), "function") {
			actorType, name = "tool", "tool"
		}
	default:
		actorType, name = "agent", role
	}
	return &adapter.Actor{
		ExternalID: sourceKind + ":" + actorType + ":" + name,
		Type:       actorType,
		Name:       name,
	}
}

func ExtractArtifacts(itemID string, m map[string]any) []adapter.Artifact {
	var out []adapter.Artifact
	if m == nil {
		return out
	}
	add := func(kind, path, url, text string) {
		if path == "" && url == "" && text == "" {
			return
		}
		out = append(out, adapter.Artifact{
			ExternalID: StableID(itemID, kind, path, url, text),
			Kind:       kind,
			Path:       path,
			URL:        url,
			Text:       TextFromAny(text, 4000),
			Hash:       "sha256:" + HashBytes([]byte(path+url+text)),
		})
	}
	for _, key := range []string{"file_path", "filepath", "path"} {
		if s := String(m, key); s != "" {
			add("file", s, "", "")
		}
	}
	for _, key := range []string{"url", "uri", "link"} {
		if s := String(m, key); s != "" {
			add("url", "", s, "")
		}
	}
	for _, key := range []string{"cmd", "command", "shell_command"} {
		if s := TextFromAny(m[key], 4000); s != "" {
			add("command", "", "", s)
		}
	}
	for _, key := range []string{"stdout", "stderr", "output", "log"} {
		if s := TextFromAny(m[key], 4000); s != "" {
			add("log", "", "", s)
		}
	}
	if arr, ok := m["artifacts"].([]any); ok {
		for _, v := range arr {
			if artifact, ok := v.(map[string]any); ok {
				kind := String(artifact, "kind", "type")
				if kind == "" {
					kind = "artifact"
				}
				add(kind, String(artifact, "path", "file_path"), String(artifact, "url"), TextFromAny(artifact["text"], 4000))
			}
		}
	}
	return out
}
