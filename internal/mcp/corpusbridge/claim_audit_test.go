package corpusbridge

import (
	"context"
	json "encoding/json/v2"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/doccorpus"
)

// DCP-V1-263: the MCP get tool shows a claim's current audit verdict beside it and leaves the
// claim and the receipt trust label unchanged.
func TestDCPV1263MCPGetShowsClaimAudit(t *testing.T) {
	root, _ := lazyFixture(t)
	git := func(args ...string) string {
		t.Helper()
		c := exec.Command("git", args...)
		c.Dir = root
		b, err := c.CombinedOutput()
		if err != nil {
			t.Fatalf("git: %v %s", err, b)
		}
		return strings.TrimSpace(string(b))
	}
	ctx := context.Background()
	m, err := doccorpus.InventoryScopes(ctx, root, git("rev-parse", "HEAD"), []string{"docs"}, doccorpus.ManifestSchemaV2, "2026-09-19T00:00:00Z")
	if err != nil {
		t.Fatal(err)
	}
	base, err := doccorpus.Build(ctx, root, m)
	if err != nil {
		t.Fatal(err)
	}
	const claimID = "native:file:docs/p1.md:paragraph:3"
	var claim doccorpus.Claim
	for _, c := range base.Claims {
		if c.ID == claimID {
			claim = c
		}
	}
	if claim.ID == "" {
		t.Fatalf("fixture has no claim %s", claimID)
	}
	record := doccorpus.ProviderRecord{Schema: doccorpus.ClaimAuditSchema, ID: "audit", Version: "1", Source: m.Repository, ClaimAudits: &doccorpus.ClaimAuditSet{Records: []doccorpus.ClaimAuditRecord{{
		ID: "audit:p1", ClaimID: claimID, SpanSHA256: doccorpus.ClaimAuditSpan(claim), Verdict: "incorrect", Reviewer: "reviewer-b", Independent: true, Method: "human", Evidence: []doccorpus.Anchor{},
	}}}}
	data, err := doccorpus.Encode(record)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "audit.json"), data, 0o600); err != nil {
		t.Fatal(err)
	}
	git("add", "audit.json")
	git("-c", "user.name=test", "-c", "user.email=test@invalid", "commit", "-qm", "audit")
	rev := git("rev-parse", "HEAD")
	m.Providers = append(m.Providers, doccorpus.Provider{ID: "audit", Kind: "records", Version: "1", Revision: rev, Record: "audit.json"})
	m.Scopes = append(m.Scopes, doccorpus.Scope{Path: "audit.json", Revision: rev})
	m.Inputs = append(m.Inputs, doccorpus.Input{Path: "audit.json", Revision: rev, Blob: git("rev-parse", rev+":audit.json"), SHA256: doccorpus.Digest(data), Provider: "audit", Purpose: "provider"})
	a, err := doccorpus.Build(ctx, root, m)
	if err != nil {
		t.Fatal(err)
	}
	corpus, err := doccorpus.Encode(a)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "corpus.json"), corpus, 0o600); err != nil {
		t.Fatal(err)
	}
	registry, rerr := New(root, "corpus.json")
	if rerr != nil {
		t.Fatal(rerr)
	}
	_, text, failure, protocol, _ := registry.CallTimed(ctx, "corvint.docs_get", []byte(`{"id":"`+claimID+`"}`))
	if failure != nil || protocol != nil {
		t.Fatalf("get failed: %v %v", failure, protocol)
	}
	if !strings.Contains(text, `"claim_audits":{"`+claimID+`":[{"id":"audit:p1","status":"current","verdict":"incorrect","method":"human"`) {
		t.Fatalf("MCP get does not show the verdict:\n%s", text)
	}
	quoted, _ := json.Marshal(claim.Text)
	if !strings.Contains(text, `"trust":"`+claim.Evidence.Trust+`"`) || !strings.Contains(text, `"text":`+string(quoted)) {
		t.Fatalf("MCP get changed the claim:\n%s", text)
	}
}
