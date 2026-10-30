package extevidence

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/Beamfall/corvint/internal/contextindex"
)

// pathCase returns one labelled case from the path-to-path fixture by name.
func pathCase(t *testing.T, name string) selectionCase {
	t.Helper()
	for _, c := range fixtureCases(t, pathFixtures) {
		if c.Name == name {
			return c
		}
	}
	t.Fatalf("no path case %q", name)
	return selectionCase{}
}

func runPathCase(t *testing.T, p pair, c selectionCase) (map[string]any, []byte) {
	t.Helper()
	return runSelection(t, p, fixtureRecord(t, p, pathFixtures, c), bindCase(p, c), selectionInput(c))
}

// pathRecord fills a path fixture's placeholders for a decode or impact test.
func pathRecord(t *testing.T, p pair, name string, values map[string]string) []byte {
	t.Helper()
	c := pathCase(t, "positive-cross-repository")
	c.Record = name
	for key, value := range values {
		c.Values[key] = value
	}
	data, err := os.ReadFile(fixtureRecord(t, p, pathFixtures, c))
	if err != nil {
		t.Fatal(err)
	}
	return data
}

// TestPathRowProvenance: every path row carries both endpoints with their
// own repository evidence, and relation_state is the worse side
// (EEP-V2-004, EEP-V2-009).
func TestPathRowProvenance(t *testing.T) {
	t.Parallel()
	p := newPair(t)
	selection, _ := runPathCase(t, p, pathCase(t, "positive-two-repository-fresh"))
	rows := selection["selected"].([]any)
	if len(rows) != 2 {
		t.Fatalf("selected = %v", rows)
	}
	for _, raw := range rows {
		row := raw.(map[string]any)
		for _, member := range []string{"authority", "confidence", "schema", "provider", "provider_revision", "test", "subject", "relation", "relation_type", "evidence", "verification", "limitations", "identity", "binding", "freshness", "test_revision", "source_revision", "relation_state", "crosses_repositories", "reason"} {
			if _, present := row[member]; !present {
				t.Errorf("path row lacks %s: %v", member, row)
			}
		}
		if row["schema"] != Schema2 || row["relation_state"] != RelationFresh || row["authority"] != Authority {
			t.Errorf("a selected path row must be V2, fresh, and external-provider: %v", row)
		}
		if _, entity := row["entity"]; entity {
			t.Errorf("a path row names no entity: %v", row)
		}
	}
	cross := rows[1].(map[string]any)
	test, subject := cross["test"].(map[string]any), cross["subject"].(map[string]any)
	if cross["crosses_repositories"] != true || test["repository"] != "e2e" || test["binding"] != BindingCheckout || subject["repository"] != "application" || subject["binding"] != BindingRoot {
		t.Fatalf("cross-repository sides = %v / %v", test, subject)
	}
	if cross["test_revision"] != p.e2e.head || cross["source_revision"] != p.app.head {
		t.Fatalf("each side keeps its own revision: %v", cross)
	}
	if same := rows[0].(map[string]any); same["crosses_repositories"] != false {
		t.Fatalf("a same-repository relation does not cross: %v", same)
	}
	worst := map[string]string{
		"unbound-test-repository":     RelationNotVerified,
		"unbound-source-repository":   RelationNotVerified,
		"missing-test-path":           RelationStale,
		"stale-pinned-blob":           RelationStale,
		"ambiguous-test-repository":   RelationUnresolved,
		"mismatched-checkout-history": RelationUnresolved,
	}
	for name, want := range worst {
		selection, _ := runPathCase(t, p, pathCase(t, name))
		candidates := selection["candidates"].([]any)
		if len(candidates) != 1 {
			t.Fatalf("%s: a non-qualifying relation stays visible: %v", name, candidates)
		}
		row := candidates[0].(map[string]any)
		if row["relation_state"] != want || row["blocking"] != true || row["reason"] == "" {
			t.Errorf("%s: relation_state %v blocking %v, want %s", name, row["relation_state"], row["blocking"], want)
		}
	}
}

// TestPathDeterministicAndBounded: record order never changes the advice,
// and the limit counts what it drops (EEP-V2-010).
func TestPathDeterministicAndBounded(t *testing.T) {
	t.Parallel()
	p := newPair(t)
	c := pathCase(t, "positive-two-repository-fresh")
	source := fixtureRecord(t, p, pathFixtures, c)
	_, first := runSelection(t, p, source, bindCase(p, c), selectionInput(c))
	data, err := os.ReadFile(source)
	if err != nil {
		t.Fatal(err)
	}
	reversed := writeRecord(t, t.TempDir(), "provider.json", mutate(t, data, func(record map[string]any) {
		list := relations(record)
		list[0], list[1] = list[1], list[0]
	}))
	second, _ := runSelection(t, p, reversed, bindCase(p, c), selectionInput(c))
	firstMap := map[string]any{}
	if err := json.Unmarshal(first, &firstMap); err != nil {
		t.Fatal(err)
	}
	delete(firstMap, "provider_evidence")
	delete(second, "provider_evidence")
	left, _ := contextindex.CanonicalJSON(firstMap)
	right, _ := contextindex.CanonicalJSON(second)
	if !bytes.Equal(left, right) {
		t.Fatalf("path advice depends on record order:\n%s\n%s", left, right)
	}
	input := selectionInput(c)
	input.Limit = 1
	bounded, _ := runSelection(t, p, source, bindCase(p, c), input)
	if len(bounded["selected"].([]any)) != 1 || bounded["omitted"].(map[string]any)["selected"] != float64(1) || bounded["state"] != SelectionNarrow {
		t.Fatalf("limit 1 keeps one, counts one, keeps the state: %v %v", bounded["omitted"], bounded["state"])
	}
}

// TestPathPrivate: a relative checkout is echoed as given and no resolved
// path or file body reaches the advice (EEP-V2-010).
func TestPathPrivate(t *testing.T) {
	t.Parallel()
	p := newPair(t)
	c := pathCase(t, "positive-cross-repository")
	checkouts := []Checkout{{ID: "e2e", Source: relativeTo(t, p.app.root, p.e2e.root)}}
	selection, out := runSelection(t, p, fixtureRecord(t, p, pathFixtures, c), checkouts, selectionInput(c))
	canonical, err := filepath.EvalSymlinks(p.e2e.root)
	if err != nil {
		t.Fatal(err)
	}
	if selection["state"] != SelectionNarrow {
		t.Fatalf("a relative checkout must bind: %v", selection["state_reason"])
	}
	for _, secret := range []string{p.app.root, canonical, "test('account')", "package main"} {
		if bytes.Contains(out, []byte(secret)) {
			t.Fatalf("advice leaks %q", secret)
		}
	}
}

// TestPathRecordStrict: only /1 and /2 decode, and the V2 shape is as strict
// as V1; an undeclared repository is a structured unknown, not a repair
// (EEP-V2-001, EEP-V2-002).
func TestPathRecordStrict(t *testing.T) {
	t.Parallel()
	p := newPair(t)
	valid := pathRecord(t, p, "path.json", nil)
	if record, err := Decode1(valid); err != nil || record.Schema != Schema2 {
		t.Fatalf("V2 fixture must decode as V2: %v", err)
	}
	invalid := map[string]func(record map[string]any){
		"future schema":    func(r map[string]any) { r["schema"] = "external-evidence-provider/3" },
		"unknown member":   func(r map[string]any) { r["path_relations"] = true },
		"unknown endpoint": func(r map[string]any) { relations(r)[0].(map[string]any)["to"].(map[string]any)["line"] = 3 },
		"mixed endpoint":   func(r map[string]any) { relations(r)[0].(map[string]any)["to"].(map[string]any)["entity"] = "cap" },
		"short blob":       func(r map[string]any) { relations(r)[0].(map[string]any)["from"].(map[string]any)["blob"] = "abc" },
		"uppercase type":   func(r map[string]any) { relations(r)[0].(map[string]any)["type"] = "Verifies" },
		"empty path":       func(r map[string]any) { relations(r)[0].(map[string]any)["from"].(map[string]any)["path"] = "" },
	}
	for name, edit := range invalid {
		if _, err := Decode1(mutate(t, valid, edit)); err == nil {
			t.Errorf("%s: record must be invalid", name)
		}
	}
	undeclared := mutate(t, valid, func(r map[string]any) {
		relations(r)[0].(map[string]any)["from"].(map[string]any)["repository"] = "elsewhere"
	})
	section := section1(t, p, undeclared, []Checkout{{ID: "e2e", Source: p.e2e.root}}, "pkg/main.go")
	if states := unknownStates(section); states["elsewhere:tests/account.spec.ts"] != unknownUnresolved {
		t.Fatalf("an undeclared repository is an unresolved unknown: %v", states)
	}
	if relations := section["path_relations"].([]any); len(relations) != 0 {
		t.Fatalf("an unresolved relation is never composed: %v", relations)
	}
}

// TestPathRelationsImpact: a V2 record adds path_relations to the impact
// section, with both endpoints and the worst-side state; the same relation
// under V1 stays unsupported and adds no member (EEP-V2-003, EEP-V2-005).
func TestPathRelationsImpact(t *testing.T) {
	t.Parallel()
	p := newPair(t)
	e2e := []Checkout{{ID: "e2e", Source: p.e2e.root}}
	section := section1(t, p, pathRecord(t, p, "two-repository.json", nil), e2e, "pkg/main.go")
	items := section["path_relations"].([]any)
	if len(items) != 2 {
		t.Fatalf("path_relations = %v", items)
	}
	for _, raw := range items {
		item := raw.(map[string]any)
		for _, member := range []string{"authority", "provider", "provider_revision", "relation", "endpoints", "relation_state", "crosses_repositories", "reason", "limitations", "verification"} {
			if _, present := item[member]; !present {
				t.Errorf("path item lacks %s: %v", member, item)
			}
		}
		if item["relation_state"] != RelationFresh || len(item["endpoints"].([]any)) != 2 {
			t.Errorf("a fresh two-sided item: %v", item)
		}
	}
	if omitted := section["omitted"].(map[string]any)["path_relations"]; omitted != float64(0) {
		t.Fatalf("omitted path_relations = %v", omitted)
	}
	unbound := section1(t, p, pathRecord(t, p, "two-repository.json", nil), nil, "pkg/main.go")
	states := map[string]any{}
	for _, raw := range unbound["path_relations"].([]any) {
		item := raw.(map[string]any)
		states[item["relation"].(map[string]any)["from"].(map[string]any)["path"].(string)] = item["relation_state"]
	}
	if states["tests/account.spec.ts"] != RelationNotVerified || states["pkg/main_test.go"] != RelationFresh {
		t.Fatalf("each relation takes its own worst side: %v", states)
	}
	v1 := section1(t, p, pathRecord(t, p, "two-repository.json", map[string]string{"SCHEMA": Schema1}), e2e, "pkg/main.go")
	if _, present := v1["path_relations"]; present {
		t.Fatal("a V1 record must not gain path_relations")
	}
	if states := unknownStates(v1); states["e2e:tests/account.spec.ts"] != unknownUnsupported || states["application:pkg/main_test.go"] != unknownUnsupported {
		t.Fatalf("a V1 path-to-path relation stays unsupported: %v", states)
	}
}

// TestPathRelationsBounded: the impact list is bounded and counted.
func TestPathRelationsBounded(t *testing.T) {
	t.Parallel()
	p := newPair(t)
	source := writeRecord(t, t.TempDir(), "provider.json", pathRecord(t, p, "two-repository.json", nil))
	out, err := contextindex.CanonicalJSON(Section(context.Background(), p.app.index(), []string{source}, []Checkout{{ID: "e2e", Source: p.e2e.root}}, []string{"pkg/main.go"}, 1))
	if err != nil {
		t.Fatal(err)
	}
	section := map[string]any{}
	if err := json.Unmarshal(out, &section); err != nil {
		t.Fatal(err)
	}
	if len(section["path_relations"].([]any)) != 1 || section["omitted"].(map[string]any)["path_relations"] != float64(1) {
		t.Fatalf("limit 1 keeps one path relation and counts one: %v", section["omitted"])
	}
}

// TestPathScope: a V2 directory scope yields one row per held path, naming
// the scope; it cannot pin a blob; a scope holding a changed path widens, and
// an unresolved scope relation blocks every path it holds (EEP-V2-012,
// EEP-V2-013).
func TestPathScope(t *testing.T) {
	t.Parallel()
	p := newPair(t)
	valid := pathRecord(t, p, "path.json", map[string]string{"SOURCE_PATH": "pkg/"})
	t.Run("EEP-V2-012 scope row names held path", func(t *testing.T) {
		selection, _ := runPathCase(t, p, pathCase(t, "scope-positive-descendant"))
		rows := selection["selected"].([]any)
		if len(rows) != 1 {
			t.Fatalf("selected = %v", rows)
		}
		row := rows[0].(map[string]any)
		if row["subject"].(map[string]any)["path"] != "pkg/main.go" || row["subject_scope"] != "application:pkg/" {
			t.Fatalf("a scope row names the held path and its scope: %v", row)
		}
	})
	t.Run("EEP-V2-012 scope must not pin blob", func(t *testing.T) {
		pinned := func(r map[string]any) {
			relations(r)[0].(map[string]any)["to"].(map[string]any)["blob"] = p.app.mainBlob
		}
		if _, err := Decode1(mutate(t, valid, pinned)); err == nil {
			t.Fatal("a V2 directory scope must not pin a blob")
		}
		v1 := pathRecord(t, p, "path.json", map[string]string{"SOURCE_PATH": "pkg/", "SCHEMA": Schema1})
		if _, err := Decode1(mutate(t, v1, pinned)); err != nil {
			t.Fatalf("V1 keeps its rules: %v", err)
		}
	})
	t.Run("EEP-V2-013 scope widens and unresolved scope blocks", func(t *testing.T) {
		widened, _ := runPathCase(t, p, pathCase(t, "scope-positive-widened"))
		if !slices.Contains(selectedTests(widened), "e2e:tests/account.spec.ts") {
			t.Fatalf("a scope holding a changed path widens to its other endpoint: %v", selectedTests(widened))
		}
		unresolved := writeRecord(t, t.TempDir(), "provider.json", mutate(t, valid, func(r map[string]any) {
			relations(r)[0].(map[string]any)["from"] = map[string]any{"provider": "mockdocs", "entity": "undeclared"}
		}))
		c := pathCase(t, "scope-positive-descendant")
		blocked, _ := runSelection(t, p, unresolved, bindCase(p, c), selectionInput(c))
		uncovered := blocked["uncovered_paths"].([]any)
		if len(uncovered) != 1 || uncovered[0].(map[string]any)["blocked"] != true {
			t.Fatalf("an unresolved scope relation blocks the path it holds: %v", uncovered)
		}
	})
}
