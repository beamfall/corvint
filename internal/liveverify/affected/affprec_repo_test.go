// SPDX-License-Identifier: Apache-2.0
package affected_test

import (
	"context"
	json "encoding/json/v2"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/liveverify/affected"
	"github.com/Beamfall/corvint/internal/liveverify/affected/golang"
	"github.com/Beamfall/corvint/internal/liveverify/affected/languages"
	"github.com/Beamfall/corvint/internal/liveverify/affected/shell"
)

func affprecFiles(t *testing.T, files map[string]string) string {
	t.Helper()
	root := t.TempDir()
	for name, body := range files {
		full := filepath.Join(root, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(full), 0700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte(body), 0600); err != nil {
			t.Fatal(err)
		}
	}
	return root
}
func affprecGraph(t *testing.T, root string, plugins ...affected.Language) *affected.Graph {
	t.Helper()
	g, err := affected.Build(context.Background(), root, plugins...)
	if err != nil {
		t.Fatal(err)
	}
	return g
}
func affprecIDs(p affected.Plan) []string {
	var ids []string
	for _, s := range p.Selected {
		ids = append(ids, s.UnitID)
	}
	slices.Sort(ids)
	return ids
}
func affprecRead(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(repositoryRoot(t), filepath.FromSlash(path)))
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

// AFP-V0-9001 / V1-4036: exercise the actual boundary test, not a replacement
// scanner. Its Go-file domain excludes hidden JSON but includes all source
// files (including foreign _test.go) and module boundaries. A real JSON reader
// remains selected for precisely the path removed from the boundary's domain.
func TestBoundaryKindBound_AFPV09001(t *testing.T) {
	root := affprecFiles(t, map[string]string{
		"go.mod":                          "module example.test/m\n",
		"internal/tasks/boundary_test.go": affprecRead(t, "internal/tasks/boundary_test.go"),
		"reader/r_test.go":                "package reader\nimport \"os\"\nfunc read(){ _,_ = os.ReadFile(\"../.corvint/data.json\") }\n",
		".corvint/data.json":              "{}\n",
		"other/o.go":                      "package other\n", "other/o_test.go": "package other\n",
	})
	g := affprecGraph(t, root, golang.New())
	const boundary = "go:example.test/m/internal/tasks"
	p := affected.Select(g, []string{".corvint/data.json"})
	if got := affprecIDs(p); !slices.Equal(got, []string{"go:example.test/m/reader"}) {
		t.Fatalf("JSON reader selection: %v", got)
	}
	for _, path := range []string{"other/o.go", "other/o_test.go", "nested/go.mod"} {
		if !slices.Contains(affprecIDs(affected.Select(g, []string{path})), boundary) {
			t.Errorf("true boundary reader of %s omitted", path)
		}
	}
}

// AFP-V0-9002 / V1-3992: reproduce a fresh importer of the actual help source.
// Help examples stay byte-identical while the embedded files remain source
// dependencies. Reading the same JSON through an executable literal still
// selects its reader; an edit of the importer's own test selects only itself.
func TestHelpTextProvenance_AFPV09002(t *testing.T) {
	files := map[string]string{
		"go.mod":                             "module github.com/Beamfall/corvint\n",
		"internal/tasks/cli/command_help.go": affprecRead(t, "internal/tasks/cli/command_help.go"),
		"internal/tasks/cli/helptext/queue-snapshot.txt":  affprecRead(t, "internal/tasks/cli/helptext/queue-snapshot.txt"),
		"internal/tasks/cli/helptext/archive-restore.txt": affprecRead(t, "internal/tasks/cli/helptext/archive-restore.txt"),
		"new/n.go":         "package new\nimport _ \"github.com/Beamfall/corvint/internal/tasks/cli\"\n",
		"new/n_test.go":    "package new\n",
		"reader/r_test.go": "package reader\nimport \"os\"\nfunc read(){ _,_ = os.ReadFile(\"../.corvint/data.json\") }\n",
	}
	root := affprecFiles(t, files)
	g := affprecGraph(t, root, golang.New())
	const importer = "go:github.com/Beamfall/corvint/new"
	if got := affprecIDs(affected.Select(g, []string{".corvint/data.json"})); !slices.Equal(got, []string{"go:github.com/Beamfall/corvint/reader"}) {
		t.Fatalf("help-only importer became a reader: %v", got)
	}
	if got := affprecIDs(affected.Select(g, []string{"new/n_test.go"})); !slices.Equal(got, []string{importer}) {
		t.Fatalf("new importer's tests select %v", got)
	}
	for _, name := range []string{"queue-snapshot.txt", "archive-restore.txt"} {
		if !slices.Contains(affprecIDs(affected.Select(g, []string{"internal/tasks/cli/helptext/" + name})), importer) {
			t.Errorf("embed %s did not reach importer", name)
		}
	}
	// A production parent-path read of the same JSON retains propagation.
	files["internal/tasks/cli/reader.go"] = "package cli\nimport \"os\"\nfunc read(){ _,_ = os.ReadFile(\"../.corvint/data.json\") }\n"
	real := affprecGraph(t, affprecFiles(t, files), golang.New())
	if !slices.Contains(affprecIDs(affected.Select(real, []string{".corvint/data.json"})), importer) {
		t.Fatal("production reader's importer omitted")
	}
}

// AFP-V0-9003 / V1-4497 / V1-4143 / V1-4535: replay the reported change kinds
// against the live repository graph. Counts and residual frontiers are measured,
// never erased. The fixture pairs each change kind with its true reader.
func TestPrecisionReplay_AFPV09003(t *testing.T) {
	g := affprecGraph(t, repositoryRoot(t), languages.All()...)
	sets := map[string][]string{
		"heavy-only":       {"script/heavy.sh"},
		"test-only":        {"internal/tasks/authority/preparation_observe_linux_test.go", "internal/tasks/authority/preparation_observe_test.go", "internal/tasks/store/claim_head_test.go"},
		"declaration-spec": {".corvint/shell-foreign-callers.json", ".corvint/test-read-scopes.json", "docs/specs/affected-plan-v0.md"},
	}
	for name, paths := range sets {
		p := affected.Select(g, paths)
		t.Logf("%s selected=%d scope=%s unknown=%+v", name, len(p.Selected), p.Scope, p.Unknown)
		for _, s := range p.Selected {
			if s.UnitID == "go:github.com/Beamfall/corvint/internal/tasks" && name == "declaration-spec" {
				t.Errorf("%s selects the boundary test via %s", name, s.Witness.Kind)
			}
		}
		if name == "heavy-only" && len(p.Selected) >= 86 {
			t.Errorf("heavy-only selected %d, want below reported 86", len(p.Selected))
		}
	}
	files := map[string]string{
		"go.mod":                 "module example.test/m\n",
		"testreader/r_test.go":   "package testreader\nimport \"os\"\nfunc read(){_,_=os.ReadFile(\"../changed/c_test.go\")}\n",
		"changed/c_test.go":      "package changed\n",
		"scriptreader/r_test.go": "package scriptreader\nimport \"os\"\nfunc read(){_,_=os.ReadFile(\"../script/heavy.sh\")}\n",
		"datareader/r_test.go":   "package datareader\nimport \"os\"\nfunc read(){_,_=os.ReadFile(\"../.corvint/data.json\")}\n",
		"specreader/r_test.go":   "package specreader\nimport \"os\"\nfunc read(){_,_=os.ReadFile(\"../docs/specs/example.md\")}\n",
	}
	fixture := affprecGraph(t, affprecFiles(t, files), golang.New())
	for path, id := range map[string]string{"changed/c_test.go": "testreader", "script/heavy.sh": "scriptreader", ".corvint/data.json": "datareader", "docs/specs/example.md": "specreader"} {
		if !slices.Contains(affprecIDs(affected.Select(fixture, []string{path})), "go:example.test/m/"+id) {
			t.Errorf("true reader %s of %s omitted", id, path)
		}
	}
}

// AFP-V0-9004 / V1-4453: an uncommitted, valid data-only declaration cannot
// self-approve a complete plan. Select(path-only) is the documented conditional
// measurement, and Select(full-dirty) keeps the exact frontier and real reader.
func TestDirtyForeignCallerDeclaration_AFPV09004(t *testing.T) {
	files := map[string]string{
		"go.mod":             "module example.test/m\n",
		"script/run.sh":      "#!/bin/sh\necho ok\n",
		"script/run_test.sh": "#!/bin/sh\nsh script/run.sh\n",
		"reader/r_test.go":   "package reader\nimport \"os\"\nfunc read(){ _,_ = os.ReadFile(\"../.corvint/shell-foreign-callers.json\") }\n",
	}
	root := affprecFiles(t, files)
	// Write the declaration after creating the source universe, with no commit.
	decl := map[string]any{"profile": "corvint-shell-foreign-callers/0", "scripts": map[string]any{"script/run.sh": map[string]any{"callers": []string{"script/run_test.sh"}, "reason": "test runs this script"}}}
	b, err := json.Marshal(decl)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(root, ".corvint"), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, affected.ShellForeignCallersPath), b, 0600); err != nil {
		t.Fatal(err)
	}
	g := affprecGraph(t, root, golang.New(), shell.New())
	full := affected.Select(g, []string{"script/run.sh", affected.ShellForeignCallersPath})
	if !slices.Contains(full.Unknown, affected.Unknown{Reason: affected.UnknownLanguageFrontier, Detail: affected.FrontierForeignCallersChanged}) {
		t.Fatalf("dirty declaration lost its frontier: %+v", full.Unknown)
	}
	if !slices.Contains(affprecIDs(full), "go:example.test/m/reader") {
		t.Fatal("true declaration reader omitted")
	}
	conditional := affected.Select(g, []string{"script/run.sh"})
	for _, u := range conditional.Unknown {
		if u.Detail == affected.FrontierForeignCallersChanged {
			t.Fatal("isolated measurement includes the declaration edit")
		}
	}
	if !slices.Contains(affprecIDs(conditional), "shell:script/run_test.sh") {
		t.Fatalf("true script reader omitted: %v", affprecIDs(conditional))
	}
	// Malformed declaration remains fail-closed, even for path-only measurement.
	if err := os.WriteFile(filepath.Join(root, affected.ShellForeignCallersPath), []byte(`{`), 0600); err != nil {
		t.Fatal(err)
	}
	invalid := affected.Select(affprecGraph(t, root, golang.New(), shell.New()), []string{"script/run.sh"})
	if !slices.ContainsFunc(invalid.Unknown, func(u affected.Unknown) bool { return strings.Contains(u.Detail, shell.FrontierForeignCallersInvalid) }) {
		t.Fatalf("invalid declaration lost its frontier: %+v", invalid.Unknown)
	}
}

// AFP-V0-9003 / V1-4143: a test-only edit must not fan out through display
// strings in an imported implementation. The changed test has a real file
// reader in both arms; no general exemption for foreign test files is used.
func TestTestOnlyImporterPrecision_AFPV09003(t *testing.T) {
	files := map[string]string{
		"go.mod":           "module example.test/m\n",
		"cli/help.go":      "package cli\nfunc help() string { return \"../queue-export.tar\" }\n",
		"new/n.go":         "package new\nimport _ \"example.test/m/cli\"\n",
		"new/n_test.go":    "package new\n",
		"reader/r_test.go": "package reader\nimport \"os\"\nfunc read(){_,_=os.ReadFile(\"../new/n_test.go\")}\n",
	}
	for i := range 32 {
		name := fmt.Sprintf("importer%d", i)
		files[name+"/i.go"] = "package " + name + "\nimport _ \"example.test/m/cli\"\n"
		files[name+"/i_test.go"] = "package " + name + "\n"
	}
	before := affected.Select(affprecGraph(t, affprecFiles(t, files), golang.New()), []string{"new/n_test.go"})
	files["cli/help.go"] = "package cli\nimport _ \"embed\"\n//go:embed help.txt\nvar help string\n"
	files["cli/help.txt"] = "../queue-export.tar"
	after := affected.Select(affprecGraph(t, affprecFiles(t, files), golang.New()), []string{"new/n_test.go"})
	want := []string{"go:example.test/m/new", "go:example.test/m/reader"}
	if len(before.Selected) != 34 || !slices.Equal(affprecIDs(after), want) {
		t.Fatalf("test-only before=%d after=%v, want 34 -> %v", len(before.Selected), affprecIDs(after), want)
	}
	t.Logf("test-only display-provenance replay: %d -> %d; true file reader retained", len(before.Selected), len(after.Selected))
}
