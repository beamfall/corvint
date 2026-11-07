// SPDX-License-Identifier: Apache-2.0

package main

import (
	"context"
	json "encoding/json/v2"
	"errors"
	"flag"
	"fmt"
	"io"
	"maps"
	"os"
	"os/exec"
	"slices"
	"strconv"
	"strings"

	"github.com/Beamfall/corvint/internal/gokernel"
	"github.com/Beamfall/corvint/internal/testreuse"
)

// The `corvint test reuse` usage lines (AFP-V0-2326).
const (
	testReuseRecordUsage = "usage: corvint test reuse record (--outcomes PATH... | --go-test-json PATH) --argv WORDS [--confine PATH] [--harness PATH]... [--source-kind main|pull_request|local] [--pull-request N] [--run-id ID] [--run-attempt N] --out PATH"
	testReuseDecideUsage = "usage: corvint test reuse decide --argv WORDS [--confine PATH] [--records PATH]... [--harness PATH]... [--trust KIND[,KIND]] [--pull-request N] [--packages PATH] [--out PATH] [--run-list PATH]"
)

// testReuseExitIncomplete is the exit status when no record or decision can
// be produced; a caller then runs its full plan (AFP-V0-2325).
const testReuseExitIncomplete = 3

// testReuseBeforePublish runs after the record or decision is built and
// before its final check and output; a test makes a controlled edit there.
var testReuseBeforePublish = func() {}

type multiFlag []string

func (m *multiFlag) String() string { return strings.Join(*m, ",") }
func (m *multiFlag) Set(value string) error {
	if value == "" {
		return errors.New("empty value")
	}
	*m = append(*m, value)
	return nil
}

type testReuseOptions struct {
	root, argv, confine, out, runList, packages, testJSON, sourceKind, trust string
	outcomes, records, harness                                               multiFlag
	pullRequest, runAttempt                                                  int
	runID                                                                    int64
}

func runTestReuse(ctx context.Context, root string, args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 || (args[0] != "record" && args[0] != "decide") {
		emitError(stderr, argumentError("corvint test reuse needs record or decide; "+testReuseRecordUsage+"; "+testReuseDecideUsage))
		return 2
	}
	mode, usage := args[0], testReuseRecordUsage
	if mode == "decide" {
		usage = testReuseDecideUsage
	}
	options := testReuseOptions{root: root, sourceKind: testreuse.SourceLocal, trust: testreuse.SourceLocal + "," + testreuse.SourceMain}
	f := flag.NewFlagSet("test reuse "+mode, flag.ContinueOnError)
	f.SetOutput(io.Discard)
	f.StringVar(&options.argv, "argv", "", "")
	f.StringVar(&options.confine, "confine", "", "")
	f.Var(&options.harness, "harness", "")
	f.StringVar(&options.out, "out", "", "")
	if mode == "record" {
		f.Var(&options.outcomes, "outcomes", "")
		f.StringVar(&options.testJSON, "go-test-json", "", "")
		f.StringVar(&options.sourceKind, "source-kind", options.sourceKind, "")
		f.IntVar(&options.pullRequest, "pull-request", 0, "")
		f.Int64Var(&options.runID, "run-id", 0, "")
		f.IntVar(&options.runAttempt, "run-attempt", 0, "")
	} else {
		f.Var(&options.records, "records", "")
		f.StringVar(&options.trust, "trust", options.trust, "")
		f.IntVar(&options.pullRequest, "pull-request", 0, "")
		f.StringVar(&options.packages, "packages", "", "")
		f.StringVar(&options.runList, "run-list", "", "")
	}
	if err := f.Parse(args[1:]); err != nil {
		emitError(stderr, argumentError(err.Error()+"; "+usage))
		return 2
	}
	switch {
	case f.NArg() != 0:
		emitError(stderr, argumentError(fmt.Sprintf("unexpected argument %q; %s", f.Arg(0), usage)))
		return 2
	case strings.TrimSpace(options.argv) == "":
		emitError(stderr, argumentError("--argv names the go test flags the result is obtained under, for example \"-race -count=1 -timeout 50m\"; "+usage))
		return 2
	case mode == "record" && (len(options.outcomes) == 0) == (options.testJSON == ""):
		emitError(stderr, argumentError("record reads either --outcomes PATH... or --go-test-json PATH; "+usage))
		return 2
	case mode == "record" && options.out == "":
		emitError(stderr, argumentError("record needs --out PATH; "+usage))
		return 2
	}
	if err := testreuse.CheckArgv(strings.Fields(options.argv)); err != nil {
		emitError(stderr, argumentError("--argv: "+err.Error()))
		return 2
	}
	if options.root == "" {
		options.root = "."
	}
	var err error
	if options.root, err = resolveExplicitRoot(options.root); err != nil {
		emitError(stderr, err)
		return 2
	}
	gitExecutable, err := exec.LookPath("git")
	if err != nil {
		emitError(stderr, affectedGitExecutableRefusal())
		return 2
	}
	for _, output := range []struct{ flag, name string }{{"--out", options.out}, {"--run-list", options.runList}} {
		if output.name == "" {
			continue
		}
		if err := testCheckOutputPath(ctx, gitExecutable, options.root, output.name); err != nil {
			emitError(stderr, argumentError(output.flag+" "+output.name+": "+err.Error()))
			return 2
		}
	}
	snapshot, identity, err := testReuseObserve(ctx, options)
	if err != nil {
		emitError(stderr, &gokernel.Error{Code: "test-reuse-incomplete", Message: "no " + mode + " is produced and the full plan runs: " + err.Error()})
		return testReuseExitIncomplete
	}
	if mode == "record" {
		return testReuseRecord(ctx, snapshot, identity, options, stderr)
	}
	return testReuseDecide(ctx, snapshot, identity, options, stdout, stderr)
}

// testReuseObserve reads the snapshot, the affected graph's UNBOUNDED_READER
// and existence-reader sets, and the identity. testReuseFinalCheck later
// rechecks the checkout so all of them, and the record or decision built from
// them, describe one observation (AFP-V0-2320, AFP-V0-2321).
func testReuseObserve(ctx context.Context, options testReuseOptions) (*testreuse.Snapshot, testreuse.Identity, error) {
	snapshot, err := testreuse.Observe(ctx, testreuse.ObserveOptions{Root: options.root, BuildFlags: testreuse.BuildFlags(strings.Fields(options.argv))})
	if err != nil {
		return nil, testreuse.Identity{}, err
	}
	invocation := affectedInvocation{Root: options.root}
	if err := resolveAffectedBatchAndCache(&invocation); err != nil {
		return nil, testreuse.Identity{}, err
	}
	graph, err := affectedBuildGraph(affectedGraphContext(ctx), invocation)
	if err != nil {
		return nil, testreuse.Identity{}, fmt.Errorf("affected graph: %w", err)
	}
	for _, id := range graph.UnboundedReaders() {
		if name, ok := strings.CutPrefix(id, "go:"); ok {
			snapshot.Unbounded[name] = true
		}
	}
	for _, directory := range graph.ExistenceReaderPackages() {
		snapshot.ExistenceReaders[directory] = true
	}
	identity, err := testreuse.ObserveIdentity(ctx, snapshot, "", nil, strings.Fields(options.argv), options.harness)
	if err != nil {
		return nil, testreuse.Identity{}, err
	}
	// Without --confine the run was unconfined and no package is eligible
	// (AFP-V0-2320); the record or decision still lists every package.
	if options.confine != "" {
		if identity.Confine, err = testreuse.ConfineDigest(options.confine); err != nil {
			return nil, testreuse.Identity{}, err
		}
	}
	return snapshot, identity, nil
}

// testReuseFinalCheck runs after the record or decision is built and before
// any of it is published (AFP-V0-2321): it rechecks HEAD, the tree, the
// status, the index flags and every worktree validation uncached, and
// re-digests the confinement wrapper, so an edit made while the output was
// being built refuses it.
func testReuseFinalCheck(ctx context.Context, snapshot *testreuse.Snapshot, identity testreuse.Identity, options testReuseOptions) error {
	if err := snapshot.Recheck(ctx); err != nil {
		return err
	}
	if options.confine == "" {
		return nil
	}
	confine, err := testreuse.ConfineDigest(options.confine)
	if err != nil {
		return err
	}
	if confine != identity.Confine {
		return errors.New(testreuse.ErrorSnapshotDrift + ": the confinement wrapper changed after it was digested")
	}
	return nil
}

func testReuseRecord(ctx context.Context, snapshot *testreuse.Snapshot, identity testreuse.Identity, options testReuseOptions, stderr io.Writer) int {
	var outcomes map[string]string
	var err error
	if options.testJSON != "" {
		var body []byte
		if body, err = readBoundedFile(options.testJSON, 1<<30); err == nil {
			outcomes, err = testreuse.ParseTestJSON(body)
		}
	} else {
		var documents []testreuse.Outcomes
		for _, name := range options.outcomes {
			body, readErr := readBoundedFile(name, 4<<20)
			if readErr != nil {
				err = fmt.Errorf("%s: %w", name, readErr)
				break
			}
			document, parseErr := testreuse.ParseOutcomes(body)
			if parseErr != nil {
				err = fmt.Errorf("%s: %w", name, parseErr)
				break
			}
			documents = append(documents, document)
		}
		if err == nil {
			outcomes, err = testreuse.MergeOutcomes(documents, snapshot.Head, snapshot.Tree)
		}
	}
	if err != nil {
		emitError(stderr, &gokernel.Error{Code: "test-reuse-incomplete", Message: "no record is produced: " + err.Error()})
		return testReuseExitIncomplete
	}
	source := testreuse.Source{Kind: options.sourceKind, PullRequest: options.pullRequest, RunID: options.runID, RunAttempt: options.runAttempt, Commit: snapshot.Head, Tree: snapshot.Tree}
	record, err := testreuse.BuildRecord(snapshot, identity, source, outcomes)
	if err != nil {
		emitError(stderr, argumentError("record: "+err.Error()))
		return 2
	}
	testReuseBeforePublish()
	if err := testReuseFinalCheck(ctx, snapshot, identity, options); err != nil {
		emitError(stderr, &gokernel.Error{Code: "test-reuse-incomplete", Message: "no record is produced: " + err.Error()})
		return testReuseExitIncomplete
	}
	if err := testReuseWrite(options.out, record); err != nil {
		emitError(stderr, &gokernel.Error{Code: "test-reuse-incomplete", Message: "the record cannot be written: " + err.Error()})
		return testReuseExitIncomplete
	}
	reasons := map[string]int{}
	for _, skipped := range record.Skipped {
		reasons[skipped.Reason]++
	}
	_, _ = fmt.Fprintf(stderr, "corvint-test-reuse: RECORD %d passed package(s) at %s; skipped %s\n", len(record.Packages), snapshot.Head, testReuseCounts(reasons))
	return 0
}

func testReuseDecide(ctx context.Context, snapshot *testreuse.Snapshot, identity testreuse.Identity, options testReuseOptions, stdout, stderr io.Writer) int {
	policy := testreuse.Policy{Trust: map[string]bool{}, PullRequest: options.pullRequest}
	for _, kind := range strings.Split(options.trust, ",") {
		switch kind {
		case testreuse.SourceMain, testreuse.SourcePullRequest, testreuse.SourceLocal:
			policy.Trust[kind] = true
		default:
			emitError(stderr, argumentError("--trust names main, pull_request or local, comma separated"))
			return 2
		}
	}
	if policy.Trust[testreuse.SourcePullRequest] != (options.pullRequest > 0) {
		emitError(stderr, argumentError("--trust pull_request needs --pull-request N, which admits only that pull request's records"))
		return 2
	}
	packages := snapshot.Universe
	if options.packages != "" {
		body, err := readBoundedFile(options.packages, 4<<20)
		if err != nil {
			emitError(stderr, argumentError("--packages: "+err.Error()))
			return 2
		}
		packages = strings.Fields(string(body))
	}
	var records []testreuse.RecordInput
	for _, name := range options.records {
		body, err := readBoundedFile(name, testreuse.MaxRecordBytes)
		records = append(records, testreuse.RecordInput{Name: name, Body: body, Err: err})
	}
	decision, err := testreuse.Decide(snapshot, identity, policy, records, packages)
	if err != nil {
		emitError(stderr, &gokernel.Error{Code: "test-reuse-incomplete", Message: "no decision is produced and the full plan runs: " + err.Error()})
		return testReuseExitIncomplete
	}
	testReuseBeforePublish()
	if err := testReuseFinalCheck(ctx, snapshot, identity, options); err != nil {
		emitError(stderr, &gokernel.Error{Code: "test-reuse-incomplete", Message: "no decision is produced and the full plan runs: " + err.Error()})
		return testReuseExitIncomplete
	}
	if options.runList != "" {
		body := strings.Join(decision.Run, "\n")
		if body != "" {
			body += "\n"
		}
		if err := os.WriteFile(options.runList, []byte(body), 0o644); err != nil {
			emitError(stderr, &gokernel.Error{Code: "test-reuse-incomplete", Message: "the run list cannot be written: " + err.Error()})
			return testReuseExitIncomplete
		}
	}
	if options.out != "" {
		if err := testReuseWrite(options.out, decision); err != nil {
			emitError(stderr, &gokernel.Error{Code: "test-reuse-incomplete", Message: "the decision cannot be written: " + err.Error()})
			return testReuseExitIncomplete
		}
	} else {
		body, err := json.Marshal(decision, json.Deterministic(true))
		if err != nil {
			emitError(stderr, err)
			return testReuseExitIncomplete
		}
		_, _ = stdout.Write(append(body, '\n'))
	}
	reasons := map[string]int{}
	for _, unit := range decision.Units {
		reasons[unit.Reason]++
	}
	_, _ = fmt.Fprintf(stderr, "corvint-test-reuse: DECIDE reuse %d, run %d of %d package(s) at %s; %s\n", len(decision.Reused), len(decision.Run), len(decision.Units), snapshot.Head, testReuseCounts(reasons))
	return 0
}

func testReuseCounts(counts map[string]int) string {
	if len(counts) == 0 {
		return "none"
	}
	var parts []string
	for _, key := range slices.Sorted(maps.Keys(counts)) {
		parts = append(parts, key+"="+strconv.Itoa(counts[key]))
	}
	return strings.Join(parts, " ")
}

func testReuseWrite(name string, value any) error {
	body, err := json.Marshal(value, json.Deterministic(true))
	if err != nil {
		return err
	}
	temporary := name + ".tmp"
	if err := os.WriteFile(temporary, append(body, '\n'), 0o644); err != nil {
		return err
	}
	return os.Rename(temporary, name)
}
