// SPDX-License-Identifier: Apache-2.0
package affected_test

import (
	"context"
	json "encoding/json/v2"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/liveverify/affected"
	"github.com/Beamfall/corvint/internal/liveverify/affected/golang"
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

// affprecBoundaryScanner is a stand-in for the retired Tasks import-direction
// test (decision 0594): a repository-wide Go import scanner with the same
// read-kinds header, directory skips, walk and parser mode.
func affprecBoundaryScanner(t *testing.T) string {
	t.Helper()
	body, err := os.ReadFile(filepath.Join("testdata", "boundary-scanner.go.txt"))
	if err != nil {
		t.Fatal(err)
	}
	return string(body)
}

// affprecHelpSource is a stand-in for the retired Tasks command help source
// (decision 0594): display text embedded verbatim beside inline usage strings
// that carry path examples.
const affprecHelpSource = `package cli

import _ "embed"

// queueSnapshotUsage is display text, embedded verbatim so examples carry no executable path provenance (AFP-V0-9002).
//
//go:embed helptext/queue-snapshot.txt
var queueSnapshotUsage string

// archiveRestoreUsage is display text, embedded verbatim so examples carry no executable path provenance (AFP-V0-9002).
//
//go:embed helptext/archive-restore.txt
var archiveRestoreUsage string

var usage = map[string]string{
	"queue snapshot":  queueSnapshotUsage,
	"archive restore": archiveRestoreUsage,
	"queue parity":    "tool queue parity --against FILE   (compares <git common dir>/taskman/head.json with FILE; example: tool queue parity --against ~/lane-inputs/queue-snapshot.json)",
}
`

const (
	affprecQueueSnapshotHelp  = "tool queue snapshot [--out FILE] [--worktree DIR]   (--worktree reads DIR/.taskman; example: tool queue snapshot --worktree ../lane --out /tmp/lane-snapshot.json)\n"
	affprecArchiveRestoreHelp = "tool archive restore FILE   (example: tool archive restore ../queue-export.tar)\n"
)

// AFP-V0-9001 / V1-4036: exercise a repository-wide boundary scanner. Its Go-file domain excludes hidden JSON but includes all source
// files (including foreign _test.go) and module boundaries. A real JSON reader
// remains selected for precisely the path removed from the boundary's domain.
func TestBoundaryKindBound_AFPV09001(t *testing.T) {
	root := affprecFiles(t, map[string]string{
		"go.mod":                             "module example.test/m\n",
		"internal/boundary/boundary_test.go": affprecBoundaryScanner(t),
		"reader/r_test.go":                   "package reader\nimport \"os\"\nfunc read(){ _,_ = os.ReadFile(\"../.corvint/data.json\") }\n",
		".corvint/data.json":                 "{}\n",
		"other/o.go":                         "package other\n", "other/o_test.go": "package other\n",
	})
	g := affprecGraph(t, root, golang.New())
	const boundary = "go:example.test/m/internal/boundary"
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

// AFP-V0-9000/9001: the boundary scanner opens a .go alias even when its
// target has another suffix. Only the target changes; the alias has no sibling
// Go source and supplies no package import edge that could rescue selection.
func TestBoundarySymlinkTarget_AFPV09001(t *testing.T) {
	for _, target := range []string{"data/imports.txt", "data/imports[1].txt", "testdata/imports.txt", "data/chain.txt"} {
		t.Run(target, func(t *testing.T) {
			root := affprecFiles(t, map[string]string{
				"go.mod":                             "module example.test/m\n",
				"internal/boundary/boundary_test.go": affprecBoundaryScanner(t),
				target:                               "package probe\n",
			})
			if err := os.Mkdir(filepath.Join(root, "probe"), 0700); err != nil {
				t.Fatal(err)
			}
			// The probe directory's relative path to the target, computed so no
			// parent-only literal makes the package's tokens root-relative.
			upTarget, err := filepath.Rel(filepath.Join(root, "probe"), filepath.Join(root, filepath.FromSlash(target)))
			if err != nil {
				t.Fatal(err)
			}
			if target == "data/chain.txt" {
				if err := os.Symlink(upTarget, filepath.Join(root, "probe/hop.txt")); err != nil {
					t.Skipf("symlinks unavailable: %v", err)
				}
			}
			alias := filepath.Join(root, "probe/check.go")
			linkTarget := upTarget
			if target == "data/chain.txt" {
				linkTarget = "hop.txt"
			}
			if err := os.Symlink(linkTarget, alias); err != nil {
				t.Skipf("symlinks unavailable: %v", err)
			}
			const boundary = "go:example.test/m/internal/boundary"
			for _, body := range []string{"package probe\n", "package probe\nimport _ \"example.test/m/internal/boundary/store\"\n"} {
				if err := os.WriteFile(filepath.Join(root, filepath.FromSlash(target)), []byte(body), 0600); err != nil {
					t.Fatal(err)
				}
				// Check the alias still reads exactly the changed target bytes.
				if got, err := os.ReadFile(alias); err != nil || string(got) != body {
					t.Fatalf("alias content %q, error %v", got, err)
				}
				g := affprecGraph(t, root, golang.New())
				if got := affprecIDs(affected.Select(g, []string{target})); !slices.Contains(got, boundary) {
					t.Fatalf("boundary scanner of changed symlink target %s omitted: %v", target, got)
				}
				if got := affprecIDs(affected.Select(g, []string{"unrelated/data.json"})); slices.Contains(got, boundary) {
					t.Fatalf("resolved alias made the boundary scanner unbounded: %v", got)
				}
				if target == "data/chain.txt" && !slices.Contains(affprecIDs(affected.Select(g, []string{"probe/hop.txt"})), boundary) {
					t.Fatal("intermediate alias retargeting lost the boundary scanner")
				}
			}
		})
	}
}

// AFP-V0-9000: creating an alias in a previously source-free directory,
// changing its target contents and retargeting it must invalidate the graph.
func TestBoundaryAliasCache_AFPV09000(t *testing.T) {
	if runtime.GOOS != "linux" && runtime.GOOS != "darwin" {
		t.Skip("graph-cache change-time stamps require Linux or Darwin")
	}
	pinnable(t)
	root := affprecFiles(t, map[string]string{
		"go.mod":                             "module example.test/m\n",
		"internal/boundary/boundary_test.go": affprecBoundaryScanner(t),
		"data/imports.txt":                   "package probe\n",
		"data/other.txt":                     "package other\n",
	})
	cache := privateCacheDir(t)
	build := func(hit bool, target string) {
		t.Helper()
		g, outcome, err := affected.BuildCached(context.Background(), root, cache, graphCacheIdentity, golang.New())
		if err != nil {
			t.Fatal(err)
		}
		if outcome.Hit != hit || (!hit && !outcome.Stored) {
			t.Fatalf("cache hit=%v: %+v", hit, outcome)
		}
		if target != "" && !slices.Contains(affprecIDs(affected.Select(g, []string{target})), "go:example.test/m/internal/boundary") {
			t.Fatalf("cached graph lost the boundary reader of %s", target)
		}
	}
	build(false, "")
	build(true, "")
	if err := os.Mkdir(filepath.Join(root, "probe"), 0700); err != nil {
		t.Fatal(err)
	}
	alias := filepath.Join(root, "probe/check.go")
	if err := os.Symlink("../data/imports.txt", alias); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	build(false, "data/imports.txt")
	build(true, "data/imports.txt")
	if err := os.WriteFile(filepath.Join(root, "data/imports.txt"), []byte("package probe\nimport _ \"example.test/m/internal/boundary/store\"\n"), 0600); err != nil {
		t.Fatal(err)
	}
	build(false, "data/imports.txt")
	if err := os.Remove(alias); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("../data/other.txt", alias); err != nil {
		t.Fatal(err)
	}
	build(false, "data/other.txt")
}

// AFP-V0-9002 / V1-3992: reproduce a fresh importer of an embedded help source.
// Help examples stay byte-identical while the embedded files remain source
// dependencies. Reading the same JSON through an executable literal still
// selects its reader; an edit of the importer's own test selects only itself.
func TestHelpTextProvenance_AFPV09002(t *testing.T) {
	files := map[string]string{
		"go.mod":                               "module github.com/Beamfall/corvint\n",
		"internal/helpsrc/cli/command_help.go": affprecHelpSource,
		"internal/helpsrc/cli/helptext/queue-snapshot.txt":  affprecQueueSnapshotHelp,
		"internal/helpsrc/cli/helptext/archive-restore.txt": affprecArchiveRestoreHelp,
		"new/n.go":         "package new\nimport _ \"github.com/Beamfall/corvint/internal/helpsrc/cli\"\n",
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
		if !slices.Contains(affprecIDs(affected.Select(g, []string{"internal/helpsrc/cli/helptext/" + name})), importer) {
			t.Errorf("embed %s did not reach importer", name)
		}
	}
	// A production parent-path read of the same JSON retains propagation.
	files["internal/helpsrc/cli/reader.go"] = "package cli\nimport \"os\"\nfunc read(){ _,_ = os.ReadFile(\"../.corvint/data.json\") }\n"
	real := affprecGraph(t, affprecFiles(t, files), golang.New())
	if !slices.Contains(affprecIDs(affected.Select(real, []string{".corvint/data.json"})), importer) {
		t.Fatal("production reader's importer omitted")
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
