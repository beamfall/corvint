package typedreach

import (
	"errors"
	"go/types"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

const fixtureModule = "example.com/fix"

func analyzeFixture(t *testing.T, changed ...Declaration) Result {
	t.Helper()
	return analyzePackage(t, "p", changed...)
}

func analyzePackage(t *testing.T, pkg string, changed ...Declaration) Result {
	t.Helper()
	program, err := Load(filepath.Join("testdata", "mod"), fixtureModule, pkg)
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
// other function flips which test is kept. A constant git start still keeps
// its test: configuration, aliases or hooks can run module code.
func TestAFPV02373KeepsOnlyTheTestsReachingAChangedFunction(t *testing.T) {
	result := analyzeFixture(t, lib("A"))
	if result.Whole != "" {
		t.Fatalf("whole: %s", result.Whole)
	}
	want := map[string]string{"TestA": KeptReachesChange, "TestBump": KeptReachesChange, "TestRead": KeptReadsSharedState, "TestExec": KeptStartsProcess, "TestGit": KeptStartsProcess}
	if got := keptCodes(result); !maps(got, want) {
		t.Fatalf("kept %v, want %v", result.Kept, want)
	}
	if !slices.Equal(result.Dropped, []string{"TestB", "TestRegistry", "TestStringer"}) {
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
	if result := analyzeFixture(t, unlinked); result.Whole != "" || len(result.Kept) != 2 {
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

// TestAFPV02374ReviewRegressions holds the independent review's soundness
// cases: a conversion through a multi-value return, an exec function taken
// as a value, a write through a copy of a package slice, a TestMain branch
// the parent process takes, a fixed-path file write, and an import change.
func TestAFPV02374ReviewRegressions(t *testing.T) {
	boxed := analyzePackage(t, "q", lib("T.String"))
	if boxed.Whole != "" || keptCodes(boxed)["TestBoxed"] != KeptReachesChange {
		t.Fatalf("tuple conversion: whole=%q kept=%v", boxed.Whole, boxed.Kept)
	}
	if keptCodes(boxed)["TestIndirectExec"] != KeptStartsProcess {
		t.Fatalf("exec as a value: kept=%v", boxed.Kept)
	}
	alias := analyzePackage(t, "q", lib("A"))
	if keptCodes(alias)["TestAliasWrite"] != KeptReachesChange || keptCodes(alias)["TestAliasRead"] != KeptReadsSharedState {
		t.Fatalf("alias write: whole=%q kept=%v", alias.Whole, alias.Kept)
	}
	// len of a package slice is a read, not a write: when only B changes,
	// TestLength is kept and the tests that only read Values are not.
	if plain := analyzePackage(t, "q", lib("B")); keptCodes(plain)["TestLength"] != KeptReachesChange || keptCodes(plain)["TestAliasRead"] != "" {
		t.Fatalf("plain read: whole=%q kept=%v", plain.Whole, plain.Kept)
	}
	if parent := analyzePackage(t, "r", lib("A")); !strings.HasPrefix(parent.Whole, WholeAlwaysRunReaches+":") {
		t.Fatalf("parent branch: whole=%q kept=%v", parent.Whole, parent.Kept)
	}
	if fixed := analyzePackage(t, "w", lib("A")); !strings.HasPrefix(fixed.Whole, WholeProcessState+":") {
		t.Fatalf("fixed path: whole=%q kept=%v", fixed.Whole, fixed.Kept)
	}
	if control := analyzePackage(t, "w", lib("B")); control.Whole != "" || keptCodes(control)["TestOther"] != KeptReachesChange {
		t.Fatalf("fixed path negative control: whole=%q kept=%v", control.Whole, control.Kept)
	}
	base := []byte("package q\n\nimport impl \"m/one\"\n\nfunc F() int { return impl.V() }\n")
	head := []byte("package q\n\nimport impl \"m/two\"\n\nfunc F() int { return impl.V() }\n")
	if change := DiffFile("q.go", base, head); !slices.Equal(change.Added, []string{"m/two"}) || !slices.Equal(change.Removed, []string{"m/one"}) {
		t.Fatalf("import change: %+v", change)
	}
	renamed := []byte("package q\n\nimport other \"m/one\"\n\nfunc F() int { return other.V() }\n")
	if change := DiffFile("q.go", base, renamed); !slices.Equal(change.Mapped, []string{"m/one"}) || !slices.Contains(change.Names, "F") {
		t.Fatalf("renamed import: %+v", change)
	}
	blank := []byte("package q\n\nimport _ \"m/two\"\n\nfunc F() int { return 1 }\n")
	if change := DiffFile("q.go", []byte("package q\n\nfunc F() int { return 1 }\n"), blank); change.AlwaysRun == "" {
		t.Fatalf("blank import: %+v", change)
	}
}

// TestAFPV02374ReReviewRegressions holds the second review's cases: an
// environment reader taken as a value, a write through a copy of a struct
// holding a pointer, a bound sync.Once method value, a shared file named
// through a local variable, and a TestMain condition the parent runs.
func TestAFPV02374ReReviewRegressions(t *testing.T) {
	result := analyzePackage(t, "s", lib("A"))
	if result.Whole != "" {
		t.Fatalf("whole: %s", result.Whole)
	}
	for _, test := range []string{"TestEnvRead", "TestStructRead", "TestOnceRead", "TestFileRead"} {
		if keptCodes(result)[test] != KeptReadsSharedState {
			t.Fatalf("%s: kept=%v", test, result.Kept)
		}
	}
	// A file under the test's own temporary directory is private: when only
	// B changes, the readers of shared files are not kept through it.
	control := analyzePackage(t, "s", lib("B"))
	if keptCodes(control)["TestPrivateFile"] != KeptReachesChange || keptCodes(control)["TestFileRead"] != "" {
		t.Fatalf("private file control: whole=%q kept=%v", control.Whole, control.Kept)
	}
	if guard := analyzePackage(t, "g", lib("Guard")); !strings.HasPrefix(guard.Whole, WholeAlwaysRunReaches+":") {
		t.Fatalf("TestMain condition: whole=%q kept=%v", guard.Whole, guard.Kept)
	}
}

// TestAFPV02373RoundThreeRegressions holds the third review's cases: a
// constant built from a changed constant and a type whose length is one; a
// test starting a process through a Cmd made by new or declared as a
// variable; a process-starting test's unbounded writes reaching a test that
// reads a shared file; ioutil and os.Root file effects; and os.Stdout
// written by a kept test while a dropped test prints through fmt.
func TestAFPV02373RoundThreeRegressions(t *testing.T) {
	constant := analyzePackage(t, "t3", lib("CA"))
	if constant.Whole != "" || keptCodes(constant)["TestConst"] != KeptReachesChange || keptCodes(constant)["TestArr"] != KeptReachesChange {
		t.Fatalf("constant: whole=%q kept=%v", constant.Whole, constant.Kept)
	}
	child := analyzePackage(t, "t3", lib("Child"))
	if keptCodes(child)["TestSelf"] != KeptStartsProcess || keptCodes(child)["TestSpawnA"] != KeptStartsProcess {
		t.Fatalf("exec.Cmd by new or var: whole=%q kept=%v", child.Whole, child.Kept)
	}
	if keptCodes(child)["TestReadShared"] != KeptReadsSharedState {
		t.Fatalf("a started process writes files a reader sees: kept=%v", child.Kept)
	}
	a := analyzePackage(t, "t3", lib("A"))
	if keptCodes(a)["TestStdoutA"] != KeptReachesChange || keptCodes(a)["TestPrint"] != KeptReadsSharedState {
		t.Fatalf("os.Stdout and fmt: whole=%q kept=%v", a.Whole, a.Kept)
	}
	program, err := Load(filepath.Join("testdata", "mod"), fixtureModule, "t3")
	if err != nil {
		t.Fatal(err)
	}
	g := buildGraph(program)
	roots := map[string]*node{}
	for _, test := range g.tests {
		roots[test.name] = test.node
	}
	if n := roots["TestIoutil"]; n == nil || !n.writes[files] {
		t.Fatalf("ioutil.WriteFile of a shared path does not write files")
	}
	if n := roots["TestRoot"]; n == nil || !n.writes[files] || !n.reads[files] {
		t.Fatalf("os.OpenRoot does not read and write files")
	}
}

// TestAFPV02374RoundFourRegressions holds the fourth review's cases: a
// write through a type assertion on an interface-held reference, a path made
// "private" by an argument the helper discards, a write through a shared
// os.File handle, net/http's implicit defaults, and a fixed-port listener.
func TestAFPV02374RoundFourRegressions(t *testing.T) {
	result := analyzePackage(t, "t4", lib("A"))
	if result.Whole != "" {
		t.Fatalf("whole: %s", result.Whole)
	}
	for _, test := range []string{"TestIfaceRead", "TestFileReader", "TestHTTPRead", "TestListenB"} {
		if keptCodes(result)[test] != KeptReadsSharedState {
			t.Fatalf("%s: kept=%v", test, result.Kept)
		}
	}
	if keptCodes(result)["TestPure"] != "" {
		t.Fatalf("a test using only pure code is kept: %v", result.Kept)
	}
	program, err := Load(filepath.Join("testdata", "mod"), fixtureModule, "t4")
	if err != nil {
		t.Fatal(err)
	}
	g := buildGraph(program)
	roots := map[string]*node{}
	for _, test := range g.tests {
		roots[test.name] = test.node
	}
	written := func(n *node, name string) bool {
		for v := range n.writes {
			if v.Name() == name {
				return true
			}
		}
		return false
	}
	switch {
	case !written(roots["TestIfaceWrite"], "shared"):
		t.Fatal("a write through shared.(*box) is not a write of shared")
	case !roots["TestFakePrivateWrite"].writes[files]:
		t.Fatal("a path a helper builds while discarding t.TempDir() is treated as private")
	case roots["TestPrivateJoin"].writes[files]:
		t.Fatal("filepath.Join(t.TempDir(), \"x\") is not private")
	case len(roots["TestHandleWrite"].unknown) == 0:
		t.Fatal("(*os.File).Write is not an unmodelled standard-library use")
	case len(roots["TestHTTPRead"].unknown) == 0 || len(roots["TestListenB"].unknown) == 0:
		t.Fatal("net/http.Get or net.Listen is treated as pure")
	}
}

// TestAFPV02374AllowListRegressions holds the confirming review's cases
// against the standard-library allow-list: hidden environment reads,
// registries, runtime setters, file-loading parsers, location loading, the
// slog bridge, NotifyContext, interface dispatch to files, fmt values,
// symlinks and lock state. An in-memory Fprintf stays pure.
func TestAFPV02374AllowListRegressions(t *testing.T) {
	program, err := Load(filepath.Join("testdata", "mod"), fixtureModule, "t5")
	if err != nil {
		t.Fatal(err)
	}
	g := buildGraph(program)
	roots := map[string]*node{}
	for _, test := range g.tests {
		roots[test.name] = test.node
	}
	var stdout *types.Var
	for v := range roots["TestPrintValue"].reads {
		if v.Name() == "Stdout" {
			stdout = v
		}
	}
	var lock bool
	for v := range roots["TestLock"].writes {
		lock = lock || v.Name() == "mu"
	}
	for _, unknownUse := range []string{"TestZip", "TestCrypto", "TestRuntime", "TestTemplate", "TestParser", "TestLocation", "TestWriterInterface", "TestFprintFile", "TestSymlink"} {
		if len(roots[unknownUse].unknown) == 0 {
			t.Errorf("%s is treated as pure", unknownUse)
		}
	}
	switch {
	case !roots["TestHome"].reads[environment]:
		t.Error("os.UserHomeDir does not read the environment")
	case !roots["TestSlog"].writes[logDefault] || !roots["TestSlog"].writes[slogDefault]:
		t.Error("slog.SetDefault does not write both default loggers")
	case !roots["TestNotifyContext"].writes[signals]:
		t.Error("signal.NotifyContext does not write signal dispositions")
	case len(roots["TestFprintBuffer"].unknown) != 0:
		t.Errorf("Fprintln to a bytes.Buffer is unknown: %v", roots["TestFprintBuffer"].unknown)
	case stdout == nil:
		t.Error("fmt.Println taken as a value does not read os.Stdout")
	case !lock:
		t.Error("locking a package mutex is not a write of it")
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
