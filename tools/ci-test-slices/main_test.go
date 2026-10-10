package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/Beamfall/corvint/.github/cishards"
)

const runURL = "https://github.com/beamfall/corvint/actions/runs/1"

func write(t *testing.T, path, body string) string {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

// stream is one shard's hosted log: package big has tests A..F, with F twice as
// slow; packages small and other are whole.
func stream(t *testing.T) string {
	var b strings.Builder
	b.WriteString("shard (0)\tGo tests\t2026-10-10T13:20:59Z not json {\n")
	b.WriteString(`2026-10-10T13:20:59Z {"Time":"2026-10-10T13:20:59Z","Action":"start","Package":"example.org/big"}` + "\n")
	for i, name := range []string{"TestA", "TestB", "TestC", "TestD", "TestE", "TestF"} {
		ms := 10.0
		if i == 5 {
			ms = 20
		}
		for _, test := range []string{name, name + "/sub"} {
			fmt.Fprintf(&b, "2026-10-10T13:20:59Z {\"Action\":\"run\",\"Package\":\"example.org/big\",\"Test\":%q}\n", test)
		}
		fmt.Fprintf(&b, "2026-10-10T13:20:59Z {\"Action\":\"pass\",\"Package\":\"example.org/big\",\"Test\":%q,\"Elapsed\":%g}\n", name+"/sub", ms)
		fmt.Fprintf(&b, "2026-10-10T13:20:59Z {\"Action\":\"pass\",\"Package\":\"example.org/big\",\"Test\":%q,\"Elapsed\":%g}\n", name, ms)
	}
	b.WriteString(`{"Action":"pass","Package":"example.org/big","Elapsed":35}
{"Action":"start","Package":"example.org/small"}
{"Action":"run","Package":"example.org/small","Test":"TestS"}
{"Action":"pass","Package":"example.org/small","Test":"TestS","Elapsed":5}
{"Action":"pass","Package":"example.org/small","Elapsed":5}
{"Action":"start","Package":"example.org/other"}
{"Action":"skip","Package":"example.org/other","Elapsed":0}
`)
	return write(t, filepath.Join(t.TempDir(), "shard0.log"), b.String())
}

func repo(t *testing.T, allow string) (string, string) {
	t.Helper()
	root := t.TempDir()
	write(t, filepath.Join(root, allowPath), allow)
	write(t, filepath.Join(root, slicesPath), "{}\n")
	for _, args := range [][]string{{"init", "-q"}, {"add", "-A"}, {"-c", "user.name=t", "-c", "user.email=t@example.org", "commit", "-q", "-m", "fixture"}} {
		if out, err := exec.Command("git", append([]string{"-C", root}, args...)...).CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v %s", args, err, out)
		}
	}
	head, err := exec.Command("git", "-C", root, "rev-parse", "HEAD").Output()
	if err != nil {
		t.Fatal(err)
	}
	return root, strings.TrimSpace(string(head))
}

func fakeList(t *testing.T, lists map[string][]string) {
	t.Helper()
	old := enumerate
	enumerate = func(_ context.Context, _ string, pkg string) ([]string, error) {
		names, ok := lists[pkg]
		if !ok {
			return nil, errors.New("build failed")
		}
		return names, nil
	}
	t.Cleanup(func() { enumerate = old })
}

const allowBig = `{"profile":"corvint-ci-test-split-allow/0","packages":{"example.org/big":"fixture","example.org/small":"fixture","example.org/broken":"fixture"}}`

func TestAFPV0041GenerateSplitsOnlySlowListablePackages(t *testing.T) {
	root, head := repo(t, allowBig)
	fakeList(t, map[string][]string{
		// TestG is new since the run: it takes the median. TestÜ cannot be named.
		"example.org/big":   {"TestA", "TestB", "TestC", "TestD", "TestE", "TestF", "TestG", "TestÜ", "ExampleBig"},
		"example.org/small": {"TestS"},
	})
	var notes bytes.Buffer
	log := stream(t)
	// Ideal share 40/2 = 20 s: big (35 s) splits in two; small stays whole.
	code, err := run(context.Background(), "generate", []string{"--root", root, "--revision", head, "--run-url", runURL, "--shards", "2", log}, nil, &notes)
	if code != 0 || err != nil {
		t.Fatalf("generate code=%d err=%v notes=%s", code, err, notes.String())
	}
	raw, err := os.ReadFile(filepath.Join(root, slicesPath))
	if err != nil {
		t.Fatal(err)
	}
	var f sliceFile
	if err = json.Unmarshal(raw, &f); err != nil {
		t.Fatal(err)
	}
	if len(f.Packages) != 1 || f.Source.Revision != head || f.Source.RunURL != runURL {
		t.Fatalf("unexpected file: %s", raw)
	}
	big := f.Packages["example.org/big"]
	// Sorted costs 10 (ExampleBig, median) 10 10 10 10 10 20 10 (TestG, median):
	// the earliest cut that minimises the larger slice is 40|50.
	if !reflect.DeepEqual(big.Named, []namedSlice{{Milliseconds: 15556, Tests: []string{"ExampleBig", "TestA", "TestB", "TestC"}}}) || big.RestMilliseconds != 19444 {
		t.Fatalf("unexpected split: %+v", big)
	}
	for _, want := range []string{"split example.org/big: 35000ms into 2 slices", "whole example.org/small: 5000ms within the 20000ms target", "whole example.org/broken: no observed outcome"} {
		if !strings.Contains(notes.String(), want) {
			t.Fatalf("notes lack %q:\n%s", want, notes.String())
		}
	}
	if got := cishards.SplitPackages([]string{"example.org/big"}, 2, []byte(allowBig), raw); len(got["example.org/big"]) != 2 {
		t.Fatal("generated file not admitted")
	}

	// An enumeration failure keeps the package whole.
	root, head = repo(t, allowBig)
	fakeList(t, map[string][]string{})
	notes.Reset()
	if code, err = run(context.Background(), "generate", []string{"--root", root, "--revision", head, "--run-url", runURL, "--shards", "2", log}, nil, &notes); code != 0 || err != nil {
		t.Fatalf("generate code=%d err=%v", code, err)
	}
	if raw, _ = os.ReadFile(filepath.Join(root, slicesPath)); !bytes.Contains(raw, []byte(`"packages": {}`)) || !strings.Contains(notes.String(), "whole example.org/big: build failed") {
		t.Fatalf("enumeration failure split the package: %s %s", raw, notes.String())
	}
}

func TestAFPV0041GenerateRefusesUnboundInputs(t *testing.T) {
	log := stream(t)
	fakeList(t, map[string][]string{"example.org/big": {"TestA", "TestB"}})
	root, head := repo(t, allowBig)
	for name, args := range map[string][]string{
		"absent revision": {"--revision", strings.Repeat("0", 40)},
		"short revision":  {"--revision", head[:12]},
		"symbolic":        {"--revision", "HEAD"},
		"bad run URL":     {"--revision", head, "--run-url", "https://example.org/run"},
	} {
		a := append([]string{"--root", root, "--shards", "2", "--run-url", runURL}, args...)
		if code, err := run(context.Background(), "generate", append(a, log), nil, &bytes.Buffer{}); code != 2 || err == nil {
			t.Fatalf("%s: code=%d err=%v", name, code, err)
		}
	}
	failed := write(t, filepath.Join(t.TempDir(), "failed.log"), `{"Action":"fail","Package":"example.org/big","Test":"TestA","Elapsed":1}`+"\n")
	twice := write(t, filepath.Join(t.TempDir(), "twice.log"), `{"Action":"start","Package":"example.org/big"}`+"\n"+`{"Action":"pass","Package":"example.org/big","Elapsed":1}`+"\n")
	for want, logs := range map[string][]string{"TestA failed": {failed}, "two terminal outcomes": {twice, twice}} {
		if code, err := run(context.Background(), "replay", logs, nil, &bytes.Buffer{}); code != 2 || err == nil || !strings.Contains(err.Error(), want) {
			t.Fatalf("unusable run accepted: %v: %v", logs, err)
		}
	}

	// Nothing splits under a long target, and the source is still checked, before
	// the checkout.
	for name, url := range map[string]string{"bad run URL, empty split": "https://example.org/run", "missing run URL, empty split": ""} {
		var notes bytes.Buffer
		code, err := run(context.Background(), "generate", []string{"--root", root, "--revision", head, "--run-url", url, "--shards", "2", "--target", "1h", log}, nil, &notes)
		if code != 2 || err == nil || !strings.Contains(err.Error(), "would reject this source") {
			t.Fatalf("%s: code=%d err=%v", name, code, err)
		}
		if raw, _ := os.ReadFile(filepath.Join(root, slicesPath)); string(raw) != "{}\n" {
			t.Fatalf("%s: slice file overwritten: %s", name, raw)
		}
	}
	// The same empty split with a usable source is written and admitted.
	if code, err := run(context.Background(), "generate", []string{"--root", root, "--revision", head, "--run-url", runURL, "--shards", "2", "--target", "1h", log}, nil, &bytes.Buffer{}); code != 0 || err != nil {
		t.Fatalf("empty generation refused: code=%d err=%v", code, err)
	}
	if raw, _ := os.ReadFile(filepath.Join(root, slicesPath)); !cishards.SliceFileUsable(raw) {
		t.Fatalf("empty generation wrote an unusable file: %s", raw)
	}
}

// TestAFPV0041GenerateEnumeratesOnlyTheRevision proves enumeration and the
// allow-list come from a checkout of exactly --revision: an untracked test file,
// a modified tracked test file and a modified allow-list in the working tree
// contribute nothing, and the repository's index and worktrees are untouched.
func TestAFPV0041GenerateEnumeratesOnlyTheRevision(t *testing.T) {
	root := t.TempDir()
	write(t, filepath.Join(root, allowPath), allowBig)
	write(t, filepath.Join(root, slicesPath), "{}\n")
	write(t, filepath.Join(root, "big", "a_test.go"), "package big\n\nfunc TestTracked(t *testing.T) {}\n")
	for _, args := range [][]string{{"init", "-q"}, {"add", "-A"}, {"-c", "user.name=t", "-c", "user.email=t@example.org", "commit", "-q", "-m", "fixture"}} {
		if out, err := exec.Command("git", append([]string{"-C", root}, args...)...).CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v %s", args, err, out)
		}
	}
	rev, err := exec.Command("git", "-C", root, "rev-parse", "HEAD").Output()
	if err != nil {
		t.Fatal(err)
	}
	head := strings.TrimSpace(string(rev))
	write(t, filepath.Join(root, "big", "untracked_test.go"), "package big\n\nfunc TestUntracked(t *testing.T) {}\n")
	write(t, filepath.Join(root, "big", "a_test.go"), "package big\n\nfunc TestModified(t *testing.T) {}\n")
	write(t, filepath.Join(root, allowPath), `{"profile":"corvint-ci-test-split-allow/0","packages":{"example.org/small":"fixture"}}`)
	// A staged change makes the repository's index differ from --revision.
	if out, err := exec.Command("git", "-C", root, "add", "big/a_test.go").CombinedOutput(); err != nil {
		t.Fatalf("git add: %v %s", err, out)
	}
	state := func() string {
		var b strings.Builder
		for _, args := range [][]string{{"status", "--porcelain", "--untracked-files=all"}, {"ls-files", "--stage"}, {"worktree", "list", "--porcelain"}, {"for-each-ref"}} {
			out, err := exec.Command("git", append([]string{"-C", root}, args...)...).CombinedOutput()
			if err != nil {
				t.Fatalf("git %v: %v %s", args, err, out)
			}
			b.Write(out)
		}
		return b.String()
	}
	before := state()
	var seen string
	old := enumerate
	enumerate = func(_ context.Context, tree string, pkg string) ([]string, error) {
		seen = tree
		var names []string
		for _, f := range []string{"a_test.go", "untracked_test.go"} {
			raw, err := os.ReadFile(filepath.Join(tree, "big", f))
			if err != nil {
				continue
			}
			for _, line := range strings.Split(string(raw), "\n") {
				if name, ok := strings.CutPrefix(line, "func "); ok {
					names = append(names, strings.SplitN(name, "(", 2)[0])
				}
			}
		}
		return names, nil
	}
	t.Cleanup(func() { enumerate = old })
	var notes bytes.Buffer
	if code, err := run(context.Background(), "generate", []string{"--root", root, "--revision", head, "--run-url", runURL, "--shards", "2", stream(t)}, nil, &notes); code != 0 || err != nil {
		t.Fatalf("generate code=%d err=%v notes=%s", code, err, notes.String())
	}
	raw, err := os.ReadFile(filepath.Join(root, slicesPath))
	if err != nil {
		t.Fatal(err)
	}
	var f sliceFile
	if err = json.Unmarshal(raw, &f); err != nil {
		t.Fatal(err)
	}
	// The committed allow-list admits big; the working tree's does not.
	big, ok := f.Packages["example.org/big"]
	if !ok || !reflect.DeepEqual(big.Named[0].Tests, []string{"TestTracked"}) {
		t.Fatalf("enumerated beyond --revision: %s", raw)
	}
	if seen == "" || strings.HasPrefix(seen, root) {
		t.Fatalf("enumerated in %q, not a checkout of the revision", seen)
	}
	if _, err = os.Stat(seen); !os.IsNotExist(err) {
		t.Fatalf("checkout %s left behind: %v", seen, err)
	}
	write(t, filepath.Join(root, slicesPath), "{}\n")
	if after := state(); after != before {
		t.Fatal("generate changed the repository's index, status or worktrees")
	}
}

// TestAFPV0041InterruptRemovesTheCheckout signals a real generate process while
// it is blocked in enumeration. The run must be cancelled, the temporary checkout
// removed and the slice file left unwritten.
func TestAFPV0041InterruptRemovesTheCheckout(t *testing.T) {
	if os.Getenv("CI_TEST_SLICES_HELPER") == "interrupt" {
		enumerate = func(ctx context.Context, tree, _ string) ([]string, error) {
			fmt.Printf("tree=%s\n", tree)
			<-ctx.Done()
			return nil, ctx.Err()
		}
		os.Exit(command(strings.Split(os.Getenv("CI_TEST_SLICES_ARGS"), "\n"), os.Stdout, os.Stderr))
	}
	if runtime.GOOS == "windows" {
		t.Skip("SIGINT and SIGTERM delivery is unix-only")
	}
	log := stream(t)
	for _, sig := range []os.Signal{os.Interrupt, syscall.SIGTERM} {
		root, head := repo(t, allowBig)
		tmp := t.TempDir()
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
		defer cancel()
		cmd := exec.CommandContext(ctx, os.Args[0], "-test.run=^TestAFPV0041InterruptRemovesTheCheckout$")
		cmd.Env = append(os.Environ(), "CI_TEST_SLICES_HELPER=interrupt", "TMPDIR="+tmp,
			"CI_TEST_SLICES_ARGS="+strings.Join([]string{"generate", "--root", root, "--revision", head, "--run-url", runURL, "--shards", "2", log}, "\n"))
		var stderr bytes.Buffer
		cmd.Stderr = &stderr
		stdout, err := cmd.StdoutPipe()
		if err != nil {
			t.Fatal(err)
		}
		if err = cmd.Start(); err != nil {
			t.Fatal(err)
		}
		line, _ := bufio.NewReader(stdout).ReadString('\n')
		tree, ok := strings.CutPrefix(strings.TrimSpace(line), "tree=")
		if !ok || !strings.HasPrefix(tree, tmp) {
			_ = cmd.Process.Kill()
			t.Fatalf("%v: helper did not reach enumeration in a checkout under %s: %q %s", sig, tmp, line, stderr.String())
		}
		if _, err = os.Stat(filepath.Join(tree, allowPath)); err != nil {
			t.Fatalf("%v: no checkout while enumerating: %v", sig, err)
		}
		if err = cmd.Process.Signal(sig); err != nil {
			t.Fatal(err)
		}
		err = cmd.Wait()
		var exit *exec.ExitError
		if !errors.As(err, &exit) || exit.ExitCode() != 2 || !strings.Contains(stderr.String(), "interrupted") {
			t.Fatalf("%v: exit=%v stderr=%s", sig, err, stderr.String())
		}
		if left, _ := os.ReadDir(tmp); len(left) != 0 {
			t.Fatalf("%v: temporary checkout left behind: %v", sig, left)
		}
		if raw, _ := os.ReadFile(filepath.Join(root, slicesPath)); string(raw) != "{}\n" {
			t.Fatalf("%v: interrupted run wrote the slice file: %s", sig, raw)
		}
	}
}

func TestAFPV0041ObserveRefusesIncompleteOrFailedLogs(t *testing.T) {
	small := `{"Action":"start","Package":"example.org/small"}
{"Action":"run","Package":"example.org/small","Test":"TestS"}
{"Action":"pass","Package":"example.org/small","Test":"TestS","Elapsed":5}
{"Action":"pass","Package":"example.org/small","Elapsed":5}
`
	complete := small + `{"Action":"start","Package":"example.org/big"}
2026-10-10T13:20:59Z {"Action":"run","Package":"example.org/big","Test":"TestA"}
{"Action":"run","Package":"example.org/big","Test":"TestA/sub"}
{"Action":"pass","Package":"example.org/big","Test":"TestA/sub","Elapsed":1}
{"Action":"pass","Package":"example.org/big","Test":"TestA","Elapsed":1}
{"Action":"pass","Package":"example.org/big","Elapsed":2}
`
	// Runner, shell and compiler lines are not records, even with braces in them.
	chatter := "2026-10-10T13:19:03Z Worker ID: {1a7b06f0}\n# example.org/big\n./a.go:3:1: syntax error near {\nprintf '{\"profile\":\"x\",\"tree\":\"%s\"\n"
	// Build output names ImportPath, not Package, and is skipped.
	built := `{"ImportPath":"example.org/big [example.org/big.test]","Action":"build-output","Output":"# example.org/big\n"}` + "\n"
	if _, err := observe([]string{write(t, filepath.Join(t.TempDir(), "complete.log"), chatter+built+complete)}); err != nil {
		t.Fatalf("complete passing log refused: %v", err)
	}
	// The same records behind every hosted prefix: gh columns, the first line's
	// byte-order mark, a fractional runner timestamp and a ##[error] annotation.
	var hosted strings.Builder
	for i, l := range strings.SplitAfter(chatter+complete, "\n") {
		if l == "" {
			continue
		}
		hosted.WriteString("shard (0)\tGo tests\t")
		if i == 0 {
			hosted.WriteString("\ufeff")
		}
		hosted.WriteString("2026-10-10T13:19:03.3027757Z ")
		l = strings.TrimPrefix(strings.TrimPrefix(l, "2026-10-10T13:19:03Z "), "2026-10-10T13:20:59Z ")
		if strings.Contains(l, `"Test":"TestA/sub","Elapsed"`) {
			hosted.WriteString("##[error]")
		}
		hosted.WriteString(l)
	}
	if o, err := observe([]string{write(t, filepath.Join(t.TempDir(), "hosted.log"), hosted.String())}); err != nil || o.packages["example.org/big"] != 2000 {
		t.Fatalf("prefixed passing log: %+v, %v", o, err)
	}
	for name, c := range map[string]struct{ log, want string }{
		// One completed package, then a failed subtest and truncated output.
		"failed subtest, truncated": {small + `{"Action":"start","Package":"example.org/big"}
{"Action":"run","Package":"example.org/big","Test":"TestA"}
{"Action":"run","Package":"example.org/big","Test":"TestA/sub"}
{"Action":"fail","Package":"example.org/big","Test":"TestA/sub","Elapsed":1}
{"Action":"outp`, "example.org/big TestA/sub failed"},
		"failed subtest only": {strings.Replace(complete, `"pass","Package":"example.org/big","Test":"TestA/sub"`, `"fail","Package":"example.org/big","Test":"TestA/sub"`, 1), "example.org/big TestA/sub failed"},
		"failed package":      {strings.Replace(complete, `"pass","Package":"example.org/big","Elapsed"`, `"fail","Package":"example.org/big","Elapsed"`, 1), "example.org/big  failed"},
		"truncated package": {small + `{"Action":"start","Package":"example.org/big"}
{"Action":"run","Package":"example.org/big","Test":"TestA"}
{"Action":"pass","Package":"example.org/big","Test":"TestA","Elapsed":1}
`, "package example.org/big has no terminal pass or skip"},
		"test never ended": {strings.Replace(complete, `{"Action":"pass","Package":"example.org/big","Test":"TestA/sub","Elapsed":1}`+"\n", "", 1), "test example.org/big TestA/sub started but never ended"},
		// start -> pass -> start -> EOF: a second lifecycle after the terminal outcome.
		"restarted after its outcome": {complete + `{"Action":"start","Package":"example.org/big"}` + "\n", "package example.org/big has a start record after its terminal outcome"},
		"record after its outcome":    {complete + `{"Action":"output","Package":"example.org/big","Output":"late\n"}` + "\n", "package example.org/big has a output record after its terminal outcome"},
		"started twice":               {strings.Replace(complete, `{"Action":"start","Package":"example.org/big"}`, `{"Action":"start","Package":"example.org/big"}`+"\n"+`{"Action":"start","Package":"example.org/big"}`, 1), "package example.org/big started twice"},
		"head truncated":              {strings.Replace(complete, `{"Action":"start","Package":"example.org/big"}`+"\n", "", 1), "package example.org/big has a run record before its start"},
		"test ended without running":  {strings.Replace(complete, `{"Action":"run","Package":"example.org/big","Test":"TestA/sub"}`+"\n", "", 1), "test example.org/big TestA/sub ended without running"},
		"test ran twice":              {strings.Replace(complete, `{"Action":"run","Package":"example.org/big","Test":"TestA/sub"}`, `{"Action":"run","Package":"example.org/big","Test":"TestA/sub"}`+"\n"+`{"Action":"run","Package":"example.org/big","Test":"TestA/sub"}`, 1), "test example.org/big TestA/sub ran twice"},
		"damaged trailing record":     {complete + `2026-10-10T13:21:00Z {"Time":"2026-10-10T13:21:00Z","Action":"pass","Pack`, "line 11: damaged go test -json record"},
		"damaged middle record":       {strings.Replace(complete, `{"Action":"run","Package":"example.org/big","Test":"TestA/sub"}`, `{"Action":"run","Package":"example.org/big","Test":"TestA/sub"`, 1), "line 7: damaged go test -json record"},
		// Completed packages followed by a record cut inside its first key.
		"truncated Time key":      {complete + `{"Time"`, "line 11: damaged go test -json record"},
		"truncated Action key":    {complete + `{"Act`, "line 11: damaged go test -json record"},
		"lone brace":              {complete + "{\n", "line 11: damaged go test -json record"},
		"truncated after stamp":   {complete + `2026-10-10T13:21:00.1234567Z {"Ti`, "line 11: damaged go test -json record"},
		"truncated gh line":       {complete + "shard (0)\tGo tests\t2026-10-10T13:21:00Z {", "line 11: damaged go test -json record"},
		"truncated annotation":    {complete + `2026-10-10T13:21:00Z ##[error]{"Action":"pa`, "line 11: damaged go test -json record"},
		"record without action":   {complete + `{"Time":"2026-10-10T13:21:00Z"}`, "line 11: damaged go test -json record: no Action"},
		"record after other text": {complete + `stderr {"Action":"output","Package":"example.org/late"}`, "line 11: go test -json record after other text"},
		// A completed passing package, then a failed build and EOF.
		"build failed, truncated":   {small + `{"Action":"build-fail","ImportPath":"example.org/broken"}` + "\n", "line 5: build of example.org/broken failed"},
		"fail without package":      {small + `{"Action":"fail"}` + "\n", "line 5: fail record without Package"},
		"output without package":    {small + `{"Action":"output","Output":"ok\n"}` + "\n", "line 5: output record without Package"},
		"build output without path": {small + `{"Action":"build-output","Output":"x"}` + "\n", "line 5: build-output record without ImportPath"},
	} {
		_, err := observe([]string{write(t, filepath.Join(t.TempDir(), "shard.log"), c.log)})
		if err == nil || !strings.Contains(err.Error(), c.want) {
			t.Fatalf("%s: err=%v, want %q", name, err, c.want)
		}
	}
}

func TestAFPV0041ListParsing(t *testing.T) {
	ok := "TestA\nBenchmarkB\nExampleC\nFuzzD\nTestÜ\nok  \texample.org/p\t0.1s\n"
	names, err := parseList([]byte(ok), "example.org/p")
	if err != nil || !reflect.DeepEqual(names, []string{"ExampleC", "FuzzD", "TestA", "TestÜ"}) {
		t.Fatalf("names=%v err=%v", names, err)
	}
	for _, bad := range []string{"TestA\n", "TestA\nsetup noise\nok  \texample.org/p\t0.1s\n", "ok  \texample.org/p\t0.1s\nTestA\n"} {
		if _, err := parseList([]byte(bad), "example.org/p"); err == nil {
			t.Fatalf("accepted %q", bad)
		}
	}
}

func TestAFPV0041SliceWithoutTimesSplitsByCount(t *testing.T) {
	s, ok := slice([]string{"TestA", "TestB", "TestC", "TestD", "TestE", "TestF"}, nil, 60000, 3)
	if !ok || len(s.Named) != 2 || len(s.Named[0].Tests) != 2 || len(s.Named[1].Tests) != 2 || s.Named[0].Milliseconds != 20000 || s.RestMilliseconds != 20000 {
		t.Fatalf("uneven count split: %+v", s)
	}
	if _, ok = slice([]string{"TestA", "TestÜ"}, nil, 1, 3); ok {
		t.Fatal("split with fewer nameable tests than named slices")
	}
}

func TestAFPV0041MinMaxCutsIsOptimal(t *testing.T) {
	for _, c := range []struct {
		costs []int64
		k     int
	}{{[]int64{5, 1, 1, 1, 5, 0}, 2}, {[]int64{9, 1, 1, 1, 1, 1, 1, 3}, 3}, {[]int64{1, 1, 1, 1}, 4}, {[]int64{3, 0}, 2}} {
		cuts := minMaxCuts(c.costs, c.k)
		if len(cuts) != c.k || cuts[0] != 0 {
			t.Fatalf("cuts %v", cuts)
		}
		if got, want := largest(c.costs, cuts), brute(c.costs, c.k); got != want {
			t.Fatalf("%v k=%d: largest %d, optimum %d", c.costs, c.k, got, want)
		}
	}
}

func largest(costs []int64, cuts []int) int64 {
	var top int64
	for i := range cuts {
		end := len(costs)
		if i+1 < len(cuts) {
			end = cuts[i+1]
		}
		var sum int64
		for _, c := range costs[cuts[i]:end] {
			sum += c
		}
		top = max(top, sum)
	}
	return top
}

func brute(costs []int64, k int) int64 {
	if k == 1 {
		var sum int64
		for _, c := range costs {
			sum += c
		}
		return sum
	}
	best := int64(1 << 62)
	for i := 1; i <= len(costs)-(k-1); i++ {
		var head int64
		for _, c := range costs[:i] {
			head += c
		}
		best = min(best, max(head, brute(costs[i:], k-1)))
	}
	return best
}

func TestAFPV0041ReplayPredictsShardSums(t *testing.T) {
	root := t.TempDir()
	write(t, filepath.Join(root, allowPath), allowBig)
	slices := `{"profile":"corvint-ci-test-slices/0","source":{"revision":"0123456789abcdef0123456789abcdef01234567","runURL":"` + runURL + `","goVersion":"go1.27.1"},
"packages":{"example.org/big":{"named":[{"milliseconds":17500,"tests":["TestA","TestB","TestC"]}],"restMilliseconds":17500}}}`
	write(t, filepath.Join(root, slicesPath), slices)
	var out bytes.Buffer
	if code, err := run(context.Background(), "replay", []string{"--root", root, "--shards", "2", stream(t)}, &out, &bytes.Buffer{}); code != 0 || err != nil {
		t.Fatalf("replay code=%d err=%v", code, err)
	}
	// big: 35 s over 70 ms of top-level tests; A..C are 30 ms (15 s), D..F 40 ms (20 s).
	for _, want := range []string{
		"universe 3 packages 40.0s ideal 20.0s over 2 shards",
		"whole    shard0=35.0s shard1=5.0s max=35.0s",
		"sliced   shard0=20.0s shard1=20.0s max=20.0s",
		"slice    shard=0 example.org/big#0/2 -run 3 tests predicted=15.0s planned=17.5s",
		"slice    shard=1 example.org/big#1/2 -skip 3 tests predicted=20.0s planned=17.5s",
	} {
		if !strings.Contains(out.String(), want) {
			t.Fatalf("replay lacks %q:\n%s", want, out.String())
		}
	}
}
