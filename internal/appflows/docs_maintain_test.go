package appflows

import (
	"bytes"
	"context"
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/doccorpus"
)

func oversizedMaintenanceRepo(t *testing.T) (string, DocMaintenanceOptions) {
	t.Helper()
	root := intentRepo(t)
	for n := 0; n < 4; n++ {
		id := fmt.Sprintf("large%d", n)
		flow := sampleIntent(id)
		flow.Steps[0].Action = strings.Repeat("a", 4096)
		flow.Steps[1].Action = strings.Repeat("a", 4096)
		for i := 2; i < 256; i++ {
			flow.Steps = append(flow.Steps, FlowStep{StepID: fmt.Sprintf("s%d", i), Action: strings.Repeat("a", 4096)})
		}
		writeIntent(t, root, "flows/"+id+".json", flow)
	}
	commitAll(t, root, "large admitted intents")
	return root, DocMaintenanceOptions{Flows: "flows", Docs: DocOptions{Page: "generated.md", Claims: "generated.json"}}
}

func TestFlowDocMaintenanceOversizedProposal(t *testing.T) {
	root, o := oversizedMaintenanceRepo(t)
	p, err := PreviewDocMaintenance(context.Background(), root, o)
	if err == nil || p.Digest != "" {
		t.Errorf("oversized preview returned digest %q with error %v (page bytes %d)", p.Digest, err, len(p.Page))
	}
	receipt, err := ApplyDocMaintenance(context.Background(), root, o, Digest(nil))
	if err == nil || len(receipt) != 0 {
		t.Errorf("empty hash authorized oversized apply: %s %v", receipt, err)
	}
	for _, name := range []string{o.Docs.Page, o.Docs.Claims} {
		if _, e := os.Lstat(filepath.Join(root, name)); !os.IsNotExist(e) {
			t.Errorf("oversized apply wrote %s", name)
		}
	}
}

func TestFlowDocMaintenanceAbsentIgnoredInput(t *testing.T) {
	for _, kind := range []string{"file", "directory", "symlink", "symlink-parent"} {
		t.Run(kind, func(t *testing.T) {
			root := intentRepo(t)
			flow := sampleIntent("checkout")
			flow.Links = append(flow.Links, FlowLink{From: "pay", Basis: "declared", Target: LinkTarget{Type: "source", Path: "ignored/target"}})
			writeIntent(t, root, "flows/checkout.json", flow)
			writeRaw(t, root, ".gitignore", []byte("ignored\n"))
			commitAll(t, root, "absent ignored link target")
			o := DocMaintenanceOptions{Flows: "flows", Docs: DocOptions{Page: "generated.md", Claims: "generated.json"}}
			p, err := PreviewDocMaintenance(context.Background(), root, o)
			if err != nil {
				t.Fatal(err)
			}
			if kind == "symlink-parent" {
				if err := os.Symlink("absent-parent", filepath.Join(root, "ignored")); err != nil {
					t.Fatal(err)
				}
			} else {
				if err := os.Mkdir(filepath.Join(root, "ignored"), 0700); err != nil {
					t.Fatal(err)
				}
				switch kind {
				case "file":
					writeRaw(t, root, "ignored/target", []byte("uncommitted input"))
				case "directory":
					if err := os.Mkdir(filepath.Join(root, "ignored/target"), 0700); err != nil {
						t.Fatal(err)
					}
				case "symlink":
					if err := os.Symlink("../app.js", filepath.Join(root, "ignored/target")); err != nil {
						t.Fatal(err)
					}
				}
			}
			if state := gitOut(t, root, "status", "--porcelain"); state != "" {
				t.Fatalf("fixture is not ignored: %s", state)
			}
			receipt, err := ApplyDocMaintenance(context.Background(), root, o, p.Digest)
			if err == nil || len(receipt) != 0 {
				t.Errorf("ignored input accepted: %s %v", receipt, err)
			}
			for _, name := range []string{o.Docs.Page, o.Docs.Claims} {
				if _, e := os.Lstat(filepath.Join(root, name)); !os.IsNotExist(e) {
					t.Errorf("changed ignored input wrote %s", name)
				}
			}
		})
	}
}

func TestFlowDocMaintenanceEncodingBounds(t *testing.T) {
	if digest, err := maintenanceDigest(strings.Repeat("a", doccorpus.MaxBytes)); err == nil || digest != "" {
		t.Fatalf("oversized encoding became a digest %q %v", digest, err)
	}

	root := t.TempDir()
	parent := strings.TrimSuffix(strings.Repeat(strings.Repeat("d", 200)+"/", 4), "/")
	if err := os.MkdirAll(filepath.Join(root, parent), 0700); err != nil {
		t.Fatal(err)
	}
	files := [2]doccorpus.MaintenanceFile{{Path: parent + "/a", Before: []byte("old-a"), Next: []byte("new-a")}, {Path: "b", Before: []byte("old-b"), Next: []byte("new-b")}}
	for _, f := range files {
		writeRaw(t, root, f.Path, f.Before)
	}
	before := treeSnapshot(t, root)
	// The source metadata fits by itself; the exact planned publication paths
	// and recovery names push the eventual receipt beyond the shared bound.
	r := DocMaintenanceReceipt{Sources: []DocIdentity{{Path: "a", SHA256: strings.Repeat("a", doccorpus.MaxBytes-1000)}}}
	if _, err := encodeMaintenanceReceipt(r); err != nil {
		t.Fatalf("baseline receipt should fit: %v", err)
	}
	calls := 0
	_, err := doccorpus.PublishMaintenancePair(context.Background(), root, files, func(publications []doccorpus.MaintenancePublication) error {
		calls++
		r.Publications = publications
		_, e := encodeMaintenanceReceipt(r)
		return e
	})
	if err == nil || calls != 1 {
		t.Fatalf("late receipt refusal: calls=%d error=%v", calls, err)
	}
	after := treeSnapshot(t, root)
	if !maps.Equal(before, after) {
		t.Fatal("receipt preflight changed the filesystem")
	}

}

func TestFlowDocMaintenanceReceiptRoundTrip(t *testing.T) {
	root := intentRepo(t)
	o := DocMaintenanceOptions{Flows: "flows", Docs: DocOptions{Page: "generated.md", Claims: "generated.json"}}
	proposal, err := PreviewDocMaintenance(context.Background(), root, o)
	if err != nil {
		t.Fatal(err)
	}
	receipt, err := ApplyDocMaintenance(context.Background(), root, o, proposal.Digest)
	if err != nil {
		t.Fatal(err)
	}
	var decoded DocMaintenanceReceipt
	if err = Decode(receipt, &decoded); err != nil {
		t.Fatal(err)
	}
	writeRaw(t, root, "receipt.json", receipt)
	commitAll(t, root, "maintenance outputs and receipt")
	o.Receipt = "receipt.json"
	repeated, err := PreviewDocMaintenance(context.Background(), root, o)
	if err != nil || !repeated.Noop {
		t.Fatalf("receipt/output cannot be read back: %+v %v", repeated, err)
	}
	again, err := ApplyDocMaintenance(context.Background(), root, o, repeated.Digest)
	if err != nil || string(again) != string(receipt) {
		t.Fatalf("repeat changed: %v", err)
	}
}

// FDM-V0-003, FDM-V0-005: reseal each altered field so the digest mismatch
// cannot mask its own integrity guard. Every case starts with a usable receipt.
func TestFlowDocMaintenanceReceiptIntegrity(t *testing.T) {
	for _, kind := range []string{"schema", "profile", "digest-format", "digest-mismatch", "canonical-bytes", "revision", "tree", "proposal", "ownership", "historical-provenance", "recorded", "unknown"} {
		t.Run(kind, func(t *testing.T) {
			root := intentRepo(t)
			o := DocMaintenanceOptions{Flows: "flows", Docs: DocOptions{Page: "generated.md", Claims: "generated.json"}}
			proposal, err := PreviewDocMaintenance(context.Background(), root, o)
			if err != nil {
				t.Fatal(err)
			}
			raw, err := ApplyDocMaintenance(context.Background(), root, o, proposal.Digest)
			if err != nil {
				t.Fatal(err)
			}
			var receipt DocMaintenanceReceipt
			if err := Decode(raw, &receipt); err != nil {
				t.Fatal(err)
			}
			canonical, err := doccorpus.Encode(receipt)
			if err != nil || !bytes.Equal(raw, canonical) {
				t.Fatalf("fixture is not canonical: %v", err)
			}
			writeRaw(t, root, "receipt.json", raw)
			commitAll(t, root, "valid maintenance receipt")
			o.Receipt = "receipt.json"
			if p, err := PreviewDocMaintenance(context.Background(), root, o); err != nil || !p.Noop {
				t.Fatalf("baseline receipt rejected: %+v %v", p, err)
			}
			switch kind {
			case "schema":
				receipt.Schema = "flow-doc-maintenance/invalid"
			case "profile":
				receipt.Profile = "invalid-template/0"
			case "revision":
				receipt.Revision = "invalid-revision"
			case "tree":
				receipt.Tree = "invalid-tree"
			case "proposal":
				receipt.Proposal = "invalid-proposal"
			case "ownership":
				receipt.Ownership = "authenticated"
			case "historical-provenance":
				receipt.HistoricalProvenance = "PROVEN"
			case "recorded":
				receipt.HistoricalProvenance = "RECORDED"
			case "unknown":
				receipt.HistoricalProvenance = "UNKNOWN"
			}
			raw, err = encodeMaintenanceReceipt(receipt)
			if err != nil {
				t.Fatalf("altered fixture cannot encode: %v", err)
			}
			if err := Decode(raw, &receipt); err != nil {
				t.Fatal(err)
			}
			switch kind {
			case "digest-format", "digest-mismatch":
				if kind == "digest-format" {
					receipt.Digest = "invalid-digest"
				} else {
					// Keep valid SHA-256 syntax and guarantee a different digest.
					replacement := "0"
					if receipt.Digest[0] == '0' {
						replacement = "1"
					}
					receipt.Digest = replacement + receipt.Digest[1:]
				}
				raw, err = doccorpus.Encode(receipt)
				if err != nil {
					t.Fatal(err)
				}
			case "canonical-bytes":
				raw = append(raw, '\n')
			}
			if err := Decode(raw, new(DocMaintenanceReceipt)); err != nil {
				t.Fatalf("fixture fails before integrity validation: %v", err)
			}
			writeRaw(t, root, o.Receipt, raw)
			if !bytes.Equal(raw, canonical) {
				commitAll(t, root, "receipt integrity fixture "+kind)
			}
			before := treeSnapshot(t, root)
			p, err := PreviewDocMaintenance(context.Background(), root, o)
			valid := kind == "recorded" || kind == "unknown"
			if valid {
				if err != nil || !p.Noop {
					t.Fatalf("valid provenance rejected: %+v %v", p, err)
				}
			} else if err == nil || !strings.Contains(err.Error(), "invalid maintenance receipt integrity or profile") {
				t.Fatalf("integrity guard not reached: %+v %v", p, err)
			}
			if valid {
				result, err := ApplyDocMaintenance(context.Background(), root, o, p.Digest)
				if err != nil || !bytes.Equal(result, raw) {
					t.Fatalf("valid receipt changed: %v", err)
				}
			} else {
				// Use a syntactically valid proposal to reach receipt validation in apply.
				result, err := ApplyDocMaintenance(context.Background(), root, o, proposal.Digest)
				if err == nil || len(result) != 0 || !strings.Contains(err.Error(), "invalid maintenance receipt integrity or profile") {
					t.Fatalf("invalid receipt authorized apply: %s %v", result, err)
				}
			}
			if !maps.Equal(before, treeSnapshot(t, root)) {
				t.Fatal("receipt validation changed the filesystem")
			}
		})
	}
}

// TestFlowDocMaintenanceDerivationRaces interleaves a competing change between
// derivation steps through the package seams and requires the named recheck,
// not the proposal digest, to refuse (FDM-V0-002).
func TestFlowDocMaintenanceDerivationRaces(t *testing.T) {
	for _, tc := range []struct{ kind, reason string }{
		{"evidence-changed-during-read", "maintenance evidence changed during read"},
		{"head-moved-during-derivation", "maintenance HEAD moved during derivation"},
		{"inputs-changed-during-staging", "maintenance inputs changed during staging"},
	} {
		t.Run(tc.kind, func(t *testing.T) {
			root := intentRepo(t)
			evidence := filepath.Join(t.TempDir(), "run.jsonl")
			writeRaw(t, filepath.Dir(evidence), filepath.Base(evidence), nil)
			o := DocMaintenanceOptions{Flows: "flows", Docs: DocOptions{Page: "generated.md", Claims: "generated.json"}, EvidenceFiles: []string{evidence}}
			p, err := PreviewDocMaintenance(context.Background(), root, o)
			if err != nil {
				t.Fatal(err)
			}
			raced := false
			race := func() {
				if raced {
					return
				}
				raced = true
				if tc.kind == "evidence-changed-during-read" {
					writeRaw(t, filepath.Dir(evidence), filepath.Base(evidence), []byte("\n"))
					return
				}
				writeRaw(t, root, "unrelated.txt", []byte(tc.kind))
				commitAll(t, root, "competing HEAD movement")
			}
			if tc.kind == "inputs-changed-during-staging" {
				t.Cleanup(func() { publishMaintenancePair = doccorpus.PublishMaintenancePair })
				publishMaintenancePair = func(ctx context.Context, root string, files [2]doccorpus.MaintenanceFile, verify func([]doccorpus.MaintenancePublication) error) ([]doccorpus.MaintenancePublication, error) {
					return doccorpus.PublishMaintenancePair(ctx, root, files, func(planned []doccorpus.MaintenancePublication) error {
						race()
						return verify(planned)
					})
				}
			} else {
				t.Cleanup(func() { readRunEvidence = ReadRunEvidence })
				readRunEvidence = func(names []string) ([]TestRunEvidence, error) {
					records, err := ReadRunEvidence(names)
					race()
					return records, err
				}
			}
			receipt, err := ApplyDocMaintenance(context.Background(), root, o, p.Digest)
			if !raced || err == nil || len(receipt) != 0 || !strings.Contains(err.Error(), tc.reason) {
				t.Fatalf("raced=%v receipt=%s error=%v, want %q", raced, receipt, err, tc.reason)
			}
			for _, name := range []string{o.Docs.Page, o.Docs.Claims} {
				if _, e := os.Lstat(filepath.Join(root, name)); !os.IsNotExist(e) {
					t.Errorf("refused apply wrote %s", name)
				}
			}
		})
	}
}
