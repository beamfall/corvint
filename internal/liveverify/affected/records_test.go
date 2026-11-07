// SPDX-License-Identifier: Apache-2.0
package affected

import (
	"context"
	"slices"
	"testing"
)

// AFP-V0-2743 (V1-4933): a new record entry, a sealed CEM or a build-log
// entry, selects only the units that read the entry or list its record
// directory. A fixture-root token that does not resolve from the unit's
// directory (`docs/build-log/` in a package that never climbs to the root)
// no longer selects its unit, nor does a token that climbs to an ancestor of
// the directory (`../.corvint`, `../docs`). A token that names
// the record directory from the root, one that names the entry itself, a unit
// that locates the root at run time and names the directory, and a declared
// read scope still select, so no real reader is dropped. Removing the record
// branch of resolves brings back the fixture-root reader of the build-log
// entry (the V1-1820 regression) and loses the locating unit's literal
// witness; removing the ancestor check of resolvesRecord brings back the
// ancestor reader of both entries.
func TestRecordEntriesSelectOnlyTheirReaders_AFPV2743(t *testing.T) {
	const sealed = ".corvint/changes/0123456789abcdef0123456789abcdef01234567.cem.json"
	const entry = "docs/build-log/2026-11-06-selgo.md"
	unit := func(id, dir string, tokens ...string) Unit {
		return Unit{ID: "go:" + id, Sources: []string{dir + "/a.go"}, Tests: []string{dir + "/a_test.go"}, PathTokens: tokens}
	}
	units := []Unit{
		unit("ancestor", "anc", "../.corvint", "../docs"),
		unit("lister", "list", "../.corvint/changes", "../docs/build-log"),
		unit("named", "named", "../"+sealed, "../"+entry),
		unit("temp", "temp", ".corvint/changes/", "docs/build-log/"),
		unit("other", "other", "../docs/specs"),
	}
	locator := unit("locator", "loc", ".corvint/changes", "docs/build-log")
	locator.UnboundedReads = "calls os.Getwd"
	scoped := unit("scoped", "scoped")
	scoped.ReadScoped, scoped.ReadScope = true, []string{".corvint/changes/", "docs/build-log/"}
	units = append(units, locator, scoped)
	graph, err := Build(context.Background(), "", fake{name: "go", units: units})
	if err != nil {
		t.Fatal(err)
	}
	for _, dirty := range []string{sealed, entry} {
		t.Run(dirty, func(t *testing.T) {
			plan := Select(graph, []string{dirty})
			witness := map[string]string{}
			for _, selection := range plan.Selected {
				witness[selection.UnitID] = selection.Witness.Kind
			}
			for _, id := range []string{"go:lister", "go:named", "go:locator"} {
				if witness[id] != WitnessPathLiteralReader {
					t.Errorf("%s: witness %q, want %s (a real reader)", id, witness[id], WitnessPathLiteralReader)
				}
			}
			if witness["go:scoped"] != WitnessDeclaredReadScope {
				t.Errorf("go:scoped: witness %q, want %s", witness["go:scoped"], WitnessDeclaredReadScope)
			}
			for _, id := range []string{"go:ancestor", "go:temp", "go:other"} {
				if kind := witness[id]; kind == WitnessPathLiteralReader {
					t.Errorf("%s is a %s of %s: its token reaches no file of the record directory", id, kind, dirty)
				}
			}
		})
	}
	// The ratchet's view (AFP-V0-2745): the undeclared test packages that read
	// a new entry of the directory by a literal, or name an ancestor of it a
	// walk could descend from; the declared one is not listed.
	for _, probe := range []string{".corvint/changes/" + "ffffffffffffffffffffffffffffffffffffffff.cem.json", "docs/build-log/0000-00-00-probe.md"} {
		got := graph.RecordReaders(probe)
		if want := []string{"go:ancestor", "go:lister", "go:locator"}; !slices.Equal(got, want) {
			t.Errorf("RecordReaders(%s) = %v, want %v", probe, got, want)
		}
	}
	if got := graph.RecordReaders("docs/specs/x.md"); got != nil {
		t.Errorf("RecordReaders of a path outside a record directory = %v, want nil", got)
	}
}

// AFP-V0-2743: only an entry directly in a record directory is a record entry.
func TestRecordDirectory_AFPV2743(t *testing.T) {
	for path, want := range map[string]string{
		".corvint/changes/abc.cem.json":     ".corvint/changes/",
		".corvint/changes/.cem.json":        "",
		".corvint/changes/sub/abc.cem.json": "",
		".corvint/changes/abc.json":         "",
		"docs/build-log/2026-11-06-x.md":    "docs/build-log/",
		"docs/build-log/README.md":          "docs/build-log/",
		"docs/build-log/.md":                "",
		"docs/build-log/sub/x.md":           "",
		"docs/build-log/x.txt":              "",
		".corvint/change.cem.json":          "",
	} {
		got, ok := recordDirectory(path)
		if got != want || ok != (want != "") {
			t.Errorf("recordDirectory(%q) = %q, %v; want %q", path, got, ok, want)
		}
	}
}
