package appflows

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	jsonv2 "encoding/json/v2"

	"github.com/Beamfall/corvint/internal/doccorpus"
	"github.com/Beamfall/corvint/internal/extevidence"
)

// envelopeFor commits inventory under evidence/ and builds a minimal valid behavior-adapter request whose
// application-flows input anchors that commit, path and blob.
func envelopeFor(t *testing.T, root string, inventory []byte) []byte {
	t.Helper()
	const inventoryPath = "evidence/application-flows.json"
	writeRaw(t, root, inventoryPath, inventory)
	gitTest(t, root, "add", "-A")
	gitTest(t, root, "-c", "user.name=Fixture", "-c", "user.email=fixture@example.invalid", "commit", "-q", "--allow-empty", "-m", "inventory")
	head := gitOut(t, root, "rev-parse", "HEAD")
	revisions := doccorpus.BehaviorRevisions{
		App:  doccorpus.Repository{ID: strings.Repeat("1", 40), Revision: strings.Repeat("a", 40)},
		E2E:  doccorpus.Repository{ID: strings.Repeat("2", 40), Revision: strings.Repeat("b", 40)},
		Docs: doccorpus.Repository{ID: strings.Repeat("3", 40), Revision: strings.Repeat("c", 40)},
	}
	encode := func(v any) []byte {
		b, err := doccorpus.Encode(v)
		if err != nil {
			t.Fatal(err)
		}
		return b
	}
	docs := map[string][]byte{
		"migration":  encode(doccorpus.BehaviorMigration{Revisions: revisions, Schema: 2, ContractID: "flows", SourceRevision: revisions.E2E.Revision, DocumentationRevision: revisions.Docs.Revision}),
		"discovery":  encode(doccorpus.BehaviorDiscovery{Schema: "corvint-playwright-discovery/1", Mode: "live-playwright-list", Revisions: revisions, Executions: []doccorpus.BehaviorExecution{}}),
		"candidates": []byte(`{"candidates":[]}`),
		"tests":      []byte(`{"tests":[]}`),
		FlowsInputID: inventory,
	}
	inputs := []doccorpus.BehaviorAdapterInput{}
	for id, doc := range docs {
		anchor := fixtureAnchor("evidence/"+id+".json", doc)
		if id == FlowsInputID {
			anchor.Revision, anchor.Blob = head, gitOut(t, root, "rev-parse", head+":"+inventoryPath)
		}
		inputs = append(inputs, doccorpus.BehaviorAdapterInput{ID: id, Anchor: anchor, Document: string(doc)})
	}
	byName := func(names ...string) map[string]string {
		fields := map[string]string{}
		for _, n := range names {
			fields[n] = "/" + n
		}
		return fields
	}
	request := doccorpus.BehaviorAdapterRequest{
		Schema: doccorpus.BehaviorAdapterRequestSchema, ProviderID: "flows-provider", ProviderVersion: "1", ContractID: "flows",
		Source: revisions.E2E, Revisions: revisions, SourceRevision: revisions.E2E.Revision, DocumentationRevision: revisions.Docs.Revision,
		MigrationInput: "migration", DiscoveryInput: "discovery", Inputs: inputs,
		Mappings: append(inventoryMappings(),
			doccorpus.BehaviorAdapterMapping{Kind: "candidates", Input: "candidates", Records: "/candidates", Fields: byName("id", "evidence", "flows")},
			doccorpus.BehaviorAdapterMapping{Kind: "tests", Input: "tests", Records: "/tests", Fields: byName("id", "project", "title", "evidence", "flows", "criteria", "assertions", "variation_claims")}),
	}
	return encode(request)
}

func exportedRequest(t *testing.T, root string, set IntentSet) []byte {
	t.Helper()
	inventory, err := CompileInventory(set)
	if err != nil {
		t.Fatal(err)
	}
	out, err := ExportRequest(context.Background(), root, envelopeFor(t, root, inventory), inventory)
	if err != nil {
		t.Fatal(err)
	}
	return out
}

// AFU-V1-003
func TestAFUV1ExportCompilesRequestAndProvider(t *testing.T) {
	root := intentRepo(t)
	set, err := LoadIntentsAt(context.Background(), root, "flows", "HEAD")
	if err != nil {
		t.Fatal(err)
	}
	request := exportedRequest(t, root, set)
	result, err := doccorpus.BuildBehaviorAdapter(request, nil)
	if err != nil {
		t.Fatalf("exported request refused by the DCP-V1 adapter: %v", err)
	}
	if len(result.Variations) != 1 || !reflect.DeepEqual(result.Variations[0].Actions, []string{"open cart", "press pay"}) ||
		!reflect.DeepEqual(result.Variations[0].Preconditions, []string{"cart has one item", "signed in"}) ||
		!reflect.DeepEqual(result.Variations[0].Tests, []string{"checkout.spec.ts > pays"}) {
		t.Fatalf("compiled variation: %+v", result.Variations)
	}
	var decoded doccorpus.BehaviorAdapterRequest
	if err = jsonv2.Unmarshal(request, &decoded); err != nil {
		t.Fatal(err)
	}
	for _, m := range decoded.Mappings {
		if (m.Kind == "flows" || m.Kind == "variations") != (m.Input == FlowsInputID) {
			t.Fatalf("mapping %s reads %s", m.Kind, m.Input)
		}
	}
	provider, err := ExportProvider(context.Background(), root, set)
	if err != nil {
		t.Fatal(err)
	}
	record, err := extevidence.Decode1(provider)
	if err != nil || record.Provider.ID != ProviderID || len(record.Relations) != 3 {
		t.Fatalf("provider record %+v %v", record, err)
	}
	if record.Relations[2].Evidence != extevidence.EvidenceInferred || record.Relations[0].To.Entity == "" && record.Relations[0].To.Path == "" {
		t.Fatalf("provider relation basis or target lost: %+v", record.Relations)
	}
	if bytes.Contains(request, []byte("proposed")) || bytes.Contains(provider, []byte("proposed")) {
		t.Fatal("proposed mark leaked into export")
	}
}

// AFU-V1-003
func TestAFUV1ExportRefusesUnanchoredInventory(t *testing.T) {
	root := intentRepo(t)
	set, err := LoadIntents(root, "flows")
	if err != nil {
		t.Fatal(err)
	}
	inventory, err := CompileInventory(set)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = ExportRequest(context.Background(), root, envelopeFor(t, root, []byte(`{"flows":[],"variations":[]}`)), inventory); err == nil || !strings.Contains(err.Error(), "anchor") {
		t.Fatalf("stale inventory anchor accepted: %v", err)
	}
	set.Flows[0].Adapter = nil
	if _, err = CompileInventory(set); err == nil {
		t.Fatal("flow without an adapter evidence anchor exported")
	}
	set.Flows[0] = sampleIntent("checkout")
	set.Flows[0].Variations[0].Projects = []string{}
	inventory, _ = CompileInventory(set)
	if _, err = ExportRequest(context.Background(), root, envelopeFor(t, root, inventory), inventory); err == nil || !strings.Contains(err.Error(), "behavior adapter refused") {
		t.Fatalf("DCP-V1 reconciler bypassed: %v", err)
	}
}

// AFU-V1-005
func TestAFUV1RoundTripByteExact(t *testing.T) {
	root := intentRepo(t)
	set, err := LoadIntents(root, "flows")
	if err != nil {
		t.Fatal(err)
	}
	first := exportedRequest(t, root, set)
	writeRaw(t, root, "imported/.keep", nil)
	source := filepath.Join(t.TempDir(), "request.json")
	if err = os.WriteFile(source, first, 0600); err != nil {
		t.Fatal(err)
	}
	written, err := Import(root, "imported", first, FormatAdapterRequest, source)
	if err != nil || len(written) != 1 {
		t.Fatalf("import %v %v", written, err)
	}
	imported, err := LoadIntents(root, "imported")
	if err != nil {
		t.Fatal(err)
	}
	if !imported.Flows[0].Proposed {
		t.Fatal("imported intent not proposed")
	}
	inventory, err := CompileInventory(imported)
	if err != nil {
		t.Fatal(err)
	}
	second, err := ExportRequest(context.Background(), root, first, inventory)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(first, second) {
		t.Fatalf("round trip changed the request:\n%s\n%s", first, second)
	}
}

// AFU-V1-003 AFU-V1-010
func TestAFUV1ExportLeavesRepositoryByteIdentical(t *testing.T) {
	root := intentRepo(t)
	set := reviewSet(t, root, gitOut(t, root, "rev-parse", "HEAD"))
	inventory, err := CompileInventory(set)
	if err != nil {
		t.Fatal(err)
	}
	envelope := envelopeFor(t, root, inventory)
	before := treeSnapshot(t, root)
	if _, err = ExportRequest(context.Background(), root, envelope, inventory); err != nil {
		t.Fatal(err)
	}
	if _, err = ExportProvider(context.Background(), root, set); err != nil {
		t.Fatal(err)
	}
	if after := treeSnapshot(t, root); !reflect.DeepEqual(before, after) {
		t.Fatal("export mutated the repository or Git state")
	}
}

// forged rewrites the application-flows input anchor of an envelope.
func forged(t *testing.T, envelope []byte, edit func(*doccorpus.Anchor)) []byte {
	t.Helper()
	var req doccorpus.BehaviorAdapterRequest
	if err := jsonv2.Unmarshal(envelope, &req); err != nil {
		t.Fatal(err)
	}
	for i := range req.Inputs {
		if req.Inputs[i].ID == FlowsInputID {
			edit(&req.Inputs[i].Anchor)
		}
	}
	out, err := doccorpus.Encode(req)
	if err != nil {
		t.Fatal(err)
	}
	return out
}

// AFU-V1-003
func TestAFUV1ExportRequestRefusesForgedAnchor(t *testing.T) {
	root := intentRepo(t)
	set, err := LoadIntentsAt(context.Background(), root, "flows", "HEAD")
	if err != nil {
		t.Fatal(err)
	}
	inventory, err := CompileInventory(set)
	if err != nil {
		t.Fatal(err)
	}
	envelope := envelopeFor(t, root, inventory)
	intentBlob := gitOut(t, root, "rev-parse", "HEAD:flows/checkout.json")
	forgeries := map[string]func(*doccorpus.Anchor){
		"unknown revision": func(a *doccorpus.Anchor) { a.Revision = strings.Repeat("e", 40) },
		"wrong blob":       func(a *doccorpus.Anchor) { a.Blob = intentBlob },
		"wrong path":       func(a *doccorpus.Anchor) { a.Path = "flows/checkout.json" },
		"uncommitted path": func(a *doccorpus.Anchor) { a.Path = "evidence/missing.json" },
	}
	for name, edit := range forgeries {
		if _, err = ExportRequest(context.Background(), root, forged(t, envelope, edit), inventory); err == nil || !strings.Contains(err.Error(), "committed inventory") {
			t.Fatalf("%s: forged anchor accepted: %v", name, err)
		}
	}
}

// AFU-V1-001 AFU-V1-003
func TestAFUV1ExportReadsCommittedIntents(t *testing.T) {
	root := intentRepo(t)
	head := gitOut(t, root, "rev-parse", "HEAD")
	dirty := sampleIntent("checkout")
	dirty.Revision = 99
	writeIntent(t, root, "flows/checkout.json", dirty)
	writeIntent(t, root, "flows/extra.json", sampleIntent("extra"))
	set, err := LoadIntentsAt(context.Background(), root, "flows", "HEAD")
	if err != nil {
		t.Fatal(err)
	}
	if set.Revision != head || len(set.Flows) != 1 || set.Flows[0].Revision != 1 {
		t.Fatalf("export read the working tree: %+v", set)
	}
	working, err := LoadIntents(root, "flows")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = ExportProvider(context.Background(), root, working); err == nil {
		t.Fatal("provider exported a working-tree intent set")
	}
	if _, err = LoadIntentsAt(context.Background(), root, "absent", "HEAD"); err == nil || !strings.Contains(err.Error(), "absent") {
		t.Fatalf("absent flows directory: %v", err)
	}
}
