// host-lifecycle-v1 runs the nine host lifecycle cases of HLQ-V1
// (docs/specs/host-lifecycle-qualification-v1.md) for one host tuple against installed
// executables, in a private temporary workspace. Plugin hosts run their own package manager with
// HOME, CLAUDE_CONFIG_DIR and CODEX_HOME pointed at a fresh directory, so the operator's host
// installation, credentials and settings are never read or written. It never publishes, signs,
// tags, or writes into the repository it is run from.
package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strings"
	"time"
)

// The order is the V1-0016 acceptance order (HLQ-V1-002).
var caseOrder = []string{"install", "discovery", "context", "expansion", "change", "frontier", "degradation", "upgrade", "uninstall"}

// coreVerbs is the CCF-V1-001 Core set the discovery case expects in root help.
var coreVerbs = []string{"init", "adopt", "index", "query", "context", "impact", "affected", "prove", "cem", "ocm", "frontier", "dogfood"}

const (
	envelopeBegin = "BEGIN CORVINT REPOSITORY DATA"
	envelopeEnd   = "END CORVINT REPOSITORY DATA"
	sessionID     = "hlq-session"
	changedSource = "package fx\n\n// Add returns a+b.\nfunc Add(a, b int) int { return b + a }\n"
)

// hostProfile is what differs between the two plugin hosts; the cases themselves are shared.
type hostProfile struct {
	executable     string
	sourceDir      string // under integrations/
	manifest       string // plugin manifest path under the plugin directory
	marketplace    string // marketplace name the source declares
	selector       string
	homeVariable   string
	homeDirectory  string // under the private HOME
	cacheDirectory string // installed plugin root under the host home, before the version
	events         []string
	install        [][]string
	reinstall      []string
	uninstall      [][]string
	// shellCommand is set when the host registers each hook as one shell command string.
	shellCommand bool
}

var profiles = map[string]hostProfile{
	"claude-code": {
		executable:     "claude",
		sourceDir:      "claude-code",
		manifest:       ".claude-plugin/plugin.json",
		marketplace:    "corvint",
		selector:       "corvint@corvint",
		homeVariable:   "CLAUDE_CONFIG_DIR",
		homeDirectory:  ".claude",
		cacheDirectory: "plugins/cache/corvint/corvint",
		events:         []string{"PostCompact", "PostToolUse", "PreCompact", "SessionEnd", "SessionStart", "Stop", "UserPromptSubmit"},
		install:        [][]string{{"plugin", "marketplace", "add", "SOURCE"}, {"plugin", "install", "corvint@corvint", "--scope", "user"}},
		reinstall:      []string{"plugin", "update", "corvint@corvint", "--scope", "user"},
		uninstall: [][]string{
			{"plugin", "disable", "corvint@corvint", "--scope", "user"},
			{"plugin", "enable", "corvint@corvint", "--scope", "user"},
			{"plugin", "uninstall", "corvint@corvint", "--scope", "user"},
			{"plugin", "marketplace", "remove", "corvint"},
		},
	},
	"codex": {
		executable:     "codex",
		sourceDir:      "codex",
		manifest:       ".codex-plugin/plugin.json",
		marketplace:    "corvint-source",
		selector:       "corvint@corvint-source",
		homeVariable:   "CODEX_HOME",
		homeDirectory:  ".codex",
		cacheDirectory: "plugins/cache/corvint-source/corvint",
		events:         []string{"SessionEnd", "SessionStart", "Stop", "UserPromptSubmit"},
		install:        [][]string{{"plugin", "marketplace", "add", "SOURCE"}, {"plugin", "add", "corvint@corvint-source"}},
		reinstall:      []string{"plugin", "add", "corvint@corvint-source"},
		uninstall:      [][]string{{"plugin", "remove", "corvint@corvint-source"}, {"plugin", "marketplace", "remove", "corvint-source"}},
		shellCommand:   true,
	},
}

type result struct {
	name, status, detail string
}

type runner struct {
	host           string
	profile        hostProfile
	source         string // checkout root holding integrations/
	current, base  string
	work, bin      string
	home, hostHome string
	fixture        string
	environment    []string
	hostExecutable string
	hostVersion    string
	adapterVersion string
	pluginRoot     string
	hooks          map[string][]string
	results        []result
}

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

func run(arguments []string, stdout, stderr io.Writer) int {
	flags := flag.NewFlagSet("host-lifecycle-v1", flag.ContinueOnError)
	flags.SetOutput(stderr)
	host := flags.String("host", "", "cli, claude-code or codex")
	current := flags.String("corvint", "", "the corvint executable under qualification")
	base := flags.String("base-corvint", "", "the N-1 corvint executable for the upgrade case")
	source := flags.String("source", ".", "checkout whose integrations/ supplies the plugin package")
	report := flags.String("report", "", "file the report is also written to")
	if err := flags.Parse(arguments); err != nil || flags.NArg() != 0 || *current == "" {
		fmt.Fprintln(stderr, "usage: host-lifecycle-v1 --host cli|claude-code|codex --corvint FILE [--base-corvint FILE] [--source DIR] [--report FILE]")
		return 2
	}
	_, plugin := profiles[*host]
	if *host != "cli" && !plugin {
		fmt.Fprintln(stderr, "host-lifecycle-v1: unknown host", *host)
		return 2
	}
	r, err := newRunner(*host, *current, *base, *source)
	if err != nil {
		fmt.Fprintln(stderr, "host-lifecycle-v1:", err)
		return 2
	}
	defer os.RemoveAll(r.work)
	if *host == "cli" {
		r.runCLI()
	} else {
		r.runPlugin()
	}
	text := r.render()
	fmt.Fprint(stdout, text)
	if *report != "" {
		if err := os.WriteFile(*report, []byte(text), 0o644); err != nil {
			fmt.Fprintln(stderr, "host-lifecycle-v1:", err)
			return 2
		}
	}
	return summaryExit(r.results)
}

func newRunner(host, current, base, source string) (*runner, error) {
	current, err := filepath.Abs(current)
	if err != nil {
		return nil, err
	}
	if base != "" {
		if base, err = filepath.Abs(base); err != nil {
			return nil, err
		}
	}
	if source, err = filepath.Abs(source); err != nil {
		return nil, err
	}
	work, err := os.MkdirTemp("", "corvint-host-lifecycle-")
	if err != nil {
		return nil, err
	}
	if work, err = filepath.EvalSymlinks(work); err != nil {
		return nil, err
	}
	r := &runner{host: host, profile: profiles[host], source: source, current: current, base: base, work: work,
		bin: filepath.Join(work, "bin"), home: filepath.Join(work, "home"), fixture: filepath.Join(work, "fixture")}
	for _, directory := range []string{r.bin, r.home} {
		if err := os.MkdirAll(directory, 0o755); err != nil {
			return nil, err
		}
	}
	gitPath, err := exec.LookPath("git")
	if err != nil {
		return nil, err
	}
	path := []string{r.bin, filepath.Dir(gitPath)}
	if host != "cli" {
		if r.hostExecutable, err = exec.LookPath(r.profile.executable); err != nil {
			return nil, err
		}
		path = append(path, filepath.Dir(r.hostExecutable))
		r.hostHome = filepath.Join(r.home, r.profile.homeDirectory)
		if err := os.MkdirAll(r.hostHome, 0o755); err != nil {
			return nil, err
		}
	}
	path = append(path, "/usr/bin", "/bin", "/usr/sbin", "/sbin")
	r.environment = []string{"PATH=" + strings.Join(path, ":"), "HOME=" + r.home, "TMPDIR=" + work, "LANG=C", "LC_ALL=C",
		"GIT_CONFIG_NOSYSTEM=1", "GIT_AUTHOR_NAME=Lifecycle", "GIT_AUTHOR_EMAIL=lifecycle@example.invalid",
		"GIT_COMMITTER_NAME=Lifecycle", "GIT_COMMITTER_EMAIL=lifecycle@example.invalid"}
	if host != "cli" {
		r.environment = append(r.environment, r.profile.homeVariable+"="+r.hostHome)
	}
	return r, nil
}

// exec runs argv with the private environment in directory and returns stdout, stderr and the
// exit code. Only a failure to start or a timeout is an error; a nonzero exit is data.
func (r *runner) exec(directory string, stdin []byte, extraEnvironment []string, argv ...string) (string, string, int, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()
	executable, err := r.lookPath(argv[0])
	if err != nil {
		return "", "", -1, err
	}
	command := exec.CommandContext(ctx, executable, argv[1:]...)
	command.Dir = directory
	command.Env = append(append([]string{}, r.environment...), extraEnvironment...)
	command.Stdin = bytes.NewReader(stdin)
	var stdout, stderr bytes.Buffer
	command.Stdout, command.Stderr = &stdout, &stderr
	err = command.Run()
	var exit *exec.ExitError
	if errors.As(err, &exit) {
		return stdout.String(), stderr.String(), exit.ExitCode(), nil
	}
	if err != nil {
		return stdout.String(), stderr.String(), -1, err
	}
	return stdout.String(), stderr.String(), 0, nil
}

// ok runs argv and requires exit 0.
func (r *runner) ok(directory string, argv ...string) (string, error) {
	stdout, stderr, code, err := r.exec(directory, nil, nil, argv...)
	if err != nil {
		return "", err
	}
	if code != 0 {
		return "", fmt.Errorf("%s exited %d: %s", strings.Join(argv, " "), code, firstLine(stderr+stdout))
	}
	return stdout, nil
}

func (r *runner) step(name string, body func() (string, error)) {
	detail, err := body()
	switch {
	case errors.Is(err, errNotRun):
		r.results = append(r.results, result{name, "NOT_RUN", detail})
	case err != nil:
		r.results = append(r.results, result{name, "FAIL", err.Error()})
	default:
		r.results = append(r.results, result{name, "PASS", detail})
	}
}

var errNotRun = errors.New("not run")

func (r *runner) installCorvint(source string) (string, error) {
	data, err := os.ReadFile(source)
	if err != nil {
		return "", err
	}
	target := filepath.Join(r.bin, "corvint")
	if err := os.WriteFile(target+".new", data, 0o755); err != nil {
		return "", err
	}
	if err := os.Rename(target+".new", target); err != nil {
		return "", err
	}
	version, err := r.ok(r.work, "corvint", "--version")
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(version), nil
}

// makeFixture commits a Go module, a governing AGENTS.md and an intent document with one
// requirement, so every Core verb the cases call has input to read.
func (r *runner) makeFixture() error {
	files := map[string]string{
		"go.mod":         "module example.com/fx\n\ngo 1.22\n",
		"add.go":         "package fx\n\n// Add returns a+b.\nfunc Add(a, b int) int { return a + b }\n",
		"AGENTS.md":      "# Fixture\n\nRun `go test ./...` before completing a change.\n",
		"docs/intent.md": "# Add\n\n## Requirements\n\n- `FIX-001`: Preserve the Add behaviour under review.\n\n## Next\n",
	}
	for name, body := range files {
		path := filepath.Join(r.fixture, name)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			return err
		}
		if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
			return err
		}
	}
	for _, argv := range [][]string{{"git", "init", "-q"}, {"git", "add", "-A"}, {"git", "commit", "-q", "-m", "fixture"}} {
		if _, err := r.ok(r.fixture, argv...); err != nil {
			return err
		}
	}
	return nil
}

func (r *runner) revision(spec string) (string, error) {
	out, err := r.ok(r.fixture, "git", "rev-parse", spec)
	return strings.TrimSpace(out), err
}

func (r *runner) worktreeClean() error {
	out, err := r.ok(r.fixture, "git", "status", "--porcelain")
	if err != nil {
		return err
	}
	if strings.TrimSpace(out) != "" {
		return fmt.Errorf("fixture worktree changed: %s", firstLine(out))
	}
	return nil
}

func (r *runner) setupFixture() error {
	if _, err := r.installCorvint(r.current); err != nil {
		return err
	}
	return r.makeFixture()
}

// ---- plain CLI tuple ----

func (r *runner) runCLI() {
	r.adapterVersion = "none"
	r.hostVersion = "none"
	r.step("install", func() (string, error) {
		version, err := r.installCorvint(r.current)
		if err != nil {
			return "", err
		}
		if !strings.HasPrefix(version, "Corvint ") {
			return "", fmt.Errorf("unexpected version line %q", version)
		}
		return version, r.makeFixture()
	})
	r.step("discovery", func() (string, error) {
		stdout, stderr, code, err := r.exec(r.work, nil, nil, "corvint", "help")
		if err != nil || code != 0 {
			return "", fmt.Errorf("corvint help exit %d: %v", code, err)
		}
		help := stdout + stderr
		if missing := missingCoreVerbs(help); len(missing) != 0 {
			return "", fmt.Errorf("root help Core section lacks %s", strings.Join(missing, ","))
		}
		return "root help lists the 12 CCF-V1-001 Core verbs", nil
	})
	r.step("context", func() (string, error) {
		document, err := r.jsonOK(r.fixture, "corvint", "context", "--task", "Explain add.go")
		if err != nil {
			return "", err
		}
		if document["tool"] != "context" || document["schema_version"] != float64(1) {
			return "", fmt.Errorf("context identifiers tool=%v schema_version=%v", document["tool"], document["schema_version"])
		}
		return "tool=context schema_version=1", nil
	})
	r.step("expansion", func() (string, error) {
		blob, err := r.revision("HEAD:add.go")
		if err != nil {
			return "", err
		}
		out, err := r.ok(r.fixture, "corvint", "query", "--task", "Explain add.go", "--limit", "3")
		if err != nil {
			return "", err
		}
		if !strings.Contains(out, `"path":"add.go"`) || !strings.Contains(out, blob) {
			return "", fmt.Errorf("query packet does not cite add.go at blob %s", blob)
		}
		return "query cites add.go at its HEAD blob", nil
	})
	r.step("change", func() (string, error) {
		if err := os.WriteFile(filepath.Join(r.fixture, "add.go"), []byte(changedSource), 0o644); err != nil {
			return "", err
		}
		document, err := r.jsonOK(r.fixture, "corvint", "impact", "add.go")
		restoreErr := r.restore()
		if err != nil {
			return "", err
		}
		if restoreErr != nil {
			return "", restoreErr
		}
		nested, _ := document["context"].(map[string]any)
		if document["tool"] != "impact" || nested["mode"] != "impact" {
			return "", fmt.Errorf("impact identifiers tool=%v mode=%v", document["tool"], nested["mode"])
		}
		return "impact on the edited add.go: tool=impact mode=impact", nil
	})
	r.step("frontier", r.cliFrontier)
	r.step("degradation", func() (string, error) {
		outside := filepath.Join(r.work, "not-a-repository")
		if err := os.MkdirAll(outside, 0o755); err != nil {
			return "", err
		}
		_, stderr, code, err := r.exec(outside, nil, nil, "corvint", "context", "--task", "Explain add.go")
		if err != nil {
			return "", err
		}
		var document map[string]any
		if code != 2 || json.Unmarshal([]byte(stderr), &document) != nil || document["ok"] != false || document["code"] != "invalid-arguments" {
			return "", fmt.Errorf("outside a repository: exit %d, stderr %q", code, firstLine(stderr))
		}
		entries, err := os.ReadDir(outside)
		if err != nil || len(entries) != 0 {
			return "", fmt.Errorf("the refused command wrote %d entries", len(entries))
		}
		return "outside a Git repository: exit 2, ok=false code=invalid-arguments, nothing written", nil
	})
	r.step("upgrade", func() (string, error) {
		if r.base == "" {
			return "no --base-corvint supplied", errNotRun
		}
		from, err := r.installCorvint(r.base)
		if err != nil {
			return "", err
		}
		if _, err := r.ok(r.fixture, "corvint", "index"); err != nil {
			return "", err
		}
		to, err := r.installCorvint(r.current)
		if err != nil {
			return "", err
		}
		if _, err := r.ok(r.fixture, "corvint", "index", "--if-stale"); err != nil {
			return "", err
		}
		if _, err := r.jsonOK(r.fixture, "corvint", "context", "--task", "Explain add.go"); err != nil {
			return "", err
		}
		return fmt.Sprintf("%s indexed, replaced by %s, index --if-stale and context succeed", from, to), nil
	})
	r.step("uninstall", func() (string, error) {
		if err := os.Remove(filepath.Join(r.bin, "corvint")); err != nil {
			return "", err
		}
		if _, _, _, err := r.exec(r.fixture, nil, nil, "corvint", "--version"); err == nil {
			return "", fmt.Errorf("corvint still resolves after removal")
		}
		if err := r.worktreeClean(); err != nil {
			return "", err
		}
		return "binary removed; fixture worktree unchanged", nil
	})
}

// cliFrontier commits one change in a clone, marks its hunk unknown, prepares the OCM over the
// fixture intent and requires a valid open frontier (exit 1, profile frontier/0).
func (r *runner) cliFrontier() (string, error) {
	clone := filepath.Join(r.work, "frontier")
	if _, err := r.ok(r.work, "git", "clone", "-q", r.fixture, clone); err != nil {
		return "", err
	}
	base, err := r.ok(clone, "git", "rev-parse", "HEAD")
	if err != nil {
		return "", err
	}
	base = strings.TrimSpace(base)
	if err := os.WriteFile(filepath.Join(clone, "add.go"), []byte(changedSource), 0o644); err != nil {
		return "", err
	}
	if _, err := r.ok(clone, "git", "commit", "-q", "-am", "change"); err != nil {
		return "", err
	}
	target, err := r.ok(clone, "git", "rev-parse", "HEAD")
	if err != nil {
		return "", err
	}
	target = strings.TrimSpace(target)
	for _, argv := range [][]string{
		{"corvint", "cem", "prepare", "--base", base, "--target", target},
		{"corvint", "cem", "mark", "--map", ".corvint/change.cem.json", "--hunk", "1", "--disposition", "unknown", "--reason", "insufficient-evidence"},
		{"corvint", "ocm", "prepare", "--target", target, "--intent", "docs/intent.md", "--expected-base", base},
	} {
		if _, err := r.ok(clone, argv...); err != nil {
			return "", err
		}
	}
	stdout, stderr, code, err := r.exec(clone, nil, nil, "corvint", "frontier", "--cem", ".corvint/change.cem.json",
		"--ocm", ".corvint/change.ocm.json", "--expected-base", base, "--target", target, "--json")
	if err != nil {
		return "", err
	}
	var document map[string]any
	if code != 1 || json.Unmarshal([]byte(stdout), &document) != nil {
		return "", fmt.Errorf("frontier exit %d: %s", code, firstLine(stderr+stdout))
	}
	if document["profile"] != "frontier/0" || document["frontierState"] != "OPEN" {
		return "", fmt.Errorf("frontier profile=%v state=%v", document["profile"], document["frontierState"])
	}
	return "unknown hunk yields frontier/0 OPEN, exit 1", nil
}

// ---- plugin host tuples ----

func (r *runner) runPlugin() {
	version, err := r.ok(r.work, r.hostExecutable, "--version")
	r.hostVersion = hostVersion(version)
	if err != nil {
		r.hostVersion = "unknown"
	}
	manifest, err := readManifestVersion(filepath.Join(r.pluginSource(), r.profile.manifest))
	r.adapterVersion = manifest
	if err != nil {
		r.adapterVersion = "unknown"
	}
	setupErr := r.setupFixture()
	r.step("install", func() (string, error) {
		if setupErr != nil {
			return "", setupErr
		}
		for _, argv := range r.profile.install {
			if _, err := r.hostCommand(substitute(argv, r.marketplaceSource())...); err != nil {
				return "", err
			}
		}
		listing, err := r.hostCommand("plugin", "list")
		if err != nil {
			return "", err
		}
		if !listed(listing, r.profile.selector) || !strings.Contains(listing, r.adapterVersion) {
			return "", fmt.Errorf("host listing does not show %s at %s", r.profile.selector, r.adapterVersion)
		}
		r.pluginRoot = filepath.Join(r.hostHome, r.profile.cacheDirectory, r.adapterVersion)
		files, err := sameTree(r.pluginSource(), r.pluginRoot)
		if err != nil {
			return "", err
		}
		return fmt.Sprintf("%s %s installed by the host package manager; %d files byte-equal to source", r.profile.selector, r.adapterVersion, files), nil
	})
	r.step("discovery", func() (string, error) {
		if r.pluginRoot == "" {
			return "", fmt.Errorf("install did not complete")
		}
		hooks, err := readHooks(filepath.Join(r.pluginRoot, "hooks", "hooks.json"), r.profile.shellCommand)
		if err != nil {
			return "", err
		}
		events := make([]string, 0, len(hooks))
		for event, argv := range hooks {
			if len(argv) < 3 || argv[0] != "corvint" || argv[1] != "adapter" || argv[2] != r.host {
				return "", fmt.Errorf("%s hook runs %q, not the corvint %s adapter", event, strings.Join(argv, " "), r.host)
			}
			events = append(events, event)
		}
		sort.Strings(events)
		if strings.Join(events, ",") != strings.Join(r.profile.events, ",") {
			return "", fmt.Errorf("registered hook events %v, expected %v", events, r.profile.events)
		}
		r.hooks = hooks
		skills, _ := filepath.Glob(filepath.Join(r.pluginRoot, "skills", "*", "SKILL.md"))
		if len(skills) == 0 {
			return "", fmt.Errorf("installed plugin carries no skill")
		}
		listing, err := r.hostCommand("plugin", "list")
		if err != nil {
			return "", err
		}
		if !strings.Contains(strings.ToLower(listing), "enabled") {
			return "", fmt.Errorf("host listing does not report the plugin enabled")
		}
		return fmt.Sprintf("host reports it enabled; hooks %s and %d skill", strings.Join(events, ","), len(skills)), nil
	})
	r.step("context", func() (string, error) {
		receipt, _, err := r.contextHook("SessionStart", map[string]any{"source": "startup"})
		if err != nil {
			return "", err
		}
		head, err := r.revision("HEAD")
		if err != nil {
			return "", err
		}
		repository, _ := receipt["repository"].(map[string]any)
		if repository["commitRevision"] != head || repository["worktreeState"] != "clean" {
			return "", fmt.Errorf("session-start receipt pins %v (%v), fixture HEAD is %s", repository["commitRevision"], repository["worktreeState"], head)
		}
		return fmt.Sprintf("SessionStart: enveloped %s receipt, support=%v, pinned to HEAD", receipt["profile"], receipt["support"]), nil
	})
	r.step("expansion", func() (string, error) {
		receipt, _, err := r.contextHook("UserPromptSubmit", map[string]any{"prompt": "Explain add.go"})
		if err != nil {
			return "", err
		}
		blob, err := r.revision("HEAD:add.go")
		if err != nil {
			return "", err
		}
		if !citesPath(receipt, "add.go", blob) {
			return "", fmt.Errorf("prompt receipt does not cite add.go at blob %s", blob)
		}
		return "UserPromptSubmit naming add.go returns task evidence at its HEAD blob", nil
	})
	r.step("change", r.pluginChange)
	r.step("frontier", r.pluginFrontier)
	r.step("degradation", func() (string, error) {
		output, err := r.hook("SessionStart", []byte("{not json"), nil)
		if err != nil {
			return "", err
		}
		var document map[string]any
		if json.Unmarshal([]byte(output), &document) != nil {
			return "", fmt.Errorf("malformed input produced non-JSON output %q", firstLine(output))
		}
		message, _ := document["systemMessage"].(string)
		if !strings.Contains(message, "degraded: malformed-hook-json") || !strings.Contains(message, "coding continues") {
			return "", fmt.Errorf("malformed input: systemMessage %q", message)
		}
		return "malformed hook input: exit 0 with visible malformed-hook-json degradation; coding continues", nil
	})
	r.step("upgrade", func() (string, error) {
		if r.base == "" {
			return "no --base-corvint supplied", errNotRun
		}
		from, err := r.installCorvint(r.base)
		if err != nil {
			return "", err
		}
		if _, _, err := r.contextHook("SessionStart", map[string]any{"source": "startup"}); err != nil {
			return "", fmt.Errorf("under %s: %w", from, err)
		}
		to, err := r.installCorvint(r.current)
		if err != nil {
			return "", err
		}
		if _, err := r.hostCommand(r.profile.reinstall...); err != nil {
			return "", err
		}
		if _, err := sameTree(r.pluginSource(), r.pluginRoot); err != nil {
			return "", err
		}
		if _, _, err := r.contextHook("SessionStart", map[string]any{"source": "startup"}); err != nil {
			return "", fmt.Errorf("under %s: %w", to, err)
		}
		return fmt.Sprintf("hooks serve context under %s and after replacement by %s; host reinstall keeps the package byte-equal", from, to), nil
	})
	r.step("uninstall", func() (string, error) {
		if r.pluginRoot == "" {
			return "", fmt.Errorf("install did not complete")
		}
		for _, argv := range r.profile.uninstall {
			if _, err := r.hostCommand(argv...); err != nil {
				return "", err
			}
			if argv[1] == "disable" {
				listing, err := r.hostCommand("plugin", "list")
				if err != nil {
					return "", err
				}
				if !strings.Contains(strings.ToLower(listing), "disabled") {
					return "", fmt.Errorf("host listing does not report the plugin disabled")
				}
			}
		}
		listing, err := r.hostCommand("plugin", "list")
		if err != nil {
			return "", err
		}
		if listed(listing, r.profile.selector) {
			return "", fmt.Errorf("host still lists %s", r.profile.selector)
		}
		// Claude Code retires an uninstalled version in place with an `.orphaned_at` marker and
		// deletes it later itself; any other surviving root is a failed uninstall.
		retired, removal := "", "installed plugin root removed"
		if _, err := os.Stat(r.pluginRoot); err == nil {
			if _, err := os.Stat(filepath.Join(r.pluginRoot, ".orphaned_at")); err != nil {
				return "", fmt.Errorf("installed plugin root %s remains without a host retirement marker", r.pluginRoot)
			}
			retired, removal = r.pluginRoot, "installed plugin root retired by the host with .orphaned_at for deferred host cleanup"
		}
		residue, err := corvintResidue(r.hostHome, retired)
		if err != nil {
			return "", err
		}
		if len(residue) != 0 {
			return "", fmt.Errorf("host home retains corvint state: %s", strings.Join(residue, ", "))
		}
		if err := r.worktreeClean(); err != nil {
			return "", err
		}
		steps := make([]string, 0, len(r.profile.uninstall))
		for _, argv := range r.profile.uninstall {
			steps = append(steps, strings.Join(argv[1:3], " "))
		}
		return "host " + strings.Join(steps, ", ") + " succeed; " + removal + "; no other corvint state in the host home; fixture unchanged", nil
	})
}

// pluginChange edits add.go; Claude Code must emit a PostToolUse harness receipt, and on both hosts
// the next prompt receipt must report exactly the one dirty tracked path.
func (r *runner) pluginChange() (string, error) {
	if r.hooks == nil {
		return "", fmt.Errorf("discovery did not complete")
	}
	path := filepath.Join(r.fixture, "add.go")
	if err := os.WriteFile(path, []byte(changedSource), 0o644); err != nil {
		return "", err
	}
	defer r.restore()
	detail := ""
	if _, registered := r.hooks["PostToolUse"]; registered {
		payload := r.payload("PostToolUse", map[string]any{"tool_name": "Edit", "tool_use_id": "hlq-tool",
			"tool_input": map[string]any{"file_path": path}, "tool_response": map[string]any{"filePath": path, "success": true}})
		output, err := r.hook("PostToolUse", payload, nil)
		if err != nil {
			return "", err
		}
		if !strings.Contains(output, "harness-receipt:sha256:") {
			return "", fmt.Errorf("PostToolUse output lacks a harness receipt: %q", firstLine(output))
		}
		detail = "PostToolUse emits a harness receipt; "
	}
	receipt, _, err := r.contextHook("UserPromptSubmit", map[string]any{"prompt": "Review the change to add.go"})
	if err != nil {
		return "", err
	}
	repository, _ := receipt["repository"].(map[string]any)
	if repository["dirtyPathCount"] != float64(1) || repository["worktreeState"] == "clean" {
		return "", fmt.Errorf("after the edit the receipt reports %v dirty paths, state %v", repository["dirtyPathCount"], repository["worktreeState"])
	}
	return detail + fmt.Sprintf("next prompt receipt reports 1 dirty path, state %v", repository["worktreeState"]), nil
}

// pluginFrontier checks the Stop lifecycle point: an unenrolled Stop releases, an enrolled
// incomplete Stop blocks once naming the unavailable Frontier authority, and the recursive Stop
// releases. The enrollment is cancelled afterwards.
func (r *runner) pluginFrontier() (string, error) {
	if r.hooks == nil {
		return "", fmt.Errorf("discovery did not complete")
	}
	stop := func(active bool) (map[string]any, error) {
		output, err := r.hook("Stop", r.payload("Stop", map[string]any{"stop_hook_active": active, "last_assistant_message": "done"}), nil)
		if err != nil {
			return nil, err
		}
		var document map[string]any
		if err := json.Unmarshal([]byte(output), &document); err != nil {
			return nil, fmt.Errorf("Stop output %q is not JSON", firstLine(output))
		}
		return document, nil
	}
	released, err := stop(false)
	if err != nil {
		return "", err
	}
	if _, decided := released["decision"]; decided {
		return "", fmt.Errorf("unenrolled Stop returned decision %v", released["decision"])
	}
	head, err := r.revision("HEAD")
	if err != nil {
		return "", err
	}
	plan := filepath.Join(r.work, "plan.json")
	body := fmt.Sprintf(`{"base":%q,"checks":[{"argv":["true"],"id":"noop","timeoutSeconds":10}],"intents":["add.go"]}`, head)
	if err := os.WriteFile(plan, []byte(body), 0o600); err != nil {
		return "", err
	}
	key, environment, err := r.sessionKey()
	if err != nil {
		return "", err
	}
	begin := append([]string{"corvint", "dogfood", "begin", "--plan", plan}, key...)
	if stdout, stderr, code, err := r.exec(r.fixture, nil, environment, begin...); err != nil || code != 0 {
		return "", fmt.Errorf("dogfood begin exit %d: %v %s", code, err, firstLine(stderr+stdout))
	}
	defer r.exec(r.fixture, nil, environment, append([]string{"corvint", "dogfood", "cancel"}, key...)...)
	blocked, err := stop(false)
	if err != nil {
		return "", err
	}
	reason, _ := blocked["reason"].(string)
	if blocked["decision"] != "block" || !strings.Contains(reason, "Frontier authority remains unavailable") {
		return "", fmt.Errorf("enrolled incomplete Stop returned decision %v", blocked["decision"])
	}
	recursive, err := stop(true)
	if err != nil {
		return "", err
	}
	if _, decided := recursive["decision"]; decided {
		return "", fmt.Errorf("recursive Stop returned decision %v", recursive["decision"])
	}
	return "unenrolled Stop releases; enrolled incomplete Stop blocks once naming unavailable Frontier authority; recursive Stop releases", nil
}

// sessionKey returns the dogfood key arguments and environment that bind an enrollment to the
// hook session: Claude Code prints its explicit key in SessionStart guidance; Codex hashes the
// thread id from the environment (docs/DOGFOOD.md).
func (r *runner) sessionKey() ([]string, []string, error) {
	if r.host == "codex" {
		return nil, []string{"CODEX_THREAD_ID=" + sessionID}, nil
	}
	_, text, err := r.contextHook("SessionStart", map[string]any{"source": "startup"})
	if err != nil {
		return nil, nil, err
	}
	key := sessionKeyPattern.FindStringSubmatch(text)
	if key == nil {
		return nil, nil, fmt.Errorf("SessionStart guidance names no session key")
	}
	return []string{"--session-key", key[1]}, nil, nil
}

var sessionKeyPattern = regexp.MustCompile(`"--session-key\\?",\\?"([0-9a-f]{64})\\?"`)

func (r *runner) payload(event string, fields map[string]any) []byte {
	document := map[string]any{"hook_event_name": event, "session_id": sessionID, "cwd": r.fixture, "transcript_path": nil}
	for name, value := range fields {
		document[name] = value
	}
	data, _ := json.Marshal(document)
	return data
}

// hook runs the installed registration for event with payload on stdin; a hook must exit 0.
func (r *runner) hook(event string, payload []byte, environment []string) (string, error) {
	argv, ok := r.hooks[event]
	if !ok {
		return "", fmt.Errorf("no %s hook registered", event)
	}
	stdout, stderr, code, err := r.exec(r.fixture, payload, environment, argv...)
	if err != nil {
		return "", err
	}
	if code != 0 {
		return "", fmt.Errorf("%s hook exited %d: %s", event, code, firstLine(stderr))
	}
	return stdout, nil
}

// contextHook runs a context-bearing hook and returns the enveloped receipt and the full
// additionalContext text.
func (r *runner) contextHook(event string, fields map[string]any) (map[string]any, string, error) {
	output, err := r.hook(event, r.payload(event, fields), nil)
	if err != nil {
		return nil, "", err
	}
	receipt, text, err := envelopedReceipt(output)
	if err != nil {
		return nil, "", fmt.Errorf("%s: %w", event, err)
	}
	return receipt, text, nil
}

// envelopedReceipt extracts the one JSON receipt line inside the repository-data envelope of a
// hook's additionalContext and requires it to be a successful corvint-dogfood-event/0 receipt.
func envelopedReceipt(output string) (map[string]any, string, error) {
	var document struct {
		HookSpecificOutput struct {
			AdditionalContext string `json:"additionalContext"`
		} `json:"hookSpecificOutput"`
	}
	if err := json.Unmarshal([]byte(output), &document); err != nil {
		return nil, "", fmt.Errorf("hook output is not JSON: %q", firstLine(output))
	}
	text := document.HookSpecificOutput.AdditionalContext
	begin := strings.Index(text, envelopeBegin+"\n")
	end := strings.Index(text, envelopeEnd)
	if begin < 0 || end < begin {
		return nil, text, fmt.Errorf("additionalContext carries no repository-data envelope")
	}
	var receipt map[string]any
	for _, line := range strings.Split(text[begin:end], "\n") {
		if strings.HasPrefix(line, "{") {
			if err := json.Unmarshal([]byte(line), &receipt); err != nil {
				return nil, text, fmt.Errorf("enveloped receipt is not JSON")
			}
			break
		}
	}
	if receipt == nil || receipt["ok"] != true || receipt["profile"] != "corvint-dogfood-event/0" || receipt["mutates"] != false {
		return nil, text, fmt.Errorf("enveloped receipt is not a successful non-mutating corvint-dogfood-event/0 receipt")
	}
	return receipt, text, nil
}

// citesPath reports whether the receipt's task evidence names path at blob.
func citesPath(receipt map[string]any, path, blob string) bool {
	nested, _ := receipt["context"].(map[string]any)
	evidence, _ := nested["task_evidence"].([]any)
	for _, item := range evidence {
		entry, _ := item.(map[string]any)
		if entry["path"] == path && entry["blob_hash"] == blob {
			return true
		}
	}
	return false
}

func (r *runner) hostCommand(argv ...string) (string, error) {
	return r.ok(r.work, append([]string{r.hostExecutable}, argv...)...)
}

func (r *runner) pluginSource() string {
	return filepath.Join(r.marketplaceSource(), "plugins", "corvint")
}

func (r *runner) marketplaceSource() string {
	return filepath.Join(r.source, "integrations", r.profile.sourceDir)
}

func (r *runner) restore() error {
	_, err := r.ok(r.fixture, "git", "checkout", "-q", "--", "add.go")
	return err
}

func (r *runner) jsonOK(directory string, argv ...string) (map[string]any, error) {
	out, err := r.ok(directory, argv...)
	if err != nil {
		return nil, err
	}
	var document map[string]any
	if err := json.Unmarshal([]byte(out), &document); err != nil {
		return nil, fmt.Errorf("%s output is not one JSON document", strings.Join(argv[:2], " "))
	}
	return document, nil
}

func substitute(argv []string, source string) []string {
	out := make([]string, len(argv))
	for index, value := range argv {
		if value == "SOURCE" {
			value = source
		}
		out[index] = value
	}
	return out
}

// readHooks maps each registered hook event to the argv its first command runs.
func readHooks(path string, shellCommand bool) (map[string][]string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var document struct {
		Hooks map[string][]struct {
			Hooks []struct {
				Command string   `json:"command"`
				Args    []string `json:"args"`
			} `json:"hooks"`
		} `json:"hooks"`
	}
	if err := json.Unmarshal(data, &document); err != nil {
		return nil, err
	}
	hooks := map[string][]string{}
	for event, groups := range document.Hooks {
		if len(groups) == 0 || len(groups[0].Hooks) == 0 {
			return nil, fmt.Errorf("%s registers no command", event)
		}
		command := groups[0].Hooks[0]
		argv := append([]string{command.Command}, command.Args...)
		if shellCommand {
			argv = strings.Fields(command.Command)
		}
		hooks[event] = argv
	}
	return hooks, nil
}

func readManifestVersion(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	var manifest struct {
		Version string `json:"version"`
	}
	if err := json.Unmarshal(data, &manifest); err != nil || manifest.Version == "" {
		return "", fmt.Errorf("%s has no version", path)
	}
	return manifest.Version, nil
}

// sameTree requires every regular file under source to exist byte-equal under installed, and
// installed to hold no other file except the host's own `.in_use` marker. It returns the count.
func sameTree(source, installed string) (int, error) {
	count := 0
	err := filepath.WalkDir(source, func(path string, entry os.DirEntry, err error) error {
		if err != nil || entry.IsDir() {
			return err
		}
		relative, _ := filepath.Rel(source, path)
		want, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		got, err := os.ReadFile(filepath.Join(installed, relative))
		if err != nil || !bytes.Equal(want, got) {
			return fmt.Errorf("installed %s differs from source", relative)
		}
		count++
		return nil
	})
	if err != nil {
		return 0, err
	}
	extra := 0
	err = filepath.WalkDir(installed, func(path string, entry os.DirEntry, err error) error {
		if err != nil || entry.IsDir() || entry.Name() == ".in_use" {
			return err
		}
		extra++
		return nil
	})
	if err != nil {
		return 0, err
	}
	if extra != count {
		return 0, fmt.Errorf("installed tree holds %d files, source %d", extra, count)
	}
	return count, nil
}

// corvintResidue lists host-home files whose path or content still names corvint, outside the
// host-retired plugin root when there is one.
func corvintResidue(home, retired string) ([]string, error) {
	var residue []string
	err := filepath.WalkDir(home, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() && path == retired {
			return filepath.SkipDir
		}
		if entry.IsDir() {
			return nil
		}
		relative, _ := filepath.Rel(home, path)
		if strings.Contains(strings.ToLower(relative), "corvint") {
			residue = append(residue, relative)
			return nil
		}
		info, err := entry.Info()
		if err != nil || info.Size() > 1<<20 {
			return err
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		if bytes.Contains(bytes.ToLower(data), []byte("corvint")) {
			residue = append(residue, relative+" (content)")
		}
		return nil
	})
	return residue, err
}

// listed reports whether a host plugin listing has a row for selector.
func listed(listing, selector string) bool {
	for _, line := range strings.Split(listing, "\n") {
		fields := strings.Fields(strings.TrimLeft(strings.TrimSpace(line), "❯ "))
		if len(fields) != 0 && fields[0] == selector && !strings.Contains(line, "not installed") {
			return true
		}
	}
	return false
}

// missingCoreVerbs returns the Core verbs absent from the root help's Core maturity section.
func missingCoreVerbs(help string) []string {
	start := strings.Index(help, "\n  Core (")
	end := strings.Index(help, "\n  Experimental,")
	if start < 0 || end < start {
		return coreVerbs
	}
	section := map[string]bool{}
	for _, word := range regexp.MustCompile(`[a-z-]+`).FindAllString(help[start:end], -1) {
		section[word] = true
	}
	var missing []string
	for _, verb := range coreVerbs {
		if !section[verb] {
			missing = append(missing, verb)
		}
	}
	return missing
}

func hostVersion(output string) string {
	match := regexp.MustCompile(`[0-9]+\.[0-9]+\.[0-9]+`).FindString(output)
	if match == "" {
		return "unknown"
	}
	return match
}

func firstLine(text string) string {
	text = strings.TrimSpace(text)
	if index := strings.IndexByte(text, '\n'); index >= 0 {
		text = text[:index]
	}
	if len(text) > 200 {
		text = text[:200]
	}
	return text
}

func (r *runner) render() string {
	var out strings.Builder
	surface := "plugin"
	if r.host == "cli" {
		surface = "native"
	}
	current, _ := r.versionOf(r.current)
	fmt.Fprintf(&out, "host-lifecycle-v1\thost=%s\tsurface=%s\thostVersion=%s\tadapterVersion=%s\tos=%s/%s\tcorvint=%s\n",
		r.host, surface, r.hostVersion, r.adapterVersion, runtime.GOOS, runtime.GOARCH, current)
	passed, failed, notRun := 0, 0, 0
	for _, name := range caseOrder {
		item := result{name, "NOT_RUN", "case did not execute"}
		for _, candidate := range r.results {
			if candidate.name == name {
				item = candidate
			}
		}
		switch item.status {
		case "PASS":
			passed++
		case "FAIL":
			failed++
		default:
			notRun++
		}
		fmt.Fprintf(&out, "case\t%s\t%s\t%s\n", item.name, item.status, item.detail)
	}
	status := "PASS"
	if failed != 0 || notRun != 0 {
		status = "FAIL"
	}
	fmt.Fprintf(&out, "SUMMARY\tstatus=%s\tpassed=%d\tfailed=%d\tnotRun=%d\n", status, passed, failed, notRun)
	return out.String()
}

func (r *runner) versionOf(path string) (string, error) {
	stdout, _, _, err := r.exec(r.work, nil, nil, path, "--version")
	return strings.TrimSpace(stdout), err
}

// summaryExit is 0 only when all nine cases passed.
func summaryExit(results []result) int {
	passed := map[string]bool{}
	for _, item := range results {
		if item.status == "PASS" {
			passed[item.name] = true
		}
	}
	for _, name := range caseOrder {
		if !passed[name] {
			return 1
		}
	}
	return 0
}

// lookPath resolves a bare command name against the private PATH, never the runner's own, so a
// removed or replaced executable is observed exactly as the host would observe it.
func (r *runner) lookPath(name string) (string, error) {
	if strings.ContainsRune(name, '/') {
		return name, nil
	}
	for _, variable := range r.environment {
		if !strings.HasPrefix(variable, "PATH=") {
			continue
		}
		for _, directory := range filepath.SplitList(strings.TrimPrefix(variable, "PATH=")) {
			candidate := filepath.Join(directory, name)
			if info, err := os.Stat(candidate); err == nil && info.Mode().IsRegular() && info.Mode()&0o111 != 0 {
				return candidate, nil
			}
		}
	}
	return "", fmt.Errorf("%s: not found on the private PATH", name)
}
