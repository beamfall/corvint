package main

import (
	"context"
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
	"strconv"
	"strings"
	"time"

	"github.com/Beamfall/corvint/internal/gateselect"
	"github.com/Beamfall/corvint/internal/gokernel"
	"github.com/Beamfall/corvint/internal/liveverify/affected"
	"github.com/Beamfall/corvint/internal/liveverify/affected/golang"
	"github.com/Beamfall/corvint/internal/sessiontest"
)

const testHelp = `Run the affected test set (experimental, STR-V0).

Usage:
  corvint [--root PATH] test --base FULL_COMMIT_ID [--shard I/N]
    [--placement universe|balanced|measured [--shard-plan PATH]]
    [--profile local|ci [--confine PATH]] [--go-parallel N] [--dry-run]
    [--manifest PATH] [--receipt PATH]
  corvint test verify-shards --manifest PATH RECEIPT...
  corvint test shards --count N --files PATH [--durations PATH]... [--runs K]
      [--test-slices PATH --split-allow PATH]

Plans in process as corvint affected --base does, adds the AFP-V0-012
attribution, and runs the selected Go packages on the unchanged
.github/cishards complete-universe partition with the Go environment and
flags declared in .github/cishards/test-commands.json (ci: the full CI
branch's). Any fallback condition runs FULL and prints corvint-test:
FALLBACK <reason>. A language with no adapter runs the full command that
file declares (ci: reports it as still required, not run), runs nothing
when it declares {"none": reason}, and is INCOMPLETE when it declares
neither. The budget covers the whole shard: 6h local, 60m ci.
A Go package selected only because its own test files changed runs just
the Test, Example and Fuzz functions those files declare (-run, slice
"selected" in the manifest) when no unchanged test file of the package
uses a name a changed one declares, no changed file declares init or
TestMain or initializes a variable with a call, and no other dirty path
reaches the package; otherwise it runs whole and the manifest unit's
"reason" names the missing proof (STR-V0-063).
--placement balanced (experimental) spreads a narrowed run's own units
over the shards by the cost table; FULL keeps the universe partition. It
executes only in the local profile (ci: --dry-run only), and a sharded run
needs every shard to match one --manifest from a --dry-run.
--placement measured (experimental, local execution only, like balanced)
puts each Go package whole, or each of its planned test slices, on the shard
a --shard-plan assigns it; a plan for another shard count or one that omits
a run-set package is INCOMPLETE. An unreadable plan is
test-shard-plan-unreadable before any Git work.

shards reads the listed files (for Go, package import paths) and the
retained full-run duration tables (corvint-ci-package-costs/0, oldest
first) and prints a corvint-test-shards/0 plan: each file weighs the median
of the last K runs (default 5), a file no used run records the median
weight (listed in unknown), longest first onto the lightest shard, with a
partition digest. Missing or invalid history prints corvint-test: shards FALLBACK
<reason> and plans the complete round-robin. With --test-slices and
--split-allow (both or neither) a package those AFP-V0-041 files split is
placed as its slices, each on a different shard. It writes only stdout.

--go-parallel N (local profile only, 1 to 64) runs the Go invocations with
-p N in place of the declared -p, so a focused selection fits a bounded
offload such as heavy.sh's 40-minute remote cap; the manifest and the
receipt argv record it, and every shard of one manifest must pass the same
value. Example: corvint test --base "$(git merge-base HEAD origin/main)"
--go-parallel 4.

--dry-run prints the shard's argv as JSON and runs nothing but Git;
--manifest PATH writes the corvint-test-manifest/0 there. Without --dry-run
a supplied --manifest must equal the recomputed one. Each shard writes one
corvint-test-run/0 receipt (default <git-dir>/corvint/test-run[.I].json).
verify-shards accepts the union only when every shard's receipt binds the
manifest and every execution unit passed exactly once.

Executes repository code with your authority; no containment is claimed.
A narrowed PASS is not a gate PASS: make gate and the required CI checks
stay binding. Exit: 0 PASS, 1 FAIL, 2 usage, 3 INCOMPLETE, 128+n signal.
`

// testCommandExit maps a shard or fan-in result to the process status:
// INCOMPLETE is distinct from FAIL so a caller can tell missing evidence
// from a failing test, and both are nonzero (STR-V0-009, STR-V0-010).
var testCommandExit = map[string]int{sessiontest.ResultPass: 0, sessiontest.ResultFail: 1, sessiontest.ResultIncomplete: 3}

// testPartitionDirectory holds the AFP-V0-022/041 placement inputs.
const testPartitionDirectory = ".github/cishards"

type testOptions struct {
	base, placement, profile, confine, manifest, receipt, shardPlan string
	shard, shardCount                                               int
	// goParallel is --go-parallel, the local Go `-p` (STR-V0-173); zero
	// keeps the declared flags.
	goParallel int
	dryRun     bool
}

// testInvocation recognises `[--root PATH] test ...`.
func testInvocation(args []string) (string, []string, bool) {
	root := ""
	for len(args) > 0 {
		if args[0] == "--root" {
			if !rootPreambleValue(args, 1) {
				return "", nil, false
			}
			root = args[1]
			args = args[2:]
			continue
		}
		if strings.HasPrefix(args[0], "--root=") {
			root = strings.TrimPrefix(args[0], "--root=")
			args = args[1:]
			continue
		}
		break
	}
	if len(args) == 0 || args[0] != "test" {
		return "", nil, false
	}
	return root, args[1:], true
}

func parseTestOptions(args []string) (testOptions, error) {
	options := testOptions{placement: sessiontest.PlacementUniverse, profile: sessiontest.ProfileLocal, shardCount: 1}
	f := flag.NewFlagSet("test", flag.ContinueOnError)
	f.SetOutput(io.Discard)
	seen := map[string]bool{}
	once := func(name string, dst *string) {
		f.Func(name, name, func(value string) error {
			if seen[name] || value == "" {
				return errors.New("repeated or empty")
			}
			seen[name], *dst = true, value
			return nil
		})
	}
	once("base", &options.base)
	once("placement", &options.placement)
	once("profile", &options.profile)
	once("confine", &options.confine)
	once("manifest", &options.manifest)
	once("receipt", &options.receipt)
	once("shard-plan", &options.shardPlan)
	shard := ""
	once("shard", &shard)
	goParallel := ""
	once("go-parallel", &goParallel)
	f.BoolVar(&options.dryRun, "dry-run", false, "")
	requireClean := f.Bool("require-clean", false, "")
	const usage = "usage: corvint test --base FULL_COMMIT_ID [--shard I/N] [--placement universe|balanced|measured [--shard-plan PATH]] [--profile local|ci --confine PATH] [--go-parallel N] [--dry-run] [--manifest PATH] [--receipt PATH]"
	// The refusal names the flag or argument at fault (STR-V0-062), as the
	// flag package reports it: "flag provided but not defined: -out".
	if err := f.Parse(args); err != nil {
		return options, argumentError(err.Error() + "; " + usage)
	}
	if f.NArg() != 0 {
		return options, argumentError(fmt.Sprintf("unexpected argument %q; %s", f.Arg(0), usage))
	}
	switch {
	case *requireClean:
		return options, argumentError("test runs the bytes that execute, dirty paths included; --require-clean is not accepted")
	case !validGitObjectID(options.base):
		return options, argumentError("test needs --base FULL_COMMIT_ID")
	case options.placement != sessiontest.PlacementUniverse && options.placement != sessiontest.PlacementBalanced && options.placement != sessiontest.PlacementMeasured:
		return options, argumentError("--placement must be universe, balanced or measured")
	case (options.placement == sessiontest.PlacementMeasured) != (options.shardPlan != ""):
		return options, argumentError("--placement measured needs --shard-plan PATH, and --shard-plan applies only to it (STR-V0-054)")
	case options.profile != sessiontest.ProfileLocal && options.profile != sessiontest.ProfileCI:
		return options, argumentError("--profile must be local or ci")
	case options.profile == sessiontest.ProfileCI && !filepath.IsAbs(options.confine):
		return options, argumentError("--profile ci needs --confine ABSOLUTE_PATH to the test-confine wrapper")
	case options.profile == sessiontest.ProfileLocal && options.confine != "":
		return options, argumentError("--confine applies only to --profile ci")
	}
	if goParallel != "" {
		n, err := strconv.Atoi(goParallel)
		switch {
		case err != nil || n < 1 || n > 64:
			return options, argumentError("--go-parallel must be an integer from 1 to 64")
		case options.profile != sessiontest.ProfileLocal:
			return options, argumentError("--go-parallel applies only to --profile local; the ci profile runs the full CI branch's flags (AFP-V0-028)")
		}
		options.goParallel = n
	}
	if shard != "" {
		index, count, ok := strings.Cut(shard, "/")
		i, iErr := strconv.Atoi(index)
		n, nErr := strconv.Atoi(count)
		if !ok || iErr != nil || nErr != nil || n < 1 || n > 64 || i < 0 || i >= n {
			return options, argumentError("--shard must be I/N with 0 <= I < N <= 64")
		}
		options.shard, options.shardCount = i, n
	}
	// Balanced placement executes only locally; CI computes and retains it
	// beside the executing universe placement (STR-V0-008, AFP-V0-028). Its
	// membership depends on the plan, so a sharded run executes only against
	// one supplied manifest, which every shard must reproduce or fail.
	// Measured placement follows the same rule (STR-V0-054).
	if options.placement != sessiontest.PlacementUniverse && !options.dryRun {
		requirement := "STR-V0-008"
		if options.placement == sessiontest.PlacementMeasured {
			requirement = "STR-V0-054"
		}
		switch {
		case options.profile == sessiontest.ProfileCI:
			return options, argumentError("--profile ci --placement " + options.placement + " may only be computed with --dry-run; CI executes universe placement (" + requirement + ")")
		case options.shardCount > 1 && options.manifest == "":
			return options, argumentError("a sharded --placement " + options.placement + " run needs --manifest PATH from one --dry-run, which every shard must reproduce (" + requirement + ")")
		}
	}
	return options, nil
}

func runTest(ctx context.Context, root string, args []string, stdout, stderr io.Writer) int {
	if len(args) > 0 && args[0] == "verify-shards" {
		return runTestVerifyShards(args[1:], stdout, stderr)
	}
	if len(args) > 0 && args[0] == "shards" {
		return runTestShards(args[1:], stdout, stderr)
	}
	options, err := parseTestOptions(args)
	if err != nil {
		emitError(stderr, err)
		return 2
	}
	var shardPlan *sessiontest.ShardPlan
	if options.shardPlan != "" {
		// STR-V0-055: the plan is read as the other shard inputs are, before
		// any git or runner work, so a FIFO or other non-regular file is
		// refused at once without blocking.
		raw, err := readBoundedFile(options.shardPlan, sessiontest.MaxShardPlanBytes)
		if err != nil {
			emitError(stderr, &gokernel.Error{Code: "test-shard-plan-unreadable", Message: "--shard-plan " + options.shardPlan + " is not a readable regular file of at most " + strconv.Itoa(sessiontest.MaxShardPlanBytes) + " bytes: " + err.Error()})
			return 2
		}
		plan, err := sessiontest.DecodeShardPlan(options.shardPlan, raw)
		if err != nil {
			emitError(stderr, argumentError("--shard-plan "+options.shardPlan+" is not usable: "+err.Error()))
			return 2
		}
		shardPlan = &plan
	}
	if root == "" {
		root = "."
	}
	if root, err = resolveExplicitRoot(root); err != nil {
		emitError(stderr, err)
		return 2
	}
	gitExecutable, err := exec.LookPath("git")
	if err != nil {
		emitError(stderr, affectedGitExecutableRefusal())
		return 2
	}
	if freshness := selfBinaryStale(ctx, root); freshness.Code != "" {
		_, _ = fmt.Fprintf(stderr, "corvint-test: DEGRADED %s: %s\n", freshness.Code, freshness.Detail)
	}
	receiptFlag := "--receipt"
	if options.receipt == "" && !options.dryRun {
		// The default destination passes the same guard as a supplied one,
		// so a linked <git-dir>/corvint cannot redirect it onto tracked state.
		if options.receipt, err = testDefaultReceiptPath(ctx, gitExecutable, root, options); err != nil {
			emitError(stderr, &gokernel.Error{Code: "invalid-arguments", Message: "the default receipt path cannot be resolved; pass --receipt PATH"})
			return 2
		}
		receiptFlag = "default receipt"
	}
	for _, output := range []struct{ flag, name string }{{receiptFlag, options.receipt}, {"--dry-run --manifest", testManifestOutput(options)}} {
		if output.name == "" {
			continue
		}
		if err := testCheckOutputPath(ctx, gitExecutable, root, output.name); err != nil {
			emitError(stderr, argumentError(output.flag+" "+output.name+": "+err.Error()))
			return 2
		}
	}
	identify := func(ctx context.Context) (sessiontest.SourceIdentity, error) {
		return testSourceIdentity(ctx, gitExecutable, root)
	}
	source, err := identify(ctx)
	if err != nil {
		emitError(stderr, &gokernel.Error{Code: "invalid-arguments", Message: "test cannot capture the source identity: " + err.Error()})
		return 2
	}
	input := testRunInput(ctx, root, options, source)
	if options.goParallel != 0 {
		input.Declared = input.Declared.WithGoParallel(options.goParallel)
	}
	input.ShardPlan = shardPlan
	manifest, err := sessiontest.Build(input)
	if err != nil {
		emitError(stderr, argumentError(err.Error()))
		return 2
	}
	digest, err := sessiontest.Digest(manifest)
	if err != nil {
		emitError(stderr, err)
		return 2
	}
	testVerdict(stderr, manifest)
	if options.dryRun {
		return testDryRun(manifest, digest, input.Declared, root, options, stdout, stderr)
	}
	// STR-V0-073: an executing run names each whole-package go test's
	// HEAVY_WHOLE_REASON before any invocation starts, as --dry-run does.
	testWholeReasons(sessiontest.Invocations(manifest, input.Declared, options.shard, root, options.confine), manifest.Reason, stderr)
	receipt, signalExit := testExecute(ctx, root, options, manifest, digest, input, identify, stdout, stderr)
	return testWriteReceipt(ctx, gitExecutable, root, options, receipt, signalExit, stderr)
}

func testManifestOutput(options testOptions) string {
	if options.dryRun {
		return options.manifest
	}
	return "" // without --dry-run the manifest is only read
}

var testDefaultReceiptName = regexp.MustCompile(`^test-run(\.[0-9]+)?\.json$`)

// testCheckOutputPath refuses an output path STR-V0-001 forbids: a tracked
// file, Git metadata other than the default receipt names under
// <git-dir>/corvint, and the .corvint/ ledgers.
func testCheckOutputPath(ctx context.Context, gitExecutable, root, name string) error {
	absolute, err := filepath.Abs(name)
	if err != nil {
		return err
	}
	// Only the parent is resolved: the write renames over the directory
	// entry itself, so a leaf symlink is the entry replaced, not its target.
	resolved := filepath.Join(testResolveExisting(filepath.Dir(absolute)), filepath.Base(absolute))
	if info, err := os.Lstat(resolved); err == nil && !info.Mode().IsRegular() {
		return errors.New("exists and is not a regular file")
	}
	deadline, cancel := context.WithTimeout(ctx, affectedRevisionDeadline)
	defer cancel()
	out, err := hermeticGitCommand(deadline, gitExecutable, root, "rev-parse", "--path-format=absolute", "--git-dir", "--git-common-dir", "--show-toplevel").Output()
	fields := strings.Split(strings.TrimSpace(string(out)), "\n")
	if err != nil || len(fields) != 3 {
		return errors.New("the repository layout cannot be read to check the path")
	}
	gitDir, commonDir, top := testResolveExisting(fields[0]), testResolveExisting(fields[1]), testResolveExisting(fields[2])
	switch {
	case testWithin(resolved, gitDir) || testWithin(resolved, commonDir):
		if filepath.Dir(resolved) != filepath.Join(gitDir, "corvint") || !testDefaultReceiptName.MatchString(filepath.Base(resolved)) {
			return errors.New("is Git metadata; only <git-dir>/corvint/test-run[.I].json may be written there")
		}
		return nil
	case testWithin(resolved, filepath.Join(top, ".corvint")):
		return errors.New("is under .corvint/, which corvint test never writes")
	case testWithin(resolved, top):
		relative, err := filepath.Rel(top, resolved)
		if err != nil {
			return err
		}
		tracked, err := hermeticGitCommand(deadline, gitExecutable, root, "ls-files", "-z", "--", ":(top,literal)"+filepath.ToSlash(relative)).Output()
		if err != nil {
			return errors.New("its tracked state cannot be read")
		}
		if len(tracked) != 0 {
			return errors.New("is a tracked file or directory")
		}
		// An unignored worktree file is a dirty path, so writing it would
		// change the source identity the run binds (STR-V0-017).
		fromRoot, err := filepath.Rel(testResolveExisting(root), resolved)
		if err != nil {
			return err
		}
		if err := hermeticGitCommand(deadline, gitExecutable, root, "check-ignore", "-q", "--no-index", "--", filepath.ToSlash(fromRoot)).Run(); err != nil {
			return errors.New("is in the worktree and not ignored, so writing it would change the source identity; write it outside the worktree, to an ignored path, or under <git-dir>/corvint")
		}
	}
	return nil
}

// testResolveExisting resolves symbolic links in the longest existing prefix
// of an absolute path and keeps the rest.
func testResolveExisting(path string) string {
	path = filepath.Clean(path)
	rest := ""
	for current := path; ; current = filepath.Dir(current) {
		if resolved, err := filepath.EvalSymlinks(current); err == nil {
			return filepath.Join(resolved, rest)
		}
		if parent := filepath.Dir(current); parent == current {
			return path
		}
		rest = filepath.Join(filepath.Base(current), rest)
	}
}

func testWithin(path, directory string) bool {
	relative, err := filepath.Rel(directory, path)
	return err == nil && relative != ".." && !strings.HasPrefix(relative, ".."+string(filepath.Separator)) && !filepath.IsAbs(relative)
}

// testRunInput plans in process through `corvint affected --base` and the
// AFP-V0-012 attribution, and reads the declared full scope and partition
// files independently of the planner (STR-V0-001..003, STR-V0-018).
func testRunInput(ctx context.Context, root string, options testOptions, source sessiontest.SourceIdentity) sessiontest.Input {
	input := sessiontest.Input{
		Base: options.base, Placement: options.placement, RunnerProfile: options.profile, ShardCount: options.shardCount, Source: source,
		Partition: sessiontest.PartitionFiles{
			Allow:  testReadOptional(filepath.Join(root, testPartitionDirectory, "test-split-allow.json")),
			Costs:  testReadOptional(filepath.Join(root, testPartitionDirectory, "package-costs.json")),
			Slices: testReadOptional(filepath.Join(root, testPartitionDirectory, "test-slices.json")),
		},
	}
	receipt, graph, err := compileAffectedGraph(ctx, affectedInvocation{Root: root, Base: options.base})
	if err == nil {
		input.GraphDigest = graph.Digest()
		err = testFillPlan(ctx, &input, receipt, graph, root, options.base)
	}
	if err != nil {
		message := err.Error()
		var kernelError *gokernel.Error
		if errors.As(err, &kernelError) {
			message = kernelError.Code + ": " + kernelError.Message
		}
		input.PlanError = message
	}
	// The declaration is read independently of the plan, so a planning
	// failure still finds every declared full command (STR-V0-003).
	input.Declared = sessiontest.ReadDeclared(root)
	return input
}

func testFillPlan(ctx context.Context, input *sessiontest.Input, receipt affectedReceipt, graph *affected.Graph, root, base string) error {
	compact, err := compactAffectedReceipt(receipt)
	if err != nil {
		return err
	}
	if input.PlanDigest, err = sessiontest.Digest(compact); err != nil {
		return err
	}
	encoded, err := gokernel.CanonicalJSON(compact)
	if err != nil {
		return err
	}
	var plan gateselect.Receipt
	if err := json.Unmarshal(encoded, &plan); err != nil {
		return err
	}
	for _, check := range receipt.Advice.Checks {
		if check.Kind == adviceKindMandatory {
			input.Checks = append(input.Checks, check.Command)
		}
	}
	for _, selection := range receipt.Plan.Selected {
		input.Selected = append(input.Selected, selection.UnitID)
	}
	input.GoSlices = testGoSlices(ctx, receipt.Plan, graph, root, base)
	for _, entry := range receipt.Plan.Unknown {
		input.Unknown = append(input.Unknown, sessiontest.Unknown{Detail: entry.Detail, Reason: entry.Reason})
	}
	for _, language := range affectedLanguages() {
		if language.Name() == "go" {
			continue
		}
		for _, dirty := range receipt.Plan.Dirty {
			if language.Owns(dirty) {
				input.DirtyLanguages = append(input.DirtyLanguages, language.Name())
				break
			}
		}
	}
	input.Universe, input.Excluded = testGoUniverse(graph)
	switch module, err := golang.ReadModulePath(root); {
	case len(input.Universe) == 0:
		input.GoSelection = "NOTHING no Go universe"
	case err != nil:
		input.GoSelection = "FALLBACK the root module path cannot be read"
	default:
		lines := gateselect.SelectStructural(plan, module, root)
		if len(lines) != 0 {
			input.GoSelection = lines[len(lines)-1]
		}
		// A change only inside nested modules no observed module builds
		// selects no root package; their own units run it (STR-V0-064).
		if strings.HasPrefix(input.GoSelection, "FALLBACK the selection is empty while ") && sessiontest.ModuleCovered(receipt.Plan.Dirty, input.Unknown) {
			input.GoSelection = "NOTHING every dirty path is inside a nested Go module whose own unit runs it"
		}
	}
	return nil
}

// testGoUniverse is the root Go universe: every graph Go unit the root
// module's `go list ./...` lists. A dependent nested module's synthetic unit
// (AFP-V0-582) is a module, not a root package, so it is excluded with its
// reason rather than left for the runner's listing to report unlisted
// (STR-V0-069).
func testGoUniverse(graph *affected.Graph) (universe, excluded []string) {
	universe = []string{}
	for _, id := range graph.UnitIDs() {
		pkg, ok := strings.CutPrefix(id, "go:")
		if !ok {
			continue
		}
		if unit, _ := graph.Unit(id); golang.DependentModuleUnit(unit) {
			excluded = append(excluded, id+": a dependent nested module (AFP-V0-582) the root module's go list ./... does not list; its own tests are not run")
			continue
		}
		universe = append(universe, pkg)
	}
	return universe, excluded
}

// testReadOptional reads at most MaxDocumentBytes through the opened
// descriptor of a regular file, never blocking on a FIFO; nil means absent,
// not regular, changed while opened, unreadable or over the bound.
func testReadOptional(name string) []byte {
	data, err := readBoundedFile(name, sessiontest.MaxDocumentBytes)
	if err != nil {
		return nil
	}
	return data
}

// testSourceIdentity is the STR-V0-017 source identity: HEAD, its tree and
// every dirty path's content digest.
func testSourceIdentity(ctx context.Context, gitExecutable, root string) (sessiontest.SourceIdentity, error) {
	head, err := affectedHeadRevision(ctx, gitExecutable, root)
	if err != nil {
		return sessiontest.SourceIdentity{}, err
	}
	deadline, cancel := context.WithTimeout(ctx, affectedRevisionDeadline)
	defer cancel()
	tree, err := hermeticGitCommand(deadline, gitExecutable, root, "rev-parse", "--verify", "--quiet", head+"^{tree}").Output()
	if err != nil {
		return sessiontest.SourceIdentity{}, errors.New("HEAD tree cannot be read")
	}
	dirty, err := affected.DirtyPaths(ctx, gitExecutable, root)
	if err != nil {
		return sessiontest.SourceIdentity{}, err
	}
	return sessiontest.Identify(root, head, strings.TrimSpace(string(tree)), dirty)
}

func testVerdict(stderr io.Writer, m sessiontest.Manifest) {
	switch m.Scope {
	case sessiontest.ScopeFull:
		_, _ = fmt.Fprintf(stderr, "corvint-test: FALLBACK %s\n", m.Reason)
	case sessiontest.ScopeNothing:
		_, _ = fmt.Fprintf(stderr, "corvint-test: NOTHING %s\n", m.Reason)
	default:
		_, _ = fmt.Fprintf(stderr, "corvint-test: NARROWED %d execution units\n", len(m.Units))
		if sliced := m.SlicedUnits(); sliced != 0 {
			_, _ = fmt.Fprintf(stderr, "corvint-test: SLICED %d of them to the test functions of their changed test files\n", sliced)
		}
	}
	for _, entry := range m.Excluded {
		_, _ = fmt.Fprintf(stderr, "corvint-test: EXCLUDED %s\n", entry)
	}
	for _, reason := range m.Incomplete {
		_, _ = fmt.Fprintf(stderr, "corvint-test: INCOMPLETE %s\n", reason)
	}
	for _, check := range m.Checks {
		_, _ = fmt.Fprintf(stderr, "corvint-test: still required, not run: %s\n", check)
	}
}

// testDryRun prints the shard's run set and argv and executes nothing but
// the Git reads planning needed; --manifest names the only file it writes.
func testDryRun(m sessiontest.Manifest, digest string, declared sessiontest.Declared, root string, options testOptions, stdout, stderr io.Writer) int {
	type plannedJSON struct {
		Adapter string             `json:"adapter"`
		Argv    []string           `json:"argv"`
		Dir     string             `json:"dir,omitempty"`
		Env     []string           `json:"env"`
		Units   []sessiontest.Unit `json:"units"`
	}
	planned := []plannedJSON{}
	invocations := sessiontest.Invocations(m, declared, options.shard, root, options.confine)
	for _, invocation := range invocations {
		planned = append(planned, plannedJSON{invocation.Adapter, invocation.Argv, invocation.Dir, invocation.Env, invocation.Units})
	}
	document := map[string]any{
		"checks": m.Checks, "incomplete": m.Incomplete, "invocations": planned, "manifestDigest": digest,
		"noSuite": m.NoSuite, "placement": m.Placement, "reason": m.Reason, "scope": m.Scope, "shardCount": m.ShardCount, "shardIndex": options.shard,
	}
	if options.manifest != "" {
		encoded, err := sessiontest.Encode(m)
		if err == nil {
			err = sessiontest.WriteFile(options.manifest, encoded)
		}
		if err != nil {
			emitError(stderr, &gokernel.Error{Code: "invalid-arguments", Message: "the manifest cannot be written: " + err.Error()})
			return 2
		}
	}
	encoded, err := gokernel.CanonicalJSON(document)
	if err != nil {
		emitError(stderr, err)
		return 2
	}
	_, _ = stdout.Write(append(encoded, '\n'))
	testWholeReasons(invocations, m.Reason, stderr)
	if len(m.Incomplete) != 0 {
		return testCommandExit[sessiontest.ResultIncomplete]
	}
	return 0
}

// testExecute runs one shard. A supplied --manifest must equal the one this
// shard recomputed (STR-V0-018); a signal kills the running group and ends
// the shard INCOMPLETE without any fallback (STR-V0-014).
func testExecute(ctx context.Context, root string, options testOptions, m sessiontest.Manifest, digest string, input sessiontest.Input, identify func(context.Context) (sessiontest.SourceIdentity, error), stdout, stderr io.Writer) (sessiontest.Receipt, int) {
	tool := "corvint " + version + " (build " + build + ")"
	if options.manifest != "" {
		_, suppliedDigest, err := sessiontest.ReadManifest(options.manifest)
		if err == nil && suppliedDigest != digest {
			err = errors.New("it differs from the manifest this shard recomputed")
		}
		if err != nil {
			receipt := sessiontest.NewReceipt(m, digest, options.shard, tool)
			receipt.Incomplete = append(receipt.Incomplete, "the supplied manifest is not usable: "+err.Error())
			return sessiontest.Finish(receipt, m), 0
		}
	}
	codes := dogfoodSignalCodes()
	notified := make(chan os.Signal, 1)
	signals := make([]os.Signal, 0, len(codes))
	for received := range codes {
		signals = append(signals, received)
	}
	signal.Notify(notified, signals...)
	defer signal.Stop(notified)
	runCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	var received os.Signal
	done := make(chan struct{})
	go func() {
		defer close(done)
		select {
		case value := <-notified:
			received = value
			cancel()
		case <-runCtx.Done():
		}
	}()
	runner := &sessiontest.Runner{
		Budget: testBudget, Confine: options.confine, Declared: input.Declared, Environ: os.Environ(),
		Errors: stderr, Output: stdout, Recheck: identify, Root: root, Tool: tool, Universe: input.Universe,
	}
	receipt := runner.Execute(runCtx, m, digest, options.shard)
	cancel()
	<-done
	if received != nil {
		receipt.Incomplete = append(receipt.Incomplete, fmt.Sprintf("terminated by signal %v; owned process groups killed; no fallback started", received))
		return sessiontest.Finish(receipt, m), codes[received]
	}
	return receipt, 0
}

// testBudget is package state so fixture tests can shorten the shard budget;
// zero selects the profile default (sessiontest.Runner.ShardBudget).
var testBudget time.Duration

// testDefaultReceiptPath is <git-dir>/corvint/test-run[.I].json.
func testDefaultReceiptPath(ctx context.Context, gitExecutable, root string, options testOptions) (string, error) {
	deadline, cancel := context.WithTimeout(ctx, affectedRevisionDeadline)
	defer cancel()
	gitDir, err := hermeticGitCommand(deadline, gitExecutable, root, "rev-parse", "--absolute-git-dir").Output()
	if err != nil {
		return "", err
	}
	base := "test-run.json"
	if options.shardCount > 1 {
		base = fmt.Sprintf("test-run.%d.json", options.shard)
	}
	return filepath.Join(strings.TrimSpace(string(gitDir)), "corvint", base), nil
}

func testWriteReceipt(ctx context.Context, gitExecutable, root string, options testOptions, receipt sessiontest.Receipt, signalExit int, stderr io.Writer) int {
	name := options.receipt
	encoded, err := sessiontest.Encode(receipt)
	if err == nil {
		err = sessiontest.WriteFile(name, encoded)
	}
	if err != nil {
		emitError(stderr, &gokernel.Error{Code: "invalid-arguments", Message: "the receipt cannot be written: " + err.Error()})
		return testCommandExit[sessiontest.ResultIncomplete]
	}
	for _, reason := range receipt.Incomplete {
		_, _ = fmt.Fprintf(stderr, "corvint-test: INCOMPLETE %s\n", reason)
	}
	_, _ = fmt.Fprintf(stderr, "corvint-test: %s %s; receipt %s\n", receipt.Result, receipt.Summary, name)
	if signalExit != 0 {
		return signalExit
	}
	return testCommandExit[receipt.Result]
}

// runTestVerifyShards is the STR-V0-009 fan-in: the expected run comes from
// the manifest, never from the receipts.
func runTestVerifyShards(args []string, stdout, stderr io.Writer) int {
	f := flag.NewFlagSet("test verify-shards", flag.ContinueOnError)
	f.SetOutput(io.Discard)
	manifestPath := f.String("manifest", "", "")
	if err := f.Parse(args); err != nil || *manifestPath == "" || f.NArg() == 0 {
		emitError(stderr, argumentError("usage: corvint test verify-shards --manifest PATH RECEIPT..."))
		return 2
	}
	manifest, digest, err := sessiontest.ReadManifest(*manifestPath)
	if err != nil {
		emitError(stderr, &gokernel.Error{Code: "invalid-arguments", Message: "the manifest is not usable: " + err.Error()})
		return 2
	}
	var receipts []sessiontest.Receipt
	var unreadable []string
	for _, name := range f.Args() {
		receipt, err := sessiontest.ReadReceipt(name)
		if err != nil {
			unreadable = append(unreadable, name+": unreadable receipt: "+err.Error())
			continue
		}
		receipts = append(receipts, receipt)
	}
	result, reasons := sessiontest.Verify(manifest, digest, receipts, unreadable)
	encoded, err := gokernel.CanonicalJSON(map[string]any{"manifestDigest": digest, "reasons": reasons, "result": result})
	if err != nil {
		emitError(stderr, err)
		return 2
	}
	_, _ = stdout.Write(append(encoded, '\n'))
	return testCommandExit[result]
}

// runTestShards is the STR-V0-050 duration-balanced shard planner. It is a
// read command: it reads the listed files and the duration tables and
// writes only stdout and stderr (invariant 4).
// plannedSlicedPackages counts the packages a plan carries as test slices
// (STR-V0-057).
func plannedSlicedPackages(plan sessiontest.ShardPlan) int {
	packages := map[string]bool{}
	for _, shard := range plan.Shards {
		for _, slice := range shard.Slices {
			packages[slice.Package] = true
		}
	}
	return len(packages)
}

func runTestShards(args []string, stdout, stderr io.Writer) int {
	f := flag.NewFlagSet("test shards", flag.ContinueOnError)
	f.SetOutput(io.Discard)
	count := f.Int("count", 0, "")
	runs := f.Int("runs", sessiontest.DefaultHistoryRuns, "")
	files := f.String("files", "", "")
	testSlices := f.String("test-slices", "", "")
	splitAllow := f.String("split-allow", "", "")
	var durations []string
	f.Func("durations", "", func(value string) error {
		if value == "" {
			return errors.New("empty")
		}
		durations = append(durations, value)
		return nil
	})
	usage := fmt.Sprintf("usage: corvint test shards --count N (1 to %d) --files PATH [--durations PATH]... [--runs K (1 to %d)] [--test-slices PATH --split-allow PATH]", sessiontest.MaxShardPlanShards, sessiontest.MaxHistoryRuns)
	if err := f.Parse(args); err != nil || f.NArg() != 0 || *files == "" || *count < 1 || *count > sessiontest.MaxShardPlanShards ||
		*runs < 1 || *runs > sessiontest.MaxHistoryRuns || (*testSlices == "") != (*splitAllow == "") {
		emitError(stderr, argumentError(usage))
		return 2
	}
	raw := testReadOptional(*files)
	if raw == nil {
		emitError(stderr, argumentError("--files "+*files+" is not a readable regular file of at most 1 MiB"))
		return 2
	}
	listed, err := sessiontest.ListedFiles(raw)
	if err != nil {
		emitError(stderr, argumentError("--files "+*files+": "+err.Error()))
		return 2
	}
	// STR-V0-057: both slice inputs or neither; an unreadable one is refused,
	// an unusable one splits nothing.
	var allowRaw, slicesRaw []byte
	for _, input := range []struct {
		flag, path string
		raw        *[]byte
	}{{"--test-slices", *testSlices, &slicesRaw}, {"--split-allow", *splitAllow, &allowRaw}} {
		if input.path == "" {
			continue
		}
		if *input.raw = testReadOptional(input.path); *input.raw == nil {
			emitError(stderr, argumentError(input.flag+" "+input.path+" is not a readable regular file of at most 1 MiB"))
			return 2
		}
	}
	history := make([]sessiontest.History, 0, len(durations))
	// Over MaxHistoryRuns tables is the STR-V0-053 fallback; none is read.
	for _, name := range durations {
		entry := sessiontest.History{Name: name}
		if len(durations) <= sessiontest.MaxHistoryRuns {
			entry.Raw = testReadOptional(name)
		}
		history = append(history, entry)
	}
	plan, err := sessiontest.PlanShardsSliced(listed, *count, *runs, history, allowRaw, slicesRaw)
	if err != nil {
		emitError(stderr, argumentError(err.Error()))
		return 2
	}
	encoded, err := sessiontest.EncodeShardPlan(plan)
	if err != nil {
		emitError(stderr, err)
		return 2
	}
	if plan.Fallback != "" {
		fmt.Fprintln(stderr, "corvint-test: shards FALLBACK "+plan.Fallback)
	} else {
		sliced := ""
		if packages := plannedSlicedPackages(plan); packages != 0 {
			sliced = fmt.Sprintf(", %d sliced", packages)
		}
		fmt.Fprintf(stderr, "corvint-test: shards %d files over %d shards from %d runs, %d unknown%s\n", len(listed), plan.Count, len(plan.History), len(plan.Unknown), sliced)
	}
	_, _ = stdout.Write(encoded)
	return 0
}
