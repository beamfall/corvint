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
