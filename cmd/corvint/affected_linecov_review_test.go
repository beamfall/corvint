// SPDX-License-Identifier: Apache-2.0
package main

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/linecov"
	"github.com/Beamfall/corvint/internal/sessiontest"
)

// GLS-V0-013: reading source bytes is independent of executing its coverage
// blocks. Keep the whole reader even when DIRECT_SOURCE_CHANGE wins its witness.
func TestAffectedLineSourceReaderKeptWhole(t *testing.T) {
	root, base, index := lineFixtureRepository(t, map[string]string{
		"calc/contents_test.go": `package calc
import ("os"; "strings"; "testing")
func TestContents(t *testing.T) {
 b, err := os.ReadFile("calc.go")
 if err != nil || !strings.Contains(string(b), "return n * 2") { t.Fatal("source changed", err) }
}
`,
		"user/contents_test.go": `package user
import ("os"; "strings"; "testing")
func TestContents(t *testing.T) {
 b, err := os.ReadFile("../calc/calc.go")
 if err != nil || !strings.Contains(string(b), "return n * 2") { t.Fatal("source changed", err) }
}
`,
	})
	lineCommit(t, root, "calc/calc.go", "return n * 2", "return n * 3")
	receipt, _, stderr, code := runAffectedCLI(t, root, "--base", base, "--line-index", index)
	if code != 0 {
		t.Fatalf("exit %d: %s", code, stderr)
	}
	_, units := lineUnits(t, receipt)
	sel := receipt["lineSelection"].(map[string]any)
	for _, value := range sel["units"].([]any) {
		u := value.(map[string]any)
		if u["category"] != "static-read" {
			t.Errorf("reader reason lost: %v", u)
		}
	}
	for _, value := range sel["paths"].([]any) {
		p := value.(map[string]any)
		if p["class"] != "body" {
			t.Errorf("body unexpectedly unmapped: %v", p)
		}
	}
	for _, want := range []string{"calc=whole", "user=whole"} {
		if !strings.Contains(strings.Join(units, " "), want) {
			t.Errorf("reader lost: %v, want %s", units, want)
		}
	}
	// The source-only oracle fails although it never calls Double.
	for _, pkg := range []string{"calc", "user"} {
		cmd := exec.Command("go", "test", "-count=1", "-run", "^TestContents$", "./"+pkg)
		cmd.Dir = root
		if out, err := cmd.CombinedOutput(); err == nil || !bytes.Contains(out, []byte("source changed")) {
			t.Fatalf("fault not detected: %v %s", err, out)
		}
	}
}

// GLS-V0-012: a pure insertion in an unexecuted body can introduce init.
func TestAffectedLineInsertionIntroducingInitKeepsWhole(t *testing.T) {
	root, base, index := lineFixtureRepository(t, map[string]string{
		"calc/state.go":      "package calc\n\nvar State int\n\nfunc unused() {\n\n}\n",
		"calc/state_test.go": "package calc\nimport \"testing\"\nfunc TestState(t *testing.T) { if State != 0 { t.Fatal(\"state changed\") } }\n",
	})
	lineCommit(t, root, "calc/state.go", "func unused() {\n", "func unused() {\n}\n\nfunc init() {\nState = 1\n")
	receipt, _, stderr, code := runAffectedCLI(t, root, "--base", base, "--line-index", index)
	if code != 0 {
		t.Fatalf("exit %d: %s", code, stderr)
	}
	_, units := lineUnits(t, receipt)
	sel := receipt["lineSelection"].(map[string]any)
	paths := sel["paths"].([]any)
	if len(paths) != 1 || paths[0].(map[string]any)["class"] != "declaration" {
		t.Errorf("init not classified as declaration: %v", paths)
	}
	if !strings.Contains(strings.Join(units, " "), "calc=whole") {
		t.Fatalf("init edit narrowed: %v", units)
	}
	cmd := exec.Command("go", "test", "-count=1", "-run", "^TestState$", "./calc")
	cmd.Dir = root
	if out, err := cmd.CombinedOutput(); err == nil || !bytes.Contains(out, []byte("state changed")) {
		t.Fatalf("fault not detected: %v %s", err, out)
	}
}

// GLS-V0-020, GLS-V0-034, LPCV-V0-955/963: an unqualified local index is
// report-only. Even a valid index must not remove tests from the manifest.
func TestCorvintTestUnqualifiedLineIndexIsShadow(t *testing.T) {
	root, base, index := lineFixtureRepository(t)
	lineCommit(t, root, "calc/calc.go", "return n * 2", "return n * 3")
	receipt, _, stderr, code := runAffectedCLI(t, root, "--base", base, "--line-index", index)
	if code != 0 {
		t.Fatalf("exit %d: %s", code, stderr)
	}
	state, units := lineUnits(t, receipt)
	if !strings.HasPrefix(state, "SHADOW ") || len(units) == 0 {
		t.Errorf("unqualified selection: %s %v", state, units)
	}
	manifest := filepath.Join(t.TempDir(), "manifest.json")
	var stdout, errout bytes.Buffer
	code = runContext(context.Background(), []string{"--root", root, "test", "--base", base, "--line-index", index, "--dry-run", "--manifest", manifest}, strings.NewReader(""), &stdout, &errout)
	if code != 0 {
		t.Fatalf("exit %d: %s", code, errout.String())
	}
	data, err := os.ReadFile(manifest)
	if err != nil {
		t.Fatal(err)
	}
	var m struct {
		Excluded  []string
		Units     []struct{ Unit, Pattern string }
		LineIndex string
	}
	if err := json.Unmarshal(data, &m); err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(m.LineIndex, "SHADOW ") || len(m.Excluded) != 0 || len(m.Units) != 2 {
		t.Fatalf("unqualified exclusions: %s", data)
	}
	// Static symbol advice may already slice a package. Compare the actual
	// static manifest rather than attributing its slices to line coverage.
	var plainOut, plainErr bytes.Buffer
	plain := manifest + ".static"
	if code := runContext(context.Background(), []string{"--root", root, "test", "--base", base, "--dry-run", "--manifest", plain}, strings.NewReader(""), &plainOut, &plainErr); code != 0 {
		t.Fatal(plainErr.String())
	}
	plainData, err := os.ReadFile(plain)
	if err != nil {
		t.Fatal(err)
	}
	var static struct {
		Excluded []string
		Units    []struct{ Unit, Pattern string }
	}
	if err := json.Unmarshal(plainData, &static); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(m.Units, static.Units) || !reflect.DeepEqual(m.Excluded, static.Excluded) {
		t.Fatalf("shadow changed static plan: %s versus %s", data, plainData)
	}

	// A shadow record cannot be accidentally applied by the manifest adapter.
	if _, units := testLineUnits(linecov.Selection{State: "SHADOW"}); len(units) != 0 {
		t.Fatal("shadow applied")
	}
}

// GLS-V0-020: a declared go flag the runner identity cannot carry refuses
// the line selection (runner-unavailable), so `corvint test` keeps the
// static plan rather than narrowing under a runner the index never named.
func TestCorvintTestUncarriedGoFlagKeepsStaticPlan(t *testing.T) {
	declaration := `{"profile": "corvint-test-commands/0", "go": {"env": {"GOTOOLCHAIN": "local"}, "flags": ["-count=1", "-shuffle=on"]}, "languages": {}}`
	root, base, index := lineFixtureRepository(t, map[string]string{".github/cishards/test-commands.json": declaration})
	lineCommit(t, root, "calc/calc.go", "return n * 2", "return n * 3")
	read := func(args ...string) (string, []string, []string) {
		manifest := filepath.Join(t.TempDir(), "manifest.json")
		var stdout, errout bytes.Buffer
		argv := append([]string{"--root", root, "test", "--base", base, "--dry-run", "--manifest", manifest}, args...)
		if code := runContext(context.Background(), argv, strings.NewReader(""), &stdout, &errout); code != 0 {
			t.Fatalf("exit %d: %s", code, errout.String())
		}
		data, err := os.ReadFile(manifest)
		if err != nil {
			t.Fatal(err)
		}
		var m struct {
			Excluded  []string
			Units     []struct{ Unit, Pattern string }
			LineIndex string
		}
		if err := json.Unmarshal(data, &m); err != nil {
			t.Fatal(err)
		}
		var units []string
		for _, u := range m.Units {
			units = append(units, u.Unit+" "+u.Pattern)
		}
		return m.LineIndex, units, m.Excluded
	}
	state, units, excluded := read("--line-index", index)
	_, static, staticExcluded := read()
	if !strings.HasPrefix(state, "FALLBACK ") {
		t.Errorf("line selection %q, want FALLBACK (without the declaration it is SHADOW)", state)
	}
	_, _, refused := sessiontest.RunnerEnv(root, sessiontest.ProfileLocal, sessiontest.ReadDeclared(root), os.Environ())
	receipt, _, err := compileAffectedGraph(context.Background(), affectedInvocation{Root: root, Base: base, LineIndex: index, lineEnvErr: refused})
	if err != nil {
		t.Fatal(err)
	}
	if sel := receipt.LineSelection; refused == nil || sel == nil || sel.Reason != linecov.ReasonRunnerUnavailable || !strings.Contains(sel.Detail, "-shuffle") {
		t.Errorf("refusal %v gave line selection %+v, want runner-unavailable naming -shuffle", refused, sel)
	}
	if !reflect.DeepEqual(units, static) || !reflect.DeepEqual(excluded, staticExcluded) {
		t.Errorf("units %v excluded %v, want the static %v %v", units, excluded, static, staticExcluded)
	}
}

// GLS-V0-020: collection runs every test verbose (-test.v=test2json), but a
// declared -v=false comes after `corvint test`'s -json and turns verbosity
// off at execution. A test that runs Double only when !testing.Verbose()
// records no Double coverage at collection, so the selection must not narrow
// under a declared verbosity override: it is runner-unavailable, and the
// static plan keeps that test, which then fails on the Double change.
func TestCorvintTestDeclaredVerbosityKeepsInverseVerboseTest(t *testing.T) {
	declaration := `{"profile": "corvint-test-commands/0", "go": {"env": {"GOTOOLCHAIN": "local"}, "flags": ["-count=1", "-v=false"]}, "languages": {}}`
	root, base, index := lineFixtureRepository(t, map[string]string{
		".github/cishards/test-commands.json": declaration,
		"calc/quiet_test.go": `package calc
import "testing"
func TestQuiet(t *testing.T) {
 if !testing.Verbose() && Double(2) != 4 { t.Fatal("quiet double changed") }
}
`,
	})
	lineCommit(t, root, "calc/calc.go", "return n * 2", "return n * 3")
	env, timeout, refused := sessiontest.RunnerEnv(root, sessiontest.ProfileLocal, sessiontest.ReadDeclared(root), os.Environ())
	receipt, _, err := compileAffectedGraph(context.Background(), affectedInvocation{Root: root, Base: base, LineIndex: index, lineEnv: env, lineTimeout: timeout, lineEnvErr: refused})
	if err != nil {
		t.Fatal(err)
	}
	data, err := json.Marshal(receipt)
	if err != nil {
		t.Fatal(err)
	}
	var decoded map[string]any
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatal(err)
	}
	state, units := lineUnits(t, decoded)
	// A FALLBACK leaves the static plan, which runs calc whole.
	keeps := strings.HasPrefix(state, linecov.StateFallback+" ")
	for _, u := range units {
		if u == "calc=whole" || (strings.HasPrefix(u, "calc=") && strings.Contains(u, "TestQuiet")) {
			keeps = true
		}
	}
	if !keeps {
		t.Errorf("line selection %s units %v drop TestQuiet, which runs Double under -v=false", state, units)
	}
	if sel := receipt.LineSelection; refused == nil || sel == nil || sel.Reason != linecov.ReasonRunnerUnavailable || !strings.Contains(sel.Detail, "-v") {
		t.Errorf("refusal %v gave line selection %+v, want runner-unavailable naming -v", refused, sel)
	}
	// The fault is real: run as `corvint test` would, TestQuiet fails.
	cmd := exec.Command("go", "test", "-json", "-count=1", "-v=false", "-run", "^TestQuiet$", "./calc")
	cmd.Dir = root
	if out, err := cmd.CombinedOutput(); err == nil || !bytes.Contains(out, []byte("quiet double changed")) {
		t.Fatalf("fault not detected: %v %s", err, out)
	}
}
