// SPDX-License-Identifier: Apache-2.0
package main

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

// repository writes a module with one bounded and one unbounded test package.
func repository(t *testing.T, record string) string {
	t.Helper()
	root := t.TempDir()
	for name, content := range map[string]string{
		"go.mod":            "module example.test/m\n\ngo 1.27\n",
		"bounded/b.go":      "package bounded\n",
		"bounded/b_test.go": "package bounded\n\nimport \"testing\"\n\nfunc TestB(t *testing.T) {}\n",
		"reader/r.go":       "package reader\n",
		"untested/u.go":     "package untested\n\nimport \"os\"\n\nfunc U() { _, _ = os.Getwd() }\n",
		"reader/r_test.go":  "package reader\n\nimport (\n\t\"os\"\n\t\"testing\"\n)\n\nfunc TestR(t *testing.T) { _, _ = os.Getwd() }\n",
		recordPath:          record,
	} {
		file := filepath.Join(root, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(file), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(file, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return root
}

func TestAFPV0025RatchetFailsOffTheRecordedSet(t *testing.T) {
	const head = `{"profile":"corvint-unbounded-reader-set/0",`
	for name, c := range map[string]struct {
		record string
		code   int
		want   string
	}{
		"the recorded set":   {head + `"units":["reader"],"reasons":{"reader":"calls os.Getwd"}}`, 0, `"count": 1`},
		"unrecorded unit":    {head + `"units":[],"reasons":{}}`, 1, "[reader] are selected on every change"},
		"stale unit":         {head + `"units":["bounded","reader"],"reasons":{"bounded":"was a reader","reader":"calls os.Getwd"}}`, 0, "records [bounded], which are not unbounded test packages"},
		"swapped unit":       {head + `"units":["bounded"],"reasons":{"bounded":"was a reader"}}`, 1, "[reader] are selected on every change"},
		"reason off the set": {head + `"units":["reader"],"reasons":{"bounded":"none"}}`, 2, "want exactly"},
		"unknown member":     {head + `"units":["reader"],"reasons":{},"extra":1}`, 2, "want exactly"},
		"old ceiling":        {`{"profile":"corvint-unbounded-reader-ceiling/0","ceiling":1,"reasons":{}}`, 2, "want exactly"},
		"missing units":      {head + `"reasons":{}}`, 2, "want exactly"},
		"null units":         {head + `"units":null,"reasons":{}}`, 2, "want exactly"},
		"unsorted units":     {head + `"units":["reader","bounded"],"reasons":{}}`, 2, "want exactly"},
		"repeated unit":      {head + `"units":["reader","reader"],"reasons":{}}`, 2, "want exactly"},
		"empty unit":         {head + `"units":["","reader"],"reasons":{}}`, 2, "want exactly"},
		"other profile":      {`{"profile":"other/0","units":["reader"],"reasons":{}}`, 2, "want exactly"},
		"empty reason":       {head + `"units":["reader"],"reasons":{"reader":" "}}`, 2, "want exactly"},
		"duplicate units":    {head + `"units":[],"units":["reader"],"reasons":{}}`, 2, "want exactly"},
		"member case":        {head + `"Units":["reader"],"reasons":{}}`, 2, "want exactly"},
		"trailing document":  {head + `"units":["reader"],"reasons":{}} {}`, 2, "want exactly"},
	} {
		var stdout, stderr bytes.Buffer
		code := run(repository(t, c.record), &stdout, &stderr)
		if code != c.code || !strings.Contains(stdout.String()+stderr.String(), c.want) {
			t.Errorf("%s: code=%d want %d; output %q %q lacks %q", name, code, c.code, stdout.String(), stderr.String(), c.want)
		}
	}
}

// Two changes cut from the same base each add an unbounded test package and its
// entry. Each passes alone, and so does their merge: the V1-0752 skew, where a
// package reaches the base branch after the other change's last check.
func TestAFPV0025ConcurrentAdditionsMergeToAPassingRecord(t *testing.T) {
	const reader = "package %s\n\nimport (\n\t\"os\"\n\t\"testing\"\n)\n\nfunc TestR(t *testing.T) { _, _ = os.Getwd() }\n"
	// Every unit carries its reason (AFP-V0-033), one line each, so two
	// additions merge line by line like the units.
	record := func(units ...string) string {
		reasons := make([]string, len(units))
		for i, unit := range units {
			reasons[i] = "\"" + unit + "\": \"calls os.Getwd\""
		}
		return "{\n  \"profile\": \"corvint-unbounded-reader-set/0\",\n  \"units\": [\n    \"" + strings.Join(units, "\",\n    \"") + "\"\n  ],\n  \"reasons\": {\n    " + strings.Join(reasons, ",\n    ") + "\n  }\n}\n"
	}
	root := repository(t, record("alpha", "middle", "reader", "zeta"))
	git := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", append([]string{"-c", "maintenance.auto=false", "-c", "gc.auto=0", "-c", "user.name=t", "-c", "user.email=t@example.test", "-c", "commit.gpgsign=false"}, args...)...)
		cmd.Dir = root
		cmd.Env = append(os.Environ(), "GIT_CONFIG_GLOBAL="+os.DevNull, "GIT_CONFIG_NOSYSTEM=1")
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	write := func(name, content string) {
		t.Helper()
		file := filepath.Join(root, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(file), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(file, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	check := func(state string, want int) string {
		t.Helper()
		var stdout, stderr bytes.Buffer
		if code := run(root, &stdout, &stderr); code != want {
			t.Fatalf("%s: code=%d want %d; %s %s", state, code, want, stdout.String(), stderr.String())
		}
		return stderr.String()
	}
	for _, name := range []string{"alpha", "middle", "zeta"} {
		write(name+"/r_test.go", fmt.Sprintf(reader, name))
	}
	git("init", "-q", "-b", "base")
	git("add", "-A")
	git("commit", "-q", "-m", "base")
	check("base", 0)
	for branch, name := range map[string]string{"first": "beta", "second": "omega"} {
		git("checkout", "-q", "-b", branch, "base")
		write(name+"/r_test.go", fmt.Sprintf(reader, name))
		units := []string{"alpha", name, "middle", "reader", "zeta"}
		sort.Strings(units)
		write(recordPath, record(units...))
		git("add", "-A")
		git("commit", "-q", "-m", branch)
		check(branch, 0)
	}
	git("checkout", "-q", "first")
	git("merge", "-q", "--no-edit", "second")
	check("merge", 0)

	// A package that arrives without its entry is named, not counted.
	write("late/r_test.go", fmt.Sprintf(reader, "late"))
	if got := check("unrecorded", 1); !strings.Contains(got, "[late] are selected on every change") {
		t.Fatalf("unrecorded package not named: %s", got)
	}
}

func TestAFPV0025RatchetWithoutARecordRefuses(t *testing.T) {
	root := repository(t, "{}")
	if err := os.Remove(filepath.Join(root, filepath.FromSlash(recordPath))); err != nil {
		t.Fatal(err)
	}
	var stdout, stderr bytes.Buffer
	if code := run(root, &stdout, &stderr); code != 2 {
		t.Fatalf("code=%d output %q", code, stderr.String())
	}
}

// Every units directory names its unbounded read (AFP-V0-033): a unit, live or
// stale, without a reasons entry fails the check and is named.
func TestAFPV0033EveryUnitNamesItsUnboundedRead(t *testing.T) {
	const head = `{"profile":"corvint-unbounded-reader-set/0",`
	for name, c := range map[string]struct {
		record string
		code   int
		want   []string
	}{
		"every unit has a reason": {head + `"units":["bounded","reader"],"reasons":{"bounded":"was a reader","reader":"r_test.go calls os.Getwd"}}`, 0, []string{`"unreasoned": []`}},
		"live unit without one":   {head + `"units":["reader"],"reasons":{}}`, 1, []string{"[reader] in units of .corvint/unbounded-readers.json have no entry in reasons"}},
		"stale unit without one":  {head + `"units":["bounded","reader"],"reasons":{"reader":"r_test.go calls os.Getwd"}}`, 1, []string{"[bounded] in units of .corvint/unbounded-readers.json have no entry in reasons"}},
		"both failures named":     {head + `"units":["bounded"],"reasons":{}}`, 1, []string{"[reader] are selected on every change", "[bounded] in units of .corvint/unbounded-readers.json have no entry in reasons"}},
	} {
		var stdout, stderr bytes.Buffer
		code := run(repository(t, c.record), &stdout, &stderr)
		for _, want := range c.want {
			if code != c.code || !strings.Contains(stdout.String()+stderr.String(), want) {
				t.Errorf("%s: code=%d want %d; output %q %q lacks %q", name, code, c.code, stdout.String(), stderr.String(), want)
			}
		}
	}
}

// TestAFPV0025FileLevelUnitsAreRecordedByTheirTestFile proves the V1-1301 rule:
// two toolchain shell tests in one directory are two units, each recorded by its
// own test file, so neither is reported as a second unit sharing a directory.
func TestAFPV0025FileLevelUnitsAreRecordedByTheirTestFile(t *testing.T) {
	const head = `{"profile":"corvint-unbounded-reader-set/0",`
	record := head + `"units":["reader","script/a_test.sh","script/b_test.sh"],"reasons":{"reader":"calls os.Getwd","script/a_test.sh":"runs go","script/b_test.sh":"runs make"}}`
	root := repository(t, record)
	for name, content := range map[string]string{"script/a_test.sh": "go test ./cmd\n", "script/b_test.sh": "make check\n"} {
		if err := os.MkdirAll(filepath.Join(root, "script"), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(root, filepath.FromSlash(name)), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	var stdout, stderr bytes.Buffer
	if code := run(root, &stdout, &stderr); code != 0 || !strings.Contains(stdout.String(), `"count": 3`) {
		t.Fatalf("code=%d stdout=%s stderr=%s", code, stdout.String(), stderr.String())
	}
}

// TSP-V0-723, V1-3780: an unrecorded reader's refusal names the record file and the
// exact entry shape to add, besides the import chain (AFP-V0-500).
func TestUnrecordedRefusalNamesTheEntryShape(t *testing.T) {
	root := repository(t, `{"profile":"corvint-unbounded-reader-set/0","units":["reader"],"reasons":{"reader":"calls os.Getwd"}}`)
	for name, content := range map[string]string{
		"late/l.go":      "package late\n\nimport _ \"example.test/m/untested\"\n",
		"late/l_test.go": "package late\n\nimport \"testing\"\n\nfunc TestL(t *testing.T) {}\n",
	} {
		file := filepath.Join(root, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(file), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(file, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	var stdout, stderr bytes.Buffer
	if code := run(root, &stdout, &stderr); code != 1 {
		t.Fatalf("exit %d, want 1: %s", code, stderr.String())
	}
	want := `  late: to record it, insert "late" into units of .corvint/unbounded-readers.json in ascending order and add "late": "<the unbounded read, and why .corvint/test-read-scopes.json cannot declare it>" to its reasons`
	if !strings.Contains(stderr.String(), want+"\n") {
		t.Fatalf("refusal lacks the entry shape:\n%s", stderr.String())
	}
	if !strings.Contains(stderr.String(), "import chain") {
		t.Fatalf("refusal lacks the import chain:\n%s", stderr.String())
	}
}

// TestAFPV0500RefusalNamesTheImportChain proves V1-2823: a package that becomes
// an unbounded reader only because it imports one is refused with the importing
// package, every import edge, and the unbounded reader at the chain's end.
func TestAFPV0500RefusalNamesTheImportChain(t *testing.T) {
	const head = `{"profile":"corvint-unbounded-reader-set/0",`
	root := repository(t, head+`"units":["reader"],"reasons":{"reader":"calls os.Getwd"}}`)
	for name, content := range map[string]string{
		"middle/m.go":    "package middle\n\nimport _ \"example.test/m/untested\"\n",
		"user/u.go":      "package user\n\nimport _ \"example.test/m/middle\"\n",
		"user/u_test.go": "package user\n\nimport \"testing\"\n\nfunc TestU(t *testing.T) {}\n",
	} {
		file := filepath.Join(root, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(file), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(file, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	var stdout, stderr bytes.Buffer
	if code := run(root, &stdout, &stderr); code != 1 {
		t.Fatalf("code=%d stderr=%s", code, stderr.String())
	}
	for _, want := range []string{
		"[user] are selected on every change",
		"go:example.test/m/user: import chain go:example.test/m/user imports go:example.test/m/middle, go:example.test/m/middle imports go:example.test/m/untested; go:example.test/m/untested is the unbounded reader (untested/u.go calls os.Getwd)",
	} {
		if !strings.Contains(stderr.String(), want) {
			t.Errorf("refusal lacks %q:\n%s", want, stderr.String())
		}
	}
	// The recorded reader is its own cause and is not explained.
	if strings.Contains(stderr.String(), "its own read is unbounded") {
		t.Errorf("recorded unit explained:\n%s", stderr.String())
	}
}

// V1-3320, V1-2159: a shell test that runs a toolchain, here `make -n` pinning a recipe
// expansion, is refused with the one registration route that applies to it: units plus a reason
// in .corvint/unbounded-readers.json, never .corvint/test-read-scopes.json. A Go reader keeps
// the generic message only.
func TestRunsToolchainRefusalNamesTheUnitsEntry(t *testing.T) {
	root := repository(t, `{"profile":"corvint-unbounded-reader-set/0","units":["reader"],"reasons":{"reader":"calls os.Getwd"}}`)
	test := filepath.Join(root, "script", "pin-recipe_test.sh")
	if err := os.MkdirAll(filepath.Dir(test), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(test, []byte("#!/bin/sh\nset -eu\nout=$(make -s -n instruction-conflicts)\ntest \"$out\" = \"script/check-instruction-conflicts.sh --repository-only\"\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	var stdout, stderr bytes.Buffer
	if code := run(root, &stdout, &stderr); code != 1 {
		t.Fatalf("exit %d, want 1: %s", code, stderr.String())
	}
	want := `script/pin-recipe_test.sh runs a build or language toolchain (shell:runs-toolchain): declare the paths it reads outside its directory in .corvint/shell-read-scopes.json (TSP-V0-950), or add "script/pin-recipe_test.sh" to units in .corvint/unbounded-readers.json with a reasons entry naming the toolchain read; .corvint/test-read-scopes.json is keyed by Go package and cannot bound a shell unit (docs/SHELL-TEST-REGISTRATION.md).`
	if !strings.Contains(stderr.String(), want) {
		t.Fatalf("refusal lacks the units route:\n%s", stderr.String())
	}
	if strings.Count(stderr.String(), "runs a build or language toolchain") != 1 {
		t.Fatalf("the toolchain route is named for a non-shell unit too:\n%s", stderr.String())
	}
}
