package doccorpus

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestCorpusIndependentFlowAdapter(t *testing.T) {
	t.Run("DCP-V1-006 DCP-V1-019 adapter", func(t *testing.T) {
		root, m := fixture(t)
		ev := declaredEvidence(t, root, m, "src/view.ts", 1)
		input := map[string]any{"provider": "shop", "repository": m.Repository, "screens": []any{map[string]any{"id": "summary", "name": "Order summary", "route": "/summary", "anchors": ev.Anchors}}, "flows": []any{map[string]any{"id": "inspect", "screen": "summary", "anchors": ev.Anchors, "preconditions": []string{"an order exists"}, "cleanup": "not-run", "steps": []any{map[string]any{"id": "open", "action": "navigate", "operation": "/summary", "expected": "Order summary visible", "anchors": ev.Anchors}}}}}
		raw, _ := Encode(input)
		command := exec.CommandContext(context.Background(), "python3", "../../examples/documentation-corpus/flow-provider.py")
		command.Stdin = bytes.NewReader(raw)
		output, err := command.CombinedOutput()
		if err != nil {
			t.Fatalf("independent adapter: %v %s", err, output)
		}
		if err := os.MkdirAll(filepath.Join(root, "evidence"), 0700); err != nil {
			t.Fatal(err)
		}
		p := "evidence/flow.json"
		if err := os.WriteFile(filepath.Join(root, p), output, 0600); err != nil {
			t.Fatal(err)
		}
		git(t, root, "add", p)
		git(t, root, "commit", "-qm", "independent flow provider")
		rev := git(t, root, "rev-parse", "HEAD")
		m.Providers = append(m.Providers, Provider{ID: "shop", Kind: "records", Version: "1", Revision: rev, Record: p})
		m.Scopes = append(m.Scopes, Scope{p, rev})
		m.Inputs = append(m.Inputs, Input{Path: p, Revision: rev, Blob: git(t, root, "rev-parse", rev+":"+p), SHA256: Digest(output), Provider: "shop", Purpose: "provider"})
		a, err := Build(context.Background(), root, m)
		if err != nil {
			t.Fatal(err)
		}
		r, err := Query(a, Request{Operation: "journey", ID: "shop:flow:inspect"}, "fresh", nil)
		if err != nil || len(r.Results) != 1 {
			t.Fatalf("adapter journey missing: %v %+v", err, r)
		}
		journey := r.Results[0].(Journey)
		if journey.Status != "generated_not_verified" || len(journey.Steps) != 1 {
			t.Fatal("authored flow promoted or steps changed")
		}
	})
}
