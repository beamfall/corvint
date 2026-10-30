package golang_test

import (
	"context"
	"fmt"
	"os"
	"slices"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/Beamfall/corvint/internal/liveverify/affected"
	"github.com/Beamfall/corvint/internal/liveverify/affected/golang"
)

// additiveBase is the base tree of the additive fixture: core, which ref and
// alias reference only through the identifier the change adds, unref and the
// test-only user, which reference only an existing one, and leaf, which
// imports unref.
var additiveBase = map[string]string{
	"go.mod":              "module example.test/m\n",
	"core/core.go":        "package core\n\nimport \"strings\"\n\n// Old is old.\nfunc Old() string { return strings.ToUpper(\"a\") }\n\ntype T struct{}\n",
	"core/core_test.go":   "package core\n",
	"ref/ref.go":          "package ref\n\nimport \"example.test/m/core\"\n\nvar F = core.Fresh\n",
	"alias/alias.go":      "package alias\n\nimport c \"example.test/m/core\"\n\nvar F = c.Fresh\n",
	"unref/unref.go":      "package unref\n\nimport \"example.test/m/core\"\n\nfunc Use() string { return core.Old() }\n",
	"leaf/leaf.go":        "package leaf\n\nimport \"example.test/m/unref\"\n\nvar F = unref.Use\n",
	"ref/ref_test.go":     "package ref\n",
	"alias/alias_test.go": "package alias\n",
	"unref/unref_test.go": "package unref\n",
	"leaf/leaf_test.go":   "package leaf\n",
	"user/user.go":        "package user\n",
	"user/user_test.go":   "package user\n\nimport (\n\t\"testing\"\n\n\t\"example.test/m/core\"\n)\n\nfunc TestUse(t *testing.T) { _ = core.Old() }\n",
	"tool/main.go":        "package main\n\nfunc main() {}\n",
}

const additiveCore = "package core\n\nimport \"strings\"\n\n// Old is old.\nfunc Old() string { return strings.ToUpper(\"a\") }\n\ntype T struct{}\n"

const additiveDeleted = "\x00deleted"

// additiveRun writes the base tree with head's edits applied (additiveDeleted
// removes a path), builds the graph from it, and returns the records for
// dirty, measured from the base tree, with the graph.
func additiveRun(t *testing.T, base, head map[string]string, dirty ...string) ([]affected.GoAdditive, *affected.Graph) {
	t.Helper()
	root := t.TempDir()
	files := map[string]string{}
	baseFS := fstest.MapFS{}
	for relative, body := range additiveBase {
		files[relative] = body
		baseFS[relative] = &fstest.MapFile{Data: []byte(body)}
	}
	for relative, body := range base {
		files[relative] = body
		baseFS[relative] = &fstest.MapFile{Data: []byte(body)}
	}
	for relative, body := range head {
		if body == additiveDeleted {
			delete(files, relative)
			continue
		}
		files[relative] = body
	}
	writeFiles(t, root, files)
	graph, err := affected.Build(context.Background(), root, golang.New())
	if err != nil {
		t.Fatal(err)
	}
	return golang.AdditiveRecords(baseFS, os.DirFS(root), graph, dirty), graph
}

func additiveRecordFor(t *testing.T, records []affected.GoAdditive, unit string) affected.GoAdditive {
	t.Helper()
	for _, record := range records {
		if record.Unit == unit {
			return record
		}
	}
	t.Fatalf("no record for %s in %+v", unit, records)
	return affected.GoAdditive{}
}

func trimGo(ids []string) string {
	out := make([]string, 0, len(ids))
	for _, id := range ids {
		out = append(out, strings.TrimPrefix(id, "go:example.test/m/"))
	}
	slices.Sort(out)
	return strings.Join(out, " ")
}

// TestADNV0006AddedFunctionSelectsOnlyReferencingImporters: adding an
// exported function to core narrows its importers to the two that name it
// (one through an import alias); core's own tests still run, and the importer
// that references only an existing function, its importer and the test-only
// user are excluded with the record as witness.
func TestADNV0006AddedFunctionSelectsOnlyReferencingImporters(t *testing.T) {
	head := map[string]string{"core/core.go": additiveCore + "\n// Fresh is new.\nfunc Fresh() string { return Old() }\n"}
	records, graph := additiveRun(t, nil, head, "core/core.go")
	if len(records) != 1 {
		t.Fatalf("records %+v, want one", records)
	}
	record := records[0]
	if record.State != affected.GoAdditiveNarrowed || fmt.Sprint(record.Declarations) != "[Fresh]" {
		t.Fatalf("record %+v, want NARROWED [Fresh]", record)
	}
	if got := trimGo(record.Referencing); got != "alias ref" {
		t.Fatalf("referencing %q", got)
	}
	if got := trimGo(record.Unreferenced); got != "unref user" {
		t.Fatalf("unreferenced %q", got)
	}
	plain := affected.Select(graph, []string{"core/core.go"})
	if got := trimGo(selectedIDs(plain)); got != "alias core leaf ref unref user" {
		t.Fatalf("plain selection %q", got)
	}
	narrowed := affected.Select(graph.WithGoAdditive(records), []string{"core/core.go"})
	if got := trimGo(selectedIDs(narrowed)); got != "alias core ref" {
		t.Fatalf("narrowed selection %q", got)
	}
	if narrowed.GraphDigest != plain.GraphDigest {
		t.Fatalf("records changed the graph digest")
	}
}

// TestADNV0004ModifiedExportedFunctionSelectsEveryImporter is the negative
// case: editing the body of an existing exported function is no additive
// change, so every importer and transitive importer stays selected.
func TestADNV0004ModifiedExportedFunctionSelectsEveryImporter(t *testing.T) {
	head := map[string]string{"core/core.go": strings.Replace(additiveCore, "\"a\"", "\"b\"", 1) + "\nfunc Fresh() string { return Old() }\n"}
	records, graph := additiveRun(t, nil, head, "core/core.go")
	record := additiveRecordFor(t, records, "go:example.test/m/core")
	if record.State != affected.GoAdditiveFallback || record.Reason != golang.AdditiveDeclarationEdited {
		t.Fatalf("record %+v, want FALLBACK %s", record, golang.AdditiveDeclarationEdited)
	}
	plan := affected.Select(graph.WithGoAdditive(records), []string{"core/core.go"})
	if got := trimGo(selectedIDs(plan)); got != "alias core leaf ref unref user" {
		t.Fatalf("selection %q, want every importer", got)
	}
	if len(plan.GoAdditiveExcluded) != 0 {
		t.Fatalf("fallback record excluded %+v", plan.GoAdditiveExcluded)
	}
}

// TestADNV0004AdditiveFallbacks: each change the syntactic rules cannot show
// to be additive is a FALLBACK record with its reason, and each additive form
// is NARROWED with its declarations.
func TestADNV0004AdditiveFallbacks(t *testing.T) {
	for _, test := range []struct {
		name   string
		base   map[string]string
		head   map[string]string
		dirty  []string
		unit   string
		state  string
		reason string
		decls  string
	}{
		{name: "init", head: map[string]string{"core/core.go": additiveCore + "\nfunc init() {}\n"}, reason: golang.AdditiveInit},
		{name: "method on existing type", head: map[string]string{"core/core.go": additiveCore + "\nfunc (T) M() {}\n"}, reason: golang.AdditiveExistingReceiver},
		{name: "method on new type", head: map[string]string{"core/core.go": additiveCore + "\ntype N struct{}\n\nfunc (*N) M() {}\n"}, state: affected.GoAdditiveNarrowed, decls: "[N N.M]"},
		{name: "var initializer call", head: map[string]string{"core/core.go": additiveCore + "\nvar V = Old()\n"}, reason: golang.AdditiveVarInitializer},
		{name: "var initializer receive", head: map[string]string{"core/core.go": additiveCore + "\nvar V = <-make(chan int)\n"}, reason: golang.AdditiveVarInitializer},
		{name: "var literal and const", head: map[string]string{"core/core.go": additiveCore + "\nvar V = 1\n\nconst C = \"c\"\n"}, state: affected.GoAdditiveNarrowed, decls: "[C V]"},
		{name: "import not in package", head: map[string]string{"core/core.go": strings.Replace(additiveCore, "import \"strings\"", "import (\n\t\"fmt\"\n\t\"strings\"\n)", 1) + "\nvar V = fmt.Sprint\n"}, reason: golang.AdditiveImportAdded},
		{name: "import already in package", head: map[string]string{"core/extra.go": "package core\n\nimport \"strings\"\n\nfunc Extra() string { return strings.ToLower(\"a\") }\n"}, dirty: []string{"core/extra.go"}, state: affected.GoAdditiveNarrowed, decls: "[Extra]"},
		{name: "import renamed", head: map[string]string{"core/core.go": strings.Replace(additiveCore, "import \"strings\"", "import str \"strings\"", 1)}, reason: golang.AdditiveImportChanged},
		{name: "dot import", head: map[string]string{"core/core.go": strings.Replace(additiveCore, "import \"strings\"", "import (\n\t. \"fmt\"\n\t\"strings\"\n)", 1)}, reason: golang.AdditiveSpecialImport},
		{name: "cgo", head: map[string]string{"core/core.go": strings.Replace(additiveCore, "import \"strings\"", "import (\n\t\"C\"\n\t\"strings\"\n)", 1)}, reason: golang.AdditiveCgo},
		{name: "directive", head: map[string]string{"core/core.go": additiveCore + "\n//go:noinline\nfunc Fresh() {}\n"}, reason: golang.AdditiveDirective},
		{name: "new platform file", head: map[string]string{"core/extra_linux.go": "package core\n\nfunc Extra() {}\n"}, dirty: []string{"core/extra_linux.go"}, reason: golang.AdditiveConstrainedFile},
		{name: "package clause", head: map[string]string{"core/core.go": strings.Replace(additiveCore, "package core", "package core2", 1)}, reason: golang.AdditivePackageRenamed},
		{name: "predeclared", head: map[string]string{"core/core.go": additiveCore + "\nfunc len() int { return 0 }\n"}, reason: golang.AdditivePredeclared},
		{name: "bodyless", head: map[string]string{"core/core.go": additiveCore + "\nfunc Ext() int\n"}, reason: golang.AdditiveBodyless},
		{name: "reordered", head: map[string]string{"core/core.go": "package core\n\nimport \"strings\"\n\ntype T struct{}\n\nfunc Old() string { return strings.ToUpper(\"a\") }\n"}, reason: golang.AdditiveDeclarationEdited},
		{name: "doc comment only", head: map[string]string{"core/core.go": strings.Replace(additiveCore, "Old is old", "Old is older", 1)}, reason: golang.AdditiveNoDeclaration},
		{name: "deleted file", base: map[string]string{"core/gone.go": "package core\n\nfunc Gone() {}\n"}, head: map[string]string{"core/gone.go": additiveDeleted, "core/core.go": additiveCore + "\nfunc Fresh() {}\n"}, dirty: []string{"core/core.go", "core/gone.go"}, reason: golang.AdditiveDeletedFile},
		{name: "non-Go path", head: map[string]string{"core/data.txt": "x\n", "core/core.go": additiveCore + "\nfunc Fresh() {}\n"}, dirty: []string{"core/core.go", "core/data.txt"}, reason: golang.AdditiveNotGoSource},
		{name: "embeds", base: map[string]string{"core/embed.go": "package core\n\nimport _ \"embed\"\n\n//go:embed data.txt\nvar data string\n", "core/data.txt": "x\n"}, head: map[string]string{"core/core.go": additiveCore + "\nfunc Fresh() {}\n"}, reason: golang.AdditiveEmbeds},
		{name: "main package", head: map[string]string{"tool/main.go": "package main\n\nfunc main() {}\n\nfunc helper() {}\n"}, dirty: []string{"tool/main.go"}, unit: "go:example.test/m/tool", reason: golang.AdditiveMainPackage},
		{name: "new package", head: map[string]string{"fresh/fresh.go": "package fresh\n\nfunc F() {}\n"}, dirty: []string{"fresh/fresh.go"}, unit: "go:example.test/m/fresh", reason: golang.AdditiveBaseUnreadable},
		{name: "var division", head: map[string]string{"core/core.go": additiveCore + "\nvar V = 4 / 2\n"}, reason: golang.AdditiveVarInitializer},
		{name: "var modulo", head: map[string]string{"core/core.go": additiveCore + "\nvar V = 4 % 3\n"}, reason: golang.AdditiveVarInitializer},
		{name: "var index", head: map[string]string{"core/core.go": additiveCore + "\nvar V = []int{1}[0]\n"}, reason: golang.AdditiveVarInitializer},
		{name: "var slice", head: map[string]string{"core/core.go": additiveCore + "\nvar V = \"ab\"[1:]\n"}, reason: golang.AdditiveVarInitializer},
		{name: "var dereference", head: map[string]string{"core/core.go": additiveCore + "\nvar P *int\n\nvar V = *P\n"}, reason: golang.AdditiveVarInitializer},
		{name: "var type assertion", head: map[string]string{"core/core.go": additiveCore + "\nvar I any\n\nvar V = I.(int)\n"}, reason: golang.AdditiveVarInitializer},
		{name: "var shift", head: map[string]string{"core/core.go": additiveCore + "\nvar N = -1\n\nvar V = 1 << N\n"}, reason: golang.AdditiveVarInitializer},
		{name: "var interface comparison", head: map[string]string{"core/core.go": additiveCore + "\nvar I any\n\nvar V = I == I\n"}, reason: golang.AdditiveVarInitializer},
		{name: "var field of a variable", head: map[string]string{"core/core.go": additiveCore + "\nvar P *struct{ F int }\n\nvar V = P.F\n"}, reason: golang.AdditiveVarInitializer},
		{name: "var map key variable", head: map[string]string{"core/core.go": additiveCore + "\nvar K any\n\nvar V = map[any]int{K: 1}\n"}, reason: golang.AdditiveVarInitializer},
		{name: "var safe forms", head: map[string]string{"core/core.go": additiveCore + "\ntype S struct{ F int }\n\nconst K = \"k\"\n\nvar A = []string{\"a\", \"b\"}\n\nvar M = map[string]int{K: 1, \"b\": 2}\n\nvar P = &S{F: 1}\n\nvar Q = strings.ToUpper\n\nvar N = -1 + 2*3\n\nvar L = func() int { return 4 / len(A) }\n"}, state: affected.GoAdditiveNarrowed, decls: "[A K L M N P Q S]"},
		{name: "type declared by a platform variant", base: map[string]string{"core/core_windows.go": "package core\n\ntype W struct{}\n"}, head: map[string]string{"core/core.go": additiveCore + "\ntype W struct{}\n"}, reason: golang.AdditiveNameDeclared},
		{name: "method on a platform variant's type", base: map[string]string{"core/core_windows.go": "package core\n\ntype W struct{}\n"}, head: map[string]string{"core/core.go": additiveCore + "\nfunc (W) M() {}\n"}, reason: golang.AdditiveExistingReceiver},
		{name: "function declared by a tagged variant", base: map[string]string{"core/variant.go": "//go:build windows\n\npackage core\n\nfunc Win() {}\n"}, head: map[string]string{"core/core.go": additiveCore + "\nfunc Win() {}\n"}, reason: golang.AdditiveNameDeclared},
		{name: "name declared twice in the change", head: map[string]string{"core/core.go": additiveCore + "\nfunc Fresh() {}\n", "core/extra.go": "package core\n\nfunc Fresh() {}\n"}, dirty: []string{"core/core.go", "core/extra.go"}, reason: golang.AdditiveNameDeclared},
		{name: "cgo in an unchanged platform file", base: map[string]string{"core/cgo_linux.go": "//go:build linux\n\npackage core\n\nimport \"C\"\n"}, head: map[string]string{"core/core.go": additiveCore + "\nfunc Fresh() {}\n"}, reason: golang.AdditiveCgo},
		{name: "dot importer", head: map[string]string{"dotuser/dotuser.go": "package dotuser\n\nimport . \"example.test/m/core\"\n\nvar F = Old\n", "core/core.go": additiveCore + "\nfunc Fresh() {}\n"}, reason: golang.AdditiveDotImporter + ": go:example.test/m/dotuser"},
	} {
		t.Run(test.name, func(t *testing.T) {
			dirty, unit, state := test.dirty, test.unit, test.state
			if dirty == nil {
				dirty = []string{"core/core.go"}
			}
			if unit == "" {
				unit = "go:example.test/m/core"
			}
			if state == "" {
				state = affected.GoAdditiveFallback
			}
			records, _ := additiveRun(t, test.base, test.head, dirty...)
			record := additiveRecordFor(t, records, unit)
			if record.State != state || record.Reason != test.reason {
				t.Fatalf("record %+v, want %s %q", record, state, test.reason)
			}
			if test.decls != "" && fmt.Sprint(record.Declarations) != test.decls {
				t.Fatalf("declarations %v, want %s", record.Declarations, test.decls)
			}
		})
	}
}

// TestADNV0003OnlyTestChangeIsNoCandidate: a change to a package's tests
// alone carries no record; the package's tests are selected the ordinary way.
func TestADNV0003OnlyTestChangeIsNoCandidate(t *testing.T) {
	records, _ := additiveRun(t, nil, map[string]string{"core/core_test.go": "package core\n\nfunc helper() {}\n"}, "core/core_test.go")
	if len(records) != 0 {
		t.Fatalf("records %+v, want none", records)
	}
}

// TestADNV0009UnreadableBaseFallsBack: with no measured-from commit, or a
// root that is no Git repository, every candidate is a FALLBACK
// BASE_UNREADABLE record, so the plan selects as before.
func TestADNV0009UnreadableBaseFallsBack(t *testing.T) {
	head := map[string]string{"core/core.go": additiveCore + "\nfunc Fresh() string { return Old() }\n"}
	_, graph := additiveRun(t, nil, head, "core/core.go")
	for _, commit := range []string{"", "0123456789abcdef0123456789abcdef01234567"} {
		records := golang.AdditiveChanges(context.Background(), t.TempDir(), commit, graph, []string{"core/core.go"})
		if len(records) != 1 || records[0].State != affected.GoAdditiveFallback || records[0].Reason != golang.AdditiveBaseUnreadable {
			t.Fatalf("commit %q: records %+v, want one FALLBACK %s", commit, records, golang.AdditiveBaseUnreadable)
		}
	}
}

// TestADNV0006ImporterSpellingsAndRetainedImporters: every importer file is
// parsed and its import paths unquoted, so a raw-string or escaped spelling
// and a reference from a test file alone are found; an importer that
// blank-imports the package, holds a //go:linkname directive, or has assembly
// in its directory is retained whatever it names. The importer that names
// only an existing identifier stays unreferenced.
func TestADNV0006ImporterSpellingsAndRetainedImporters(t *testing.T) {
	head := map[string]string{
		"core/core.go":            additiveCore + "\nfunc Fresh() string { return Old() }\n",
		"raw/raw.go":              "package raw\n\nimport c `example.test/m/core`\n\nvar F = c.Fresh\n",
		"escaped/escaped.go":      "package escaped\n\nimport \"example.test/m/\\x63ore\"\n\nvar F = core.Fresh\n",
		"tonly/tonly.go":          "package tonly\n",
		"tonly/tonly_test.go":     "package tonly\n\nimport (\n\t\"testing\"\n\n\t\"example.test/m/core\"\n)\n\nfunc TestFresh(t *testing.T) { _ = core.Fresh() }\n",
		"blank/blank.go":          "package blank\n\nimport _ \"example.test/m/core\"\n",
		"blank/blank_test.go":     "package blank\n",
		"link/link.go":            "package link\n\nimport (\n\t_ \"unsafe\"\n\n\t\"example.test/m/core\"\n)\n\n//go:linkname fresh example.test/m/core.fresh\nfunc fresh() string\n\nvar F = core.Old\n",
		"link/link_test.go":       "package link\n",
		"asm/asm.go":              "package asm\n\nimport \"example.test/m/core\"\n\nvar F = core.Old\n",
		"asm/asm_amd64.s":         "TEXT ·f(SB),0,$0\n\tRET\n",
		"asm/asm_test.go":         "package asm\n",
		"raw/raw_test.go":         "package raw\n",
		"escaped/escaped_test.go": "package escaped\n",
	}
	records, graph := additiveRun(t, nil, head, "core/core.go")
	record := additiveRecordFor(t, records, "go:example.test/m/core")
	if record.State != affected.GoAdditiveNarrowed {
		t.Fatalf("record %+v, want NARROWED", record)
	}
	for _, dependent := range []string{"raw", "escaped", "tonly", "blank", "link", "asm"} {
		if !slices.Contains(graph.Dependents("go:example.test/m/core"), "go:example.test/m/"+dependent) {
			t.Errorf("%s is no dependent of core in the graph", dependent)
		}
		if !slices.Contains(record.Referencing, "go:example.test/m/"+dependent) {
			t.Errorf("%s is not retained: referencing %q unreferenced %q", dependent, trimGo(record.Referencing), trimGo(record.Unreferenced))
		}
	}
	if !slices.Contains(record.Unreferenced, "go:example.test/m/unref") {
		t.Fatalf("unref is retained: unreferenced %q", trimGo(record.Unreferenced))
	}
	plan := affected.Select(graph.WithGoAdditive(records), []string{"core/core.go"})
	for _, dependent := range record.Referencing {
		if !slices.Contains(selectedIDs(plan), dependent) {
			t.Errorf("referencing importer %s is not selected", dependent)
		}
	}
}
