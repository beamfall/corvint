package bridge

import (
	"bytes"
	"fmt"
	"path/filepath"
	"strings"
	"testing"
)

// NGI-V0-305: additive import-graph tool, closed arguments and named refusals.
func TestGraphImportsToolAdmission(t *testing.T) {
	root := makeRepository(t)
	revision := strings.TrimSpace(string(gitOutput(t, root, "rev-parse", "HEAD")))
	legacy, err := New(root)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := legacy.Call(t.Context(), ToolGraphImports, []byte(`{}`)); err == nil || err.Code != "unsupported-tool" {
		t.Fatal(err)
	}
	for _, constructor := range []func(string) (*Registry, *Error){NewTaskReview, NewTaskReviewLSP, NewFlows} {
		existing, err := constructor(root)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := existing.Call(t.Context(), ToolGraphImports, []byte(`{}`)); err == nil || err.Code != "unsupported-tool" {
			t.Fatal(err)
		}
	}
	registry, err := NewImportGraph(root)
	if err != nil {
		t.Fatal(err)
	}
	for _, item := range []struct{ request, code string }{
		{`{}`, "graph-invalid-revision"},
		{`{"revision":"HEAD"}`, "graph-invalid-revision"},
		{`{"revision":"` + strings.Repeat("1", 40) + `"}`, "graph-unknown-revision"},
		{`{"revision":"` + revision + `","offset":-1}`, "invalid-arguments"},
		{`{"revision":"` + revision + `","offset":null}`, "invalid-arguments"},
		{`{"revision":"` + revision + `","kinds":null}`, "invalid-arguments"},
		{`{"revision":"` + revision + `","kinds":"bad"}`, "graph-invalid-kinds"},
		{`{"revision":"` + revision + `","extra":true}`, "invalid-arguments"},
		{`{"revision":"` + revision + `","revision":"` + revision + `"}`, "invalid-arguments"},
		{`{"revision":"` + revision + `","offset":9999}`, "invalid-arguments"},
	} {
		if _, err := registry.Call(t.Context(), ToolGraphImports, []byte(item.request)); err == nil || err.Code != item.code {
			t.Fatalf("%s: %v", item.request, err)
		}
	}
	request := []byte(fmt.Sprintf(`{"revision":%q}`, revision))
	before := rootDigest(t, root)
	result, err := registry.Call(t.Context(), ToolGraphImports, request)
	if err != nil || result.State != "READY" || result.Repository.CommitRevision != revision {
		t.Fatalf("%+v %v", result, err)
	}
	first, err := result.CanonicalJSON()
	if err != nil {
		t.Fatal(err)
	}
	result, err = registry.Call(t.Context(), ToolGraphImports, request)
	if err != nil {
		t.Fatal(err)
	}
	second, err := result.CanonicalJSON()
	if err != nil || !bytes.Equal(first, second) {
		t.Fatal("non-deterministic MCP result", err)
	}
	if after := rootDigest(t, root); after != before {
		t.Fatal("graph wrote repository state")
	}
	// The object-only read never observes the checkout, so a dirty file is
	// neither counted nor labelled CLEAN.
	writeFile(t, filepath.Join(root, "internal/widget/widget.go"), "package widget\n\nfunc Value() int { return 2 }\n")
	result, err = registry.Call(t.Context(), ToolGraphImports, request)
	if err != nil || result.State != "READY" || result.Repository.WorktreeState != "NOT_OBSERVED" || result.Repository.DirtyPathCount != 0 {
		t.Fatalf("%+v %v", result.Repository, err)
	}
	if _, err := result.CanonicalJSON(); err != nil {
		t.Fatal(err)
	}
}
