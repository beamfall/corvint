// Command corvint-pr-tests executes typed Go test argv from a separately trusted
// affected planner and the existing gate-affected-select. AFP-V0-013/014.
package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strings"
	"syscall"
	"time"
)

const schema = "corvint-pr-tests/0"

var testArgs = []string{"test", "-json", "-p", "1", "-count=1", "-race", "-timeout", "50m"}
var oid = regexp.MustCompile(`^(?:[0-9a-f]{40}|[0-9a-f]{64})$`)
var packageName = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._~/-]*$`)

type options struct {
	rows, row                                                                                                        int
	rowsSet                                                                                                          bool
	profile, evidence, dockerContext, trusted, containerMode                                                         string
	mode, root, base, head, target, planner, selector, source, out, qualification, qualificationSHA, corpus, runtime string
}
type identity struct {
	Container                                                                             *containerProfile
	CachePolicy                                                                           string
	Limits                                                                                []int64
	Source, Planner, Selector, Driver, GoBinary, GoVersion, OS, Arch, OSRelease, Compiler string
	Args, Env                                                                             []string
}
type selection struct {
	Schema, Base, Head, Target, Tree, Reason, ObservedTarget, ObservedTree string
	Identity                                                               identity
	Packages                                                               []string
	PlanSHA, AuditSHA                                                      string
}
type execution struct {
	Env            []string
	Selection      selection
	Args           []string
	Exit           int
	ElapsedSeconds float64
	Error          string
}
type receipt struct {
	Profile, Tool, Revision string
	OK                      bool
	Mutates                 bool
	Range                   struct{ Base string }
	Plan                    struct{ Dirty []string }
	Provider                struct {
		Go struct {
			State    string
			Packages []string
		}
	}
}

func main() {
	var o options
	flag.StringVar(&o.mode, "mode", "run", "run, freeze, shadow, or qualify")
	flag.StringVar(&o.root, "root", ".", "tested repository")
	flag.StringVar(&o.base, "base", "", "event base OID")
	flag.StringVar(&o.head, "head", "", "event PR head OID")
	flag.StringVar(&o.target, "target", "", "captured merge OID or corpus end")
	flag.StringVar(&o.planner, "planner", "", "trusted planner executable")
	flag.StringVar(&o.selector, "selector", "", "trusted selector executable")
	flag.StringVar(&o.source, "source", "", "trusted tool source OID")
	flag.StringVar(&o.out, "out", "", "owned output directory outside repository")
	flag.StringVar(&o.qualification, "qualification", "", "trusted external qualification JSON")
	flag.StringVar(&o.qualificationSHA, "qualification-sha256", "", "trusted qualification SHA256")
	flag.StringVar(&o.corpus, "corpus", "", "frozen corpus JSON")
	flag.IntVar(&o.rows, "rows", 1, "maximum frozen shadow rows to observe (1..200); default measures one row")
	flag.StringVar(&o.runtime, "runtime", "/tmp/corvint-pr-tests-runtime", "exclusive fixed runtime layout (must not already exist)")
	flag.IntVar(&o.row, "row", 0, "one indexed frozen row (1..200), exclusive with --rows")
	flag.StringVar(&o.profile, "container-profile", "", "trusted read-only inspected container profile")
	flag.StringVar(&o.evidence, "evidence", "", "retained rows directory for qualify")
	flag.StringVar(&o.dockerContext, "docker-context", "", "explicit Docker context for container launcher")
	flag.StringVar(&o.trusted, "trusted", "", "read-only directory of frozen Linux trusted binaries")
	flag.StringVar(&o.containerMode, "container-mode", "shadow", "container operation: run, freeze, shadow, qualify")
	flag.Parse()
	flag.Visit(func(f *flag.Flag) {
		if f.Name == "rows" {
			o.rowsSet = true
		}
		if f.Name == "row" && o.row == 0 {
			o.row = -1
		}
	})
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM, syscall.SIGHUP)
	defer stop()
	exit, err := dispatch(ctx, o)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		if exit == 0 {
			exit = 2
		}
	}
	os.Exit(exit)
}
func dispatch(ctx context.Context, o options) (code int, err error) {
	if o.mode == "container" {
		return launchContainer(ctx, o)
	}
	if o.row != 0 && (o.rowsSet || o.row < 1 || o.row > 200 || o.mode != "shadow") {
		return 2, errors.New("--row requires shadow, 1..200, and no --rows")
	}
	root, err := filepath.Abs(o.root)
	if err != nil {
		return 2, err
	}
	root, err = filepath.EvalSymlinks(root)
	if err != nil {
		return 2, err
	}
	o.root = root
	out, err := externalPath(root, o.out)
	if err != nil {
		return 2, err
	}
	o.out = out
	if o.mode != "qualify" {
		if !filepath.IsAbs(o.runtime) {
			return 2, errors.New("runtime must be absolute")
		}
		o.runtime, err = externalPath(root, o.runtime)
		if err != nil {
			return 2, err
		}
	}
	if err = os.MkdirAll(out, 0700); err != nil {
		return 2, err
	}
	if err = realExternalDirectory(root, out); err != nil {
		return 2, err
	}
	if o.mode != "qualify" {
		if err = os.Mkdir(o.runtime, 0700); err != nil {
			return 2, fmt.Errorf("refusing existing runtime: %w", err)
		}
		defer finishCleanup(&code, &err, o.runtime, os.RemoveAll)
		if err = realExternalDirectory(root, o.runtime); err != nil {
			return 2, err
		}
		if err = prepareCache(o); err != nil {
			return 2, err
		}
	}
	switch o.mode {
	case "freeze":
		return 0, freeze(ctx, o)
	case "shadow":
		return 0, shadow(ctx, o)
	case "qualify":
		return 0, qualify(o)
	case "run":
		return runPR(ctx, o)
	default:
		return 2, errors.New("unknown mode")
	}
}

// Resolve the nearest existing ancestor before creating anything. In particular,
// a missing suffix below a symlink must never be created inside the tested tree.
func externalPath(root, requested string) (string, error) {
	if requested == "" {
		return "", errors.New("external path is required")
	}
	absolute, err := filepath.Abs(requested)
	if err != nil {
		return "", err
	}
	ancestor := absolute
	for {
		_, err = os.Lstat(ancestor)
		if err == nil {
			break
		}
		if !os.IsNotExist(err) {
			return "", err
		}
		parent := filepath.Dir(ancestor)
		if parent == ancestor {
			return "", errors.New("no existing path ancestor")
		}
		ancestor = parent
	}
	resolved, err := filepath.EvalSymlinks(ancestor)
	if err != nil {
		return "", err
	}
	suffix, err := filepath.Rel(ancestor, absolute)
	if err != nil {
		return "", err
	}
	canonical := filepath.Join(resolved, suffix)
	if within(root, canonical) {
		return "", errors.New("output/runtime resolves inside tested repository")
	}
	return canonical, nil
}
func realExternalDirectory(root, path string) error {
	st, err := os.Lstat(path)
	if err != nil {
		return err
	}
	if !st.IsDir() || st.Mode()&os.ModeSymlink != 0 {
		return errors.New("created path is not a real directory")
	}
	resolved, err := filepath.EvalSymlinks(path)
	if err != nil {
		return err
	}
	if resolved != path || within(root, resolved) {
		return errors.New("created directory identity changed")
	}
	return nil
}
func finishCleanup(code *int, result *error, path string, remove func(string) error) {
	if err := remove(path); err != nil {
		*result = errors.Join(*result, fmt.Errorf("owned runtime cleanup failed: %w", err))
		if *code == 0 {
			*code = 2
		}
	}
}
func within(root, path string) bool {
	rel, err := filepath.Rel(root, path)
	return err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(os.PathSeparator))
}
func runtimePath(o options) string {
	if o.runtime != "" {
		return o.runtime
	}
	return filepath.Join(o.out, "runtime")
}
func closedEnv(o options) []string {
	goPath, _ := exec.LookPath("go")
	goPath, _ = filepath.EvalSymlinks(goPath)
	r := runtimePath(o)
	return []string{"PATH=" + filepath.Dir(goPath) + ":/usr/bin:/bin", "HOME=" + filepath.Join(r, "home"), "TMPDIR=" + filepath.Join(r, "tmp"), "TMP=" + filepath.Join(r, "tmp"), "TEMP=" + filepath.Join(r, "tmp"), "LANG=C", "LC_ALL=C", "TZ=UTC", "CC=/usr/bin/cc", "GOENV=off", "GOTOOLCHAIN=local", "GOPROXY=off", "GOWORK=off", "GOFLAGS=", "GOSUMDB=off", "CGO_ENABLED=1", "GOCACHE=" + filepath.Join(r, "cache"), "GOMODCACHE=" + filepath.Join(r, "modules"), "GIT_NO_REPLACE_OBJECTS=1", "GIT_GRAFT_FILE=" + os.DevNull, "GIT_CONFIG_NOSYSTEM=1", "GIT_CONFIG_GLOBAL=" + os.DevNull, "GIT_CONFIG_COUNT=0", "GIT_TERMINAL_PROMPT=0", "GOMAXPROCS=2"}
}
func prepareCache(o options) error {
	r := runtimePath(o)
	for _, name := range []string{"home", "tmp", "cache"} {
		if err := os.MkdirAll(filepath.Join(r, name), 0700); err != nil {
			return err
		}
	}
	if err := os.MkdirAll(filepath.Join(r, "modules"), 0500); err != nil {
		return err
	}
	return os.Chmod(filepath.Join(r, "modules"), 0500)
}
func resetPrivateRuntime(o options) error {
	for _, name := range []string{"home", "tmp", "cache"} {
		if err := os.RemoveAll(filepath.Join(runtimePath(o), name)); err != nil {
			return err
		}
	}
	return prepareCache(o)
}
func executable(name string, env []string) (string, error) {
	if filepath.IsAbs(name) {
		return filepath.EvalSymlinks(name)
	}
	for _, v := range env {
		if !strings.HasPrefix(v, "PATH=") {
			continue
		}
		for _, dir := range filepath.SplitList(strings.TrimPrefix(v, "PATH=")) {
			p := filepath.Join(dir, name)
			st, err := os.Stat(p)
			if err == nil && !st.IsDir() && st.Mode()&0111 != 0 {
				return filepath.EvalSymlinks(p)
			}
		}
	}
	return "", fmt.Errorf("executable unavailable in closed PATH: %s", name)
}
func command(ctx context.Context, root string, env []string, stdout, stderr io.Writer, timeout time.Duration, name string, args ...string) (int, error) {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	path, err := executable(name, env)
	if err != nil {
		return -1, err
	}
	cmd := exec.CommandContext(ctx, path, args...)
	cmd.Dir = root
	cmd.Env = env
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	attachGroup(cmd)
	if err := cmd.Start(); err != nil {
		return -1, err
	}
	defer killGroup(cmd)
	err = cmd.Wait()
	if ctx.Err() != nil {
		return -1, ctx.Err()
	}
	if err == nil {
		return 0, nil
	}
	var exit *exec.ExitError
	if errors.As(err, &exit) {
		return exit.ExitCode(), nil
	}
	return -1, err
}

type boundedBuffer struct {
	buf    bytes.Buffer
	limit  int
	cancel context.CancelFunc
}

func (b *boundedBuffer) Len() int       { return b.buf.Len() }
func (b *boundedBuffer) Bytes() []byte  { return b.buf.Bytes() }
func (b *boundedBuffer) String() string { return b.buf.String() }
func (b *boundedBuffer) Write(p []byte) (int, error) {
	if b.Len()+len(p) > b.limit {
		if b.cancel != nil {
			b.cancel()
		}
		return 0, errors.New("command output bound exceeded")
	}
	return b.buf.Write(p)
}
func capture(ctx context.Context, o options, name string, args ...string) (string, error) {
	b := &boundedBuffer{limit: 8 << 20}
	e := &boundedBuffer{limit: 1 << 20}
	code, err := command(ctx, o.root, closedEnv(o), b, e, 2*time.Minute, name, args...)
	if err != nil {
		return "", err
	}
	if code != 0 {
		return "", fmt.Errorf("%s exited %d: %s", name, code, e.String())
	}
	return strings.TrimSpace(b.String()), nil
}
func git(ctx context.Context, o options, args ...string) (string, error) {
	return capture(ctx, o, "git", append([]string{"-c", "safe.directory=" + o.root, "-c", "core.hooksPath=/dev/null", "-c", "core.fsmonitor=false"}, args...)...)
}
func digestFile(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := sha256.New()
	if _, err = io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}
func writeJSON(path string, v any) error {
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(b, '\n'), 0600)
}
func readJSON(path string, v any) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()
	b, err := io.ReadAll(io.LimitReader(f, 8<<20+1))
	if err != nil {
		return err
	}
	if len(b) > 8<<20 {
		return errors.New("JSON exceeds bound")
	}
	return json.Unmarshal(b, v)
}
func toolIdentity(ctx context.Context, o options) (identity, error) {
	id := identity{CachePolicy: "empty-immediately-before-test/1", Source: o.source, OS: runtime.GOOS, Arch: runtime.GOARCH, Args: append([]string{}, testArgs...), Env: closedEnv(o), Limits: []int64{maxStdout, maxStderr}}
	if o.profile != "" {
		var p containerProfile
		if o.profile != "/profile/profile.json" {
			return id, errors.New("unexpected container profile path")
		}
		if err := readJSON(o.profile, &p); err != nil {
			return id, err
		}
		if err := p.validate(); err != nil {
			return id, err
		}
		id.Container = &p
	}
	goPath, err := executable("go", closedEnv(o))
	if err != nil {
		return id, err
	}
	id.GoBinary, err = digestFile(goPath)
	if err != nil {
		return id, err
	}
	id.GoVersion, err = capture(ctx, o, "go", "env", "GOVERSION")
	if err != nil {
		return id, err
	}
	compiler, err := capture(ctx, o, "go", "env", "CC")
	if err != nil {
		return id, err
	}
	if len(strings.Fields(compiler)) != 1 {
		return id, errors.New("unsupported compiler command")
	}
	compilerPath, err := executable(compiler, closedEnv(o))
	if err != nil {
		return id, err
	}
	compilerHash, err := digestFile(compilerPath)
	if err != nil {
		return id, err
	}
	compilerVersion, err := capture(ctx, o, compilerPath, "--version")
	id.Compiler = compiler + "\n" + compilerHash + "\n" + compilerVersion
	if err != nil {
		return id, err
	}
	if runtime.GOOS == "linux" {
		b, err := os.ReadFile("/etc/os-release")
		if err != nil {
			return id, err
		}
		id.OSRelease = string(b)
	} else {
		id.OSRelease, err = capture(ctx, o, "uname", "-sr")
		if err != nil {
			return id, err
		}
	}
	if !oid.MatchString(o.source) {
		return id, errors.New("missing trusted source OID")
	}
	paths := []string{o.planner, o.selector}
	self, err := os.Executable()
	if err != nil {
		return id, err
	}
	paths = append(paths, self)
	paths = append(paths, goPath)
	hashes := []*string{&id.Planner, &id.Selector, &id.Driver, &id.GoBinary}
	for i, p := range paths {
		if !filepath.IsAbs(p) {
			return id, errors.New("tool paths must be absolute")
		}
		resolved, err := filepath.EvalSymlinks(p)
		if err != nil {
			return id, err
		}
		if within(o.root, resolved) {
			return id, errors.New("trusted tool is inside tested checkout")
		}
		h, err := digestFile(p)
		if err != nil {
			return id, err
		}
		*hashes[i] = h
	}

	if id.Container != nil && (id.Container.Source != id.Source || id.Container.Tools["corvint"] != id.Planner || id.Container.Tools["gate-affected-select"] != id.Selector || id.Container.Tools["corvint-pr-tests"] != id.Driver || runtime.GOOS != "linux" || runtime.GOARCH != "amd64") {
		return id, errors.New("container trusted tool identity mismatch")
	}
	if id.GoVersion != "go1.27.1" {
		return id, errors.New("unsupported Go version")
	}
	return id, nil
}
func topology(ctx context.Context, o options, merge bool) (string, error) {
	if !oid.MatchString(o.base) || !oid.MatchString(o.target) {
		return "", errors.New("base/target must be full OIDs")
	}
	actual, err := git(ctx, o, "rev-parse", "HEAD")
	if err != nil {
		return "", err
	}
	if actual != o.target {
		return "", errors.New("HEAD drift")
	}
	status, err := git(ctx, o, "status", "--porcelain=v1", "--untracked-files=all")
	if err != nil {
		return "", err
	}
	if status != "" {
		return "", errors.New("tested checkout is dirty")
	}
	graft, err := git(ctx, o, "rev-parse", "--git-common-dir")
	if err != nil {
		return "", err
	}
	if !filepath.IsAbs(graft) {
		graft = filepath.Join(o.root, graft)
	}
	graft = filepath.Join(graft, "info", "grafts")
	if _, err = os.Stat(graft); !os.IsNotExist(err) {
		return "", errors.New("grafts are unsupported")
	}
	parents, err := git(ctx, o, "show", "-s", "--format=%P", o.target)
	if err != nil {
		return "", err
	}
	p := strings.Fields(parents)
	if merge {
		if !oid.MatchString(o.head) || len(p) != 2 || p[0] != o.base || p[1] != o.head {
			return "", errors.New("event base/head do not match exact merge parents")
		}
	} else {
		if len(p) == 0 || p[0] != o.base {
			return "", errors.New("historical pair is not first-parent")
		}
	}
	if _, err = git(ctx, o, "merge-base", "--is-ancestor", o.base, o.target); err != nil {
		return "", err
	}
	return git(ctx, o, "rev-parse", "HEAD^{tree}")
}
func plan(ctx context.Context, o options, id identity, merge bool) selection {
	s := selection{Schema: schema, Base: o.base, Head: o.head, Target: o.target, Identity: id, Packages: []string{"./..."}}
	tree, err := topology(ctx, o, merge)
	if err != nil {
		s.Reason = err.Error()
		return s
	}
	s.Tree = tree
	raw, err := capture(ctx, o, o.planner, "affected", "--base", o.base)
	if err != nil {
		s.Reason = "planner failed: " + err.Error()
		return s
	}
	path := filepath.Join(o.out, "plan.json")
	if err = os.WriteFile(path, []byte(raw), 0600); err != nil {
		s.Reason = err.Error()
		return s
	}
	s.PlanSHA, _ = digestFile(path)
	var r receipt
	if err = json.Unmarshal([]byte(raw), &r); err != nil || r.Profile != "affected-plan/0" || r.Tool != "affected" || !r.OK || r.Mutates || r.Revision != o.target || r.Range.Base != o.base || r.Plan.Dirty == nil || r.Provider.Go.State == "" {
		s.Reason = "malformed or mismatched planner receipt"
		return s
	}
	moduleData, err := os.ReadFile(filepath.Join(o.root, "go.mod"))
	if err != nil {
		s.Reason = err.Error()
		return s
	}
	module := ""
	for _, line := range strings.Split(string(moduleData), "\n") {
		fields := strings.Fields(line)
		if len(fields) == 2 && fields[0] == "module" {
			module = fields[1]
			break
		}
	}
	if !packageName.MatchString(module) {
		s.Reason = "invalid root module"
		return s
	}
	audit, err := capture(ctx, o, o.selector, path, module, o.root)
	if err != nil {
		s.Reason = "selector failed: " + err.Error()
		return s
	}
	auditPath := filepath.Join(o.out, "selector.txt")
	if err = os.WriteFile(auditPath, []byte(audit), 0600); err != nil {
		s.Reason = err.Error()
		return s
	}
	s.AuditSHA, _ = digestFile(auditPath)
	lines := strings.Split(audit, "\n")
	verdict := strings.Fields(lines[len(lines)-1])
	if len(verdict) < 2 || verdict[0] != "run" {
		s.Reason = "selector: " + lines[len(lines)-1]
		return s
	}
	packages := verdict[1:]
	if len(packages) > 4091 {
		s.Reason = "selection exceeds argv bound"
		return s
	}
	for _, p := range packages {
		if !packageName.MatchString(p) || strings.Contains(p, "..") || (p != module && !strings.HasPrefix(p, module+"/")) {
			s.Reason = "invalid executable package argument"
			return s
		}
	}
	check, err := topology(ctx, o, merge)
	if err != nil || check != tree {
		s.Reason = "source drift after planning"
		return s
	}
	s.Packages = packages
	return s
}
func runPR(ctx context.Context, o options) (int, error) {
	if err := prepareCache(o); err != nil {
		return 2, err
	}
	id, err := toolIdentity(ctx, o)
	s := selection{Schema: schema, Base: o.base, Head: o.head, Target: o.target, Identity: id, Packages: []string{"./..."}}
	if err != nil {
		s.Reason = "tool profile unavailable: " + err.Error()
	} else {
		s = plan(ctx, o, id, true)
	}
	if s.Reason == "" {
		if err = admit(o, id); err != nil {
			s.Reason = "qualification unavailable: " + err.Error()
			s.Packages = []string{"./..."}
		}
	}
	if ctx.Err() != nil {
		return 130, ctx.Err()
	}
	return execute(ctx, o, s)
}
func execute(ctx context.Context, o options, s selection) (int, error) {
	current, identityErr := toolIdentity(ctx, o)
	if current.GoBinary == "" || current.Compiler == "" {
		return 2, fmt.Errorf("execution runtime identity unavailable: %v", identityErr)
	}
	if !equal(current.Container, s.Identity.Container) {
		return 2, errors.New("container profile drift before execution")
	}
	if s.Identity.GoBinary != "" && (current.GoBinary != s.Identity.GoBinary || current.Compiler != s.Identity.Compiler || !equal(current.Env, s.Identity.Env)) {
		return 2, errors.New("Go/compiler/environment drift before execution")
	}
	if s.Reason == "" && (identityErr != nil || !equal(current, s.Identity)) {
		s.Reason = "planner/selector/driver drift before execution"
		s.Packages = []string{"./..."}
	}
	// Both PR and historical execution start cold after all planning/enumeration.
	if err := resetPrivateRuntime(o); err != nil {
		return 2, err
	}
	ctx, cancelOutput := context.WithCancel(ctx)
	defer cancelOutput()

	s.ObservedTarget, _ = git(ctx, o, "rev-parse", "HEAD")
	s.ObservedTree, _ = git(ctx, o, "rev-parse", "HEAD^{tree}")
	if err := writeJSON(filepath.Join(o.out, "selection.json"), s); err != nil {
		return 2, err
	}
	if s.Reason == "" {
		tree, err := topology(ctx, o, o.mode == "run")
		if err != nil || tree != s.Tree {
			s.Reason = "source drift before execution"
			s.Packages = []string{"./..."}
			if err = writeJSON(filepath.Join(o.out, "selection.json"), s); err != nil {
				return 2, err
			}
		}
	}
	fmt.Fprintf(os.Stderr, "corvint-pr-tests: packages=%v fallback=%q\n", s.Packages, s.Reason)
	args := append(append([]string{}, testArgs...), s.Packages...)
	out, err := os.Create(filepath.Join(o.out, "go.json"))
	if err != nil {
		return 2, err
	}
	defer out.Close()
	stderr, err := os.Create(filepath.Join(o.out, "go.stderr"))
	if err != nil {
		return 2, err
	}
	defer stderr.Close()
	boundedOut := &cappedWriter{dst: out, remaining: maxStdout, cancel: cancelOutput}
	boundedErr := &cappedWriter{dst: stderr, remaining: maxStderr, cancel: cancelOutput}
	start := time.Now()
	code, runErr := command(ctx, o.root, closedEnv(o), boundedOut, boundedErr, 70*time.Minute, "go", args...)
	if boundedOut.overflow || boundedErr.overflow {
		runErr = errors.New("test output limit exceeded")
	}
	e := execution{Env: closedEnv(o), Selection: s, Args: args, Exit: code, ElapsedSeconds: time.Since(start).Seconds()}
	if runErr != nil {
		e.Error = runErr.Error()
	}
	if err = writeJSON(filepath.Join(o.out, "execution.json"), e); err != nil {
		return 2, err
	}
	if runErr != nil {
		return 2, runErr
	}
	return code, nil
}
func equal(a, b any) bool        { x, _ := json.Marshal(a); y, _ := json.Marshal(b); return bytes.Equal(x, y) }
func sorted(v []string) []string { r := append([]string{}, v...); sort.Strings(r); return r }

const maxStdout int64 = 128 << 20
const maxStderr int64 = 8 << 20

// Overflow cancels the command group immediately while retaining a bounded prefix.
type cappedWriter struct {
	dst       io.Writer
	remaining int64
	cancel    context.CancelFunc
	overflow  bool
}

func (w *cappedWriter) Write(p []byte) (int, error) {
	if int64(len(p)) <= w.remaining {
		n, err := w.dst.Write(p)
		w.remaining -= int64(n)
		return n, err
	}
	w.overflow = true
	w.cancel()
	n, err := w.dst.Write(p[:w.remaining])
	w.remaining = 0
	if err != nil {
		return n, err
	}
	return n, errors.New("test output limit exceeded")
}
