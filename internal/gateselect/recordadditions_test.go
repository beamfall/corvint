// SPDX-License-Identifier: Apache-2.0
package gateselect

import (
	"context"
	json "encoding/json/v2"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/liveverify/affected"
	"github.com/Beamfall/corvint/internal/liveverify/affected/golang"
)

// AFP-V0-3423: re-observe A instead of trusting the receipt. Name-only, fixed
// sibling and declared directory readers narrow in both selectors; exact
// names, real enumeration and unknowns remain. M and an absent base never narrow.
// The selectors now run everything for the sealed CEM (NCM-V1-006, V1-5424);
// the build-log entry keeps its narrowing.
func TestAddedRecordEntryBothSelectors_AFPV3423(t *testing.T) {
	const cem = ".corvint/changes/1111111111111111111111111111111111111111.cem.json" //corvint:fixture-path
	const log = "docs/build-log/2026-11-09-probe.md"                                 //corvint:fixture-path
	root := writeFixture(t, map[string]string{
		"go.mod":                "module " + fixtureModule + "\n\ngo 1.27\n",
		"names/a_test.go":       "package names\nvar names = []string{\"../.corvint/changes\", \"../docs/build-log\"}\n",                                    //corvint:fixture-path
		"fixed/a_test.go":       "package fixed\nvar names = []string{\"../.corvint/changes\", \"../docs/build-log\"}\n",                                    //corvint:fixture-path
		"listing/a_test.go":     "package listing\nimport \"os\"\nfunc read() { os.ReadDir(\"../.corvint/changes\"); os.ReadDir(\"../docs/build-log\") }\n", //corvint:fixture-path
		"unknown/a_test.go":     "package unknown\nvar names = []string{\"../.corvint/changes\", \"../docs/build-log\"}\n",                                  //corvint:fixture-path
		"exact/a_test.go":       "package exact\nvar names = []string{\"../" + cem + "\", \"../" + log + "\"}\n",
		"declared/a_test.go":    "package declared\n",
		affected.ReadScopesPath: `{"profile":"corvint-test-read-scopes/0","packages":{"declared":[".corvint/changes/","docs/build-log/"]}}`, //corvint:fixture-path
	})
	write := func(name string, body []byte) {
		t.Helper()
		name = filepath.Join(root, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(name), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(name, body, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	git := func(args ...string) string {
		t.Helper()
		cmd := exec.Command("git", append([]string{"-C", root, "-c", "user.name=test", "-c", "user.email=test@example.invalid", "-c", "commit.gpgsign=false"}, args...)...)
		cmd.Env = append(os.Environ(), "GIT_CONFIG_NOSYSTEM=1", "GIT_CONFIG_GLOBAL=/dev/null")
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("git %v: %v %s", args, err, out)
		}
		return strings.TrimSpace(string(out))
	}
	graph, err := affected.BuildPartial(context.Background(), root, golang.New())
	if err != nil {
		t.Fatal(err)
	}
	scopes := map[string]affected.RecordEntryScope{}
	for _, name := range []string{"names", "fixed", "listing", "exact", "declared"} {
		id := "go:" + fixtureModule + "/" + name
		unit, ok := graph.Unit(id)
		if !ok {
			t.Fatal(id)
		}
		digest, err := affected.RecordEntrySourceDigest(root, unit)
		if err != nil {
			t.Fatal(err)
		}
		reads := []string{}
		switch name {
		case "listing":
			reads = []string{affected.SealedChangesDirectory, affected.BuildLogDirectory}
		case "exact":
			reads = []string{cem, log}
		case "fixed", "declared":
			reads = []string{affected.SealedChangesDirectory + "fixed.cem.json", affected.BuildLogDirectory + "fixed.md"}
		}
		scopes[id] = affected.RecordEntryScope{SourceSHA256: digest, Reads: reads, Reason: "fixture read audit"}
	}
	body, err := json.Marshal(map[string]any{"profile": affected.RecordEntryScopesProfile, "units": scopes})
	if err != nil {
		t.Fatal(err)
	}
	write(affected.RecordEntryScopesPath, body)
	git("init", "-q")
	git("add", ".")
	git("commit", "-qm", "fixture")
	base := git("rev-parse", "HEAD")
	write(cem, []byte("{}\n"))
	write(log, []byte("entry\n"))
	git("add", ".")
	for _, mode := range []string{"A", "unknown", "M"} {
		if mode == "M" {
			git("commit", "-qm", "records")
			base = git("rev-parse", "HEAD")
			write(cem, []byte("changed\n"))
			write(log, []byte("changed\n"))
		}
		for _, dirty := range []string{cem, log} {
			var receipt Receipt
			receipt.Provider.Go.State = "RUNNABLE"
			receipt.Plan.Scope = "BOUNDED"
			receipt.Plan.Dirty = []string{dirty}
			receipt.Range.Base = base
			if mode == "unknown" {
				receipt.Range.Base = ""
			}
			measured := graph.WithRecordEntryBase(context.Background(), "git", root, receipt.Range.Base, []string{dirty})
			want := []string{"exact", "listing", "unknown"}
			if mode != "A" {
				want = []string{"declared", "exact", "fixed", "listing", "names", "unknown"}
			}
			var planner []string
			for _, entry := range affected.Select(measured, []string{dirty}).Selected {
				planner = append(planner, strings.TrimPrefix(entry.UnitID, "go:"+fixtureModule+"/"))
			}
			slices.Sort(planner)
			if !slices.Equal(planner, want) {
				t.Errorf("%s planner %s: %v; want %v", mode, dirty, planner, want)
			}
			for name, selectFn := range map[string]func(Receipt, string, string) []string{"gate": Select, "structural": SelectStructural, "qualified": SelectQualified} {
				lines := selectFn(receipt, fixtureModule, root)
				got := strings.TrimPrefix(lines[len(lines)-1], "run ")
				var packages []string
				for _, pkg := range want {
					packages = append(packages, fixtureModule+"/"+pkg)
				}
				expected := strings.Join(packages, " ")
				switch {
				case dirty == cem:
					// NCM-V1-006 (V1-5424): a sealed CEM is Corvint
					// configuration; every selector runs everything.
					expected = "FALLBACK corvint-config " + cem
				}
				if got != expected {
					t.Errorf("%s %s %s: %v; want %v", mode, name, dirty, lines, expected)
				}
			}
		}
	}
}
