// Command gate-ledger is the memory of `make gate` (docs/specs/gate-ledger-v0.md).
// A gate step is keyed on the content it reads, never on the worktree or commit
// it ran in, so a step whose inputs are byte-identical to a recorded pass is not
// executed again, from any worktree of the same user. It only ever refuses to
// repeat a step; it never narrows what a step tests.
//
// Usage:
//
//	gate-ledger run STEP -- CMD [ARG...]   run CMD unless the ledger holds a pass for STEP's inputs
//	gate-ledger go-test -- GO_TEST [FLAG...] run the resolved packages through Go's own test cache
//	                                        and the unresolved packages under a tree-keyed record
//	gate-ledger plan STEP...                print HIT or RUN per step; reads nothing but the ledger
//
// Every doubt is a RUN: a step with no declared scope, a worktree the tool cannot
// digest (skip-worktree or assume-unchanged entries, an ignored Go file, a failed
// git), a ledger directory it does not own, or a record it cannot read. A failed
// command leaves no record. CORVINT_GATE_LEDGER=off runs every command untouched;
// CORVINT_GATE_LEDGER_DIR overrides the per-user ledger directory.
package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"syscall"
	"time"
)

const (
	schema      = "gate-ledger/1"
	maxRecords  = 4096
	prefixOut   = "gate-ledger: "
	treeScope   = "tree"
	neverScope  = "never"
	unresolved  = "go-test-unresolved"
	toolingPath = "Makefile"
)

// tooling is read by every step: the gate's own definition and helpers.
var tooling = []string{toolingPath, "go.mod", "go.sum", "script/", "tools/"}

// scopes declares what each `make gate` step reads, as repository-relative
// directory prefixes (trailing slash), exact paths, or `*.ext` suffix globs over
// the whole tree. `tree` keys on the whole worktree; `never` always runs. A step
// absent here always runs and is never recorded (GL-V0-003).
var scopes = map[string][]string{
	"host-adapter-test":             {treeScope},
	"go-version":                    {},
	unresolved:                      {treeScope},
	"go-vet":                        {"*.go", "*.s", "*.c", "*.h"},
	"cross-vet":                     {"*.go", "*.s", "*.c", "*.h", "interop/"},
	"go-format-check":               {"*.go"},
	"go-format-test":                {},
	"go-archive-gate":               {neverScope},
	"go-archive-gate-test":          {},
	"interop-gate":                  {"interop/cem01-go/"},
	"spec-requirements-check":       {"docs/specs/"},
	"spec-requirements-test":        {},
	"requirement-definitions-check": {"docs/specs/"},
	"traceability-tests-check":      {"docs/specs/", "*.go"},
	"decision-numbers-check":        {"docs/decisions/"},
	"eol-policy-check":              {treeScope},
	"eol-policy-test":               {},
	"line-citations-check":          {treeScope},
	"line-citations-test":           {},
	"ci-least-privilege-check":      {".github/"},
	"ci-least-privilege-test":       {},
	"release-checklist-test":        {treeScope},
	"gate-receipt-test":             {},
	"error-code-ownership-check":    {treeScope},
	"error-code-ownership-test":     {},
	"cem-verify-pr-test":            {treeScope},
	"host-package-versions-check":   {treeScope},
	"host-package-versions-test":    {},
	"diagnostic-coverage-check":     {treeScope},
}

type record struct {
	Schema     string   `json:"schema"`
	Step       string   `json:"step"`
	Key        string   `json:"key"`
	Tree       string   `json:"tree"`
	RecordedAt string   `json:"recorded_at"`
	DurationMS int64    `json:"duration_ms"`
	Host       string   `json:"host"`
	Packages   []string `json:"packages,omitempty"`
}

// ledger is one invocation's view: the repository root, the worktree digest and
// the per-user record directory. A nil digest or an empty dir means every step runs.
type ledger struct {
	root      string
	tree      string
	entries   []treeEntry
	digestErr string
	dir       string
	dirErr    string
	identity  string
	stdout    io.Writer
	stderr    io.Writer
}

type treeEntry struct {
	path string
	line string
}

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

func run(args []string, stdout, stderr io.Writer) int {
	if len(args) < 1 {
		return usage(stderr)
	}
	switch args[0] {
	case "run":
		step, command, ok := splitCommand(args[1:])
		if !ok {
			return usage(stderr)
		}
		return open(stdout, stderr).runStep(step, nil, command)
	case "go-test":
		_, command, ok := splitCommand(append([]string{unresolved}, args[1:]...))
		if !ok {
			return usage(stderr)
		}
		return open(stdout, stderr).goTest(command)
	case "plan":
		if len(args) < 2 {
			return usage(stderr)
		}
		return open(stdout, stderr).plan(args[1:])
	}
	return usage(stderr)
}

func usage(stderr io.Writer) int {
	fmt.Fprintln(stderr, "usage: gate-ledger run STEP -- CMD [ARG...] | go-test -- GO_TEST [FLAG...] | plan STEP...")
	return 2
}

// splitCommand parses `STEP -- CMD ARG...`.
func splitCommand(args []string) (string, []string, bool) {
	if len(args) < 3 || args[1] != "--" {
		return "", nil, false
	}
	return args[0], args[2:], true
}

func open(stdout, stderr io.Writer) *ledger {
	l := &ledger{stdout: stdout, stderr: stderr}
	if os.Getenv("CORVINT_GATE_LEDGER") == "off" {
		l.dirErr = "CORVINT_GATE_LEDGER=off"
		l.stdout = io.Discard
		return l
	}
	root, err := gitOutput("", "rev-parse", "--show-toplevel")
	if err != nil {
		l.dirErr = "not inside a git worktree: " + err.Error()
		return l
	}
	l.root = strings.TrimSpace(root)
	l.identity = toolIdentity()
	l.dir, l.dirErr = ledgerDirectory()
	l.tree, l.entries, l.digestErr = worktreeDigest(l.root)
	return l
}

// plan prints one HIT or RUN line per step without executing or recording anything.
func (l *ledger) plan(steps []string) int {
	for _, step := range steps {
		key, reason := l.key(step, nil)
		if reason != "" {
			fmt.Fprintf(l.stdout, "%sRUN %s: %s\n", prefixOut, step, reason)
			continue
		}
		if _, hit := l.lookup(key); hit {
			fmt.Fprintf(l.stdout, "%sHIT %s %s\n", prefixOut, step, short(key))
		} else {
			fmt.Fprintf(l.stdout, "%sRUN %s: no recorded pass for %s\n", prefixOut, step, short(key))
		}
	}
	return 0
}

// runStep executes command unless a pass for step's current inputs is recorded,
// and records a pass when the command exits zero.
func (l *ledger) runStep(step string, packages []string, command []string) int {
	key, reason := l.key(step, packages)
	if reason != "" {
		fmt.Fprintf(l.stdout, "%sRUN %s: %s\n", prefixOut, step, reason)
		return execute(command)
	}
	if rec, hit := l.lookup(key); hit {
		fmt.Fprintf(l.stdout, "%sHIT %s %s (recorded %s on %s)\n", prefixOut, step, short(key), rec.RecordedAt, rec.Host)
		return 0
	}
	unlock, err := l.lock(key, step)
	if err != nil {
		fmt.Fprintf(l.stdout, "%sRUN %s: ledger lock unavailable: %s\n", prefixOut, step, err)
		return execute(command)
	}
	defer unlock()
	if rec, hit := l.lookup(key); hit {
		fmt.Fprintf(l.stdout, "%sHIT %s %s (recorded %s on %s while waiting)\n", prefixOut, step, short(key), rec.RecordedAt, rec.Host)
		return 0
	}
	fmt.Fprintf(l.stdout, "%sRUN %s: no recorded pass for %s\n", prefixOut, step, short(key))
	started := time.Now()
	code := execute(command)
	if code != 0 {
		return code
	}
	if err := l.record(record{Schema: schema, Step: step, Key: key, Tree: l.tree, RecordedAt: started.UTC().Format(time.RFC3339), DurationMS: time.Since(started).Milliseconds(), Host: hostname(), Packages: packages}); err != nil {
		fmt.Fprintf(l.stderr, "%sRECORD %s failed: %s\n", prefixOut, step, err)
		return 0
	}
	fmt.Fprintf(l.stdout, "%sRECORD %s %s\n", prefixOut, step, short(key))
	return 0
}

// goTest partitions `./...` into the packages whose reads their literals bound,
// which Go's own test cache may answer, and the unresolved packages, which run
// with -count=1 under a tree-keyed record (GL-V0-004). When the partition cannot
// be computed every package runs with -count=1 under the tree key.
func (l *ledger) goTest(goTest []string) int {
	resolved, unresolvedPkgs, reason := l.partition()
	if reason != "" {
		fmt.Fprintf(l.stdout, "%sPARTITION unavailable: %s; every package runs uncached\n", prefixOut, reason)
		return l.runStep(unresolved, []string{"./..."}, append(append([]string{}, goTest...), "-count=1", "./..."))
	}
	fmt.Fprintf(l.stdout, "%sPARTITION %d resolved packages through the Go test cache, %d unresolved under the tree key\n", prefixOut, len(resolved), len(unresolvedPkgs))
	if len(resolved) > 0 {
		if code := execute(append(append([]string{}, goTest...), resolved...)); code != 0 {
			return code
		}
	}
	if len(unresolvedPkgs) == 0 {
		return 0
	}
	return l.runStep(unresolved, unresolvedPkgs, append(append(append([]string{}, goTest...), "-count=1"), unresolvedPkgs...))
}

func (l *ledger) partition() (resolved, unresolvedPkgs []string, reason string) {
	if l.root == "" {
		return nil, nil, "no repository root"
	}
	module, err := modulePath(filepath.Join(l.root, "go.mod"))
	if err != nil {
		return nil, nil, err.Error()
	}
	listed, err := output(l.root, "go", "list", "./...")
	if err != nil {
		return nil, nil, "go list ./... failed: " + err.Error()
	}
	flagged, err := output(l.root, "go", "run", "./tools/gate-affected-select", "-unresolved", module, l.root)
	if err != nil {
		return nil, nil, "gate-affected-select -unresolved failed: " + err.Error()
	}
	isUnresolved := map[string]bool{}
	for _, line := range strings.Split(flagged, "\n") {
		if !strings.HasPrefix(line, "unresolved ") {
			continue
		}
		pkg, _, ok := strings.Cut(strings.TrimPrefix(line, "unresolved "), ": ")
		if !ok {
			return nil, nil, "unreadable unresolved line: " + line
		}
		isUnresolved[pkg] = true
	}
	for _, pkg := range strings.Fields(listed) {
		if !strings.HasPrefix(pkg, module+"/") && pkg != module {
			return nil, nil, "go list named a package outside the module: " + pkg
		}
		if isUnresolved[pkg] {
			unresolvedPkgs = append(unresolvedPkgs, pkg)
		} else {
			resolved = append(resolved, pkg)
		}
	}
	sort.Strings(resolved)
	sort.Strings(unresolvedPkgs)
	return resolved, unresolvedPkgs, ""
}

// key derives the content key for step, or a reason the step must run.
func (l *ledger) key(step string, packages []string) (string, string) {
	if l.dirErr != "" {
		return "", l.dirErr
	}
	scope, ok := scopes[step]
	if !ok {
		return "", "no declared input scope"
	}
	if len(scope) == 1 && scope[0] == neverScope {
		return "", "always runs"
	}
	if l.digestErr != "" {
		return "", l.digestErr
	}
	inputs := l.inputs(scope)
	h := sha256.New()
	fmt.Fprintf(h, "%s\nstep %s\ntool %s\nenv GO_TEST_TIMEOUT=%s\ninputs %s\n", schema, step, l.identity, os.Getenv("GO_TEST_TIMEOUT"), inputs)
	for _, pkg := range packages {
		fmt.Fprintf(h, "package %s\n", pkg)
	}
	return hex.EncodeToString(h.Sum(nil)), ""
}

// inputs digests the tree entries scope selects; a tree scope is the tree id itself.
func (l *ledger) inputs(scope []string) string {
	for _, s := range scope {
		if s == treeScope {
			return "tree " + l.tree
		}
	}
	h := sha256.New()
	for _, entry := range l.entries {
		if inScope(entry.path, scope) || inScope(entry.path, tooling) {
			fmt.Fprintf(h, "%s\n", entry.line)
		}
	}
	return "entries " + hex.EncodeToString(h.Sum(nil))
}

func inScope(path string, scope []string) bool {
	for _, s := range scope {
		switch {
		case strings.HasPrefix(s, "*."):
			if strings.HasSuffix(path, s[1:]) {
				return true
			}
		case strings.HasSuffix(s, "/"):
			if strings.HasPrefix(path, s) {
				return true
			}
		default:
			if path == s {
				return true
			}
		}
	}
	return false
}

// worktreeDigest writes the exact worktree (tracked and untracked, ignored
// excluded) as a tree object through a private index and lists its entries.
// It refuses a worktree `git status` cannot see fully (GL-V0-005).
func worktreeDigest(root string) (string, []treeEntry, string) {
	if flagged, err := gitOutput(root, "ls-files", "-v"); err != nil {
		return "", nil, "git ls-files -v failed: " + err.Error()
	} else if hasFlaggedEntry(flagged) {
		return "", nil, "a tracked file is skip-worktree or assume-unchanged"
	}
	ignored, err := gitOutput(root, "ls-files", "--others", "--ignored", "--exclude-standard", "--", "*.go")
	if err != nil {
		return "", nil, "git ls-files --ignored failed: " + err.Error()
	}
	if name := compiledIgnored(ignored); name != "" {
		return "", nil, "an ignored Go file is inside the build: " + name
	}
	indexPath, err := gitOutput(root, "rev-parse", "--git-path", "index")
	if err != nil {
		return "", nil, "git rev-parse --git-path index failed: " + err.Error()
	}
	private, err := os.CreateTemp("", "corvint-gate-ledger-index.")
	if err != nil {
		return "", nil, "private index: " + err.Error()
	}
	defer os.Remove(private.Name())
	indexPath = strings.TrimSpace(indexPath)
	if !filepath.IsAbs(indexPath) {
		indexPath = filepath.Join(root, indexPath)
	}
	if err := copyIndex(indexPath, private); err != nil {
		return "", nil, "private index: " + err.Error()
	}
	env := "GIT_INDEX_FILE=" + private.Name()
	if _, err := gitOutputEnv(root, env, "add", "-A", "--", "."); err != nil {
		return "", nil, "git add -A into the private index failed: " + err.Error()
	}
	tree, err := gitOutputEnv(root, env, "write-tree")
	if err != nil {
		return "", nil, "git write-tree failed: " + err.Error()
	}
	tree = strings.TrimSpace(tree)
	listing, err := gitOutput(root, "ls-tree", "-r", "-z", tree)
	if err != nil {
		return "", nil, "git ls-tree failed: " + err.Error()
	}
	var entries []treeEntry
	for _, line := range strings.Split(listing, "\x00") {
		if line == "" {
			continue
		}
		_, path, ok := strings.Cut(line, "\t")
		if !ok {
			return "", nil, "unreadable ls-tree entry"
		}
		entries = append(entries, treeEntry{path: path, line: line})
	}
	return tree, entries, ""
}

// hasFlaggedEntry mirrors script/gate-receipt: `ls-files -v` tags a
// skip-worktree entry S and an assume-unchanged entry in lower case.
func hasFlaggedEntry(listing string) bool {
	for _, line := range strings.Split(listing, "\n") {
		if line == "" {
			continue
		}
		if c := line[0]; c == 'S' || (c >= 'a' && c <= 'z') {
			return true
		}
	}
	return false
}

// compiledIgnored returns an ignored .go path that `go ./...` would still compile:
// one outside `.`- and `_`-prefixed and testdata directories.
func compiledIgnored(listing string) string {
	for _, name := range strings.Split(listing, "\n") {
		if name == "" {
			continue
		}
		hidden := false
		for _, part := range strings.Split(filepath.Dir(name), "/") {
			if part == "testdata" || strings.HasPrefix(part, ".") || strings.HasPrefix(part, "_") {
				hidden = true
			}
		}
		if !hidden {
			return name
		}
	}
	return ""
}

// copyIndex copies the repository index into the private one. A linked
// worktree's index lives under the main repository's `.git/worktrees/`, which
// `--git-path` reports absolute. A repository with no index yet gets none, so
// git creates the private index itself rather than reading an empty file.
func copyIndex(from string, to *os.File) error {
	defer to.Close()
	source, err := os.Open(from)
	if errors.Is(err, os.ErrNotExist) {
		return os.Remove(to.Name())
	}
	if err != nil {
		return err
	}
	defer source.Close()
	_, err = io.Copy(to, source)
	return err
}

// ledgerDirectory returns the per-user record directory once it is a private,
// owned, non-symlink directory, or the reason it cannot be used.
func ledgerDirectory() (string, string) {
	dir := os.Getenv("CORVINT_GATE_LEDGER_DIR")
	if dir == "" {
		base := os.Getenv("XDG_CACHE_HOME")
		if base == "" {
			home, err := os.UserHomeDir()
			if err != nil {
				return "", "no home directory: " + err.Error()
			}
			base = filepath.Join(home, ".cache")
		}
		dir = filepath.Join(base, "corvint", "gate-ledger")
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", "ledger directory: " + err.Error()
	}
	info, err := os.Lstat(dir)
	if err != nil {
		return "", "ledger directory: " + err.Error()
	}
	if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return "", "ledger directory is not a plain directory"
	}
	if info.Mode().Perm()&0o077 != 0 {
		return "", "ledger directory is group- or world-accessible"
	}
	if stat, ok := info.Sys().(*syscall.Stat_t); ok && int(stat.Uid) != os.Getuid() {
		return "", "ledger directory is owned by another user"
	}
	return dir, ""
}

func (l *ledger) recordPath(key string) string { return filepath.Join(l.dir, key+".json") }

func (l *ledger) lookup(key string) (record, bool) {
	data, err := os.ReadFile(l.recordPath(key))
	if err != nil {
		return record{}, false
	}
	var rec record
	if err := json.Unmarshal(data, &rec); err != nil || rec.Schema != schema || rec.Key != key {
		return record{}, false
	}
	return rec, true
}

func (l *ledger) record(rec record) error {
	data, err := json.Marshal(rec)
	if err != nil {
		return err
	}
	tmp, err := os.CreateTemp(l.dir, "record.")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())
	if _, err := tmp.Write(append(data, '\n')); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Rename(tmp.Name(), l.recordPath(rec.Key)); err != nil {
		return err
	}
	return l.prune()
}

// prune keeps the ledger bounded at maxRecords, dropping the oldest records.
func (l *ledger) prune() error {
	matches, err := filepath.Glob(filepath.Join(l.dir, "*.json"))
	if err != nil || len(matches) <= maxRecords {
		return err
	}
	type aged struct {
		name string
		at   time.Time
	}
	var records []aged
	for _, name := range matches {
		info, err := os.Lstat(name)
		if err != nil {
			continue
		}
		records = append(records, aged{name, info.ModTime()})
	}
	sort.Slice(records, func(i, j int) bool { return records[i].at.Before(records[j].at) })
	for _, old := range records[:len(records)-maxRecords] {
		os.Remove(old.name)
	}
	return nil
}

// lock serialises concurrent runs of one key so the step executes once and the
// waiter reads the record.
func (l *ledger) lock(key, step string) (func(), error) {
	f, err := os.OpenFile(filepath.Join(l.dir, key+".lock"), os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return nil, err
	}
	if err := syscall.Flock(int(f.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		fmt.Fprintf(l.stdout, "%sWAIT %s: another run holds %s\n", prefixOut, step, short(key))
		if err := syscall.Flock(int(f.Fd()), syscall.LOCK_EX); err != nil {
			f.Close()
			return nil, err
		}
	}
	return func() { syscall.Flock(int(f.Fd()), syscall.LOCK_UN); f.Close() }, nil
}

// execute runs command with inherited stdio and returns its exit status; a
// command killed by a signal exits 128+signal.
func execute(command []string) int {
	cmd := exec.Command(command[0], command[1:]...)
	cmd.Stdin, cmd.Stdout, cmd.Stderr = os.Stdin, os.Stdout, os.Stderr
	cmd.Env = withoutJobserver(os.Environ())
	err := cmd.Run()
	if err == nil {
		return 0
	}
	var exit *exec.ExitError
	if errors.As(err, &exit) {
		if status, ok := exit.Sys().(syscall.WaitStatus); ok && status.Signaled() {
			return 128 + int(status.Signal())
		}
		return exit.ExitCode()
	}
	fmt.Fprintf(os.Stderr, "%s%s: %s\n", prefixOut, command[0], err)
	return 1
}

// withoutJobserver drops make's jobserver handles from MAKEFLAGS: os/exec closes the
// descriptors they name, so a sub-make would otherwise warn and fall back anyway.
func withoutJobserver(env []string) []string {
	for i, entry := range env {
		if !strings.HasPrefix(entry, "MAKEFLAGS=") {
			continue
		}
		var kept []string
		for _, flag := range strings.Fields(strings.TrimPrefix(entry, "MAKEFLAGS=")) {
			if !strings.HasPrefix(flag, "--jobserver-") {
				kept = append(kept, flag)
			}
		}
		env[i] = "MAKEFLAGS=" + strings.Join(kept, " ")
	}
	return env
}

func toolIdentity() string {
	out, err := output("", "go", "env", "GOVERSION", "GOOS", "GOARCH")
	if err != nil {
		return "unknown"
	}
	return strings.Join(strings.Fields(out), " ")
}

func modulePath(goMod string) (string, error) {
	data, err := os.ReadFile(goMod)
	if err != nil {
		return "", err
	}
	for _, line := range strings.Split(string(data), "\n") {
		if fields := strings.Fields(line); len(fields) >= 2 && fields[0] == "module" {
			return fields[1], nil
		}
	}
	return "", errors.New("go.mod declares no module path")
}

func gitOutput(dir string, args ...string) (string, error) {
	return gitOutputEnv(dir, "", args...)
}

func gitOutputEnv(dir, env string, args ...string) (string, error) {
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	if env != "" {
		cmd.Env = append(os.Environ(), env)
	}
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("git %s: %s", args[0], strings.TrimSpace(stderr.String()))
	}
	return string(out), nil
}

func output(dir string, name string, args ...string) (string, error) {
	cmd := exec.Command(name, args...)
	cmd.Dir = dir
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("%s: %s", strings.TrimSpace(stderr.String()), err)
	}
	return string(out), nil
}

func hostname() string {
	name, err := os.Hostname()
	if err != nil {
		return "unknown"
	}
	return name
}

func short(key string) string { return key[:12] }
