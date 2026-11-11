// SPDX-License-Identifier: Apache-2.0

//go:build unix

package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/sessiontest"
	"github.com/Beamfall/corvint/internal/testinventory"
)

// TIV-V1-002, TIV-V1-006 to TIV-V1-009: corvint affected --summary (the
// verdict corvint.affected returns), corvint watch and corvint test read the
// committed inventory and print the same reasons for each state, run no
// lister and leave every repository file unchanged; only inventory refresh
// writes it.
func TestTIVV1002ReadersPrintReasonsAndWriteNothing(t *testing.T) {
	root, index := watchModule(t)
	if err := os.MkdirAll(filepath.Join(root, ".corvint"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, filepath.FromSlash(sessiontest.CommandsPath)), []byte(sessionDeclaration("")), 0o644); err != nil {
		t.Fatal(err)
	}
	commitAll(t, root, "declare")
	base := gitOutput(t, root, "rev-parse", "HEAD")
	inventoryPath := filepath.Join(root, filepath.FromSlash(testinventory.Path))
	refresh := func() {
		t.Helper()
		if code, _, stderr := inventoryCLI(t, "--root", root, "inventory", "refresh"); code != 0 {
			t.Fatalf("refresh: %s", stderr)
		}
	}
	edit := func(name, content string) {
		t.Helper()
		writeInventoryFiles(t, root, map[string]string{name: content})
	}
	cases := []struct {
		name    string
		prepare func()
		reasons []string
		full    string
	}{
		{"absent", func() {}, []string{testinventory.ReasonAbsent}, testinventory.ReasonAbsent},
		{"fresh", refresh, []string{}, ""},
		{"unassigned", func() {
			edit("p/p_test.go", "package p\n\nimport \"testing\"\n\nfunc TestPass(t *testing.T) {}\n\nfunc TestFail(t *testing.T) { t.Fail() }\n\nfunc TestMore(t *testing.T) {}\n")
		}, []string{testinventory.ReasonUnassigned}, ""},
		{"stale", func() {
			refresh()
			edit("go.sum", "")
		}, []string{testinventory.ReasonStale}, testinventory.ReasonStale},
		{"invalid", func() {
			data, err := os.ReadFile(inventoryPath)
			if err != nil {
				t.Fatal(err)
			}
			edit(testinventory.Path, string(data[:len(data)/2]))
		}, []string{testinventory.ReasonInvalid}, testinventory.ReasonInvalid},
	}
	for _, c := range cases {
		c.prepare()
		if gitOutput(t, root, "status", "--porcelain") != "" {
			commitAll(t, root, c.name)
		}
		before := treeDigest(t, root)
		check := func(surface string, verdict testinventory.Verdict) {
			t.Helper()
			reasons := verdict.Reasons()
			if !reflect.DeepEqual(reasons, c.reasons) {
				t.Errorf("%s %s: reasons %v, want %v (%+v)", c.name, surface, reasons, c.reasons, verdict)
			}
			if len(verdict.Instances) != 1 || verdict.Instances[0].Key != "go@." || verdict.Instances[0].Full != (c.full != "") || verdict.Instances[0].Reason != c.full {
				t.Errorf("%s %s: instances %+v", c.name, surface, verdict.Instances)
			}
		}

		code, stdout, stderr := runCLI(t, "--root", root, "affected", "--summary")
		var summary struct {
			Inventory testinventory.Verdict `json:"inventory"`
		}
		if code != 0 || json.Unmarshal([]byte(stdout), &summary) != nil {
			t.Fatalf("%s affected --summary: exit %d %s %s", c.name, code, stdout, stderr)
		}
		check("affected --summary", summary.Inventory)
		code, stdout, stderr = runCLI(t, "--root", root, "affected", "--summary", "--format", "text")
		if code != 0 || !strings.Contains(stdout, "\ninventory ") {
			t.Errorf("%s affected --summary --format text: exit %d %q %s", c.name, code, stdout, stderr)
		}

		code, stdout, stderr = watchCLI(t, "", "--root", root, "watch", "--index", index, "--once", "--dry-run", "--packages", "p")
		events := watchEvents(t, stdout)
		if code != 0 || len(events) != 1 {
			t.Fatalf("%s watch: exit %d %s %s", c.name, code, stdout, stderr)
		}
		raw, _ := json.Marshal(events[0].Inventory)
		var watched testinventory.Verdict
		if err := json.Unmarshal(raw, &watched); err != nil {
			t.Fatal(err)
		}
		check("watch", watched)

		code, stdout, stderr = runCLI(t, "--root", root, "test", "--base", base, "--dry-run", "--format", "json")
		var planned struct {
			Inventory testinventory.Verdict `json:"inventory"`
		}
		if code != 0 || json.Unmarshal([]byte(stdout), &planned) != nil || !strings.Contains(stderr, "corvint-test: INVENTORY ") {
			t.Fatalf("%s test --dry-run: exit %d %s %s", c.name, code, stdout, stderr)
		}
		check("test --dry-run", planned.Inventory)

		if after := treeDigest(t, root); after != before {
			t.Fatalf("%s: a read command changed the repository", c.name)
		}
	}
}
