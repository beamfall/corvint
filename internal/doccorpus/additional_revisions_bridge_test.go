package doccorpus_test

import (
	"context"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/doccorpus"
	"github.com/Beamfall/corvint/internal/mcp/corpusbridge"
)

// DCP-V1-310: corvint.docs_locate and corvint.docs_recommend_tests admit an optional repository
// and carry it to the query; other tools and an empty value are refused.
func TestDCPV1310DocsLocateRepositoryArgument(t *testing.T) {
	t.Parallel()
	root, m, extra, _ := doccorpus.MultiRevisionForTest(t)
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
	r, e := corpusbridge.New(root, "corpus.json")
	if e != nil {
		t.Fatal(e.Code)
	}
	for _, tool := range r.Tools() {
		props := tool.InputSchema["properties"].(map[string]any)
		if want := tool.Name == "corvint.docs_locate" || tool.Name == "corvint.docs_recommend_tests"; (props["repository"] != nil) != want {
			t.Fatalf("repository schema on %s", tool.Name)
		}
	}
	for _, tool := range []string{"corvint.docs_locate", "corvint.docs_recommend_tests"} {
		result, _, failure, perr := r.Call(context.Background(), tool, []byte(`{"path":"src/value.go","repository":"`+extra.ID+`"}`))
		if perr != nil || failure != nil {
			t.Fatalf("%s: %v %v", tool, perr, failure)
		}
		ids := []string{}
		for _, v := range result["results"].([]any) {
			if row, ok := v.(map[string]any); ok && row["kind"] == "capability" {
				ids = append(ids, row["id"].(string))
			}
		}
		slices.Sort(ids)
		if !slices.Equal(ids, []string{"adapter:lib", "adapter:lib-old"}) {
			t.Fatalf("%s repository B returned %v", tool, ids)
		}
		names, _ := result["result_repositories"].(map[string]any)
		if names["adapter:lib"] != extra.ID {
			t.Fatalf("%s result repositories %v", tool, names)
		}
	}
	for tool, args := range map[string]string{
		"corvint.docs_locate": `{"path":"src/value.go","repository":""}`,
		"corvint.docs_search": `{"query":"quokka","repository":"` + extra.ID + `"}`,
	} {
		if _, _, _, perr := r.Call(context.Background(), tool, []byte(args)); perr == nil || perr.Code != "invalid-arguments" {
			t.Fatalf("%s %s accepted", tool, args)
		}
	}
	_, _, failure, perr := r.Call(context.Background(), "corvint.docs_locate", []byte(`{"path":"src/value.go","repository":"`+strings.Repeat("e", 40)+`"}`))
	if perr != nil || failure == nil || failure.Code != "corpus-refused" {
		t.Fatalf("undeclared repository not refused: %v %v", perr, failure)
	}
}
