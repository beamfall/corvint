// Command str-parity is the STR-V0-013 parity check of the session test runner
// (docs/specs/session-test-runner-v0.md). For each first-parent pair (P, T) on
// a history it checks T out in a private shared clone and compares what
// `corvint test --dry-run` plans against what the two current drivers plan:
//
//   - the local driver, script/gate-affected.sh with the go-test command of the
//     checked-out Makefile, as `make gate-affected BASE=P` runs it;
//   - the CI driver, tools/corvint-pr-tests --mode run over the merge T of
//     P and its second parent, with no qualification pin.
//
// Every `go test` either driver starts is intercepted by a recording `go` on
// PATH, which delegates every other subcommand to the real toolchain, so
// nothing executes tests. It compares, per pair, the Go package set, the argv
// flags, the environment overlay and the fallback verdict, and names each
// difference with a category the build-log entry explains; a difference no
// rule recognises is UNEXPLAINED. It works offline on local objects only.
//
// Usage: str-parity -repo DIR -end OID -corvint BIN -selector BIN -pr-tests BIN
// -script FILE -source OID -work DIR [-pairs 50] > report.json
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
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"
)

var oid = regexp.MustCompile(`^[0-9a-f]{40}$`)

type options struct {
	repo, end, corvint, selector, prTests, script, source, work, realGo string
	corpus, declaration                                                 string
	pairs                                                               int
}

// Plan is one driver's planned go test: verdict, reason, package set, argv
// flags (the argv without its package arguments) and environment overlay.
type Plan struct {
	Argv     []string `json:"argv"`
	Env      []string `json:"env"`
	Error    string   `json:"error,omitempty"`
	Packages []string `json:"-"`
	Count    int      `json:"packageCount"`
	Reason   string   `json:"reason"`
	Scope    string   `json:"scope"`
	// Declared names the extra make targets corvint test would run.
	Declared []string `json:"declared,omitempty"`
	// Incomplete holds corvint's INCOMPLETE reasons as printed.
	Incomplete []string `json:"-"`
	// Mismatch names a go invocation whose package arguments differ from its
	// units; it is its own difference and never explained.
	Mismatch string `json:"mismatch,omitempty"`
}

// Difference is one compared field that disagrees, with its category.
type Difference struct {
	Category string `json:"category"`
	Detail   string `json:"detail"`
	Field    string `json:"field"`
	Profile  string `json:"profile"`
}

// Pair is one compared first-parent pair.
type Pair struct {
	Base        string       `json:"base"`
	CI          Plan         `json:"ci"`
	CIPlanned   string       `json:"ciPlanned"`
	CorvintCI   Plan         `json:"corvintCI"`
	CorvintLoc  Plan         `json:"corvintLocal"`
	Differences []Difference `json:"differences"`
	Head        string       `json:"head"`
	Local       Plan         `json:"local"`
	Target      string       `json:"target"`
	// CIPlannedPackages is the selector's planned set; CI.Packages is the set
	// the driver executes, which admission may widen to the universe.
	CIPlannedPackages []string `json:"-"`
	CIPlannedCount    int      `json:"ciPlannedPackageCount"`
	Root              string   `json:"-"`
	Universe          []string `json:"-"`
	UniverseCount     int      `json:"universeCount"`
}

// Report is the deterministic parity report: no timestamps, sorted members.
type Report struct {
	Categories map[string]int `json:"categories"`
	// Corpus is the sha256 of the -corpus pair file, when one was given.
	Corpus string `json:"corpus,omitempty"`
	// DeclarationDigest is the sha256 of the -declaration file every pair
	// ran with (STR-V0-003).
	DeclarationDigest string `json:"declarationDigest"`
	End               string `json:"end"`
	Pairs             []Pair `json:"pairs"`
	Profile           string `json:"profile"`
	Source            string `json:"source"`
}

func main() {
	var o options
	flag.StringVar(&o.repo, "repo", "", "repository holding the history's objects")
	flag.StringVar(&o.end, "end", "", "full OID of the newest first-parent commit")
	flag.IntVar(&o.pairs, "pairs", 50, "number of first-parent pairs ending at -end")
	flag.StringVar(&o.corpus, "corpus", "", "absolute retained pair file (tools/str-parity/testdata); sets -end and -pairs and must equal the history")
	flag.StringVar(&o.declaration, "declaration", "", "absolute .github/cishards/test-commands.json written into every checkout (default: the -repo file)")
	flag.StringVar(&o.corvint, "corvint", "", "absolute corvint binary (planner and runner)")
	flag.StringVar(&o.selector, "selector", "", "absolute gate-affected-select binary for the CI driver")
	flag.StringVar(&o.prTests, "pr-tests", "", "absolute corvint-pr-tests binary")
	flag.StringVar(&o.script, "script", "", "absolute script/gate-affected.sh of the driver source")
	flag.StringVar(&o.source, "source", "", "full OID the tools were built from")
	flag.StringVar(&o.work, "work", "", "absolute scratch directory that must not exist")
	flag.Parse()
	report, err := run(context.Background(), o)
	if err != nil {
		fmt.Fprintln(os.Stderr, "str-parity:", err)
		os.Exit(2)
	}
	encoded, err := json.MarshalIndent(report, "", " ")
	if err != nil {
		fmt.Fprintln(os.Stderr, "str-parity:", err)
		os.Exit(2)
	}
	// Every scratch path is spelled $WORK so the report does not depend on
	// where it ran.
	os.Stdout.Write(append(bytes.ReplaceAll(encoded, []byte(o.work), []byte("$WORK")), '\n'))
}

func run(ctx context.Context, o options) (Report, error) {
	for _, path := range []string{o.repo, o.corvint, o.selector, o.prTests, o.script, o.work} {
		if !filepath.IsAbs(path) {
			return Report{}, errors.New("-repo, -corvint, -selector, -pr-tests, -script and -work must be absolute")
		}
	}
	if o.declaration == "" {
		o.declaration = filepath.Join(o.repo, declarationPath)
	}
	if !filepath.IsAbs(o.declaration) || (o.corpus != "" && !filepath.IsAbs(o.corpus)) {
		return Report{}, errors.New("-corpus and -declaration must be absolute")
	}
	declaration, err := os.ReadFile(o.declaration)
	if err != nil {
		return Report{}, err
	}
	if declaredCommands, err = languageCommands(declaration); err != nil {
		return Report{}, err
	}
	var corpus []string
	corpusDigest := ""
	if o.corpus != "" {
		raw, err := os.ReadFile(o.corpus)
		if err != nil {
			return Report{}, err
		}
		if corpus, err = readCorpus(raw); err != nil {
			return Report{}, fmt.Errorf("-corpus: %w", err)
		}
		end := strings.Fields(corpus[0])[0]
		if o.end != "" && o.end != end {
			return Report{}, fmt.Errorf("-end %s is not the corpus end %s", o.end, end)
		}
		o.end, o.pairs, corpusDigest = end, len(corpus), sha256Hex(raw)
	}
	if !oid.MatchString(o.end) || !oid.MatchString(o.source) || o.pairs < 1 || o.pairs > 200 {
		return Report{}, errors.New("-end and -source must be full OIDs and -pairs 1..200")
	}
	if o.realGo, err = exec.LookPath("go"); err != nil {
		return Report{}, err
	}
	if o.realGo, err = filepath.EvalSymlinks(o.realGo); err != nil {
		return Report{}, err
	}
	if err := os.Mkdir(o.work, 0o700); err != nil {
		return Report{}, fmt.Errorf("-work must not exist: %w", err)
	}
	if err := os.WriteFile(filepath.Join(o.work, "test-commands.json"), declaration, 0o600); err != nil {
		return Report{}, err
	}
	clone := filepath.Join(o.work, "clone")
	if _, err := gitOut(ctx, o.work, "clone", "-q", "--shared", "--no-checkout", o.repo, clone); err != nil {
		return Report{}, err
	}
	for _, setting := range [][]string{{"gc.auto", "0"}, {"maintenance.auto", "false"}, {"advice.detachedHead", "false"}} {
		if _, err := gitOut(ctx, clone, "config", setting[0], setting[1]); err != nil {
			return Report{}, err
		}
	}
	if err := os.WriteFile(filepath.Join(clone, ".git", "info", "exclude"), []byte("/"+declarationPath+"\n"), 0o600); err != nil {
		return Report{}, err
	}
	bin, record := filepath.Join(o.work, "bin"), filepath.Join(o.work, "go-test.record")
	if err := os.MkdirAll(bin, 0o700); err != nil {
		return Report{}, err
	}
	fake := "#!/bin/sh\nif [ \"$1\" = test ]; then\n  { printf 'argv'; for a in \"$@\"; do printf '\\t%s' \"$a\"; done; printf '\\n'; env | LC_ALL=C sort | sed 's/^/env\\t/'; } >> " +
		shellQuote(record) + "\n  exit 0\nfi\nexec " + shellQuote(o.realGo) + " \"$@\"\n"
	if err := os.WriteFile(filepath.Join(bin, "go"), []byte(fake), 0o755); err != nil {
		return Report{}, err
	}
	listed, err := gitOut(ctx, o.repo, "rev-list", "--first-parent", "--parents", "-n", fmt.Sprint(o.pairs), o.end)
	if err != nil {
		return Report{}, err
	}
	lines := strings.Split(strings.TrimSpace(listed), "\n")
	if len(lines) != o.pairs {
		return Report{}, fmt.Errorf("history has %d first-parent commits, want %d", len(lines), o.pairs)
	}
	for index := range corpus {
		// The retained corpus is the parity input (STR-V0-013); the history
		// must still contain exactly those pairs.
		if strings.Join(strings.Fields(lines[index]), " ") != corpus[index] {
			return Report{}, fmt.Errorf("corpus line %d is %q, history has %q", index+1, corpus[index], lines[index])
		}
	}
	report := Report{Categories: map[string]int{}, Corpus: corpusDigest, DeclarationDigest: sha256Hex(declaration), End: o.end, Profile: "corvint-str-parity/0", Source: o.source}
	for index := len(lines) - 1; index >= 0; index-- {
		fields := strings.Fields(lines[index])
		if len(fields) != 3 {
			return Report{}, fmt.Errorf("%s is not a two-parent merge", fields[0])
		}
		pair, err := comparePair(ctx, o, clone, bin, record, fields[0], fields[1], fields[2], len(lines)-index)
		if err != nil {
			return Report{}, fmt.Errorf("%s: %w", fields[0], err)
		}
		for _, difference := range pair.Differences {
			report.Categories[difference.Category]++
		}
		report.Pairs = append(report.Pairs, pair)
		fmt.Fprintf(os.Stderr, "str-parity: %d/%d %s corvint=%s local=%s ci=%s/%s differences=%d\n", len(lines)-index, len(lines), fields[0][:12], pair.CorvintLoc.Scope, pair.Local.Scope, pair.CIPlanned, pair.CI.Scope, len(pair.Differences))
	}
	return report, nil
}

// environment is the controlled base environment every planner and driver
// starts from, so overlays are comparable and nothing inherited types them.
func environment(o options, bin string) []string {
	return []string{
		"PATH=" + bin + ":" + filepath.Dir(o.realGo) + ":/usr/bin:/bin", "HOME=" + filepath.Join(o.work, "home"),
		"TMPDIR=" + filepath.Join(o.work, "tmp"), "LANG=C", "LC_ALL=C", "TZ=UTC",
	}
}

// confinePath is the -exec wrapper the ci profile is asked to use; the
// CI driver has none (AFP-V0-028 confinement is the call site's).
const confinePath = "/str-parity/test-confine"

// ciGoEnv is the environment overlay corvint's ci profile sets besides
// CORVINT_CONFINE_ROOT (internal/sessiontest ciGoEnv, AFP-V0-028). A ci
// environment difference is explained only when corvint sets exactly these
// plus the confine root and the driver's environment is exactly its closed
// environment: ciDriverFixed, ciGoEnv, and each ciDriverPaths key once with
// any value (tools/corvint-pr-tests ciEnvironment).
var ciGoEnv = []string{"CGO_ENABLED=1", "GOFLAGS=", "GOPROXY=off", "GOSUMDB=off", "GOTOOLCHAIN=local", "GOWORK=off"}

var ciDriverFixed = []string{"CC=/usr/bin/cc", "GIT_CONFIG_COUNT=1", "GIT_CONFIG_GLOBAL=/dev/null", "GIT_CONFIG_KEY_0=advice.graftFileDeprecated",
	"GIT_CONFIG_NOSYSTEM=1", "GIT_CONFIG_VALUE_0=false", "GIT_GRAFT_FILE=/dev/null", "GIT_NO_REPLACE_OBJECTS=1", "GIT_TERMINAL_PROMPT=0",
	"GOENV=off", "GOMAXPROCS=2", "LANG=C", "LC_ALL=C", "TZ=UTC"}

var ciDriverPaths = []string{"GOCACHE", "GOMODCACHE", "HOME", "PATH", "TEMP", "TMP", "TMPDIR"}

func comparePair(ctx context.Context, o options, clone, bin, record, target, base, head string, ordinal int) (Pair, error) {
	pair := Pair{Base: base, Head: head, Root: clone, Target: target}
	if _, err := gitOut(ctx, clone, "checkout", "-q", "--detach", "--force", target); err != nil {
		return pair, err
	}
	if _, err := gitOut(ctx, clone, "clean", "-q", "-fdx"); err != nil {
		return pair, err
	}
	// Every checkout plans with the same declaration (STR-V0-003): the file
	// is written untracked, kept out of the dirty set through
	// .git/info/exclude, and removed before the cleanliness check.
	declared := filepath.Join(clone, declarationPath)
	if err := os.MkdirAll(filepath.Dir(declared), 0o700); err != nil {
		return pair, err
	}
	if err := copyFile(filepath.Join(o.work, "test-commands.json"), declared); err != nil {
		return pair, err
	}
	env := environment(o, bin)
	for _, dir := range []string{"home", "tmp"} {
		if err := os.MkdirAll(filepath.Join(o.work, dir), 0o700); err != nil {
			return pair, err
		}
	}
	universe, err := capture(ctx, clone, env, o.realGo, "list", "./...")
	if err != nil {
		return pair, fmt.Errorf("go list: %w", err)
	}
	all := strings.Fields(universe)
	pair.Universe, pair.UniverseCount = sortedCopy(all), len(all)
	pair.CorvintLoc = corvintPlan(ctx, o, clone, env, base, all, "--profile", "local")
	pair.CorvintCI = corvintPlan(ctx, o, clone, env, base, all, "--profile", "ci", "--confine", confinePath)
	pair.Local = localPlan(ctx, o, clone, env, record, base, all)
	pair.CI, pair.CIPlanned, pair.CIPlannedPackages = ciPlan(ctx, o, clone, env, base, head, target, ordinal, all)
	pair.CIPlannedCount = len(pair.CIPlannedPackages)
	pair.Differences = compare(pair)
	if err := os.Remove(declared); err != nil {
		return pair, err
	}
	if status, _ := gitOut(ctx, clone, "status", "--porcelain=v1", "--untracked-files=all", "--ignored"); status != "" {
		return pair, fmt.Errorf("a planner or driver changed the checkout: %s", status)
	}
	return pair, nil
}

// corvintPlan reads `corvint test --dry-run` for the one unsharded shard.
func corvintPlan(ctx context.Context, o options, clone string, env []string, base string, universe []string, profile ...string) Plan {
	args := append([]string{"--root", clone, "test", "--base", base, "--dry-run"}, profile...)
	stdout, stderr, code := runCommand(ctx, clone, env, o.corvint, args...)
	var printed struct {
		Incomplete  []string `json:"incomplete"`
		Invocations []struct {
			Adapter string   `json:"adapter"`
			Argv    []string `json:"argv"`
			Env     []string `json:"env"`
			Units   []struct {
				Unit string `json:"unit"`
			} `json:"units"`
		} `json:"invocations"`
		Reason string `json:"reason"`
		Scope  string `json:"scope"`
	}
	if err := json.Unmarshal([]byte(stdout), &printed); err != nil {
		return Plan{Error: fmt.Sprintf("exit %d: %s", code, firstLine(stderr)), Scope: "ERROR"}
	}
	plan := Plan{Reason: printed.Reason, Scope: printed.Scope, Env: []string{}, Argv: []string{}}
	if len(printed.Incomplete) != 0 {
		plan.Incomplete = append([]string{}, printed.Incomplete...)
		plan.Error = "INCOMPLETE: " + strings.Join(printed.Incomplete, "; ")
	}
	set := map[string]bool{}
	for _, invocation := range printed.Invocations {
		if invocation.Adapter != "go" {
			plan.Declared = append(plan.Declared, strings.Join(invocation.Argv, " "))
			continue
		}
		units := []string{}
		for _, unit := range invocation.Units {
			units = append(units, strings.TrimPrefix(unit.Unit, "go:"))
		}
		invoked, agrees := invokedPackages(invocation.Argv[1:], units, universe)
		if !agrees {
			plan.Mismatch = fmt.Sprintf("arguments name %v, units name %v", sortedKeys(invoked), sortedCopy(units))
		}
		for pkg := range invoked {
			set[pkg] = true
		}
		plan.Argv = flagsOnly(invocation.Argv[1:], invoked)
		plan.Env = sortedCopy(invocation.Env)
	}
	plan.Packages = sortedKeys(set)
	plan.Count = len(plan.Packages)
	return plan
}

// invokedPackages returns the packages a go invocation's arguments name, the
// set that is compared, and whether its units name exactly those packages.
func invokedPackages(argv, units, universe []string) (map[string]bool, bool) {
	invoked := packageSet(argv, universe)
	unitSet := map[string]bool{}
	for _, unit := range units {
		unitSet[unit] = true
	}
	return invoked, equalStrings(sortedKeys(invoked), sortedKeys(unitSet))
}

// localPlan runs the local driver as `make gate-affected BASE=P` does, with
// the checked-out Makefile's go-test command and the recording go.
func localPlan(ctx context.Context, o options, clone string, env []string, record, base string, universe []string) Plan {
	recipe, _, code := runCommand(ctx, clone, env, "make", "-s", "-n", "go-test")
	command, ok := strings.CutSuffix(strings.TrimSpace(recipe), " ./...")
	if code != 0 || !ok || strings.Contains(command, "\n") {
		return Plan{Error: "the go-test recipe is not one `... ./...` line", Scope: "ERROR"}
	}
	_ = os.Remove(record)
	driverEnv := append(append([]string{}, env...), "GO_TEST_COMMAND="+command, "CORVINT_PLANNER="+shellQuote(o.corvint))
	stdout, stderr, code := runCommand(ctx, clone, driverEnv, "/bin/sh", o.script, base)
	plan := Plan{Env: []string{}, Argv: []string{}}
	switch {
	case strings.Contains(stderr, "gate-affected: FALLBACK "):
		plan.Scope = "FULL"
		reason := afterPrefix(stderr, "gate-affected: FALLBACK ")
		plan.Reason = strings.TrimSuffix(reason, "; running the full go-test command")
	case strings.Contains(stdout, "gate-affected: run "):
		plan.Scope = "NARROWED"
	default:
		plan.Scope = "NOTHING"
		plan.Reason = lastLine(stdout)
		plan.Reason = strings.TrimPrefix(plan.Reason, "gate-affected: ")
	}
	argv, overlay, err := readRecord(record, env)
	switch {
	case err != nil && plan.Scope != "NOTHING":
		plan.Error = fmt.Sprintf("exit %d: no go test recorded: %v", code, err)
	case err == nil && plan.Scope == "NOTHING":
		plan.Error = "NOTHING started go test"
	case err == nil:
		set := packageSet(argv, universe)
		plan.Argv, plan.Env = flagsOnly(argv, set), overlay
		plan.Packages = sortedKeys(set)
	}
	if code != 0 {
		plan.Error = strings.TrimSpace(plan.Error + fmt.Sprintf(" exit %d: %s", code, firstLine(stderr)))
	}
	plan.Count = len(plan.Packages)
	return plan
}

// ciPlan runs the CI driver over the merge and returns its executed plan, and
// the selector verdict and package set it would have executed with a
// qualification pin.
func ciPlan(ctx context.Context, o options, clone string, env []string, base, head, target string, ordinal int, universe []string) (Plan, string, []string) {
	out := filepath.Join(o.work, "ci", fmt.Sprintf("%03d", ordinal))
	runtime := filepath.Join(o.work, "ci-runtime")
	_ = os.RemoveAll(runtime)
	_, stderr, code := runCommand(ctx, clone, env, o.prTests, "--mode", "run", "--root", clone, "--base", base, "--head", head,
		"--target", target, "--source", o.source, "--planner", o.corvint, "--selector", o.selector, "--out", out, "--runtime", runtime)
	var execution struct {
		Args      []string
		Env       []string
		Selection struct {
			Packages []string
			Reason   string
		}
	}
	plan := Plan{Env: []string{}, Argv: []string{}}
	raw, err := os.ReadFile(filepath.Join(out, "execution.json"))
	if err == nil {
		err = json.Unmarshal(raw, &execution)
	}
	if err != nil || code != 0 {
		plan.Error, plan.Scope = fmt.Sprintf("exit %d: %s", code, firstLine(stderr)), "ERROR"
		return plan, "ERROR", nil
	}
	plan.Reason = execution.Selection.Reason
	plan.Scope = "NARROWED"
	if plan.Reason != "" {
		plan.Scope = "FULL"
	}
	plan.Env = sortedCopy(execution.Env)
	audit, _ := os.ReadFile(filepath.Join(out, "selector.txt"))
	verdict := lastLine(string(audit))
	planned, packages := "FULL", universe
	switch {
	case strings.HasPrefix(verdict, "run "):
		planned, packages = "NARROWED", strings.Fields(strings.TrimPrefix(verdict, "run "))
	case strings.HasPrefix(verdict, "NOTHING"):
		planned, packages = "NOTHING", nil
	}
	if plan.Scope == "NARROWED" {
		packages = execution.Selection.Packages
	}
	executed := packageSet(execution.Args, universe)
	plan.Packages = sortedKeys(executed)
	plan.Count = len(plan.Packages)
	plan.Argv = flagsOnly(execution.Args, executed)
	return plan, planned, sortedCopy(packages)
}

// compare lists every disagreement between corvint test and the driver it
// would replace, with its category.
func compare(pair Pair) []Difference {
	differences := []Difference{}
	add := func(profile, field, detail string, driverOnly ...string) {
		differences = append(differences, Difference{Category: categorise(pair, profile, field, driverOnly), Detail: detail, Field: field, Profile: profile})
	}
	for _, side := range []struct {
		profile         string
		corvint, driver Plan
		driverScope     string
	}{
		{"local", pair.CorvintLoc, pair.Local, pair.Local.Scope},
		{"ci", pair.CorvintCI, pair.CI, pair.CIPlanned},
	} {
		if side.corvint.Mismatch != "" {
			add(side.profile, "units", side.corvint.Mismatch)
		}
		if side.corvint.Error != "" || side.driver.Error != "" {
			add(side.profile, "error", fmt.Sprintf("corvint %q driver %q", side.corvint.Error, side.driver.Error))
		}
		if side.corvint.Scope != side.driverScope {
			add(side.profile, "verdict", fmt.Sprintf("corvint %s (%s), driver %s (%s)", side.corvint.Scope, side.corvint.Reason, side.driverScope, side.driver.Reason))
		}
		if side.profile == "ci" && pair.CI.Scope != pair.CIPlanned {
			add(side.profile, "admission", fmt.Sprintf("driver plans %s and executes %s: %s", pair.CIPlanned, pair.CI.Scope, pair.CI.Reason))
		}
		driverPackages := side.driver.Packages
		if side.profile == "ci" {
			driverPackages = pair.CIPlannedPackages
		}
		if only, missing := setDifference(side.corvint.Packages, driverPackages); len(only)+len(missing) != 0 {
			add(side.profile, "packages", fmt.Sprintf("corvint only %d %s; driver only %d %s", len(only), clip(only), len(missing), clip(missing)), missing...)
		}
		if side.profile == "ci" && side.corvint.Scope != "ERROR" && side.driver.Error == "" { // an INCOMPLETE plan still names its packages
			if only, missing := setDifference(side.corvint.Packages, side.driver.Packages); len(only)+len(missing) != 0 {
				add(side.profile, "executed", fmt.Sprintf("driver executes %d packages: corvint only %d %s; driver only %d %s", side.driver.Count, len(only), clip(only), len(missing), clip(missing)), only...)
			}
		}
		if side.corvint.Count != 0 && len(side.driver.Argv) != 0 && !equalStrings(side.corvint.Argv, side.driver.Argv) {
			add(side.profile, "argv", fmt.Sprintf("corvint %v, driver %v", side.corvint.Argv, side.driver.Argv))
		}
		if side.corvint.Count != 0 && len(side.driver.Argv) != 0 && !equalStrings(side.corvint.Env, side.driver.Env) {
			add(side.profile, "env", fmt.Sprintf("corvint %v, driver %v", side.corvint.Env, side.driver.Env))
		}
		if len(side.corvint.Declared) != 0 {
			add(side.profile, "declared", fmt.Sprintf("corvint also runs %v", side.corvint.Declared))
		}
	}
	return differences
}

// categorise names the explanation for one difference; the build-log entry
// explains each category, and UNEXPLAINED marks one no rule recognises.
// For "packages", extra holds the packages only the driver runs, which an
// explained widening must not have; for "executed", the packages only corvint
// runs, which an explained admission widening must not have.
func categorise(pair Pair, profile, field string, extra []string) string {
	corvint := pair.CorvintLoc
	if profile == "ci" {
		corvint = pair.CorvintCI
	}
	widened := corvint.Scope == "FULL" && strings.HasPrefix(corvint.Reason, "plan.unknown entry outside the AFP-V0-028 allowlist: ")
	switch field {
	case "error":
		if undeclaredOnly(corvint) && pair.Local.Error == "" && (profile == "local" || pair.CI.Error == "") {
			return "str-v0-004-undeclared-language"
		}
	case "admission", "executed":
		if pair.CI.Scope != "FULL" || !equalStrings(pair.CI.Packages, pair.Universe) || (field == "executed" && len(extra) != 0) {
			break
		}
		if strings.HasPrefix(pair.CI.Reason, "qualification unavailable: ") {
			return "ci-admission-no-qualification"
		}
		if strings.HasPrefix(pair.CI.Reason, "selector: NOTHING") && pair.CIPlanned == "NOTHING" {
			return "ci-nothing-runs-full"
		}
	case "verdict":
		if widened {
			return "str-v0-003-unknown-widening"
		}
	case "packages":
		if widened && len(extra) == 0 && equalStrings(corvint.Packages, pair.Universe) {
			return "str-v0-003-unknown-widening"
		}
	case "argv":
		if profile == "local" && equalStrings(withoutJSON(corvint.Argv), pair.Local.Argv) {
			return "local-json-flag"
		}
		if profile == "ci" && confinedOnce(corvint.Argv) && equalStrings(withoutExec(corvint.Argv), pair.CI.Argv) {
			return "ci-exec-confine"
		}
	case "env":
		if profile == "ci" && expectedCIEnv(corvint.Env, pair.CI.Env, pair.Root) {
			return "ci-closed-environment"
		}
	case "declared":
		if len(corvint.Declared) != 0 && allDeclared(corvint.Declared) {
			return "str-v0-004-declared-language"
		}
	}
	return "UNEXPLAINED"
}

// declaredCommands is each language command the -declaration file names,
// its argv joined by single spaces, in sorted order.
var declaredCommands []string

// languageCommands reads the language commands out of the declaration bytes
// (corvint itself reads them strictly; this only needs the argv lists), so a
// `declared` difference is explained when corvint runs exactly a declared
// command (STR-V0-004: a language with selected units and no adapter runs
// its declared full command, which neither driver runs).
func languageCommands(declaration []byte) ([]string, error) {
	var parsed struct {
		Languages map[string]json.RawMessage `json:"languages"`
	}
	if err := json.Unmarshal(declaration, &parsed); err != nil {
		return nil, fmt.Errorf("-declaration: %v", err)
	}
	commands := []string{}
	for _, raw := range parsed.Languages {
		var argv []string
		if json.Unmarshal(raw, &argv) == nil && len(argv) != 0 {
			commands = append(commands, strings.Join(argv, " "))
		}
	}
	sort.Strings(commands)
	return commands, nil
}

// allDeclared reports whether every command corvint also runs is one the
// declaration names.
func allDeclared(commands []string) bool {
	for _, command := range commands {
		if !containsArg(declaredCommands, command) {
			return false
		}
	}
	return true
}

// confinedOnce reports whether argv is `test -exec <confinePath> ...` with no
// other -exec.
func confinedOnce(argv []string) bool {
	return len(argv) >= 3 && argv[0] == "test" && argv[1] == "-exec" && argv[2] == confinePath && !containsArg(argv[3:], "-exec")
}

// expectedCIEnv reports whether corvint's ci overlay is exactly ciGoEnv plus
// CORVINT_CONFINE_ROOT=<root>, and the CI driver's environment is exactly
// ciDriverFixed and ciGoEnv plus one entry for each ciDriverPaths key.
func expectedCIEnv(corvint, driver []string, root string) bool {
	want := append([]string{"CORVINT_CONFINE_ROOT=" + root}, ciGoEnv...)
	if !equalStrings(sortedCopy(corvint), sortedCopy(want)) {
		return false
	}
	fixed := []string{}
	paths := []string{}
	for _, entry := range driver {
		if key, _, _ := strings.Cut(entry, "="); containsArg(ciDriverPaths, key) {
			paths = append(paths, key)
		} else {
			fixed = append(fixed, entry)
		}
	}
	return equalStrings(sortedCopy(fixed), sortedCopy(append(append([]string{}, ciDriverFixed...), ciGoEnv...))) &&
		equalStrings(sortedCopy(paths), sortedCopy(ciDriverPaths))
}

func containsArg(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}

// undeclaredOnly reports whether the plan's only error is INCOMPLETE and
// every reason is exactly "<language>: no qualified adapter and
// .github/cishards/test-commands.json declares no <language> full command"
// (STR-V0-004).
func undeclaredOnly(plan Plan) bool {
	if len(plan.Incomplete) == 0 || plan.Error != "INCOMPLETE: "+strings.Join(plan.Incomplete, "; ") {
		return false
	}
	for _, reason := range plan.Incomplete {
		language, _, _ := strings.Cut(reason, ": ")
		if language == "" || strings.ContainsAny(language, " ;:") || reason != language+": no qualified adapter and "+declarationPath+" declares no "+language+" full command" {
			return false
		}
	}
	return true
}

// declarationPath is where corvint test reads the full-suite declaration
// (internal/sessiontest DeclarationPath).
const declarationPath = ".github/cishards/test-commands.json"

// readCorpus reads a retained pair file: comment and blank lines skipped,
// every other line "<merge> <first parent> <second parent>" in full OIDs,
// newest first, at least one line, no merge repeated.
func readCorpus(raw []byte) ([]string, error) {
	lines := []string{}
	seen := map[string]bool{}
	for number, line := range strings.Split(string(raw), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) != 3 || !oid.MatchString(fields[0]) || !oid.MatchString(fields[1]) || !oid.MatchString(fields[2]) {
			return nil, fmt.Errorf("line %d: want three full OIDs, got %q", number+1, line)
		}
		if seen[fields[0]] {
			return nil, fmt.Errorf("line %d: merge %s repeated", number+1, fields[0])
		}
		seen[fields[0]] = true
		lines = append(lines, strings.Join(fields, " "))
	}
	if len(lines) == 0 {
		return nil, errors.New("no pairs")
	}
	return lines, nil
}

func sha256Hex(raw []byte) string {
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:])
}

func copyFile(from, to string) error {
	raw, err := os.ReadFile(from)
	if err != nil {
		return err
	}
	return os.WriteFile(to, raw, 0o600)
}

func withoutJSON(argv []string) []string {
	out := []string{}
	for _, arg := range argv {
		if arg != "-json" {
			out = append(out, arg)
		}
	}
	return out
}

func withoutExec(argv []string) []string {
	out := []string{}
	for index := 0; index < len(argv); index++ {
		if argv[index] == "-exec" && index+1 < len(argv) {
			index++
			continue
		}
		out = append(out, argv[index])
	}
	return out
}

// readRecord returns the first recorded go test argv and its environment
// overlay: each variable whose value differs from the base environment.
func readRecord(record string, base []string) ([]string, []string, error) {
	raw, err := os.ReadFile(record)
	if err != nil {
		return nil, nil, err
	}
	lines := strings.Split(strings.TrimSuffix(string(raw), "\n"), "\n")
	if len(lines) == 0 || !strings.HasPrefix(lines[0], "argv\t") {
		return nil, nil, errors.New("malformed record")
	}
	if strings.Count(string(raw), "argv\t") != 1 {
		return nil, nil, errors.New("more than one go test recorded")
	}
	argv := strings.Split(strings.TrimPrefix(lines[0], "argv\t"), "\t")
	recorded := []string{}
	for _, line := range lines[1:] {
		if value, ok := strings.CutPrefix(line, "env\t"); ok {
			recorded = append(recorded, value)
		}
	}
	return argv, overlay(recorded, base), nil
}

// overlay keeps each recorded NAME=VALUE absent from base, ignoring the
// variables a shell maintains itself.
func overlay(recorded, base []string) []string {
	inBase := map[string]bool{}
	for _, entry := range base {
		inBase[entry] = true
	}
	out := []string{}
	for _, entry := range recorded {
		name, _, _ := strings.Cut(entry, "=")
		switch name {
		case "PWD", "OLDPWD", "SHLVL", "_", "GO_TEST_COMMAND", "CORVINT_PLANNER":
			continue
		}
		if !inBase[entry] {
			out = append(out, entry)
		}
	}
	sort.Strings(out)
	return out
}

// packageSet returns the package arguments of a go test argv: ./... expands
// to universe; every other non-flag argument that is not a flag's value.
func packageSet(args []string, universe []string) map[string]bool {
	set := map[string]bool{}
	valued := map[string]bool{"-p": true, "-timeout": true, "-exec": true, "-run": true, "-skip": true}
	for index := 1; index < len(args); index++ { // args[0] is "test"
		arg := args[index]
		switch {
		case valued[arg]:
			index++
		case strings.HasPrefix(arg, "-"):
		case arg == "./...":
			for _, pkg := range universe {
				set[pkg] = true
			}
		default:
			set[arg] = true
		}
	}
	delete(set, "./...")
	return set
}

// flagsOnly drops the package arguments (and ./...) from argv.
func flagsOnly(argv []string, packages map[string]bool) []string {
	out := []string{}
	for _, arg := range argv {
		if !packages[arg] && arg != "./..." {
			out = append(out, arg)
		}
	}
	return out
}

func runCommand(ctx context.Context, dir string, env []string, name string, args ...string) (string, string, int) {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Minute)
	defer cancel()
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Dir, cmd.Env = dir, env
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	err := cmd.Run()
	code := 0
	var exit *exec.ExitError
	if errors.As(err, &exit) {
		code = exit.ExitCode()
	} else if err != nil {
		code = -1
		stderr.WriteString(err.Error())
	}
	return stdout.String(), stderr.String(), code
}

func capture(ctx context.Context, dir string, env []string, name string, args ...string) (string, error) {
	stdout, stderr, code := runCommand(ctx, dir, env, name, args...)
	if code != 0 {
		return "", fmt.Errorf("exit %d: %s", code, firstLine(stderr))
	}
	return stdout, nil
}

func gitOut(ctx context.Context, dir string, args ...string) (string, error) {
	env := []string{"PATH=/usr/bin:/bin", "GIT_CONFIG_NOSYSTEM=1", "GIT_CONFIG_GLOBAL=/dev/null", "GIT_TERMINAL_PROMPT=0", "LC_ALL=C"}
	return capture(ctx, dir, env, "git", args...)
}

func setDifference(a, b []string) (onlyA, onlyB []string) {
	inA, inB := map[string]bool{}, map[string]bool{}
	for _, v := range a {
		inA[v] = true
	}
	for _, v := range b {
		inB[v] = true
	}
	for _, v := range a {
		if !inB[v] {
			onlyA = append(onlyA, v)
		}
	}
	for _, v := range b {
		if !inA[v] {
			onlyB = append(onlyB, v)
		}
	}
	sort.Strings(onlyA)
	sort.Strings(onlyB)
	return onlyA, onlyB
}

// clip names at most five packages so the report stays bounded.
func clip(values []string) string {
	if len(values) > 5 {
		return fmt.Sprintf("%v ...", values[:5])
	}
	return fmt.Sprint(values)
}

func equalStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func sortedKeys(set map[string]bool) []string {
	out := []string{}
	for key := range set {
		out = append(out, key)
	}
	sort.Strings(out)
	return out
}

func sortedCopy(values []string) []string {
	out := append([]string{}, values...)
	sort.Strings(out)
	return out
}

func firstLine(text string) string {
	line, _, _ := strings.Cut(strings.TrimSpace(text), "\n")
	return line
}

func lastLine(text string) string {
	lines := strings.Split(strings.TrimSpace(text), "\n")
	return lines[len(lines)-1]
}

func afterPrefix(text, prefix string) string {
	_, rest, _ := strings.Cut(text, prefix)
	return firstLine(rest)
}

func shellQuote(value string) string {
	return "'" + strings.ReplaceAll(value, "'", `'\''`) + "'"
}
