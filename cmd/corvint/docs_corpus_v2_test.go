package main

import (
	"context"
	"encoding/json"
	"path/filepath"
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
		Registry: doccorpus.BehaviorRegistry{Schema: 2, ContractID: "example", SourceRevision: rev, DocumentationRevision: rev, Repositories: []doccorpus.BehaviorRepository{{RootCommit: m.Repository.ID, Revision: rev}}, Discovery: a, Manifest: a, Flows: []doccorpus.BehaviorFlow{}, Tests: []doccorpus.BehaviorTest{}, Behaviors: []doccorpus.BehaviorSource{}}}
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

func TestDocsCorpusTypedCLIInvocation(t *testing.T) {
	t.Run("DCP-V1-038 typed argument parity", func(t *testing.T) {
		root, e := filepath.Abs("../..")
		if e != nil {
			t.Fatal(e)
		}
		for _, op := range []string{"concept", "claims", "flow", "dependencies", "recommend-tests", "navigation", "vocabulary", "intent"} {
			kind := doccorpus.OperationInput(op)
			args := []string{"--root", root, "docs", "corpus", op, "--artifact", "corpus.json", "--" + kind, "value"}
			o, handled, e := parseCorpusInvocation(args)
			if e != nil || !handled || o.op != op {
				t.Fatal(op, e)
			}
			args = append(args, "--retirement", "policy.json")
			if _, _, e := parseCorpusInvocation(args); e != nil {
				t.Fatal("retirement argument rejected", e)
			}
		}
	})
}
