package main

import (
	"context"
	"encoding/json"
	"slices"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/doccorpus"
	"github.com/Beamfall/corvint/internal/mcp/corpusbridge"
)

// ownerCorpusCLIFixture builds a corpus over proof.go and owner.go. The flow
// proof:owner, anchored at owner.go, documents proof:impl, anchored at
// proof.go, and asserts proof:owner-test through a TestLink; proof:near,
// also at owner.go, is only related_to proof:impl. GitHub #2012.
func ownerCorpusCLIFixture(t *testing.T) string {
	t.Helper()
	root := taskContextRepository(t)
	cemWrite(t, root, "proof.go", "package cache\nfunc Checkout() {}\n")
	cemWrite(t, root, "owner.go", "package cache\nfunc Owner() {}\n")
	cemGit(t, root, "add", "proof.go", "owner.go")
	cemGit(t, root, "commit", "-qm", "owner source")
	rev := cemGit(t, root, "rev-parse", "HEAD")
	m, err := doccorpus.InventoryScopes(context.Background(), root, rev, []string{"owner.go", "proof.go"}, doccorpus.ManifestSchemaV2, "2026-09-30T00:00:00Z")
	if err != nil {
		t.Fatal(err)
	}
	evidence := func(path, line, symbol string) doccorpus.Evidence {
		for _, in := range m.Inputs {
			if in.Path == path {
				return doccorpus.Evidence{Derivation: "declared", Trust: "generated", State: "supported", Freshness: "fresh", Anchors: []doccorpus.Anchor{{Repository: m.Repository.ID, Revision: rev, Path: in.Path, Blob: in.Blob, SHA256: in.SHA256, Start: 2, End: 2, SpanSHA256: doccorpus.Digest([]byte(line)), Symbol: symbol, Authority: "external-provider", Kind: "declared", Reason: "bounded synthetic typed declaration"}}, Limitations: []string{"provider declarations are not authenticated observations"}}
			}
		}
		t.Fatalf("no input %s", path)
		return doccorpus.Evidence{}
	}
	atProof, atOwner := evidence("proof.go", "func Checkout() {}\n", "Checkout"), evidence("owner.go", "func Owner() {}\n", "Owner")
	sections := []doccorpus.FlowSection{}
	for _, name := range []string{"readme", "entry-points", "flow-diagram", "functional-overview", "technical-deep-dive", "entities", "exceptions", "related-flows"} {
		sections = append(sections, doccorpus.FlowSection{Name: name, Paragraphs: []string{}})
	}
	flow := doccorpus.RecordDetails{Flow: &doccorpus.FlowDetails{Sections: sections, Actors: []string{}, Entrypoints: []string{}, Variations: []string{}}}
	p := doccorpus.ProviderRecord{Schema: doccorpus.AdoptionProviderSchema, ID: "proof", Version: "1", Source: m.Repository,
		Subjects: []doccorpus.Subject{
			{ID: "proof:impl", Kind: "ui_surface", Name: "Checkout", Provider: "proof", Evidence: atProof},
			{ID: "proof:owner", Kind: "flow", Name: "Owner flow", Provider: "proof", Evidence: atOwner},
			{ID: "proof:near", Kind: "flow", Name: "Neighbour flow", Provider: "proof", Evidence: atOwner},
			{ID: "proof:owner-test", Kind: "test", Name: "TestOwner", Provider: "proof", Evidence: atOwner},
		},
		Claims: []doccorpus.Claim{{ID: "proof:claim", Subject: "proof:impl", Text: "Checkout is declared", Provider: "proof", Evidence: atProof}},
		Relations: []doccorpus.Relation{
			{ID: "proof:owns", From: "proof:owner", To: "proof:impl", Type: "documents", Provider: "proof", Evidence: atOwner},
			{ID: "proof:owner-asserts", From: "proof:owner", To: "proof:owner-test", Type: "asserts", Provider: "proof", Evidence: atOwner},
			{ID: "proof:near-rel", From: "proof:near", To: "proof:impl", Type: "related_to", Provider: "proof", Evidence: atOwner},
		},
		Journeys:     []doccorpus.Journey{{ID: "proof:journey", Subject: "proof:owner", Provider: "proof", Status: "non_ui", Preconditions: []string{}, Cleanup: "not-run", Steps: []doccorpus.Step{}, Evidence: atOwner}},
		Observations: []doccorpus.ObservationLink{},
		Details: map[string]doccorpus.RecordDetails{
			"proof:claim": {ClaimKind: "behavior"}, "proof:owner": flow, "proof:near": flow,
			"proof:owner-asserts": {TestLink: &doccorpus.TestLinkDetails{Role: "asserting", JoinConfidence: "exact", File: "owner.go", Title: "TestOwner", Project: "declared"}},
		},
		Capabilities: []doccorpus.CapabilityDeclaration{{Name: "subjects", State: "present", Reason: "declared inventory"}, {Name: "claims", State: "present", Reason: "declared claims"}, {Name: "relations", State: "present", Reason: "declared relations"}, {Name: "journeys", State: "present", Reason: "explicit non-UI disposition"}},
	}
	raw, err := doccorpus.Encode(p)
	if err != nil {
		t.Fatal(err)
	}
	cemWrite(t, root, "provider.json", string(raw))
	cemGit(t, root, "add", "provider.json")
	cemGit(t, root, "commit", "-qm", "owner provider")
	pr := cemGit(t, root, "rev-parse", "HEAD")
	m.Providers = append(m.Providers, doccorpus.Provider{ID: "proof", Kind: "records", Version: "1", Revision: pr, Record: "provider.json"})
	m.Scopes = append(m.Scopes, doccorpus.Scope{Path: "provider.json", Revision: pr})
	m.Inputs = append(m.Inputs, doccorpus.Input{Path: "provider.json", Revision: pr, Blob: cemGit(t, root, "rev-parse", pr+":provider.json"), SHA256: doccorpus.Digest(raw), Provider: "proof", Purpose: "provider"})
	a, err := doccorpus.Build(context.Background(), root, m)
	if err != nil {
		t.Fatal(err)
	}
	raw, err = doccorpus.Encode(a)
	if err != nil {
		t.Fatal(err)
	}
	cemWrite(t, root, "corpus.json", string(raw))
	return root
}

// corpusParity runs one corpus operation through the CLI and the MCP tool and
// requires byte-identical receipts; it returns the receipt text.
func corpusParity(t *testing.T, root string, registry *corpusbridge.Registry, tool, arguments string, cli ...string) string {
	t.Helper()
	code, out, stderr := corpusCLI(t, root, append([]string{"docs", "corpus"}, append(cli, "--artifact", "corpus.json")...)...)
	if code != 0 {
		t.Fatalf("%v: exit %d: %s", cli, code, stderr)
	}
	_, text, failure, protocol := registry.Call(context.Background(), tool, []byte(arguments))
	if failure != nil || protocol != nil || text != out {
		t.Fatalf("%s %s: CLI/MCP parity lost: %v %v\ncli %s\nmcp %s", tool, arguments, failure, protocol, out, text)
	}
	return out
}

func idsOf(t *testing.T, out string) []string {
	t.Helper()
	ids := strings.Trim(receiptIDs(t, out), "[]")
	return strings.Fields(ids)
}

// DCS-V1-594, DCS-V1-595 (GitHub #2012): corpus locate and recommend-tests on
// a path return the subject documenting a subject anchored there, with its
// documents relation, through the CLI and the MCP tool alike; recommend-tests
// also returns that owner's TestLink relation. A related_to neighbour is no
// owner, and a batch keeps one receipt per path.
func TestDCSV1594CorpusPathOperationsReturnDocumentingOwners(t *testing.T) {
	root := ownerCorpusCLIFixture(t)
	registry, err := corpusbridge.New(root, "corpus.json")
	if err != nil {
		t.Fatal(err)
	}
	locate := idsOf(t, corpusParity(t, root, registry, "corvint.docs_locate", `{"path":"proof.go"}`, "locate", "--path", "proof.go"))
	tests := idsOf(t, corpusParity(t, root, registry, "corvint.docs_recommend_tests", `{"path":"proof.go"}`, "recommend-tests", "--path", "proof.go"))
	for name, c := range map[string]struct{ got, want []string }{
		"locate":          {locate, []string{"proof:impl", "proof:owner", "proof:owns"}},
		"recommend-tests": {tests, []string{"proof:impl", "proof:owner", "proof:owns", "proof:owner-asserts"}},
	} {
		for _, id := range c.want {
			if !slices.Contains(c.got, id) {
				t.Fatalf("%s lacks %s: %v", name, id, c.got)
			}
		}
		for _, id := range []string{"proof:near", "proof:near-rel"} {
			if slices.Contains(c.got, id) {
				t.Fatalf("%s treated a related_to neighbour as an owner: %v", name, c.got)
			}
		}
	}
	if slices.Contains(locate, "proof:owner-asserts") {
		t.Fatalf("locate followed the owner's test link: %v", locate)
	}
	// A batch keeps one receipt per path: owner.go has no documented subject.
	result, _, failure, protocol := registry.Call(context.Background(), "corvint.docs_locate", []byte(`{"paths":["proof.go","owner.go"]}`))
	if failure != nil || protocol != nil {
		t.Fatal(failure, protocol)
	}
	receipts, _ := result["receipts"].([]any)
	if len(receipts) != 2 {
		t.Fatalf("batch receipts = %v", result)
	}
	for i, path := range []string{"proof.go", "owner.go"} {
		row, _ := receipts[i].(map[string]any)
		if row["path"] != path {
			t.Fatalf("batch row %d = %v", i, row["path"])
		}
		raw, _ := json.Marshal(row["receipt"])
		ids := idsOf(t, string(raw))
		if owns := slices.Contains(ids, "proof:owns"); owns != (path == "proof.go") {
			t.Fatalf("batch receipt for %q: owner relation %v in %v", path, owns, ids)
		}
	}
}

// DCS-V1-591, DCS-V1-592, DCS-V1-593 (GitHub #2014): a claims query labels
// each claim with its kind, ranks supplied test runs after behavioural
// claims even when the run text repeats the spec name, and takes --kind and
// --exclude-kind (MCP kind and exclude_kind); invalid filters are refused.
func TestDCSV1592CorpusClaimKindsCLIAndMCP(t *testing.T) {
	root, _ := typedCorpusCLIFixtureWith(t, func(p *doccorpus.ProviderRecord, ev doccorpus.Evidence) {
		p.Claims = append(p.Claims,
			doccorpus.Claim{ID: "proof:mailer", Subject: "proof:flow", Text: "the confirmation mailer sends one email", Provider: "proof", Evidence: ev},
			doccorpus.Claim{ID: "proof:run", Subject: "proof:flow", Text: "passed: spec/mailers/booking_confirmation_mailer_spec.rb", Provider: "proof", Evidence: ev},
		)
		p.Details["proof:mailer"] = doccorpus.RecordDetails{ClaimKind: "behavior"}
		p.Details["proof:run"] = doccorpus.RecordDetails{TestRun: &doccorpus.SuppliedTestRunDetails{Outcome: "passed", Run: "run-1", Origin: "supplied"}}
	})
	registry, err := corpusbridge.New(root, "corpus.json")
	if err != nil {
		t.Fatal(err)
	}
	out := corpusParity(t, root, registry, "corvint.docs_claims", `{"query":"booking confirmation mailer"}`, "claims", "--query", "booking confirmation mailer")
	var r struct {
		ClaimKinds map[string]string `json:"claim_kinds"`
	}
	if err := json.Unmarshal([]byte(out), &r); err != nil {
		t.Fatal(err)
	}
	if ids := idsOf(t, out); len(ids) < 2 || ids[0] != "proof:mailer" || ids[len(ids)-1] != "proof:run" {
		t.Fatalf("behavioural claim should rank above the run record: %v", ids)
	}
	if r.ClaimKinds["proof:mailer"] != "behavior" || r.ClaimKinds["proof:run"] != "test_run" {
		t.Fatalf("claim_kinds = %v", r.ClaimKinds)
	}
	if ids := idsOf(t, corpusParity(t, root, registry, "corvint.docs_claims", `{"query":"booking confirmation mailer","exclude_kind":"test_run"}`, "claims", "--query", "booking confirmation mailer", "--exclude-kind", "test_run")); slices.Contains(ids, "proof:run") || !slices.Contains(ids, "proof:mailer") {
		t.Fatalf("exclude_kind kept %v", ids)
	}
	if ids := idsOf(t, corpusParity(t, root, registry, "corvint.docs_claims", `{"path":"proof.go","kind":["test_run"]}`, "claims", "--path", "proof.go", "--kind", "test_run")); !slices.Equal(ids, []string{"proof:run"}) {
		t.Fatalf("kind kept %v", ids)
	}
	for _, c := range []struct{ arguments, flag, value string }{
		{`{"query":"mailer","kind":"run"}`, "--kind", "run"},
		{`{"query":"mailer","kind":"test_run","exclude_kind":"test_run"}`, "--exclude-kind", "test_run"},
		{`{"query":"mailer","exclude_kind":[]}`, "--exclude-kind", "test_run,test_run"},
		// An explicitly given empty filter is refused, not read as no filter.
		{`{"query":"mailer","kind":""}`, "--kind", ""},
		{`{"query":"mailer","exclude_kind":""}`, "--exclude-kind", ""},
		{`{"query":"mailer","kind":[]}`, "--kind", " "},
	} {
		args := []string{"docs", "corpus", "claims", "--artifact", "corpus.json", "--query", "mailer", c.flag + "=" + c.value}
		if strings.Contains(c.arguments, `"kind":"test_run"`) {
			args = append(args, "--kind", "test_run")
		}
		if code, _, _ := corpusCLI(t, root, args...); code != 2 {
			t.Fatalf("%v: exit %d, want 2", args, code)
		}
		if _, _, _, protocol := registry.Call(context.Background(), "corvint.docs_claims", []byte(c.arguments)); protocol == nil || protocol.Code != "invalid-arguments" {
			t.Fatalf("%s: %v", c.arguments, protocol)
		}
	}
	for _, tool := range registry.Tools() {
		properties := tool.InputSchema["properties"].(map[string]any)
		_, kind := properties["kind"]
		_, exclude := properties["exclude_kind"]
		if tool.Name == "corvint.docs_claims" && (!kind || !exclude) || tool.Name != "corvint.docs_claims" && exclude {
			t.Fatalf("%s schema = %v", tool.Name, properties)
		}
	}
}

// DCS-V1-596, DCS-V1-597 (GitHub #2013): corpus navigation takes --query (MCP
// query) instead of --id, selecting the flows whose id, name, or linked test
// file or title hold every query word, with their typed relations and
// TestLink details; id and query together, or neither, are refused.
func TestDCSV1597CorpusNavigationQueryCLIAndMCP(t *testing.T) {
	root, _ := typedCorpusCLIFixture(t)
	registry, err := corpusbridge.New(root, "corpus.json")
	if err != nil {
		t.Fatal(err)
	}
	out := corpusParity(t, root, registry, "corvint.docs_navigation", `{"query":"TestCheckout"}`, "navigation", "--query", "TestCheckout")
	ids := idsOf(t, out)
	for _, id := range []string{"proof:flow", "proof:route", "proof:screen"} {
		if !slices.Contains(ids, id) {
			t.Fatalf("navigation query lacks %s: %v", id, ids)
		}
	}
	var r struct {
		Details map[string]doccorpus.RecordDetails `json:"details"`
	}
	if err := json.Unmarshal([]byte(out), &r); err != nil || r.Details["proof:route"].TestLink == nil {
		t.Fatalf("navigation query lost the TestLink detail: %v %v", err, r.Details)
	}
	if ids := idsOf(t, corpusParity(t, root, registry, "corvint.docs_navigation", `{"query":"zebra"}`, "navigation", "--query", "zebra")); len(ids) != 0 {
		t.Fatalf("an unmatched query selected %v", ids)
	}
	// The id form is unchanged.
	corpusParity(t, root, registry, "corvint.docs_navigation", `{"id":"proof:flow"}`, "navigation", "--id", "proof:flow")
	for _, c := range []struct {
		arguments string
		cli       []string
	}{
		{`{"id":"proof:flow","query":"Checkout"}`, []string{"--id", "proof:flow", "--query", "Checkout"}},
		{`{}`, nil},
		{`{"query":""}`, []string{"--query", ""}},
		// Presence, not value, decides which was given (review repair).
		{`{"id":"","query":"checkout"}`, []string{"--id=", "--query", "checkout"}},
		{`{"id":"proof:flow","query":""}`, []string{"--id", "proof:flow", "--query="}},
		{`{"id":""}`, []string{"--id="}},
	} {
		if code, _, _ := corpusCLI(t, root, append([]string{"docs", "corpus", "navigation", "--artifact", "corpus.json"}, c.cli...)...); code != 2 {
			t.Fatalf("%v: exit %d, want 2", c.cli, code)
		}
		if _, _, _, protocol := registry.Call(context.Background(), "corvint.docs_navigation", []byte(c.arguments)); protocol == nil || protocol.Code != "invalid-arguments" {
			t.Fatalf("%s: %v", c.arguments, protocol)
		}
	}
	for _, tool := range registry.Tools() {
		if tool.Name == "corvint.docs_navigation" {
			properties := tool.InputSchema["properties"].(map[string]any)
			if _, ok := properties["query"]; !ok || slices.Contains(tool.InputSchema["required"].([]any), any("id")) {
				t.Fatalf("docs_navigation schema = %v", tool.InputSchema)
			}
		}
	}
}
