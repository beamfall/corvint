package main

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/doccorpus"
	"github.com/Beamfall/corvint/internal/mcp/corpusbridge"
)

func corpusCLI(t *testing.T, root string, args ...string) (int, string, string) {
	t.Helper()
	var out, err bytes.Buffer
	code := runContext(context.Background(), append([]string{"--root", root}, args...), strings.NewReader(""), &out, &err)
	return code, out.String(), err.String()
}
func writeCorpusFixture(t *testing.T, root, rev, scope string) string {
	t.Helper()
	m, err := doccorpus.Inventory(context.Background(), root, rev, scope, "2026-09-19T00:00:00Z")
	if err != nil {
		t.Fatal(err)
	}
	data, _ := doccorpus.Encode(m)
	cemWrite(t, root, "corpus-input.json", string(data))
	code, out, stderr := corpusCLI(t, root, "docs", "corpus", "build", "--manifest", "corpus-input.json")
	if code != 0 {
		t.Fatal(stderr)
	}
	cemWrite(t, root, "corpus.json", out)
	return out
}
func TestCorpusCLIBuildQueryAndMCPParity(t *testing.T) {
	t.Run("DCP-V1-012 DCP-V1-016 parity", func(t *testing.T) {
		root := taskContextRepository(t)
		rev := cemGit(t, root, "rev-parse", "HEAD")
		artifact := writeCorpusFixture(t, root, rev, "cache")
		code, out, stderr := corpusCLI(t, root, "docs", "corpus", "search", "--artifact", "corpus.json", "--query", "Split")
		if code != 0 {
			t.Fatal(stderr)
		}
		var receipt doccorpus.Receipt
		if err := json.Unmarshal([]byte(out), &receipt); err != nil {
			t.Fatal(err)
		}
		if len(receipt.Results) == 0 {
			t.Fatal("CLI search has no source result")
		}
		registry, err := corpusbridge.New(root, "corpus.json")
		if err != nil {
			t.Fatal(err)
		}
		_, text, failure, protocol := registry.Call(context.Background(), "corvint.docs_search", []byte(`{"query":"Split"}`))
		if failure != nil || protocol != nil || text != out {
			t.Fatalf("CLI/MCP receipt bytes differ: %v %v\n%s\n%s", failure, protocol, out, text)
		}
		for _, tool := range registry.Tools() {
			if tool.Name == "corvint.docs_get_journey" || tool.Name == "corvint.docs_get_stability" {
				t.Fatal("absent capability advertised")
			}
		}
		if _, _, _, err := registry.Call(context.Background(), "corvint.docs_get_journey", []byte(`{"id":"missing"}`)); err == nil {
			t.Fatal("absent tool callable")
		}
		cemWrite(t, root, "corpus.json", strings.Replace(artifact, "Repository evidence", "tampered", 1))
		if _, _, failure, _ := registry.Call(context.Background(), "corvint.docs_info", []byte(`{}`)); failure == nil {
			t.Fatal("artifact drift accepted")
		}
	})
}

func TestBehaviorAdapterCLI(t *testing.T) {
	root := taskContextRepository(t)
	revision := strings.Repeat("1", 40)
	source := doccorpus.Repository{ID: strings.Repeat("2", 40), Revision: revision}
	revisions := doccorpus.BehaviorRevisions{App: doccorpus.Repository{ID: strings.Repeat("3", 40), Revision: revision}, E2E: source, Docs: doccorpus.Repository{ID: strings.Repeat("4", 40), Revision: revision}}
	migrationRaw, _ := doccorpus.Encode(doccorpus.BehaviorMigration{Schema: 2, ContractID: "contract", SourceRevision: revision, DocumentationRevision: revision, Revisions: revisions})
	discoveryRaw, _ := doccorpus.Encode(doccorpus.BehaviorDiscovery{Schema: "corvint-playwright-discovery/1", Mode: "live-playwright-list", Revisions: revisions, Executions: []doccorpus.BehaviorExecution{}})
	empty := []byte("{\"items\":[]}\n")
	input := func(id string, raw []byte, kind string) doccorpus.BehaviorAdapterInput {
		digest := doccorpus.Digest(raw)
		return doccorpus.BehaviorAdapterInput{ID: id, Anchor: doccorpus.Anchor{Repository: source.ID, Revision: revision, Path: "evidence/" + id + ".json", Blob: strings.Repeat("5", 40), SHA256: digest, Start: 1, End: 1, SpanSHA256: digest, Authority: "external-provider", Kind: kind, Reason: "synthetic CLI fixture"}, Document: string(raw)}
	}
	request := doccorpus.BehaviorAdapterRequest{
		Schema: doccorpus.BehaviorAdapterRequestSchema, ProviderID: "behavior", ProviderVersion: "1", ContractID: "contract",
		Source: source, Revisions: revisions, SourceRevision: revision, DocumentationRevision: revision,
		MigrationInput: "migration", DiscoveryInput: "discovery",
		Inputs: []doccorpus.BehaviorAdapterInput{input("migration", migrationRaw, "declared"), input("discovery", discoveryRaw, "observed"), input("flows", empty, "review"), input("variations", empty, "review"), input("candidates", empty, "declared"), input("tests", empty, "declared")},
		Mappings: []doccorpus.BehaviorAdapterMapping{
			{Kind: "flows", Input: "flows", Records: "/items", Fields: map[string]string{"id": "/id", "derivation": "/derivation", "evidence": "/evidence", "required_pages": "/required_pages", "negative_controls": "/negative_controls", "ordered_events": "/ordered_events"}},
			{Kind: "variations", Input: "variations", Records: "/items", Fields: map[string]string{"id": "/id", "flow": "/flow", "preconditions": "/preconditions", "actions": "/actions", "observable_facts": "/observable_facts", "expected_outcomes": "/expected_outcomes", "projects": "/projects", "tests": "/tests"}},
			{Kind: "candidates", Input: "candidates", Records: "/items", Fields: map[string]string{"id": "/id", "evidence": "/evidence", "flows": "/flows"}},
			{Kind: "tests", Input: "tests", Records: "/items", Fields: map[string]string{"id": "/id", "project": "/project", "title": "/title", "evidence": "/evidence", "flows": "/flows", "criteria": "/criteria", "assertions": "/assertions", "variation_claims": "/variation_claims"}},
		},
		Observations: []doccorpus.ObservationLink{},
	}
	raw, err := doccorpus.Encode(request)
	if err != nil {
		t.Fatal(err)
	}
	cemWrite(t, root, "request.json", string(raw))
	code, out, stderr := corpusCLI(t, root, "docs", "corpus", "behavior-adapter", "--input", "request.json")
	if code != 0 {
		t.Fatal(stderr)
	}
	var result doccorpus.BehaviorAdapterResult
	if err := json.Unmarshal([]byte(out), &result); err != nil {
		t.Fatal(err)
	}
	if result.Schema != doccorpus.BehaviorAdapterResultSchema || result.Fallback != "full-relevant-suite" || len(result.Coverage) != 5 || result.Coverage[0].Defined {
		t.Fatalf("invalid CLI result: %+v", result)
	}
}
func TestCorpusNativeReadIntegrationParity(t *testing.T) {
	t.Run("DCP-V1-013 native", func(t *testing.T) {
		root := taskContextRepository(t)
		rev := cemGit(t, root, "rev-parse", "HEAD")
		writeCorpusFixture(t, root, rev, "cache")
		commands := [][]string{{"query", "--task", "Split implementation"}, {"context", "--task", "Split implementation", "--subject", "cache/demux.go"}, {"impact", "cache/demux.go"}, {"affected"}, {"test-validity"}}
		for _, args := range commands {
			t.Run(args[0], func(t *testing.T) {
				code, base, stderr := corpusCLI(t, root, args...)
				if code != 0 {
					t.Fatalf("native baseline %d: %s", code, stderr)
				}
				code, joined, stderr := corpusCLI(t, root, append(args, "--corpus=corpus.json")...)
				if code != 0 {
					t.Fatalf("corpus %d: %s", code, stderr)
				}
				var left, right map[string]any
				if err := json.Unmarshal([]byte(base), &left); err != nil {
					t.Fatal(err)
				}
				if err := json.Unmarshal([]byte(joined), &right); err != nil {
					t.Fatal(err)
				}
				if right["documentation"] == nil {
					t.Fatal("native evidence attachment missing")
				}
				delete(right, "documentation")
				if !reflect.DeepEqual(left, right) {
					t.Fatal("core receipt changed under opt-in attachment")
				}
			})
		}
		code, _, _ := corpusCLI(t, root, "record", "--corpus=corpus.json")
		if code != 2 {
			t.Fatal("corpus wrapper admitted mutation")
		}
	})
}
func TestCorpusCEMProjectionBindsOriginalEvidence(t *testing.T) {
	t.Run("DCP-V1-014 citation", func(t *testing.T) {
		root, base, target := cemRepo(t)
		writeCorpusFixture(t, root, base, "docs/rule.txt")
		code, _, stderr := corpusCLI(t, root, "cem", "prepare", "--base", base, "--target", target)
		if code != 0 {
			t.Fatal(stderr)
		}
		code, _, stderr = corpusCLI(t, root, "cem", "cite", "--map", ".corvint/change.cem.json", "--hunk", "1", "--evidence-path", "docs/rule.txt", "--lines", "1:1", "--relation", "specification")
		if code != 0 {
			t.Fatal(stderr)
		}
		code, out, stderr := corpusCLI(t, root, "docs", "corpus", "cem", "--artifact", "corpus.json", "--cem", ".corvint/change.cem.json", "--id", "native:file:docs/rule.txt:excerpt")
		if code != 0 {
			t.Fatal(stderr)
		}
		if !strings.Contains(out, `"bound_in_cem":true`) || !strings.Contains(out, `"authority":"generated-documentation"`) {
			t.Fatal(out)
		}
		before, err := os.ReadFile(filepath.Join(root, ".corvint/change.cem.json"))
		if err != nil {
			t.Fatal(err)
		}
		code, _, stderr = corpusCLI(t, root, "cem", "status", "--map", ".corvint/change.cem.json", "--expected-base", base, "--target", target, "--corpus=corpus.json")
		if code != 0 {
			t.Fatal(stderr)
		}
		after, _ := os.ReadFile(filepath.Join(root, ".corvint/change.cem.json"))
		if !bytes.Equal(before, after) {
			t.Fatal("documentation review mutated CEM")
		}
	})
}

func TestCorpusNativeReceiptJoin(t *testing.T) {
	t.Run("DCP-V1-007 DCP-V1-013 observation", func(t *testing.T) {
		root := taskContextRepository(t)
		source := cemGit(t, root, "rev-parse", "HEAD")
		manifest, err := doccorpus.Inventory(context.Background(), root, source, "cache", "2026-09-19T00:00:00Z")
		if err != nil {
			t.Fatal(err)
		}
		base, err := doccorpus.Build(context.Background(), root, manifest)
		if err != nil {
			t.Fatal(err)
		}
		subject := ""
		for _, s := range base.Subjects {
			if s.Kind == "test" {
				subject = s.ID
				break
			}
		}
		if subject == "" {
			t.Fatal("test declaration missing")
		}
		raw := `{"profile":"corvint-go-live-session-event/0","state":"failed","identity":"opaque","sequence":1,"scope":["./cache"],"detail":"","testProjections":[{"package":"example.test/ctx/cache","name":"TestSplit","action":"fail"}],"testProjectionsOmitted":0}`
		cemWrite(t, root, "evidence/run.json", raw)
		cemGit(t, root, "add", "evidence/run.json")
		cemGit(t, root, "commit", "-qm", "run")
		runRev := cemGit(t, root, "rev-parse", "HEAD")
		record := doccorpus.ProviderRecord{Schema: doccorpus.ProviderSchema, ID: "retained", Version: "1", Source: manifest.Repository, Capabilities: []doccorpus.CapabilityDeclaration{{Name: "observations", State: "present", Reason: "explicit receipt"}}, Observations: []doccorpus.ObservationLink{{ID: "retained:run", Subject: subject, Input: "evidence/run.json", InputRevision: runRev, SourceRevision: source, RunID: doccorpus.Digest([]byte(raw)), Package: "example.test/ctx/cache", Test: "TestSplit"}}}
		bytes, err := doccorpus.Encode(record)
		if err != nil {
			t.Fatal(err)
		}
		cemWrite(t, root, "evidence/provider.json", string(bytes))
		cemGit(t, root, "add", "evidence/provider.json")
		cemGit(t, root, "commit", "-qm", "provider")
		providerRev := cemGit(t, root, "rev-parse", "HEAD")
		manifest.Providers = append(manifest.Providers, doccorpus.Provider{ID: "retained", Kind: "records", Version: "1", Revision: providerRev, Record: "evidence/provider.json"})
		for _, in := range []struct{ path, rev, purpose, data string }{{"evidence/run.json", runRev, "observation", raw}, {"evidence/provider.json", providerRev, "provider", string(bytes)}} {
			manifest.Scopes = append(manifest.Scopes, doccorpus.Scope{Path: in.path, Revision: in.rev})
			manifest.Inputs = append(manifest.Inputs, doccorpus.Input{Path: in.path, Revision: in.rev, Blob: cemGit(t, root, "rev-parse", in.rev+":"+in.path), SHA256: doccorpus.Digest([]byte(in.data)), Provider: "retained", Purpose: in.purpose})
		}
		corpus, err := doccorpus.Build(context.Background(), root, manifest)
		if err != nil {
			t.Fatal(err)
		}
		data, _ := doccorpus.Encode(corpus)
		cemWrite(t, root, "corpus.json", string(data))
		code, out, stderr := corpusCLI(t, root, "test-validity", "--receipt", filepath.Join(root, "evidence/run.json"), "--corpus=corpus.json")
		if code != 0 {
			t.Fatal(stderr)
		}
		var result struct {
			Documentation struct {
				Observations []doccorpus.Observation `json:"observations"`
			} `json:"documentation"`
		}
		if err := json.Unmarshal([]byte(out), &result); err != nil {
			t.Fatal(err)
		}
		if len(result.Documentation.Observations) != 1 || result.Documentation.Observations[0].Document.Tests[0].Projection.Execution.State != "FAILED" {
			t.Fatalf("exact native failed receipt not preserved: %s", out)
		}
	})
}

func TestCorpusNativeOptionIsolation(t *testing.T) {
	t.Run("DCP-V1-012 DCP-V1-018 options", func(t *testing.T) {
		root := taskContextRepository(t)
		for _, args := range [][]string{
			{"query", "--task", "--corpus"},
			{"query", "--task", "--corpus=corpus.json"},
			{"work", "plan-fixture", "--executor", "--corpus=fixture.json"},
			{"work", "plan-fixture", "--observations", "--corpus=fixture.json"},
			{"work", "init", "--repository", "--corpus=fixture.json"},
		} {
			var out, stderr bytes.Buffer
			_, handled := runCorpusIntegration(context.Background(), append([]string{"--root", root}, args...), strings.NewReader(""), &out, &stderr)
			if handled {
				t.Fatalf("native option value intercepted: %v", args)
			}
		}
		for _, args := range [][]string{{"docs", "corpus", "render", "--artifact", "corpus.json", "--apply"}, {"docs", "corpus", "info", "--artifact", "corpus.json", "--id", "x"}, {"docs", "corpus", "search", "--artifact", "corpus.json"}} {
			code, _, _ := corpusCLI(t, root, args...)
			if code == 0 {
				t.Fatalf("invalid operation flags accepted: %v", args)
			}
		}
	})
}

func TestCorpusImpactNativePathSelection(t *testing.T) {
	t.Run("DCP-V1-013 native range and flags", func(t *testing.T) {
		root := taskContextRepository(t)
		cemWrite(t, root, ".gitignore", "corpus-input.json\ncorpus.json\n")
		cemGit(t, root, "add", ".gitignore")
		cemGit(t, root, "commit", "-qm", "ignore explicit outputs")
		base := cemGit(t, root, "rev-parse", "HEAD")
		cemWrite(t, root, "cache/demux.go", "package cache\nfunc Split(key string) string { return key + \"!\" }\n")
		cemGit(t, root, "add", "cache/demux.go")
		cemGit(t, root, "commit", "-qm", "changed implementation")
		writeCorpusFixture(t, root, cemGit(t, root, "rev-parse", "HEAD"), "cache")
		for _, args := range [][]string{{"impact", "--base", base}, {"impact", "--", "cache/demux.go"}, {"impact", "--working-tree-untracked", "cache/new.go"}} {
			if args[1] == "--working-tree-untracked" {
				cemWrite(t, root, "cache/new.go", "package cache\nfunc New() {}\n")
			}
			// Place the additive flag before the native positional delimiter.
			invocation := append([]string{args[0], "--corpus=corpus.json"}, args[1:]...)
			code, out, stderr := corpusCLI(t, root, invocation...)
			if code != 0 {
				t.Fatalf("%v: %d %s", args, code, stderr)
			}
			var result struct {
				Documentation struct {
					Impact struct {
						Subjects []doccorpus.Subject `json:"subjects"`
					} `json:"impact"`
				} `json:"documentation"`
			}
			if err := json.Unmarshal([]byte(out), &result); err != nil {
				t.Fatal(err)
			}
			if args[1] == "--working-tree-untracked" {
				if !strings.Contains(out, "no documented subject for cache/new.go") {
					t.Fatal("untracked path was lost")
				}
				continue
			}
			if len(result.Documentation.Impact.Subjects) == 0 {
				t.Fatalf("native paths did not join: %v %s", args, out)
			}
		}
	})
}

func TestCorpusRootPreambleRefusesOptionLikeValue(t *testing.T) {
	var out, stderr bytes.Buffer
	args := []string{"--root", "--corpus=corpus.json", "docs", "corpus", "info", "--artifact", "a.json"}
	code, handled := runCorpusIntegration(context.Background(), args, strings.NewReader(""), &out, &stderr)
	if !handled || code != 2 || out.Len() != 0 {
		t.Fatalf("--corpus swallowed as the --root value: handled=%v code=%d stdout=%q stderr=%q", handled, code, out.String(), stderr.String())
	}
}

func TestCorpusRelayWriteFailureReportsOutputFailed(t *testing.T) {
	root := taskContextRepository(t)
	writeCorpusFixture(t, root, cemGit(t, root, "rev-parse", "HEAD"), "cache")
	args := []string{"--root", root, "query", "--corpus=corpus.json"}
	var baseline, baselineErr bytes.Buffer
	if code, handled := runCorpusIntegration(context.Background(), args, strings.NewReader(""), &baseline, &baselineErr); !handled || code == 0 {
		t.Fatalf("native relay baseline: handled=%v code=%d stderr=%q", handled, code, baselineErr.String())
	}
	var out stdoutBrokenPipeWriter
	var stderr bytes.Buffer
	code, _ := runCorpusIntegration(context.Background(), args, strings.NewReader(""), &out, &stderr)
	if code != 2 || !strings.Contains(stderr.String(), `"output-failed"`) {
		t.Fatalf("exit %d, want 2 with output-failed: stderr=%q", code, stderr.String())
	}
}
