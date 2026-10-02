package appflows

import (
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
