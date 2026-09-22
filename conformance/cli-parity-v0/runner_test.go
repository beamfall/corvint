package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"slices"
	"sort"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/Beamfall/corvint/internal/procgroup"
)

func TestGPKV0001ManifestIsClosed(t *testing.T) {
	manifestPath := filepath.Join(moduleRootForTest(t), "conformance", "cli-parity-v0", "manifest.json")
	raw, err := os.ReadFile(manifestPath)
	if err != nil {
		t.Fatal(err)
	}
	manifest, err := loadManifest(raw)
	if err != nil {
		t.Fatal(err)
	}
	if manifest.ExpectationProduction.CandidateUsed || len(manifest.Cases) != 133 || len(manifest.Refusals) != 3 {
		t.Fatalf("manifest closure=%#v", manifest)
	}
}

func TestGPKV0005ManifestWorkerDefault(t *testing.T) {
	manifestPath := filepath.Join(moduleRootForTest(t), "conformance", "cli-parity-v0", "manifest.json")
	raw, err := os.ReadFile(manifestPath)
	if err != nil {
		t.Fatal(err)
	}
	var header map[string]json.RawMessage
	if err := json.Unmarshal(raw, &header); err != nil {
		t.Fatal(err)
	}
	if string(header["workers"]) != "1" {
		t.Fatalf("workers=%s, want explicit default 1", header["workers"])
	}
}

func testGPKV0002ManifestReplay(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 9*time.Minute)
	t.Cleanup(cancel)
	root := moduleRootForTest(t)
	candidate := filepath.Join(t.TempDir(), "corvint")
	build := exec.CommandContext(ctx, "go", "build", "-o", candidate, "./cmd/corvint")
	build.Dir = root
	build.Env = append(os.Environ(), "GOTOOLCHAIN=local", "GOCACHE="+filepath.Join(t.TempDir(), "go-cache"))
	if output, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build candidate: %v\n%s", err, output)
	}
	workspace, err := newReplayWorkspace()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = makeTreeWritable(workspace)
		_ = os.RemoveAll(workspace)
	})
	var output bytes.Buffer
	err = replay(ctx, replayOptions{
		ManifestPath: filepath.Join(root, "conformance", "cli-parity-v0", "manifest.json"),
		Candidate:    candidate,
		Output:       &output,
		workspace:    workspace,
	})
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSuffix(output.String(), "\n"), "\n")
	if len(lines) < 2 || !strings.HasPrefix(lines[len(lines)-2], "CANDIDATE-IDENTITY sha256=") || !strings.Contains(lines[len(lines)-2], " go-version-m=") {
		t.Fatalf("candidate identity is not immediately before SUMMARY:\n%s", &output)
	}
	for _, line := range []string{
		"PASS eval-missing-corpus\n",
		"PASS eval-unrecognized-argument\n",
		"PASS impact-clean-tracked\n",
		"PASS-WITH-KNOWN-DIVERGENCE impact-go-root register=DR-0017 clause=GPK-V0-027 rewrites=5",
		// The four rules `GPK-V0-027` gained on 2026-08-29: the module and
		// package forms of `.py`, and the file and directory forms of the web
		// set. Each is an unqualified byte-exact row, so a regression that
		// dropped a resolution arm would surface here and not only in the count.
		"PASS impact-python-module\n",
		"PASS impact-python-nomodule\n",
		"PASS impact-python-package\n",
		"PASS impact-web-component\n",
		"PASS impact-web-directory\n",
		"PASS lrf-evaluate-supported\n",
		"PASS lrf-missing-cem\n",
		"PASS migrate-traces-mode-conflict\n",
		"PASS migrate-traces-mode-required\n",
		"PASS-WITH-ACCEPTED-DIVERGENCE migrate-traces-plan-digest-mismatch oracle-only-created-path=.context-atlas/traces/.trace-operation.lock",
		"PASS-WITH-KNOWN-DIVERGENCE query-clean-authority-start register=DR-0023 clause=GPK-V0-063 rewrites=1",
		"PASS-WITH-KNOWN-DIVERGENCE query-repository-budget-selection register=DR-0023 clause=GPK-V0-063 rewrites=1",
		"PASS-WITH-KNOWN-DIVERGENCE query-repository-default register=DR-0023 clause=GPK-V0-063 rewrites=1",
		"PASS-WITH-KNOWN-DIVERGENCE query-repository-limit-1 register=DR-0025 clause=GPK-V0-040 rewrites=3",
		"PASS-WITH-KNOWN-DIVERGENCE query-repository-limit-1 register=DR-0023 clause=GPK-V0-063 rewrites=1",
		"PASS-WITH-KNOWN-DIVERGENCE query-repository-limit-10 register=DR-0023 clause=GPK-V0-063 rewrites=1",
		"PASS-WITH-KNOWN-DIVERGENCE query-repository-limit-50 register=DR-0023 clause=GPK-V0-063 rewrites=1",
		"PASS-WITH-KNOWN-DIVERGENCE query-repository-task-oversized register=DR-0016 clause=GPK-V0-028 rewrites=1",
		"PASS-WITH-KNOWN-DIVERGENCE query-repository-trace-mixed-worktree register=DR-0023 clause=GPK-V0-063 rewrites=1",
		"PASS-WITH-KNOWN-DIVERGENCE query-repository-out-of-scope register=DR-0008 clause=GPK-V0-039 rewrites=8",
		"PASS-WITH-KNOWN-DIVERGENCE query-version-token register=DR-0015 clause=GPK-V0-043 rewrites=7",
		"PASS-WITH-KNOWN-DIVERGENCE query-version-token register=DR-0023 clause=GPK-V0-063 rewrites=1",
		"PASS ocm-missing-action\n",
		"PASS ocm-report-missing-map\n",
		"PASS ocm-status-missing-map\n",
		"PASS ocm-verify-missing-map\n",
		"PASS record-missing-changed\n",
		"PASS-WITH-ACCEPTED-DIVERGENCE record-untracked-changed oracle-only-created-path=.context-atlas/traces/.trace-operation.lock",
		"UNSUPPORTED query-missing-authority-refusal code=unsupported-query-authority\n",
		"RETIRED lrf-ocm-python-claim-refusal decision=0308 reason=",
		"RETIRED query-present-trace-store-refusal decision=0308 reason=",
		"PARTIAL command:lrf reason=",
		"PASS-WITH-KNOWN-DIVERGENCE impact-self-authored-adr register=DR-0007 clause=CF-V0-031 rewrites=3",
		"PASS-WITH-KNOWN-DIVERGENCE harness-user-prompt-out-of-scope register=DR-0008 clause=GPK-V0-039 rewrites=8",
		"PASS-WITH-IDENTITY-RENAME harness-user-prompt-non-ascii register=DR-0038 clause=CRB-DEC-002 rewrites=1",
		"PASS-WITH-IDENTITY-RENAME harness-user-prompt-out-of-scope register=DR-0038 clause=CRB-DEC-002 rewrites=1",
		"PASS-WITH-KNOWN-DIVERGENCE harness-file-change register=DR-0023 clause=GPK-V0-063 rewrites=1",
		"PASS-WITH-IDENTITY-RENAME harness-file-change register=DR-0038 clause=CRB-DEC-002 rewrites=1",
		"PASS-WITH-IDENTITY-RENAME harness-session-start register=DR-0038 clause=CRB-DEC-002 rewrites=1",
		// Decision 0308: the frozen oracle inputs address `.atlas/` and
		// `.context-atlas/`; these rows are named, never scored, and never PASS.
		"RETIRED cem-prepare-create decision=0308 reason=",
		"RETIRED eval-frozen-corpus decision=0308 reason=",
		"RETIRED migrate-traces-apply-legacy decision=0308 reason=",
		"RETIRED ocm-verify-linked decision=0308 reason=",
		"RETIRED query-repository-agent-tooling decision=0308 reason=",
		"RETIRED query-repository-trace-matching decision=0308 reason=",
		"RETIRED record-create-trace decision=0308 reason=",
		"PARTIAL command:cem reason=",
		"PARTIAL command:query reason=",
		"PASS-WITH-KNOWN-DIVERGENCE impact-ranked-past-limit register=DR-0009 clause=GPK-V0-040 rewrites=3",
		"PASS-WITH-KNOWN-DIVERGENCE query-authority-start-default register=DR-0023 clause=GPK-V0-063 rewrites=1",
		"PASS-WITH-KNOWN-DIVERGENCE query-authority-start-learned register=DR-0023 clause=GPK-V0-063 rewrites=1",
		"PASS-WITH-KNOWN-DIVERGENCE query-authority-start-limit-50 register=DR-0023 clause=GPK-V0-063 rewrites=1",
		"PASS query-authority-start-limit-51\n",
		"PASS-WITH-KNOWN-DIVERGENCE query-authority-start-non-ascii register=DR-0023 clause=GPK-V0-063 rewrites=1",
		"PASS-WITH-KNOWN-DIVERGENCE query-repository-non-ascii register=DR-0023 clause=GPK-V0-063 rewrites=1",
		"SUMMARY parity=104 retired=29 identity-renames=10 accepted-divergences=3 known-divergences=24 location-normalizations=1 structural-fields=0 unsupported-refusals=1 retired-refusals=2 full-gpk-v0-005=PARTIAL detached-descendants=NOT_RUN\n",
	} {
		if !strings.Contains(output.String(), line) {
			t.Fatalf("replay output missing %q:\n%s", line, &output)
		}
	}
	// Every retired row is exactly the pinned set, and no retired id is ever
	// reported under a PASS vocabulary.
	retiredRows := 0
	for _, line := range lines {
		if strings.HasPrefix(line, "RETIRED ") {
			retiredRows++
			if id := strings.Fields(line)[1]; !retiredCases[id] && !retiredRefusalCases[id] {
				t.Fatalf("unpinned retired row %q", line)
			}
		}
		if id := strings.Fields(line)[1]; (strings.HasPrefix(line, "PASS") || strings.HasPrefix(line, "UNSUPPORTED ")) && (retiredCases[id] || retiredRefusalCases[id]) {
			t.Fatalf("retired item reported as certified: %q", line)
		}
	}
	if retiredRows != len(retiredCases)+len(retiredRefusalCases) {
		t.Fatalf("retired rows=%d pinned=%d", retiredRows, len(retiredCases)+len(retiredRefusalCases))
	}
}

func TestGPKV0005ReplayWorkersAreBoundedIsolatedAndOrdered(t *testing.T) {
	started := make(chan int, 4)
	release := make(chan struct{})
	jobs := make([]replayJob, 4)
	for index := range jobs {
		index := index
		jobs[index] = func() (string, error) {
			started <- index
			<-release
			return strconv.Itoa(index), nil
		}
	}
	done := make(chan []replayResult, 1)
	go func() {
		done <- runReplayJobs(2, jobs)
	}()
	for range 2 {
		<-started
	}
	select {
	case index := <-started:
		t.Fatalf("job %d started beyond worker bound", index)
	case <-time.After(50 * time.Millisecond):
	}
	close(release)
	results := <-done
	var output strings.Builder
	for _, result := range results {
		if result.err != nil {
			t.Fatal(result.err)
		}
		output.WriteString(result.output)
	}
	if output.String() != "0123" {
		t.Fatalf("ordered output=%q", output.String())
	}
	workspace := t.TempDir()
	firstRoot, err := replayCaseRoot(workspace, 0)
	if err != nil {
		t.Fatal(err)
	}
	secondRoot, err := replayCaseRoot(workspace, 1)
	if err != nil {
		t.Fatal(err)
	}
	if filepath.Dir(firstRoot) == filepath.Dir(secondRoot) {
		t.Fatalf("case roots share run-state parent: %q and %q", firstRoot, secondRoot)
	}
	if got, want := manifestWorkerCount(runtime.GOMAXPROCS(0)+1), runtime.GOMAXPROCS(0); got != want {
		t.Fatalf("GOMAXPROCS cap=%d, want %d", got, want)
	}
}

func TestGPKV0005ReplayCaseRootCreatesItsPrivateParent(t *testing.T) {
	caseRoot, err := replayCaseRoot(t.TempDir(), 0)
	if err != nil {
		t.Fatal(err)
	}
	if info, err := os.Stat(filepath.Dir(caseRoot)); err != nil || !info.IsDir() {
		t.Fatalf("private case parent: info=%v error=%v", info, err)
	}
}

// TestReplayCandidateDefaultsToScratchBuildFromModuleRoot pins the fix for a manual replay sweep
// run with no --candidate: it must never resolve the manifest's bare command name against PATH
// (a stale install elsewhere on PATH could then be scored as the candidate), and must instead
// build that command fresh from the supplied module root into the replay workspace.
func TestReplayCandidateDefaultsToScratchBuildFromModuleRoot(t *testing.T) {
	moduleRoot := t.TempDir()
	if err := os.WriteFile(filepath.Join(moduleRoot, "go.mod"), []byte("module scratchcandidate\n\ngo 1.21\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	cmdDir := filepath.Join(moduleRoot, "cmd", "scratch-candidate")
	if err := os.MkdirAll(cmdDir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(cmdDir, "main.go"), []byte("package main\n\nfunc main() {}\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	workspace := t.TempDir()
	argv, err := resolveCandidate(context.Background(), workspace, moduleRoot, "", "scratch-candidate --flag")
	if err != nil {
		t.Fatal(err)
	}
	if len(argv) != 2 || argv[1] != "--flag" {
		t.Fatalf("argv=%v, want the built binary followed by the preserved trailing argument", argv)
	}
	if !strings.HasPrefix(argv[0], workspace) {
		t.Fatalf("argv[0]=%q, want a path under the replay workspace %q, not a PATH lookup", argv[0], workspace)
	}
	if _, err := os.Stat(argv[0]); err != nil {
		t.Fatalf("built candidate binary missing: %v", err)
	}
}

func TestGPKV0005CandidateIdentityIsStableAndComplete(t *testing.T) {
	candidate, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	identity, err := readCandidateIdentity(context.Background(), candidate)
	if err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(candidate)
	if err != nil {
		t.Fatal(err)
	}
	buildInfo, err := exec.Command("go", "version", "-m", candidate).Output()
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSuffix(string(buildInfo), "\n"), "\n")
	lines[0] = strings.TrimPrefix(lines[0], candidate+": ")
	encodedBuildInfo, err := json.Marshal(lines)
	if err != nil {
		t.Fatal(err)
	}
	want := fmt.Sprintf("CANDIDATE-IDENTITY sha256=%s go-version-m=%s", sha256Hex(raw), encodedBuildInfo)
	if identity != want {
		t.Fatalf("identity=%q, want %q", identity, want)
	}
	if strings.Contains(identity, candidate) {
		t.Fatalf("identity contains unstable candidate path %q", candidate)
	}
}

func TestGPKV0005CandidateIdentityBindsTheStagedExecutable(t *testing.T) {
	original, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(original)
	if err != nil {
		t.Fatal(err)
	}
	source := filepath.Join(t.TempDir(), "candidate")
	if err := os.WriteFile(source, raw, 0o700); err != nil {
		t.Fatal(err)
	}
	stagedArgv, err := stageCandidate(t.TempDir(), []string{source, "--version"})
	if err != nil {
		t.Fatal(err)
	}
	before, err := readCandidateIdentity(context.Background(), stagedArgv[0])
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(source, []byte("replacement"), 0o700); err != nil {
		t.Fatal(err)
	}
	after, err := readCandidateIdentity(context.Background(), stagedArgv[0])
	if err != nil {
		t.Fatal(err)
	}
	if before != after || stagedArgv[0] == source || stagedArgv[1] != "--version" {
		t.Fatalf("staged identity not bound: before=%q after=%q argv=%q", before, after, stagedArgv)
	}
	info, err := os.Stat(stagedArgv[0])
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o500 {
		t.Fatalf("staged candidate mode=%#o, want 0500", info.Mode().Perm())
	}
}

func TestSweepStaleReplayWorkspaces(t *testing.T) {
	base := t.TempDir()
	old := time.Now().Add(-2 * staleReplayWorkspaceAge)
	stale := []string{
		filepath.Join(base, replayWorkspacePrefix+"one"),
		filepath.Join(base, replayWorkspacePrefix+"two"),
	}
	for _, root := range stale {
		if err := os.MkdirAll(filepath.Join(root, "oracle-source", "src"), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := makeTreeReadOnly(root); err != nil {
			t.Fatal(err)
		}
		if err := os.Chtimes(root, old, old); err != nil {
			t.Fatal(err)
		}
	}
	fresh := filepath.Join(base, replayWorkspacePrefix+"fresh")
	unrelated := filepath.Join(base, "unrelated")
	for _, root := range []string{fresh, unrelated} {
		if err := os.Mkdir(root, 0o700); err != nil {
			t.Fatal(err)
		}
	}
	t.Cleanup(func() {
		for _, root := range stale {
			_ = makeTreeWritable(root)
		}
	})

	sweepStaleReplayWorkspaces(base, time.Now().Add(-staleReplayWorkspaceAge), 1)
	remaining := 0
	for _, root := range stale {
		if _, err := os.Stat(root); err == nil {
			remaining++
		} else if !os.IsNotExist(err) {
			t.Fatal(err)
		}
	}
	if remaining != 1 {
		t.Fatalf("stale replay workspaces remaining=%d, want 1", remaining)
	}
	for _, root := range []string{fresh, unrelated} {
		if _, err := os.Stat(root); err != nil {
			t.Fatalf("preserved workspace %s: %v", root, err)
		}
	}
}

func TestRemoveWritableTree(t *testing.T) {
	root := filepath.Join(t.TempDir(), "replay")
	if err := os.MkdirAll(filepath.Join(root, "oracle-source", "src"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := makeTreeReadOnly(root); err != nil {
		t.Fatal(err)
	}
	removeWritableTree(root)
	if _, err := os.Stat(root); !os.IsNotExist(err) {
		t.Fatalf("removed replay workspace: %v", err)
	}
}

func TestAcceptedDivergenceRejectsCandidateOnlyMutation(t *testing.T) {
	base := acceptedDivergenceSnapshot(t, []snapshotEntry{{Path: "worktree/go.mod", Type: "file", Mode: 0o644, Content: []byte("module example.test/repository\n")}})
	withLock := acceptedDivergenceSnapshot(t, append(append([]snapshotEntry(nil), base.Entries...), snapshotEntry{
		Path: "worktree/.context-atlas/traces/.trace-operation.lock", Type: "file", Mode: 0o600,
	}))
	item := parityCase{
		ID: "migrate-traces-plan-digest-mismatch", Argv: []string{"migrate-traces", "--apply", "--plan-digest", strings.Repeat("0", 64)},
		Mutation: "oracle-only-trace-operation-lock", AcceptedDivergence: &acceptedDivergence{
			OracleOnlyCreatedPath: ".context-atlas/traces/.trace-operation.lock", CandidateMutation: "none", Reason: "operator decision",
		},
	}
	evidence := executionEvidence{Before: base, After: base}
	candidate := executionEvidence{Before: base, After: withLock}
	err := compareCandidateExecution(item, candidate, evidence)
	if err == nil || !strings.Contains(err.Error(), "required-no-mutation") {
		t.Fatalf("error=%v", err)
	}
}

// GPK-V0-008: native replay swaps an accepted row's frozen After digests for its
// Before digests, so nothing else reads them. Each must still be the fixture's
// Before tree plus exactly the declared oracle-only creation.
func TestAcceptedDivergenceFrozenAfterDigestsAreTheDeclaredCreation(t *testing.T) {
	ctx := context.Background()
	manifestPath := filepath.Join(moduleRootForTest(t), "conformance", "cli-parity-v0", "manifest.json")
	manifest, _, err := readManifest(manifestPath, false)
	if err != nil {
		t.Fatal(err)
	}
	workspace, err := newReplayWorkspace()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { removeWritableTree(workspace) })
	accepted := 0
	for index, item := range manifest.Cases {
		if item.AcceptedDivergence == nil {
			continue
		}
		accepted++
		creation, valid := declaredOracleOnlyCreation(item)
		if !valid || item.AcceptedDivergence.CandidateMutation != "none" {
			t.Fatalf("%s: accepted declaration is invalid", item.ID)
		}
		root, err := replayCaseRoot(workspace, index)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := materializeFixture(ctx, filepath.Join(filepath.Dir(manifestPath), "fixtures"), item.Repository, root); err != nil {
			t.Fatal(err)
		}
		if err := applySetups(root, item.Setup); err != nil {
			t.Fatal(err)
		}
		before, err := snapshotRepository(ctx, root)
		if err != nil {
			t.Fatal(err)
		}
		entries := append(append([]snapshotEntry(nil), before.Entries...), snapshotEntry{Path: creation.snapshotPath, Type: creation.entryType, Mode: creation.mode})
		sort.Slice(entries, func(i, j int) bool { return entries[i].Path < entries[j].Path })
		repositoryAfter, fileModesAfter, err := snapshotComponentDigests(entries)
		if err != nil {
			t.Fatal(err)
		}
		got := []string{before.StatusSHA256, before.RepositorySHA256, before.FileModesSHA256, before.StatusSHA256, repositoryAfter, fileModesAfter}
		want := []string{item.StatusBeforeSHA256, item.RepositoryBeforeSHA256, item.FileModesBeforeSHA256, item.StatusAfterSHA256, item.RepositoryAfterSHA256, item.FileModesAfterSHA256}
		if !slices.Equal(got, want) {
			t.Errorf("%s: rebuilt status/repository/fileModes Before,After = %v, frozen = %v", item.ID, got, want)
		}
	}
	if accepted != 3 {
		t.Fatalf("accepted rows = %d, want 3", accepted)
	}
}

// GPK-V0-008, SOL-V0-007: a refusal may append only the gitignored ledger row;
// every other mutation, and a ledger that changes Git status, still fails.
func TestRefusalSnapshotExceptsOnlyTheIgnoredLedger(t *testing.T) {
	module := snapshotEntry{Path: "worktree/go.mod", Type: "file", Mode: 0o644, Content: []byte("module example.test/repository\n")}
	corvint := snapshotEntry{Path: "worktree/.corvint", Type: "directory", Mode: 0o700}
	ledger := snapshotEntry{Path: selfObservationLedgerEntry, Type: "file", Mode: 0o600, Content: []byte(`{"kind":"unsupported"}` + "\n")}
	before := acceptedDivergenceSnapshot(t, []snapshotEntry{corvint, module})
	withEntries := func(entries ...snapshotEntry) repositorySnapshot {
		return acceptedDivergenceSnapshot(t, entries)
	}
	edited := module
	edited.Content = []byte("module example.test/changed\n")
	publicLedger := ledger
	publicLedger.Mode = 0o644
	statusChanged := withEntries(corvint, ledger, module)
	statusChanged.StatusSHA256 = sha256Hex([]byte("?? .corvint/self-observations.jsonl\x00"))
	tests := []struct {
		name    string
		after   repositorySnapshot
		mutated bool
	}{
		{"unchanged", before, false},
		{"ignored ledger row", withEntries(corvint, ledger, module), false},
		{"other file edited", withEntries(corvint, edited), true},
		{"ledger row and other file edited", withEntries(corvint, ledger, edited), true},
		{"other file created", withEntries(corvint, snapshotEntry{Path: "worktree/.corvint/change.lock", Type: "file", Mode: 0o600}, module), true},
		{"ledger not owner-only", withEntries(corvint, publicLedger, module), true},
		{"ledger changes status", statusChanged, true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := refusalMutatedRepository(before, test.after); got != test.mutated {
				t.Fatalf("mutated=%t, want %t", got, test.mutated)
			}
		})
	}
}

func acceptedDivergenceSnapshot(t *testing.T, entries []snapshotEntry) repositorySnapshot {
	t.Helper()
	repositorySHA256, fileModesSHA256, err := snapshotComponentDigests(entries)
	if err != nil {
		t.Fatal(err)
	}
	return repositorySnapshot{
		Entries: entries, StatusSHA256: sha256Hex(nil), RepositorySHA256: repositorySHA256, FileModesSHA256: fileModesSHA256,
	}
}

func TestLocationDependentReplayAcceptsRepositoryRootOnly(t *testing.T) {
	item := locationNormalizedRecordCase()
	captured := locationNormalizedRecordEvidence(t, filepath.Join(string(filepath.Separator), "capture", "root"), "revision.jsonl", "same")
	if err := captureExpectation(&item, captured); err != nil {
		t.Fatal(err)
	}
	replayed := locationNormalizedRecordEvidence(t, filepath.Join(string(filepath.Separator), "different", "environment", "root"), "revision.jsonl", "same")
	if err := compareManifestExpectation(item, replayed); err != nil {
		t.Fatal(err)
	}
}

func TestLocationDependentReplayAcceptsPythonEscapedRepositoryRoots(t *testing.T) {
	item := locationNormalizedRecordCase()
	captured := locationNormalizedRecordEvidence(t, "/capture/røøt<&>", "revision.jsonl", "same")
	captured.Process.Stdout = []byte(`{"note":"same","store":"/capture/r\u00f8\u00f8t<&>/.context-corvint/traces/revision.jsonl"}` + "\n")
	if err := captureExpectation(&item, captured); err != nil {
		t.Fatal(err)
	}
	replayed := locationNormalizedRecordEvidence(t, "/later/é<&>", "revision.jsonl", "same")
	replayed.Process.Stdout = []byte(`{"note":"same","store":"/later/\u00e9<&>/.context-corvint/traces/revision.jsonl"}` + "\n")
	if err := compareManifestExpectation(item, replayed); err != nil {
		t.Fatal(err)
	}
}

func TestLocationDependentReplayRejectsDifferentStoreSuffix(t *testing.T) {
	item := locationNormalizedRecordCase()
	captured := locationNormalizedRecordEvidence(t, filepath.Join(string(filepath.Separator), "capture", "root"), "expected.jsonl", "same")
	if err := captureExpectation(&item, captured); err != nil {
		t.Fatal(err)
	}
	replayed := locationNormalizedRecordEvidence(t, filepath.Join(string(filepath.Separator), "replay", "root"), "different.jsonl", "same")
	err := compareManifestExpectation(item, replayed)
	if err == nil || !strings.Contains(err.Error(), "stdoutSha256") {
		t.Fatalf("error=%v", err)
	}
}

func TestLocationDependentReplayDoesNotNormalizeUndeclaredField(t *testing.T) {
	item := locationNormalizedRecordCase()
	captureRoot := filepath.Join(string(filepath.Separator), "capture", "root")
	replayRoot := filepath.Join(string(filepath.Separator), "replay", "root")
	captured := locationNormalizedRecordEvidence(t, captureRoot, "revision.jsonl", captureRoot)
	if err := captureExpectation(&item, captured); err != nil {
		t.Fatal(err)
	}
	replayed := locationNormalizedRecordEvidence(t, replayRoot, "revision.jsonl", replayRoot)
	err := compareManifestExpectation(item, replayed)
	if err == nil || !strings.Contains(err.Error(), "stdoutSha256") {
		t.Fatalf("error=%v", err)
	}
}

func TestCrossRuntimeParityDoesNotNormalizeLocationDependentOutput(t *testing.T) {
	item := locationNormalizedRecordCase()
	root := filepath.Join(string(filepath.Separator), "same", "root")
	oracle := locationNormalizedRecordEvidence(t, root, "revision.jsonl", "same")
	candidate := locationNormalizedRecordEvidence(t, root, "revision.jsonl", "same")
	candidate.Process.Stdout = recordStdout(t, filepath.Join(string(filepath.Separator), "different", "root", ".context-corvint", "traces", "revision.jsonl"), "same")
	err := compareCandidateExecution(item, candidate, oracle)
	if err == nil || !strings.Contains(err.Error(), "stdout") {
		t.Fatalf("error=%v", err)
	}
}

func TestCrossRuntimeParityRejectsCandidateStoreSuffixDifference(t *testing.T) {
	item := locationNormalizedRecordCase()
	root := filepath.Join(string(filepath.Separator), "same", "root")
	oracle := locationNormalizedRecordEvidence(t, root, "expected.jsonl", "same")
	candidate := locationNormalizedRecordEvidence(t, root, "different.jsonl", "same")
	err := compareCandidateExecution(item, candidate, oracle)
	if err == nil || !strings.Contains(err.Error(), "stdout") {
		t.Fatalf("error=%v", err)
	}
}

func TestLocationDependentReplayRejectsMissingOrExtraField(t *testing.T) {
	item := locationNormalizedRecordCase()
	root := filepath.Join(string(filepath.Separator), "same", "root")
	captured := locationNormalizedRecordEvidence(t, root, "revision.jsonl", "same")
	if err := captureExpectation(&item, captured); err != nil {
		t.Fatal(err)
	}
	for name, stdout := range map[string][]byte{
		"missing store": []byte("{\"note\":\"same\"}\n"),
		"extra field":   append(bytes.TrimSuffix(captured.Process.Stdout, []byte("}\n")), []byte(",\"extra\":true}\n")...),
	} {
		t.Run(name, func(t *testing.T) {
			replayed := captured
			replayed.Process.Stdout = stdout
			if err := compareManifestExpectation(item, replayed); err == nil {
				t.Fatal("changed field set replayed")
			}
		})
	}
}

func TestLocationNormalizationDeclarationIsClosed(t *testing.T) {
	valid := locationNormalizedRecordCase()
	if !validLocationNormalization(valid) {
		t.Fatal("exact record store declaration rejected")
	}
	for name, mutate := range map[string]func(*parityCase){
		"arbitrary field": func(item *parityCase) { item.LocationNormalization.Fields = []string{"stdout.trace.task"} },
		"latency":         func(item *parityCase) { item.LocationNormalization.Fields = []string{"stdout.latency_ms"} },
		"wrong kind":      func(item *parityCase) { item.LocationNormalization.Kind = "ignore-path" },
		"missing reason":  func(item *parityCase) { item.LocationNormalization.Reason = "" },
		"missing declaration": func(item *parityCase) {
			item.LocationNormalization = nil
		},
		"combined divergence": func(item *parityCase) {
			item.AcceptedDivergence = &acceptedDivergence{Reason: "not allowed"}
		},
		"hostile case": func(item *parityCase) {
			item.ID = "record-untracked-changed"
			item.Argv[3] = "missing.go"
		},
	} {
		t.Run(name, func(t *testing.T) {
			item := locationNormalizedRecordCase()
			mutate(&item)
			if validLocationNormalization(item) {
				t.Fatal("invalid location normalization accepted")
			}
		})
	}
}

func locationNormalizedRecordCase() parityCase {
	return parityCase{
		ID:       "record-create-trace",
		Argv:     []string{"record", "--task", "task", "--changed", "internal/example/value.go", "--verify", "go test ./...", "--outcome", "passed"},
		Mutation: "trace-store",
		LocationNormalization: &locationNormalization{
			Fields: []string{"stdout.store"}, Kind: "repository-root-prefix", Reason: "accepted decision 0004",
		},
	}
}

func locationNormalizedRecordEvidence(t *testing.T, root, suffix, note string) executionEvidence {
	t.Helper()
	return executionEvidence{
		Fixture: materializedFixture{
			Root: root, CommitRevision: strings.Repeat("1", 40), TreeRevision: strings.Repeat("2", 40), FixtureSHA256: strings.Repeat("3", 64),
		},
		Process: procgroup.Observation{Stdout: recordStdout(t, filepath.Join(root, ".context-corvint", "traces", suffix), note)},
	}
}

func recordStdout(t *testing.T, store, note string) []byte {
	t.Helper()
	raw, err := json.Marshal(map[string]string{"note": note, "store": store})
	if err != nil {
		t.Fatal(err)
	}
	return append(raw, '\n')
}

func TestCaptureCLIRejectsCandidateAuthority(t *testing.T) {
	marker := filepath.Join(t.TempDir(), "candidate-marker")
	err := runCLI(context.Background(), []string{"capture", "--candidate", marker})
	if err == nil || !strings.Contains(err.Error(), "flag provided but not defined") {
		t.Fatalf("capture error=%v", err)
	}
	if _, err := os.Stat(marker); !os.IsNotExist(err) {
		t.Fatalf("candidate marker exists: %v", err)
	}
}

func TestSeededManifestDigestDivergenceHasStableFailureLine(t *testing.T) {
	item := parityCase{
		ID: "seeded-divergence", CommitRevision: strings.Repeat("1", 40), TreeRevision: strings.Repeat("2", 40),
		FixtureSHA256: strings.Repeat("3", 64), StdoutSHA256: strings.Repeat("0", 64),
		StderrSHA256: sha256Hex(nil), StatusBeforeSHA256: sha256Hex(nil), StatusAfterSHA256: sha256Hex(nil),
		RepositoryBeforeSHA256: strings.Repeat("4", 64), RepositoryAfterSHA256: strings.Repeat("4", 64),
		FileModesBeforeSHA256: strings.Repeat("5", 64), FileModesAfterSHA256: strings.Repeat("5", 64),
	}
	actual := executionEvidence{
		Fixture: materializedFixture{CommitRevision: item.CommitRevision, TreeRevision: item.TreeRevision, FixtureSHA256: item.FixtureSHA256},
		Before:  repositorySnapshot{StatusSHA256: item.StatusBeforeSHA256, RepositorySHA256: item.RepositoryBeforeSHA256, FileModesSHA256: item.FileModesBeforeSHA256},
		After:   repositorySnapshot{StatusSHA256: item.StatusAfterSHA256, RepositorySHA256: item.RepositoryAfterSHA256, FileModesSHA256: item.FileModesAfterSHA256},
		Process: procgroup.Observation{Stdout: []byte("oracle bytes")},
	}
	err := compareManifestExpectation(item, actual)
	want := "FAIL seeded-divergence stdoutSha256: candidate=" + sha256Hex(actual.Process.Stdout) + " manifest=" + item.StdoutSHA256
	if err == nil || err.Error() != want {
		t.Fatalf("error=%q want=%q", err, want)
	}
}

func moduleRootForTest(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("cannot resolve module root")
	}
	return filepath.Clean(filepath.Join(filepath.Dir(file), "..", ".."))
}

// TestFilteredRunsCannotProduceCorpusEvidence pins the two fail-closed guards on
// --only. The flag exists to make authoring iterations cheap; neither a written
// manifest nor a corpus SUMMARY may ever come out of a partial run.
func TestFilteredRunsCannotProduceCorpusEvidence(t *testing.T) {
	manifestPath := filepath.Join(moduleRootForTest(t), "conformance", "cli-parity-v0", "manifest.json")
	var output bytes.Buffer
	err := capture(context.Background(), captureOptions{
		ManifestPath: manifestPath, Only: "feature-", Write: true, Output: &output,
	})
	if err == nil || !strings.Contains(err.Error(), "capture retired under decision 0088") {
		t.Fatalf("partial capture must refuse --write, got %v", err)
	}
	if output.Len() != 0 {
		t.Fatalf("refused capture wrote output: %q", output.String())
	}
}

func TestFilterSelectsByIDPrefix(t *testing.T) {
	cases := []struct {
		id, prefix string
		selected   bool
	}{
		{"feature-known-id", "", true},
		{"feature-known-id", "feature-", true},
		{"feature-known-id", "feature-known", true},
		{"feature-known-id", "init-", false},
		{"init-sealed-summary", "feature-", false},
	}
	for _, test := range cases {
		if got := selectedByPrefix(test.id, test.prefix); got != test.selected {
			t.Fatalf("selectedByPrefix(%q, %q)=%v", test.id, test.prefix, got)
		}
	}
}

// TestKnownDivergenceRejectsACandidateThatDropsTheSpecRequiredForm proves the
// DR-0007 declaration is not an exclusion. A candidate that stops emitting the
// withheld-authority form `CF-V0-032` requires -- the pre-`CF-V0-031` behavior,
// which agreed with the oracle -- fails the declared rewrite instead of passing
// quietly, so the corpus discriminates the defect it was added for.
func TestKnownDivergenceRejectsACandidateThatDropsTheSpecRequiredForm(t *testing.T) {
	item := knownDivergenceCase()
	oracle := []byte(`{"a":{"authority":"accepted-contract","confidence":"authoritative"},"uncertainty":[],"packet_bytes":40}` + "\n")
	laundering := []byte(`{"a":{"authority":"accepted-contract","confidence":"authoritative"},"uncertainty":[],"packet_bytes":40}` + "\n")
	if _, err := applyKnownDivergence(item, laundering, oracle); err == nil ||
		!strings.Contains(err.Error(), "occurs 0 times") {
		t.Fatalf("a self-certifying candidate was admitted: %v", err)
	}
}

// TestKnownDivergenceRejectsAnUnaccountedByte proves the arithmetic reconciliation
// of `packet_bytes` closes the one member no clause can author: a candidate whose
// receipt moved for any reason other than the declared rewrites breaks the
// identity, even though every declared rewrite still matches.
func TestKnownDivergenceRejectsAnUnaccountedByte(t *testing.T) {
	item := knownDivergenceCase()
	oracle := []byte(`{"authority":"accepted-contract","confidence":"authoritative","uncertainty":[],"packet_bytes":40}` + "\n")
	candidate := []byte(`{"authority":"unverified-contract","confidence":"low","uncertainty":["x no caller-independent revision is available y"],"packet_bytes":99}` + "\n")
	if _, err := applyKnownDivergence(item, candidate, oracle); err == nil ||
		!strings.Contains(err.Error(), "not accounted for") {
		t.Fatalf("an unaccounted receipt byte was admitted: %v", err)
	}
}

// TestKnownDivergenceReconcilesAPacketBytesWidthChange proves the byte-count
// identity holds when a declared rewrite moves `packet_bytes` across a digit
// boundary. The count encodes its own member, so the packet grows by that
// member's width too, and without that term the identity is off by exactly the
// width change -- which is what DR-0008 does for real, withdrawing a 1720-byte
// packet to an 838-byte one.
func TestKnownDivergenceReconcilesAPacketBytesWidthChange(t *testing.T) {
	item := relevanceFloorDivergenceCase()
	oracle := []byte(relevanceFloorPublishedStdout(1000))
	candidate := []byte(relevanceFloorWithdrawnStdout(947))
	rewritten, err := applyKnownDivergence(item, candidate, oracle)
	if err != nil {
		t.Fatalf("a correctly withdrawn packet was rejected: %v", err)
	}
	if !bytes.Equal(rewritten, oracle) {
		t.Fatalf("rewritten stdout is not the oracle's: %s", rewritten)
	}
}

// TestKnownDivergenceRejectsAWidthChangeThatHidesAByte proves the width term
// widened nothing: one unaccounted byte inside the withdrawn packet still breaks
// the identity even though every declared rewrite matches and the count crosses
// a digit boundary.
func TestKnownDivergenceRejectsAWidthChangeThatHidesAByte(t *testing.T) {
	item := relevanceFloorDivergenceCase()
	oracle := []byte(relevanceFloorPublishedStdout(1000))
	candidate := []byte(relevanceFloorWithdrawnStdout(948))
	if _, err := applyKnownDivergence(item, candidate, oracle); err == nil ||
		!strings.Contains(err.Error(), "not accounted for") {
		t.Fatalf("an unaccounted receipt byte was admitted: %v", err)
	}
}

func TestStandaloneRelevanceFloorDivergenceRequiresObservableAbstention(t *testing.T) {
	manifest, _, err := readManifest(filepath.Join(moduleRootForTest(t), "conformance", "cli-parity-v0", "manifest.json"), false)
	if err != nil {
		t.Fatal(err)
	}
	var item parityCase
	for _, candidate := range manifest.Cases {
		if candidate.ID == "query-repository-out-of-scope" {
			item = candidate
			break
		}
	}
	if !validKnownDivergence(item) || len(item.KnownDivergence.Rewrites) != 8 {
		t.Fatal("standalone relevance-floor declaration is not the closed eight-member shape")
	}
	divergence := *item.KnownDivergence
	divergence.Rewrites = append([]divergenceRewrite(nil), divergence.Rewrites[1:]...)
	item.KnownDivergence = &divergence
	if validKnownDivergence(item) {
		t.Fatal("standalone declaration without its observable abstention rewrite was admitted")
	}
}

// TestTraceRevisionDisclosureDivergenceNamesTheManifestRevision pins DR-0035:
// the committed declaration is admitted, and one whose candidate side names any
// revision other than the manifest's own commit is refused.
func TestTraceRevisionDisclosureDivergenceNamesTheManifestRevision(t *testing.T) {
	manifest, _, err := readManifest(filepath.Join(moduleRootForTest(t), "conformance", "cli-parity-v0", "manifest.json"), false)
	if err != nil {
		t.Fatal(err)
	}
	var item parityCase
	for _, candidate := range manifest.Cases {
		if candidate.ID == "query-repository-trace-matching" {
			item = candidate
			break
		}
	}
	if !validKnownDivergence(item) {
		t.Fatal("committed DR-0035 declaration is not admitted")
	}
	divergence := *item.KnownDivergence
	divergence.Rewrites = append([]divergenceRewrite(nil), divergence.Rewrites...)
	divergence.Rewrites[1].Candidate = strings.Replace(divergence.Rewrites[1].Candidate, item.CommitRevision[:12], "000000000000", 1)
	item.KnownDivergence = &divergence
	if validKnownDivergence(item) {
		t.Fatal("DR-0035 declaration naming a revision other than the manifest commit was admitted")
	}
}

// TestImpactArgvIsClosedToThePortedSeed pins the `impact` argv allowlist that
// `docs/reviews/cli-parity-mutation-audit-2026-09-13.md` found missing: every
// committed `impact` row is admitted, and a copy with its trailing `--limit 10`
// dropped -- the seven rows the audit found neither the self-check nor the
// comparison could see -- is refused.
func TestImpactArgvIsClosedToThePortedSeed(t *testing.T) {
	manifest, _, err := readManifest(filepath.Join(moduleRootForTest(t), "conformance", "cli-parity-v0", "manifest.json"), false)
	if err != nil {
		t.Fatal(err)
	}
	found := 0
	for _, item := range manifest.Cases {
		if item.Argv[0] != "impact" {
			continue
		}
		found++
		if !validImpactArgv(item) {
			t.Fatalf("committed impact argv on %s is not admitted", item.ID)
		}
		if len(item.Argv) < 4 || item.Argv[2] != "--limit" || item.Argv[3] != "10" {
			continue
		}
		mutated := item
		mutated.Argv = append([]string(nil), item.Argv[:2]...)
		if validImpactArgv(mutated) {
			t.Fatalf("impact argv on %s with --limit 10 dropped was admitted", item.ID)
		}
	}
	if found != 9 {
		t.Fatalf("impact cases = %d, want 9", found)
	}
}

// TestRootPackageDivergenceRejectsAnUnpinnedImporterRewrite pins DR-0017's
// reverse-import rewrite exactly, closing the gap the audit found: appending
// one byte to the declared candidate text used to still satisfy the
// `strings.Contains` admission, so only the comparison -- not the self-check
// -- caught a changed declaration.
func TestRootPackageDivergenceRejectsAnUnpinnedImporterRewrite(t *testing.T) {
	manifest, _, err := readManifest(filepath.Join(moduleRootForTest(t), "conformance", "cli-parity-v0", "manifest.json"), false)
	if err != nil {
		t.Fatal(err)
	}
	var item parityCase
	for _, candidate := range manifest.Cases {
		if candidate.ID == "impact-go-root" {
			item = candidate
			break
		}
	}
	if !validKnownDivergence(item) {
		t.Fatal("committed DR-0017 declaration is not admitted")
	}
	divergence := *item.KnownDivergence
	divergence.Rewrites = append([]divergenceRewrite(nil), divergence.Rewrites...)
	divergence.Rewrites[0].Candidate += "x"
	item.KnownDivergence = &divergence
	if validKnownDivergence(item) {
		t.Fatal("DR-0017 declaration with a byte appended to its importer rewrite was admitted")
	}
}

// TestExclusionCountDivergenceAdmitsOnlyALargerCount pins DR-0023: every
// committed declaration is admitted, and a copy whose candidate count no longer
// exceeds the oracle's, whose region is not closed by the comma, or that sits on
// a case outside the measured set is refused.
func TestExclusionCountDivergenceAdmitsOnlyALargerCount(t *testing.T) {
	manifest, _, err := readManifest(filepath.Join(moduleRootForTest(t), "conformance", "cli-parity-v0", "manifest.json"), false)
	if err != nil {
		t.Fatal(err)
	}
	var item parityCase
	declared := 0
	for _, candidate := range manifest.Cases {
		if candidate.ExclusionCountDivergence == nil {
			continue
		}
		declared++
		item = candidate
		if !validExclusionCountDivergence(candidate) {
			t.Fatalf("committed DR-0023 declaration on %s is not admitted", candidate.ID)
		}
	}
	if declared != len(exclusionCountDivergenceCases) {
		t.Fatalf("DR-0023 declarations = %d, measured set = %d", declared, len(exclusionCountDivergenceCases))
	}
	for name, mutate := range map[string]func(*parityCase){
		"equal count":  func(c *parityCase) { c.ExclusionCountDivergence.Rewrites[0].Candidate = `"exclusions":{"count":0,` },
		"open region":  func(c *parityCase) { c.ExclusionCountDivergence.Rewrites[0].Candidate = `"exclusions":{"count":1` },
		"other case":   func(c *parityCase) { c.ID = "query-repository-out-of-scope" },
		"other member": func(c *parityCase) { c.ExclusionCountDivergence.Rewrites[0].Oracle = `"unparsed":{"count":0,` },
	} {
		mutated := item
		divergence := *item.ExclusionCountDivergence
		divergence.Rewrites = append([]divergenceRewrite(nil), divergence.Rewrites...)
		mutated.ExclusionCountDivergence = &divergence
		mutate(&mutated)
		if validExclusionCountDivergence(mutated) {
			t.Fatalf("DR-0023 declaration with %s was admitted", name)
		}
	}
}

// TestDecision0308DeclarationsAreClosed pins the two decision-0308 declarations
// to their sets: every committed retirement and identity rename is admitted,
// the counts equal the pinned sets, and each mutation that would widen either
// declaration is rejected.
func TestDecision0308DeclarationsAreClosed(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join(moduleRootForTest(t), "conformance", "cli-parity-v0", "manifest.json"))
	if err != nil {
		t.Fatal(err)
	}
	manifest, err := loadManifest(raw)
	if err != nil {
		t.Fatal(err)
	}
	var retired, renamed parityCase
	retiredDeclared, renamedDeclared := 0, 0
	for _, item := range manifest.Cases {
		if item.Retired != nil {
			retiredDeclared++
			retired = item
			if !validRetired(item) {
				t.Fatalf("committed retirement of %s is not admitted", item.ID)
			}
		}
		if item.IdentityRenameDivergence != nil {
			renamedDeclared++
			renamed = item
			if !validIdentityRenameDivergence(item) {
				t.Fatalf("committed DR-0038 declaration on %s is not admitted", item.ID)
			}
		}
	}
	if retiredDeclared != len(retiredCases) || retiredDeclared != 29 || renamedDeclared != len(identityRenameDivergenceCases) || renamedDeclared != 10 {
		t.Fatalf("retired=%d/%d identity-renames=%d/%d", retiredDeclared, len(retiredCases), renamedDeclared, len(identityRenameDivergenceCases))
	}
	retiredRefusals := 0
	for _, item := range manifest.Refusals {
		if item.Retired == nil {
			continue
		}
		retiredRefusals++
		if !validRetiredRefusal(item) {
			t.Fatalf("committed retirement of refusal %s is not admitted", item.ID)
		}
		other := item
		other.ID = "query-missing-authority-refusal"
		if validRetiredRefusal(other) {
			t.Fatal("refusal retirement on an unpinned id was admitted")
		}
	}
	if retiredRefusals != len(retiredRefusalCases) || retiredRefusals != 2 {
		t.Fatalf("retired-refusals=%d/%d", retiredRefusals, len(retiredRefusalCases))
	}
	for name, mutate := range map[string]func(*parityCase){
		"other case":     func(c *parityCase) { c.ID = "impact-clean-tracked" },
		"other decision": func(c *parityCase) { c.Retired.Decision = "0088" },
		"missing reason": func(c *parityCase) { c.Retired.Reason = "" },
	} {
		mutated := retired
		declaration := *retired.Retired
		mutated.Retired = &declaration
		mutate(&mutated)
		if validRetired(mutated) {
			t.Fatalf("retirement with %s was admitted", name)
		}
	}
	for name, mutate := range map[string]func(*parityCase){
		"other case":     func(c *parityCase) { c.ID = "query-repository-default" },
		"other register": func(c *parityCase) { c.IdentityRenameDivergence.Register = "DR-0023" },
		"other member": func(c *parityCase) {
			c.IdentityRenameDivergence.Rewrites[0].Oracle = `"profile":"atlas-dogfood-event/0"`
		},
		"extra rewrite": func(c *parityCase) {
			c.IdentityRenameDivergence.Rewrites = append(c.IdentityRenameDivergence.Rewrites, divergenceRewrite{Candidate: "a", Oracle: "b"})
		},
		"stderr rewrite": func(c *parityCase) {
			c.IdentityRenameDivergence.StderrRewrites = []divergenceRewrite{{Candidate: "a", Oracle: "b"}}
		},
		"retired too": func(c *parityCase) { c.Retired = &retiredCase{Decision: "0308", Reason: "x"} },
	} {
		mutated := renamed
		divergence := *renamed.IdentityRenameDivergence
		divergence.Rewrites = append([]divergenceRewrite(nil), divergence.Rewrites...)
		mutated.IdentityRenameDivergence = &divergence
		mutate(&mutated)
		if validIdentityRenameDivergence(mutated) {
			t.Fatalf("DR-0038 declaration with %s was admitted", name)
		}
	}
	// The manifest itself refuses a dropped declaration of either kind.
	for name, mutate := range map[string]func(*parityManifest){
		"dropped retirement": func(m *parityManifest) {
			for index := range m.Cases {
				if m.Cases[index].Retired != nil {
					m.Cases[index].Retired = nil
					return
				}
			}
		},
		"dropped refusal retirement": func(m *parityManifest) {
			m.Refusals = append([]refusalCase(nil), m.Refusals...)
			for index := range m.Refusals {
				if m.Refusals[index].Retired != nil {
					m.Refusals[index].Retired = nil
					return
				}
			}
		},
		"dropped rename": func(m *parityManifest) {
			for index := range m.Cases {
				if m.Cases[index].IdentityRenameDivergence != nil {
					m.Cases[index].IdentityRenameDivergence = nil
					return
				}
			}
		},
	} {
		mutated := manifest
		mutated.Cases = append([]parityCase(nil), manifest.Cases...)
		mutate(&mutated)
		if validateManifest(mutated) == nil {
			t.Fatalf("manifest with %s was accepted", name)
		}
	}
}

// relevanceFloorWithdrawnStdout is the withdrawn form GPK-V0-039 requires, with
// the receipt's own byte count left free so a test can move it across a digit
// boundary.
func relevanceFloorWithdrawnStdout(packetBytes int) string {
	return `{"abstention":{"active":true,"reason":"below-relevance-floor"},"authoritative_results":0,"critical":[],"included_results":0,"packet_bytes":` +
		strconv.Itoa(packetBytes) +
		`,"requested_results":0,"results":[],"state":"OUT_OF_SCOPE","verification":["make gate"]}` + "\n"
}

// relevanceFloorPublishedStdout is what the oracle emits in its place: the same
// members carrying the packet the floor withdraws.
func relevanceFloorPublishedStdout(packetBytes int) string {
	return `{"abstention":{"active":false,"reason":"none"},"authoritative_results":1,"critical":["feature:a11y-high-contrast"],"included_results":1,"packet_bytes":` +
		strconv.Itoa(packetBytes) +
		`,"requested_results":1,"results":[{"id":"a11y-high-contrast"}],"state":"READY","verification":["go test ./pkg/...","make gate"]}` + "\n"
}

func relevanceFloorDivergenceCase() parityCase {
	return parityCase{
		ID: "harness-user-prompt-out-of-scope", Argv: []string{"harness", "event"}, Mutation: "none",
		KnownDivergence: &knownDivergence{
			Register: "DR-0008", Clause: "GPK-V0-039", Reason: "relevance floor",
			Rewrites: []divergenceRewrite{
				{Candidate: `"abstention":{"active":true,"reason":"below-relevance-floor"}`, Oracle: `"abstention":{"active":false,"reason":"none"}`},
				{Candidate: `"authoritative_results":0`, Oracle: `"authoritative_results":1`},
				{Candidate: `"critical":[]`, Oracle: `"critical":["feature:a11y-high-contrast"]`},
				{Candidate: `"included_results":0`, Oracle: `"included_results":1`},
				{Candidate: `"requested_results":0`, Oracle: `"requested_results":1`},
				{Candidate: `"results":[]`, Oracle: `"results":[{"id":"a11y-high-contrast"}]`},
				{Candidate: `"state":"OUT_OF_SCOPE"`, Oracle: `"state":"READY"`},
				{Candidate: `"verification":["make gate"]`, Oracle: `"verification":["go test ./pkg/...","make gate"]`},
			},
		},
	}
}

// TestKnownDivergenceReconcilesATruncatedCoverageReceipt proves the DR-0009
// declaration accepts exactly the receipt `GPK-V0-040` requires: the counts
// denominated in the admitted universe, the omission named in `uncertainty`,
// and `packet_bytes` reconciled from the declared rewrites alone.
func TestKnownDivergenceReconcilesATruncatedCoverageReceipt(t *testing.T) {
	item := admittedUniverseDivergenceCase(4, 7, 3)
	oracle := []byte(admittedUniverseStdout(`"omitted_results":0`, `"requested_results":3`, `"uncertainty":[]`, 100))
	candidate := []byte(admittedUniverseStdout(`"omitted_results":4`, `"requested_results":7`,
		`"uncertainty":["4 ranked results omitted by result limit"]`, 142))
	rewritten, err := applyKnownDivergence(item, candidate, oracle)
	if err != nil {
		t.Fatalf("a correctly denominated receipt was rejected: %v", err)
	}
	if !bytes.Equal(rewritten, oracle) {
		t.Fatalf("rewritten stdout is not the oracle's: %s", rewritten)
	}
}

// TestKnownDivergenceRejectsACoverageIdentityThatDoesNotClose proves the
// declaration carries no free number. `GPK-V0-040` fixes the admitted count as
// the emitted count plus the results the ceiling dropped, so a `requested_results`
// rewrite that satisfies neither side of that identity is refused as an invalid
// declaration rather than admitted as a divergence.
func TestKnownDivergenceRejectsACoverageIdentityThatDoesNotClose(t *testing.T) {
	item := admittedUniverseDivergenceCase(4, 8, 3)
	oracle := []byte(admittedUniverseStdout(`"omitted_results":0`, `"requested_results":3`, `"uncertainty":[]`, 100))
	candidate := []byte(admittedUniverseStdout(`"omitted_results":4`, `"requested_results":8`,
		`"uncertainty":["4 ranked results omitted by result limit"]`, 142))
	if _, err := applyKnownDivergence(item, candidate, oracle); err == nil ||
		!strings.Contains(err.Error(), "declaration is invalid") {
		t.Fatalf("an unclosed coverage identity was admitted: %v", err)
	}
}

// admittedUniverseStdout is one receipt carrying only the three members DR-0009
// declares plus the byte count the runner reconciles.
func admittedUniverseStdout(omitted, requested, uncertainty string, packetBytes int) string {
	return `{"coverage":{` + omitted + `,"packet_bytes":` + strconv.Itoa(packetBytes) + `,` +
		requested + `,` + uncertainty + "}}\n"
}

func admittedUniverseDivergenceCase(dropped, admitted, emitted int) parityCase {
	return parityCase{
		ID: "impact-ranked-past-limit", Argv: []string{"impact", "pkg/sample.go", "--limit", "3"}, Mutation: "none",
		KnownDivergence: &knownDivergence{
			Register: "DR-0009", Clause: "GPK-V0-040", Reason: "admitted universe",
			Rewrites: []divergenceRewrite{
				{Candidate: `"omitted_results":` + strconv.Itoa(dropped), Oracle: `"omitted_results":0`},
				{Candidate: `"requested_results":` + strconv.Itoa(admitted), Oracle: `"requested_results":` + strconv.Itoa(emitted)},
				{
					Candidate: `"uncertainty":["` + strconv.Itoa(dropped) + ` ranked results omitted by result limit"]`,
					Oracle:    `"uncertainty":[]`,
				},
			},
		},
	}
}

func knownDivergenceCase() parityCase {
	return parityCase{
		ID: "impact-self-authored-adr", Argv: []string{"impact", "pkg/sample.go", "--limit", "10"}, Mutation: "none",
		KnownDivergence: &knownDivergence{
			Register: "DR-0007", Clause: "CF-V0-031", Reason: "decision 0006",
			Rewrites: []divergenceRewrite{
				{Candidate: `"authority":"unverified-contract"`, Oracle: `"authority":"accepted-contract"`},
				{Candidate: `"confidence":"low"`, Oracle: `"confidence":"authoritative"`},
				{Candidate: `"uncertainty":["x no caller-independent revision is available y"]`, Oracle: `"uncertainty":[]`},
			},
		},
	}
}

func TestGPKV0002ManifestReplay(t *testing.T) {
	t.Run("GOC-V0-002 immutable expectations without a live oracle", testGPKV0002ManifestReplay)
}

// TestGPKV0002ReplayRefusesLiveOracle falsifies a candidate replay that tries
// to execute, reconstruct or install the retired Python oracle: an --oracle
// override must be refused before any process is started, never silently
// ignored or run.
func TestGPKV0002ReplayRefusesLiveOracle(t *testing.T) {
	err := replay(context.Background(), replayOptions{
		ManifestPath: filepath.Join(moduleRootForTest(t), "conformance", "cli-parity-v0", "manifest.json"),
		Oracle:       "python3 -m corvint_cli",
		Output:       &bytes.Buffer{},
	})
	want := "live oracle overrides are retired under decision 0088"
	if err == nil || err.Error() != want {
		t.Fatalf("GOC-V0-002 error=%v want=%q", err, want)
	}
}

// TestGPKV0002CaptureCannotRegenerateExpectationsFromCandidate falsifies the
// other half of GOC-V0-002: expectations must never be produced by running
// the candidate against itself. capture must refuse unconditionally, even
// with Write requested.
func TestGPKV0002CaptureCannotRegenerateExpectationsFromCandidate(t *testing.T) {
	err := capture(context.Background(), captureOptions{
		ManifestPath: filepath.Join(moduleRootForTest(t), "conformance", "cli-parity-v0", "manifest.json"),
		Write:        true,
		Output:       &bytes.Buffer{},
	})
	want := "capture retired under decision 0088; frozen expectations are immutable and cannot be regenerated from the candidate"
	if err == nil || err.Error() != want {
		t.Fatalf("GOC-V0-002 error=%v want=%q", err, want)
	}
}

func TestCompareManifestExpectationReportsTimeoutBeforeStdoutDigestMismatch(t *testing.T) {
	item := parityCase{
		ID:                       "timeout-precedence",
		StdoutSHA256:             sha256Hex([]byte("expected stdout")),
		ProcessStarted:           true,
		WaitCompleted:            true,
		PipesDrained:             true,
		OwnedProcessGroupCleanup: true,
		CleanupScope:             "owned-process-group",
	}
	actual := executionEvidence{Process: procgroup.Observation{
		Started: true, TimedOut: true, WaitCompleted: true, PipesDrained: true,
		OwnedProcessGroupCleanup: true, DescendantCleanupStatus: "owned-process-group",
	}}

	err := compareManifestExpectation(item, actual)
	want := "FAIL timeout-precedence timedOut: candidate=true manifest=false"
	if err == nil || err.Error() != want {
		t.Fatalf("error=%q want=%q", err, want)
	}
}
