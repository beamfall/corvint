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

	"github.com/Beamfall/corvint/internal/gateselect"
	"github.com/Beamfall/corvint/internal/gokernel"
	"github.com/Beamfall/corvint/internal/liveverify/affected"
	"github.com/Beamfall/corvint/internal/liveverify/affected/golang"
	"github.com/Beamfall/corvint/internal/sessiontest"
)

const testHelp = `Run the affected test set (experimental, STR-V0).

Usage:
  corvint [--root PATH] test --base FULL_COMMIT_ID [--shard I/N]
    [--placement universe|balanced] [--profile local|ci [--confine PATH]]
    [--dry-run] [--manifest PATH] [--receipt PATH]
  corvint test verify-shards --manifest PATH RECEIPT...

Plans in process as corvint affected --base does, adds the AFP-V0-012
attribution, and runs the selected Go packages on the unchanged
.github/cishards complete-universe partition with the Go environment and
flags declared in .github/cishards/test-commands.json (ci: the full CI
branch's). Any fallback condition runs FULL and prints corvint-test:
FALLBACK <reason>. A language with no adapter runs the full command that
file declares, runs nothing when it declares {"none": reason}, and is
INCOMPLETE when it declares neither.
--placement balanced (experimental) spreads a narrowed run's own units
over the shards by the cost table; FULL keeps the universe partition. It
executes only in the local profile (ci: --dry-run only), and a sharded run
needs every shard to match one --manifest from a --dry-run.

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
	base, placement, profile, confine, manifest, receipt string
	shard, shardCount                                    int
	dryRun                                               bool
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
	shard := ""
	once("shard", &shard)
	f.BoolVar(&options.dryRun, "dry-run", false, "")
	requireClean := f.Bool("require-clean", false, "")
	if err := f.Parse(args); err != nil || f.NArg() != 0 {
		return options, argumentError("usage: corvint test --base FULL_COMMIT_ID [--shard I/N] [--placement universe|balanced] [--profile local|ci --confine PATH] [--dry-run] [--manifest PATH] [--receipt PATH]")
	}
	switch {
	case *requireClean:
		return options, argumentError("test runs the bytes that execute, dirty paths included; --require-clean is not accepted")
	case !validGitObjectID(options.base):
		return options, argumentError("test needs --base FULL_COMMIT_ID")
	case options.placement != sessiontest.PlacementUniverse && options.placement != sessiontest.PlacementBalanced:
		return options, argumentError("--placement must be universe or balanced")
	case options.profile != sessiontest.ProfileLocal && options.profile != sessiontest.ProfileCI:
		return options, argumentError("--profile must be local or ci")
	case options.profile == sessiontest.ProfileCI && !filepath.IsAbs(options.confine):
		return options, argumentError("--profile ci needs --confine ABSOLUTE_PATH to the test-confine wrapper")
	case options.profile == sessiontest.ProfileLocal && options.confine != "":
		return options, argumentError("--confine applies only to --profile ci")
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
	if options.placement == sessiontest.PlacementBalanced && !options.dryRun {
		switch {
		case options.profile == sessiontest.ProfileCI:
			return options, argumentError("--profile ci --placement balanced may only be computed with --dry-run; CI executes universe placement (STR-V0-008)")
		case options.shardCount > 1 && options.manifest == "":
			return options, argumentError("a sharded --placement balanced run needs --manifest PATH from one --dry-run, which every shard must reproduce (STR-V0-008)")
		}
	}
	return options, nil
}

func runTest(ctx context.Context, root string, args []string, stdout, stderr io.Writer) int {
	if len(args) > 0 && args[0] == "verify-shards" {
		return runTestVerifyShards(args[1:], stdout, stderr)
	}
	options, err := parseTestOptions(args)
	if err != nil {
		emitError(stderr, err)
		return 2
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
		err = testFillPlan(&input, receipt, graph, root)
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

func testFillPlan(input *sessiontest.Input, receipt affectedReceipt, graph *affected.Graph, root string) error {
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
	input.Universe = []string{}
	for _, id := range graph.UnitIDs() {
		if pkg, ok := strings.CutPrefix(id, "go:"); ok {
			input.Universe = append(input.Universe, pkg)
		}
	}
	switch module, err := golang.ReadModulePath(root); {
	case len(input.Universe) == 0:
		input.GoSelection = "NOTHING no Go universe"
	case err != nil:
		input.GoSelection = "FALLBACK the root module path cannot be read"
	default:
		lines := gateselect.Select(plan, module, root)
		if len(lines) != 0 {
			input.GoSelection = lines[len(lines)-1]
		}
	}
	return nil
}

func testReadOptional(name string) []byte {
	info, err := os.Lstat(name)
	if err != nil || !info.Mode().IsRegular() || info.Size() > sessiontest.MaxDocumentBytes {
		return nil
	}
	data, err := os.ReadFile(name)
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
		Env     []string           `json:"env"`
		Units   []sessiontest.Unit `json:"units"`
	}
	planned := []plannedJSON{}
	for _, invocation := range sessiontest.Invocations(m, declared, options.shard, root, options.confine) {
		planned = append(planned, plannedJSON{invocation.Adapter, invocation.Argv, invocation.Env, invocation.Units})
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

// testBudget is package state so fixture tests can shorten the budget.
var testBudget = sessiontest.DefaultBudget

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
