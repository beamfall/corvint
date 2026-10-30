package docmaintain

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"testing"
	"time"
)

const widgetExamples = `package widget

import "fmt"

func ExampleSplit() {
	fmt.Println(Split("k"))
	// Output: k
}

func ExampleSplit_wrong() {
	fmt.Println(Split("k"))
	// Output: not k
}
`

func declare(name string) string {
	return `<!-- corvint:docmaintain example source="owner.md" package="widget" name="` + name + `" -->` + "\n"
}

// SDD-V0-014: example declarations are prose to a maintenance session. A refresh keeps them byte
// for byte, a second refresh is a no-op, and only CheckExamples runs them, through go test.
func TestSDDV0014ExamplesCheckedThroughGoTest(t *testing.T) {
	goBin, err := exec.LookPath("go")
	if err != nil {
		t.Skip("go toolchain not on PATH")
	}
	root := fixtureRepo(t, digestV1)
	if err := os.WriteFile(filepath.Join(root, "widget", "example_test.go"), []byte(widgetExamples), 0o644); err != nil {
		t.Fatal(err)
	}
	commit(t, root, "examples")
	selector := []Selector{{Source: "owner.md", Package: "widget"}}
	writePage(t, root, "# Page\n\n"+insertionPoint+"\n")
	refresh := func() *Result {
		t.Helper()
		r, err := Run(context.Background(), root, "docs/page.md", selector, alwaysPolicy())
		if err != nil {
			t.Fatal(err)
		}
		return r
	}
	refresh()
	tail := "\nChecked examples:\n" + declare("ExampleSplit") + declare("ExampleSplit_wrong") + declare("ExampleSplit_missing")
	writePage(t, root, readPage(t, root)+tail)
	commit(t, root, "declare")

	// A source change refreshes the block; the declarations survive unchanged, and a repeat is a no-op.
	if err := os.WriteFile(filepath.Join(root, "owner.md"), []byte(digestV2), 0o644); err != nil {
		t.Fatal(err)
	}
	commit(t, root, "owner v2")
	refresh()
	page := readPage(t, root)
	if !strings.HasSuffix(page, tail) || !strings.Contains(page, "splits keys in half.") {
		t.Fatalf("refresh lost the declarations or the new source:\n%s", page)
	}
	if again := refresh(); string(again.ProposedPage) != page {
		t.Fatal("a second refresh changed the page")
	}

	calls := 0
	counting := func(ctx context.Context, root, pkg string, names []string) (map[string]string, error) {
		calls++
		return GoTestRunner(goBin)(ctx, root, pkg, names)
	}
	report, err := CheckExamples(context.Background(), root, []byte(page), counting)
	if err != nil {
		t.Fatal(err)
	}
	if calls != 1 {
		t.Fatalf("want one provider run for one package, got %d", calls)
	}
	got := map[string]string{}
	for _, r := range report.Results {
		got[r.Name] = r.Status
	}
	want := map[string]string{"ExampleSplit": ExamplePass, "ExampleSplit_wrong": ExampleFail, "ExampleSplit_missing": ExampleNotRun}
	for k, v := range want {
		if got[k] != v {
			t.Fatalf("%s: want %s, got %s (%+v)", k, v, got[k], report)
		}
	}
	if report.Verdict != ExampleFail || report.Profile != ExampleCheckProfile {
		t.Fatalf("want a FAIL report, got %+v", report)
	}
}

// SDD-V0-014: a declaration is read only outside fences and block bodies, must bind to a block on
// the page, and must name a relative package directory; every other form is a problem, never
// silently dropped.
func TestSDDV0014DeclarationProblems(t *testing.T) {
	block := string(renderBlock(Selector{Source: "owner.md", Package: "widget"}, "c", "t", "ab", []byte("body\n"+declare("ExampleInBody"))))
	page := "# Page\n\n" + block +
		declare("ExampleSplit") +
		"```\n" + declare("ExampleFenced") + "```\n" +
		`<!-- corvint:docmaintain example source="other.md" package="widget" name="ExampleSplit" -->` + "\n" +
		`<!-- corvint:docmaintain example source="owner.md" package="widget" name="Split" -->` + "\n" +
		`<!-- corvint:docmaintain example source="owner.md" package="widget" name="ExampleSplit" extra="1" -->` + "\n"
	decls, problems, refusal := DeclaredExamples([]byte(page))
	if refusal != "" {
		t.Fatalf("refused: %s", refusal)
	}
	if len(decls) != 1 || decls[0].Name != "ExampleSplit" || decls[0].Package != "widget" {
		t.Fatalf("decls: %+v", decls)
	}
	var codes []string
	for _, p := range problems {
		codes = append(codes, p.Code)
	}
	if strings.Join(codes, ",") != ExampleInBlockBody+","+ExampleUnboundBlock+","+ExampleMalformed+","+ExampleMalformed {
		t.Fatalf("problems: %+v", problems)
	}
	for _, pkg := range []string{"../widget", "/abs", "-run", "a/./b", ".git/x", "a b", ""} {
		if validExamplePackage(pkg) {
			t.Fatalf("package %q admitted", pkg)
		}
	}
	report, err := CheckExamples(context.Background(), t.TempDir(), []byte(page), func(context.Context, string, string, []string) (map[string]string, error) {
		return map[string]string{"ExampleSplit": ExamplePass}, nil
	})
	if err != nil || report.Verdict != ExampleFail {
		t.Fatalf("problems must fail the check: %+v %v", report, err)
	}
}

// fakeGo writes an executable standing in for the go command: it ignores its arguments and runs
// body as a POSIX shell script.
func fakeGo(t *testing.T, body string) string {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("the fake go command is a POSIX shell script")
	}
	path := filepath.Join(t.TempDir(), "go")
	if err := os.WriteFile(path, []byte("#!/bin/sh\n"+body+"\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	return path
}

const passEvent = `{"Action":"pass","Test":"ExampleA"}`

// SDD-V0-014: output past the scanner's bound (here one unterminated line far longer than a
// line's limit) is drained, so a child that keeps writing still exits and the run returns what it
// observed instead of blocking until the context ends.
func TestSDDV0014GoTestRunnerDrainsOutputPastTheBound(t *testing.T) {
	goBin := fakeGo(t, "printf '%s\\n' '"+passEvent+"'\nexec head -c 8000000 /dev/zero")
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	states, err := GoTestRunner(goBin)(ctx, t.TempDir(), "widget", []string{"ExampleA"})
	if err != nil || states["ExampleA"] != ExamplePass {
		t.Fatalf("states=%v err=%v, want ExampleA PASS without blocking", states, err)
	}
}

// SDD-V0-014: a nonzero go test exit that recorded no FAIL (a TestMain or teardown failure) is not
// a passing run: every example of that package stays NOT_RUN, so the verdict is FAIL.
func TestSDDV0014GoTestRunnerNonzeroExitWithoutFailIsNotRun(t *testing.T) {
	goBin := fakeGo(t, "printf '%s\\n' '"+passEvent+"'\nexit 1")
	states, err := GoTestRunner(goBin)(context.Background(), t.TempDir(), "widget", []string{"ExampleA"})
	if err != nil || len(states) != 0 {
		t.Fatalf("states=%v err=%v, want no observed state", states, err)
	}
	failing := fakeGo(t, "printf '%s\\n' '"+passEvent+"' '{\"Action\":\"fail\",\"Test\":\"ExampleB\"}'\nexit 1")
	states, err = GoTestRunner(failing)(context.Background(), t.TempDir(), "widget", []string{"ExampleA", "ExampleB"})
	if err != nil || states["ExampleA"] != ExamplePass || states["ExampleB"] != ExampleFail {
		t.Fatalf("states=%v err=%v, want the recorded PASS and FAIL kept", states, err)
	}
}

// SDD-V0-014: a cancelled run returns the context's error promptly even when a descendant of the
// go command, which CommandContext does not kill, still holds the output pipes open.
func TestSDDV0014GoTestRunnerCancelReturnsDespiteDescendantHoldingPipes(t *testing.T) {
	for _, redirect := range []string{"", " 2>/dev/null"} {
		pidFile := filepath.Join(t.TempDir(), "pid")
		goBin := fakeGo(t, "/bin/sleep 60"+redirect+" &\necho $! > '"+pidFile+"'\nexec /bin/sleep 60")
		t.Cleanup(func() {
			if raw, err := os.ReadFile(pidFile); err == nil {
				if pid, err := strconv.Atoi(strings.TrimSpace(string(raw))); err == nil {
					if p, err := os.FindProcess(pid); err == nil {
						_ = p.Kill()
					}
				}
			}
		})
		ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
		started := time.Now()
		_, err := GoTestRunner(goBin)(ctx, t.TempDir(), "widget", []string{"ExampleA"})
		cancel()
		if err == nil || !errors.Is(err, context.DeadlineExceeded) {
			t.Fatalf("redirect %q: err=%v, want the context's deadline", redirect, err)
		}
		if elapsed := time.Since(started); elapsed > 20*time.Second {
			t.Fatalf("redirect %q: the cancelled run blocked for %s", redirect, elapsed)
		}
	}
}
