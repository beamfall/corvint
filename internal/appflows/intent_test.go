package appflows

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/doccorpus"
)

func sampleIntent(id string) FlowIntent {
	return FlowIntent{
		Schema: FlowIntentSchema, FlowID: id, Revision: 1, Kind: "ui", Actor: "shopper", Preconditions: []string{"signed in"},
		Steps:    []FlowStep{{StepID: "open", Action: "open cart"}, {StepID: "pay", Action: "press pay"}},
		Outcomes: []FlowOutcome{{OutcomeID: "paid", Behavior: "order confirmed", Matcher: "toHaveText", Locator: "#status", Value: "paid"}},
		Variations: []FlowVariation{{VariationID: id + ".happy", Preconditions: []string{"cart has one item"}, Steps: []string{"open", "pay"},
			ObservableFacts: []string{"status visible"}, Outcomes: []string{"paid"}, Projects: []string{"chromium"}}},
		Links: []FlowLink{
			{From: id + ".happy", Basis: "declared", Target: LinkTarget{Type: "test", TestKey: id + ".spec.ts > pays", Path: "checkout.spec.ts"}},
			{From: "pay", Basis: "declared", Target: LinkTarget{Type: "source", Path: "app.js", StartLine: 1, EndLine: 1}},
			{From: "pay", Basis: "inferred", Target: LinkTarget{Type: "source", Path: "app.js"}},
		},
		Adapter: &FlowAdapter{Derivation: "declared", Evidence: fixtureAnchor("flows/"+id+".md", []byte(id)), RequiredPages: []string{"cart"}, NegativeControls: []string{}, OrderedEvents: []doccorpus.BehaviorEvent{}},
	}
}

func fixtureAnchor(path string, doc []byte) doccorpus.Anchor {
	d := doccorpus.Digest(doc)
	return doccorpus.Anchor{Repository: strings.Repeat("2", 40), Revision: strings.Repeat("b", 40), Path: path, Blob: strings.Repeat("d", 40), SHA256: d, Start: 1, End: 1, SpanSHA256: d, Authority: "external-provider", Kind: "declared", Reason: "fixture"}
}

func writeIntent(t *testing.T, root, name string, f FlowIntent) {
	t.Helper()
	data, err := encodeIntent(f)
	if err != nil {
		t.Fatal(err)
	}
	writeRaw(t, root, name, data)
}

func writeRaw(t *testing.T, root, name string, data []byte) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(filepath.Join(root, name)), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, name), data, 0600); err != nil {
		t.Fatal(err)
	}
}

// intentRepo commits app.js, a test file and one intent under flows/.
func intentRepo(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	writeRaw(t, root, "app.js", []byte("function pay() {}\nconst unrelated = 1\n"))
	writeRaw(t, root, "checkout.spec.ts", []byte("test('pays', () => {})\n"))
	writeIntent(t, root, "flows/checkout.json", sampleIntent("checkout"))
	gitTest(t, root, "init", "-q")
	commitAll(t, root, "fixture")
	return root
}

func commitAll(t *testing.T, root, message string) string {
	t.Helper()
	gitTest(t, root, "add", "-A")
	gitTest(t, root, "-c", "user.name=Fixture", "-c", "user.email=fixture@example.invalid", "commit", "-qm", message)
	return gitOut(t, root, "rev-parse", "HEAD")
}

func gitOut(t *testing.T, root string, args ...string) string {
	t.Helper()
	c := exec.Command("git", args...)
	c.Dir = root
	out, err := c.Output()
	if err != nil {
		t.Fatalf("git %v: %v", args, err)
	}
	return strings.TrimSpace(string(out))
}

// treeSnapshot hashes every file under root, .git included.
func treeSnapshot(t *testing.T, root string) map[string]string {
	t.Helper()
	snap := map[string]string{}
	err := filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		b, err := os.ReadFile(p)
		sum := sha256.Sum256(b)
		snap[p] = hex.EncodeToString(sum[:])
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	return snap
}

// AFU-V1-001
func TestAFUV1IntentClosedSchema(t *testing.T) {
	root := intentRepo(t)
	set, err := LoadIntents(root, "flows")
	if err != nil || len(set.Flows) != 1 || set.Flows[0].FlowID != "checkout" {
		t.Fatalf("load %+v %v", set, err)
	}
	cases := map[string]func(){
		"unknown field": func() {
			writeRaw(t, root, "flows/checkout.json", []byte(`{"schema":"application-flow-intent/1","flow_id":"checkout","revision":1,"kind":"ui","actor":"a","preconditions":[],"steps":[],"outcomes":[],"variations":[],"links":[],"owner":"x"}`))
		},
		"file name": func() { writeIntent(t, root, "flows/checkout.json", sampleIntent("other")) },
		"revision": func() {
			f := sampleIntent("checkout")
			f.Revision = 0
			writeIntent(t, root, "flows/checkout.json", f)
		},
		"kind": func() {
			f := sampleIntent("checkout")
			f.Kind = "batch"
			writeIntent(t, root, "flows/checkout.json", f)
		},
		"dangling step": func() {
			f := sampleIntent("checkout")
			f.Variations[0].Steps = []string{"missing"}
			writeIntent(t, root, "flows/checkout.json", f)
		},
	}
	for name, mutate := range cases {
		mutate()
		if _, err := LoadIntents(root, "flows"); err == nil {
			t.Fatalf("%s accepted", name)
		}
	}
	for _, dir := range []string{"../flows", ".", "/tmp", "missing"} {
		if _, err := LoadIntents(root, dir); err == nil {
			t.Fatalf("--flows %q accepted outside the root", dir)
		}
	}
	if err := os.Symlink(filepath.Join(root, "flows"), filepath.Join(root, "linked")); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadIntents(root, "linked"); err == nil {
		t.Fatal("symlinked --flows directory accepted")
	}
}

// AFU-V1-002
func TestAFUV1RetiredAndDuplicateIDsRefused(t *testing.T) {
	root := intentRepo(t)
	writeRaw(t, root, "flows/retired.json", []byte(`{"schema":"application-flow-retired/1","flow_ids":["checkout"],"variation_ids":[]}`))
	if _, err := LoadIntents(root, "flows"); err == nil || !strings.Contains(err.Error(), "retired") {
		t.Fatalf("retired flow reuse: %v", err)
	}
	writeRaw(t, root, "flows/retired.json", []byte(`{"schema":"application-flow-retired/1","flow_ids":[],"variation_ids":["checkout.happy"]}`))
	if _, err := LoadIntents(root, "flows"); err == nil || !strings.Contains(err.Error(), "retired") {
		t.Fatalf("retired variation reuse: %v", err)
	}
	writeRaw(t, root, "flows/retired.json", []byte(`{"schema":"application-flow-retired/1","flow_ids":["gone"],"variation_ids":[]}`))
	twin := sampleIntent("refund")
	twin.Variations[0].VariationID = "checkout.happy"
	twin.Links = twin.Links[1:]
	writeIntent(t, root, "flows/refund.json", twin)
	if _, err := LoadIntents(root, "flows"); err == nil || !strings.Contains(err.Error(), "declared by flows") {
		t.Fatalf("duplicate variation across flows: %v", err)
	}
}

// AFU-V1-037
func TestAFUV1IntentBoundsRefused(t *testing.T) {
	root := intentRepo(t)
	f := sampleIntent("checkout")
	for i := 0; i <= maxFlowLinks; i++ {
		f.Links = append(f.Links, f.Links[2])
	}
	writeIntent(t, root, "flows/checkout.json", f)
	if _, err := LoadIntents(root, "flows"); err == nil || !strings.Contains(err.Error(), "bound") {
		t.Fatalf("link bound: %v", err)
	}
	writeRaw(t, root, "flows/checkout.json", []byte(strings.Repeat(" ", MaxBytes+1)))
	if _, err := LoadIntents(root, "flows"); err == nil || !strings.Contains(err.Error(), "byte limit") {
		t.Fatalf("byte bound: %v", err)
	}
}

// AFU-V1-038
func TestAFUV1IntentSecretScreened(t *testing.T) {
	root := intentRepo(t)
	f := sampleIntent("checkout")
	f.Actor = "token ghp_abcdefghijklmnopqrstuvwxyz0123456789"
	writeIntent(t, root, "flows/checkout.json", f)
	if _, err := LoadIntents(root, "flows"); err == nil || !strings.Contains(err.Error(), "secret") {
		t.Fatalf("secret-shaped intent accepted: %v", err)
	}
}

// AFU-V1-037
func TestAFUV1IntentCountBoundedBeforeRead(t *testing.T) {
	root := intentRepo(t)
	for i := range MaxFlows + 1 {
		writeRaw(t, root, fmt.Sprintf("flows/f%d.json", i), []byte("not json"))
	}
	if _, err := LoadIntents(root, "flows"); err == nil || !strings.Contains(err.Error(), "exceeds") {
		t.Fatalf("candidate count not bounded before reading: %v", err)
	}
	commitAll(t, root, "too many intents")
	if _, err := LoadIntentsAt(context.Background(), root, "flows", "HEAD"); err == nil || !strings.Contains(err.Error(), "exceeds") {
		t.Fatalf("committed candidate count not bounded before reading: %v", err)
	}
}

// AFU-V1-037
func TestAFUV1IntentCountBoundedWithoutRetired(t *testing.T) {
	root := t.TempDir()
	for i := range MaxFlows + 1 {
		writeIntent(t, root, fmt.Sprintf("flows/f%03d.json", i), sampleIntent(fmt.Sprintf("f%03d", i)))
	}
	if _, err := LoadIntents(root, "flows"); err == nil || !strings.Contains(err.Error(), "exceeds") {
		t.Fatalf("%d intents without retired.json loaded: %v", MaxFlows+1, err)
	}
}
