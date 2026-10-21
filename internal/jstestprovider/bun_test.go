package jstestprovider

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"
)

// TestLPCVV0260_ParseBunJUnitLiveReport parses a report bun 1.3.11 wrote for a describe block with
// a passing, a failing and a skipped test plus a preloaded integration file (LPCV-V0-260).
func TestLPCVV0260_ParseBunJUnitLiveReport(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("testdata", "bun-1.3.11-junit.xml"))
	if err != nil {
		t.Fatal(err)
	}
	tests, infra, err := ParseBunJUnit(data)
	if err != nil || infra != nil {
		t.Fatalf("ParseBunJUnit: %v %+v", err, infra)
	}
	type row struct {
		full  string
		state ExecutionState
		file  string
		line  int
	}
	var got []row
	for _, o := range tests {
		got = append(got, row{o.FullName, o.State, o.Anchor.File, o.Anchor.Line})
	}
	want := []row{
		{"math > adds", StatePassed, "test/unit/math.test.ts", 4},
		{"math > fails", StateFailed, "test/unit/math.test.ts", 8},
		{"math > skipped", StateSkipped, "test/unit/math.test.ts", 11},
		{"db ready", StatePassed, "test/integration/db.test.ts", 3},
	}
	if !slices.Equal(got, want) {
		t.Fatalf("outcomes = %+v, want %+v", got, want)
	}
	if tests[1].FailureMessage != "AssertionError" || tests[0].Name != "adds" {
		t.Fatalf("failure/name = %q %q", tests[1].FailureMessage, tests[0].Name)
	}
}

// TestLPCVV0260_ParseBunJUnitHookAndShape: a failing beforeAll hook is one infrastructure outcome,
// never a test result; a count mismatch and an empty report are run-level failures.
func TestLPCVV0260_ParseBunJUnitHookAndShape(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("testdata", "bun-1.3.11-hook-junit.xml"))
	if err != nil {
		t.Fatal(err)
	}
	tests, infra, err := ParseBunJUnit(data)
	if err != nil || infra != nil || len(tests) != 1 {
		t.Fatalf("hook: %v %+v %+v", err, infra, tests)
	}
	if tests[0].State != StateInfrastructure || tests[0].FullName != "db > (hook)" || tests[0].Anchor != nil {
		t.Fatalf("hook outcome = %+v", tests[0])
	}
	mismatch := `<testsuites tests="2"><testsuite name="a.test.ts" file="a.test.ts" tests="2"><testcase name="x" file="a.test.ts" line="1"/></testsuite></testsuites>`
	if _, infra, err := ParseBunJUnit([]byte(mismatch)); err != nil || infra == nil || infra.Reason != "report-count-mismatch" {
		t.Fatalf("mismatch: %v %+v", err, infra)
	}
	if _, infra, err := ParseBunJUnit([]byte(`<testsuites tests="0"></testsuites>`)); err != nil || infra == nil || infra.Reason != "no-suites-collected" {
		t.Fatalf("empty: %v %+v", err, infra)
	}
	if _, _, err := ParseBunJUnit([]byte(`<html></html>`)); err == nil {
		t.Fatal("a non-JUnit document parsed")
	}
}

// fakeBun writes an executable that reports version and, for `test`, records its argv and copies
// report into the --reporter-outfile path, exiting exit.
func fakeBun(t *testing.T, dir, version, report string, exit int) (string, string) {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("fake bun is a POSIX shell script")
	}
	argsFile := filepath.Join(dir, "argv.txt")
	script := fmt.Sprintf(`#!/bin/sh
if [ "$1" = "--version" ]; then echo %q; exit 0; fi
for a in "$@"; do case "$a" in --reporter-outfile=*) out="${a#--reporter-outfile=}";; esac; done
printf '%%s\n' "$@" > %q
cp %q "$out"
exit %d
`, version, argsFile, report, exit)
	path := filepath.Join(dir, "bun")
	if err := os.WriteFile(path, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	return path, argsFile
}

// bunFixtureRepo is a committed Git repository with a unit and a preloaded integration suite.
func bunFixtureRepo(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	files := map[string]string{
		"package.json":                "{\"name\":\"x\",\"devDependencies\":{}}\n",
		"bun.lock":                    "{\n  \"lockfileVersion\": 1,\n  \"configVersion\": 1,\n  \"workspaces\": {\n    \"\": {\n      \"name\": \"x\",\n    },\n  },\n}\n",
		"test/unit/math.test.ts":      "import { describe, expect, test } from \"bun:test\";\n\ndescribe(\"math\", () => {\n  test(\"adds\", () => {\n    expect(1 + 1).toBe(2);\n  });\n});\n",
		"test/integration/db.test.ts": "import { expect, test } from \"bun:test\";\n\ntest(\"db ready\", () => {\n  expect((globalThis as any).DB_READY).toBe(true);\n});\n",
		"test/integration/preload.ts": "(globalThis as any).DB_READY = true;\n",
	}
	for name, body := range files {
		path := filepath.Join(root, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	for _, args := range [][]string{{"init", "-q"}, {"add", "-A"}, {"-c", "user.name=t", "-c", "user.email=t@t", "commit", "-qm", "fixture"}} {
		if out, err := exec.Command("git", append([]string{"-C", root}, args...)...).CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v %s", args, err, out)
		}
	}
	return root
}

// TestLPCVV0255_RunBunBindsCommandIdentityAndTree runs a fake bun that exits 1 for a failed test
// and checks the exact command (preload, concurrency, name pattern, files), the bun member and the
// clean before/after test repository (LPCV-V0-255, LPCV-V0-256, LPCV-V0-259).
func TestLPCVV0255_RunBunBindsCommandIdentityAndTree(t *testing.T) {
	root := bunFixtureRepo(t)
	report, err := filepath.Abs(filepath.Join("testdata", "bun-1.3.11-junit.xml"))
	if err != nil {
		t.Fatal(err)
	}
	bin, argsFile := fakeBun(t, t.TempDir(), "1.3.11", report, 1)
	unit, integration := filepath.Join(root, "test/unit/math.test.ts"), filepath.Join(root, "test/integration/db.test.ts")
	cfg := BunConfig{
		Config:     Config{Dir: root, TestFiles: []string{unit, integration}, PackageJSON: filepath.Join(root, "package.json"), Lockfile: filepath.Join(root, "bun.lock")},
		Executable: bin, Preload: []string{"./test/integration/preload.ts"}, MaxConcurrency: 1, TestNamePattern: "math|db", Services: []string{"db=fixture"},
	}
	receipt, err := RunBun(context.Background(), cfg)
	if err != nil {
		t.Fatal(err)
	}
	if receipt.Infrastructure != nil || len(receipt.Tests) != 4 {
		t.Fatalf("receipt = %+v", receipt)
	}
	argv := receipt.Identity.Argv
	want := []string{bin, "test", "--reporter=junit", argv[3], "--preload", "./test/integration/preload.ts", "--max-concurrency=1", "--test-name-pattern=math|db", unit, integration}
	if !slices.Equal(argv, want) || !strings.HasPrefix(argv[3], "--reporter-outfile=") {
		t.Fatalf("argv = %q, want %q", argv, want)
	}
	ran, err := os.ReadFile(argsFile)
	if err != nil || strings.Join(argv[1:], "\n")+"\n" != string(ran) {
		t.Fatalf("bun received %q, receipt argv %q (%v)", ran, argv[1:], err)
	}
	b := receipt.Bun
	if b == nil || b.Version != "1.3.11" || b.LockfileVersion != 1 || !strings.HasPrefix(b.Producer.Name, "corvint-js-test-provider") || b.Producer.GoVersion == "" ||
		len(b.LockfileDigest) != 64 || len(b.ExecutableDigest) != 64 || len(b.PreloadDigests["./test/integration/preload.ts"]) != 64 ||
		b.Isolation != BunIsolationSharedService || !b.ExitObserved || b.ExitStatus != 1 || b.RepositoryUnchanged == nil || !*b.RepositoryUnchanged {
		t.Fatalf("bun member = %+v", b)
	}
	if receipt.Identity.RunnerName != "bun" || receipt.Identity.RunnerVersion != "1.3.11" || receipt.Identity.PackageDigest == "" || len(receipt.Identity.TestFileDigests) != 2 {
		t.Fatalf("identity = %+v", receipt.Identity)
	}
	start, end := receipt.TestRepositoryAtStart, receipt.TestRepositoryAtPublish
	if start == nil || end == nil || start.DirtyState != "clean" || *start != *end {
		t.Fatalf("test repository start %+v end %+v", start, end)
	}
	data, err := EncodeQualified(receipt)
	if err != nil || !strings.Contains(string(data), `"bun":{"producer":`) {
		t.Fatalf("encoding: %v %s", err, data)
	}
	profiled := receipt
	profiled.Profile = ExternalProfile
	profiled.TestRepositoryAtStart, profiled.TestRepositoryAtPublish = nil, nil
	if _, err := EncodeQualified(profiled); err == nil || err.Error() != "bun-member-requires-unit-default-profile" {
		t.Fatalf("profiled bun receipt: %v", err)
	}
}

// TestLPCVV0258_BunRefusalsAreTypedAndPreRun: each refusal has its own closed code and starts no
// test (LPCV-V0-257, LPCV-V0-258).
func TestLPCVV0258_BunRefusalsAreTypedAndPreRun(t *testing.T) {
	root := bunFixtureRepo(t)
	report, _ := filepath.Abs(filepath.Join("testdata", "bun-1.3.11-junit.xml"))
	good, goodArgs := fakeBun(t, t.TempDir(), "1.3.11", report, 0)
	old, oldArgs := fakeBun(t, t.TempDir(), "1.2.0", report, 0)
	binary := filepath.Join(t.TempDir(), "bun.lockb")
	v2 := filepath.Join(t.TempDir(), "bun.lock")
	if err := os.WriteFile(binary, []byte{0, 1, 2}, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(v2, []byte(`{"lockfileVersion": 2}`), 0o644); err != nil {
		t.Fatal(err)
	}
	base := Config{Dir: root, TestFiles: []string{filepath.Join(root, "test/unit/math.test.ts")}, Lockfile: filepath.Join(root, "bun.lock")}
	with := func(f func(*BunConfig)) BunConfig {
		c := BunConfig{Config: base, Executable: good}
		f(&c)
		return c
	}
	cases := []struct {
		name string
		cfg  BunConfig
		code string
	}{
		{"unqualified version", with(func(c *BunConfig) { c.Executable = old }), BunVersionUnqualified},
		{"pinned version differs", with(func(c *BunConfig) { c.RunnerVersion = "1.3.10" }), BunVersionUnqualified},
		{"missing executable", with(func(c *BunConfig) { c.Executable = filepath.Join(root, "no-bun") }), BunVersionUnqualified},
		{"no lockfile", with(func(c *BunConfig) { c.Lockfile = "" }), BunLockfileUnqualified},
		{"binary lockfile", with(func(c *BunConfig) { c.Lockfile = binary }), BunLockfileUnqualified},
		{"lockfile version 2", with(func(c *BunConfig) { c.Lockfile = v2 }), BunLockfileUnqualified},
		{"service without serial run", with(func(c *BunConfig) { c.Services = []string{"db=x"} }), BunIsolationUndeclared},
	}
	for _, tc := range cases {
		_, err := RunBun(context.Background(), tc.cfg)
		var refusal *BunRefusal
		if !errors.As(err, &refusal) || refusal.Code != tc.code {
			t.Errorf("%s: err = %v, want %s", tc.name, err, tc.code)
			continue
		}
		var doc bunRefusalDocument
		if err := json.Unmarshal(EncodeBunRefusal(refusal), &doc); err != nil || doc.Schema != BunRefusalSchema || doc.Code != tc.code || !slices.Equal(doc.QualifiedBunVersions, []string{"1.3.11"}) {
			t.Errorf("%s: document %+v %v", tc.name, doc, err)
		}
	}
	for _, f := range []string{goodArgs, oldArgs} {
		if _, err := os.Stat(f); !os.IsNotExist(err) {
			t.Fatalf("a refused run started bun test: %s exists (%v)", f, err)
		}
	}
}

// TestLPCVV0262_LiveBunQualification runs the real bun named by CORVINT_BUN_EXECUTABLE against
// the fixture repository; without it the live qualification is NOT_RUN.
func TestLPCVV0262_LiveBunQualification(t *testing.T) {
	bin := os.Getenv("CORVINT_BUN_EXECUTABLE")
	if bin == "" {
		t.Skip("NOT_RUN: CORVINT_BUN_EXECUTABLE names no qualified bun")
	}
	root := bunFixtureRepo(t)
	receipt, err := RunBun(context.Background(), BunConfig{
		Config:     Config{Dir: root, TestFiles: []string{filepath.Join(root, "test/unit/math.test.ts"), filepath.Join(root, "test/integration/db.test.ts")}, Lockfile: filepath.Join(root, "bun.lock")},
		Executable: bin, Preload: []string{"./test/integration/preload.ts"}, MaxConcurrency: 1,
	})
	if err != nil {
		t.Fatal(err)
	}
	if receipt.Infrastructure != nil || len(receipt.Tests) != 2 || receipt.Tests[0].State != StatePassed || receipt.Tests[1].State != StatePassed ||
		receipt.Bun.ExitStatus != 0 || !*receipt.Bun.RepositoryUnchanged {
		t.Fatalf("live receipt = %+v bun %+v", receipt, receipt.Bun)
	}
}
