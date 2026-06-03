package app

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/openclaw/agenttrail/internal/sources"
	"github.com/openclaw/agenttrail/internal/sources/claude"
	"github.com/openclaw/agenttrail/internal/sources/codex"
	"github.com/openclaw/agenttrail/internal/sources/openclaw"
)

const Version = "0.1.0"

type commandDef struct {
	name        string
	description string
	defaultRoot func() string
	generator   sources.Generator
}

var commands = map[string]commandDef{
	"codex": {
		name:        "codex",
		description: "export Codex session JSONL",
		defaultRoot: func() string { return homePath(".codex", "sessions") },
		generator:   codex.Generate,
	},
	"claude": {
		name:        "claude",
		description: "export Claude project JSONL",
		defaultRoot: func() string { return homePath(".claude", "projects") },
		generator:   claude.Generate,
	},
	"openclaw": {
		name:        "openclaw",
		description: "export OpenClaw agent session JSONL",
		defaultRoot: func() string { return homePath(".openclaw", "agents") },
		generator:   openclaw.Generate,
	},
}

func Run(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 || args[0] == "-h" || args[0] == "--help" || args[0] == "help" {
		printHelp(stdout)
		return 0
	}
	switch args[0] {
	case "version":
		fmt.Fprintf(stdout, "agenttrail %s\n", Version)
		return 0
	case "discover":
		if err := runDiscover(args[1:], stdout); err != nil {
			fmt.Fprintln(stderr, "error:", err)
			return 1
		}
		return 0
	case "doctor":
		if err := runDoctor(args[1:], stdout); err != nil {
			fmt.Fprintln(stderr, "error:", err)
			return 1
		}
		return 0
	default:
		def, ok := commands[args[0]]
		if !ok {
			fmt.Fprintln(stderr, "error: unknown command", args[0])
			return 1
		}
		if err := runExport(def, args[1:], stdout, stderr); err != nil {
			fmt.Fprintln(stderr, "error:", err)
			return 1
		}
		return 0
	}
}

func printHelp(w io.Writer) {
	fmt.Fprintln(w, "agenttrail exports local agent session logs to logspine.adapter.v1 JSONL.")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "Usage:")
	fmt.Fprintln(w, "  agenttrail discover [--json]")
	fmt.Fprintln(w, "  agenttrail doctor [--json]")
	fmt.Fprintln(w, "  agenttrail codex [path-or-dir] --out <file|-> [--limit N] [--since DATE] [--dry-run] [--redact paths,secrets] [--json]")
	fmt.Fprintln(w, "  agenttrail claude [path-or-dir] --out <file|-> [--limit N] [--since DATE] [--dry-run] [--redact paths,secrets] [--json]")
	fmt.Fprintln(w, "  agenttrail openclaw [path-or-dir] --out <file|-> [--limit N] [--since DATE] [--dry-run] [--redact paths,secrets] [--json]")
	fmt.Fprintln(w, "  agenttrail version")
}

func runExport(def commandDef, args []string, stdout, stderr io.Writer) error {
	fs := flag.NewFlagSet(def.name, flag.ContinueOnError)
	fs.SetOutput(stderr)
	outPath := fs.String("out", "-", "output file or - for stdout")
	limit := fs.Int("limit", 0, "maximum records to emit")
	since := fs.String("since", "", "minimum item timestamp as RFC3339 or YYYY-MM-DD")
	dryRun := fs.Bool("dry-run", false, "scan and summarize without writing records")
	redact := fs.String("redact", "", "comma-separated redactions: paths,secrets")
	jsonSummary := fs.Bool("json", false, "write a JSON summary after export")
	pathArg, flagArgs, err := splitPathAndFlags(args)
	if err != nil {
		return err
	}
	if err := fs.Parse(flagArgs); err != nil {
		return err
	}
	path := def.defaultRoot()
	if pathArg != "" {
		path = pathArg
	}
	if path == "" {
		return errors.New("path is required")
	}
	redactions, err := parseRedactions(*redact)
	if err != nil {
		return err
	}
	var out io.Writer = stdout
	var file *os.File
	if *dryRun {
		out = io.Discard
	} else if *outPath != "-" {
		if err := os.MkdirAll(filepath.Dir(*outPath), 0o755); err != nil && filepath.Dir(*outPath) != "." {
			return err
		}
		f, err := os.Create(*outPath)
		if err != nil {
			return err
		}
		defer f.Close()
		file = f
		out = f
	}
	result, err := def.generator(path, sources.Options{
		Limit:         *limit,
		Since:         *since,
		RedactPaths:   redactions["paths"],
		RedactSecrets: redactions["secrets"],
	}, out)
	if err != nil {
		return err
	}
	if file != nil {
		if err := file.Close(); err != nil {
			return err
		}
	}
	if *jsonSummary {
		target := stdout
		if *outPath == "-" && !*dryRun {
			target = stderr
		}
		warnings := result.Warnings
		if warnings == nil {
			warnings = []string{}
		}
		return writeJSON(target, map[string]any{
			"source":       def.name,
			"path":         path,
			"out":          exportTarget(*outPath, *dryRun),
			"dry_run":      *dryRun,
			"redactions":   sortedRedactions(redactions),
			"records":      result.Records,
			"warnings":     warnings,
			"files":        result.Files,
			"generated_at": time.Now().UTC().Format(time.RFC3339Nano),
		})
	}
	if *dryRun {
		fmt.Fprintf(stdout, "%s dry-run: records=%d warnings=%d files=%d\n", def.name, result.Records, len(result.Warnings), len(result.Files))
		return nil
	}
	if len(result.Warnings) > 0 {
		for _, warning := range result.Warnings {
			fmt.Fprintln(stderr, "warning:", warning)
		}
	}
	return nil
}

func parseRedactions(raw string) (map[string]bool, error) {
	out := map[string]bool{}
	for _, part := range strings.Split(raw, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		switch part {
		case "paths", "secrets":
			out[part] = true
		default:
			return nil, fmt.Errorf("unsupported redaction %q", part)
		}
	}
	return out, nil
}

func sortedRedactions(redactions map[string]bool) []string {
	out := []string{}
	for _, key := range []string{"paths", "secrets"} {
		if redactions[key] {
			out = append(out, key)
		}
	}
	return out
}

func exportTarget(outPath string, dryRun bool) string {
	if dryRun {
		return ""
	}
	return outPath
}

func splitPathAndFlags(args []string) (string, []string, error) {
	var path string
	var flags []string
	valueFlags := map[string]bool{"-out": true, "--out": true, "-limit": true, "--limit": true, "-since": true, "--since": true, "-redact": true, "--redact": true}
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if strings.HasPrefix(arg, "-") {
			flags = append(flags, arg)
			if strings.Contains(arg, "=") {
				continue
			}
			if valueFlags[arg] {
				if i+1 >= len(args) {
					return "", nil, fmt.Errorf("missing value for %s", arg)
				}
				i++
				flags = append(flags, args[i])
			}
			continue
		}
		if path != "" {
			return "", nil, fmt.Errorf("unexpected argument %q", arg)
		}
		path = arg
	}
	return path, flags, nil
}

type DoctorReport struct {
	GeneratedAt string           `json:"generated_at"`
	OK          bool             `json:"ok"`
	Sources     []DiscoveredRoot `json:"sources"`
	Warnings    []string         `json:"warnings"`
}

func runDoctor(args []string, stdout io.Writer) error {
	fs := flag.NewFlagSet("doctor", flag.ContinueOnError)
	jsonOut := fs.Bool("json", false, "write JSON")
	if err := fs.Parse(args); err != nil {
		return err
	}
	discovery := Discover()
	report := DoctorReport{
		GeneratedAt: time.Now().UTC().Format(time.RFC3339Nano),
		OK:          true,
		Sources:     discovery.Sources,
		Warnings:    []string{},
	}
	for _, source := range discovery.Sources {
		switch source.Status {
		case "ready", "missing", "blocked_on_samples":
		default:
			report.OK = false
			report.Warnings = append(report.Warnings, fmt.Sprintf("%s status is %s", source.Kind, source.Status))
		}
		if source.Exists && source.JSONLFiles == 0 && source.Status == "ready" {
			report.Warnings = append(report.Warnings, fmt.Sprintf("%s root exists but has no JSONL files", source.Kind))
		}
	}
	if *jsonOut {
		return writeJSON(stdout, report)
	}
	if report.OK {
		fmt.Fprintln(stdout, "ok")
	}
	for _, source := range report.Sources {
		fmt.Fprintf(stdout, "%s\t%s\tjsonl=%d\t%s\n", source.Kind, source.Root, source.JSONLFiles, source.Status)
	}
	for _, warning := range report.Warnings {
		fmt.Fprintln(stdout, "warning:", warning)
	}
	return nil
}

type Discovery struct {
	GeneratedAt string           `json:"generated_at"`
	Sources     []DiscoveredRoot `json:"sources"`
}

type DiscoveredRoot struct {
	Kind        string `json:"kind"`
	Root        string `json:"root"`
	Exists      bool   `json:"exists"`
	JSONLFiles  int    `json:"jsonl_files"`
	Status      string `json:"status"`
	Description string `json:"description"`
}

func runDiscover(args []string, stdout io.Writer) error {
	fs := flag.NewFlagSet("discover", flag.ContinueOnError)
	jsonOut := fs.Bool("json", false, "write JSON")
	if err := fs.Parse(args); err != nil {
		return err
	}
	discovery := Discover()
	if *jsonOut {
		return writeJSON(stdout, discovery)
	}
	for _, source := range discovery.Sources {
		fmt.Fprintf(stdout, "%s\t%s\tjsonl=%d\t%s\n", source.Kind, source.Root, source.JSONLFiles, source.Status)
	}
	return nil
}

func Discover() Discovery {
	specs := []DiscoveredRoot{
		{Kind: "codex", Root: homePath(".codex", "sessions"), Description: "Codex session JSONL"},
		{Kind: "claude", Root: homePath(".claude", "projects"), Description: "Claude project JSONL"},
		{Kind: "openclaw", Root: homePath(".openclaw", "agents"), Description: "OpenClaw agent session JSONL"},
		{Kind: "opencode", Root: homePath(".opencode"), Description: "OpenCode logs, parser pending samples"},
		{Kind: "hermes", Root: homePath(".hermes"), Description: "Hermes logs, parser blocked on samples"},
	}
	for i := range specs {
		specs[i] = enrichDiscovery(specs[i])
	}
	return Discovery{
		GeneratedAt: time.Now().UTC().Format(time.RFC3339Nano),
		Sources:     specs,
	}
}

func enrichDiscovery(root DiscoveredRoot) DiscoveredRoot {
	if root.Root == "" {
		root.Status = "unavailable"
		return root
	}
	info, err := os.Stat(root.Root)
	if err != nil || !info.IsDir() {
		root.Status = "missing"
		return root
	}
	root.Exists = true
	files, err := sources.ListJSONLFiles(root.Root, sources.DefaultInclude)
	if err != nil {
		root.Status = "error"
		return root
	}
	root.JSONLFiles = len(files)
	switch root.Kind {
	case "opencode", "hermes":
		root.Status = "blocked_on_samples"
	default:
		root.Status = "ready"
	}
	return root
}

func writeJSON(w io.Writer, v any) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(v)
}

func homePath(parts ...string) string {
	home, err := os.UserHomeDir()
	if err != nil || strings.TrimSpace(home) == "" {
		return ""
	}
	all := append([]string{home}, parts...)
	return filepath.Join(all...)
}
