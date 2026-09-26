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
	"path"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

const (
	schema      = "gate-ledger/1"
	maxRecords  = 4096
	prefixOut   = "gate-ledger: "
	treeScope   = "tree"
	neverScope  = "never"
	unresolved  = "go-test-unresolved"
	packageStep = "go-test-package"
	toolingPath = "Makefile"
)

// listFields are the `go list -json` fields the per-package bound reads: the
// files compiled or embedded into a package's test binary and its dependencies.
const listFields = "ImportPath,Dir,Module,Deps,GoFiles,CgoFiles,IgnoredGoFiles,TestGoFiles,XTestGoFiles,EmbedFiles,TestEmbedFiles,XTestEmbedFiles,SFiles,CFiles,HFiles,CXXFiles,MFiles,FFiles,SwigFiles,SwigCXXFiles,SysoFiles"

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
	Bound      string   `json:"bound,omitempty"` // how a per-package bound was proven (GL-V0-009)
}

// packageKey is one resolved package's per-package content key, or the reason
// it runs unrecorded when its bound cannot be proven.
type packageKey struct {
	pkg, key, bound, reason string
}

// listedPackage is the `go list -json` subset in listFields.
type listedPackage struct {
	ImportPath string
	Dir        string
	Module     *struct{ Path string }
	Deps       []string
	GoFiles, CgoFiles, IgnoredGoFiles, TestGoFiles, XTestGoFiles, EmbedFiles, TestEmbedFiles, XTestEmbedFiles,
	SFiles, CFiles, HFiles, CXXFiles, MFiles, FFiles, SwigFiles, SwigCXXFiles, SysoFiles []string
}

func (p listedPackage) files() []string {
	var files []string
	for _, list := range [][]string{p.GoFiles, p.CgoFiles, p.IgnoredGoFiles, p.TestGoFiles, p.XTestGoFiles, p.EmbedFiles, p.TestEmbedFiles, p.XTestEmbedFiles, p.SFiles, p.CFiles, p.HFiles, p.CXXFiles, p.MFiles, p.FFiles, p.SwigFiles, p.SwigCXXFiles, p.SysoFiles} {
		files = append(files, list...)
	}
	return files
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
// which run under one content key each (GL-V0-009) and Go's own test cache,
// and the unresolved packages, which run with -count=1 under a tree-keyed
// record (GL-V0-004). When the partition cannot be computed every package runs
// with -count=1 under the tree key; when the bounds cannot be computed the
// resolved packages run through the Go test cache alone, unrecorded.
func (l *ledger) goTest(goTest []string) int {
	resolved, unresolvedPkgs, reason := l.partition()
	if reason != "" {
		fmt.Fprintf(l.stdout, "%sPARTITION unavailable: %s; every package runs uncached\n", prefixOut, reason)
		return l.runStep(unresolved, []string{"./..."}, append(append([]string{}, goTest...), "-count=1", "./..."))
	}
	fmt.Fprintf(l.stdout, "%sPARTITION %d resolved packages under per-package keys, %d unresolved under the tree key\n", prefixOut, len(resolved), len(unresolvedPkgs))
	if len(resolved) > 0 {
		keyed, reason := l.packageKeys(resolved)
		if reason != "" {
			fmt.Fprintf(l.stdout, "%sBOUNDS unavailable: %s; resolved packages run through the Go test cache\n", prefixOut, reason)
			if code := execute(append(append([]string{}, goTest...), resolved...)); code != 0 {
				return code
			}
		} else if code := l.runPackages(keyed, goTest); code != 0 {
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
	return l.digestKey(step, l.inputs(scope), packages), ""
}

// digestKey is the key of step over inputs: the schema, the tool identity,
// the Go environment a test reads, and the packages the step names.
func (l *ledger) digestKey(step, inputs string, packages []string) string {
	h := sha256.New()
	fmt.Fprintf(h, "%s\nstep %s\ntool %s\nenv GO_TEST_TIMEOUT=%s\nenv GOFLAGS=%s\ninputs %s\n", schema, step, l.identity, os.Getenv("GO_TEST_TIMEOUT"), os.Getenv("GOFLAGS"), inputs)
	for _, pkg := range packages {
		fmt.Fprintf(h, "package %s\n", pkg)
	}
	return hex.EncodeToString(h.Sum(nil))
}

// packageKeys derives one content key per resolved package (GL-V0-009) over
// its proven bound: every file `go list -deps -test` compiles or embeds into
// the package's test binary, every path the affected-plan selector's rules (a)
// to (c) attribute to the package, and the gate tooling. A package whose bound
// cannot be proven carries a reason instead and runs unrecorded; a bound the
// tool cannot compute at all returns the reason for every package.
func (l *ledger) packageKeys(resolved []string) ([]packageKey, string) {
	if l.dirErr != "" {
		return nil, l.dirErr
	}
	if l.digestErr != "" {
		return nil, l.digestErr
	}
	module, err := modulePath(filepath.Join(l.root, "go.mod"))
	if err != nil {
		return nil, err.Error()
	}
	bounds, err := l.selectorBounds(module)
	if err != nil {
		return nil, err.Error()
	}
	listed, err := l.listedFiles(module)
	if err != nil {
		return nil, err.Error()
	}
	present := map[string]bool{}
	for _, entry := range l.entries {
		present[entry.path] = true
	}
	var keyed []packageKey
	for _, pkg := range resolved {
		keyed = append(keyed, l.packageKey(pkg, bounds[pkg], listed[pkg], present))
	}
	return keyed, ""
}

func (l *ledger) packageKey(pkg string, bound map[string]string, files []string, present map[string]bool) packageKey {
	if bound == nil {
		return packageKey{pkg: pkg, reason: "the selector attributes no path to it"}
	}
	if files == nil {
		return packageKey{pkg: pkg, reason: "go list does not list it"}
	}
	inputs := map[string]bool{}
	rules := map[string]int{}
	for p, rule := range bound {
		inputs[p] = true
		rules[rule]++
	}
	for _, f := range files {
		if !present[f] {
			return packageKey{pkg: pkg, reason: "go list names " + f + ", which the worktree digest does not hold"}
		}
		inputs[f] = true
	}
	h := sha256.New()
	digested := 0
	for _, entry := range l.entries {
		if inputs[entry.path] || inScope(entry.path, tooling) {
			fmt.Fprintf(h, "%s\n", entry.line)
			digested++
		}
	}
	proof := fmt.Sprintf("go list -deps -test %d files; selector frontier %d, reader %d paths; %d entries digested with the gate tooling", len(files), rules["frontier"], rules["reader"], digested)
	return packageKey{pkg: pkg, key: l.digestKey(packageStep, "entries "+hex.EncodeToString(h.Sum(nil)), []string{pkg}), bound: proof}
}

// selectorBounds asks gate-affected-select for the paths rules (a) to (c)
// attribute to each resolved package, over the exact worktree entries.
func (l *ledger) selectorBounds(module string) (map[string]map[string]string, error) {
	var paths strings.Builder
	for _, entry := range l.entries {
		paths.WriteString(entry.path + "\n")
	}
	out, err := outputInput(l.root, strings.NewReader(paths.String()), "go", "run", "./tools/gate-affected-select", "-bounds", module, l.root)
	if err != nil {
		return nil, errors.New("gate-affected-select -bounds failed: " + err.Error())
	}
	bounds := map[string]map[string]string{}
	for _, line := range strings.Split(out, "\n") {
		if !strings.HasPrefix(line, "bound ") {
			continue
		}
		fields := strings.SplitN(strings.TrimPrefix(line, "bound "), " ", 3)
		if len(fields) != 3 {
			return nil, errors.New("unreadable bound line: " + line)
		}
		if bounds[fields[0]] == nil {
			bounds[fields[0]] = map[string]string{}
		}
		bounds[fields[0]][fields[2]] = fields[1]
	}
	return bounds, nil
}

// listedFiles maps every root-module package to the repository-relative files
// of its test binary's transitive closure within the module, from one
// `go list -deps -test -json ./...`.
func (l *ledger) listedFiles(module string) (map[string][]string, error) {
	out, err := output(l.root, "go", "list", "-deps", "-test", "-json="+listFields, "./...")
	if err != nil {
		return nil, errors.New("go list -deps -test failed: " + err.Error())
	}
	root, err := filepath.EvalSymlinks(l.root)
	if err != nil {
		return nil, err
	}
	packages := map[string]listedPackage{}
	decoder := json.NewDecoder(strings.NewReader(out))
	for decoder.More() {
		var p listedPackage
		if err := decoder.Decode(&p); err != nil {
			return nil, errors.New("go list output: " + err.Error())
		}
		packages[p.ImportPath] = p
	}
	files := map[string][]string{}
	for name, p := range packages {
		if p.Module == nil || p.Module.Path != module || strings.Contains(name, " [") || strings.HasSuffix(name, ".test") {
			continue
		}
		closure := p
		if test, ok := packages[name+".test"]; ok {
			closure = test
		}
		set := map[string]bool{}
		for _, dep := range append([]string{name}, closure.Deps...) {
			d, ok := packages[dep]
			if !ok || d.Module == nil || d.Module.Path != module {
				continue
			}
			dir, err := relativeDirectory(root, d.Dir)
			if err != nil {
				return nil, err
			}
			for _, f := range d.files() {
				set[path.Join(dir, filepath.ToSlash(f))] = true
			}
		}
		files[name] = sortedKeys(set)
	}
	return files, nil
}

func relativeDirectory(root, dir string) (string, error) {
	dir, err := filepath.EvalSymlinks(dir)
	if err != nil {
		return "", err
	}
	rel, err := filepath.Rel(root, dir)
	if err != nil {
		return "", err
	}
	if rel == "." {
		return "", nil
	}
	if strings.HasPrefix(rel, "..") {
		return "", errors.New("go list names a directory outside the worktree: " + dir)
	}
	return filepath.ToSlash(rel), nil
}

func sortedKeys(set map[string]bool) []string {
	keys := make([]string, 0, len(set))
	for key := range set {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

// runPackages runs the resolved packages whose key has no recorded pass in one
// go test invocation, holding each key's lock, and records each package on
// exit zero (GL-V0-009). A package with a reason runs in the same invocation
// and is not recorded.
func (l *ledger) runPackages(keyed []packageKey, goTest []string) int {
	var pending []packageKey
	for _, p := range keyed {
		if p.reason != "" {
			fmt.Fprintf(l.stdout, "%sRUN %s %s: %s\n", prefixOut, packageStep, p.pkg, p.reason)
			pending = append(pending, p)
			continue
		}
		if rec, hit := l.lookup(p.key); hit {
			fmt.Fprintf(l.stdout, "%sHIT %s %s %s (recorded %s on %s)\n", prefixOut, packageStep, p.pkg, short(p.key), rec.RecordedAt, rec.Host)
			continue
		}
		pending = append(pending, p)
	}
	sort.Slice(pending, func(i, j int) bool { return pending[i].key < pending[j].key })
	var unlocks []func()
	defer func() {
		for _, unlock := range unlocks {
			unlock()
		}
	}()
	var run []packageKey
	for _, p := range pending {
		if p.reason != "" {
			run = append(run, p)
			continue
		}
		unlock, err := l.lock(p.key, packageStep+" "+p.pkg)
		if err != nil {
			fmt.Fprintf(l.stdout, "%sRUN %s %s: ledger lock unavailable: %s\n", prefixOut, packageStep, p.pkg, err)
			p.reason = err.Error()
			run = append(run, p)
			continue
		}
		unlocks = append(unlocks, unlock)
		if rec, hit := l.lookup(p.key); hit {
			fmt.Fprintf(l.stdout, "%sHIT %s %s %s (recorded %s on %s while waiting)\n", prefixOut, packageStep, p.pkg, short(p.key), rec.RecordedAt, rec.Host)
			continue
		}
		fmt.Fprintf(l.stdout, "%sRUN %s %s: no recorded pass for %s\n", prefixOut, packageStep, p.pkg, short(p.key))
		run = append(run, p)
	}
	if len(run) == 0 {
		return 0
	}
	sort.Slice(run, func(i, j int) bool { return run[i].pkg < run[j].pkg })
	command := append([]string{}, goTest...)
	for _, p := range run {
		command = append(command, p.pkg)
	}
	started := time.Now()
	if code := execute(command); code != 0 {
		return code
	}
	for _, p := range run {
		if p.reason != "" {
			continue
		}
		rec := record{Schema: schema, Step: packageStep, Key: p.key, Tree: l.tree, RecordedAt: started.UTC().Format(time.RFC3339), DurationMS: time.Since(started).Milliseconds(), Host: hostname(), Packages: []string{p.pkg}, Bound: p.bound}
		if err := l.record(rec); err != nil {
			fmt.Fprintf(l.stderr, "%sRECORD %s %s failed: %s\n", prefixOut, packageStep, p.pkg, err)
			continue
		}
		fmt.Fprintf(l.stdout, "%sRECORD %s %s %s\n", prefixOut, packageStep, p.pkg, short(p.key))
	}
	return 0
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
	entriesInput, err := gitOutput(root, "ls-files", "--stage", "-z")
	if err != nil {
		return "", nil, "git ls-files --stage failed: " + err.Error()
	}
	private, err := os.CreateTemp("", "corvint-gate-ledger-index.")
	if err != nil {
		return "", nil, "private index: " + err.Error()
	}
	defer os.Remove(private.Name())
	defer os.Remove(private.Name() + ".lock")
	if err := private.Close(); err != nil {
		return "", nil, "private index: " + err.Error()
	}
	env := "GIT_INDEX_FILE=" + private.Name()
	if _, err := gitOutputEnv(root, env, "read-tree", "--empty"); err != nil {
		return "", nil, "private index initialization failed: " + err.Error()
	}
	// Retain tracked membership (including ignored and intent-to-add paths),
	// but discard cached stats so restored timestamps cannot conceal changed bytes.
	if _, err := gitOutputEnvInput(root, env, strings.NewReader(entriesInput), "update-index", "-z", "--index-info"); err != nil {
		return "", nil, "private index import failed: " + err.Error()
	}
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
	if !ownedByInvokingUser(info) {
		return "", "ledger directory is not owned by this user"
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
	if err := lockExclusive(f, true); err != nil {
		fmt.Fprintf(l.stdout, "%sWAIT %s: another run holds %s\n", prefixOut, step, short(key))
		if err := lockExclusive(f, false); err != nil {
			f.Close()
			return nil, err
		}
	}
	return func() { unlock(f); f.Close() }, nil
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
		if code, ok := signalExit(exit); ok {
			return code
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
	return gitOutputEnvInput(dir, env, nil, args...)
}

func gitOutputEnvInput(dir, env string, input io.Reader, args ...string) (string, error) {
	operation := args[0]
	// Every call, not only private-index ones: a repository with core.fsmonitor
	// set makes git wait on its monitor daemon, which a loaded host can stall
	// past the test timeout (V1-0355).
	args = append([]string{"-c", "core.fsmonitor=false", "-c", "core.ignorestat=false"}, args...)
	cmd := exec.Command("git", args...)
	cmd.Stdin = input
	cmd.Dir = dir
	if env != "" {
		cmd.Env = append(os.Environ(), env)
	}
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("git %s: %s", operation, strings.TrimSpace(stderr.String()))
	}
	return string(out), nil
}

func output(dir string, name string, args ...string) (string, error) {
	return outputInput(dir, nil, name, args...)
}

func outputInput(dir string, input io.Reader, name string, args ...string) (string, error) {
	cmd := exec.Command(name, args...)
	cmd.Dir = dir
	cmd.Stdin = input
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
