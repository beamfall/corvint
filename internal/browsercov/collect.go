// SPDX-License-Identifier: Apache-2.0

package browsercov

import (
	"context"
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/Beamfall/corvint/internal/linecov"
)

//go:embed hook.cjs
var hookSource []byte

// Options configures one collection over a clean checkout at the index
// commit.
type Options struct {
	Root   string
	Module string // the records' path prefix
	Runner string // playwright or cypress
	// Config is the runner's configuration file, root-relative; empty means
	// the runner's default.
	Config string
	// Server starts the application under test for runners that do not start
	// it themselves (Cypress); ServerURL answers once it is ready.
	Server, ServerURL string
	// Browser is Cypress's --browser (a Chromium-family name or path).
	Browser string
	// Bin holds the runner executables; empty means Root/node_modules/.bin.
	Bin     string
	Work    string
	Timeout time.Duration // per runner invocation
	Log     func(format string, args ...any)
}

// Outcome is one collected test: a Record when it passed alone with
// complete coverage, otherwise the reason it yields none.
type Outcome struct {
	Package, Name string
	Record        *linecov.Record
	Reason        string
}

// result is one test's state in one runner invocation.
type result struct {
	Name  string
	State string // passed, failed or skipped
	key   any    // what the adapter needs to run this test alone
}

type adapter interface {
	// specs lists the spec files, root-relative.
	specs(ctx context.Context, c *collector) ([]string, error)
	// run runs spec, every test of it or only the test only names, writing
	// coverage under dir.
	run(ctx context.Context, c *collector, dir, spec string, only *result) ([]result, error)
	// version names the runner and its version.
	version(ctx context.Context, c *collector) (string, error)
	// serves reports whether the runner starts a server under test itself.
	serves(c *collector) bool
}

type collector struct {
	Options
	repo  *Repo
	hook  string
	ad    adapter
	runs  int
	agent string // browser version every run must share
	// cleanup lists generated files to remove when the collection ends.
	cleanup []string
	tmp     string // short TMPDIR for runner processes, when the inherited one is too long
}

// chromeSocket is the longest name Chromium adds to TMPDIR for its singleton
// socket; past sun_path's 108 bytes the browser exits before it listens.
const chromeSocket = "/.org.chromium.Chromium.XXXXXX/SingletonSocket"

// Collect runs each spec file once in order, then each test it lists alone,
// and returns one Outcome per test plus each spec's in-order run, with the
// runner identity every record shares (LPCV-V0-970, LPCV-V0-962).
func Collect(ctx context.Context, o Options) ([]Outcome, string, error) {
	if o.Module == "" {
		o.Module = "browser"
	}
	// Processes under test report file: URLs, which bind only under an
	// absolute root.
	root, err := filepath.Abs(o.Root)
	if err != nil {
		return nil, "", err
	}
	o.Root = root
	if o.Bin == "" {
		o.Bin = filepath.Join(o.Root, "node_modules", ".bin")
	}
	if o.Timeout == 0 {
		o.Timeout = 10 * time.Minute
	}
	if o.Log == nil {
		o.Log = func(string, ...any) {}
	}
	if err := unsupportedPlatform(); err != nil {
		return nil, "", err
	}
	c := &collector{Options: o}
	defer func() {
		for _, f := range c.cleanup {
			os.Remove(f)
		}
	}()
	switch o.Runner {
	case "playwright":
		c.ad = &playwright{}
	case "cypress":
		c.ad = &cypress{}
	default:
		return nil, "", fmt.Errorf("browsercov: runner %q: want playwright or cypress (%s)", o.Runner, refusals)
	}
	oids, err := linecov.OIDs(ctx, o.Root, "HEAD")
	if err != nil {
		return nil, "", err
	}
	c.repo = NewRepo(o.Root, oids)
	if work, err := filepath.Abs(o.Work); err == nil {
		c.repo.Generated = []string{work} // the hook, when Work is inside Root
	}
	c.hook = filepath.Join(o.Work, "hook.cjs")
	if err := os.MkdirAll(o.Work, 0o755); err != nil {
		return nil, "", err
	}
	if err := os.WriteFile(c.hook, hookSource, 0o644); err != nil {
		return nil, "", err
	}
	if len(os.TempDir()+chromeSocket) >= 108 {
		if c.tmp, err = os.MkdirTemp("/tmp", "browsercov-"); err != nil {
			return nil, "", err
		}
		defer os.RemoveAll(c.tmp)
	}
	ver, err := c.ad.version(ctx, c)
	if err != nil {
		return nil, "", err
	}
	node, err := exec.CommandContext(ctx, "node", "--version").Output()
	if err != nil {
		return nil, "", fmt.Errorf("browsercov: node: %v", err)
	}
	specs, err := c.ad.specs(ctx, c)
	if err != nil {
		return nil, "", err
	}
	if !c.ad.serves(c) && o.Server == "" {
		return nil, "", fmt.Errorf("browsercov: %s does not start the application under test: pass a server command", o.Runner)
	}
	var out []Outcome
	var runs []Run
	var keep []int // index into out for each run
	add := func(spec, name string, obs []Observation, elapsed time.Duration, reason string) {
		out = append(out, Outcome{Package: spec, Name: name, Reason: reason})
		if reason == "" {
			runs = append(runs, Run{Package: spec, Name: name, Elapsed: elapsed, Obs: obs})
			keep = append(keep, len(out)-1)
		}
	}
	for _, spec := range specs {
		o.Log("browsercov: %s in order", spec)
		obs, tests, elapsed, reason := c.once(ctx, spec, nil)
		add(spec, linecov.InOrder, obs, elapsed, reason)
		dup := duplicates(tests)
		reported := map[string]bool{}
		for i := range tests {
			t := tests[i]
			if dup[t.Name] {
				if !reported[t.Name] {
					reported[t.Name] = true
					add(spec, t.Name, nil, 0, "another test in the file has the same name")
				}
				continue
			}
			if t.State == "skipped" {
				continue
			}
			o.Log("browsercov: %s %s", spec, t.Name)
			obs, ran, elapsed, reason := c.once(ctx, spec, &t)
			if reason == "" {
				reason = alone(ran, t.Name)
			}
			add(spec, t.Name, obs, elapsed, reason)
		}
	}
	for i, r := range Records(o.Module, runs) {
		rec := r
		out[keep[i]].Record = &rec
	}
	sort.SliceStable(out, func(i, j int) bool {
		a, b := out[i], out[j]
		return a.Package < b.Package || a.Package == b.Package && a.Name < b.Name
	})
	id := fmt.Sprintf("browsercov/1 %s %s node@%s v8-precise", ver, c.agent, strings.TrimSpace(string(node)))
	return out, id, nil
}

// alone checks that an isolated run ran exactly the one test and it passed.
func alone(ran []result, name string) string {
	var others []string
	passed := 0
	for _, r := range ran {
		switch {
		case r.State == "skipped":
		case r.Name != name:
			others = append(others, r.Name)
		case r.State == "passed":
			passed++
		default:
			return "failed when run alone"
		}
	}
	if len(others) > 0 {
		return fmt.Sprintf("running it alone also ran %q", others)
	}
	if passed > 1 {
		return fmt.Sprintf("running it alone ran %d tests of that name", passed)
	}
	if passed == 0 {
		return "did not run when run alone"
	}
	return ""
}

// duplicates returns the names that more than one test of a file shares;
// a runner's filter cannot run one of them alone, so none gets a record.
func duplicates(tests []result) map[string]bool {
	seen := map[string]int{}
	for _, t := range tests {
		seen[t.Name]++
	}
	dup := map[string]bool{}
	for name, n := range seen {
		if n > 1 {
			dup[name] = true
		}
	}
	return dup
}

// once runs one runner invocation with every process instrumented and reads
// what it observed. A non-empty reason means the run yields no record.
func (c *collector) once(ctx context.Context, spec string, only *result) ([]Observation, []result, time.Duration, string) {
	dir, err := c.newDir()
	if err != nil {
		return nil, nil, 0, err.Error()
	}
	start := time.Now()
	var ran []result
	err = c.invoke(ctx, dir, func(rctx context.Context) error {
		var err error
		ran, err = c.ad.run(rctx, c, dir, spec, only)
		return err
	})
	elapsed := time.Since(start)
	if err != nil {
		return nil, ran, elapsed, err.Error()
	}
	if only == nil {
		for _, r := range ran {
			if r.State == "failed" {
				return nil, ran, elapsed, "a test failed in the in-order run"
			}
		}
	}
	obs, reason := c.observe(dir, spec)
	return obs, ran, elapsed, reason
}

// newDir returns a fresh directory for one runner invocation.
func (c *collector) newDir() (string, error) {
	c.runs++
	dir := filepath.Join(c.Work, fmt.Sprintf("run%d", c.runs))
	return dir, os.MkdirAll(filepath.Join(dir, "node"), 0o755)
}

// invoke runs fn under the per-invocation timeout, with the server under
// test started first when the collector starts it.
func (c *collector) invoke(ctx context.Context, dir string, fn func(context.Context) error) error {
	rctx, cancel := context.WithTimeout(ctx, c.Timeout)
	defer cancel()
	if c.Server == "" {
		return fn(rctx)
	}
	stop, err := c.startServer(rctx, dir)
	if err != nil {
		return err
	}
	err = fn(rctx)
	if reason := stop(); reason != "" && err == nil {
		err = errors.New(reason)
	}
	return err
}

// env is the environment of every runner and server process of one run.
func (c *collector) env(dir string) []string {
	opts := strings.TrimSpace(os.Getenv("NODE_OPTIONS") + " --require " + quoteNodeOption(c.hook))
	env := os.Environ()
	if c.tmp != "" {
		env = append(env, "TMPDIR="+c.tmp)
	}
	return append(env,
		"NODE_OPTIONS="+opts,
		"NODE_V8_COVERAGE="+filepath.Join(dir, "node"),
		"CORVINT_BROWSERCOV_DIR="+dir,
		"CORVINT_BROWSERCOV_HOOK="+c.hook,
	)
}

func quoteNodeOption(p string) string {
	if !strings.ContainsAny(p, " \"\\") {
		return p
	}
	return strconv.Quote(p)
}

func (c *collector) command(ctx context.Context, dir, name string, args ...string) *exec.Cmd {
	cmd := exec.CommandContext(ctx, filepath.Join(c.Bin, name), args...)
	cmd.Dir = c.Root
	cmd.Env = c.env(dir)
	cmd.WaitDelay = 10 * time.Second
	setGroup(cmd)
	cmd.Cancel = func() error { return signalGroup(cmd.Process.Pid, sigKill) }
	return cmd
}

// runGroup runs a runner command, then kills whatever it left in its process
// group: a runner that dies abnormally leaves its web server holding the port.
func runGroup(cmd *exec.Cmd) error {
	err := cmd.Run()
	if cmd.Process != nil {
		_ = signalGroup(cmd.Process.Pid, sigKill)
	}
	return err
}

// isRunnerConfig reports whether a root-relative path is a runner
// configuration file, which only the runner loads.
func isRunnerConfig(rel string) bool {
	base := path.Base(rel)
	return strings.HasPrefix(base, "playwright.config.") || strings.HasPrefix(base, "cypress.config.")
}

// reapServers kills each process under test a run left alive: a web server
// that starts its own process group outlives the kill of the runner's group.
func reapServers(dir string) {
	entries, _ := os.ReadDir(dir)
	for _, e := range entries {
		if !strings.HasPrefix(e.Name(), "role-") {
			continue
		}
		var r roleFile
		body, err := os.ReadFile(filepath.Join(dir, e.Name()))
		if err != nil || json.Unmarshal(body, &r) != nil || r.Role != "process" || !alive(r.Pid) {
			continue
		}
		_ = signalGroup(r.Pid, sigKill)
		_ = signalPid(r.Pid, sigKill)
	}
}

type roleFile struct {
	Pid  int    `json:"pid"`
	Role string `json:"role"`
}

type browserFile struct {
	Browser string   `json:"browser"`
	Scripts []Script `json:"scripts"`
	Spec    string   `json:"spec"`
	Tests   []struct {
		Title []string `json:"title"`
		State string   `json:"state"`
	} `json:"tests"`
}

var nodeCoverageFile = regexp.MustCompile(`^coverage-(\d+)-`)

// observe joins one run's browser and Node coverage onto tracked files. Any
// note the hook left, any script it cannot bind, a declared server with no
// coverage, or a run whose browser recorded nothing makes it incomplete
// (LPCV-V0-954, LPCV-V0-972, LPCV-V0-974).
func (c *collector) observe(dir, spec string) ([]Observation, string) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err.Error()
	}
	roles := map[int]string{}
	var obs []Observation
	browsers := 0
	for _, e := range entries {
		name := e.Name()
		body, err := os.ReadFile(filepath.Join(dir, name))
		switch {
		case e.IsDir():
			continue
		case err != nil:
			return nil, err.Error()
		case strings.HasPrefix(name, "incomplete-"):
			var n struct{ Kind, Detail string }
			first, _, _ := strings.Cut(string(body), "\n")
			_ = json.Unmarshal([]byte(first), &n)
			return nil, fmt.Sprintf("hook: %s: %s", n.Kind, n.Detail)
		case strings.HasPrefix(name, "role-"):
			var r roleFile
			if err := json.Unmarshal(body, &r); err != nil {
				return nil, name + ": " + err.Error()
			}
			roles[r.Pid] = r.Role
		case strings.HasPrefix(name, "browser-"):
			var b browserFile
			if err := json.Unmarshal(body, &b); err != nil {
				return nil, name + ": " + err.Error()
			}
			if b.Browser != "" {
				if c.agent != "" && c.agent != b.Browser {
					return nil, fmt.Sprintf("browser %s differs from %s", b.Browser, c.agent)
				}
				c.agent = b.Browser
			}
			for _, s := range b.Scripts {
				o, reason := c.repo.Attribute(s)
				if reason != "" {
					return nil, reason
				}
				obs = append(obs, o...)
				browsers++
			}
		}
	}
	if browsers == 0 {
		return nil, "the browser reported no coverage"
	}
	nodeEntries, err := os.ReadDir(filepath.Join(dir, "node"))
	if err != nil {
		return nil, err.Error()
	}
	served := false
	for _, e := range nodeEntries {
		m := nodeCoverageFile.FindStringSubmatch(e.Name())
		if m == nil {
			continue
		}
		pid, _ := strconv.Atoi(m[1])
		runner := roles[pid] == "runner" || roles[pid] == "worker"
		body, err := os.ReadFile(filepath.Join(dir, "node", e.Name()))
		if err != nil {
			return nil, err.Error()
		}
		var d nodeDump
		if err := json.Unmarshal(body, &d); err != nil {
			return nil, e.Name() + ": " + err.Error()
		}
		for _, s := range d.Result {
			if runner {
				// The runner's processes run test code transpiled by the
				// runner, whose offsets are not offsets into the file; any
				// other repository code they ran is code no record can name.
				if rel, ok := c.repo.code(s.URL); ok && rel != spec && !isRunnerConfig(rel) {
					return nil, fmt.Sprintf("the test runner itself ran %s, whose lines a record cannot name", rel)
				}
				continue
			}
			o, reason := c.repo.Attribute(Script{URL: s.URL, Functions: s.Functions})
			if reason != "" {
				return nil, reason
			}
			served = served || len(o) > 0
			obs = append(obs, o...)
		}
	}
	if (c.Server != "" || c.ad.serves(c)) && !served {
		return nil, "no coverage from the server under test: it was not started by this run, or ran no tracked file"
	}
	return obs, ""
}

// refusals names the runners and platforms this collector declines.
const refusals = "Protractor, WebdriverIO, TestCafe, Nightwatch, Storybook test-runner and Detox are not supported; Windows is refused, since a process under test cannot be asked to write its coverage there"
