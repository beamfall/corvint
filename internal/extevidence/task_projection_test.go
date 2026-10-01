// SPDX-License-Identifier: AGPL-3.0-or-later

package extevidence

import (
	"bytes"
	"context"
	"encoding/json"
	"slices"
	"testing"
)

// TCP-V0-054: prioritize either root-subject endpoint before bounding, without
// promoting a same-path foreign endpoint or relying on provider record order.
func TestTaskPathRelationsSubjectBeforeBound(t *testing.T) {
	t.Parallel()
	makeItem := func(from, to Endpoint1) item {
		return item{provider: "gopls", link: link{
			from:       endpoint{repository: from.Repository, path: from.Path},
			to:         endpoint{repository: to.Repository, path: to.Path},
			structured: &Relation1{From: from, To: to, Type: "gopls:uses-definition", Evidence: EvidenceInferred},
		}}
	}
	rows := []item{
		makeItem(Endpoint1{Repository: "root", Path: "a.go"}, Endpoint1{Repository: "foreign", Path: "subject.go"}),
		makeItem(Endpoint1{Repository: "root", Path: "b.go"}, Endpoint1{Repository: "root", Path: "c.go"}),
		makeItem(Endpoint1{Repository: "root", Path: "z-caller.go"}, Endpoint1{Repository: "root", Path: "subject.go"}),
		makeItem(Endpoint1{Repository: "root", Path: "subject.go"}, Endpoint1{Repository: "root", Path: "z-definition.go"}),
	}
	project := func(input []item, subject endpoint, limit int) map[string]any {
		section := map[string]any{"omitted": map[string]any{}, "untrusted_text_fields": []any{}}
		addPathRelations(section, slices.Clone(input), subject, limit)
		return section
	}
	subject := endpoint{repository: "root", path: "subject.go"}
	selected := project(rows, subject, 2)
	got := selected["path_relations"].([]any)
	if len(got) != 2 || selected["omitted"].(map[string]any)["path_relations"] != 2 {
		t.Fatalf("bound and omissions: %v", selected)
	}
	for _, raw := range got {
		relation := raw.(map[string]any)["relation"].(map[string]any)
		from, to := relation["from"].(map[string]any), relation["to"].(map[string]any)
		if !(from["repository"] == "root" && from["path"] == "subject.go" || to["repository"] == "root" && to["path"] == "subject.go") {
			t.Fatalf("unrelated/foreign relation displaced a caller or definition: %v", relation)
		}
	}
	slices.Reverse(rows)
	if !bytes.Equal(canonical(t, selected), canonical(t, project(rows, subject, 2))) {
		t.Fatal("record order changed bounded projection")
	}
	legacy := project(rows, endpoint{}, 2)["path_relations"].([]any)
	if legacy[0].(map[string]any)["relation"].(map[string]any)["from"].(map[string]any)["path"] != "a.go" {
		t.Fatal("no-subject projection changed legacy order")
	}
	zero := project(rows, subject, 0)
	if len(zero["path_relations"].([]any)) != 0 || zero["omitted"].(map[string]any)["path_relations"] != 4 {
		t.Fatalf("zero limit: %v", zero)
	}
}

// TCP-V0-054: the task API derives the root repository identity after binding
// and retains verified provenance and the legacy projection without a subject.
func TestInlineTaskSectionBoundRootSubject(t *testing.T) {
	t.Parallel()
	r := newRepository(t)
	record, err := Decode1(lspRecord(t, r))
	if err != nil {
		t.Fatal(err)
	}
	// Make the first lexical relation unrelated to the subject but still
	// admitted via the other anchor; both blob pins remain valid.
	record.Relations[0].To = record.Relations[0].From
	data, err := json.Marshal(record)
	if err != nil {
		t.Fatal(err)
	}
	anchors := []string{"pkg/main.go", "pkg/main_test.go"}
	legacy := InlineSection(context.Background(), r.index(), "lsp:gopls", data, "", anchors, 1)
	withSubject := InlineTaskSection(context.Background(), r.index(), "lsp:gopls", data, "", anchors, "pkg/main_test.go", 1)
	row := withSubject["path_relations"].([]any)[0].(map[string]any)
	if row["relation"].(map[string]any)["type"] != "gopls:uses-definition" || row["relation_state"] != RelationFresh || row["authority"] != Authority {
		t.Fatalf("subject relation lost or provenance changed: %v", row)
	}
	if bytes.Equal(canonical(t, legacy["path_relations"]), canonical(t, withSubject["path_relations"])) {
		t.Fatal("subject did not change selection before the bound")
	}
	delete(legacy, "path_relations")
	delete(withSubject, "path_relations")
	if !bytes.Equal(canonical(t, legacy), canonical(t, withSubject)) {
		t.Fatal("task priority changed other external members")
	}
	if !bytes.Equal(canonical(t, InlineSection(context.Background(), r.index(), "lsp:gopls", data, "", anchors, 1)), canonical(t, InlineTaskSection(context.Background(), r.index(), "lsp:gopls", data, "", anchors, "", 1))) {
		t.Fatal("empty subject changed legacy inline bytes")
	}
}
