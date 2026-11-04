package typedreach

import (
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

const fixtureModule = "example.com/fix"

func analyzeFixture(t *testing.T, changed ...Declaration) Result {
	t.Helper()
	program, err := Load(filepath.Join("testdata", "mod"), fixtureModule, "p")
	if err != nil {
		t.Fatal(err)
	}
	return Analyze(program, changed, nil)
}

func lib(name string) Declaration { return Declaration{Package: fixtureModule + "/lib", Name: name} }

func keptCodes(result Result) map[string]string {
	codes := map[string]string{}
	for test, why := range result.Kept {
		codes[test], _, _ = strings.Cut(why, ":")
	}
	return codes
}

// TestAFPV02373KeepsOnlyTheTestsReachingAChangedFunction is the narrowing
// case and its negative control: a changed function keeps the tests that
// call it, the test reading state a kept test writes, and the test starting
// an unproved program; a test running only git is dropped. Changing the
// other function flips which test is kept.
func TestAFPV02373KeepsOnlyTheTestsReachingAChangedFunction(t *testing.T) {
	result := analyzeFixture(t, lib("A"))
	if result.Whole != "" {
		t.Fatalf("whole: %s", result.Whole)
	}
	want := map[string]string{"TestA": KeptReachesChange, "TestBump": KeptReachesChange, "TestRead": KeptReadsSharedState, "TestExec": KeptStartsProcess}
	if got := keptCodes(result); !maps(got, want) {
		t.Fatalf("kept %v, want %v", result.Kept, want)
	}
	if !slices.Equal(result.Dropped, []string{"TestB", "TestGit", "TestRegistry", "TestStringer"}) {
		t.Fatalf("dropped %v", result.Dropped)
	}
	// Negative control: TestB reaches B, and TestMain's child-only branch,
	// which also calls B, does not make the package whole (AFP-V0-2372).
	control := analyzeFixture(t, lib("B"))
	if control.Whole != "" || keptCodes(control)["TestB"] != KeptReachesChange || keptCodes(control)["TestA"] != "" {
		t.Fatalf("negative control: whole=%q kept=%v", control.Whole, control.Kept)
	}
}

// TestAFPV02371ConversionsAndHeldValuesCarryReach: a method reached only
// through a standard-library interface call is reached by the test that
// converts the value, and a function a package variable holds is reached by
// the test that reads the variable, not by package initialization.
func TestAFPV02371ConversionsAndHeldValuesCarryReach(t *testing.T) {
	stringer := analyzeFixture(t, lib("T.String"))
	if stringer.Whole != "" || keptCodes(stringer)["TestStringer"] != KeptReachesChange {
		t.Fatalf("String: whole=%q kept=%v", stringer.Whole, stringer.Kept)
	}
	held := analyzeFixture(t, lib("HandlerA"))
	if held.Whole != "" || keptCodes(held)["TestRegistry"] != KeptReachesChange || keptCodes(held)["TestA"] != "" {
		t.Fatalf("HandlerA: whole=%q kept=%v", held.Whole, held.Kept)
	}
}

// TestAFPV02372AlwaysRunReachKeepsThePackageWhole: a function a package
// variable initializer calls, and a changed initialization, keep every test.
func TestAFPV02372AlwaysRunReachKeepsThePackageWhole(t *testing.T) {
	if result := analyzeFixture(t, lib("Init")); !strings.HasPrefix(result.Whole, WholeAlwaysRunReaches+":") {
		t.Fatalf("Init: whole=%q kept=%v", result.Whole, result.Kept)
	}
	initialization := Declaration{Package: fixtureModule + "/lib", Name: "lib/lib.go: Eager", Initialization: true}
	if result := analyzeFixture(t, initialization); !strings.HasPrefix(result.Whole, WholeAlwaysRunReaches+":") {
		t.Fatalf("initialization: whole=%q", result.Whole)
	}
	// A package the binary does not link bears on nothing.
	unlinked := Declaration{Package: fixtureModule + "/other", Name: "x.go: init#+", Initialization: true}
	if result := analyzeFixture(t, unlinked); result.Whole != "" || len(result.Kept) != 1 {
		t.Fatalf("unlinked: whole=%q kept=%v", result.Whole, result.Kept)
	}
}

// TestAFPV02376DiffFileNamesChangedDeclarations: a body edit names the
// function and a method as Type.Method; an init or a calling package
// variable initializer is a change at initialization.
func TestAFPV02376DiffFileNamesChangedDeclarations(t *testing.T) {
	base := []byte("package q\n\nvar x = 1\n\nfunc F() int { return 1 }\n\ntype T struct{}\n\nfunc (*T) M() {}\n")
	head := []byte("package q\n\nvar x = 1\n\nfunc F() int { return 2 }\n\ntype T struct{}\n\nfunc (*T) M() { _ = 1 }\n")
	change := DiffFile("q.go", base, head)
	if change.AlwaysRun != "" || !slices.Equal(change.Names, []string{"F", "T.M"}) {
		t.Fatalf("body edit: %+v", change)
	}
	for _, edited := range []string{
		"package q\n\nvar x = 1\n\nfunc init() { _ = 2 }\n",
		"package q\n\nvar x = g()\n\nfunc g() int { return 1 }\n",
	} {
		if change := DiffFile("q.go", []byte("package q\n\nvar x = 1\n"), []byte(edited)); change.AlwaysRun == "" {
			t.Fatalf("initialization change not flagged: %+v", change)
		}
	}
	if change := DiffFile("q.go", base, []byte("package q\nfunc {")); change.AlwaysRun == "" {
		t.Fatal("parse failure not flagged")
	}
}

// TestAFPV02372LoadRefusesUnmodelledSource: assembly in a linked module
// package is an UNMODELLED_SOURCE refusal, never a slice.
func TestAFPV02372LoadRefusesUnmodelledSource(t *testing.T) {
	root := t.TempDir()
	for name, body := range map[string]string{
		"go.mod":      "module example.com/asm\n\ngo 1.27\n",
		"q/q.go":      "package q\n\nfunc F() int\n",
		"q/q_amd64.s": "TEXT ·F(SB),$0\n\tRET\n",
		"q/q_arm64.s": "TEXT ·F(SB),$0\n\tRET\n",
		"q/q_test.go": "package q\n\nimport \"testing\"\n\nfunc TestF(t *testing.T) { F() }\n",
	} {
		if err := os.MkdirAll(filepath.Join(root, filepath.Dir(name)), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(root, name), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := Load(root, "example.com/asm", "q"); !errors.Is(err, ErrUnmodelled) {
		t.Fatalf("err=%v, want ErrUnmodelled", err)
	}
}

func maps(got, want map[string]string) bool {
	if len(got) != len(want) {
		return false
	}
	for k, v := range want {
		if got[k] != v {
			return false
		}
	}
	return true
}
