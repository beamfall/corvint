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
}

// Report is the deterministic parity report: no timestamps, sorted members.
type Report struct {
	Categories map[string]int `json:"categories"`
	End        string         `json:"end"`
	Pairs      []Pair         `json:"pairs"`
	Profile    string         `json:"profile"`
	Source     string         `json:"source"`
}

func main() {
	var o options
	flag.StringVar(&o.repo, "repo", "", "repository holding the history's objects")
	flag.StringVar(&o.end, "end", "", "full OID of the newest first-parent commit")
	flag.IntVar(&o.pairs, "pairs", 50, "number of first-parent pairs ending at -end")
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
	if !oid.MatchString(o.end) || !oid.MatchString(o.source) || o.pairs < 1 || o.pairs > 200 {
		return Report{}, errors.New("-end and -source must be full OIDs and -pairs 1..200")
	}
	var err error
	if o.realGo, err = exec.LookPath("go"); err != nil {
		return Report{}, err
	}
	if o.realGo, err = filepath.EvalSymlinks(o.realGo); err != nil {
		return Report{}, err
	}
	if err := os.Mkdir(o.work, 0o700); err != nil {
		return Report{}, fmt.Errorf("-work must not exist: %w", err)
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
	report := Report{Categories: map[string]int{}, End: o.end, Profile: "corvint-str-parity/0", Source: o.source}
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

func comparePair(ctx context.Context, o options, clone, bin, record, target, base, head string, ordinal int) (Pair, error) {
	pair := Pair{Base: base, Head: head, Target: target}
	if _, err := gitOut(ctx, clone, "checkout", "-q", "--detach", "--force", target); err != nil {
		return pair, err
	}
	if _, err := gitOut(ctx, clone, "clean", "-q", "-fdx"); err != nil {
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
	pair.CorvintLoc = corvintPlan(ctx, o, clone, env, base, "--profile", "local")
	pair.CorvintCI = corvintPlan(ctx, o, clone, env, base, "--profile", "ci", "--confine", "/str-parity/test-confine")
	pair.Local = localPlan(ctx, o, clone, env, record, base, all)
	var ciPlanned string
	pair.CI, ciPlanned = ciPlan(ctx, o, clone, env, base, head, target, ordinal, all)
	pair.CIPlanned = ciPlanned
	pair.Differences = compare(pair)
	if status, _ := gitOut(ctx, clone, "status", "--porcelain=v1", "--untracked-files=all", "--ignored"); status != "" {
		return pair, fmt.Errorf("a planner or driver changed the checkout: %s", status)
	}
	return pair, nil
}

// corvintPlan reads `corvint test --dry-run` for the one unsharded shard.
func corvintPlan(ctx context.Context, o options, clone string, env []string, base string, profile ...string) Plan {
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
		plan.Error = "INCOMPLETE: " + strings.Join(printed.Incomplete, "; ")
	}
	set := map[string]bool{}
	for _, invocation := range printed.Invocations {
		if invocation.Adapter != "go" {
			plan.Declared = append(plan.Declared, strings.Join(invocation.Argv, " "))
			continue
		}
		for _, unit := range invocation.Units {
			set[strings.TrimPrefix(unit.Unit, "go:")] = true
		}
		plan.Argv = flagsOnly(invocation.Argv[1:], set)
		plan.Env = sortedCopy(invocation.Env)
	}
	plan.Packages = sortedKeys(set)
	plan.Count = len(plan.Packages)
	return plan
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

// ciPlan runs the CI driver over the merge and returns its effective plan and
// the selector verdict it would have executed with a qualification pin.
func ciPlan(ctx context.Context, o options, clone string, env []string, base, head, target string, ordinal int, universe []string) (Plan, string) {
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
		return plan, "ERROR"
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
	set := map[string]bool{}
	for _, pkg := range packages {
		set[pkg] = true
	}
	plan.Packages = sortedKeys(set)
	plan.Count = len(plan.Packages)
	plan.Argv = flagsOnly(execution.Args, packageSet(execution.Args, nil))
	return plan, planned
}

// compare lists every disagreement between corvint test and the driver it
// would replace, with its category.
func compare(pair Pair) []Difference {
	differences := []Difference{}
	add := func(profile, field, detail string) {
		differences = append(differences, Difference{Category: categorise(pair, profile, field, detail), Detail: detail, Field: field, Profile: profile})
	}
	for _, side := range []struct {
		profile         string
		corvint, driver Plan
		driverScope     string
	}{
		{"local", pair.CorvintLoc, pair.Local, pair.Local.Scope},
		{"ci", pair.CorvintCI, pair.CI, pair.CIPlanned},
	} {
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
		if side.profile == "ci" && pair.CIPlanned == "NOTHING" {
			driverPackages = nil
		}
		if only, missing := setDifference(side.corvint.Packages, driverPackages); len(only)+len(missing) != 0 {
			add(side.profile, "packages", fmt.Sprintf("corvint only %d %s; driver only %d %s", len(only), clip(only), len(missing), clip(missing)))
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
func categorise(pair Pair, profile, field, detail string) string {
	corvint := pair.CorvintLoc
	if profile == "ci" {
		corvint = pair.CorvintCI
	}
	switch field {
	case "error":
		if strings.HasPrefix(corvint.Error, "INCOMPLETE: ") && undeclaredOnly(strings.TrimPrefix(corvint.Error, "INCOMPLETE: ")) && pair.Local.Error == "" && (profile == "local" || pair.CI.Error == "") {
			return "str-v0-004-undeclared-language"
		}
	case "admission":
		if strings.HasPrefix(pair.CI.Reason, "qualification unavailable: ") {
			return "ci-admission-no-qualification"
		}
		if strings.HasPrefix(pair.CI.Reason, "selector: NOTHING") {
			return "ci-nothing-runs-full"
		}
	case "verdict":
		if corvint.Scope == "FULL" && strings.HasPrefix(corvint.Reason, "plan.unknown entry outside the AFP-V0-028 allowlist: ") {
			return "str-v0-003-unknown-widening"
		}
	case "packages":
		if corvint.Scope == "FULL" && strings.HasPrefix(corvint.Reason, "plan.unknown entry outside the AFP-V0-028 allowlist: ") {
			return "str-v0-003-unknown-widening"
		}
	case "argv":
		if profile == "local" && equalStrings(withoutJSON(corvint.Argv), pair.Local.Argv) {
			return "local-json-flag"
		}
		if profile == "ci" && equalStrings(withoutExec(corvint.Argv), pair.CI.Argv) {
			return "ci-exec-confine"
		}
	case "env":
		if profile == "ci" && closedSuperset(corvint.Env, pair.CI.Env) {
			return "ci-closed-environment"
		}
	case "declared":
		return "full-declared-languages"
	}
	return "UNEXPLAINED"
}

// closedSuperset reports whether the CI driver's closed environment holds
// every variable corvint sets except CORVINT_CONFINE_ROOT, which only the
// AFP-V0-028 confined branch needs.
func closedSuperset(corvint, driver []string) bool {
	closed := map[string]bool{}
	for _, entry := range driver {
		closed[entry] = true
	}
	for _, entry := range corvint {
		if !closed[entry] && !strings.HasPrefix(entry, "CORVINT_CONFINE_ROOT=") {
			return false
		}
	}
	return true
}

// undeclaredOnly reports whether every INCOMPLETE reason is a language with
// no adapter and no declared full command (STR-V0-004).
func undeclaredOnly(reasons string) bool {
	for _, reason := range strings.Split(reasons, "; ") {
		if !strings.Contains(reason, ": no qualified adapter and no declared ") {
			return false
		}
	}
	return true
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
