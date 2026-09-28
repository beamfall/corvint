package main

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/doccorpus"
)

func TestBehaviorProviderV2CLI(t *testing.T) {
	// AFU-V1-006: the public command emits /2 without mutating its inputs.
	root := taskContextRepository(t)
	rev := cemGit(t, root, "rev-parse", "HEAD")
	m, err := doccorpus.Inventory(context.Background(), root, rev, "cache", "2026-09-28T00:00:00Z")
	if err != nil {
		t.Fatal(err)
	}
	a := doccorpus.Anchor{Repository: m.Repository.ID, Revision: rev, Path: m.Inputs[0].Path, Blob: m.Inputs[0].Blob, SHA256: m.Inputs[0].SHA256, Start: 1, End: 1, SpanSHA256: strings.Repeat("a", 64), Authority: "external-provider", Kind: "declared", Reason: "producer declaration; consumer must verify bytes"}
	request := doccorpus.BehaviorProviderRequestV2{Schema: doccorpus.BehaviorProviderRequestSchemaV2, ID: "behavior", Version: "2", Source: m.Repository,
		Registry: doccorpus.BehaviorRegistry{Schema: 2, ContractID: "example", SourceRevision: rev, DocumentationRevision: rev, Repositories: []doccorpus.BehaviorRepository{{m.Repository.ID, rev}}, Discovery: a, Manifest: a, Flows: []doccorpus.BehaviorFlow{}, Tests: []doccorpus.BehaviorTest{}, Behaviors: []doccorpus.BehaviorSource{}}}
	raw, err := doccorpus.Encode(request)
	if err != nil {
		t.Fatal(err)
	}
	cemWrite(t, root, "request-v2.json", string(raw))
	before := treeDigest(t, root)
	code, out, stderr := corpusCLI(t, root, "docs", "corpus", "behavior-provider", "--input", "request-v2.json")
	if code != 0 {
		t.Fatalf("%d %s", code, stderr)
	}
	var p doccorpus.ProviderRecord
	if err := json.Unmarshal([]byte(out), &p); err != nil {
		t.Fatal(err)
	}
	if err := doccorpus.ValidateBehaviorProviderV2(p); err != nil {
		t.Fatal(err)
	}
	_, again, _ := corpusCLI(t, root, "docs", "corpus", "behavior-provider", "--input", "request-v2.json")
	if out != again || treeDigest(t, root) != before {
		t.Fatal("nondeterminism or mutation")
	}
	code, _, _ = corpusCLI(t, root, "docs", "corpus", "behavior-provider", "--input", "request-v2.json", "--previous", "request-v2.json")
	if code != 2 {
		t.Fatal("/2 silently admitted /1 adapter options")
	}
}
