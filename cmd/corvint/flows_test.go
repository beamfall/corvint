package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/appflows"
	"github.com/Beamfall/corvint/internal/doccorpus"
	"github.com/Beamfall/corvint/internal/extevidence"
)

func flowRepo(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	manifest := `{"profile":"application-flow-intent/0","application":"fixture","origin":"http://127.0.0.1:12345","sources":["app.js"],"tests":[],"backendSource":"app.js","fixture":"seed","identityPath":"/identity","resetPath":"/reset","server":["node","app.js"],"scenarios":[{"id":"home","role":"reader","path":"/","basis":"inferred","actions":[],"checks":[{"id":"visible","kind":"visible","selector":"#home","want":true}]}]}`
	for p, b := range map[string]string{"flows.json": manifest, "app.js": "fixture"} {
		if err := os.WriteFile(filepath.Join(root, p), []byte(b), 0600); err != nil {
			t.Fatal(err)
		}
	}
	for _, args := range [][]string{{"init", "-q"}, {"add", "."}, {"-c", "user.name=Fixture", "-c", "user.email=fixture@example.invalid", "commit", "-qm", "fixture"}} {
		c := exec.Command("git", args...)
		c.Dir = root
		if b, err := c.CombinedOutput(); err != nil {
			t.Fatalf("git %v %s", err, b)
		}
	}
	return root
}

func flowSnapshot(t *testing.T, root string) map[string]string {
	t.Helper()
	r := map[string]string{}
	err := filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		b, e := os.ReadFile(p)
		if e != nil {
			return e
		}
		r[p] = fmt.Sprintf("%x", sha256.Sum256(b))
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return r
}

// AFU-V0-001 AFU-V0-009 AFU-V0-012
func TestFlowsCLIReadPurity(t *testing.T) {
	root := flowRepo(t)
	before := flowSnapshot(t, root)
	var out, diagnostic bytes.Buffer
	code := runContext(context.Background(), []string{"--root", root, "flows", "--manifest", "flows.json"}, bytes.NewReader(nil), &out, &diagnostic)
	if code != 0 {
		t.Fatalf("%d %s", code, diagnostic.String())
	}
	var r appflows.Report
	if err := json.Unmarshal(out.Bytes(), &r); err != nil {
		t.Fatal(err)
	}
	if r.Profile != appflows.ReportProfile || r.Complete || r.Flows[0].Runtime != "unobserved" {
		t.Fatal("unsupported confirmation", out.String())
	}
	if !reflect.DeepEqual(before, flowSnapshot(t, root)) {
		t.Fatal("flow read changed repository bytes")
	}
	out.Reset()
	diagnostic.Reset()
	code = runContext(context.Background(), []string{"--root", root, "flows", "--manifest", "missing.json"}, bytes.NewReader(nil), &out, &diagnostic)
	if code != 2 {
		t.Fatal(code)
	}
	if !reflect.DeepEqual(before, flowSnapshot(t, root)) {
		t.Fatal("failed read wrote state")
	}
}

func TestFlowsHelpDoesNotRequireRepository(t *testing.T) {
	for _, args := range [][]string{{"help", "flows"}, {"flows", "--help"}, {"flows", "record", "--help"}} {
		var out, diagnostic bytes.Buffer
		if code := runContext(context.Background(), args, bytes.NewReader(nil), &out, &diagnostic); code != 0 {
			t.Fatal(args, code, diagnostic.String())
		}
		if !bytes.Contains(out.Bytes(), []byte("application-flow-report/0")) {
			t.Fatal("missing help")
		}
	}
}

// flowIntentRepo commits app.js and one application-flow-intent/1 file under flows/.
func flowIntentRepo(t *testing.T) string {
	t.Helper()
	d := doccorpus.Digest([]byte("checkout"))
	anchor := doccorpus.Anchor{Repository: strings.Repeat("2", 40), Revision: strings.Repeat("b", 40), Path: "flows/checkout.md", Blob: strings.Repeat("d", 40),
		SHA256: d, Start: 1, End: 1, SpanSHA256: d, Authority: "external-provider", Kind: "declared", Reason: "fixture"}
	intent, err := json.Marshal(appflows.FlowIntent{Schema: appflows.FlowIntentSchema, FlowID: "checkout", Revision: 1, Kind: "ui", Actor: "shopper",
		Preconditions: []string{}, Steps: []appflows.FlowStep{{StepID: "pay", Action: "press pay"}},
		Outcomes: []appflows.FlowOutcome{{OutcomeID: "paid", Behavior: "order confirmed", Matcher: "toHaveText", Locator: "#status", Value: "paid"}},
		Variations: []appflows.FlowVariation{{VariationID: "checkout.happy", Preconditions: []string{}, Steps: []string{"pay"},
			ObservableFacts: []string{"status visible"}, Outcomes: []string{"paid"}, Projects: []string{"chromium"}}},
		Links:   []appflows.FlowLink{{From: "pay", Basis: "inferred", Target: appflows.LinkTarget{Type: "source", Path: "app.js"}}},
		Adapter: &appflows.FlowAdapter{Derivation: "declared", Evidence: anchor, RequiredPages: []string{"cart"}, NegativeControls: []string{}, OrderedEvents: []doccorpus.BehaviorEvent{}},
	})
	if err != nil {
		t.Fatal(err)
	}
	root := flowRepo(t)
	if err = os.Mkdir(filepath.Join(root, "flows"), 0700); err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(filepath.Join(root, "flows", "checkout.json"), intent, 0600); err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{{"add", "."}, {"-c", "user.name=Fixture", "-c", "user.email=fixture@example.invalid", "commit", "-qm", "intent"}} {
		c := exec.Command("git", args...)
		c.Dir = root
		if b, err := c.CombinedOutput(); err != nil {
			t.Fatalf("git %v %s", err, b)
		}
	}
	return root
}

func runFlowsCLI(root string, args ...string) (int, string, string) {
	var out, diagnostic bytes.Buffer
	code := runContext(context.Background(), append([]string{"--root", root, "flows"}, args...), bytes.NewReader(nil), &out, &diagnostic)
	return code, out.String(), diagnostic.String()
}

// AFU-V1-003 AFU-V1-010
func TestAFUV1FlowsCLIExportIsReadOnly(t *testing.T) {
	root := flowIntentRepo(t)
	before := flowSnapshot(t, root)
	code, out, diagnostic := runFlowsCLI(root, "export", "--flows", "flows", "--emit", "inventory")
	if code != 0 || !strings.Contains(out, `"schema":"application-flow-inventory/1"`) {
		t.Fatalf("inventory %d %s %s", code, out, diagnostic)
	}
	code, out, diagnostic = runFlowsCLI(root, "export", "--flows", "flows", "--emit", "provider")
	if code != 0 {
		t.Fatalf("provider %d %s", code, diagnostic)
	}
	if _, err := extevidence.Decode1([]byte(out)); err != nil {
		t.Fatalf("provider record invalid: %v", err)
	}
	for _, args := range [][]string{{"export", "--flows", "flows", "--emit", "request"}, {"export", "--flows", "flows", "--emit", "graph"}, {"export", "--emit", "inventory"}, {"import", "--flows", "flows"}} {
		if code, _, _ = runFlowsCLI(root, args...); code != 2 {
			t.Fatalf("%v exited %d", args, code)
		}
	}
	if !reflect.DeepEqual(before, flowSnapshot(t, root)) {
		t.Fatal("flows export changed repository bytes")
	}
}

// AFU-V1-004
func TestAFUV1FlowsCLIImportNeverOverwrites(t *testing.T) {
	root := flowIntentRepo(t)
	source := filepath.Join(t.TempDir(), "openapi.json")
	spec := `{"openapi":"3.0.3","paths":{"/orders":{"post":{"operationId":"checkout","responses":{"201":{"description":"made"}}}}}}`
	if err := os.WriteFile(source, []byte(spec), 0600); err != nil {
		t.Fatal(err)
	}
	before := flowSnapshot(t, root)
	if code, _, diagnostic := runFlowsCLI(root, "import", "--flows", "flows", "--from", source, "--format", "openapi"); code != 2 || !strings.Contains(diagnostic, "already exists") {
		t.Fatalf("import over an existing intent %d %s", code, diagnostic)
	}
	if !reflect.DeepEqual(before, flowSnapshot(t, root)) {
		t.Fatal("refused import changed repository bytes")
	}
	spec = strings.Replace(spec, `"checkout"`, `"place-order"`, 1)
	if err := os.WriteFile(source, []byte(spec), 0600); err != nil {
		t.Fatal(err)
	}
	code, out, diagnostic := runFlowsCLI(root, "import", "--flows", "flows", "--from", source, "--format", "openapi")
	if code != 0 || out != "flows/place-order.json\n" {
		t.Fatalf("import %d %q %s", code, out, diagnostic)
	}
	commitFlows(t, root)
	if code, out, _ = runFlowsCLI(root, "export", "--flows", "flows", "--emit", "provider"); code != 0 || !strings.Contains(out, "place-order") {
		t.Fatal("imported proposed intent is not loadable once committed")
	}
}

func commitFlows(t *testing.T, root string) {
	t.Helper()
	for _, args := range [][]string{{"add", "."}, {"-c", "user.name=Fixture", "-c", "user.email=fixture@example.invalid", "commit", "-qm", "flows"}} {
		c := exec.Command("git", args...)
		c.Dir = root
		if b, err := c.CombinedOutput(); err != nil {
			t.Fatalf("git %v %s", err, b)
		}
	}
}

// AFU-V1-001 AFU-V1-003
func TestAFUV1FlowsCLIExportUsesCommittedIntents(t *testing.T) {
	root := flowIntentRepo(t)
	intent := filepath.Join(root, "flows", "checkout.json")
	raw, err := os.ReadFile(intent)
	if err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(intent, bytes.ReplaceAll(raw, []byte("press pay"), []byte("press dirty")), 0600); err != nil {
		t.Fatal(err)
	}
	extra := bytes.ReplaceAll(raw, []byte(`"flow_id":"checkout"`), []byte(`"flow_id":"extra"`))
	if err = os.WriteFile(filepath.Join(root, "flows", "extra.json"), extra, 0600); err != nil {
		t.Fatal(err)
	}
	for _, emit := range []string{"inventory", "provider"} {
		code, out, diagnostic := runFlowsCLI(root, "export", "--flows", "flows", "--emit", emit)
		if code != 0 || strings.Contains(out, "dirty") || strings.Contains(out, "extra") {
			t.Fatalf("%s export read the working tree: %d %s %s", emit, code, out, diagnostic)
		}
	}
	if code, out, _ := runFlowsCLI(root, "export", "--flows", "flows", "--emit", "inventory"); code != 0 || !strings.Contains(out, "press pay") {
		t.Fatalf("inventory lost the committed intent: %s", out)
	}
}

// AFU-V1-004
func TestAFUV1FlowsCLIImportReportsNothingWritten(t *testing.T) {
	root := flowIntentRepo(t)
	if err := os.WriteFile(filepath.Join(root, "flows", "post-orders.JSON"), []byte("{}"), 0600); err != nil {
		t.Fatal(err)
	}
	source := filepath.Join(t.TempDir(), "openapi.json")
	spec := `{"openapi":"3.0.3","paths":{"/orders":{"post":{"responses":{"201":{"description":"made"}}}}}}`
	if err := os.WriteFile(source, []byte(spec), 0600); err != nil {
		t.Fatal(err)
	}
	before := flowSnapshot(t, root)
	code, out, diagnostic := runFlowsCLI(root, "import", "--flows", "flows", "--from", source, "--format", "openapi")
	if code != 2 || out != "" || !strings.Contains(diagnostic, "no intent from this import remains written") {
		t.Fatalf("failed import %d %q %s", code, out, diagnostic)
	}
	if !reflect.DeepEqual(before, flowSnapshot(t, root)) {
		t.Fatal("failed import changed repository bytes")
	}
}
