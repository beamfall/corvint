// SPDX-License-Identifier: Apache-2.0
package affected

import (
	"context"
	"path"
	"slices"
	"strings"
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
	// The ratchet's view (AFP-V0-2745, AFP-V0-2747): the undeclared test
	// packages that read a new entry of the directory by a literal, and,
	// apart, those that name an ancestor of it a walk could descend from; the
	// declared one is not listed, nor the one naming a single entry.
	for _, directory := range []string{SealedChangesDirectory, BuildLogDirectory} {
		readers, walkers := graph.RecordReaders(directory)
		if want := []string{"go:lister", "go:locator"}; !slices.Equal(readers, want) {
			t.Errorf("RecordReaders(%s) readers = %v, want %v", directory, readers, want)
		}
		if want := []string{"go:ancestor"}; !slices.Equal(walkers, want) {
			t.Errorf("RecordReaders(%s) walkers = %v, want %v", directory, walkers, want)
		}
	}
	if readers, walkers := graph.RecordReaders("docs/specs/"); readers != nil || walkers != nil {
		t.Errorf("RecordReaders of a directory that is no record directory = %v, %v, want nil", readers, walkers)
	}
}

// AFP-V0-2747 (V1-4933, review finding P2): the ratchet matches each literal
// and glob against a record directory's whole entry namespace, not one
// synthetic name, so a glob that admits some entries (`2026-*.md`,
// `0*.cem.json`, an unanchored `docs/build-log/*.md`) and a literal
// fragment completed at the entry position (`docs/build-log/2026-`) are
// readers of new entries. A glob or literal that names one entry, and a glob
// whose names never end in the entry suffix, are not. A unit whose ancestor
// literal joins no literal that reaches the directory is a walker; one whose
// literals join it is a reader (AFP-V0-2746). With recordProbes reduced to
// its fixed probe, the constrained glob and fragment readers are lost.
func TestRecordReadersMatchTheEntryNamespace_AFPV2747(t *testing.T) {
	unit := func(id, dir string, tokens []string, globs ...string) Unit {
		slices.Sort(tokens)
		slices.Sort(globs)
		return Unit{ID: "go:" + id, Sources: []string{dir + "/a.go"}, Tests: []string{dir + "/a_test.go"}, PathTokens: tokens, PathGlobs: globs}
	}
	graph, err := Build(context.Background(), "", fake{name: "go", units: []Unit{
		unit("constrained", "constrained", nil, "/docs/build-log/2026-*.md", "/.corvint/changes/0*.cem.json"),
		unit("unanchored", "unanchored", nil, "docs/build-log/*.md", ".corvint/changes/*"),
		unit("fragment", "fragment", []string{"../docs/build-log/2026-", "../.corvint/changes/0"}),
		unit("joined", "joined", []string{"../docs", "build-log", "../.corvint", "changes"}),
		unit("exact", "exact", []string{"../docs/build-log/2026-11-06-selgo.md", "../.corvint/changes/0123.cem.json"}, "/docs/build-log/2026-11-06-selgo.md"),
		unit("json", "json", nil, "/docs/build-log/*.json", "/.corvint/changes/*.md"),
		unit("walker", "walker", []string{"../docs", "../.corvint", "README.md"}),
	}})
	if err != nil {
		t.Fatal(err)
	}
	for _, directory := range []string{SealedChangesDirectory, BuildLogDirectory} {
		readers, walkers := graph.RecordReaders(directory)
		if want := []string{"go:constrained", "go:fragment", "go:joined", "go:unanchored"}; !slices.Equal(readers, want) {
			t.Errorf("RecordReaders(%s) readers = %v, want %v", directory, readers, want)
		}
		if want := []string{"go:walker"}; !slices.Equal(walkers, want) {
			t.Errorf("RecordReaders(%s) walkers = %v, want %v", directory, walkers, want)
		}
	}
	for _, c := range []struct {
		pattern, suffix string
		want            bool
	}{
		{"2026-*.md", ".md", true},
		{"*", ".md", true},
		{"*.m?", ".md", true},
		{"?.md", ".md", true},
		{"2026-??-??-*", ".md", true},
		{"[0-9a-f]*.cem.json", ".cem.json", true},
		{"*[^x]", ".md", true},
		{"[é]*", ".md", true},
		{"*.json", ".md", false},
		{"?", ".md", false},
		{"[^.]md", ".md", false},
		{"README.md", ".md", false},
		{`\*.md`, ".md", false},
		{"[", ".md", false},
		{"*.md", ".cem.json", false},
	} {
		name, ok := globEntry(c.pattern, c.suffix)
		if ok != c.want {
			t.Errorf("globEntry(%q, %q) = %q, %v; want %v", c.pattern, c.suffix, name, ok, c.want)
			continue
		}
		if matched, _ := path.Match(c.pattern, name); ok && (!matched || !strings.HasSuffix(name, c.suffix) || len(name) <= len(c.suffix) || strings.Contains(name, "/")) {
			t.Errorf("globEntry(%q, %q) = %q: not an entry name the pattern matches", c.pattern, c.suffix, name)
		}
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

// AFP-V0-2746 (V1-4933, review finding P1): a unit that reaches a record
// directory by joining separate literals onto an ancestor literal
// (`filepath.Join("../docs", "build-log", name)`) still selects on a new entry
// of it, so the ancestor narrowing of AFP-V0-2743 drops no composed reader. A
// unit whose literals join the ancestor to another directory (`build`,
// `change`) or to nothing leaves the ancestor narrowed; one that computes the
// directory's name at run time is AFP-V0-2747's walker. Removing the join
// from reachesRecord drops every joined unit.
func TestRecordEntryReadersJoinSeparateLiterals_AFPV2746(t *testing.T) {
	const sealed = ".corvint/changes/0123456789abcdef0123456789abcdef01234567.cem.json"
	const entry = "docs/build-log/2026-11-06-selgo.md"
	unit := func(id, dir string, tokens ...string) Unit {
		slices.Sort(tokens)
		return Unit{ID: "go:" + id, Sources: []string{dir + "/a.go"}, Tests: []string{dir + "/a_test.go"}, PathTokens: tokens}
	}
	graph, err := Build(context.Background(), "", fake{name: "go", units: []Unit{
		unit("joined", "reader", "../docs", "build-log", "entry.md", "../.corvint", "changes"),
		unit("deep", "internal/deep", "../../docs", "../../.corvint", "build-log/", "changes/"),
		unit("rooted", "rooted", "/docs", "/.corvint", "build-log", "changes"),
		unit("fragment", "fragment", "../docs", "build", "../.corvint", "change"),
		unit("ancestor", "anc", "../.corvint", "../docs"),
	}})
	if err != nil {
		t.Fatal(err)
	}
	for _, dirty := range []string{sealed, entry} {
		witness := map[string]string{}
		for _, selection := range Select(graph, []string{dirty}).Selected {
			witness[selection.UnitID] = selection.Witness.Kind
		}
		for _, id := range []string{"go:joined", "go:deep", "go:rooted"} {
			if witness[id] != WitnessPathLiteralReader {
				t.Errorf("%s: %s witness %q, want %s: its literals join the record directory", dirty, id, witness[id], WitnessPathLiteralReader)
			}
		}
		for _, id := range []string{"go:fragment", "go:ancestor"} {
			if kind := witness[id]; kind == WitnessPathLiteralReader {
				t.Errorf("%s: %s is a %s: no literal of it names the record directory's last component", dirty, id, kind)
			}
		}
	}
	for _, c := range []struct {
		tokens, components []string
		want               bool
	}{
		{[]string{"build-log"}, []string{"build-log"}, true},
		{[]string{"x/build-log/y"}, []string{"build-log"}, true},
		{[]string{"build-logs", "abuild-log"}, []string{"build-log"}, false},
		{[]string{".corvint"}, []string{".corvint", "changes"}, false},
		{[]string{".corvint", "../changes"}, []string{".corvint", "changes"}, true},
	} {
		if got := joinsComponents(c.tokens, c.components); got != c.want {
			t.Errorf("joinsComponents(%q, %q) = %v, want %v", c.tokens, c.components, got, c.want)
		}
	}
}
