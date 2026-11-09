// SPDX-License-Identifier: Apache-2.0
package affected

import (
	"context"
	json "encoding/json/v2"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

const addedCEM = SealedChangesDirectory + "1111111111111111111111111111111111111111.cem.json"
const addedLog = BuildLogDirectory + "2026-11-09-probe.md"

func recordWrite(t *testing.T, root, name, body string) {
	t.Helper()
	name = filepath.Join(root, filepath.FromSlash(name))
	if err := os.MkdirAll(filepath.Dir(name), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(name, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
}

func recordGit(t *testing.T, root string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-C", root, "-c", "user.name=test", "-c", "user.email=test@example.invalid", "-c", "commit.gpgsign=false", "-c", "maintenance.auto=false", "-c", "gc.auto=0"}, args...)...)
	cmd.Env = append(os.Environ(), "GIT_CONFIG_NOSYSTEM=1", "GIT_CONFIG_GLOBAL=/dev/null")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v: %s", args, err, out)
	}
	return strings.TrimSpace(string(out))
}

func recordFixture(t *testing.T) (string, string, *Graph) {
	t.Helper()
	root := t.TempDir()
	var units []Unit
	scopes := map[string]RecordEntryScope{}
	for _, name := range []string{"corpusbinding", "wire", "cemchain", "cli", "declared", "exact", "unknown"} {
		unit := Unit{ID: "go:" + name, Tests: []string{name + "/a_test.go"}, PathTokens: []string{"../.corvint/changes", "../docs/build-log"}}
		reads := []string{}
		body := "package reader\nvar names = []string{\"../.corvint/changes\", \"../docs/build-log\"}\n"
		switch name {
		case "corpusbinding", "wire":
			// Real counterparts enumerate all CEMs: binding_test.go verifyCorpus
			// and corpus_test.go corpusFailures (AFP-V0-3420).
			body += "func read() { os.ReadDir(names[0]) }\n"
			reads = []string{SealedChangesDirectory}
		case "exact":
			body += "var exact = \"" + addedLog + "\"\n"
			reads = []string{addedLog}
		case "declared":
			unit.ReadScoped, unit.ReadScope = true, []string{SealedChangesDirectory, BuildLogDirectory}
			reads = []string{SealedChangesDirectory + "fixed.cem.json"}
		}
		recordWrite(t, root, unit.Tests[0], body)
		digest, err := RecordEntrySourceDigest(root, unit)
		if err != nil {
			t.Fatal(err)
		}
		if name != "unknown" {
			scopes[unit.ID] = RecordEntryScope{SourceSHA256: digest, Reads: reads, Reason: "fixture audit: fixed names except the two corpus enumerators"}
		}
		units = append(units, unit)
	}
	for _, name := range []string{"batch-precheck", "dogfood-cache", "check-line-citations", "gen-documentation-paths", "gen-spec-requirements", "lane-scripts", "requirement-id-collisions", "listing"} {
		unit := Unit{ID: "shell:" + name, Tests: []string{"fixture-scripts/" + name + "_test.sh"}, PathTokens: []string{SealedChangesDirectory, BuildLogDirectory}}
		body, reads := "#!/bin/sh\nprintf '%s' 'docs/build-log/' '.corvint/changes/'\n", []string{}
		if name == "listing" {
			body = "#!/bin/sh\nfind docs/build-log -name '*.md'\n"
			reads = []string{BuildLogDirectory}
		}
		recordWrite(t, root, unit.Tests[0], body)
		digest, err := RecordEntrySourceDigest(root, unit)
		if err != nil {
			t.Fatal(err)
		}
		scopes[unit.ID] = RecordEntryScope{SourceSHA256: digest, Reads: reads, Reason: "fixture paths or an explicit listing"}
		units = append(units, unit)
	}
	data, err := json.Marshal(map[string]any{"profile": RecordEntryScopesProfile, "units": scopes})
	if err != nil {
		t.Fatal(err)
	}
	recordWrite(t, root, RecordEntryScopesPath, string(data))
	recordWrite(t, root, SealedChangesDirectory+"fixed.cem.json", "{}\n")
	recordGit(t, root, "init", "-q")
	recordGit(t, root, "add", ".")
	recordGit(t, root, "commit", "-qm", "fixture")
	base := recordGit(t, root, "rev-parse", "HEAD")
	graph, err := Build(context.Background(), root, fake{name: "go", units: units[:7]}, fake{name: "shell", units: units[7:]})
	if err != nil {
		t.Fatal(err)
	}
	return root, base, graph
}

func recordSelections(graph *Graph, dirty string) []string {
	var ids []string
	for _, selected := range Select(graph, []string{dirty}).Selected {
		ids = append(ids, selected.UnitID)
	}
	slices.Sort(ids)
	return ids
}

// AFP-V0-3420, AFP-V0-3422: the status is A before narrowing, and the positive
// controls first prove all readers were reached. Exact-file and entry-set
// readers remain; fixture-only/name-only readers and a fixed sibling scope do
// not. Modifications and deletions preserve the original selections.
func TestAddedRecordEntryReaders_AFPV3420(t *testing.T) {
	root, base, graph := recordFixture(t)
	dirty := []string{addedCEM, addedLog}
	for _, name := range dirty {
		recordWrite(t, root, name, "new\n")
	}
	recordGit(t, root, "add", ".")
	if got := AddedRecordEntries(context.Background(), "git", root, base, dirty); !slices.Equal(got, dirty) {
		t.Fatalf("fixture did not reach Git A: %v", got)
	}
	measured := graph.WithRecordEntryBase(context.Background(), "git", root, base, dirty)
	for file, want := range map[string][]string{
		addedCEM: {"go:corpusbinding", "go:unknown", "go:wire"},
		addedLog: {"go:exact", "go:unknown", "shell:listing"},
	} {
		if baseline := recordSelections(graph, file); len(baseline) != 15 {
			t.Fatalf("reader positive control %s: %v", file, baseline)
		}
		if got := recordSelections(measured, file); !slices.Equal(got, want) {
			t.Errorf("added %s: %v; want %v", file, got, want)
		}
	}
	firstBase := base
	recordGit(t, root, "commit", "-qm", "add records")
	base = recordGit(t, root, "rev-parse", "HEAD")
	for _, status := range []string{"M", "D"} {
		for _, name := range dirty {
			if status == "M" {
				recordWrite(t, root, name, "changed\n")
			} else if err := os.Remove(filepath.Join(root, name)); err != nil {
				t.Fatal(err)
			}
		}
		measured = graph.WithRecordEntryBase(context.Background(), "git", root, base, dirty)
		// A graph measured against the record-free base sees additions; reusing
		// it for the current base must not keep those exclusions.
		reused := graph.WithRecordEntryBase(context.Background(), "git", root, firstBase, dirty).WithRecordEntryBase(context.Background(), "git", root, base, dirty)
		for _, name := range dirty {
			if got, want := recordSelections(measured, name), recordSelections(graph, name); !slices.Equal(got, want) {
				t.Errorf("%s %s narrowed: %v; want %v", status, name, got, want)
			}
			if got, want := recordSelections(reused, name), recordSelections(graph, name); !slices.Equal(got, want) {
				t.Errorf("reused %s %s narrowed: %v; want %v", status, name, got, want)
			}
		}
	}
}

// AFP-V0-3421: malformed status/declarations, missing bindings, a new test
// added to a unit, and unparseable Go source never exclude a reader.
func TestAddedRecordEntryUnknownFailsClosed_AFPV3421(t *testing.T) {
	for _, raw := range []string{"A\x00" + addedLog, "A\x00" + addedLog + "\x00junk", "?\x00" + addedLog + "\x00", "A\x00../escape.md\x00"} {
		if got := addedRecordEntries([]byte(raw), []string{addedLog}); got != nil {
			t.Fatalf("malformed status admitted: %q => %v", raw, got)
		}
	}
	for _, mode := range []string{"malformed declaration", "unknown declaration member", "stale source", "unparseable source", "new test", "dirty declaration", "unknown base", "untracked"} {
		t.Run(mode, func(t *testing.T) {
			root, base, graph := recordFixture(t)
			recordWrite(t, root, addedLog, "new\n")
			if mode != "untracked" {
				recordGit(t, root, "add", addedLog)
			}
			dirty := []string{addedLog}
			switch mode {
			case "malformed declaration":
				recordWrite(t, root, RecordEntryScopesPath, "{")
			case "unknown declaration member":
				recordWrite(t, root, RecordEntryScopesPath, `{"profile":"corvint-record-entry-read-scopes/0","units":{},"typo":true}`)
			case "stale source":
				recordWrite(t, root, "cli/a_test.go", "package reader\nvar changed = true\n")
			case "unparseable source":
				recordWrite(t, root, "cli/a_test.go", "package reader\nfunc broken(")
			case "new test":
				recordWrite(t, root, "cli/new_test.go", "package reader\n")
				unit := graph.units["go:cli"]
				unit.Tests = append(unit.Tests, "cli/new_test.go")
				graph.units[unit.ID] = unit
			case "dirty declaration":
				dirty = append(dirty, RecordEntryScopesPath)
			case "unknown base":
				base = "not-a-commit"
			}
			measured := graph.WithRecordEntryBase(context.Background(), "git", root, base, dirty)
			if !slices.Contains(recordSelections(measured, addedLog), "go:cli") {
				t.Fatal("unknown input excluded cli")
			}
		})
	}
}
