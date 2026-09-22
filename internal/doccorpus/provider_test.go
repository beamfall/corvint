package doccorpus

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/testvalidity"
)

func declaredEvidence(t *testing.T, root string, m Manifest, p string, line int) Evidence {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(root, p))
	if err != nil {
		t.Fatal(err)
	}
	var input Input
	for _, in := range m.Inputs {
		if in.Path == p {
			input = in
		}
	}
	excerpt, err := span(data, line, line)
	if err != nil {
		t.Fatal(err)
	}
	return Evidence{Derivation: "declared", Trust: "generated", State: "supported", Freshness: "fresh", Anchors: []Anchor{{Repository: m.Repository.ID, Revision: m.Repository.Revision, Path: p, Blob: input.Blob, SHA256: Digest(data), Start: line, End: line, SpanSHA256: Digest(excerpt), Authority: "external-provider", Kind: "declared", Reason: "explicit fixture provider assertion"}}, Limitations: []string{"provider declaration; structural anchor does not prove behavior"}}
}
func providerFixture(t *testing.T, edit func(*ProviderRecord)) (string, Manifest) {
	t.Helper()
	root, m := fixture(t)
	ev := declaredEvidence(t, root, m, "src/value.go", 4)
	testEv := declaredEvidence(t, root, m, "src/value_test.go", 3)
	record := ProviderRecord{Schema: ProviderSchema, ID: "adapter", Version: "1", Source: m.Repository, Subjects: []Subject{{"adapter:count", "capability", "Count capability", "adapter", ev}}, Claims: []Claim{{"adapter:claim", "adapter:count", "Count is a provider-declared capability", "adapter", ev}}, Relations: []Relation{}, Journeys: []Journey{{ID: "adapter:journey", Subject: "adapter:count", Provider: "adapter", Status: "non_ui", Preconditions: []string{}, Cleanup: "not-run", Steps: []Step{}, Evidence: ev}}, Observations: []ObservationLink{{ID: "adapter:observation", Subject: "native:symbol:src/value_test.go:3:TestCount", Input: "evidence/run.json", SourceRevision: m.Repository.Revision, RunID: "placeholder", Package: "example.invalid/corpus/src", Test: "TestCount", StepEvidence: []Anchor{}}}, Capabilities: []CapabilityDeclaration{{"subjects", "present", "explicit provider subjects"}, {"claims", "present", "explicit provider claims"}, {"relations", "present", "declared complete zero relation inventory"}, {"journeys", "present", "explicit non-UI disposition"}, {"observations", "present", "retained native test run"}}}
	_ = testEv
	// Retained run is committed before the provider so both its commit and its
	// digest can be pinned without self-reference.
	run := `{"profile":"corvint-go-live-session-event/0","state":"failed","identity":"opaque","sequence":1,"scope":["./src"],"detail":"","projection":{"execution":{"state":"PASSED"}},"testProjections":[{"package":"example.invalid/corpus/src","name":"TestCount","action":"fail","projection":{"execution":{"state":"PASSED"}}}],"testProjectionsOmitted":0}`
	if err := os.MkdirAll(filepath.Join(root, "evidence"), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "evidence/run.json"), []byte(run), 0600); err != nil {
		t.Fatal(err)
	}
	git(t, root, "add", "evidence/run.json")
	git(t, root, "commit", "-qm", "retained run")
	runRevision := git(t, root, "rev-parse", "HEAD")
	record.Observations[0].InputRevision = runRevision
	record.Observations[0].RunID = Digest([]byte(run))
	if edit != nil {
		edit(&record)
	}
	data, err := Encode(record)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "evidence/provider.json"), data, 0600); err != nil {
		t.Fatal(err)
	}
	git(t, root, "add", "evidence/provider.json")
	git(t, root, "commit", "-qm", "provider")
	providerRevision := git(t, root, "rev-parse", "HEAD")
	m.Providers = append(m.Providers, Provider{ID: "adapter", Kind: "records", Version: "1", Revision: providerRevision, Record: "evidence/provider.json"})
	for _, in := range []struct{ path, revision, purpose string }{{"evidence/run.json", runRevision, "observation"}, {"evidence/provider.json", providerRevision, "provider"}} {
		raw, err := os.ReadFile(filepath.Join(root, in.path))
		if err != nil {
			t.Fatal(err)
		}
		m.Scopes = append(m.Scopes, Scope{in.path, in.revision})
		m.Inputs = append(m.Inputs, Input{Path: in.path, Revision: in.revision, Blob: git(t, root, "rev-parse", in.revision+":"+in.path), SHA256: Digest(raw), Provider: "adapter", Purpose: in.purpose})
	}
	return root, m
}
func TestCorpusIndependentProviderAndNativeObservation(t *testing.T) {
	t.Run("DCP-V1-005 DCP-V1-007 DCP-V1-009 provider", func(t *testing.T) {
		root, m := providerFixture(t, nil)
		a, err := Build(context.Background(), root, m)
		if err != nil {
			t.Fatal(err)
		}
		if len(a.Observations) != 1 || a.Observations[0].Document.Run.Execution.State != testvalidity.ExecutionFailed {
			t.Fatal("forged carried projection was trusted")
		}
		if a.Observations[0].Document.Run.Freshness.State != testvalidity.FreshnessUnknown {
			t.Fatal("opaque retained identity became current")
		}
		if a.Observations[0].Document.Promotable == nil || *a.Observations[0].Document.Promotable {
			t.Fatal("preview run promoted")
		}
		if a.Journeys[0].Status != "non_ui" {
			t.Fatal("invented journey")
		}
		fresh, _, err := Freshness(context.Background(), root, a)
		if err != nil || fresh != "fresh" {
			t.Fatalf("later provider commit staled source: %s %v", fresh, err)
		}
		empty, err := Query(a, Request{Operation: "related", ID: "adapter:count"}, fresh, nil)
		if err != nil || empty.Miss != "no-match" {
			t.Fatalf("present zero confused with absence: %+v %v", empty, err)
		}
		impact := Impact(a, []string{"src/value.go"}, fresh)
		if len(impact["subjects"].([]Subject)) == 0 {
			t.Fatal("path did not reach documented subjects")
		}
	})
}
func TestCorpusHostileProviderRefusals(t *testing.T) {
	t.Run("DCP-V1-004 DCP-V1-005 hostile", func(t *testing.T) {
		cases := map[string]func(*ProviderRecord){
			"version drift": func(r *ProviderRecord) { r.Version = "2" },
			"unknown relationship": func(r *ProviderRecord) {
				r.Relations = []Relation{{"adapter:r", "adapter:count", "adapter:count", "navigates_to", "adapter", r.Subjects[0].Evidence}}
			},
			"false authority": func(r *ProviderRecord) { r.Subjects[0].Evidence.Anchors[0].Authority = "accepted-spec" },
			"generated verified": func(r *ProviderRecord) {
				r.Claims[0].Evidence.Derivation = "generated"
				r.Claims[0].Evidence.Trust = "verified"
			},
			"moved anchor":          func(r *ProviderRecord) { r.Subjects[0].Evidence.Anchors[0].Start = 1 },
			"empty evidence":        func(r *ProviderRecord) { r.Claims[0].Evidence.Anchors = nil },
			"self verified journey": func(r *ProviderRecord) { r.Journeys[0].Status = "verified" },
			"undeclared capability": func(r *ProviderRecord) { r.Capabilities = nil },
			"unknown capability": func(r *ProviderRecord) {
				r.Capabilities = append(r.Capabilities, CapabilityDeclaration{"magic", "present", "invented"})
			},
			"unrelated run":  func(r *ProviderRecord) { r.Observations[0].RunID = strings.Repeat("a", 64) },
			"unrelated test": func(r *ProviderRecord) { r.Observations[0].Test = "TestOther" },
			"source drift":   func(r *ProviderRecord) { r.Source.Revision = strings.Repeat("b", 40) },
		}
		for name, edit := range cases {
			t.Run(name, func(t *testing.T) {
				root, m := providerFixture(t, edit)
				if _, err := Build(context.Background(), root, m); err == nil {
					t.Fatal("hostile provider accepted")
				}
			})
		}
	})
}
func TestCorpusUnsupportedAndGeneratedInputClosure(t *testing.T) {
	t.Run("DCP-V1-003 unsupported", func(t *testing.T) {
		root, m := fixture(t)
		for name, body := range map[string]string{"unknown.zig": "const unsupported = 1;\n", "generated.json": `{"schema":"corvint-evidence-corpus/1"}`} {
			if err := os.WriteFile(filepath.Join(root, "src", name), []byte(body), 0600); err != nil {
				t.Fatal(err)
			}
		}
		git(t, root, "add", "src")
		git(t, root, "commit", "-qm", "additional inputs")
		revision := git(t, root, "rev-parse", "HEAD")
		inventory, err := Inventory(context.Background(), root, revision, "src", m.BuiltAt)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := Build(context.Background(), root, inventory); err == nil {
			t.Fatal("generated output read as source")
		}
		// Exact-file scope proves unsupported input remains visible as a gap.
		unsupported, err := Inventory(context.Background(), root, revision, "src/unknown.zig", m.BuiltAt)
		if err != nil {
			t.Fatal(err)
		}
		a, err := Build(context.Background(), root, unsupported)
		if err != nil {
			t.Fatal(err)
		}
		if len(a.Gaps) == 0 {
			t.Fatal("unsupported analyzer became valid empty answer")
		}
		inventory.Inputs = inventory.Inputs[:len(inventory.Inputs)-1]
		if _, err := Build(context.Background(), root, inventory); err == nil {
			t.Fatal("undeclared inventory accepted")
		}
	})
}
func TestCorpusUnknownAndConflictStayExplicit(t *testing.T) {
	t.Run("DCP-V1-005 DCP-V1-010 unknown", func(t *testing.T) {
		root, m := providerFixture(t, func(r *ProviderRecord) {
			r.Claims[0].Evidence = Evidence{Derivation: "declared", Trust: "generated", State: "unknown", Freshness: "fresh", Anchors: []Anchor{}, Unknown: "provider has not collected evidence", Limitations: []string{}}
			r.Subjects[0].Evidence.State = "conflicted"
		})
		a, err := Build(context.Background(), root, m)
		if err != nil {
			t.Fatal(err)
		}
		data, _ := json.Marshal(a)
		if !strings.Contains(string(data), "provider has not collected evidence") || !strings.Contains(string(data), "conflicted") {
			t.Fatal("evidence axes collapsed")
		}
	})
}
func TestCorpusMaintenanceHumanProseAndTampering(t *testing.T) {
	t.Run("DCP-V1-017 maintenance", func(t *testing.T) {
		root, m := fixture(t)
		a, err := Build(context.Background(), root, m)
		if err != nil {
			t.Fatal(err)
		}
		p := filepath.Join(root, "human.md")
		human := []byte("# Human prose\n\nKeep **exactly** this.\n")
		if err := os.WriteFile(p, human, 0640); err != nil {
			t.Fatal(err)
		}
		preview, err := Maintain(root, "human.md", a, false)
		if err != nil {
			t.Fatal(err)
		}
		actual, _ := os.ReadFile(p)
		if string(actual) != string(human) {
			t.Fatal("preview wrote page")
		}
		applied, err := Maintain(root, "human.md", a, true)
		if err != nil {
			t.Fatal(err)
		}
		if !applied.Applied || !strings.HasPrefix(applied.Proposed, string(human)) {
			t.Fatal("human prose was overwritten")
		}
		info, _ := os.Stat(p)
		if info.Mode().Perm() != 0640 {
			t.Fatal("page permission changed")
		}
		again, err := Maintain(root, "human.md", a, false)
		if err != nil || again.Proposed != preview.Proposed {
			t.Fatalf("maintenance nondeterminism: %v", err)
		}
		edited := strings.Replace(again.Proposed, "Identity checks", "Tampered identity checks", 1)
		if err := os.WriteFile(p, []byte(edited), 0640); err != nil {
			t.Fatal(err)
		}
		if _, err := Maintain(root, "human.md", a, true); err == nil {
			t.Fatal("tampered generated block accepted")
		}
		if err := os.Symlink(p, filepath.Join(root, "link.md")); err != nil {
			t.Fatal(err)
		}
		if _, err := Maintain(root, "link.md", a, true); err == nil {
			t.Fatal("symlink accepted")
		}
		if err := os.WriteFile(p, []byte("Intent status: accepted\n"), 0640); err != nil {
			t.Fatal(err)
		}
		if _, err := Maintain(root, "human.md", a, true); err == nil {
			t.Fatal("accepted prose overwritten")
		}
	})
}
