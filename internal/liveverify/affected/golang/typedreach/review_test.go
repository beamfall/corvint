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
