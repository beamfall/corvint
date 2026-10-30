package appflows

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

const openAPIFixture = `{"openapi":"3.1.0","info":{"title":"pets","version":"1"},"paths":{"/pets":{
 "get":{"operationId":"listPets","responses":{"200":{"description":"ok"},"404":{"description":"none"}}},
 "post":{"responses":{"201":{"description":"made"}}}}}}`

const playwrightFixture = `{"config":{"rootDir":ROOT},"suites":[{"title":"checkout.spec.ts","file":"checkout.spec.ts","specs":[],"suites":[{"title":"cart","file":"checkout.spec.ts",
 "specs":[{"title":"pays","file":"checkout.spec.ts","line":3,"tests":[
  {"projectName":"firefox","annotations":[{"type":"corvint-test-key","description":"checkout:pays"}]},
  {"projectName":"chromium","annotations":[]}]}]}]}]}`

// playwrightList puts dir, JSON-quoted, in place of ROOT in a list fixture.
func playwrightList(t *testing.T, fixture, dir string) []byte {
	t.Helper()
	quoted, err := json.Marshal(dir)
	if err != nil {
		t.Fatal(err)
	}
	return []byte(strings.ReplaceAll(fixture, "ROOT", string(quoted)))
}

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
	written, err = Import(root, "flows", playwrightList(t, playwrightFixture, root), FormatPlaywrightList, "list.json")
	if err != nil || !reflect.DeepEqual(written, []string{"flows/checkout.spec-cart-pays.json"}) {
		t.Fatalf("playwright import %v %v", written, err)
	}
	set, _ = LoadIntents(root, "flows")
	spec := set.Flows[1]
	if spec.Kind != "ui" || spec.Links[0].Target.TestKey != "checkout:pays" || spec.Links[0].Target.Path != "checkout.spec.ts" || !reflect.DeepEqual(spec.Variations[0].Projects, []string{"chromium", "firefox"}) {
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
	request := exportedRequest(t, root, set)
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

// AFU-V1-004
func TestAFUV1ImportRollsBackOnFailedWrite(t *testing.T) {
	root := intentRepo(t)
	before := treeSnapshot(t, root)
	calls := 0
	writeIntentFile = func(root, filename string, data []byte) error {
		if calls++; calls == 2 {
			return errors.New("disk full")
		}
		return WriteConfined(root, filename, data)
	}
	t.Cleanup(func() { writeIntentFile = WriteConfined })
	written, err := Import(root, "flows", []byte(openAPIFixture), FormatOpenAPI, "")
	if err == nil || written != nil || !strings.Contains(err.Error(), "rolled back [flows/listpets.json]") || !strings.Contains(err.Error(), nothingWritten) {
		t.Fatalf("failed import: %v %v", written, err)
	}
	if !reflect.DeepEqual(before, treeSnapshot(t, root)) {
		t.Fatal("failed import left a written intent")
	}
}

// AFU-V1-004 AFU-V1-036
func TestAFUV1ImportRefusesCaseVariantName(t *testing.T) {
	root := intentRepo(t)
	writeRaw(t, root, "flows/post-pets.JSON", []byte("{}"))
	before := treeSnapshot(t, root)
	if _, err := Import(root, "flows", []byte(openAPIFixture), FormatOpenAPI, ""); err == nil || !strings.Contains(err.Error(), nothingWritten) {
		t.Fatalf("case variant of an intent name ignored: %v", err)
	}
	if !reflect.DeepEqual(before, treeSnapshot(t, root)) {
		t.Fatal("refused import wrote a file")
	}
}

// AFU-V1-037
func TestAFUV1ImportCombinedFlowBound(t *testing.T) {
	root := intentRepo(t)
	paths := []string{}
	for i := range MaxFlows {
		paths = append(paths, fmt.Sprintf(`"/p%d":{"get":{"responses":{"200":{"description":"ok"}}}}`, i))
	}
	doc := `{"openapi":"3.1.0","info":{"title":"t","version":"1"},"paths":{` + strings.Join(paths, ",") + `}}`
	before := treeSnapshot(t, root)
	if _, err := Import(root, "flows", []byte(doc), FormatOpenAPI, ""); err == nil || !strings.Contains(err.Error(), "import would exceed 512 intents") {
		t.Fatalf("existing plus imported flows exceeded the bound: %v", err)
	}
	if !reflect.DeepEqual(before, treeSnapshot(t, root)) {
		t.Fatal("refused import wrote a file")
	}
}

// A list with testDir './tests': same-title tests under different describe blocks, a describe pair
// that differs only in case, a true duplicate, and a file outside the root.
const playwrightTestDirFixture = `{"config":{"rootDir":ROOT,"projects":[{"name":"chromium","testDir":ROOT}]},"suites":[
 {"title":"cart.spec.ts","file":"cart.spec.ts","specs":[],"suites":[
  {"title":"guest","file":"cart.spec.ts","specs":[{"title":"pays","file":"cart.spec.ts","tests":[{"projectName":"chromium"}]},
   {"title":"refunds","file":"cart.spec.ts","tests":[]},{"title":"refunds","file":"cart.spec.ts","tests":[]}]},
  {"title":"member","file":"cart.spec.ts","specs":[{"title":"pays","file":"cart.spec.ts","tests":[{"projectName":"chromium"}]}]},
  {"title":"Saved Card","file":"cart.spec.ts","specs":[{"title":"pays","file":"cart.spec.ts","tests":[]}]},
  {"title":"saved card","file":"cart.spec.ts","specs":[{"title":"pays","file":"cart.spec.ts","tests":[]}]}]},
 {"title":"../../escape.spec.ts","file":"../../escape.spec.ts","specs":[{"title":"leaks","file":"../../escape.spec.ts","tests":[]}],"suites":[]}]}`

// AFU-V1-128 AFU-V1-129
func TestAFUV1128PlaywrightListResolvesDescribeKeys(t *testing.T) {
	root := intentRepo(t)
	writeRaw(t, root, "tests/cart.spec.ts", []byte("test('pays', () => {})\n"))
	list := playwrightList(t, playwrightTestDirFixture, filepath.Join(root, "tests"))
	written, err := Import(root, "flows", list, FormatPlaywrightList, "")
	var skipped *ImportSkipped
	if !errors.As(err, &skipped) || skipped.Written != 4 || len(skipped.Items) != 3 {
		t.Fatalf("partial import %v %v", written, err)
	}
	suffix := func(key string) string {
		sum := sha256.Sum256([]byte(key))
		return "-" + hex.EncodeToString(sum[:4])
	}
	want := []string{"flows/cart.spec-guest-pays.json", "flows/cart.spec-member-pays.json",
		"flows/cart.spec-saved-card-pays" + suffix("tests/cart.spec.ts > Saved Card > pays") + ".json",
		"flows/cart.spec-saved-card-pays" + suffix("tests/cart.spec.ts > saved card > pays") + ".json"}
	if !reflect.DeepEqual(written, want) {
		t.Fatalf("written %v want %v", written, want)
	}
	for _, item := range skipped.Items {
		switch {
		case item.TestKey == "tests/cart.spec.ts > guest > refunds" && strings.Contains(item.Reason, "2 listed tests share"):
		case item.TestKey == "../../escape.spec.ts > leaks" && strings.Contains(item.Reason, "under --root"):
		default:
			t.Fatalf("skip %+v", item)
		}
	}
	set, err := LoadIntents(root, "flows")
	if err != nil {
		t.Fatal(err)
	}
	guest := set.Flows[0]
	if guest.FlowID != "cart.spec-guest-pays" || guest.Links[0].Target.TestKey != "tests/cart.spec.ts > guest > pays" || guest.Links[0].Target.Path != "tests/cart.spec.ts" {
		t.Fatalf("guest flow %+v", guest)
	}
	// Re-importing reports every test against the flows it already wrote and writes nothing.
	written, err = Import(root, "flows", list, FormatPlaywrightList, "")
	if !errors.As(err, &skipped) || len(written) != 0 || skipped.Written != 0 || len(skipped.Items) != 7 || !strings.Contains(err.Error(), "already exists") {
		t.Fatalf("re-import %v %v", written, err)
	}
	for name, dir := range map[string]string{"outside --root": t.TempDir(), "not an existing": filepath.Join(root, "missing")} {
		if _, err = Import(root, "flows", playwrightList(t, playwrightTestDirFixture, dir), FormatPlaywrightList, ""); err == nil || errors.As(err, &skipped) || !strings.Contains(err.Error(), name) {
			t.Fatalf("rootDir %s: %v", dir, err)
		}
	}
	if _, err = Import(root, "flows", []byte(`{"config":{},"suites":[]}`), FormatPlaywrightList, ""); err == nil || !strings.Contains(err.Error(), "declares neither") {
		t.Fatalf("no rootDir: %v", err)
	}
}

// AFU-V1-251 (#1930): a list that repeats a test once per project imports it once, under every
// project; a duplicate within one project, or one without a project name, is still a collision.
func TestAFUV1251PlaywrightListMergesProjects(t *testing.T) {
	root := intentRepo(t)
	writeRaw(t, root, "tests/cart.spec.ts", []byte("test('pays', () => {})\n"))
	spec := func(title, project string) string {
		tests := `[]`
		if project != "" {
			tests = `[{"projectName":"` + project + `"}]`
		}
		return `{"title":"` + title + `","file":"cart.spec.ts","tests":` + tests + `}`
	}
	suite := func(specs ...string) string {
		return `{"title":"cart.spec.ts","file":"cart.spec.ts","specs":[],"suites":[{"title":"guest","file":"cart.spec.ts","specs":[` + strings.Join(specs, ",") + `]}]}`
	}
	fixture := `{"config":{"rootDir":ROOT,"projects":[{"name":"a","testDir":ROOT},{"name":"b","testDir":ROOT}]},"suites":[` +
		suite(spec("pays", "a"), spec("refunds", "a"), spec("lists", ""), spec("voids", "a")) + "," +
		suite(spec("pays", "b"), spec("refunds", "a"), spec("lists", ""), spec("voids", "b"), spec("voids", "b")) + `]}`
	written, err := Import(root, "flows", playwrightList(t, fixture, filepath.Join(root, "tests")), FormatPlaywrightList, "")
	var skipped *ImportSkipped
	if !errors.As(err, &skipped) || !reflect.DeepEqual(written, []string{"flows/cart.spec-guest-pays.json"}) || len(skipped.Items) != 6 {
		t.Fatalf("import %v %v", written, err)
	}
	for _, item := range skipped.Items {
		if !strings.Contains(item.Reason, "within one project") {
			t.Fatalf("skip %+v", item)
		}
	}
	set, err := LoadIntents(root, "flows")
	if err != nil {
		t.Fatal(err)
	}
	if v := set.Flows[0].Variations[0]; !reflect.DeepEqual(v.Projects, []string{"a", "b"}) || len(set.Flows[0].Links) != 1 {
		t.Fatalf("merged flow %+v", set.Flows[0])
	}
}
