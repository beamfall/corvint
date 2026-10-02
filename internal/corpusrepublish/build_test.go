package corpusrepublish

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/doccorpus"
)

func git(t *testing.T, root string, args ...string) string {
	t.Helper()
	c := exec.Command("git", append([]string{"-C", root}, args...)...)
	c.Env = append(os.Environ(), "GIT_CONFIG_GLOBAL=/dev/null", "GIT_CONFIG_SYSTEM=/dev/null", "GIT_CONFIG_NOSYSTEM=1")
	b, e := c.CombinedOutput()
	if e != nil {
		t.Fatalf("git %v %s", e, b)
	}
	return strings.TrimSpace(string(b))
}
func write(t *testing.T, root, path string, raw []byte) {
	t.Helper()
	if e := os.MkdirAll(filepath.Dir(filepath.Join(root, path)), 0700); e != nil {
		t.Fatal(e)
	}
	if e := os.WriteFile(filepath.Join(root, path), raw, 0600); e != nil {
		t.Fatal(e)
	}
}
func fixture(t *testing.T) (string, Request, Policy) {
	t.Helper()
	root, _ := filepath.EvalSymlinks(t.TempDir())
	git(t, root, "init", "-q")
	git(t, root, "config", "user.email", "fixture@example.invalid")
	git(t, root, "config", "user.name", "Fixture")
	write(t, root, "src/app.go", []byte("package app\nfunc Count() int { return 1 }\n"))
	git(t, root, "add", ".")
	git(t, root, "commit", "-qm", "source")
	source := git(t, root, "rev-parse", "HEAD")
	m, e := doccorpus.Inventory(context.Background(), root, source, "src", "2026-10-02T00:00:00Z")
	if e != nil {
		t.Fatal(e)
	}
	m.Schema = doccorpus.ManifestSchemaV2
	ev := doccorpus.Evidence{Derivation: "declared", Trust: "generated", State: "unknown", Freshness: "unknown", Unknown: "fixture declaration; no independent outcome", Anchors: []doccorpus.Anchor{}, Limitations: []string{}}
	a := doccorpus.ProviderRecord{Schema: doccorpus.AdoptionProviderSchema, ID: "docs", Version: "1", Source: m.Repository, Subjects: []doccorpus.Subject{{ID: "docs:a", Kind: "capability", Name: "Documentation A", Provider: "docs", Evidence: ev}}, Claims: []doccorpus.Claim{{ID: "docs:claim", Subject: "docs:a", Text: "Declared documentation", Provider: "docs", Evidence: ev}}, Relations: []doccorpus.Relation{{ID: "docs:edge", From: "docs:a", To: "docs:b", Type: "related_to", Provider: "docs", Evidence: ev}}, Journeys: []doccorpus.Journey{}, Observations: []doccorpus.ObservationLink{}, Details: map[string]doccorpus.RecordDetails{"docs:claim": {ClaimKind: "behavior"}}, Capabilities: []doccorpus.CapabilityDeclaration{{Name: "subjects", State: "present", Reason: "explicit subjects"}, {Name: "claims", State: "present", Reason: "explicit claims"}, {Name: "relations", State: "present", Reason: "explicit relation"}}}
	b := a
	b.Subjects = []doccorpus.Subject{{ID: "docs:b", Kind: "capability", Name: "Documentation B", Provider: "docs", Evidence: ev}}
	b.Claims = nil
	b.Relations = nil
	b.Details = nil
	paths := []string{"docs/a.json", "docs/b.json"}
	for i, p := range []doccorpus.ProviderRecord{a, b} {
		raw, e := doccorpus.Encode(p)
		if e != nil {
			t.Fatal(e)
		}
		write(t, root, paths[i], raw)
	}
	git(t, root, "add", "docs")
	git(t, root, "commit", "-qm", "approved docs candidate")
	docs := git(t, root, "rev-parse", "HEAD")
	m.Providers = append(m.Providers, doccorpus.Provider{ID: "docs", Kind: "records", Version: "1", Revision: docs, Shards: paths})
	m.Scopes = append(m.Scopes, doccorpus.Scope{Path: "docs", Revision: docs})
	for _, path := range paths {
		raw, _ := os.ReadFile(filepath.Join(root, path))
		m.Inputs = append(m.Inputs, doccorpus.Input{Path: path, Revision: docs, Blob: git(t, root, "rev-parse", docs+":"+path), SHA256: doccorpus.Digest(raw), Provider: "docs", Purpose: "provider"})
	}
	raw, _ := doccorpus.Encode(m)
	binding := Binding{Repository: m.Repository.ID, Target: "public-corpus", DocumentationRevision: docs, SourceRevision: source, ManifestSHA256: doccorpus.Digest(raw), BuiltAt: m.BuiltAt, ReuseEligibility: []doccorpus.ShardIdentity{}}
	req := Request{Profile, binding, m, "https://example.invalid/evidence"}
	p := Policy{Profile: Profile, Binding: binding, Decision: "APPROVED", DecisionRecord: "fixture-host-grant", PreviousResultSHA256: "NONE", PreviousArtifactSHA256: "NONE", PreviousSidecarProfile: "NONE", PreviousSidecarSHA256: "NONE", ExpectedPendingSHA256: "NONE", AllowedShardRetirements: []doccorpus.ShardIdentity{}, URLOrigins: []string{"https://example.invalid"}}
	return root, req, p
}
func build(t *testing.T, root string, r Request, p Policy, prior Output) (Output, error) {
	t.Helper()
	rr, e := Encode(r)
	if e != nil {
		t.Fatal(e)
	}
	pp, e := Encode(p)
	if e != nil {
		t.Fatal(e)
	}
	return Build(context.Background(), root, rr, pp, doccorpus.Digest(pp), prior.ResultBytes, prior.Corpus, prior.Sidecar)
}
func repin(t *testing.T, root string, r *Request, p *Policy) {
	t.Helper()
	rev := git(t, root, "rev-parse", "HEAD")
	for i := range r.Manifest.Providers {
		if r.Manifest.Providers[i].Kind == "records" {
			r.Manifest.Providers[i].Revision = rev
		}
	}
	for i := range r.Manifest.Scopes {
		if r.Manifest.Scopes[i].Path == "docs" || strings.HasPrefix(r.Manifest.Scopes[i].Path, "docs/") {
			r.Manifest.Scopes[i].Revision = rev
		}
	}
	for i := range r.Manifest.Inputs {
		in := &r.Manifest.Inputs[i]
		if in.Purpose == "provider" {
			in.Revision = rev
			in.Blob = git(t, root, "rev-parse", rev+":"+in.Path)
			raw, _ := os.ReadFile(filepath.Join(root, in.Path))
			in.SHA256 = doccorpus.Digest(raw)
		}
	}
	raw, _ := doccorpus.Encode(r.Manifest)
	r.Binding.DocumentationRevision = rev
	r.Binding.ManifestSHA256 = doccorpus.Digest(raw)
	p.Binding = r.Binding
}
func TestApprovedRepublish(t *testing.T) {
	t.Run("RCP-V0-001 RCP-V0-003 RCP-V0-004 RCP-V0-008 approved docs produce identical provenance and real indexed consumer", func(t *testing.T) {
		root, r, p := fixture(t)
		a, e := build(t, root, r, p, Output{})
		if e != nil {
			t.Fatal(e)
		}
		b, e := build(t, root, r, p, Output{})
		if e != nil {
			t.Fatal(e)
		}
		if !bytes.Equal(a.ResultBytes, b.ResultBytes) || !bytes.Equal(a.Corpus, b.Corpus) || !bytes.Equal(a.Index, b.Index) {
			t.Fatal("same inputs differ")
		}
		if a.Result.Consumer.Envelope == nil || a.Result.Consumer.Envelope.SourceValidation != "index-digest-validated; source-revalidation-unavailable" {
			t.Fatal("missing actual indexed consumer limits")
		}
		if a.Result.Provenance.Binding.DocumentationRevision == a.Result.Provenance.Binding.SourceRevision {
			t.Fatal("documentation event collapsed into source")
		}
	})
}
func TestApprovalRefusals(t *testing.T) {
	t.Run("RCP-V0-001 RCP-V0-002 closed and separately approved pins", func(t *testing.T) {
		root, r, p := fixture(t)
		cases := map[string]func(*Request, *Policy){"pending": func(r *Request, p *Policy) { p.Decision = "PENDING" }, "revoked": func(r *Request, p *Policy) { p.Decision = "REVOKED" }, "missing": func(r *Request, p *Policy) { p.Decision = "" }, "wrong docs": func(r *Request, p *Policy) {
			r.Binding.DocumentationRevision = r.Binding.SourceRevision
			p.Binding = r.Binding
		}, "wrong source": func(r *Request, p *Policy) {
			r.Binding.SourceRevision = r.Binding.DocumentationRevision
			p.Binding = r.Binding
		}, "manifest": func(r *Request, p *Policy) { r.Binding.ManifestSHA256 = strings.Repeat("0", 64); p.Binding = r.Binding }, "input": func(r *Request, p *Policy) {
			r.Manifest.Inputs[0].SHA256 = strings.Repeat("0", 64)
			raw, _ := doccorpus.Encode(r.Manifest)
			r.Binding.ManifestSHA256 = doccorpus.Digest(raw)
			p.Binding = r.Binding
		}}
		rr, _ := Encode(r)
		pp, _ := Encode(p)
		for name, edit := range cases {
			t.Run(name, func(t *testing.T) {
				var req Request
				var policy Policy
				Decode(rr, &req)
				Decode(pp, &policy)
				edit(&req, &policy)
				if _, e := build(t, root, req, policy, Output{}); e == nil {
					t.Fatal("invalid approval admitted")
				}
			})
		}
		unknown := append([]byte(`{"approval":true,`), rr[1:]...)
		if _, e := Build(context.Background(), root, unknown, pp, doccorpus.Digest(pp), nil, nil, nil); e == nil {
			t.Fatal("author approval field admitted")
		}
		duplicate := append([]byte(`{"profile":"ignored",`), rr[1:]...)
		if _, e := Build(context.Background(), root, duplicate, pp, doccorpus.Digest(pp), nil, nil, nil); e == nil {
			t.Fatal("duplicate member admitted")
		}
		if _, e := Build(context.Background(), root, rr, pp, strings.Repeat("0", 64), nil, nil, nil); e == nil {
			t.Fatal("untrusted policy digest admitted")
		}
	})
}
func TestRepublishIncrementalAndParity(t *testing.T) {
	t.Run("RCP-V0-005 RCP-V0-006 RCP-V0-007 changed shard reused source parity and dropped shard", func(t *testing.T) {
		root, r, p := fixture(t)
		ids := []doccorpus.ShardIdentity{{Provider: "docs", Path: "docs/a.json"}, {Provider: "docs", Path: "docs/b.json"}}
		r.Binding.ReuseEligibility = ids
		p.Binding = r.Binding
		prior, e := build(t, root, r, p, Output{})
		if e != nil {
			t.Fatal(e)
		}
		raw, _ := os.ReadFile(filepath.Join(root, "docs/b.json"))
		var part doccorpus.ProviderRecord
		if e := Decode(raw, &part); e != nil {
			t.Fatal(e)
		}
		part.Subjects[0].Name = "Documentation B revised"
		raw, _ = doccorpus.Encode(part)
		write(t, root, "docs/b.json", raw)
		git(t, root, "add", "docs/b.json")
		git(t, root, "commit", "-qm", "second approved docs candidate")
		repin(t, root, &r, &p)
		p.PreviousResultSHA256 = doccorpus.Digest(prior.ResultBytes)
		p.PreviousArtifactSHA256 = doccorpus.Digest(prior.Corpus)
		p.PreviousSidecarProfile = doccorpus.IncrementalSchema
		p.PreviousSidecarSHA256 = doccorpus.Digest(prior.Sidecar)
		next, e := build(t, root, r, p, prior)
		if e != nil {
			t.Fatal(e)
		}
		if len(next.Stats.Reused) != 1 || len(next.Stats.Compiled) != 1 || next.Stats.Reused[0] != ids[0] || len(next.Result.Parity.Subjects.Changed) != 1 {
			t.Fatalf("incremental/parity %+v %+v", next.Stats, next.Result.Parity)
		}
		coldR, coldP := r, p
		coldR.Binding.ReuseEligibility = []doccorpus.ShardIdentity{}
		coldP.Binding = coldR.Binding
		coldP.PreviousSidecarProfile = "NONE"
		coldP.PreviousSidecarSHA256 = "NONE"
		coldPrior := prior
		coldPrior.Sidecar = nil
		cold, e := build(t, root, coldR, coldP, coldPrior)
		if e != nil {
			t.Fatal(e)
		}
		if !bytes.Equal(next.Corpus, cold.Corpus) || !bytes.Equal(next.Index, cold.Index) {
			t.Fatal("warm/cold complete bytes differ")
		}
		for _, name := range []string{"eligibility", "profile", "digest", "bytes"} {
			t.Run(name, func(t *testing.T) {
				rr, _ := Encode(r)
				pp, _ := Encode(p)
				var x Request
				var y Policy
				Decode(rr, &x)
				Decode(pp, &y)
				z := prior
				switch name {
				case "eligibility":
					x.Binding.ReuseEligibility = ids[:1]
					y.Binding = x.Binding
				case "profile":
					y.PreviousSidecarProfile = "unknown"
				case "digest":
					y.PreviousSidecarSHA256 = strings.Repeat("0", 64)
				case "bytes":
					z.Sidecar = append([]byte{}, prior.Sidecar...)
					z.Sidecar[0] = '['
				}
				if _, e := build(t, root, x, y, z); e == nil {
					t.Fatal("altered sidecar admitted")
				}
			})
		}
		// Omit a whole shard from both declarations and inputs. Existing compiler
		// joins can also refuse missing records; parity independently sees its identity.
		a, _ := doccorpus.ParseArtifact(prior.Corpus)
		b, _ := doccorpus.ParseArtifact(next.Corpus)
		b.Manifest.Providers[1].Shards = b.Manifest.Providers[1].Shards[:1]
		if _, e := compare(a, b, []doccorpus.ShardIdentity{}); e == nil {
			t.Fatal("omitted shard not caught")
		}
		parity, e := compare(a, b, ids[1:])
		if e != nil || len(parity.Shards.Retired) != 1 || len(parity.RetiredShards) != 1 {
			t.Fatal("explicit retirement hid inventory", e)
		}
		// Changed second shard removes the first shard's cross-shard join target.
		part.Subjects = nil
		part.Capabilities = nil
		raw, _ = doccorpus.Encode(part)
		write(t, root, "docs/b.json", raw)
		git(t, root, "add", "docs/b.json")
		git(t, root, "commit", "-qm", "broken global join")
		repin(t, root, &r, &p)
		if _, e := build(t, root, r, p, prior); e == nil {
			t.Fatal("reuse skipped global joins")
		}
	})
}

func TestWholeOmittedEmptyShard(t *testing.T) {
	t.Run("RCP-V0-005 RCP-V0-006 empty shard omitted from declarations and inputs refuses real replacement build", func(t *testing.T) {
		root, r, p := fixture(t)
		for _, path := range []string{"docs/a.json", "docs/b.json"} {
			raw, _ := os.ReadFile(filepath.Join(root, path))
			var part doccorpus.ProviderRecord
			if e := Decode(raw, &part); e != nil {
				t.Fatal(e)
			}
			part.Relations = nil
			if path == "docs/b.json" {
				part.Subjects = nil
			}
			raw, _ = doccorpus.Encode(part)
			write(t, root, path, raw)
		}
		git(t, root, "add", "docs")
		git(t, root, "commit", "-qm", "explicit empty shard")
		repin(t, root, &r, &p)
		prior, e := build(t, root, r, p, Output{})
		if e != nil {
			t.Fatal(e)
		}
		r.Manifest.Providers[1].Shards = []string{"docs/a.json"}
		inputs := []doccorpus.Input{}
		for _, in := range r.Manifest.Inputs {
			if in.Path != "docs/b.json" {
				inputs = append(inputs, in)
			}
		}
		r.Manifest.Inputs = inputs
		r.Manifest.Scopes[1].Path = "docs/a.json"
		raw, _ := doccorpus.Encode(r.Manifest)
		r.Binding.ManifestSHA256 = doccorpus.Digest(raw)
		p.Binding = r.Binding
		p.PreviousResultSHA256 = doccorpus.Digest(prior.ResultBytes)
		p.PreviousArtifactSHA256 = doccorpus.Digest(prior.Corpus)
		if _, e := build(t, root, r, p, prior); e == nil || !strings.Contains(e.Error(), "dropped shard") {
			t.Fatalf("whole empty shard disappeared: %v", e)
		}
		p.AllowedShardRetirements = []doccorpus.ShardIdentity{{Provider: "docs", Path: "docs/b.json"}}
		next, e := build(t, root, r, p, prior)
		if e != nil {
			t.Fatal(e)
		}
		if len(next.Result.Parity.Shards.Retired) != 1 || len(next.Result.Parity.Subjects.Retired) != 0 {
			t.Fatal("empty shard inventory/counts hidden")
		}
		prior.ResultBytes[0] = '['
		if _, e := build(t, root, r, p, prior); e == nil {
			t.Fatal("corrupt previous result admitted")
		}
	})
}
