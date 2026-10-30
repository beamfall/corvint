package corpusbridge

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/doccorpus"
)

// sourceFixture builds a schema /2 corpus over a docs scope and a Go source
// scope, the shape GitHub #1972 reports, and writes it to corpus.json.
func sourceFixture(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	files := map[string]string{
		"docs/cart.md": "# Cart\n\nThe cart totals the order.\n",
		"src/cart.go":  "package src\n\n// Cart holds the order.\ntype Cart struct{}\n\n// TotalDue sums the order.\nfunc TotalDue() int { return 1 }\n",
	}
	for p, data := range files {
		if err := os.MkdirAll(filepath.Dir(filepath.Join(root, p)), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(root, p), []byte(data), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	git := func(args ...string) string {
		t.Helper()
		c := exec.Command("git", args...)
		c.Dir = root
		b, err := c.CombinedOutput()
		if err != nil {
			t.Fatalf("git: %v %s", err, b)
		}
		return strings.TrimSpace(string(b))
	}
	git("init", "-q")
	git("add", "docs", "src")
	git("-c", "user.name=test", "-c", "user.email=test@invalid", "commit", "-qm", "src")
	m, err := doccorpus.InventoryScopes(context.Background(), root, git("rev-parse", "HEAD"), []string{"docs", "src"}, doccorpus.ManifestSchemaV2, "2026-10-28T00:00:00Z")
	if err != nil {
		t.Fatal(err)
	}
	a, err := doccorpus.Build(context.Background(), root, m)
	if err != nil {
		t.Fatal(err)
	}
	data, err := doccorpus.Encode(a)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "corpus.json"), data, 0o600); err != nil {
		t.Fatal(err)
	}
	return root
}

// DCP-V1-650, DCP-V1-651 (V1-3070, GitHub #1972): docs_locate takes a query in
// place of path or paths, answers with module and symbol subjects only, and
// lists each result's line anchors; its kind filter names module or symbol.
func TestDCPV1650DocsLocateQuery(t *testing.T) {
	root := sourceFixture(t)
	registry, err := New(root, "corpus.json")
	if err != nil {
		t.Fatal(err)
	}
	structured, _, failure, protocol := registry.Call(context.Background(), "corvint.docs_locate", []byte(`{"query":"TotalDue"}`))
	if failure != nil || protocol != nil {
		t.Fatalf("locate query failed: %v %v", failure, protocol)
	}
	results := structured["results"].([]any)
	if structured["operation"] != "locate" || len(results) == 0 {
		t.Fatalf("locate query receipt: %v", structured)
	}
	locations := structured["result_locations"].(map[string]any)
	first := results[0].(map[string]any)
	if first["kind"] != "symbol" || first["name"] != "TotalDue" {
		t.Fatalf("TotalDue should rank first: %v", first)
	}
	for _, v := range results {
		subject := v.(map[string]any)
		if kind := subject["kind"]; kind != "module" && kind != "symbol" {
			t.Fatalf("non-source result %v", subject)
		}
		anchors, ok := locations[subject["id"].(string)].([]any)
		if !ok {
			t.Fatalf("result %v has no locations", subject["id"])
		}
		for _, x := range anchors {
			l := x.(map[string]any)
			if l["path"] != "src/cart.go" || l["start_line"].(float64) < 1 || l["end_line"].(float64) < l["start_line"].(float64) {
				t.Fatalf("location %v is not a line anchor in src/cart.go", l)
			}
		}
	}
	if anchors := locations[first["id"].(string)].([]any); len(anchors) == 0 || anchors[0].(map[string]any)["start_line"].(float64) != 7 {
		t.Fatalf("TotalDue anchor = %v, want line 7", anchors)
	}
	// The kind filter narrows to modules.
	structured, _, failure, protocol = registry.Call(context.Background(), "corvint.docs_locate", []byte(`{"query":"cart","kind":"module"}`))
	if failure != nil || protocol != nil {
		t.Fatalf("module locate failed: %v %v", failure, protocol)
	}
	if len(structured["results"].([]any)) == 0 {
		t.Fatalf("module locate found nothing: %v", structured)
	}
	for _, v := range structured["results"].([]any) {
		if kind := v.(map[string]any)["kind"]; kind != "module" {
			t.Fatalf("module filter returned %v", v)
		}
	}
	// The path form is unchanged: no ranks and no locations.
	structured, _, failure, protocol = registry.Call(context.Background(), "corvint.docs_locate", []byte(`{"path":"src/cart.go"}`))
	if failure != nil || protocol != nil || len(structured["results"].([]any)) == 0 || structured["result_locations"] != nil || structured["result_ranks"] != nil {
		t.Fatalf("path locate changed: %v %v %v", structured, failure, protocol)
	}
	for _, arguments := range []string{
		`{"query":"cart","path":"src/cart.go"}`,
		`{"query":"cart","path":""}`,
		`{"query":"cart","paths":["src/cart.go"]}`,
		`{"query":""}`,
		`{"query":"cart","kind":"file"}`,
		`{"query":"cart","kind":["symbol","claim"]}`,
		`{"path":"src/cart.go","kind":"symbol"}`,
		`{"query":"cart","repository":"` + strings.Repeat("a", 40) + `"}`,
	} {
		if _, _, _, protocol := registry.Call(context.Background(), "corvint.docs_locate", []byte(arguments)); protocol == nil || protocol.Code != "invalid-arguments" {
			t.Fatalf("%s admitted: %v", arguments, protocol)
		}
	}
	// The locate schema advertises the optional query and the closed kind enum.
	for _, tool := range registry.Tools() {
		if tool.Name != "corvint.docs_locate" {
			continue
		}
		properties := tool.InputSchema["properties"].(map[string]any)
		if _, ok := properties["query"]; !ok || len(tool.InputSchema["required"].([]any)) != 0 {
			t.Fatalf("docs_locate schema = %v", tool.InputSchema)
		}
		kind := properties["kind"].(map[string]any)["oneOf"].([]any)[0].(map[string]any)
		if !slices.Equal(kind["enum"].([]string), []string{"module", "symbol"}) {
			t.Fatalf("docs_locate kind = %v", kind)
		}
		return
	}
	t.Fatal("docs_locate is not advertised")
}
