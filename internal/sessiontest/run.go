package sessiontest

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os/exec"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"time"
)

// Terminal outcomes of one execution unit (STR-V0-010).
const (
	OutcomePass        = "PASS"
	OutcomeNoTests     = "NO_TESTS"
	OutcomeFail        = "FAIL"
	OutcomeBuildFailed = "BUILD_FAILED"
	OutcomeTimedOut    = "TIMED_OUT"
	OutcomeNotRun      = "NOT_RUN"
)

// DefaultBudget is the total wall-clock budget of one local shard, every
// adapter invocation of the shard included (STR-V0-015). It is an outer hang
// bound, not a speed target: at the declared -p 1 a change reaching about 236
// packages outran 70 minutes, and the script it replaces had no total budget.
const DefaultBudget = 6 * time.Hour

// CIBudget is the total wall-clock budget of one ci-profile shard. It sits
// below the 75-minute timeout of the go-product-shard job in
// .github/workflows/ci.yml, leaving room for checkout, toolchain setup and the
// receipt upload, so the job timeout never kills a shard the runner still owns
// (STR-V0-015).
const CIBudget = 60 * time.Minute

// ShardBudget is the budget Execute applies to a shard of m: the runner's own
// when set, otherwise the profile default.
func (r *Runner) ShardBudget(m Manifest) time.Duration {
	switch {
	case r.Budget > 0:
		return r.Budget
	case m.RunnerProfile == ProfileCI:
		return CIBudget
	default:
		return DefaultBudget
	}
}

// discoverDeadline bounds each runner version and universe probe.
const discoverDeadline = 2 * time.Minute

// ciGoEnv and ciGoFlags are the full CI branch's go test environment and
// flags (AFP-V0-028, STR-V0-006); CORVINT_CONFINE_ROOT is added per root.
var (
	ciGoEnv   = []string{"GOTOOLCHAIN=local", "GOPROXY=off", "GOSUMDB=off", "GOWORK=off", "GOFLAGS=", "CGO_ENABLED=1"}
	ciGoFlags = []string{"-p", "1", "-count=1", "-race", "-timeout", "50m"}
)

// Outcome is one execution unit's terminal outcome.
type Outcome struct {
	Outcome string `json:"outcome"`
	Slice   string `json:"slice"`
	Unit    string `json:"unit"`
}

// Invocation is one adapter launch as the receipt records it.
// AdapterManifest is the capability manifest version the adapter ran under
// (STR-V0-059).
type Invocation struct {
	Adapter             string    `json:"adapter"`
	AdapterManifest     string    `json:"adapterManifest"`
	Argv                []string  `json:"argv"`
	Dir                 string    `json:"dir,omitempty"`
	ElapsedMilliseconds int64     `json:"elapsedMilliseconds"`
	Env                 []string  `json:"env"`
	EnvDigest           string    `json:"envDigest"`
	Exit                int       `json:"exit"`
	Outcomes            []Outcome `json:"outcomes"`
	Runner              string    `json:"runner"`
	Version             string    `json:"version"`
}

// Receipt is the STR-V0-010 corvint-test-run/0 document of one shard.
type Receipt struct {
	Base   string   `json:"base"`
	Checks []string `json:"checks"`
	// Excluded copies the manifest's STR-V0-069 list; absent when empty.
	Excluded        []string       `json:"excluded,omitempty"`
	GraphDigest     string         `json:"graphDigest"`
	Incomplete      []string       `json:"incomplete"`
	Invocations     []Invocation   `json:"invocations"`
	ManifestDigest  string         `json:"manifestDigest"`
	PartitionDigest string         `json:"partitionDigest"`
	Placement       string         `json:"placement"`
	PlanDigest      string         `json:"planDigest"`
	Profile         string         `json:"profile"`
	Reason          string         `json:"reason"`
	Result          string         `json:"result"`
	Revision        string         `json:"revision"`
	RunnerProfile   string         `json:"runnerProfile"`
	Scope           string         `json:"scope"`
	ShardCount      int            `json:"shardCount"`
	ShardIndex      int            `json:"shardIndex"`
	Source          SourceIdentity `json:"source"`
	SourceDigest    string         `json:"sourceDigest"`
	Summary         string         `json:"summary"`
	Tool            string         `json:"tool"`
}

// Planned is one adapter invocation derived from the manifest.
type Planned struct {
	Adapter string
	Argv    []string
	// Dir is the repository-relative directory a nested-module invocation
	// runs from (STR-V0-064); empty is the repository root.
	Dir   string
	Env   []string
	Units []Unit
}

// Invocations derives one shard's typed argv (STR-V0-005, STR-V0-006): one
// go test over the shard's whole packages, one per test slice, and one
// invocation per declared unit: `declared:go` is the declaration's Go
// environment and flags over ./... (ci: the CI branch's), any other the
// declared language argv. A nested-module unit is its own go test ./...
// from the module directory with GOWORK=off (STR-V0-064). Only the target
// set differs between a narrowed and a FULL run.
func Invocations(m Manifest, declared Declared, shard int, root, confine string) []Planned {
	var whole []Unit
	var planned []Planned
	goEnv, goFlags := append([]string{}, declared.GoEnv...), append([]string{}, declared.GoFlags...)
	goPrefix := append([]string{"go", "test", "-json"}, goFlags...)
	if m.RunnerProfile == ProfileCI {
		goEnv, goFlags = append([]string{"CORVINT_CONFINE_ROOT=" + root}, ciGoEnv...), append([]string{"-exec", confine}, ciGoFlags...)
		goPrefix = append([]string{"go", "test", "-exec", confine, "-json"}, ciGoFlags...)
	}
	for _, unit := range m.Units {
		if unit.Shard != shard || unit.Adapter != AdapterGo || unit.Slice != SliceWhole || unit.Dir != "" {
			continue
		}
		whole = append(whole, unit)
	}
	if len(whole) != 0 {
		argv := append([]string{}, goPrefix...)
		for _, unit := range whole {
			argv = append(argv, strings.TrimPrefix(unit.Unit, "go:"))
		}
		planned = append(planned, Planned{Adapter: AdapterGo, Argv: argv, Env: goEnv, Units: whole})
	}
	for _, unit := range m.Units {
		if unit.Shard != shard {
			continue
		}
		switch name := strings.TrimPrefix(unit.Unit, "declared:"); {
		case unit.Adapter == AdapterGo && unit.Dir != "":
			argv := append(append([]string{}, goPrefix...), "./...")
			env := append(append([]string{}, goEnv...), "GOWORK=off")
			planned = append(planned, Planned{Adapter: AdapterGo, Argv: argv, Dir: unit.Dir, Env: env, Units: []Unit{unit}})
		case unit.Adapter == AdapterGo && unit.Slice != SliceWhole:
			argv := append(append([]string{}, goPrefix...), unit.Flag, unit.Pattern, strings.TrimPrefix(unit.Unit, "go:"))
			planned = append(planned, Planned{Adapter: AdapterGo, Argv: argv, Env: goEnv, Units: []Unit{unit}})
		case unit.Adapter == AdapterDeclared && name == "go":
			argv := append(append([]string{"go", "test"}, goFlags...), "./...")
			planned = append(planned, Planned{Adapter: AdapterDeclared, Argv: argv, Env: goEnv, Units: []Unit{unit}})
		case unit.Adapter == AdapterDeclared:
			// A manifest unit the declaration no longer names has no argv;
			// Execute refuses it before discovery.
			argv := []string{}
			if entry, ok := declared.Command(name); ok {
				argv = append(argv, entry.Argv...)
			}
			planned = append(planned, Planned{Adapter: AdapterDeclared, Argv: argv, Env: []string{}, Units: []Unit{unit}})
		}
	}
	return planned
}

// Runner executes one shard of a manifest.
type Runner struct {
	// Budget is the total wall-clock budget of one shard; zero selects the
	// profile default (ShardBudget).
	Budget   time.Duration
	Confine  string
	Declared Declared
	Environ  []string
	Errors   io.Writer
	Output   io.Writer
	// Recheck recomputes the source identity before each launch and after
	// the last (STR-V0-017).
	Recheck func(context.Context) (SourceIdentity, error)
	Root    string
	Tool    string
	// Universe is the planned Go universe the runner's own listing must
	// equal before any Go unit runs.
	Universe []string
	// now is the shard clock, time.Now unless a test fixes it.
	now func() time.Time
	// deadline binds a context to the shard deadline on the same clock as
	// now; context.WithDeadline unless a test drives a virtual clock.
	deadline func(context.Context, time.Time) (context.Context, context.CancelFunc)
	// adapters replaces AdapterManifests when a test sets it.
	adapters map[string]AdapterManifest
}

func (r *Runner) clock() time.Time {
	if r.now != nil {
		return r.now()
	}
	return time.Now()
}

func (r *Runner) withDeadline(ctx context.Context, deadline time.Time) (context.Context, context.CancelFunc) {
	if r.deadline != nil {
		return r.deadline(ctx, deadline)
	}
	return context.WithDeadline(ctx, deadline)
}

// errDiscoveryBudget is a probe refused because the shard budget ran out.
var errDiscoveryBudget = errors.New("the shard budget ran out before the probe")

// Execute runs shard of m and returns its receipt. A cancelled ctx kills the
// running group and launches nothing more; the receipt is then INCOMPLETE.
func (r *Runner) Execute(ctx context.Context, m Manifest, manifestDigest string, shard int) Receipt {
	receipt := NewReceipt(m, manifestDigest, shard, r.Tool)
	budget := r.ShardBudget(m)
	deadline := r.clock().Add(budget)
	planned := Invocations(m, r.Declared, shard, r.Root, r.Confine)
	// STR-V0-004: only an adapter with a registered capability manifest of
	// this version is discovered or launched.
	manifests := r.adapters
	if manifests == nil {
		manifests = AdapterManifests()
	}
	planned = refuseUnmanifested(&receipt, manifests, m, shard, planned)
	if len(planned) != 0 && !groupsSupported {
		receipt.Incomplete = append(receipt.Incomplete, "this platform cannot own and kill a process group; nothing ran")
		planned = nil
	}
	versions := map[string][2]string{}
	// Discovery spends the shard budget too (STR-V0-015): each probe is bound
	// by the shard deadline, and none starts after it.
	discoverCtx, cancelDiscover := r.withDeadline(ctx, deadline)
	defer cancelDiscover()
	for _, invocation := range planned {
		key := runnerKey(invocation)
		if _, done := versions[key]; done {
			continue
		}
		if !r.clock().Before(deadline) {
			receipt.Incomplete = append(receipt.Incomplete, fmt.Sprintf("the shard's %s total budget ran out during runner discovery; nothing launched", budget))
			planned = nil
			break
		}
		if len(invocation.Argv) == 0 {
			receipt.Incomplete = append(receipt.Incomplete, invocation.Units[0].Unit+": "+DeclarationPath+" names no command for it")
			planned = withoutRunner(planned, key)
			continue
		}
		runner, version, err := r.discover(discoverCtx, invocation, deadline)
		if err != nil && (errors.Is(err, errDiscoveryBudget) || (ctx.Err() == nil && discoverCtx.Err() != nil)) {
			// The budget ended this runner's discovery: no later probe or
			// adapter starts (V1-1311).
			receipt.Incomplete = append(receipt.Incomplete, fmt.Sprintf("%s: the shard's %s total budget ran out during runner discovery; nothing launched", key, budget))
			planned = nil
			break
		}
		if err != nil {
			receipt.Incomplete = append(receipt.Incomplete, key+": "+err.Error())
			planned = withoutRunner(planned, key)
			continue
		}
		versions[key] = [2]string{runner, version}
	}
	launched, drifted := false, false
	for _, invocation := range planned {
		if ctx.Err() != nil {
			receipt.Incomplete = append(receipt.Incomplete, "cancelled before every invocation launched")
			break
		}
		if !r.clock().Before(deadline) {
			receipt.Incomplete = append(receipt.Incomplete, fmt.Sprintf("the shard's %s total budget ran out before every invocation launched", budget))
			break
		}
		if reason := r.drift(ctx, m.Source); reason != "" {
			receipt.Incomplete = append(receipt.Incomplete, reason+" before an adapter launch; nothing further launched")
			drifted = true
			break
		}
		launched = true
		recorded, stop := r.launch(ctx, invocation, versions[runnerKey(invocation)], budget, deadline)
		recorded.AdapterManifest = manifests[invocation.Adapter].Profile
		receipt.Invocations = append(receipt.Invocations, recorded)
		if stop != "" {
			receipt.Incomplete = append(receipt.Incomplete, stop)
			break
		}
	}
	// The closing recheck also covers a shard that launched nothing, so an
	// empty or NOTHING run cannot pass over changed source.
	if !drifted {
		if reason := r.drift(context.WithoutCancel(ctx), m.Source); reason != "" {
			when := " after the last adapter launch"
			if !launched {
				when = " before the shard finished; nothing launched"
			}
			receipt.Incomplete = append(receipt.Incomplete, reason+when)
		}
	}
	return Finish(receipt, m)
}

// NewReceipt starts a shard receipt from its manifest.
func NewReceipt(m Manifest, manifestDigest string, shard int, tool string) Receipt {
	return Receipt{
		Base: m.Base, Checks: nonNil(m.Checks), Excluded: m.Excluded, GraphDigest: m.GraphDigest, Incomplete: nonNil(m.Incomplete),
		Invocations: []Invocation{}, ManifestDigest: manifestDigest, PartitionDigest: m.PartitionDigest,
		Placement: m.Placement, PlanDigest: m.PlanDigest, Profile: ReceiptProfile, Reason: m.Reason,
		Revision: m.Source.Head, RunnerProfile: m.RunnerProfile, Scope: m.Scope, ShardCount: m.ShardCount,
		ShardIndex: shard, Source: m.Source, SourceDigest: m.SourceDigest, Tool: tool,
	}
}

// Finish sets the result and summary and applies the receipt bound: any
// incomplete reason or unit without a terminal outcome is INCOMPLETE, then
// any failed outcome or nonzero exit is FAIL (STR-V0-009, STR-V0-010).
func Finish(receipt Receipt, m Manifest) Receipt {
	expected := map[string]bool{}
	for _, unit := range m.Units {
		if unit.Shard == receipt.ShardIndex {
			expected[unit.Key()] = true
		}
	}
	passed, failed, notRun := 0, false, 0
	for _, invocation := range receipt.Invocations {
		if invocation.Exit != 0 {
			failed = true
		}
		for _, outcome := range invocation.Outcomes {
			delete(expected, outcome.Unit+" "+outcome.Slice)
			switch outcome.Outcome {
			case OutcomePass, OutcomeNoTests:
				passed++
			case OutcomeNotRun:
				notRun++
			default:
				failed = true
			}
		}
	}
	if notRun+len(expected) != 0 {
		receipt.Incomplete = append(receipt.Incomplete, fmt.Sprintf("%d execution units have no terminal outcome", notRun+len(expected)))
	}
	total := 0
	for _, unit := range m.Units {
		if unit.Shard == receipt.ShardIndex {
			total++
		}
	}
	switch {
	case len(receipt.Incomplete) != 0:
		receipt.Result = ResultIncomplete
	case failed:
		receipt.Result = ResultFail
	default:
		receipt.Result = ResultPass
	}
	where := "; the full suite runs under make gate and the required CI checks"
	switch receipt.Scope {
	case ScopeNarrowed:
		receipt.Summary = fmt.Sprintf("selected units passed: %d of %d", passed, total) + where
	case ScopeFull:
		receipt.Summary = fmt.Sprintf("FULL units passed: %d of %d", passed, total)
	default:
		receipt.Summary = "no execution unit selected" + where
	}
	if _, err := Encode(receipt); err != nil {
		for index := range receipt.Invocations {
			receipt.Invocations[index].Argv, receipt.Invocations[index].Outcomes = []string{}, []Outcome{}
		}
		receipt.Incomplete = append(receipt.Incomplete, fmt.Sprintf("the receipt exceeded %d bytes; argv and outcomes dropped", MaxDocumentBytes))
		receipt.Result = ResultIncomplete
	}
	return receipt
}

// runnerKey names the runner one invocation needs: the Go adapter, or the
// declared adapter's command name. Each is discovered once per shard.
func runnerKey(invocation Planned) string {
	if invocation.Dir != "" {
		return invocation.Adapter + " ./" + invocation.Dir
	}
	if invocation.Adapter == AdapterGo || len(invocation.Argv) == 0 {
		return invocation.Adapter
	}
	return invocation.Adapter + " " + invocation.Argv[0]
}

func withoutRunner(planned []Planned, key string) []Planned {
	kept := []Planned{}
	for _, invocation := range planned {
		if runnerKey(invocation) != key {
			kept = append(kept, invocation)
		}
	}
	return kept
}

// drift names a source-identity difference, or "" when the bytes that
// execute are still the ones planned.
func (r *Runner) drift(ctx context.Context, planned SourceIdentity) string {
	current, err := r.Recheck(ctx)
	if err != nil {
		return "source identity unavailable (" + err.Error() + ")"
	}
	if !reflect.DeepEqual(current, planned) {
		return "source drift"
	}
	return ""
}

// discover captures the runner and its exact version before execution, and
// for the Go adapter checks that `go list ./...` is the planned universe
// (STR-V0-004). A declared command is probed by its own name: `go` answers
// `go env GOVERSION`, anything else `--version`. A declared argv[0] with a
// path separator is a repository-relative path (STR-V0-003: commands run
// from the repository root), so it is resolved against Root, never against
// the caller's working directory; a bare name is looked up on PATH.
func (r *Runner) discover(ctx context.Context, invocation Planned, deadline time.Time) (string, string, error) {
	// The probes run with the launch's cache (STR-V0-174), so a declared
	// cache the caller replaced cannot fail discovery first.
	name, versionArgs, env := invocation.Argv[0], []string{"--version"}, GoCacheOverride(invocation.Env, r.Environ)
	if name == "go" {
		versionArgs = []string{"env", "GOVERSION"}
	}
	lookup := name
	if strings.Contains(name, "/") && !filepath.IsAbs(name) {
		root, err := filepath.Abs(r.Root)
		if err != nil {
			return "", "", fmt.Errorf("runner %s not found: %v", name, err)
		}
		lookup = filepath.Join(root, name)
	}
	runner, err := exec.LookPath(lookup)
	if err != nil {
		return "", "", fmt.Errorf("runner %s not found", name)
	}
	output, err := r.probe(ctx, deadline, runner, invocation.Dir, env, versionArgs...)
	version, _, _ := strings.Cut(strings.TrimSpace(string(output)), "\n")
	if errors.Is(err, errDiscoveryBudget) {
		return "", "", err
	}
	if err != nil || version == "" {
		return "", "", errors.New("runner version cannot be read")
	}
	if invocation.Adapter == AdapterGo {
		listed, err := r.probe(ctx, deadline, runner, invocation.Dir, env, "list", "./...")
		if errors.Is(err, errDiscoveryBudget) {
			return "", "", err
		}
		if err != nil && invocation.Dir != "" {
			return "", "", errors.New("go list ./... failed in " + invocation.Dir + "; the nested module cannot be confirmed")
		}
		if err != nil {
			return "", "", errors.New("go list ./... failed; the Go universe cannot be confirmed")
		}
		if invocation.Dir != "" {
			// A nested module's packages are not indexed (STR-V0-064): the
			// listing must succeed, and the root universe is not its own.
			return runner, version, nil
		}
		if difference := universeDifference(strings.Fields(string(listed)), r.Universe); difference != "" {
			return "", "", errors.New("go list ./... differs from the planned Go universe: " + difference)
		}
	}
	return runner, version, nil
}

// workDir is the directory an invocation runs from: the repository root, or
// a nested-module directory below it (STR-V0-064).
func (r *Runner) workDir(dir string) string {
	if dir == "" {
		return r.Root
	}
	return filepath.Join(r.Root, filepath.FromSlash(dir))
}

// probe runs one discovery command within the remaining shard budget; none
// starts once the shard deadline has passed (STR-V0-015).
func (r *Runner) probe(ctx context.Context, deadline time.Time, runner, dir string, env []string, args ...string) ([]byte, error) {
	if !r.clock().Before(deadline) {
		return nil, errDiscoveryBudget
	}
	probeCtx, cancel := context.WithTimeout(ctx, discoverDeadline)
	defer cancel()
	cmd := exec.CommandContext(probeCtx, runner, args...)
	cmd.Dir, cmd.Env, cmd.WaitDelay = r.workDir(dir), append(append([]string{}, r.Environ...), env...), 5*time.Second
	ownGroup(cmd)
	var stdout bytes.Buffer
	cmd.Stdout = &limitedBuffer{buffer: &stdout, remaining: MaxDocumentBytes * 8}
	err := cmd.Run()
	killGroup(cmd)
	return stdout.Bytes(), err
}

func universeDifference(listed, planned []string) string {
	want := map[string]bool{}
	for _, pkg := range planned {
		want[pkg] = true
	}
	var extra, missing []string
	got := map[string]bool{}
	for _, pkg := range listed {
		got[pkg] = true
		if !want[pkg] {
			extra = append(extra, pkg)
		}
	}
	for _, pkg := range planned {
		if !got[pkg] {
			missing = append(missing, pkg)
		}
	}
	if len(extra)+len(missing) == 0 {
		return ""
	}
	sort.Strings(extra)
	sort.Strings(missing)
	return fmt.Sprintf("%d unplanned %s, %d unlisted %s", len(extra), firstOf(extra), len(missing), firstOf(missing))
}

func firstOf(values []string) string {
	if len(values) == 0 {
		return "[]"
	}
	if len(values) > 3 {
		return fmt.Sprintf("%v...", values[:3])
	}
	return fmt.Sprintf("%v", values)
}

// launch runs one invocation in its own process group until the shard's
// deadline and returns what the receipt records, plus a reason to stop
// launching when the budget or a cancellation ended it.
func (r *Runner) launch(ctx context.Context, planned Planned, identity [2]string, budget time.Duration, deadline time.Time) (Invocation, string) {
	runCtx, cancel := r.withDeadline(ctx, deadline)
	defer cancel()
	plannedEnv := GoCacheOverride(planned.Env, r.Environ)
	env := append(append([]string{}, r.Environ...), plannedEnv...)
	envDigest, _ := Digest(env)
	recorded := Invocation{
		Adapter: planned.Adapter, Argv: planned.Argv, Dir: planned.Dir, Env: nonNil(plannedEnv), EnvDigest: envDigest,
		Exit: -1, Outcomes: []Outcome{}, Runner: identity[0], Version: identity[1],
	}
	cmd := exec.CommandContext(runCtx, identity[0], planned.Argv[1:]...)
	cmd.Dir, cmd.Env, cmd.Stderr, cmd.WaitDelay = r.workDir(planned.Dir), env, r.Errors, 10*time.Second
	ownGroup(cmd)
	stdout, err := cmd.StdoutPipe()
	started := time.Now()
	if err == nil {
		err = cmd.Start()
	}
	if err != nil {
		recorded.Outcomes = notRun(planned.Units)
		return recorded, "an adapter could not start: " + err.Error()
	}
	packages := map[string]string{}
	if planned.Adapter == AdapterGo {
		packages = parseTestEvents(stdout, r.Output)
	} else {
		_, _ = io.Copy(r.Output, stdout)
	}
	_ = cmd.Wait()
	killGroup(cmd)
	recorded.ElapsedMilliseconds = time.Since(started).Milliseconds()
	if cmd.ProcessState != nil {
		recorded.Exit = cmd.ProcessState.ExitCode()
	}
	stop := ""
	switch {
	case ctx.Err() != nil:
		stop = "cancelled while an adapter ran; its process group was killed"
	case runCtx.Err() != nil:
		stop = fmt.Sprintf("the shard exceeded its %s total budget while an adapter ran; its process group was killed", budget)
	}
	for _, unit := range planned.Units {
		outcome := OutcomeNotRun
		switch {
		case planned.Adapter == AdapterGo && unit.Dir != "":
			if stop == "" {
				outcome = moduleOutcome(packages, recorded.Exit)
			}
		case planned.Adapter == AdapterGo:
			if value, ok := packages[strings.TrimPrefix(unit.Unit, "go:")]; ok {
				outcome = value
			}
		case stop == "" && recorded.Exit == 0:
			outcome = OutcomePass
		case stop == "":
			outcome = OutcomeFail
		}
		recorded.Outcomes = append(recorded.Outcomes, Outcome{Outcome: outcome, Slice: unit.Slice, Unit: unit.Unit})
	}
	return recorded, stop
}

// moduleOutcome folds a nested module's per-package outcomes into its one
// unit's outcome (STR-V0-064): the first failing package outcome in
// BUILD_FAILED, TIMED_OUT, FAIL order; else FAIL for a nonzero exit; else
// PASS when some package passed and NO_TESTS when none did.
func moduleOutcome(packages map[string]string, exit int) string {
	seen := map[string]bool{}
	for _, value := range packages {
		seen[value] = true
	}
	for _, failing := range []string{OutcomeBuildFailed, OutcomeTimedOut, OutcomeFail} {
		if seen[failing] {
			return failing
		}
	}
	switch {
	case exit != 0:
		return OutcomeFail
	case seen[OutcomePass]:
		return OutcomePass
	}
	return OutcomeNoTests
}

func notRun(units []Unit) []Outcome {
	outcomes := make([]Outcome, 0, len(units))
	for _, unit := range units {
		outcomes = append(outcomes, Outcome{Outcome: OutcomeNotRun, Slice: unit.Slice, Unit: unit.Unit})
	}
	return outcomes
}

// testEvent is the part of one test2json event the adapter reads.
type testEvent struct {
	Action      string
	FailedBuild string
	Output      string
	Package     string
	Test        string
}

const maxEventLine = 1 << 20

// parseTestEvents streams `go test -json` output text to out and returns
// each package's terminal outcome. A line that is not an event passes
// through; an overlong line passes through unparsed, so a terminal event it
// held is missing and its unit stays NOT_RUN.
func parseTestEvents(stream io.Reader, out io.Writer) map[string]string {
	outcomes := map[string]string{}
	timedOut := map[string]bool{}
	reader := bufio.NewReaderSize(stream, maxEventLine)
	overlong := false
	for {
		line, err := reader.ReadSlice('\n')
		if errors.Is(err, bufio.ErrBufferFull) {
			_, _ = out.Write(line)
			overlong = true
			continue
		}
		if len(line) != 0 && !overlong {
			var event testEvent
			if json.Unmarshal(line, &event) != nil || event.Action == "" {
				_, _ = out.Write(line)
			} else {
				_, _ = io.WriteString(out, event.Output)
				if strings.Contains(event.Output, "panic: test timed out") && event.Package != "" {
					timedOut[event.Package] = true
				}
				if event.Test == "" && event.Package != "" {
					switch event.Action {
					case "pass":
						outcomes[event.Package] = OutcomePass
					case "skip":
						outcomes[event.Package] = OutcomeNoTests
					case "fail":
						switch {
						case event.FailedBuild != "":
							outcomes[event.Package] = OutcomeBuildFailed
						case timedOut[event.Package]:
							outcomes[event.Package] = OutcomeTimedOut
						default:
							outcomes[event.Package] = OutcomeFail
						}
					}
				}
			}
		} else if len(line) != 0 {
			_, _ = out.Write(line)
		}
		overlong = false
		if err != nil {
			return outcomes
		}
	}
}

// limitedBuffer keeps at most remaining bytes and then fails the write.
type limitedBuffer struct {
	buffer    *bytes.Buffer
	remaining int
}

func (w *limitedBuffer) Write(p []byte) (int, error) {
	if len(p) > w.remaining {
		return 0, errors.New("probe output limit exceeded")
	}
	w.remaining -= len(p)
	return w.buffer.Write(p)
}
