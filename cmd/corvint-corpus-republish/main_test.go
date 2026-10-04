package main

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/corpusrepublish"
	"github.com/Beamfall/corvint/internal/doccorpus"
	"github.com/Beamfall/corvint/internal/postmergeconnector"
)

func testGit(t *testing.T, root string, args ...string) string {
	t.Helper()
	c := exec.Command("git", append([]string{"-C", root}, args...)...)
	c.Env = append(os.Environ(), "GIT_CONFIG_GLOBAL=/dev/null", "GIT_CONFIG_SYSTEM=/dev/null", "GIT_CONFIG_NOSYSTEM=1")
	raw, e := c.CombinedOutput()
	if e != nil {
		t.Fatalf("git %v %s", e, raw)
	}
	return strings.TrimSpace(string(raw))
}
func temp(t *testing.T) string {
	t.Helper()
	p, e := filepath.EvalSymlinks(t.TempDir())
	if e != nil {
		t.Fatal(e)
	}
	return p
}
func put(t *testing.T, path string, b []byte) {
	t.Helper()
	if e := os.WriteFile(path, b, 0600); e != nil {
		t.Fatal(e)
	}
}
func TestCLIReferenceWorkflow(t *testing.T) {
	t.Run("RCP-V0-001 RCP-V0-003 RCP-V0-009 RCP-V0-010 RCP-V0-011 complete actual command and pending disk consumer", func(t *testing.T) {
		t.Setenv("GITHUB_TOKEN", "credential-sentinel-not-read")
		root, host := temp(t), temp(t)
		testGit(t, root, "init", "-q")
		testGit(t, root, "config", "user.name", "Fixture")
		testGit(t, root, "config", "user.email", "fixture@example.invalid")
		if e := os.Mkdir(filepath.Join(root, "src"), 0700); e != nil {
			t.Fatal(e)
		}
		put(t, filepath.Join(root, "src/app.go"), []byte("package app\nfunc Count() int {return 1}\n"))
		testGit(t, root, "add", ".")
		testGit(t, root, "commit", "-qm", "source")
		source := testGit(t, root, "rev-parse", "HEAD")
		m, e := doccorpus.Inventory(context.Background(), root, source, "src", "2026-10-02T00:00:00Z")
		if e != nil {
			t.Fatal(e)
		}
		m.Schema = doccorpus.ManifestSchemaV2
		record := doccorpus.ProviderRecord{Schema: doccorpus.ProviderSchema, ID: "docs", Version: "1", Source: m.Repository, Subjects: []doccorpus.Subject{{ID: "docs:page", Kind: "document", Name: "approved first documentation", Provider: "docs", Evidence: doccorpus.Evidence{Derivation: "declared", Trust: "generated", State: "unknown", Freshness: "unknown", Unknown: "fixture declaration", Anchors: []doccorpus.Anchor{}, Limitations: []string{}}}}, Capabilities: []doccorpus.CapabilityDeclaration{{Name: "subjects", State: "present", Reason: "explicit document inventory"}}}
		requestPath, policyPath := filepath.Join(host, "request.json"), filepath.Join(host, "policy.json")
		makeGrant := func(previous string, generation uint64, stateDigest string) ([]string, string) {
			raw, err := doccorpus.Encode(record)
			if err != nil {
				t.Fatal(err)
			}
			put(t, filepath.Join(root, "docs.json"), raw)
			testGit(t, root, "add", "docs.json")
			testGit(t, root, "commit", "-qm", "documentation candidate")
			docs := testGit(t, root, "rev-parse", "HEAD")
			m.Providers = []doccorpus.Provider{m.Providers[0], {ID: "docs", Kind: "records", Version: "1", Revision: docs, Record: "docs.json"}}
			m.Inputs = m.Inputs[:1]
			m.Inputs = append(m.Inputs, doccorpus.Input{Path: "docs.json", Revision: docs, Blob: testGit(t, root, "rev-parse", docs+":docs.json"), SHA256: doccorpus.Digest(raw), Provider: "docs", Purpose: "provider"})
			m.Scopes = []doccorpus.Scope{{Path: "src", Revision: source}, {Path: "docs.json", Revision: docs}}
			manifest, _ := doccorpus.Encode(m)
			binding := corpusrepublish.Binding{Repository: m.Repository.ID, Target: "corpus", DocumentationRevision: docs, SourceRevision: source, ManifestSHA256: doccorpus.Digest(manifest), BuiltAt: m.BuiltAt, ReuseEligibility: []doccorpus.ShardIdentity{}}
			req := corpusrepublish.Request{Profile: corpusrepublish.Profile, Binding: binding, Manifest: m, EvidenceURL: "https://example.invalid/evidence"}
			policy := corpusrepublish.Policy{Profile: corpusrepublish.Profile, Binding: binding, Decision: "APPROVED", DecisionRecord: "host-fixture-grant", PreviousResultSHA256: "NONE", PreviousArtifactSHA256: "NONE", PreviousSidecarProfile: "NONE", PreviousSidecarSHA256: "NONE", ExpectedPendingGeneration: generation, ExpectedPendingSHA256: stateDigest, AllowedShardRetirements: []doccorpus.ShardIdentity{}, URLOrigins: []string{"https://example.invalid"}}
			extra := []string{}
			if previous != "" {
				r, _ := os.ReadFile(filepath.Join(previous, "result.json"))
				a, _ := os.ReadFile(filepath.Join(previous, "corpus.json"))
				policy.PreviousResultSHA256 = doccorpus.Digest(r)
				policy.PreviousArtifactSHA256 = doccorpus.Digest(a)
				extra = []string{"--previous-result", filepath.Join(previous, "result.json"), "--previous-artifact", filepath.Join(previous, "corpus.json")}
			}
			rr, _ := corpusrepublish.Encode(req)
			pp, _ := corpusrepublish.Encode(policy)
			put(t, requestPath, rr)
			put(t, policyPath, pp)
			return append([]string{"--trusted-local", "--root", root, "--request", requestPath, "--policy", policyPath, "--expected-policy-sha256", doccorpus.Digest(pp)}, extra...), docs
		}
		firstArgs, firstDocs := makeGrant("", 0, "NONE")
		a, b := temp(t), temp(t)
		invoke := func(mode string, args []string) []byte {
			t.Helper()
			var out, errout bytes.Buffer
			if e := run(context.Background(), append([]string{mode}, args...), &out, &errout); e != nil {
				t.Fatalf("CLI %v stderr %s", e, errout.Bytes())
			}
			if bytes.Contains(out.Bytes(), []byte("credential-sentinel")) {
				t.Fatal("credential leaked")
			}
			return out.Bytes()
		}
		invoke("build", append(append([]string{}, firstArgs...), "--output-dir", a))
		invoke("build", append(append([]string{}, firstArgs...), "--output-dir", b))
		for _, file := range []string{"result.json", "corpus.json", "index.json", "request.json", "consumer.json"} {
			x, _ := os.ReadFile(filepath.Join(a, file))
			y, _ := os.ReadFile(filepath.Join(b, file))
			if !bytes.Equal(x, y) {
				t.Fatalf("two actual command builds differ %s", file)
			}
		}
		pending := filepath.Join(host, "pending.json")
		queueArgs := append(append([]string{}, firstArgs...), "--pending", pending)
		invoke("queue", queueArgs)
		before, _ := os.ReadFile(pending)
		invoke("queue", queueArgs)
		again, _ := os.ReadFile(pending)
		if !bytes.Equal(before, again) {
			t.Fatal("actual queue retry changed bytes")
		}
		oldRequest, _ := os.ReadFile(requestPath)
		oldPolicy, _ := os.ReadFile(policyPath)
		record.Subjects[0].Name = "approved second documentation"
		secondArgs, secondDocs := makeGrant(a, 1, doccorpus.Digest(before))
		if firstDocs == secondDocs {
			t.Fatal("no separate documentation event")
		}
		nextDir := temp(t)
		invoke("build", append(append([]string{}, secondArgs...), "--output-dir", nextDir))
		invoke("queue", append(append([]string{}, secondArgs...), "--pending", pending))
		now, _ := os.ReadFile(pending)
		var state postmergeconnector.RepublishPending
		if postmergeconnector.Decode(now, &state) != nil || state.Generation != 2 || state.Plan.Input.DocumentationRevision != secondDocs || state.Plan.Request.Key != postmergeconnector.RepublishKey(m.Repository.ID, "corpus") {
			t.Fatal("actual pending replacement invalid")
		}
		put(t, requestPath, oldRequest)
		put(t, policyPath, oldPolicy)
		var out bytes.Buffer
		if e := run(context.Background(), append([]string{"queue"}, queueArgs...), &out, &out); e == nil {
			t.Fatal("old command grant replaced newer")
		}
		again, _ = os.ReadFile(pending)
		if !bytes.Equal(now, again) {
			t.Fatal("stale CLI changed bytes")
		}
		var rejected corpusrepublish.Policy
		if e := corpusrepublish.Decode(oldPolicy, &rejected); e != nil {
			t.Fatal(e)
		}
		rejected.Decision = "REVOKED"
		raw, _ := corpusrepublish.Encode(rejected)
		put(t, policyPath, raw)
		refusedDir := temp(t)
		badArgs := append([]string{}, firstArgs...)
		badArgs[len(badArgs)-1] = doccorpus.Digest(raw)
		badArgs = append(badArgs, "--output-dir", refusedDir)
		if e := run(context.Background(), append([]string{"build"}, badArgs...), &out, &out); e == nil {
			t.Fatal("revoked command built")
		}
		entries, _ := os.ReadDir(refusedDir)
		if len(entries) != 0 {
			t.Fatal("refusal wrote outputs")
		}
		// A policy copied into an author checkout is not admitted as host-owned.
		inside := filepath.Join(root, "policy.json")
		put(t, inside, oldPolicy)
		badArgs = append([]string{}, firstArgs...)
		for i, s := range badArgs {
			if s == "--policy" {
				badArgs[i+1] = inside
			}
		}
		badArgs = append(badArgs, "--output-dir", temp(t))
		if e := run(context.Background(), append([]string{"build"}, badArgs...), &out, &out); e == nil {
			t.Fatal("author policy admitted")
		}
	})
}
func TestCLIExplicitHostBoundary(t *testing.T) {
	t.Run("RCP-V0-011 explicit trusted-local invocation and cancelled context", func(t *testing.T) {
		var out bytes.Buffer
		for _, args := range [][]string{nil, {"publish"}, {"build"}, {"queue", "--trusted-local"}} {
			if e := run(context.Background(), args, &out, &out); e == nil {
				t.Fatal("incomplete host invocation accepted")
			}
		}
	})
}
