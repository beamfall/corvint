package contextindex

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"testing"
)

const sharedQueryTask = "fix the demux key split in the cache reader"

// sharedQueryFixture builds the authority fixture, writes its snapshot, and
// loads it back the way the query path does, returning the loader's opening.
func sharedQueryFixture(t *testing.T) (*Index, *LoaderObservation) {
	t.Helper()
	root := authorityRepository(t)
	index, err := Build(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := WriteSnapshot(index); err != nil {
		t.Fatal(err)
	}
	loaded, hit, opening, err := LoadQuerySnapshot(context.Background(), root)
	if err != nil || !hit || opening == nil {
		t.Fatalf("LoadQuerySnapshot hit=%v opening=%v err=%v", hit, opening, err)
	}
	return loaded, opening
}

// installDriftingGitWrapper puts a `git` on PATH that tallies status (s),
// rev-parse (r) and log (l) calls into the returned path and, once `git log`
// has run, reports `tree` in place of the index revision and, when
// statusAfter is set, a modified AGENTS.md.
func installDriftingGitWrapper(t *testing.T, index *Index, tree string, statusAfter bool) string {
	t.Helper()
	realGit, err := exec.LookPath("git")
	if err != nil {
		t.Fatal(err)
	}
	bin, state := t.TempDir(), t.TempDir()
	calls, mark := filepath.Join(state, "calls"), filepath.Join(state, "logged")
	statusOutput := ""
	if statusAfter {
		statusOutput = " M AGENTS.md\\000"
	}
	wrapper := `#!/bin/sh
kind=
for argument in "$@"; do
  case "$argument" in
    status|rev-parse|log) kind="$argument" ;;
  esac
done
case "$kind" in
  status) printf s >> ` + strconv.Quote(calls) + ` ;;
  rev-parse) printf r >> ` + strconv.Quote(calls) + ` ;;
  log) printf l >> ` + strconv.Quote(calls) + ` ; : > ` + strconv.Quote(mark) + ` ;;
esac
if [ -f ` + strconv.Quote(mark) + ` ] && [ "$kind" = rev-parse ]; then
  ` + strconv.Quote(realGit) + ` "$@" | sed s/` + index.Revision + `/` + tree + `/
  exit 0
fi
if [ -f ` + strconv.Quote(mark) + ` ] && [ "$kind" = status ] && [ -n ` + strconv.Quote(statusOutput) + ` ]; then
  printf ` + strconv.Quote(statusOutput) + `
  exit 0
fi
exec ` + strconv.Quote(realGit) + ` "$@"
`
	if err := os.WriteFile(filepath.Join(bin, "git"), []byte(wrapper), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
	return calls
}

func readCalls(t *testing.T, calls string) string {
	t.Helper()
	raw, err := os.ReadFile(calls)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(calls); err != nil {
		t.Fatal(err)
	}
	return string(raw)
}

// TestEvalQuerySharedMatchesEvalQueryWithFewerGitProcesses pins the proposed
// shared query bracket: on a snapshot hit the shared receipt is byte-identical
// to the concurrent path's, the learn stage issues no opening pair of its own,
// and its closing is one identity read beside one status scan.
func TestEvalQuerySharedMatchesEvalQueryWithFewerGitProcesses(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("test wrapper uses a POSIX shell")
	}
	index, opening := sharedQueryFixture(t)
	snapshot, err := NewQueryTraceSnapshot("absent", nil)
	if err != nil {
		t.Fatal(err)
	}
	calls := installDriftingGitWrapper(t, index, index.Revision, false)
	concurrent, err := EvalQuery(context.Background(), index, sharedQueryTask, 10, nil, snapshot)
	if err != nil {
		t.Fatal(err)
	}
	concurrentCalls := readCalls(t, calls)
	want, err := CanonicalJSON(concurrent)
	if err != nil {
		t.Fatal(err)
	}
	for _, traceClosing := range []bool{false, true} {
		shared, err := EvalQueryShared(context.Background(), index, sharedQueryTask, 10, nil, snapshot, opening, traceClosing)
		if err != nil {
			t.Fatalf("traceClosing=%v: %v", traceClosing, err)
		}
		got, err := CanonicalJSON(shared)
		if err != nil {
			t.Fatal(err)
		}
		if string(got) != string(want) {
			t.Fatalf("traceClosing=%v: shared receipt differs\nshared:     %s\nconcurrent: %s", traceClosing, got, want)
		}
		if sharedCalls := readCalls(t, calls); sharedCalls != "lsr" && sharedCalls != "lrs" {
			t.Fatalf("shared Git calls=%q (concurrent %q) want one log, one status, one rev-parse", sharedCalls, concurrentCalls)
		}
	}
	if strings.Count(concurrentCalls, "s") != 2 || strings.Count(concurrentCalls, "r") != 2 || strings.Count(concurrentCalls, "l") != 1 {
		t.Fatalf("concurrent Git calls=%q want two status, two rev-parse, one log", concurrentCalls)
	}
}

// TestEvalQuerySharedRefusesDriftInsideTheWindow pins the refusal a repository
// change inside the shared window earns: the trace read's deferred comparison
// speaks first when it is owed, and the learn stage's own words otherwise.
func TestEvalQuerySharedRefusesDriftInsideTheWindow(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("test wrapper uses a POSIX shell")
	}
	index, opening := sharedQueryFixture(t)
	snapshot, err := NewQueryTraceSnapshot("absent", nil)
	if err != nil {
		t.Fatal(err)
	}
	alternate := "0" + index.Revision[1:]
	if index.Revision[0] == '0' {
		alternate = "1" + index.Revision[1:]
	}
	for _, test := range []struct {
		name, tree, want string
		statusAfter      bool
		traceClosing     bool
	}{
		{"tree drift owed to the trace read", alternate, "repository changed while reading local traces: repository identity changed", false, true},
		{"tree drift after the trace read", alternate, "repository revision changed during history learning", false, false},
		{"status drift owed to the trace read", index.Revision, "repository changed while reading local traces: repository identity changed", true, true},
		{"status drift after the trace read", index.Revision, "repository worktree changed during history learning", true, false},
	} {
		t.Run(test.name, func(t *testing.T) {
			installDriftingGitWrapper(t, index, test.tree, test.statusAfter)
			_, err := EvalQueryShared(context.Background(), index, sharedQueryTask, 10, nil, snapshot, opening, test.traceClosing)
			if err == nil || err.Error() != test.want {
				t.Fatalf("error=%v want %q", err, test.want)
			}
			if code := err.(*Error).Code; code != "unsupported-query-drift" {
				t.Fatalf("code=%q", code)
			}
		})
	}
}

func TestEvalQuerySharedRequiresTheLoaderObservation(t *testing.T) {
	index, _ := sharedQueryFixture(t)
	snapshot, err := NewQueryTraceSnapshot("absent", nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := EvalQueryShared(context.Background(), index, sharedQueryTask, 10, nil, snapshot, nil, false); err == nil {
		t.Fatal("EvalQueryShared accepted a nil opening")
	}
	if _, err := EvalQueryShared(context.Background(), index, sharedQueryTask, 10, nil, QueryTraceSnapshot{}, nil, false); err == nil {
		t.Fatal("EvalQueryShared accepted an unconstructed trace snapshot")
	}
}
