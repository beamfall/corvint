// SPDX-License-Identifier: Apache-2.0

package typedreach

import (
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// reviewFixture is also usable as an executable ordered-suite fault fixture.
func reviewFixture(t *testing.T, files map[string]string) string {
	t.Helper()
	root := t.TempDir()
	files["go.mod"] = "module example.com/review\n\ngo 1.27\n"
	for name, body := range files {
		full := filepath.Join(root, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return root
}

func TestAFPV02371RunnerConfigurationRefusesHiddenSource(t *testing.T) {
	for _, tc := range []struct{ name, constraint string }{
		{"race", "//go:build race\n\n"},
		{"custom tag", "//go:build runner_tag\n\n"},
		{"legacy tag", "// +build runner_tag\n\n"},
		{"legacy tab", "//\t+build runner_tag\n\n"},
		{"legacy spaces", "//  +build runner_tag\n\n"},
		{"cgo tag", "//go:build cgo\n\n"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := reviewFixture(t, map[string]string{
				"lib/lib.go":       "package lib\nfunc A() int { return 1 }\n",
				"p/p.go":           "package p\n",
				"p/p_test.go":      "package p\nimport \"testing\"\nfunc TestControl(t *testing.T) {}\n",
				"p/hidden_test.go": tc.constraint + "package p\nimport (\"testing\"; \"example.com/review/lib\")\nfunc TestHidden(t *testing.T) { if lib.A() != 0 { t.Fatal(\"changed dependency\") } }\n",
			})
			if _, err := Load(root, "example.com/review", "p"); !errors.Is(err, ErrUnmodelled) {
				t.Fatalf("hidden runner source must refuse narrowing: %v", err)
			}
		})
	}
	t.Run("cgo dependency", func(t *testing.T) {
		root := reviewFixture(t, map[string]string{
			"p/p.go":   "package p\n",
			"p/cgo.go": "package p\nimport \"C\"\n",
		})
		if _, err := Load(root, "example.com/review", "p"); !errors.Is(err, ErrUnmodelled) {
			t.Fatalf("cgo must refuse narrowing: %v", err)
		}
	})
	for _, tc := range []struct{ name, file, source string }{
		{"dependency", "lib/race.go", "package lib\nfunc Hidden() int { return 2 }\n"},
		{"external test", "p/race_test.go", "package p_test\nimport \"testing\"\nfunc TestHidden(t *testing.T) {}\n"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := reviewFixture(t, map[string]string{
				"lib/lib.go":  "package lib\nfunc A() int { return 1 }\n",
				"p/p_test.go": "package p\nimport (\"testing\"; \"example.com/review/lib\")\nfunc TestA(t *testing.T) { lib.A() }\nfunc TestControl(t *testing.T) {}\n",
				tc.file:       "//go:build race\n\n" + tc.source,
			})
			if _, err := Load(root, "example.com/review", "p"); err == nil || !strings.Contains(err.Error(), "runner configuration parity unproved") {
				t.Fatalf("hidden %s must refuse narrowing: %v", tc.name, err)
			}
			if err := os.Remove(filepath.Join(root, tc.file)); err != nil {
				t.Fatal(err)
			}
			program, err := Load(root, "example.com/review", "p")
			if err != nil {
				t.Fatal(err)
			}
			result := Analyze(program, []Declaration{{Package: "example.com/review/lib", Name: "A"}}, nil)
			if result.Whole != "" || !slices.Equal(result.Dropped, []string{"TestControl"}) {
				t.Fatalf("unconstrained control: %+v", result)
			}
		})
	}
}

func TestAFPV02376MovedPositionsKeepDependentReaders(t *testing.T) {
	base := "package lib\nimport \"runtime\"\nfunc A() int { return 0 }\nfunc B() int { _, _, line, _ := runtime.Caller(0); return line }\n"
	for _, tc := range []struct{ name, head string }{
		{"lines", strings.Replace(base, "return 0", "\n\nreturn 1", 1)},
		{"columns", strings.Replace(base, "func B", "  func B", 1)},
		{"line directive", strings.Replace(base, "func B", "//line virtual.go:100\nfunc B", 1)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := reviewFixture(t, map[string]string{
				"lib/lib.go":  tc.head,
				"p/p_test.go": "package p\nimport (\"testing\"; \"example.com/review/lib\")\nfunc TestA(t *testing.T) { lib.A() }\nfunc TestB(t *testing.T) { if lib.B() != 4 { t.Fatal(lib.B()) } }\nfunc TestControl(t *testing.T) {}\n",
			})
			change := DiffFile("lib/lib.go", []byte(base), []byte(tc.head))
			if !slices.Contains(change.Names, "B") {
				t.Fatalf("moved B absent: %+v", change)
			}
			program, err := Load(root, "example.com/review", "p")
			if err != nil {
				t.Fatal(err)
			}
			result := Analyze(program, change.Declarations("example.com/review/lib"), nil)
			if result.Whole != "" || keptCodes(result)["TestB"] != KeptReachesChange || !slices.Contains(result.Dropped, "TestControl") {
				t.Fatalf("moved dependency: %+v", result)
			}
		})
	}
}

func TestAFPV02374LoggerReceiverSettersKeepOrderedReaders(t *testing.T) {
	for _, setter := range []string{
		"log.Default().SetPrefix(strconv.Itoa(lib.A()))",
		"set := log.Default().SetPrefix; set(strconv.Itoa(lib.A()))",
		"(*log.Logger).SetPrefix(log.Default(), strconv.Itoa(lib.A()))",
		"log.Default().SetFlags(lib.A())",
		"log.Default().SetOutput(io.Discard); _ = lib.A()",
	} {
		t.Run(setter, func(t *testing.T) {
			root := reviewFixture(t, map[string]string{
				"lib/lib.go":  "package lib\nfunc A() int { return 1 }\n",
				"p/p_test.go": "package p\nimport (\"testing\"; \"log\"; \"strconv\"; \"io\"; \"example.com/review/lib\")\nvar _ = strconv.Itoa\nvar _ = io.Discard\nfunc TestAWriter(t *testing.T) { " + setter + " }\nfunc TestBReader(t *testing.T) { if log.Prefix() != \"0\" { t.Fatal(log.Prefix()) } }\nfunc TestControl(t *testing.T) {}\n",
			})
			program, err := Load(root, "example.com/review", "p")
			if err != nil {
				t.Fatal(err)
			}
			result := Analyze(program, []Declaration{{Package: "example.com/review/lib", Name: "A"}}, nil)
			if result.Whole != "" || keptCodes(result)["TestBReader"] != KeptReadsSharedState || !slices.Contains(result.Dropped, "TestControl") {
				t.Fatalf("logger mutation: %+v", result)
			}
		})
	}
}

func TestAFPV02374SignalCleanupCannotEraseWrites(t *testing.T) {
	for _, cleanup := range []string{
		"signal.Stop(make(chan os.Signal, 1))",
		"if lib.A() == 0 { signal.Stop(ch) }",
		"if lib.A() != 0 { return }; signal.Stop(ch)",
		"defer signal.Stop(ch)",
	} {
		t.Run(cleanup, func(t *testing.T) {
			root := reviewFixture(t, map[string]string{
				"lib/lib.go":  "package lib\nfunc A() int { return 1 }\n",
				"p/p_test.go": "package p\nimport (\"testing\"; \"os\"; \"os/signal\"; \"example.com/review/lib\")\nfunc TestMain(m *testing.M) { signal.Ignore(os.Interrupt); code := m.Run(); signal.Reset(os.Interrupt); os.Exit(code) }\nfunc TestAWriter(t *testing.T) { ch := make(chan os.Signal, 1); if lib.A() != 0 { signal.Notify(ch, []os.Signal{os.Interrupt}...) }; " + cleanup + " }\nfunc TestBReader(t *testing.T) { if !signal.Ignored(os.Interrupt) { t.Fatal(\"signal disposition changed\") } }\nfunc TestControl(t *testing.T) {}\n",
			})
			program, err := Load(root, "example.com/review", "p")
			if err != nil {
				t.Fatal(err)
			}
			result := Analyze(program, []Declaration{{Package: "example.com/review/lib", Name: "A"}}, nil)
			if result.Whole != "" || keptCodes(result)["TestBReader"] != KeptReadsSharedState || !slices.Contains(result.Dropped, "TestControl") {
				t.Fatalf("unproved signal cleanup: %+v", result)
			}
		})
	}
}

func TestAFPV02374StoredMethodValuesCarryImplicitReads(t *testing.T) {
	for _, tc := range []struct{ name, holder, read string }{
		{"bound method value", "var prefix = log.Default().Prefix", "prefix()"},
		{"method expression", "var prefix = (*log.Logger).Prefix\nvar logger = log.Default()", "prefix(logger)"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := reviewFixture(t, map[string]string{
				"lib/lib.go":  "package lib\nfunc A() int { return 1 }\n",
				"p/p_test.go": "package p\nimport (\"testing\"; \"log\"; \"strconv\"; \"example.com/review/lib\")\n" + tc.holder + "\nfunc TestAWriter(t *testing.T) { log.SetPrefix(strconv.Itoa(lib.A())) }\nfunc TestBReader(t *testing.T) { if " + tc.read + " != \"1\" { t.Fatal(" + tc.read + ") } }\nfunc TestControl(t *testing.T) {}\n",
			})
			program, err := Load(root, "example.com/review", "p")
			if err != nil {
				t.Fatal(err)
			}
			result := Analyze(program, []Declaration{{Package: "example.com/review/lib", Name: "A"}}, nil)
			if result.Whole != "" || keptCodes(result)["TestBReader"] != KeptReadsSharedState || !slices.Contains(result.Dropped, "TestControl") {
				t.Fatalf("stored logger method value: %+v", result)
			}
		})
	}
}

func TestAFPV02374SignalStopWritesSignalState(t *testing.T) {
	for _, cleanup := range []string{
		"if lib.A() != 0 { signal.Stop(ch) }",
		"stop := signal.Stop; if lib.A() != 0 { stop(ch) }",
	} {
		t.Run(cleanup, func(t *testing.T) {
			root := reviewFixture(t, map[string]string{
				"lib/lib.go":  "package lib\nfunc A() int { return 1 }\n",
				"p/p_test.go": "package p\nimport (\"testing\"; \"os\"; \"os/signal\"; \"syscall\"; \"example.com/review/lib\")\nvar ch = make(chan os.Signal, 1)\nfunc TestMain(m *testing.M) { signal.Notify(ch, syscall.SIGTERM); os.Exit(m.Run()) }\nfunc TestAStop(t *testing.T) { " + cleanup + " }\nfunc TestBSignal(t *testing.T) { self, _ := os.FindProcess(os.Getpid()); if err := self.Signal(syscall.SIGTERM); err != nil { t.Fatal(err) } }\nfunc TestControl(t *testing.T) {}\n",
			})
			program, err := Load(root, "example.com/review", "p")
			if err != nil {
				t.Fatal(err)
			}
			result := Analyze(program, []Declaration{{Package: "example.com/review/lib", Name: "A"}}, nil)
			if result.Whole != "" || keptCodes(result)["TestBSignal"] != KeptReadsSharedState || !slices.Contains(result.Dropped, "TestControl") {
				t.Fatalf("signal.Stop must write signal dispositions: %+v", result)
			}
		})
	}
}

// streamFixture redirects the process streams and the default logger to one
// file that TestBReader checks after TestAWrite has written lib.A's value to
// it: run as a suite, a changed lib.A makes TestBReader fail.
func streamFixture(t *testing.T, imports, holder, write string) *Result {
	t.Helper()
	root := reviewFixture(t, map[string]string{
		"lib/lib.go": "package lib\nfunc A() int { return 1 }\n",
		"p/p_test.go": "package p\nimport (\"testing\"; \"log\"; \"os\"; \"path/filepath\"; \"strings\"; \"example.com/review/lib\"" + imports + ")\n" + holder + "\nvar out string\n" +
			"func TestMain(m *testing.M) { dir, _ := os.MkdirTemp(\"\", \"review\"); out = filepath.Join(dir, \"out\"); f, _ := os.Create(out); os.Stdout = f; log.SetOutput(f); code := m.Run(); f.Close(); os.RemoveAll(dir); os.Exit(code) }\n" +
			"func TestAWrite(t *testing.T) { " + write + " }\n" +
			"func TestBReader(t *testing.T) { b, err := os.ReadFile(out); if err != nil || !strings.Contains(string(b), \"value=1\") { t.Fatalf(\"%q %v\", b, err) } }\n" +
			"func TestControl(t *testing.T) {}\n",
	})
	program, err := Load(root, "example.com/review", "p")
	if err != nil {
		t.Fatal(err)
	}
	result := Analyze(program, []Declaration{{Package: "example.com/review/lib", Name: "A"}}, nil)
	return &result
}

func TestAFPV02374StreamOutputWritesFiles(t *testing.T) {
	for _, tc := range []struct{ name, imports, write string }{
		{"fmt.Print", "; \"fmt\"", "fmt.Print(\"value=\", lib.A())"},
		{"fmt.Println value", "; \"fmt\"; \"strconv\"", "p := fmt.Println; p(\"value=\" + strconv.Itoa(lib.A()))"},
		{"log.Print", "", "log.Print(\"value=\", lib.A())"},
		{"logger method", "", "log.Default().Printf(\"value=%d\", lib.A())"},
		{"logger method value", "", "p := log.Default().Printf; p(\"value=%d\", lib.A())"},
		{"slog.Info", "; \"log/slog\"; \"strconv\"", "slog.Info(\"value=\" + strconv.Itoa(lib.A()))"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			result := streamFixture(t, tc.imports, "", tc.write)
			if result.Whole != "" || keptCodes(*result)["TestBReader"] != KeptReadsSharedState || !slices.Contains(result.Dropped, "TestControl") {
				t.Fatalf("stream output must write the file system: %+v", *result)
			}
		})
	}
}

func TestAFPV02374IndirectStdCallsKeepOpaqueArguments(t *testing.T) {
	for _, tc := range []struct{ name, holder, write string }{
		{"direct io.Copy", "", "io.Copy(os.Stdout, strings.NewReader(\"value=\" + strconv.Itoa(lib.A())))"},
		{"local io.Copy value", "", "copyFn := io.Copy; copyFn(os.Stdout, strings.NewReader(\"value=\" + strconv.Itoa(lib.A())))"},
		{"package io.Copy value", "var copyFn = io.Copy", "copyFn(os.Stdout, strings.NewReader(\"value=\" + strconv.Itoa(lib.A())))"},
		{"direct WriteTo", "", "strings.NewReader(\"value=\" + strconv.Itoa(lib.A())).WriteTo(os.Stdout)"},
		{"WriteTo method value", "", "w := strings.NewReader(\"value=\" + strconv.Itoa(lib.A())).WriteTo; w(os.Stdout)"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			result := streamFixture(t, "; \"io\"; \"strconv\"", tc.holder+"\nvar _ = io.EOF", tc.write)
			if slices.Contains(result.Dropped, "TestBReader") {
				t.Fatalf("an indirect call must keep the opaque-argument check: %+v", *result)
			}
		})
	}
}

func TestAFPV02376ImportPathOfRootPackageFiles(t *testing.T) {
	for file, want := range map[string]string{
		"lib.go":         "example.com/review",
		"lib/lib.go":     "example.com/review/lib",
		"a/b/c/thing.go": "example.com/review/a/b/c",
	} {
		if got := ImportPath("example.com/review", file); got != want {
			t.Fatalf("ImportPath(%q) = %q, want %q", file, got, want)
		}
	}
}

func TestAFPV02374RangeAssignmentEndsPrivacy(t *testing.T) {
	for name, loop := range map[string]string{
		"value": "for _, p = range []string{shared} {}",
		"key":   "for p = range map[string]int{shared: 0} {}",
		// A target that is not a bare identifier is not modelled, so it
		// ends the privacy of every variable in the body.
		"parenthesized value":      "for _, (p) = range []string{shared} {}",
		"parenthesized key":        "for (p) = range map[string]int{shared: 0} {}",
		"parenthesized assignment": "(p) = shared",
	} {
		t.Run(name, func(t *testing.T) {
			root := reviewFixture(t, map[string]string{
				"lib/lib.go": "package lib\nfunc A() int { return 1 }\n",
				"p/p_test.go": "package p\nimport (\"testing\"; \"os\"; \"path/filepath\"; \"strconv\"; \"strings\"; \"example.com/review/lib\")\nvar shared string\n" +
					"func TestMain(m *testing.M) { dir, _ := os.MkdirTemp(\"\", \"review\"); shared = filepath.Join(dir, \"shared\"); os.WriteFile(shared, []byte(\"value=1\"), 0o600); code := m.Run(); os.RemoveAll(dir); os.Exit(code) }\n" +
					"func TestAWrite(t *testing.T) { p := t.TempDir(); " + loop + "; os.WriteFile(p, []byte(\"value=\" + strconv.Itoa(lib.A())), 0o600) }\n" +
					"func TestBReader(t *testing.T) { b, err := os.ReadFile(shared); if err != nil || !strings.Contains(string(b), \"value=1\") { t.Fatalf(\"%q %v\", b, err) } }\n" +
					"func TestControl(t *testing.T) {}\n",
			})
			program, err := Load(root, "example.com/review", "p")
			if err != nil {
				t.Fatal(err)
			}
			result := Analyze(program, []Declaration{{Package: "example.com/review/lib", Name: "A"}}, nil)
			if result.Whole != "" || keptCodes(result)["TestBReader"] != KeptReadsSharedState || !slices.Contains(result.Dropped, "TestControl") {
				t.Fatalf("a range assignment must end the temporary path's privacy: %+v", result)
			}
		})
	}
}

func TestAFPV02374ScanReadsFiles(t *testing.T) {
	for name, scan := range map[string]string{
		"fmt.Scan":         "fmt.Scan(&v)",
		"fmt.Scanf":        "fmt.Scanf(\"%d\", &v)",
		"fmt.Scanln value": "s := fmt.Scanln; s(&v)",
		// Fscan* on a stream needed no change: its io.Reader argument is
		// opaque, so the reader stays kept as unknown.
		"fmt.Fscan stdin guard": "fmt.Fscan(os.Stdin, &v)",
	} {
		t.Run(name, func(t *testing.T) {
			root := reviewFixture(t, map[string]string{
				"lib/lib.go": "package lib\nfunc A() int { return 1 }\n",
				"p/p_test.go": "package p\nimport (\"testing\"; \"fmt\"; \"os\"; \"path/filepath\"; \"strconv\"; \"example.com/review/lib\")\nvar in string\n" +
					"func TestMain(m *testing.M) { dir, _ := os.MkdirTemp(\"\", \"review\"); in = filepath.Join(dir, \"in\"); os.WriteFile(in, []byte(\"1\\n\"), 0o600); f, _ := os.Open(in); os.Stdin = f; code := m.Run(); f.Close(); os.RemoveAll(dir); os.Exit(code) }\n" +
					"func TestAWrite(t *testing.T) { os.WriteFile(in, []byte(strconv.Itoa(lib.A()) + \"\\n\"), 0o600) }\n" +
					"func TestBReader(t *testing.T) { var v int; " + scan + "; if v != 1 { t.Fatal(v) } }\n" +
					"func TestControl(t *testing.T) {}\n",
			})
			program, err := Load(root, "example.com/review", "p")
			if err != nil {
				t.Fatal(err)
			}
			result := Analyze(program, []Declaration{{Package: "example.com/review/lib", Name: "A"}}, nil)
			want := keptCodes(result)["TestBReader"] == KeptReadsSharedState
			if strings.HasPrefix(name, "fmt.Fscan") {
				want = !slices.Contains(result.Dropped, "TestBReader")
			}
			if result.Whole != "" || !want || !slices.Contains(result.Dropped, "TestControl") {
				t.Fatalf("a scan from stdin must read the file system: %+v", result)
			}
		})
	}
}

// TestAFPV02376BaseEffectsSeeHeldCallbacks is the fault-injection regression
// for a write removed from code a changed package variable holds: at head the
// callback Hook no longer writes State, so head-only reach keeps the test
// calling Hook and drops the test reading State, which then fails after it at
// head. The base check must see the held write, in a function literal or
// through a stored function or method value, and keep the package whole; a
// pure base callback stays narrowable. A held literal that calls is already an
// initialization change.
func TestAFPV02376BaseEffectsSeeHeldCallbacks(t *testing.T) {
	const head = "package lib\nvar State int\nfunc set() { State = 1 }\ntype counter struct{}\nfunc (counter) bump() { State = 1 }\nvar Hook = func() {}\n"
	const tests = "package p\nimport (\"testing\"; \"example.com/review/lib\")\nfunc TestAHook(t *testing.T) { lib.Hook() }\nfunc TestBState(t *testing.T) { if lib.State != 1 { t.Fatal(lib.State) } }\nfunc TestControl(t *testing.T) {}\n"
	for _, tc := range []struct {
		name, base             string
		writes, initialization bool
	}{
		{"function literal", strings.Replace(head, "Hook = func() {}", "Hook = func() { State = 1 }", 1), true, false},
		{"function value", strings.Replace(head, "Hook = func() {}", "Hook = set", 1), true, false},
		{"method value", strings.Replace(head, "Hook = func() {}", "Hook = counter{}.bump", 1), true, false},
		{"pure callback", strings.Replace(head, "Hook = func() {}", "Hook = func() { _ = State }", 1), false, false},
		{"literal calling a writer", strings.Replace(head, "Hook = func() {}", "Hook = func() { set() }", 1), true, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			change := DiffFile("lib/lib.go", []byte(tc.base), []byte(head))
			if tc.initialization {
				if change.AlwaysRun == "" {
					t.Fatalf("a held literal that calls is not an initialization change: %+v", change)
				}
				return
			}
			if change.AlwaysRun != "" || !slices.Equal(change.Names, []string{"Hook"}) {
				t.Fatalf("changed declarations: %+v", change)
			}
			declarations := change.Declarations("example.com/review/lib")
			root := reviewFixture(t, map[string]string{"lib/lib.go": head, "p/p_test.go": tests})
			program, err := Load(root, "example.com/review", "p")
			if err != nil {
				t.Fatal(err)
			}
			// Head-only reach cannot see the removed write: the reader is dropped.
			result := Analyze(program, declarations, nil)
			if result.Whole != "" || keptCodes(result)["TestAHook"] != KeptReachesChange || !slices.Contains(result.Dropped, "TestBState") {
				t.Fatalf("head reach: %+v", result)
			}
			before, err := LoadBase(root, "example.com/review", "p", map[string][]byte{"lib/lib.go": []byte(tc.base)})
			if err != nil {
				t.Fatal(err)
			}
			why := BaseEffects(before, declarations)
			if (why != "") != tc.writes || why != "" && (!strings.HasPrefix(why, WholeBaseWrites+": ") || !strings.Contains(why, "Hook")) {
				t.Fatalf("base effects %q, want writes=%v", why, tc.writes)
			}
		})
	}
}

// TestAFPV02376BlankInitializersKeepOwnKeys is the regression for blank
// declarations sharing one key: an edit of an earlier `var _ = setup(...)`
// that moves no position was hidden behind the last blank declaration, so an
// ordinary edit in the same change narrowed the package and dropped the test
// reading the state the initializers set. Each blank declaration is its own
// occurrence with its own initialization status, so a blank constant after it
// cannot hide it either; the ordinary edit alone still narrows.
func TestAFPV02376BlankInitializersKeepOwnKeys(t *testing.T) {
	const base = "package lib\nvar State int\nfunc setup(v int) int { State += v; return 0 }\nvar _ = setup(1)\nvar _ = setup(2)\nvar _ = setup(4)\nconst _ = 0\nfunc Pure() int { return 1 }\n"
	const tests = "package p\nimport (\"testing\"; \"example.com/review/lib\")\nfunc TestPure(t *testing.T) { if lib.Pure() != 2 { t.Fatal(lib.Pure()) } }\nfunc TestState(t *testing.T) { if lib.State != 7 { t.Fatal(lib.State) } }\nfunc TestControl(t *testing.T) {}\n"
	pure := strings.Replace(base, "return 1 }", "return 2 }", 1)
	load := func(t *testing.T, head string) *Program {
		t.Helper()
		program, err := Load(reviewFixture(t, map[string]string{"lib/lib.go": head, "p/p_test.go": tests}), "example.com/review", "p")
		if err != nil {
			t.Fatal(err)
		}
		return program
	}
	for _, tc := range []struct{ name, from, to string }{
		{"first", "setup(1)", "setup(8)"},
		{"middle", "setup(2)", "setup(9)"},
		{"last before a blank constant", "setup(4)", "setup(5)"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			head := strings.Replace(pure, tc.from, tc.to, 1)
			change := DiffFile("lib/lib.go", []byte(base), []byte(head))
			if change.AlwaysRun == "" {
				t.Fatalf("blank initializer edit hidden: %+v", change)
			}
			result := Analyze(load(t, head), change.Declarations("example.com/review/lib"), nil)
			if !strings.HasPrefix(result.Whole, WholeAlwaysRunReaches+": ") {
				t.Fatalf("changed blank initializer must keep the package whole: %+v", result)
			}
		})
	}
	t.Run("ordinary edit alone", func(t *testing.T) {
		change := DiffFile("lib/lib.go", []byte(base), []byte(pure))
		if change.AlwaysRun != "" || !slices.Equal(change.Names, []string{"Pure"}) {
			t.Fatalf("unchanged blank declarations reported: %+v", change)
		}
		result := Analyze(load(t, pure), change.Declarations("example.com/review/lib"), nil)
		if result.Whole != "" || keptCodes(result)["TestPure"] != KeptReachesChange || !slices.Equal(result.Dropped, []string{"TestControl", "TestState"}) {
			t.Fatalf("negative control: %+v", result)
		}
	})
}

// TestAFPV02374SetupWritersOfKeptReadsAreKept is the fault-injection
// regression for a dropped setup test: unchanged TestASetup sets the package
// flag enabled, and TestBCheck calls the changed dependency only when enabled
// is true. The head breaks lib.Check, so the whole package fails at TestBCheck,
// but a run of TestBCheck alone never reaches the broken branch and passes.
// The changed dependency writes no state, so the base check admits narrowing;
// a test that may write state a kept test reads must therefore be kept, to a
// fixpoint over its own reads, and a writer of state no kept test reads stays
// dropped.
func TestAFPV02374SetupWritersOfKeptReadsAreKept(t *testing.T) {
	const base = "package lib\nfunc Check() int { return 1 }\n"
	const head = "package lib\nfunc Check() int { return 0 }\n"
	const check = "func TestBCheck(t *testing.T) { if enabled && lib.Check() != 1 { t.Fatal(\"changed dependency\") } }\nfunc TestControl(t *testing.T) {}\nfunc TestOther(t *testing.T) { other = 1 }\n"
	const prelude = "package p\nimport (\"testing\"; \"example.com/review/lib\")\nvar enabled bool\nvar mode, other int\n"
	for _, tc := range []struct {
		name, setup string
		want        map[string]string
	}{
		{"setup flag", "func TestASetup(t *testing.T) { enabled = true }\n",
			map[string]string{"TestASetup": "example.com/review/p.enabled"}},
		{"setup helper", "func enable() { enabled = true }\nfunc TestASetup(t *testing.T) { enable() }\n",
			map[string]string{"TestASetup": "example.com/review/p.enabled"}},
		{"range value setup", "func TestASetup(t *testing.T) { for _, enabled = range []bool{true} {} }\n",
			map[string]string{"TestASetup": "example.com/review/p.enabled"}},
		{"range key setup", "func TestASetup(t *testing.T) { for enabled = range map[bool]int{true: 0} {} }\n",
			map[string]string{"TestASetup": "example.com/review/p.enabled"}},
		{"transitive setup", "func TestA0Mode(t *testing.T) { mode = 1 }\nfunc TestASetup(t *testing.T) { enabled = mode == 1 }\n",
			map[string]string{"TestA0Mode": "example.com/review/p.mode", "TestASetup": "example.com/review/p.enabled"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			change := DiffFile("lib/lib.go", []byte(base), []byte(head))
			if change.AlwaysRun != "" || !slices.Equal(change.Names, []string{"Check"}) {
				t.Fatalf("changed declarations: %+v", change)
			}
			declarations := change.Declarations("example.com/review/lib")
			root := reviewFixture(t, map[string]string{"lib/lib.go": head, "p/p_test.go": prelude + tc.setup + check})
			before, err := LoadBase(root, "example.com/review", "p", map[string][]byte{"lib/lib.go": []byte(base)})
			if err != nil {
				t.Fatal(err)
			}
			if why := BaseEffects(before, declarations); why != "" {
				t.Fatalf("the changed dependency writes no state, so the base check must admit narrowing: %q", why)
			}
			program, err := Load(root, "example.com/review", "p")
			if err != nil {
				t.Fatal(err)
			}
			result := Analyze(program, declarations, nil)
			if result.Whole != "" || keptCodes(result)["TestBCheck"] != KeptReachesChange || !slices.Equal(result.Dropped, []string{"TestControl", "TestOther"}) {
				t.Fatalf("setup writers of a kept test's reads must be kept: %+v", result)
			}
			for test, state := range tc.want {
				if result.Kept[test] != KeptWritesSharedState+": "+state {
					t.Fatalf("%s: got %q, want %s on %s: %+v", test, result.Kept[test], KeptWritesSharedState, state, result)
				}
			}
		})
	}
	// A processState setter is not tracked per reader: setup that changes
	// the working directory a kept test resolves a relative path against
	// keeps the package whole, as does setup writing a fixed path a kept test
	// reads; with no test kept, both stay dropped.
	for _, tc := range []struct{ name, setup, check, want string }{
		{"process-wide setup", "os.Chdir(\"testdata\")", "os.Stat(\"enabled\")", "calls os.Chdir"},
		{"fixed-path setup", "os.WriteFile(\"enabled\", nil, 0o600)", "os.Stat(\"enabled\")", "os.WriteFile of the fixed path"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			tests := "package p\nimport (\"os\"; \"testing\"; \"example.com/review/lib\")\nfunc TestASetup(t *testing.T) { " + tc.setup + " }\nfunc TestBCheck(t *testing.T) { if _, err := " + tc.check + "; err == nil && lib.Check() != 1 { t.Fatal(\"changed dependency\") } }\nfunc TestControl(t *testing.T) {}\n"
			program, err := Load(reviewFixture(t, map[string]string{"lib/lib.go": head + "func Unused() int { return 0 }\n", "p/p_test.go": tests}), "example.com/review", "p")
			if err != nil {
				t.Fatal(err)
			}
			if result := Analyze(program, []Declaration{{Package: "example.com/review/lib", Name: "Check"}}, nil); !strings.HasPrefix(result.Whole, WholeProcessState+": TestASetup: ") || !strings.Contains(result.Whole, tc.want) {
				t.Fatalf("setup writing state a kept test reads must keep the package whole: %+v", result)
			}
			if result := Analyze(program, []Declaration{{Package: "example.com/review/lib", Name: "Unused"}}, nil); result.Whole != "" || len(result.Kept) != 0 || !slices.Equal(result.Dropped, []string{"TestASetup", "TestBCheck", "TestControl"}) {
				t.Fatalf("setup with no kept test: %+v", result)
			}
		})
	}
}
