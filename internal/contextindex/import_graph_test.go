package contextindex

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"testing"
)

func graphRecords(t *testing.T, index *Index, filter string) ([]ImportGraphRecord, []byte) {
	t.Helper()
	kinds, err := ImportGraphKinds(filter)
	if err != nil {
		t.Fatal(err)
	}
	rows := []ImportGraphRecord{}
	var output bytes.Buffer
	encoder := json.NewEncoder(&output)
	if err := WalkImportGraph(t.Context(), index, kinds, func(row ImportGraphRecord) error {
		rows = append(rows, row)
		return encoder.Encode(row)
	}); err != nil {
		t.Fatal(err)
	}
	return rows, output.Bytes()
}

// NGI-V0-301, NGI-V0-303: every resolved edge shares impact's original evidence.
func TestImportGraphImpactParity(t *testing.T) {
	files := map[string]string{
		"go.mod":        "module example.test/graph\n\ngo 1.27.0\n",
		"lib/value.go":  "package lib\nfunc Value() int { return 1 }\n",
		"cmd/main.go":   "package main\nimport \"example.test/graph/lib\"\nfunc main() { lib.Value() }\n",
		"src/value.py":  "VALUE = 1\n",
		"src/cli.py":    "import value\n",
		"web/value.ts":  "export type Value = number;\n",
		"web/use.ts":    "import type { Value } from './value';\nexport type X = Value;\n",
		"web/run.ts":    "import { Value } from '@graph/value';\n",
		"tsconfig.json": `{"compilerOptions":{"paths":{"@graph/*":["./web/*"]}}}`,
	}
	root := impactRepositoryWithFiles(t, files)
	revision := strings.TrimSpace(testGit(t, root, "rev-parse", "HEAD"))
	index, err := BuildImportGraph(t.Context(), root, revision)
	if err != nil {
		t.Fatal(err)
	}
	rows, first := graphRecords(t, index, "")
	_, second := graphRecords(t, index, "")
	if !bytes.Equal(first, second) {
		t.Fatal("graph bytes differ")
	}
	for _, target := range []string{"lib/value.go", "src/value.py", "web/value.ts"} {
		packet, err := ImpactSyntax(index, []string{target}, 50)
		if err != nil {
			t.Fatal(err)
		}
		want := reverseImportIDs(packet)
		got := map[string]map[string]bool{}
		for _, row := range rows {
			if row.Type != "edge" || row.To != target {
				continue
			}
			if got[row.From] == nil {
				got[row.From] = map[string]bool{}
			}
			got[row.From]["imports "+row.Evidence["specifier"].(string)] = true
			if row.Revision != revision || row.Schema != "graph-imports/1" || row.Evidence["blob_hash"] != index.Sources[row.From].BlobHash {
				t.Fatalf("unbound edge: %+v", row)
			}
			line, _ := importEvidenceLine(index.Sources[row.From], row.Evidence["specifier"].(string))
			if row.Evidence["line"] != line {
				t.Fatalf("wrong site: %+v", row)
			}
		}
		if len(want) == 0 || !reflect.DeepEqual(got, want) {
			t.Fatalf("%s: graph=%v impact=%v", target, got, want)
		}
	}
	filtered, _ := graphRecords(t, index, "type-import")
	edges := 0
	for _, row := range filtered {
		if row.Type == "edge" {
			edges++
			if row.Kind != "type-import" || row.From != "web/use.ts" {
				t.Fatalf("unexpected edge: %+v", row)
			}
		}
	}
	if edges != 1 {
		t.Fatalf("type edges = %d", edges)
	}
}

// NGI-V0-301: historical content, all importers beyond impact's 50-row ceiling.
func TestImportGraphHistoricalUncapped(t *testing.T) {
	files := map[string]string{"web/value.ts": "export const value = 1;\n"}
	for i := 0; i < 65; i++ {
		files[fmt.Sprintf("web/use%02d.ts", i)] = "import { value } from './value';\n"
	}
	root := impactRepositoryWithFiles(t, files)
	revision := strings.TrimSpace(testGit(t, root, "rev-parse", "HEAD"))
	before, err := BuildImportGraph(t.Context(), root, revision)
	if err != nil {
		t.Fatal(err)
	}
	_, original := graphRecords(t, before, "")
	writeTestFile(t, root, "web/use00.ts", "export const changed = 1;\n")
	testGit(t, root, "add", ".")
	testGit(t, root, "commit", "-qm", "later content")
	writeTestFile(t, root, "web/use01.ts", "uncommitted content\n")
	historical, err := BuildImportGraph(t.Context(), root, revision)
	if err != nil {
		t.Fatal(err)
	}
	rows, after := graphRecords(t, historical, "")
	if !bytes.Equal(original, after) {
		t.Fatal("HEAD or dirty bytes contaminated historical graph")
	}
	count := 0
	for _, row := range rows {
		if row.Type == "edge" {
			count++
		}
	}
	if count != 65 {
		t.Fatalf("edges = %d, want 65", count)
	}
}

// NGI-V0-302: same language unknowns as impact; no invented dynamic edges.
func TestImportGraphUncertainty(t *testing.T) {
	root := impactRepositoryWithFiles(t, map[string]string{
		"app/models/user.rb":  "class User; end\n",
		"app/services/run.rb": "User.call\nload computed_name\n",
		"web/component.ts":    "import external from 'unknown-package';\n@Component({templateUrl: './view.html', styleUrls: ['./style.css']})\nclass C { run() { registry[key](); } }\n",
		"web/view.html":       "<div>Hello</div>\n",
		"lib/value.rs":        "pub fn value() {}\n",
		"vendor/third.ts":     "export const third = 1;\n",
	})
	index, err := BuildImportGraph(t.Context(), root, strings.TrimSpace(testGit(t, root, "rev-parse", "HEAD")))
	if err != nil {
		t.Fatal(err)
	}
	rows, first := graphRecords(t, index, "import")
	_, second := graphRecords(t, index, "import")
	if !bytes.Equal(first, second) {
		t.Fatal("uncertainty order differs")
	}
	unknowns := map[string]bool{}
	for _, row := range rows {
		if row.Type == "edge" {
			t.Fatalf("invented edge: %+v", row)
		}
		if row.Type == "uncertainty" {
			data, _ := json.Marshal(row.Unknown)
			unknowns[string(data)] = true
		}
	}
	for _, target := range []string{"app/models/user.rb", "app/services/run.rb", "web/component.ts"} {
		packet, err := ImpactSyntax(index, []string{target}, 50)
		if err != nil {
			t.Fatal(err)
		}
		for _, row := range packet["unknowns"].([]any) {
			data, _ := json.Marshal(row)
			if !unknowns[string(data)] {
				t.Fatalf("missing impact unknown: %s", data)
			}
		}
	}
	_, raw := graphRecords(t, index, "type-import")
	for _, code := range []string{"ruby-autoload-not-import", "framework-resources-unresolved", "string-keyed-di-unresolved", "bare-import-unresolved", "reverse-import-rule-unavailable", "index-excluded"} {
		if !bytes.Contains(raw, []byte(code)) {
			t.Fatalf("missing %s", code)
		}
	}
}

// NGI-V0-303: the bulk lookup retains directory/profile aliases and self edges.
func TestImportGraphLookupParity(t *testing.T) {
	files := withBeamfallSignals(webImpactFiles())
	files["pkg/__init__.py"] = "VALUE = 1\n"
	files["pkg/value.py"] = "VALUE = 1\n"
	files["pkg/user.py"] = "from . import VALUE\nfrom pkg import value\nimport pkg.value\n"
	files["lib/a.go"] = "package lib\nfunc A() {}\n"
	files["lib/b.go"] = "package lib\nfunc B() {}\n"
	files["main.go"] = "package main\nimport \"example.test/fixture/lib\"\nfunc main() { lib.A() }\n"
	files["web/self.ts"] = "import './self';\n"
	root := impactRepositoryWithFiles(t, files)
	index, err := BuildImportGraph(t.Context(), root, strings.TrimSpace(testGit(t, root, "rev-parse", "HEAD")))
	if err != nil {
		t.Fatal(err)
	}
	lookup := graphImportLookup(index)
	for target := range index.Sources {
		got, want := sortImporters(lookup(target)), reverseImporters(index, target)
		if len(got) != len(want) {
			t.Fatalf("%s: graph=%v impact=%v", target, got, want)
		}
		for i := range got {
			if got[i] != want[i] {
				t.Fatalf("%s: graph=%v impact=%v", target, got, want)
			}
		}
	}
}

// NGI-V0-304: named revision refusals and prompt propagation of stream errors.
func TestImportGraphRefusalsAndCancellation(t *testing.T) {
	root := impactRepositoryWithFiles(t, map[string]string{"value.ts": "export const value = 1;\n"})
	for _, item := range []struct{ revision, code string }{
		{"HEAD", "graph-invalid-revision"}, {"1234567", "graph-invalid-revision"},
		{strings.Repeat("1", 40), "graph-unknown-revision"},
		{strings.TrimSpace(testGit(t, root, "rev-parse", "HEAD^{tree}")), "graph-invalid-revision"},
	} {
		_, err := BuildImportGraph(t.Context(), root, item.revision)
		var detail *Error
		if !errors.As(err, &detail) || detail.Code != item.code {
			t.Fatalf("%s: %v", item.revision, err)
		}
	}
	for _, value := range []string{"unknown", "import,import", ",import"} {
		if _, err := ImportGraphKinds(value); err == nil {
			t.Fatalf("accepted %q", value)
		}
	}
	index, err := BuildImportGraph(t.Context(), root, strings.TrimSpace(testGit(t, root, "rev-parse", "HEAD")))
	if err != nil {
		t.Fatal(err)
	}
	broken := errors.New("closed consumer")
	if err := WalkImportGraph(t.Context(), index, nil, func(ImportGraphRecord) error { return broken }); !errors.Is(err, broken) {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	if err := WalkImportGraph(ctx, index, nil, func(ImportGraphRecord) error { t.Fatal("wrote after cancel"); return nil }); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
}

// NGI-V0-304: a pinned graph reads only Git objects. Neither HEAD (detached
// elsewhere or unborn) nor a dirty worktree copy of a pinned path is read.
func TestImportGraphRevisionObjectOnly(t *testing.T) {
	root := impactRepositoryWithFiles(t, map[string]string{
		"web/value.ts": "export const value = 1;\n",
		"web/user.ts":  "import { value } from './value';\nexport const user = value;\n",
	})
	revision := testGit(t, root, "rev-parse", "HEAD")
	clean, err := BuildImportGraph(t.Context(), root, revision)
	if err != nil {
		t.Fatal(err)
	}
	_, baseline := graphRecords(t, clean, "")
	writeTestFile(t, root, "web/user.ts", "export const user = 2;\n")
	writeTestFile(t, root, "web/other.ts", "import { value } from './value';\n")
	testGit(t, root, "add", ".")
	testGit(t, root, "commit", "-qm", "later")
	testGit(t, root, "checkout", "-q", "--detach")
	writeTestFile(t, root, "web/value.ts", "export const value = 'dirty';\n")
	original := worktreePinRead
	worktreePinRead = func(name string) { t.Errorf("worktree read %s", name) }
	t.Cleanup(func() { worktreePinRead = original })
	for _, state := range []string{"detached", "unborn"} {
		if state == "unborn" {
			testGit(t, root, "checkout", "-q", "--orphan", "fresh")
		}
		index, err := BuildImportGraph(t.Context(), root, revision)
		if err != nil {
			t.Fatalf("%s HEAD: %v", state, err)
		}
		if _, got := graphRecords(t, index, ""); !bytes.Equal(got, baseline) {
			t.Fatalf("%s HEAD graph differs:\n%s\nwant:\n%s", state, got, baseline)
		}
	}
}

// NGI-V0-302: every indexed path with no import rule discloses it, whatever
// its admitted suffix, so no admitted source reads as dependency-free.
func TestImportGraphRuleUnavailableCoversAdmittedSources(t *testing.T) {
	root := impactRepositoryWithFiles(t, map[string]string{
		"db/schema.sql":       "create table users (id int);\n",
		"lib/tasks/seed.rake": "task :seed do\nend\n",
		"web/a.vue":           "<script>import x from './x'</script>\n",
		"web/b.svelte":        "<script>import x from './x'</script>\n",
		"web/view.html":       "<div></div>\n",
		"web/site.css":        "@import 'base.css';\n",
		"lib/value.rs":        "pub fn value() {}\n",
		"README.md":           "# fixture\n",
		"web/x.ts":            "export const x = 1;\n",
		"pkg/value.py":        "VALUE = 1\n",
	})
	index, err := BuildImportGraph(t.Context(), root, testGit(t, root, "rev-parse", "HEAD"))
	if err != nil {
		t.Fatal(err)
	}
	rows, _ := graphRecords(t, index, "")
	disclosed := map[string]string{}
	for _, row := range rows {
		if row.Type == "uncertainty" && row.Unknown["code"] == "reverse-import-rule-unavailable" {
			disclosed[row.Unknown["path"].(string)], _ = row.Unknown["language"].(string)
		}
	}
	for name := range index.Sources {
		_, got := disclosed[name]
		if want := NonGoImpactLanguage(name) == "" && !ImpactRuleNamed(name); got != want {
			t.Fatalf("%s: disclosed=%v want %v (%v)", name, got, want, disclosed)
		}
	}
	for _, name := range []string{"db/schema.sql", "lib/tasks/seed.rake", "web/a.vue", "web/b.svelte"} {
		if _, ok := disclosed[name]; !ok {
			t.Fatalf("%s: no reverse-import-rule-unavailable in %v", name, disclosed)
		}
	}
	if disclosed["lib/tasks/seed.rake"] != "rake" {
		t.Fatalf("language: %v", disclosed)
	}
}
