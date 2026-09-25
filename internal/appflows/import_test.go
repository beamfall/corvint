package appflows

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

const openAPIFixture = `{"openapi":"3.1.0","info":{"title":"pets","version":"1"},"paths":{"/pets":{
 "get":{"operationId":"listPets","responses":{"200":{"description":"ok"},"404":{"description":"none"}}},
 "post":{"responses":{"201":{"description":"made"}}}}}}`

const playwrightFixture = `{"config":{},"suites":[{"title":"checkout.spec.ts","file":"checkout.spec.ts","specs":[],"suites":[{"title":"cart","file":"checkout.spec.ts",
 "specs":[{"title":"pays","file":"checkout.spec.ts","line":3,"tests":[
  {"projectName":"firefox","annotations":[{"type":"corvint-test-key","description":"checkout:pays"}]},
  {"projectName":"chromium","annotations":[]}]}]}]}]}`

func assertProposed(t *testing.T, intents []FlowIntent) {
	t.Helper()
	for _, f := range intents {
		if !f.Proposed || f.Revision != 1 {
			t.Fatalf("imported flow %s not proposed", f.FlowID)
		}
		for _, l := range f.Links {
			if l.Basis != "inferred" || l.ReviewedAt != "" {
				t.Fatalf("imported link not inferred: %+v", l)
			}
		}
	}
}

// AFU-V1-004
func TestAFUV1ImportOpenAPIAndPlaywright(t *testing.T) {
	root := intentRepo(t)
	writeRaw(t, root, "api/openapi.json", []byte(openAPIFixture))
	written, err := Import(root, "flows", []byte(openAPIFixture), FormatOpenAPI, filepath.Join(root, "api/openapi.json"))
	if err != nil || !reflect.DeepEqual(written, []string{"flows/listpets.json", "flows/post-pets.json"}) {
		t.Fatalf("openapi import %v %v", written, err)
	}
	set, err := LoadIntents(root, "flows")
	if err != nil {
		t.Fatal(err)
	}
	pets := set.Flows[1]
	if pets.FlowID != "listpets" || pets.Kind != "api" || len(pets.Outcomes) != 2 || pets.Outcomes[0].Value != "200" || pets.Links[0].Target.Path != "api/openapi.json" {
		t.Fatalf("openapi flow %+v", pets)
	}
	assertProposed(t, set.Flows[1:])
	written, err = Import(root, "flows", []byte(playwrightFixture), FormatPlaywrightList, "list.json")
	if err != nil || !reflect.DeepEqual(written, []string{"flows/checkout.spec-pays.json"}) {
		t.Fatalf("playwright import %v %v", written, err)
	}
	set, _ = LoadIntents(root, "flows")
	spec := set.Flows[1]
	if spec.Kind != "ui" || spec.Links[0].Target.TestKey != "checkout:pays" || !reflect.DeepEqual(spec.Variations[0].Projects, []string{"chromium", "firefox"}) {
		t.Fatalf("playwright flow %+v", spec)
	}
	assertProposed(t, []FlowIntent{spec})
	if _, err = Import(root, "flows", []byte(`{"openapi":"2.0","paths":{}}`), FormatOpenAPI, ""); err == nil {
		t.Fatal("OpenAPI 2 accepted")
	}
}

// AFU-V1-004 AFU-V1-036
func TestAFUV1ImportNeverOverwrites(t *testing.T) {
	root := intentRepo(t)
	set, err := LoadIntents(root, "flows")
	if err != nil {
		t.Fatal(err)
	}
	request := exportedRequest(t, set)
	before := treeSnapshot(t, root)
	if _, err = Import(root, "flows", request, FormatAdapterRequest, ""); err == nil || !strings.Contains(err.Error(), "already exists") {
		t.Fatalf("import over an existing intent: %v", err)
	}
	if !reflect.DeepEqual(before, treeSnapshot(t, root)) {
		t.Fatal("refused import wrote a file")
	}
	writeRaw(t, root, "other/.keep", nil)
	if err = os.WriteFile(filepath.Join(root, "other", "checkout.json"), []byte("{}"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err = Import(root, "other", request, FormatAdapterRequest, ""); err == nil {
		t.Fatal("import replaced an unreadable existing intent file")
	}
	if b, _ := os.ReadFile(filepath.Join(root, "other", "checkout.json")); string(b) != "{}" {
		t.Fatal("existing file overwritten")
	}
}

// AFU-V1-002 AFU-V1-004
func TestAFUV1ImportRefusesRetiredIDs(t *testing.T) {
	root := intentRepo(t)
	writeRaw(t, root, "flows/retired.json", []byte(`{"schema":"application-flow-retired/1","flow_ids":["listpets"],"variation_ids":[]}`))
	if _, err := Import(root, "flows", []byte(openAPIFixture), FormatOpenAPI, ""); err == nil || !strings.Contains(err.Error(), "retired") {
		t.Fatalf("retired flow ID reissued by import: %v", err)
	}
	if _, err := os.Lstat(filepath.Join(root, "flows", "post-pets.json")); err == nil {
		t.Fatal("refused import wrote a partial set")
	}
}

// AFU-V1-038 AFU-V1-037
func TestAFUV1ImportScreensAndBoundsSource(t *testing.T) {
	root := intentRepo(t)
	secret := strings.Replace(openAPIFixture, `"listPets"`, `"ghp_abcdefghijklmnopqrstuvwxyz0123456789"`, 1)
	if _, err := Import(root, "flows", []byte(secret), FormatOpenAPI, ""); err == nil || !strings.Contains(err.Error(), "secret") {
		t.Fatalf("secret-shaped import source accepted: %v", err)
	}
	if _, err := Import(root, "flows", []byte(strings.Repeat(" ", MaxBytes+1)), FormatPlaywrightList, ""); err == nil || !strings.Contains(err.Error(), "byte limit") {
		t.Fatalf("oversized import source accepted: %v", err)
	}
	if _, err := Import(root, "flows", []byte("{}"), "yaml", ""); err == nil {
		t.Fatal("unknown import format accepted")
	}
}
