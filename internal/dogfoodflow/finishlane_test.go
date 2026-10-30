package dogfoodflow

import (
	"bytes"
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// DCW-V0-061: only a one-parent commit titled as the bind commit that changes
// nothing but the sidecar resumes at rebind; any other commit is the change.
func TestFinishLaneDCWV0061RecognizesOnlyTheBindCommit(t *testing.T) {
	root := t.TempDir()
	write := func(path, data string) {
		t.Helper()
		if err := os.MkdirAll(filepath.Dir(filepath.Join(root, path)), 0o777); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(root, path), []byte(data), 0o666); err != nil {
			t.Fatal(err)
		}
	}
	commit := func(message string) string {
		t.Helper()
		testGit(t, root, "add", "-A")
		testGit(t, root, "commit", "-q", "-m", message)
		return testGit(t, root, "rev-parse", "HEAD")
	}
	testGit(t, root, "init", "-q")
	write("a.txt", "a\n")
	commit("base")
	write("a.txt", "b\n")
	change := commit(bindSubject)
	write(finishLaneSidecar, "{}\n")
	bind := commit(bindSubject)
	write(finishLaneSidecar, "{\"x\":1}\n")
	write("a.txt", "c\n")
	mixed := commit(bindSubject)
	write(finishLaneSidecar, "{\"x\":2}\n")
	retitled := commit("bind")

	var stderr bytes.Buffer
	lane := &finishLane{flow: flow{ctx: context.Background(), prefix: "dogfood-finish-lane", root: root, stderr: &stderr}}
	for commit, want := range map[string]bool{change: false, bind: true, mixed: false, retitled: false} {
		if got := lane.isBindCommit(commit); got != want {
			t.Fatalf("isBindCommit(%s) = %v, want %v", commit, got, want)
		}
	}

	// Before the bind commit, a stopped run's sidecar is the one path that may
	// differ; at the bind commit nothing may.
	clean := func(resume bool) (int, string) {
		stderr.Reset()
		lane.base = change
		lane.target = bind
		code, err := boundary(context.Background(), func() int { lane.requireCleanWorktree(resume, false); return 0 }, func() {})
		if err != nil {
			t.Fatal(err)
		}
		return code, stderr.String()
	}
	write(finishLaneSidecar, "{\"x\":3}\n")
	if code, out := clean(false); code != 0 || out != "" {
		t.Fatalf("sidecar-only exit=%d stderr=%q", code, out)
	}
	if code, out := clean(true); code != 2 || out != "dogfood-finish-lane: REFUSE dirty-worktree\n  fix: git restore --source=HEAD --staged --worktree -- .corvint/change.cem.json .corvint/changes/"+bind+".cem.json, commit or remove every other change, then rerun corvint dogfood finish-lane --base "+change+"\n" {
		t.Fatalf("resume exit=%d stderr=%q", code, out)
	}
	write("notes.txt", "draft\n")
	if code, out := clean(false); code != 2 || !strings.HasPrefix(out, "dogfood-finish-lane: REFUSE dirty-worktree\n  fix: commit the change ") {
		t.Fatalf("dirty exit=%d stderr=%q", code, out)
	}
}

// DCW-V0-067 (V1-1397): a finish-lane run whose context ends during a pass
// names the interrupted step and the reasons kept so far instead of exiting
// with no output.
func TestFinishLaneDCWV0067NamesTheInterruptedStep(t *testing.T) {
	root := t.TempDir()
	testGit(t, root, "init", "-q")
	if err := os.WriteFile(filepath.Join(root, "a.txt"), []byte("a\n"), 0o666); err != nil {
		t.Fatal(err)
	}
	testGit(t, root, "add", "a.txt")
	testGit(t, root, "commit", "-q", "-m", "base")
	base := testGit(t, root, "rev-parse", "HEAD")
	if err := os.WriteFile(filepath.Join(root, "a.txt"), []byte("b\n"), 0o666); err != nil {
		t.Fatal(err)
	}
	testGit(t, root, "commit", "-q", "-am", "change")
	plan := filepath.Join(t.TempDir(), "cites.tsv")
	if err := os.WriteFile(plan, []byte("1\ta.txt\t1:1\tspecification\n"), 0o666); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	// The first step command the prepare pass runs ends the context, as a
	// SIGINT or SIGTERM delivered to the CLI does.
	steps := Runner{Path: "corvint", Run: func(context.Context, string, []string, io.Writer, io.Writer) int {
		cancel()
		return 1
	}}
	var stdout, stderr bytes.Buffer
	_, err := FinishLane(ctx, FinishLaneOptions{Change: ChangeOptions{Root: root, Base: base, Steps: steps, Citations: plan}}, &stdout, &stderr)
	if !errors.Is(err, ErrInterrupted) {
		t.Fatalf("FinishLane error = %v, want ErrInterrupted", err)
	}
	if want := "dogfood-finish-lane: prepare INTERRUPTED\ndogfood-finish-lane: NOT_PRODUCED rows=0\n"; stdout.String() != want {
		t.Fatalf("stdout = %q, want %q (stderr %q)", stdout.String(), want, stderr.String())
	}
	// DCW-V0-116: the interruption names the exact rerun that resumes it, and
	// DCW-V0-115: the step's timing and the run's total are still written.
	if want := "dogfood-finish-lane: INTERRUPTED prepare\n  fix: rerun corvint dogfood finish-lane --base " + base + " (or make dogfood-finish BASE=" + base + ") with the same inputs; it prepares the map again"; !strings.Contains(stderr.String(), want) {
		t.Fatalf("stderr lacks %q:\n%s", want, stderr.String())
	}
	if !strings.Contains(stderr.String(), "dogfood-finish-lane: TIMING prepare seconds=") || !strings.Contains(stderr.String(), "dogfood-finish-lane: TIMING total seconds=") {
		t.Fatalf("stderr lacks the TIMING lines:\n%s", stderr.String())
	}
	// A row the pass recorded before the interruption stays in the block.
	ctx, cancel = context.WithCancel(context.Background())
	defer cancel()
	calls := 0
	steps.Run = func(context.Context, string, []string, io.Writer, io.Writer) int {
		calls++
		if calls > 1 {
			cancel()
		}
		return 1
	}
	stdout.Reset()
	stderr.Reset()
	if _, err = FinishLane(ctx, FinishLaneOptions{Change: ChangeOptions{Root: root, Base: base, Steps: steps, Citations: plan}}, &stdout, &stderr); !errors.Is(err, ErrInterrupted) {
		t.Fatalf("second FinishLane error = %v, want ErrInterrupted", err)
	}
	if out := stdout.String(); !strings.HasPrefix(out, "dogfood-finish-lane: prepare INTERRUPTED\ndogfood-finish-lane: NOT_PRODUCED rows=") || strings.Contains(out, "rows=0") || !strings.Contains(out, "\n  prepare ") {
		t.Fatalf("interrupted run lost the recorded rows: %q (stderr %q)", out, stderr.String())
	}
}

// DCW-V0-067: a seal interrupted after its check passed is named seal and keeps
// the check's verdict lines; one interrupted during the check is named check.
func TestFinishLaneDCWV0067NamesAnInterruptedSeal(t *testing.T) {
	for _, tc := range []struct{ verdict, step string }{
		{"dogfood-check: SUMMARY hunks=1\ndogfood-check: PASS\n", "seal"},
		{"dogfood-check: SUMMARY hunks=1\n", "check"},
	} {
		var stdout, stderr bytes.Buffer
		lane := &finishLane{flow: flow{ctx: context.Background(), prefix: "dogfood-finish-lane", stderr: &stderr, base: "b0", target: "t0"}, stdout: &stdout, step: "check"}
		if _, err := boundary(context.Background(), func() int { lane.sealInterrupted([]byte(tc.verdict)); return 0 }, func() {}); !errors.Is(err, ErrInterrupted) {
			t.Fatalf("sealInterrupted error = %v, want ErrInterrupted", err)
		}
		lane.interrupted()
		if want := tc.verdict + "dogfood-finish-lane: " + tc.step + " INTERRUPTED\ndogfood-finish-lane: NOT_PRODUCED rows=0\n"; stdout.String() != want {
			t.Errorf("stdout = %q, want %q", stdout.String(), want)
		}
		// DCW-V0-116: the fix restores the bind commit's sidecar, and a stopped
		// seal's staged rename, before the rerun resumes from the bind commit.
		restore := "git restore --source=HEAD --staged --worktree -- " + finishLaneSidecar
		if tc.step == "seal" {
			restore += " .corvint/changes/t0.cem.json"
		}
		if !strings.Contains(stderr.String(), "\n  fix: ") || !strings.Contains(stderr.String(), restore) || !strings.Contains(stderr.String(), "rerun corvint dogfood finish-lane --base b0 ") || !strings.Contains(stderr.String(), "resumes from the bind commit t0") {
			t.Errorf("%s stderr = %q, want the restore %q and the rerun", tc.step, stderr.String(), restore)
		}
	}
}

// DCW-V0-068 (V1-1397): a sidecar removed while the bind pass cites it stops
// finish-lane at bind with the map-unavailable row and a fix line naming the
// map path, not only the CEM command's fixed "cannot read CEM map".
func TestFinishLaneDCWV0068NamesTheUnreadableMap(t *testing.T) {
	root := t.TempDir()
	testGit(t, root, "init", "-q")
	if err := os.WriteFile(filepath.Join(root, "a.txt"), []byte("a\n"), 0o666); err != nil {
		t.Fatal(err)
	}
	testGit(t, root, "add", "a.txt")
	testGit(t, root, "commit", "-q", "-m", "base")
	base := testGit(t, root, "rev-parse", "HEAD")
	if err := os.WriteFile(filepath.Join(root, "a.txt"), []byte("b\n"), 0o666); err != nil {
		t.Fatal(err)
	}
	testGit(t, root, "commit", "-q", "-am", "change")
	plan := filepath.Join(t.TempDir(), "cites.tsv")
	if err := os.WriteFile(plan, []byte("1\ta.txt\t1:1\tspecification\n"), 0o666); err != nil {
		t.Fatal(err)
	}
	sidecar := filepath.Join(root, finishLaneSidecar)
	steps := Runner{Path: "corvint", Run: func(_ context.Context, _ string, args []string, _, stderr io.Writer) int {
		switch {
		case len(args) > 1 && args[0] == "cem" && args[1] == "prepare":
			if err := os.MkdirAll(filepath.Dir(sidecar), 0o777); err != nil {
				return 1
			}
			if err := os.WriteFile(sidecar, []byte("{\n  \"hunks\": [\n    {\n      \"disposition\": \"unknown\",\n      \"id\": \"hunk:a\",\n      \"path\": \"a.txt\"\n    }\n  ]\n}\n"), 0o666); err != nil {
				return 1
			}
			return 0
		case len(args) > 1 && args[0] == "cem" && args[1] == "cite" && strings.Contains(strings.Join(args, " "), " --output .corvint/.cem-citations-probe."):
			// The prepare pass's plan precheck (DCW-V0-335) admits the row.
			return 0
		case len(args) > 1 && args[0] == "cem" && args[1] == "cite":
			// The operator removes the sidecar while the pass runs.
			if err := os.Remove(sidecar); err != nil {
				return 1
			}
			_, _ = io.WriteString(stderr, `{"code": "map-unavailable", "error": "cannot read CEM map", "ok": false}`+"\n")
			return 2
		}
		return 1
	}}
	var stdout, stderr bytes.Buffer
	code, err := FinishLane(context.Background(), FinishLaneOptions{Change: ChangeOptions{Root: root, Base: base, Steps: steps, Citations: plan}}, &stdout, &stderr)
	if err != nil || code == 0 {
		t.Fatalf("FinishLane = (%d, %v), want a failing exit", code, err)
	}
	if !strings.Contains(stdout.String(), "dogfood-finish-lane: prepare PASS hunks=1\n") || !strings.Contains(stdout.String(), "dogfood-finish-lane: bind FAIL exit=") ||
		!strings.Contains(stdout.String(), "  bind cem-cite NOT_PRODUCED map-unavailable\n") {
		t.Fatalf("stdout does not name the bind step and code:\n%s", stdout.String())
	}
	if want := "  cem-cite: map-unavailable\n    fix: the CEM map " + root + "/" + finishLaneSidecar + " could not be read ("; !strings.Contains(stderr.String(), want) {
		t.Fatalf("stderr lacks %q:\n%s", want, stderr.String())
	}
}

// DCW-V0-068: when a later citation row fails, its cite command read the staged
// citation map, so the fix line names that map, not the sidecar.
func TestChangeDCWV0068NamesTheStagedMapOfALaterRow(t *testing.T) {
	root := t.TempDir()
	testGit(t, root, "init", "-q")
	for _, name := range []string{"a.txt", "b.txt"} {
		if err := os.WriteFile(filepath.Join(root, name), []byte("a\n"), 0o666); err != nil {
			t.Fatal(err)
		}
	}
	testGit(t, root, "add", "a.txt", "b.txt")
	testGit(t, root, "commit", "-q", "-m", "base")
	base := testGit(t, root, "rev-parse", "HEAD")
	for _, name := range []string{"a.txt", "b.txt"} {
		if err := os.WriteFile(filepath.Join(root, name), []byte("b\n"), 0o666); err != nil {
			t.Fatal(err)
		}
	}
	testGit(t, root, "commit", "-q", "-am", "change")
	plan := filepath.Join(t.TempDir(), "cites.tsv")
	if err := os.WriteFile(plan, []byte("1\ta.txt\t1:1\tspecification\n2\tb.txt\t1:1\tspecification\n"), 0o666); err != nil {
		t.Fatal(err)
	}
	sidecar := filepath.Join(root, finishLaneSidecar)
	var citedMaps []string
	steps := Runner{Path: "corvint", Run: func(_ context.Context, _ string, args []string, _, stderr io.Writer) int {
		switch {
		case len(args) > 1 && args[0] == "cem" && args[1] == "prepare":
			if err := os.MkdirAll(filepath.Dir(sidecar), 0o777); err != nil {
				return 1
			}
			hunk := "    {\n      \"disposition\": \"unknown\",\n      \"id\": \"hunk:%s\",\n      \"path\": \"%s.txt\"\n    }"
			if err := os.WriteFile(sidecar, []byte("{\n  \"hunks\": [\n"+fmt.Sprintf(hunk, "a", "a")+",\n"+fmt.Sprintf(hunk, "b", "b")+"\n  ]\n}\n"), 0o666); err != nil {
				return 1
			}
			return 0
		case len(args) > 3 && args[0] == "cem" && args[1] == "cite":
			citedMaps = append(citedMaps, args[3])
			if len(citedMaps) == 1 {
				return 0
			}
			_, _ = io.WriteString(stderr, `{"code": "map-unavailable", "error": "cannot read CEM map", "ok": false}`+"\n")
			return 2
		}
		return 1
	}}
	var stderr bytes.Buffer
	if _, err := Change(context.Background(), ChangeOptions{Root: root, Base: base, Steps: steps, Citations: plan}, &stderr); err != nil {
		t.Fatal(err)
	}
	if len(citedMaps) != 2 || citedMaps[1] == finishLaneSidecar {
		t.Fatalf("cite maps = %q, want the second row to read the staged map", citedMaps)
	}
	if want := "  cem-cite: map-unavailable\n    fix: the CEM map " + root + "/" + citedMaps[1] + " could not be read ("; !strings.Contains(stderr.String(), want) {
		t.Fatalf("stderr lacks %q:\n%s", want, stderr.String())
	}
}

// DCW-V0-092 (V1-1657): finish-lane refuses a verify input the bind's
// local-outcome row would refuse before prepare runs any step, with the same
// reason code; a quoted -run regex line is the common case.
func TestFinishLaneDCWV0092RefusesVerifySyntaxBeforePrepare(t *testing.T) {
	root := t.TempDir()
	testGit(t, root, "init", "-q")
	if err := os.WriteFile(filepath.Join(root, "a.txt"), []byte("a\n"), 0o666); err != nil {
		t.Fatal(err)
	}
	testGit(t, root, "add", "a.txt")
	testGit(t, root, "commit", "-q", "-m", "base")
	base := testGit(t, root, "rev-parse", "HEAD")
	if err := os.WriteFile(filepath.Join(root, "a.txt"), []byte("b\n"), 0o666); err != nil {
		t.Fatal(err)
	}
	testGit(t, root, "commit", "-q", "-am", "change")
	scratch := t.TempDir()
	write := func(name string, data []byte) string {
		t.Helper()
		path := filepath.Join(scratch, name)
		if err := os.WriteFile(path, data, 0o666); err != nil {
			t.Fatal(err)
		}
		return path
	}
	plan := write("cites.tsv", []byte("1\ta.txt\t1:1\tspecification\n"))
	for _, tc := range []struct {
		name, verifyFile, verify, reason string
	}{
		{name: "quoted-run-regex", verifyFile: write("quoted.txt", []byte("go test -count=1 -run '^TestA$' ./internal/a\n")), reason: "unsupported-verify-syntax"},
		{name: "env-quoted", verify: "go test -run \"TestA\" ./internal/a", reason: "unsupported-verify-syntax"},
		{name: "quoted-run-anchored-alternation", verifyFile: write("alternation.txt", []byte("go test -count=1 -run '^(TestA|TestB)$' ./internal/a\n")), reason: "unsupported-verify-syntax"},
		{name: "over-bound", verifyFile: write("big.txt", bytes.Repeat([]byte("\n"), maxVerifyFileBytes+1)), reason: "verify-file-over-bound"},
		{name: "unavailable", verifyFile: filepath.Join(scratch, "missing.txt"), reason: "verify-file-unavailable"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			calls := 0
			steps := Runner{Path: "corvint", Run: func(context.Context, string, []string, io.Writer, io.Writer) int {
				calls++
				return 1
			}}
			var stdout, stderr bytes.Buffer
			code, err := FinishLane(context.Background(), FinishLaneOptions{Change: ChangeOptions{Root: root, Base: base, Steps: steps, Citations: plan, VerifyFile: tc.verifyFile, Verify: tc.verify, Outcome: "passed"}}, &stdout, &stderr)
			if err != nil {
				t.Fatal(err)
			}
			if code != 2 || calls != 0 || stdout.Len() != 0 {
				t.Fatalf("exit=%d step calls=%d stdout=%q stderr=%q, want exit 2 before any step", code, calls, stdout.String(), stderr.String())
			}
			out := stderr.String()
			if !strings.HasPrefix(out, "dogfood-finish-lane: REFUSE "+tc.reason+"\n") || !strings.Contains(out, "  fix: pass a readable verify file of at most 65536 bytes") {
				t.Fatalf("stderr = %q, want REFUSE %s with a fix line", out, tc.reason)
			}
			if tc.reason == "unsupported-verify-syntax" && !strings.Contains(out, "outside the admitted set") {
				t.Fatalf("stderr = %q, want the refused byte named", out)
			}
			// DCW-V0-170 (V1-2344): the refusal names the accepted form for a
			// quoted -run alternation: one command per test name, or a script.
			if tc.reason == "unsupported-verify-syntax" {
				for _, want := range []string{"accepted form:", "-run 'TestA|TestB' is admitted as one argv word (DCW-V0-384)", "-run '^TestA$', is refused even inside quotes", "go test -run TestA ./pkg, then go test -run TestB ./pkg", "sh path/to/script.sh"} {
					if !strings.Contains(out, want) {
						t.Fatalf("stderr = %q, want accepted-form hint %q", out, want)
					}
				}
			}
		})
	}
	// A shell-free verify file, and one quoted -run alternation of test names
	// (DCW-V0-384), pass the early check and reach prepare.
	calls := 0
	steps := Runner{Path: "corvint", Run: func(context.Context, string, []string, io.Writer, io.Writer) int {
		calls++
		return 1
	}}
	var stdout, stderr bytes.Buffer
	good := write("good.txt", []byte("go test -count=1 -run TestA ./internal/a\ngo test -count=1 -run 'TestA|TestB' ./internal/a\n"))
	if _, err := FinishLane(context.Background(), FinishLaneOptions{Change: ChangeOptions{Root: root, Base: base, Steps: steps, Citations: plan, VerifyFile: good, Outcome: "passed"}}, &stdout, &stderr); err != nil {
		t.Fatal(err)
	}
	if calls == 0 || strings.Contains(stderr.String(), "REFUSE unsupported-verify-syntax") || !strings.Contains(stdout.String(), "prepare FAIL") {
		t.Fatalf("valid verify file did not reach prepare: calls=%d stdout=%q stderr=%q", calls, stdout.String(), stderr.String())
	}
}

// DCW-V0-115: finish-lane's prepare pass runs only the CEM prepare its
// decision reads, never the coordination query, impact, OCM or recorder steps
// the rebind recomputes, and the run keeps one timing row per command.
func TestFinishLaneDCWV0115PreparePassRunsOnlyCEMPrepare(t *testing.T) {
	root := t.TempDir()
	testGit(t, root, "init", "-q")
	if err := os.WriteFile(filepath.Join(root, "a.txt"), []byte("a\n"), 0o666); err != nil {
		t.Fatal(err)
	}
	testGit(t, root, "add", "a.txt")
	testGit(t, root, "commit", "-q", "-m", "base")
	base := testGit(t, root, "rev-parse", "HEAD")
	if err := os.WriteFile(filepath.Join(root, "a.txt"), []byte("b\n"), 0o666); err != nil {
		t.Fatal(err)
	}
	testGit(t, root, "commit", "-q", "-am", "change")
	plan := filepath.Join(t.TempDir(), "cites.tsv")
	if err := os.WriteFile(plan, []byte("1\ta.txt\t1:1\tspecification\n"), 0o666); err != nil {
		t.Fatal(err)
	}
	var labels []string
	steps := Runner{Path: "corvint", Run: func(_ context.Context, _ string, args []string, _ io.Writer, _ io.Writer) int {
		labels = append(labels, commandLabel(args))
		return 1
	}}
	var stdout, stderr bytes.Buffer
	code, err := FinishLane(context.Background(), FinishLaneOptions{Change: ChangeOptions{Root: root, Base: base, Steps: steps, Citations: plan}}, &stdout, &stderr)
	if err != nil || code == 0 || !strings.Contains(stdout.String(), "dogfood-finish-lane: prepare FAIL") {
		t.Fatalf("code=%d err=%v stdout=%q stderr=%q", code, err, stdout.String(), stderr.String())
	}
	if len(labels) == 0 || labels[0] != "cem prepare" {
		t.Fatalf("prepare pass commands %q, want cem prepare first", labels)
	}
	for _, label := range labels {
		if label != "cem prepare" && label != "dogfood-observe" {
			t.Fatalf("prepare pass ran %q; commands %q", label, labels)
		}
	}
	gitDir := testGit(t, root, "rev-parse", "--absolute-git-dir")
	detail := gitDir + "/corvint/" + finishLaneTimings
	if want := "dogfood-finish-lane: TIMING total seconds="; !strings.Contains(stderr.String(), want) || !strings.Contains(stderr.String(), " commands="+fmt.Sprint(len(labels))+" detail="+detail+"\n") {
		t.Fatalf("stderr lacks the total TIMING line naming %s:\n%s", detail, stderr.String())
	}
	table, err := os.ReadFile(detail)
	if err != nil || !strings.HasPrefix(string(table), "step\tcommand\texit\tseconds\nprepare\tcem prepare\t1\t") || strings.Count(string(table), "\n") != len(labels)+1 {
		t.Fatalf("timings %q err=%v", table, err)
	}
}

// DCW-V0-378 (V1-2889): a supplied input file rewritten during the run is
// refused by name, instead of surfacing as the step's own drift refusal; an
// unchanged input leaves the step's failure as it was.
func TestFinishLaneDCWV0378RefusesAnInputChangedDuringTheRun(t *testing.T) {
	scratch := t.TempDir()
	plan := filepath.Join(scratch, "cites.tsv")
	intents := filepath.Join(scratch, "intents.json")
	for _, tc := range []struct {
		name, rewrite, flag string
	}{
		{name: "citations", rewrite: plan, flag: "--citations"},
		{name: "intents", rewrite: intents, flag: "--intents"},
		{name: "unchanged"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			testGit(t, root, "init", "-q")
			if err := os.WriteFile(filepath.Join(root, "a.txt"), []byte("a\n"), 0o666); err != nil {
				t.Fatal(err)
			}
			testGit(t, root, "add", "a.txt")
			testGit(t, root, "commit", "-q", "-m", "base")
			base := testGit(t, root, "rev-parse", "HEAD")
			if err := os.WriteFile(filepath.Join(root, "a.txt"), []byte("b\n"), 0o666); err != nil {
				t.Fatal(err)
			}
			testGit(t, root, "commit", "-q", "-am", "change")
			if err := os.WriteFile(plan, []byte("1\ta.txt\t1:1\tspecification\n"), 0o666); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(intents, []byte("{}\n"), 0o666); err != nil {
				t.Fatal(err)
			}
			steps := Runner{Path: "corvint", Run: func(context.Context, string, []string, io.Writer, io.Writer) int {
				if tc.rewrite != "" {
					if err := os.WriteFile(tc.rewrite, []byte("another lane's input\n"), 0o666); err != nil {
						t.Error(err)
					}
				}
				return 1
			}}
			var stdout, stderr bytes.Buffer
			code, err := FinishLane(context.Background(), FinishLaneOptions{Change: ChangeOptions{Root: root, Base: base, Steps: steps, Citations: plan, IntentsFile: intents}}, &stdout, &stderr)
			if err != nil {
				t.Fatal(err)
			}
			refused := strings.Contains(stderr.String(), "dogfood-finish-lane: REFUSE input-changed-during-run step=prepare\n")
			if tc.rewrite == "" {
				if refused || !strings.Contains(stdout.String(), "dogfood-finish-lane: prepare FAIL exit=") {
					t.Fatalf("unchanged inputs: code=%d stdout=%q stderr=%q", code, stdout.String(), stderr.String())
				}
				return
			}
			if code != 2 || !refused || !strings.Contains(stderr.String(), "  changed: "+tc.flag+" "+tc.rewrite+"\n") || !strings.Contains(stderr.String(), "lane-unique directory") || !strings.Contains(stdout.String(), "dogfood-finish-lane: prepare FAIL exit=2 input-changed\n") {
				t.Fatalf("code=%d stdout=%q stderr=%q, want the changed %s named", code, stdout.String(), stderr.String(), tc.flag)
			}
		})
	}
}

// DCW-V0-378: a non-regular input path is recorded as absent without being
// opened (a FIFO would block the open), and a regular file streams to its
// SHA-256 digest.
func TestFinishLaneDCWV0378DigestsOnlyRegularFiles(t *testing.T) {
	dir := t.TempDir()
	if _, present := fileDigest(dir); present {
		t.Fatalf("fileDigest(directory) reported a present digest")
	}
	if _, present := fileDigest(filepath.Join(dir, "missing")); present {
		t.Fatalf("fileDigest(missing) reported a present digest")
	}
	name := filepath.Join(dir, "plan.tsv")
	if err := os.WriteFile(name, []byte("plan\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	digest, present := fileDigest(name)
	if !present || digest != sha256.Sum256([]byte("plan\n")) {
		t.Fatalf("fileDigest(regular) = %x, %v", digest, present)
	}
}
