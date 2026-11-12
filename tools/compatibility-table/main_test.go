// SPDX-License-Identifier: Apache-2.0
package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

// fixture writes a repository under a temporary root whose list names every catalogue surface:
// the CMP-V1-003 surface "flow-file" is stable through a real test and label, "plan" ships here
// without a test and is preview, "action-inputs" does not exist yet and every other CMP-V1-003
// surface is owned by another repository, both not-promised; the CMP-V1-005 surfaces are preview.
func fixture(t *testing.T) (string, List) {
	t.Helper()
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "docs"), 0o755); err != nil {
		t.Fatal(err)
	}
	writeFile(t, root, "internal/flows/flows.go", "package flows\n\nconst Profile = \"corvint-flows/1\"\n")
	writeFile(t, root, "internal/flows/flows_test.go", "package flows\n\nimport \"testing\"\n\nfunc TestFlowFileContract(t *testing.T) {}\n")
	var ids []string
	for id := range catalogue {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	var list List
	for _, id := range ids {
		surface := Surface{ID: id, Surface: "surface " + id, Kind: "file", Stability: preview, Notes: "note"}
		switch {
		case id == "flow-file":
			surface.Class, surface.Stability = inRepo, stable
			surface.Label = "corvint-flows/1"
			surface.LabelSource = "internal/flows/flows.go"
			surface.Tests = []Test{{Package: "./internal/flows", Name: "TestFlowFileContract"}}
		case id == "plan":
			surface.Class = inRepo
		case id == "action-inputs":
			surface.Class, surface.Stability, surface.WaitsOn = notYet, notPromised, "change `github-action`"
		case catalogue[id] == stableSet:
			surface.Class, surface.Stability, surface.WaitsOn = otherRepo, notPromised, "the corvint-tasks repository"
		}
		list.Surfaces = append(list.Surfaces, surface)
	}
	writeList(t, root, list)
	return root, list
}

func writeFile(t *testing.T, root, name, body string) {
	t.Helper()
	target := filepath.Join(root, filepath.FromSlash(name))
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(target, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func writeList(t *testing.T, root string, list List) {
	t.Helper()
	data, err := json.MarshalIndent(list, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	writeFile(t, root, listPath, string(data)+"\n")
}

func runTool(t *testing.T, args ...string) (int, string, string) {
	t.Helper()
	var stdout, stderr bytes.Buffer
	code := run(args, &stdout, &stderr)
	return code, stdout.String(), stderr.String()
}

func readPage(t *testing.T, root string) []byte {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(pagePath)))
	if err != nil {
		t.Fatal(err)
	}
	return data
}

// TestCMPV1002RegenerationIsByteIdentical: a second generation leaves the page byte-identical, and
// the check then passes.
func TestCMPV1002RegenerationIsByteIdentical(t *testing.T) {
	root, _ := fixture(t)
	if code, _, stderr := runTool(t, "--root", root, "--write"); code != 0 {
		t.Fatalf("first write: %d %s", code, stderr)
	}
	first := readPage(t, root)
	if code, _, stderr := runTool(t, "--root", root, "--write"); code != 0 {
		t.Fatalf("second write: %d %s", code, stderr)
	}
	if second := readPage(t, root); !bytes.Equal(first, second) {
		t.Fatal("a second generation changed the page")
	}
	code, stdout, stderr := runTool(t, "--root", root)
	if code != 0 || !strings.Contains(stdout, "compatibility-check: PASS") {
		t.Fatalf("check: %d %q %q", code, stdout, stderr)
	}
	if !strings.Contains(stdout, "1 stable, 1 preview without a contract test, 20 not-promised; CMP-V1-005 preview surfaces: 10") {
		t.Fatalf("check output lacks the counts: %q", stdout)
	}
}

// TestCMPV1001And002StabilityComesFromTheTest: the only stable row is the one that names a test;
// a CMP-V1-003 surface that ships here without one is a preview row, one that does not ship here
// is not-promised and names what it waits on, and the page carries the rules.
func TestCMPV1001And002StabilityComesFromTheTest(t *testing.T) {
	root, _ := fixture(t)
	if code, _, stderr := runTool(t, "--root", root, "--write"); code != 0 {
		t.Fatalf("write: %d %s", code, stderr)
	}
	page := string(readPage(t, root))
	want := "| surface flow-file (label `corvint-flows/1`) | file | stable | `TestFlowFileContract` in `./internal/flows` | nothing | note |\n"
	if !strings.Contains(page, want) {
		t.Fatalf("page lacks the stable row %q:\n%s", want, page)
	}
	if strings.Count(page, "| stable |") != 1 {
		t.Fatalf("want exactly one stable row:\n%s", page)
	}
	if !strings.Contains(page, "| surface plan | file | preview | none | nothing | note |\n") {
		t.Fatalf("an untested CMP-V1-003 surface is not a preview row:\n%s", page)
	}
	if !strings.Contains(page, "| surface action-inputs | file | not-promised | none | change `github-action` | note |\n") {
		t.Fatalf("a CMP-V1-003 surface that does not exist yet is not a not-promised row:\n%s", page)
	}
	for _, rule := range []string{"A surface that has no row is `preview`.", "`corvint-harness-event/0`", "not a stability mark", "still serves `name/0` for one minor release", "names the change or repository it waits on"} {
		if !strings.Contains(page, rule) {
			t.Fatalf("page lacks rule text %q", rule)
		}
	}
}

// TestCMPV1002MissingContractTest: a stable row whose named test is gone, or that names none, fails
// the check and the write, naming the row.
func TestCMPV1002MissingContractTest(t *testing.T) {
	root, list := fixture(t)
	if code, _, stderr := runTool(t, "--root", root, "--write"); code != 0 {
		t.Fatalf("write: %d %s", code, stderr)
	}
	for _, missing := range []Test{
		{Package: "./internal/flows", Name: "TestDeletedContract"},
		{Package: "./internal/gone", Name: "TestFlowFileContract"},
		{Package: "./../outside", Name: "TestFlowFileContract"},
	} {
		setTests(list, "flow-file", []Test{{Package: "./internal/flows", Name: "TestFlowFileContract"}, missing})
		writeList(t, root, list)
		for _, args := range [][]string{{"--root", root}, {"--root", root, "--write"}} {
			code, _, stderr := runTool(t, args...)
			if code != 1 || !strings.Contains(stderr, `row "flow-file": stable row names contract test `+missing.Name) {
				t.Fatalf("%v with %v: %d %q", args, missing, code, stderr)
			}
		}
	}
	setTests(list, "flow-file", nil)
	writeList(t, root, list)
	if code, _, stderr := runTool(t, "--root", root); code != 1 || !strings.Contains(stderr, `row "flow-file": stable row names no contract test`) {
		t.Fatalf("stable without a test: %d %q", code, stderr)
	}
}

// setTests replaces the tests of row id.
func setTests(list List, id string, tests []Test) {
	for i := range list.Surfaces {
		if list.Surfaces[i].ID == id {
			list.Surfaces[i].Tests = tests
		}
	}
}

// TestCMPV1003NotPromisedRows: a surface that does not ship from this repository is not-promised,
// names what it waits on and names no test; a surface that ships here is never not-promised.
func TestCMPV1003NotPromisedRows(t *testing.T) {
	for name, tc := range map[string]struct {
		id     string
		change func(*Surface)
		want   string
	}{
		"no waitsOn":      {"action-inputs", func(s *Surface) { s.WaitsOn = "" }, "names the change or repository it waits on"},
		"names a test":    {"corvint-tasks-verbs", func(s *Surface) { s.Tests = []Test{{Package: "./internal/flows", Name: "TestFlowFileContract"}} }, "a not-promised surface names no contract test"},
		"marked stable":   {"ticket-schema-v1", func(s *Surface) { s.Stability = stable }, `stability "stable", but its class and tests make it "not-promised"`},
		"in-repo waiting": {"plan", func(s *Surface) { s.Stability = notPromised }, `stability "not-promised", but its class and tests make it "preview"`},
		"in-repo waitsOn": {"plan", func(s *Surface) { s.WaitsOn = "change `x`" }, "waits on nothing"},
		"no class":        {"run-evidence", func(s *Surface) { s.Class = "" }, "class must be in-repo, not-yet or other-repo"},
		"tested preview":  {"flow-file", func(s *Surface) { s.Stability = preview }, `stability "preview", but its class and tests make it "stable"`},
		"classed preview": {"index", func(s *Surface) { s.Class = inRepo }, "takes no class and no waitsOn"},
		"preview stable":  {"caches", func(s *Surface) { s.Stability = stable }, `stability "stable", but its class and tests make it "preview"`},
	} {
		t.Run(name, func(t *testing.T) {
			root, list := fixture(t)
			for i := range list.Surfaces {
				if list.Surfaces[i].ID == tc.id {
					tc.change(&list.Surfaces[i])
				}
			}
			writeList(t, root, list)
			if code, _, stderr := runTool(t, "--root", root, "--write"); code != 1 || !strings.Contains(stderr, fmt.Sprintf("row %q: ", tc.id)) || !strings.Contains(stderr, tc.want) {
				t.Fatalf("%d %q", code, stderr)
			}
		})
	}
}

// TestCMPV1002HandEditedPage: any byte changed in the committed page fails the check, naming the
// line and the fix.
func TestCMPV1002HandEditedPage(t *testing.T) {
	root, _ := fixture(t)
	if code, _, stderr := runTool(t, "--root", root, "--write"); code != 0 {
		t.Fatalf("write: %d %s", code, stderr)
	}
	page := readPage(t, root)
	edited := bytes.Replace(page, []byte("| surface plan | file | preview |"), []byte("| surface plan | file | stable |"), 1)
	if bytes.Equal(edited, page) {
		t.Fatal("fixture page lacks the row to edit")
	}
	writeFile(t, root, pagePath, string(edited))
	code, _, stderr := runTool(t, "--root", root)
	if code != 1 || !strings.Contains(stderr, "docs/COMPATIBILITY.md differs from the generated page at line") || !strings.Contains(stderr, "make compatibility-table") {
		t.Fatalf("hand edit: %d %q", code, stderr)
	}
	if err := os.Remove(filepath.Join(root, filepath.FromSlash(pagePath))); err != nil {
		t.Fatal(err)
	}
	if code, _, stderr := runTool(t, "--root", root); code != 1 || !strings.Contains(stderr, "fix: make compatibility-table") {
		t.Fatalf("missing page: %d %q", code, stderr)
	}
}

// TestCMPV1003UnlistedSurface: a row for a surface CMP-V1-003 and CMP-V1-005 do not name fails,
// and so does a listed surface with no row.
func TestCMPV1003UnlistedSurface(t *testing.T) {
	root, list := fixture(t)
	extra := list
	extra.Surfaces = append(append([]Surface{}, list.Surfaces...), Surface{ID: "corvint-feature-map-v0-fields", Surface: "feature map", Kind: "JSON field set", Class: inRepo, Stability: stable, Notes: "n", Tests: []Test{{Package: "./internal/flows", Name: "TestFlowFileContract"}}})
	writeList(t, root, extra)
	code, _, stderr := runTool(t, "--root", root, "--write")
	if code != 1 || !strings.Contains(stderr, `row "corvint-feature-map-v0-fields": unlisted surface`) {
		t.Fatalf("unlisted: %d %q", code, stderr)
	}
	short := list
	short.Surfaces = nil
	for _, surface := range list.Surfaces {
		if surface.ID != "mcp-tool-names" {
			short.Surfaces = append(short.Surfaces, surface)
		}
	}
	writeList(t, root, short)
	code, _, stderr = runTool(t, "--root", root, "--write")
	if code != 1 || !strings.Contains(stderr, `row "mcp-tool-names": missing`) {
		t.Fatalf("missing row: %d %q", code, stderr)
	}
}

// TestCMPV1005PreviewByNameNamesNoTest: a CMP-V1-005 surface cannot be made stable by naming a
// test.
func TestCMPV1005PreviewByNameNamesNoTest(t *testing.T) {
	root, list := fixture(t)
	for i := range list.Surfaces {
		if list.Surfaces[i].ID == "index" {
			list.Surfaces[i].Tests = []Test{{Package: "./internal/flows", Name: "TestFlowFileContract"}}
		}
	}
	writeList(t, root, list)
	if code, _, stderr := runTool(t, "--root", root); code != 1 || !strings.Contains(stderr, `row "index": a CMP-V1-005 surface is preview by name`) {
		t.Fatalf("preview with test: %d %q", code, stderr)
	}
}

// TestCMPV1003LabelReadFromCode: a format label must be a string literal in its label source.
func TestCMPV1003LabelReadFromCode(t *testing.T) {
	root, list := fixture(t)
	for i := range list.Surfaces {
		if list.Surfaces[i].ID == "flow-file" {
			list.Surfaces[i].Label = "corvint-flows/0"
		}
	}
	writeList(t, root, list)
	if code, _, stderr := runTool(t, "--root", root); code != 1 || !strings.Contains(stderr, `label corvint-flows/0 is not a string literal in internal/flows/flows.go`) {
		t.Fatalf("label: %d %q", code, stderr)
	}
}
