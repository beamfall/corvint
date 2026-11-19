// SPDX-License-Identifier: Apache-2.0
package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/gateselect"
	"github.com/Beamfall/corvint/internal/plansnapshot"
)

func TestAffectedRuntimeEdgeWidening_CINV1940(t *testing.T) {
	root := affectedFixtureRepository(t)
	if err := os.MkdirAll(filepath.Join(root, ".corvint"), 0700); err != nil {
		t.Fatal(err)
	}
	declaration := `{"profile":"corvint-runtime-edges/0","edges":[{"tests":"leaf/leaf_test.go","runs":"python:new.analyzer.unit","reason":"newly analyzable runtime tool"}]}`
	if err := os.WriteFile(filepath.Join(root, ".corvint/runtime-edges.json"), []byte(declaration), 0600); err != nil {
		t.Fatal(err)
	}
	affectedGit(t, root, "add", ".corvint/runtime-edges.json")
	affectedGit(t, root, "commit", "-qm", "declare runtime edge")
	var first []byte
	for _, cache := range []string{"", filepath.Join(t.TempDir(), "cache")} {
		args := []string{"--path", "leaf/leaf.go"}
		if cache != "" {
			args = append(args, "--cache", cache)
		}
		_, raw, stderr, code := runAffectedCLI(t, root, args...)
		if code != 0 {
			t.Fatalf("exit=%d stderr=%s", code, stderr)
		}
		if first == nil {
			first = raw
		} else if !bytes.Equal(first, raw) {
			t.Fatal("cache changed degraded receipt")
		}
		var plan gateselect.Receipt
		if err := json.Unmarshal(raw, &plan); err != nil {
			t.Fatal(err)
		}
		lines := gateselect.SelectQualified(plan, "example.com/fixture", root)
		verdict := lines[len(lines)-1]
		if !strings.HasPrefix(verdict, "run ") || !strings.Contains(verdict, "example.com/fixture/leaf") || strings.Contains(verdict, "example.com/fixture/solo") {
			t.Fatalf("not narrowed: %v", lines)
		}
		found := false
		for _, unknown := range plan.Plan.Unknown {
			if strings.HasPrefix(unknown.Detail, `runtime:unresolved-edge: tests="leaf/leaf_test.go" runs="python:new.analyzer.unit": runs names no unit`) {
				found = true
			}
		}
		if !found {
			t.Fatalf("degradation missing: %s", raw)
		}
	}
	// The retry must validate the rest of the file, even after the unknown target.
	invalid := strings.Replace(declaration, `}]}`, `},{"tests":"missing_test.go","runs":"go:example.com/fixture/core","reason":"stale holder"}]}`, 1)
	if err := os.WriteFile(filepath.Join(root, ".corvint/runtime-edges.json"), []byte(invalid), 0600); err != nil {
		t.Fatal(err)
	}
	_, _, stderr, code := runAffectedCLI(t, root)
	if code == 0 || !strings.Contains(stderr, "holds no test file") {
		t.Fatalf("invalid retry exit=%d stderr=%s", code, stderr)
	}
}

func TestAffectedRuntimeEdgeSnapshot_CINV1940(t *testing.T) {
	root := affectedFixtureRepository(t)
	const declaration = `{"profile":"corvint-runtime-edges/0","edges":[
		{"tests":"leaf/leaf_test.go","runs":"future:target","reason":"runtime tool"}]}`
	writePlaywrightCLIFile(t, root, ".corvint/runtime-edges.json", declaration)
	affectedGit(t, root, "add", ".")
	affectedGit(t, root, "commit", "-qm", "runtime declaration")
	base := strings.TrimSpace(affectedGit(t, root, "rev-parse", "HEAD"))
	const dirty = "future/input.unrecognized"
	writePlaywrightCLIFile(t, root, dirty, "unknown source form\n")
	affectedGit(t, root, "add", ".")
	affectedGit(t, root, "commit", "-qm", "unknown runtime source")
	snapshot := plansnapshot.Receipt{
		Schema: plansnapshot.Schema, Base: base,
		Commit: strings.TrimSpace(affectedGit(t, root, "rev-parse", "HEAD")),
		Tree:   strings.TrimSpace(affectedGit(t, root, "rev-parse", "HEAD^{tree}")),
		Paths:  []string{dirty}, Digest: plansnapshot.PathDigest([]string{dirty}),
	}
	raw, err := json.Marshal(snapshot)
	if err != nil {
		t.Fatal(err)
	}
	name := filepath.Join(t.TempDir(), "snapshot.json")
	if err := os.WriteFile(name, raw, 0600); err != nil {
		t.Fatal(err)
	}
	_, clean, stderr, code := runAffectedCLI(t, root, "--snapshot", name)
	if code != 0 {
		t.Fatalf("snapshot exit=%d stderr=%s", code, stderr)
	}
	assertRuntimeWidenedLeaf(t, clean)
	// A retry must read the materialized tree, even if the live declaration
	// has an unknown form that cannot be interpreted safely.
	writePlaywrightCLIFile(t, root, ".corvint/runtime-edges.json",
		strings.Replace(declaration, `"reason":`, `"futureRelation":true,"reason":`, 1))
	_, changed, stderr, code := runAffectedCLI(t, root, "--snapshot", name)
	if code != 0 || !bytes.Equal(clean, changed) {
		t.Fatalf("live declaration changed snapshot: exit=%d stderr=%s", code, stderr)
	}
	_, _, stderr, code = runAffectedCLI(t, root, "--no-cache")
	if code == 0 || !strings.Contains(stderr, "unsupported-affected-graph") {
		t.Fatalf("unknown runtime form admitted: exit=%d stderr=%s", code, stderr)
	}
}

func TestAffectedRuntimeEdgeIgnoredRebuild_CINV1940(t *testing.T) {
	root := affectedFixtureRepository(t)
	writePlaywrightCLIFile(t, root, ".corvint/runtime-edges.json",
		`{"profile":"corvint-runtime-edges/0","edges":[
		{"tests":"leaf/leaf_test.go","runs":"future:target","reason":"runtime tool"}]}`)
	writePlaywrightCLIFile(t, root, ".gitignore", "/playwright-report/\n")
	affectedGit(t, root, "add", ".")
	affectedGit(t, root, "commit", "-qm", "runtime declaration and ignored report")
	const report = "playwright-report/trace/assets/report.js"
	writePlaywrightCLIFile(t, root, report,
		"export async function load(name) { return import(name) }\n")
	for _, cache := range [][]string{
		{"--no-cache"}, {"--cache", filepath.Join(t.TempDir(), "cache")},
	} {
		args := append([]string{"--path", "leaf/leaf.go"}, cache...)
		_, raw, stderr, code := runAffectedCLI(t, root, args...)
		if code != 0 {
			t.Fatalf("ignored rebuild exit=%d stderr=%s", code, stderr)
		}
		assertRuntimeWidenedLeaf(t, raw)
		if strings.Contains(string(raw), "playwright-report") {
			t.Fatalf("retry lost ignored exclusion: %s", raw)
		}
	}
	// Unsupported ignored inputs outside the TypeScript exclusion contract
	// must still fail closed, even with a retryable runtime target.
	writePlaywrightCLIFile(t, root, "playwright-report/gen.py", "import os\n")
	_, raw, stderr, code := runAffectedCLI(t, root, "--no-cache", "--path", "leaf/leaf.go")
	if code != 2 || len(raw) != 0 ||
		!strings.Contains(stderr, "unsupported-affected-mixed-worktree") {
		t.Fatalf("unsupported ignored form admitted: exit=%d stderr=%s", code, stderr)
	}
}

func assertRuntimeWidenedLeaf(t *testing.T, raw []byte) {
	t.Helper()
	var receipt gateselect.Receipt
	if err := json.Unmarshal(raw, &receipt); err != nil {
		t.Fatal(err)
	}
	for _, selection := range receipt.Plan.Selected {
		if selection.UnitID == "go:example.com/fixture/leaf" &&
			selection.Witness.Kind == "UNBOUNDED_READER" {
			return
		}
	}
	t.Fatalf("missing widened leaf: %s", raw)
}
