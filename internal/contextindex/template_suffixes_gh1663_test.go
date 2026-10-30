package contextindex

import (
	"context"
	"testing"
)

// IDX-SNAP-V0-109: GitHub #1663. A Rake file is Ruby, so it is indexed with Ruby symbols and hash
// comment markers; the web template suffixes enter the source and term tables as searchable text
// with no symbol extractor.
func TestTemplateAndRakeSuffixesAreIndexed_GH1663(t *testing.T) {
	root := t.TempDir()
	testGit(t, root, "init", "-q", "-b", "main")
	files := map[string]string{
		"lib/tasks/seed.rake":          "# feature:seed-demo\ndef seed_profiles\n  Profile.new\nend\n",
		"app/views/home.html.erb":      "<p><%= render_greeting %></p>\n",
		"public/index.html":            "<h1>greeting</h1>\n",
		"public/legacy.htm":            "<h1>greeting</h1>\n",
		"app/views/card.haml":          "%p greeting\n",
		"app/views/list.slim":          "p greeting\n",
		"web/templates/row.hbs":        "<td>{{greeting}}</td>\n",
		"web/components/Greeting.vue":  "<template><p>greeting</p></template>\n",
		"web/components/Banner.svelte": "<p>greeting</p>\n",
		"assets/unknown.xyz":           "greeting\n",
	}
	for path, content := range files {
		writeTestFile(t, root, path, content)
	}
	testGit(t, root, "add", "-A")
	testGit(t, root, "commit", "-qm", "templates")
	index, err := Build(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
	for path := range files {
		_, admitted := index.Sources[path]
		if want := path != "assets/unknown.xyz"; admitted != want {
			t.Errorf("%s admitted=%v want %v", path, admitted, want)
		}
	}
	found := false
	for _, symbol := range index.Symbols {
		if symbol.Path == "lib/tasks/seed.rake" && symbol.Name == "seed_profiles" {
			found = true
		}
		if _, template := files[symbol.Path]; template && symbol.Path != "lib/tasks/seed.rake" {
			t.Errorf("template %s carries symbol %+v; templates have no extractor", symbol.Path, symbol)
		}
	}
	if !found {
		t.Errorf("seed.rake carries no Ruby symbol seed_profiles: %+v", index.Symbols)
	}
	marked := false
	for _, markers := range index.Markers {
		for _, marker := range markers {
			marked = marked || marker.Path == "lib/tasks/seed.rake" && marker.Line == 1
		}
	}
	if !marked {
		t.Errorf("seed.rake hash-comment feature marker is missing: %+v", index.Markers)
	}
	if got := SymbolRegionLanguage("lib/tasks/seed.rake"); got != "ruby" {
		t.Errorf("SymbolRegionLanguage(.rake)=%q want ruby", got)
	}
}
