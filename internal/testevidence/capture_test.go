// SPDX-License-Identifier: Apache-2.0
package testevidence

import (
	"path/filepath"
	"strings"
	"testing"
)

// RSP-V0-007, RSP-V0-010: replacing target contents cannot erase a link's
// identity. Retargeting the retained lexical dependency must invalidate evidence.
func TestRSPV0007WriteProofRetainsLinkRetargeting(t *testing.T) {
	for _, kind := range []string{"exclusive-create", "lookup-create", "rename-replacement"} {
		t.Run(kind, func(t *testing.T) {
			root := t.TempDir()
			reads := Component[Blob]{State: Present}
			var counts OutsideCounts
			CaptureRead(&reads, &counts, []Repository{{ID: "repo", Root: root}},
				filepath.Join(root, "alias"), filepath.Join(root, "target"),
				blob().OID, strings.Repeat("4", 40), Other)
			if reads.State != Present || len(reads.Items) != 1 {
				t.Fatalf("capture failed: %+v", reads)
			}
			read := reads.Items[0]
			proof := WriteProof{
				Write:    Write{Path: Path{"repo", "target", "target"}, Kind: Create},
				SameTree: true, BeforeRead: true, Succeeded: true,
			}
			switch kind {
			case "exclusive-create":
				proof.ExclusiveCreate = true
			case "lookup-create":
				proof.OwnFailedLookup = true
			case "rename-replacement":
				proof.Write.Kind = Replace
				proof.RenamedTreeCreatedFile = true
			}
			if DropRead(read, proof) {
				t.Fatal("content proof discarded the lexical link dependency")
			}
			r := fixture()
			r.Reads = reads
			snapshot := Snapshot{Blobs: map[PathKey]string{
				{"repo", "target"}: read.OID,
				{"repo", "alias"}:  read.LexicalOID,
			}}
			if got := r.Fresh(r.Runner, r.Execution, snapshot); !got.Fresh {
				t.Fatalf("unchanged link is stale: %+v", got)
			}
			// Only the link changes; the exclusively created/replaced target
			// retains its bytes. The old record must no longer be usable.
			snapshot.Blobs[PathKey{"repo", "alias"}] = strings.Repeat("5", 40)
			if got := r.Fresh(r.Runner, r.Execution, snapshot); got.Fresh || got.Reason != "link-drift-or-missing" {
				t.Fatalf("retargeted link accepted: %+v", got)
			}
		})
	}
}

// RSP-V0-007: even every proof flag cannot authorize an unknown write form.
// Exercise a direct path so the unknown-kind default itself must retain it.
func TestRSPV0007UnknownWriteProofRetainsRead(t *testing.T) {
	read := blob()
	proof := WriteProof{
		Write:    Write{Path: read.Path, Kind: "future-write-kind"},
		SameTree: true, BeforeRead: true, Succeeded: true,
		ExclusiveCreate: true, OwnFailedLookup: true, RenamedTreeCreatedFile: true,
	}
	if DropRead(read, proof) {
		t.Fatal("unknown write form discarded a read dependency")
	}
}
