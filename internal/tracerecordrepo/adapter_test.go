package tracerecordrepo

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/contextindex"
	"github.com/Beamfall/corvint/internal/trace"
)

// GPK-V0-050.
func TestDogfoodRecordAdmitsNestedGitignoreWithoutIndexingIt(t *testing.T) {
	root := newAdapterFixture(t)
	base := gitAdapterFixture(t, root, "rev-parse", "HEAD")
	path := "config/local/.gitignore"
	writeAdapterFixture(t, root, path, "*.tmp\n")
	gitAdapterFixture(t, root, "add", path)
	gitAdapterFixture(t, root, "commit", "-qm", "gitignore")
	target := gitAdapterFixture(t, root, "rev-parse", "HEAD")

	index, err := contextindex.Build(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
	if _, indexed := index.Sources[path]; indexed {
		t.Fatalf("%s entered index sources", path)
	}
	for _, symbol := range index.Symbols {
		if symbol.Path == path {
			t.Fatalf("%s contributed symbol %+v", path, symbol)
		}
	}

	result, err := RecordDogfood(context.Background(), root, base, target, Input{Task: "gitignore", Outcome: "passed"})
	if err != nil {
		t.Fatal(err)
	}
	want := []string{path}
	if result.State != "recorded" || !reflect.DeepEqual(result.Admitted, want) || result.Recorded == nil ||
		!reflect.DeepEqual(result.Recorded.Record.ChangedPaths, want) {
		t.Fatalf("result=%+v", result)
	}
	if result.AdmittedSHA256 != digestPaths(want) || result.AdmittedSHA256 == digestPaths(nil) {
		t.Fatalf("admitted digest=%q", result.AdmittedSHA256)
	}

	recorded, err := Record(context.Background(), root, Input{Task: "direct gitignore", ChangedPaths: want, Outcome: "passed"})
	if err != nil || !reflect.DeepEqual(recorded.Record.ChangedPaths, want) {
		t.Fatalf("record=%+v error=%v", recorded, err)
	}
}

// GPK-V0-050.
func TestDogfoodRecordStillRejectsUnrelatedNonSourcePath(t *testing.T) {
	root := newAdapterFixture(t)
	base := gitAdapterFixture(t, root, "rev-parse", "HEAD")
	writeAdapterFixture(t, root, "README.csv", "not an index source\n")
	gitAdapterFixture(t, root, "add", "README.csv")
	gitAdapterFixture(t, root, "commit", "-qm", "readme")
	target := gitAdapterFixture(t, root, "rev-parse", "HEAD")

	result, err := RecordDogfood(context.Background(), root, base, target, Input{Task: "readme", Outcome: "passed"})
	if err != nil {
		t.Fatal(err)
	}
	if result.State != "no-source-paths" || len(result.Admitted) != 0 {
		t.Fatalf("result=%+v", result)
	}
}

// LTPM-V0-011.
func TestDogfoodStabilityBindsCandidateAndAdmittedDigests(t *testing.T) {
	root := newAdapterFixture(t)
	base := gitAdapterFixture(t, root, "rev-parse", "HEAD")
	writeAdapterFixture(t, root, "internal/value.go", "package internal\n\nconst Changed = true\n")
	gitAdapterFixture(t, root, "add", "internal/value.go")
	gitAdapterFixture(t, root, "commit", "-qm", "target")
	target := gitAdapterFixture(t, root, "rev-parse", "HEAD")

	index, err := contextindex.Build(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
	candidates, err := changedPaths(context.Background(), root, base, target)
	if err != nil {
		t.Fatal(err)
	}
	admitted, err := trace.AdmissibleCurrentPaths(candidates, sortedSourcePaths(index.Sources))
	if err != nil {
		t.Fatal(err)
	}
	authority := &dogfoodAuthority{base: base, target: target, candidates: digestPaths(candidates), admitted: digestPaths(admitted)}
	if err := stabilityCheck(context.Background(), root, index, authority)(); err != nil {
		t.Fatalf("correct authority rejected: %v", err)
	}
	authority.candidates = "sha256:" + string(make([]byte, 64))
	if err := stabilityCheck(context.Background(), root, index, authority)(); err == nil {
		t.Fatal("candidate digest drift was accepted")
	}
}

func newAdapterFixture(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	writeAdapterFixture(t, root, ".gitignore", ".context-corvint/\n")
	writeAdapterFixture(t, root, "go.mod", "module example.test/admission\n\ngo 1.27.0\n")
	writeAdapterFixture(t, root, "internal/value.go", "package internal\n")
	gitAdapterFixture(t, root, "init", "-q")
	gitAdapterFixture(t, root, "config", "user.name", "Corvint Test")
	gitAdapterFixture(t, root, "config", "user.email", "corvint@example.test")
	gitAdapterFixture(t, root, "add", ".")
	gitAdapterFixture(t, root, "commit", "-qm", "base")
	return root
}

func writeAdapterFixture(t *testing.T, root, path, data string) {
	t.Helper()
	full := filepath.Join(root, filepath.FromSlash(path))
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(full, []byte(data), 0o644); err != nil {
		t.Fatal(err)
	}
}

func gitAdapterFixture(t *testing.T, root string, arguments ...string) string {
	t.Helper()
	command := exec.Command("git", arguments...)
	command.Dir = root
	output, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v\n%s", arguments, err, output)
	}
	return string(bytesTrimSpace(output))
}

func bytesTrimSpace(value []byte) []byte {
	for len(value) != 0 && (value[len(value)-1] == '\n' || value[len(value)-1] == '\r') {
		value = value[:len(value)-1]
	}
	return value
}

// The closing check for a plain record observes header fields instead of
// compiling a second index. It must still refuse a repository that moved.
func TestPlainRecordStabilityCheckRefusesAMovedRepository(t *testing.T) {
	root := newAdapterFixture(t)
	index, err := contextindex.Build(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
	stable := stabilityCheck(context.Background(), root, index, nil)
	if err := stable(); err != nil {
		t.Fatalf("an unmoved repository was refused: %v", err)
	}
	writeAdapterFixture(t, root, "moved.go", "package fixture\n")
	gitAdapterFixture(t, root, "add", "moved.go")
	gitAdapterFixture(t, root, "commit", "-qm", "move")
	var changed *repositoryChangedError
	if err := stable(); !errors.As(err, &changed) {
		t.Fatalf("a moved repository was accepted: %v", err)
	}
}

// A silent identity failure names the Python reference's rev-parse argv, so
// the matcher must track the argv readIdentity actually spawns.
func TestRecordIndexErrorNamesThePythonIdentityCommand(t *testing.T) {
	root := newAdapterFixture(t)
	realGit, err := exec.LookPath("git")
	if err != nil {
		t.Fatal(err)
	}
	bin := t.TempDir()
	script := "#!/bin/sh\ncase \" $* \" in *\" --show-object-format \"*) exit 128;; esac\nexec " + realGit + " \"$@\"\n"
	if err := os.WriteFile(filepath.Join(bin, "git"), []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
	_, _, err = recordIndex(context.Background(), root)
	want := fmt.Sprintf("Git error: Command '['git', '-C', '%s', 'rev-parse', '--show-object-format', 'HEAD^{tree}']' returned non-zero exit status 128.", root)
	if err == nil || err.Error() != want {
		t.Fatalf("error = %v\nwant %s", err, want)
	}
}

// LTPM-V0-002, GPK-V0-044.
func TestReadBoundsTraceReplayWithoutRefusingLargeRepositories(t *testing.T) {
	previousLimit := replayAncestryLimit
	replayAncestryLimit = 2
	t.Cleanup(func() { replayAncestryLimit = previousLimit })

	newFixture := func(t *testing.T) (string, string) {
		t.Helper()
		root := newAdapterFixture(t)
		oldest := gitAdapterFixture(t, root, "rev-parse", "HEAD")
		for index := 0; index < replayAncestryLimit; index++ {
			writeAdapterFixture(t, root, "internal/value.go", "package internal\n\nconst Value = "+string(rune('0'+index))+"\n")
			gitAdapterFixture(t, root, "add", "internal/value.go")
			gitAdapterFixture(t, root, "commit", "-qm", "advance")
		}
		return root, oldest
	}
	read := func(t *testing.T, root string) (string, error) {
		t.Helper()
		index, err := contextindex.Build(context.Background(), root)
		if err != nil {
			t.Fatal(err)
		}
		_, state, err := Read(context.Background(), root, index)
		return state, err
	}

	t.Run("no trace candidates", func(t *testing.T) {
		root, _ := newFixture(t)
		if state, err := read(t, root); err != nil || state != "absent" {
			t.Fatalf("Read() state=%q error=%v, want absent", state, err)
		}
	})
	t.Run("candidate inside bounded replay", func(t *testing.T) {
		root, _ := newFixture(t)
		head := gitAdapterFixture(t, root, "rev-parse", "HEAD")
		writeAdapterFixture(t, root, filepath.ToSlash(filepath.Join(".context-corvint", "traces", head+".jsonl")), "")
		if state, err := read(t, root); err != nil || state != "ready" {
			t.Fatalf("Read() state=%q error=%v, want ready", state, err)
		}
	})
	t.Run("bounded revisions retain ancestry distance", func(t *testing.T) {
		root, _ := newFixture(t)
		head := gitAdapterFixture(t, root, "rev-parse", "HEAD")
		parent := gitAdapterFixture(t, root, "rev-parse", "HEAD^")
		parentTree := gitAdapterFixture(t, root, "rev-parse", parent+"^{tree}")
		for _, revision := range []string{head, parent, parentTree} {
			writeAdapterFixture(t, root, filepath.ToSlash(filepath.Join(".context-corvint", "traces", revision+".jsonl")), "")
		}
		index, err := contextindex.Build(context.Background(), root)
		if err != nil {
			t.Fatal(err)
		}
		revisions, _, err := bindRevisions(context.Background(), root, index, recordPaths(index), strictRevisionBinding)
		if err != nil {
			t.Fatal(err)
		}
		if got := revisions[head].AncestryDistance; got != 0 {
			t.Fatalf("HEAD ancestry distance=%d, want 0", got)
		}
		if got := revisions[parent].AncestryDistance; got != 1 {
			t.Fatalf("parent ancestry distance=%d, want 1", got)
		}
		if got := revisions[parentTree].AncestryDistance; got != 1 || !revisions[parentTree].Legacy {
			t.Fatalf("legacy tree revision=%+v, want distance 1 and legacy", revisions[parentTree])
		}
	})
	t.Run("candidate outside bounded replay", func(t *testing.T) {
		root, oldest := newFixture(t)
		writeAdapterFixture(t, root, filepath.ToSlash(filepath.Join(".context-corvint", "traces", oldest+".jsonl")), "")
		_, err := read(t, root)
		if kind, ok := ReadFailureOf(err); !ok || kind != ReadFailureReplayWindow {
			t.Fatalf("Read() error=%v kind=%q, want replay-window", err, kind)
		}
		if got := err.Error(); got != "candidate revision predates the bounded replay window (truncated ancestry: 1)" {
			t.Fatalf("Read() error=%q, want bounded-window diagnostic", got)
		}
		if count, ok := TruncatedAncestryOf(err); !ok || count != 1 {
			t.Fatalf("TruncatedAncestryOf() count=%d ok=%t, want 1, true", count, ok)
		}
	})
	t.Run("unreachable candidate remains trace state", func(t *testing.T) {
		root := newAdapterFixture(t)
		unreachable := "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
		writeAdapterFixture(t, root, filepath.ToSlash(filepath.Join(".context-corvint", "traces", unreachable+".jsonl")), "")
		_, err := read(t, root)
		if kind, ok := ReadFailureOf(err); !ok || kind != ReadFailureTraceState {
			t.Fatalf("Read() error=%v kind=%q, want trace-state", err, kind)
		}
		if got := err.Error(); got != "local trace store contains unreachable revision: "+unreachable {
			t.Fatalf("Read() error=%q, want unreachable diagnostic", got)
		}
	})
	t.Run("candidate probe failure is history", func(t *testing.T) {
		root, _ := newFixture(t)
		unavailable := "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
		writeAdapterFixture(t, root, filepath.ToSlash(filepath.Join(".context-corvint", "traces", unavailable+".jsonl")), "")
		_, err := read(t, root)
		if kind, ok := ReadFailureOf(err); !ok || kind != ReadFailureHistory {
			t.Fatalf("Read() error=%v kind=%q, want history", err, kind)
		}
	})
	t.Run("candidate without merge base is unreachable", func(t *testing.T) {
		root, _ := newFixture(t)
		tree := gitAdapterFixture(t, root, "rev-parse", "HEAD^{tree}")
		orphan := gitAdapterFixture(t, root, "commit-tree", tree, "-m", "orphan")
		writeAdapterFixture(t, root, filepath.ToSlash(filepath.Join(".context-corvint", "traces", orphan+".jsonl")), "")
		_, err := read(t, root)
		if kind, ok := ReadFailureOf(err); !ok || kind != ReadFailureTraceState {
			t.Fatalf("Read() error=%v kind=%q, want trace-state", err, kind)
		}
		if got := err.Error(); got != "local trace store contains unreachable revision: "+orphan {
			t.Fatalf("Read() error=%q, want unreachable diagnostic", got)
		}
	})
}

// TestTruncatedAncestryOfSurvivesContextIndexErrorRebuild proves the
// truncation count is no longer disclosed only as error text: it must still
// be recoverable after cmd/corvint's mapRepositoryQueryTraceError rebuilds
// this package's error as a *contextindex.Error, which carries only Code and
// Message plus the original error on its Cause field.
func TestTruncatedAncestryOfSurvivesContextIndexErrorRebuild(t *testing.T) {
	previousLimit := replayAncestryLimit
	replayAncestryLimit = 1
	t.Cleanup(func() { replayAncestryLimit = previousLimit })

	root := newAdapterFixture(t)
	oldest := gitAdapterFixture(t, root, "rev-parse", "HEAD")
	writeAdapterFixture(t, root, "internal/value.go", "package internal\n\nconst Value = 0\n")
	gitAdapterFixture(t, root, "add", "internal/value.go")
	gitAdapterFixture(t, root, "commit", "-qm", "advance")
	writeAdapterFixture(t, root, filepath.ToSlash(filepath.Join(".context-corvint", "traces", oldest+".jsonl")), "")

	index, err := contextindex.Build(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
	_, _, readErr := Read(context.Background(), root, index)
	if _, ok := TruncatedAncestryOf(readErr); !ok {
		t.Fatalf("Read() error=%v, want a truncated-ancestry read failure", readErr)
	}

	rebuilt := &contextindex.Error{Code: "unsupported-query-trace-state", Message: readErr.Error(), Cause: readErr}
	if count, ok := TruncatedAncestryOf(rebuilt); !ok || count != 1 {
		t.Fatalf("TruncatedAncestryOf(rebuilt)=%d,%t, want 1,true", count, ok)
	}
}

// TestRecordResultDisclosesTruncatedAncestryCount covers LTPM-V0-011: the
// count is a member of the successful record receipt, not only recoverable
// from a failure.
func TestRecordResultDisclosesTruncatedAncestryCount(t *testing.T) {
	previousLimit := replayAncestryLimit
	replayAncestryLimit = 1
	t.Cleanup(func() { replayAncestryLimit = previousLimit })

	root := newAdapterFixture(t)
	writeAdapterFixture(t, root, "internal/value.go", "package internal\n\nconst Value = 1\n")
	gitAdapterFixture(t, root, "add", "internal/value.go")
	gitAdapterFixture(t, root, "commit", "-qm", "advance")

	result, err := Record(context.Background(), root, Input{Task: "disclose truncated ancestry", Outcome: "passed"})
	if err != nil {
		t.Fatal(err)
	}
	if result.TruncatedAncestry != 1 {
		t.Fatalf("Record() truncated ancestry=%d, want 1", result.TruncatedAncestry)
	}
}

func TestBindRevisionsUsesShortestAncestryDistanceAcrossMergeParents(t *testing.T) {
	previousLimit := replayAncestryLimit
	replayAncestryLimit = 3
	t.Cleanup(func() { replayAncestryLimit = previousLimit })

	root := newAdapterFixture(t)
	primary := gitAdapterFixture(t, root, "branch", "--show-current")
	gitAdapterFixture(t, root, "branch", "side")
	writeAdapterFixture(t, root, "internal/left.go", "package internal\n")
	gitAdapterFixture(t, root, "add", "internal/left.go")
	gitAdapterFixture(t, root, "commit", "-qm", "left")
	left := gitAdapterFixture(t, root, "rev-parse", "HEAD")
	gitAdapterFixture(t, root, "switch", "-q", "side")
	writeAdapterFixture(t, root, "internal/right.go", "package internal\n")
	gitAdapterFixture(t, root, "add", "internal/right.go")
	gitAdapterFixture(t, root, "commit", "-qm", "right")
	right := gitAdapterFixture(t, root, "rev-parse", "HEAD")
	gitAdapterFixture(t, root, "switch", "-q", primary)
	gitAdapterFixture(t, root, "merge", "-q", "--no-ff", "-m", "merge", "side")
	for _, revision := range []string{left, right} {
		writeAdapterFixture(t, root, filepath.ToSlash(filepath.Join(".context-corvint", "traces", revision+".jsonl")), "")
	}
	index, err := contextindex.Build(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
	revisions, _, err := bindRevisions(context.Background(), root, index, recordPaths(index), strictRevisionBinding)
	if err != nil {
		t.Fatal(err)
	}
	if revisions[left].AncestryDistance != 1 || revisions[right].AncestryDistance != 1 {
		t.Fatalf("merge-parent distances: left=%d right=%d, want both 1",
			revisions[left].AncestryDistance, revisions[right].AncestryDistance)
	}
}

// LTA-V0-003.
func TestRecordReclaimsAtCapUnreachableFileWhileReadRemainsStrict(t *testing.T) {
	root := newAdapterFixture(t)
	unreachable := strings.Repeat("a", 40)
	unreachablePath := filepath.ToSlash(filepath.Join(".context-corvint", "traces", unreachable+".jsonl"))
	var rows strings.Builder
	for index := 0; index < trace.MaxTraces; index++ {
		record, err := trace.NewRecord(trace.Input{
			Revision: unreachable, Task: fmt.Sprintf("stale trace %d", index), Outcome: "passed",
		}, nil)
		if err != nil {
			t.Fatal(err)
		}
		row, err := trace.Encode(record)
		if err != nil {
			t.Fatal(err)
		}
		rows.Write(row)
	}
	writeAdapterFixture(t, root, unreachablePath, rows.String())

	index, err := contextindex.Build(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := Read(context.Background(), root, index); err == nil {
		t.Fatal("Read() accepted an unreachable trace revision")
	}

	result, err := Record(context.Background(), root, Input{Task: "reclaim stale trace", Outcome: "passed"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(root, filepath.FromSlash(unreachablePath))); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("unreachable trace was not evicted: %v", err)
	}
	if result.Store != trace.StorePath(root, result.Record.Revision) {
		t.Fatalf("Record() store=%q revision=%q", result.Store, result.Record.Revision)
	}
	if records, state, err := Read(context.Background(), root, index); err != nil || state != "ready" || len(records) != 1 {
		t.Fatalf("Read() after reclaim = (%+v, %q, %v)", records, state, err)
	}
}

// LTA-V0-003.
func TestRecordRecoversInterruptedRetentionBeforeCandidateDiscovery(t *testing.T) {
	for _, published := range []bool{false, true} {
		t.Run(fmt.Sprintf("published=%t", published), func(t *testing.T) {
			root := newAdapterFixture(t)
			candidate := gitAdapterFixture(t, root, "rev-parse", "HEAD")
			writeAdapterFixture(t, root, "internal/value.go", "package internal\n\nconst Value = 1\n")
			gitAdapterFixture(t, root, "add", "internal/value.go")
			gitAdapterFixture(t, root, "commit", "-qm", "target")
			target := gitAdapterFixture(t, root, "rev-parse", "HEAD")
			old, err := trace.NewRecord(trace.Input{Revision: target, Task: "old target row", Outcome: "passed"}, nil)
			if err != nil {
				t.Fatal(err)
			}
			oldRow, err := trace.Encode(old)
			if err != nil {
				t.Fatal(err)
			}
			interrupted, err := trace.NewRecord(trace.Input{Revision: target, Task: "interrupted target row", Outcome: "passed"}, nil)
			if err != nil {
				t.Fatal(err)
			}
			interruptedRow, err := trace.Encode(interrupted)
			if err != nil {
				t.Fatal(err)
			}
			candidateRecord, err := trace.NewRecord(trace.Input{Revision: candidate, Task: "staged candidate", Outcome: "failed"}, nil)
			if err != nil {
				t.Fatal(err)
			}
			candidateRow, err := trace.Encode(candidateRecord)
			if err != nil {
				t.Fatal(err)
			}
			targetRows := append([]byte(nil), oldRow...)
			if published {
				targetRows = append(targetRows, interruptedRow...)
			}
			writeAdapterFixture(t, root, filepath.ToSlash(filepath.Join(".context-corvint", "traces", target+".jsonl")), string(targetRows))
			hidden := ".evict-" + target + "-" + interrupted.TraceID + "-" + candidate + ".tmp"
			writeAdapterFixture(t, root, filepath.ToSlash(filepath.Join(".context-corvint", "traces", hidden)), string(candidateRow))
			if !published {
				stagedTarget := append(append([]byte(nil), oldRow...), interruptedRow...)
				writeAdapterFixture(t, root, filepath.ToSlash(filepath.Join(".context-corvint", "traces", ".record-"+target+".tmp")), string(stagedTarget))
			}
			index, err := contextindex.Build(context.Background(), root)
			if err != nil {
				t.Fatal(err)
			}
			if _, _, err := Read(context.Background(), root, index); err == nil {
				t.Fatal("Read() accepted interrupted retention residue")
			}
			if _, err := Record(context.Background(), root, Input{Task: " ", Outcome: "passed"}); err == nil {
				t.Fatal("Record() accepted an invalid task")
			}
			oversizedPaths := make([]string, trace.MaxTracePaths)
			for pathIndex := range oversizedPaths {
				oversizedPaths[pathIndex] = fmt.Sprintf("internal/%03d-%s.go", pathIndex, strings.Repeat("x", 1_500))
			}
			if _, err := recordWithIndex(context.Background(), root, index, oversizedPaths, func() error { return nil }, Input{
				Task: "oversized encoded row", OpenedPaths: oversizedPaths, Outcome: "passed",
			}); err == nil || !strings.Contains(err.Error(), "row exceeds") {
				t.Fatalf("recordWithIndex() oversized error=%v, want row bound", err)
			}
			if got, err := os.ReadFile(filepath.Join(root, ".context-corvint", "traces", hidden)); err != nil || !bytes.Equal(got, candidateRow) {
				t.Fatalf("invalid record mutated staged eviction: error=%v got=%q", err, got)
			}
			if got, err := os.ReadFile(trace.StorePath(root, target)); err != nil || !bytes.Equal(got, targetRows) {
				t.Fatalf("invalid record mutated target: error=%v got=%q", err, got)
			}
			if !published {
				stagedTarget := append(append([]byte(nil), oldRow...), interruptedRow...)
				got, err := os.ReadFile(filepath.Join(root, ".context-corvint", "traces", ".record-"+target+".tmp"))
				if err != nil || !bytes.Equal(got, stagedTarget) {
					t.Fatalf("invalid record mutated staged target: error=%v got=%q", err, got)
				}
			}
			result, err := Record(context.Background(), root, Input{Task: "record after recovery", Outcome: "passed"})
			if err != nil {
				t.Fatal(err)
			}
			if _, err := os.Stat(filepath.Join(root, ".context-corvint", "traces", hidden)); !errors.Is(err, os.ErrNotExist) {
				t.Fatalf("record did not clear staged eviction: %v", err)
			}
			_, candidateErr := os.Stat(trace.StorePath(root, candidate))
			if published && !errors.Is(candidateErr, os.ErrNotExist) {
				t.Fatalf("published recovery restored candidate: %v", candidateErr)
			}
			if !published {
				got, err := os.ReadFile(trace.StorePath(root, candidate))
				if err != nil || !bytes.Equal(got, candidateRow) {
					t.Fatalf("unpublished recovery did not restore candidate: error=%v", err)
				}
			}
			records, state, err := Read(context.Background(), root, index)
			if err != nil || state != "ready" || len(records) != 3 {
				t.Fatalf("Read() after record recovery = (%d records, %q, %v)", len(records), state, err)
			}
			if result.Record.Revision != target {
				t.Fatalf("record revision=%q want %q", result.Record.Revision, target)
			}
			entries, err := os.ReadDir(filepath.Join(root, ".context-corvint", "traces"))
			if err != nil || len(entries) > trace.MaxTraceFiles {
				t.Fatalf("final directory entries=%d error=%v", len(entries), err)
			}
		})
	}
}

// LTA-V0-003.
func TestRecordReclaimsAfterExactCapRecoveryLeavesOperationLockOverage(t *testing.T) {
	root := newAdapterFixture(t)
	target := gitAdapterFixture(t, root, "rev-parse", "HEAD")
	writeAdapterFixture(t, root, filepath.ToSlash(filepath.Join(".context-corvint", "traces", target+".jsonl")), "")
	written := 1
	for value := 1; written < trace.MaxTraceFiles; value++ {
		revision := fmt.Sprintf("%040x", value)
		if revision == target {
			continue
		}
		writeAdapterFixture(t, root, filepath.ToSlash(filepath.Join(".context-corvint", "traces", revision+".jsonl")), "")
		written++
	}
	writeAdapterFixture(t, root, ".context-corvint/traces/.trace-operation.lock", "")
	if _, err := trace.CandidateRevisions(root); err == nil || !strings.Contains(err.Error(), "exceeds 1000") {
		t.Fatalf("strict CandidateRevisions() error=%v, want exact-cap refusal", err)
	}

	result, err := Record(context.Background(), root, Input{Task: "record after exact-cap recovery", Outcome: "passed"})
	if err != nil {
		t.Fatal(err)
	}
	if result.Record.Revision != target {
		t.Fatalf("record revision=%q want %q", result.Record.Revision, target)
	}
	entries, err := os.ReadDir(filepath.Join(root, ".context-corvint", "traces"))
	if err != nil || len(entries) != trace.MaxTraceFiles {
		t.Fatalf("final directory entries=%d error=%v, want %d", len(entries), err, trace.MaxTraceFiles)
	}
}

func TestRecordPreservesStoreErrorPrecedenceWithoutRecoveryResidue(t *testing.T) {
	root := newAdapterFixture(t)
	writeAdapterFixture(t, root, ".context-corvint/traces/not-a-revision.jsonl", "")
	_, err := Record(context.Background(), root, Input{Task: " ", Outcome: "passed"})
	if err == nil || !strings.Contains(err.Error(), "invalid local trace revision filename") {
		t.Fatalf("Record() error=%v, want store validation before task validation", err)
	}
}

// GPK-V0-061: the untracked allowance that range impact applies does not
// reach learned-trace reads; a disjoint untracked path still blocks them.
func TestReadStaysBlockedForDisjointUntrackedPath(t *testing.T) {
	root := newAdapterFixture(t)
	writeAdapterFixture(t, root, "scratch/notes.md", "operator notes\n")
	index, err := contextindex.Build(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
	records, state, err := Read(context.Background(), root, index)
	if err != nil || state != "blocked-mixed-worktree" || len(records) != 0 {
		t.Fatalf("Read() records=%d state=%q error=%v, want blocked-mixed-worktree", len(records), state, err)
	}
}
