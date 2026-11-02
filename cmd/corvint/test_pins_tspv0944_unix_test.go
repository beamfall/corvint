//go:build darwin || linux

package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TSP-V0-944: --dry-run --pins names, per selected package, the tests whose
// bodies or helpers name a changed declaration or quote a changed literal.
// TestRefuse names it, TestViaHelper reaches it through a helper and
// TestStartupRefusals in another package quotes its message; the unrelated
// tests are not pinned and the flag refuses outside a plain dry run (V1-3808).
func TestTSPV0944DryRunPinsNamesPinnedTests(t *testing.T) {
	root, _ := sourceSliceFixture(t)
	fixture := map[string]string{
		"s/s.go": "package s\n\nimport \"errors\"\n\n" +
			"func Refuse() error { return errors.New(\"startup refused: no store\") }\n\n" +
			"func Other() string { return \"an unchanged message text\" }\n",
		"s/s_test.go": "package s\n\nimport \"testing\"\n\n" +
			"func TestRefuse(t *testing.T) { if Refuse() == nil { t.Fatal() } }\n\n" +
			"func check(t *testing.T) { if Refuse() == nil { t.Fatal() } }\n\n" +
			"func TestViaHelper(t *testing.T) { check(t) }\n\n" +
			"func TestUnrelated(t *testing.T) { _ = Other() }\n",
		"c/c.go": "package c\n\nimport \"example.test/m/s\"\n\nfunc Start() error { return s.Refuse() }\n",
		"c/c_test.go": "package c\n\nimport (\n\t\"strings\"\n\t\"testing\"\n)\n\n" +
			"func TestStartupRefusals(t *testing.T) {\n\tif !strings.Contains(Start().Error(), \"refused: no store\") {\n\t\tt.Fatal()\n\t}\n}\n\n" +
			"func TestCOther(t *testing.T) { if strings.Contains(\"an unchanged message text\", \"x\") { t.Fatal() } }\n",
	}
	for name, body := range fixture {
		if err := os.MkdirAll(filepath.Dir(filepath.Join(root, name)), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(root, name), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	affectedGit(t, root, "add", ".")
	affectedGit(t, root, "commit", "-qm", "pins")
	base := strings.TrimSpace(affectedGit(t, root, "rev-parse", "HEAD"))
	edited := strings.Replace(fixture["s/s.go"], "no store\"", "no store found\"", 1)
	if err := os.WriteFile(filepath.Join(root, "s/s.go"), []byte(edited), 0o644); err != nil {
		t.Fatal(err)
	}
	manifest := filepath.Join(t.TempDir(), "m.json")
	code, stdout, stderr := runCLI(t, "--root", root, "test", "--base", base, "--dry-run", "--pins", "--manifest", manifest)
	if code != 0 {
		t.Fatalf("code %d stderr %q", code, stderr)
	}
	for _, want := range []string{
		"corvint-test: PINS manifest=",
		"pins go:example.test/m/s -run '^(TestRefuse|TestViaHelper)$'\n",
		"pins go:example.test/m/c -run '^(TestStartupRefusals)$'\n",
		"advisory: run the pinned tests first",
	} {
		if !strings.Contains(stdout, want) {
			t.Fatalf("stdout lacks %q:\n%s", want, stdout)
		}
	}
	for _, unwanted := range []string{"TestUnrelated", "TestCOther", "TestQ", "{"} {
		if strings.Contains(stdout, unwanted) {
			t.Fatalf("stdout holds %q:\n%s", unwanted, stdout)
		}
	}
	for _, args := range [][]string{
		{"--root", root, "test", "--base", base, "--pins", "--manifest", manifest},
		{"--root", root, "test", "--base", base, "--dry-run", "--summary", "--pins", "--manifest", manifest},
	} {
		if code, _, stderr := runCLI(t, args...); code == 0 || !strings.Contains(stderr, "TSP-V0-944") {
			t.Fatalf("%v: code %d stderr %q", args, code, stderr)
		}
	}
}
