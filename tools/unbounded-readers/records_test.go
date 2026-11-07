// SPDX-License-Identifier: Apache-2.0
package main

import (
	"bytes"
	"maps"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// AFP-V0-2745 (V1-4933): the record-directory ratchet fails when a Go test
// package without a declared read scope reads a record directory by a path
// literal and the record does not list it, so a new literal reader cannot
// rejoin every change's selection unnoticed (the V1-1820 regression). A
// package whose read scope is declared is bounded by that declaration and is
// never listed. A listed package that no longer reads the directory is
// reported without failing; a malformed record exits 2. An absent record is
// the empty set.
func TestAFPV2745RecordDirectoryReadersRatchet(t *testing.T) {
	const unbounded = `{"profile":"corvint-unbounded-reader-set/0","units":[],"reasons":{}}`
	const head = `{"profile":"corvint-record-readers/0","directories":`
	for name, c := range map[string]struct {
		record string // "" leaves the record absent
		code   int
		want   string
	}{
		"absent record":      {"", 1, "[lister] name the record directory docs/build-log/"},                                                                                        //corvint:fixture-path
		"listed reader":      {head + `{"docs/build-log/":{"lister":"reads every entry"}}}`, 0, ""},                                                                                //corvint:fixture-path
		"stale entry":        {head + `{"docs/build-log/":{"gone":"was a reader","lister":"reads every entry"},".corvint/changes/":{}}}`, 0, "lists [gone] under docs/build-log/"}, //corvint:fixture-path
		"other directory":    {head + `{".corvint/changes/":{"lister":"reads every entry"}}}`, 1, "[lister] name the record directory docs/build-log/"},                            //corvint:fixture-path
		"blank reason":       {head + `{"docs/build-log/":{"lister":" "}}}`, 2, "want exactly"},                                                                                    //corvint:fixture-path
		"unprobed directory": {head + `{"docs/":{"lister":"reads"}}}`, 2, "want exactly"},                                                                                          //corvint:fixture-path
		"unknown member":     {head + `{},"extra":1}`, 2, "want exactly"},
		"wrong profile":      {`{"profile":"corvint-record-readers/1","directories":{}}`, 2, "want exactly"},
	} {
		t.Run(name, func(t *testing.T) {
			root := t.TempDir()
			files := map[string]string{
				"go.mod":                         "module example.test/m\n\ngo 1.27\n",
				"lister/l.go":                    "package lister\n",
				"lister/l_test.go":               "package lister\n\nvar entries = \"../docs/build-log/\"\n", //corvint:fixture-path
				"scoped/s.go":                    "package scoped\n",
				"scoped/s_test.go":               "package scoped\n\nvar entries = \"../docs/build-log/\"\n", //corvint:fixture-path
				"docs/build-log/README.md":       "log\n",                                                    //corvint:fixture-path
				recordPath:                       unbounded,
				".corvint/test-read-scopes.json": `{"profile":"corvint-test-read-scopes/0","packages":{"scoped":["docs/build-log/"]}}`, //corvint:fixture-path
			}
			if c.record != "" {
				files[recordReadersPath] = c.record
			}
			for file, body := range files {
				path := filepath.Join(root, filepath.FromSlash(file))
				if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
					t.Fatal(err)
				}
			}
			var stdout, stderr bytes.Buffer
			code := run(root, &stdout, &stderr)
			if code != c.code || !strings.Contains(stderr.String(), c.want) {
				t.Fatalf("code %d, want %d; stderr:\n%s", code, c.code, stderr.String())
			}
			if strings.Contains(stderr.String(), "[lister scoped]") || strings.Contains(stderr.String(), "[scoped]") {
				t.Fatalf("a package with a declared read scope is listed:\n%s", stderr.String())
			}
		})
	}
}

// AFP-V0-2747 (V1-4933, review finding P2): the ratchet matches a package's
// globs and literal fragments against the record directory's entry
// namespace, not one synthetic name. A glob whose file names are constrained
// (`filepath.Glob("../docs/build-log/2026-*.md")`) and a literal fragment
// completed by a variable (`"../docs/build-log/2026-" + name`) read every new
// 2026 entry, so the ratchet fails until the record lists them. A package
// that names only docs/build-log/'s parent and walks it reads new entries
// that no selector selects it for, so it fails even when the record lists it.
func TestAFPV2747RecordRatchetMatchesTheEntryNamespace(t *testing.T) {
	const unbounded = `{"profile":"corvint-unbounded-reader-set/0","units":[],"reasons":{}}`
	const head = `{"profile":"corvint-record-readers/0","directories":{"docs/build-log/":` //corvint:fixture-path
	source := func(name, body string) string {
		return "package " + name + "\n\nimport (\n\t\"io/fs\"\n\t\"os\"\n\t\"path/filepath\"\n)\n\nvar _, _, _ = fs.ErrNotExist, os.ReadFile, filepath.Glob\n\n" + body
	}
	files := map[string]string{
		"go.mod":                             "module example.test/m\n\ngo 1.27\n",
		"globber/g.go":                       "package globber\n",
		"globber/g_test.go":                  source("globber", "var entries, _ = filepath.Glob(\"../docs/build-log/2026-*.md\")\n"), //corvint:fixture-path
		"fragment/f.go":                      "package fragment\n",
		"fragment/f_test.go":                 source("fragment", "func read(name string) ([]byte, error) { return os.ReadFile(\"../docs/build-log/2026-\" + name) }\n"), //corvint:fixture-path
		"exact/e.go":                         "package exact\n",
		"exact/e_test.go":                    source("exact", "var one, _ = filepath.Glob(\"../docs/build-log/2026-11-06-selgo.md\")\n"), //corvint:fixture-path
		"walker/w.go":                        "package walker\n",
		"walker/w_test.go":                   source("walker", "var walk = filepath.WalkDir(\"../docs\", func(string, fs.DirEntry, error) error { return nil })\n"), //corvint:fixture-path
		"docs/build-log/2026-11-06-selgo.md": "entry\n",                                                                                                             //corvint:fixture-path
		recordPath:                           unbounded,
	}
	for name, c := range map[string]struct {
		record string // "" leaves the record absent
		code   int
		want   []string
		absent []string
	}{
		"absent record":                     {"", 1, []string{"[fragment globber] name the record directory docs/build-log/", "[walker] name docs/build-log/'s parent directory"}, []string{"exact"}},                                                                                            //corvint:fixture-path
		"listed readers, walker listed too": {head + `{"fragment":"reads 2026 entries","globber":"globs 2026 entries","walker":"walks docs"}}}`, 1, []string{"[walker] name docs/build-log/'s parent directory", "lists [walker] under docs/build-log/"}, []string{"name the record directory"}}, //corvint:fixture-path
	} {
		t.Run(name, func(t *testing.T) {
			root := t.TempDir()
			all := maps.Clone(files)
			if c.record != "" {
				all[recordReadersPath] = c.record
			}
			for file, body := range all {
				path := filepath.Join(root, filepath.FromSlash(file))
				if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
					t.Fatal(err)
				}
			}
			var stdout, stderr bytes.Buffer
			code := run(root, &stdout, &stderr)
			if code != c.code {
				t.Fatalf("code %d, want %d; stderr:\n%s", code, c.code, stderr.String())
			}
			for _, want := range c.want {
				if !strings.Contains(stderr.String(), want) {
					t.Errorf("stderr lacks %q:\n%s", want, stderr.String())
				}
			}
			for _, absent := range c.absent {
				if strings.Contains(stderr.String(), absent) {
					t.Errorf("stderr names %q:\n%s", absent, stderr.String())
				}
			}
		})
	}
	// Without the walker the listed readers pass.
	root := t.TempDir()
	all := maps.Clone(files)
	delete(all, "walker/w.go")
	delete(all, "walker/w_test.go")
	all[recordReadersPath] = head + `{"fragment":"reads 2026 entries","globber":"globs 2026 entries"}}}` //corvint:fixture-path
	for file, body := range all {
		path := filepath.Join(root, filepath.FromSlash(file))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	var stdout, stderr bytes.Buffer
	if code := run(root, &stdout, &stderr); code != 0 {
		t.Fatalf("listed readers: code %d, want 0; stderr:\n%s", code, stderr.String())
	}
}

// AFP-V0-2748 (V1-4933, review fix round 2, finding P1): a package that walks
// docs/build-log/'s parent and also globs some of its entries
// (`filepath.Glob("../docs/*/2026-*.md")`) is a reader only of the entries
// its glob admits, while its walk reads every new entry: a
// `docs/build-log/2027-entry.md` selects it under no rule. Listing it in the
// record must not pass the check, and the check names the listed entry stale.
// A package that walks the parent and globs every entry
// (`../docs/build-log/*.md`) is selected for each new entry and passes once
// listed. Before AFP-V0-2748 the constrained glob masked the walker, and the
// listed package passed.
func TestAFPV2748RecordRatchetRefusesAMaskedWalker(t *testing.T) {
	const unbounded = `{"profile":"corvint-unbounded-reader-set/0","units":[],"reasons":{}}`
	const head = `{"profile":"corvint-record-readers/0","directories":{"docs/build-log/":`                           //corvint:fixture-path
	const walk = "var walk = filepath.WalkDir(\"../docs\", func(string, fs.DirEntry, error) error { return nil })\n" //corvint:fixture-path
	source := func(glob string) string {
		return "package globwalker\n\nimport (\n\t\"io/fs\"\n\t\"path/filepath\"\n)\n\n" + walk + "\nvar entries, _ = filepath.Glob(\"" + glob + "\")\n"
	}
	for name, c := range map[string]struct {
		glob   string
		code   int
		want   []string
		absent []string
	}{
		"constrained glob": {"../docs/*/2026-*.md", 1, []string{"[globwalker] name docs/build-log/'s parent directory", "lists [globwalker] under docs/build-log/"}, nil}, //corvint:fixture-path
		"covering glob":    {"../docs/build-log/*.md", 0, nil, []string{"globwalker"}},                                                                                    //corvint:fixture-path
	} {
		t.Run(name, func(t *testing.T) {
			root := t.TempDir()
			for file, body := range map[string]string{
				"go.mod":                             "module example.test/m\n\ngo 1.27\n",
				"globwalker/g.go":                    "package globwalker\n",
				"globwalker/g_test.go":               source(c.glob),
				"docs/build-log/2026-11-06-selgo.md": "entry\n", //corvint:fixture-path
				recordPath:                           unbounded,
				recordReadersPath:                    head + `{"globwalker":"globs and walks the build log"}}}`,
			} {
				path := filepath.Join(root, filepath.FromSlash(file))
				if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
					t.Fatal(err)
				}
			}
			var stdout, stderr bytes.Buffer
			code := run(root, &stdout, &stderr)
			if code != c.code {
				t.Fatalf("code %d, want %d; stderr:\n%s", code, c.code, stderr.String())
			}
			for _, want := range c.want {
				if !strings.Contains(stderr.String(), want) {
					t.Errorf("stderr lacks %q:\n%s", want, stderr.String())
				}
			}
			for _, absent := range c.absent {
				if strings.Contains(stderr.String(), absent) {
					t.Errorf("stderr names %q:\n%s", absent, stderr.String())
				}
			}
		})
	}
}
