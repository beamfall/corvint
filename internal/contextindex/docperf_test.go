// SPDX-License-Identifier: Apache-2.0

package contextindex

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"testing"
)

// IDX-SNAP-V0-7804: compare the actual checkout's packets through the gob and
// non-gob pack readers. A scratch store isolates this opt-in experiment from
// the repository's normal snapshot store.
func TestDocperfPackSelfCorpusParity(t *testing.T) {
	root := os.Getenv("CORVINT_DOCPERF_ROOT")
	if root == "" {
		t.Skip("set CORVINT_DOCPERF_ROOT to the corpus checkout")
	}
	t.Setenv("CORVINT_INDEX_STORE", t.TempDir())
	t.Setenv(snapshotFormatEnv, packFormatValue)
	ctx := context.Background()
	built, err := Build(ctx, root)
	if err != nil {
		t.Fatal(err)
	}
	receipt, err := WriteSnapshot(built)
	if err != nil {
		t.Fatal(err)
	}
	identity := repositoryIdentity{objectFormat: built.ObjectFormat, commitRevision: built.CommitRevision, treeRevision: built.Revision}
	file, err := os.Open(receipt.Path)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	gob, err := decodeSnapshotLoad(file, identity, receipt.Engine, true)
	if err != nil {
		t.Fatal(err)
	}
	defer gob.Release()
	pack, err := readPackSnapshot(receipt.PackPath, identity, analyzerEngine(), loadContext)
	if err != nil {
		t.Fatal(err)
	}
	defer pack.Release()
	for _, index := range []*Index{gob, pack} {
		index.Root, index.DirtyPaths, index.StatusSHA256 = built.Root, built.DirtyPaths, built.StatusSHA256
	}
	for _, task := range []string{"snapshot decode", "documentation corpus verified read", "checkout stamp freshness", "document reference imports", "task evidence requirement", "test selection affected"} {
		var outputs [][]byte
		for _, index := range []*Index{gob, pack} {
			packet, err := TaskContext(ctx, index, task, "", 20)
			if err != nil {
				t.Fatal(err)
			}
			data, err := json.Marshal(packet)
			if err != nil {
				t.Fatal(err)
			}
			outputs = append(outputs, data)
			if err := index.SnapshotRefusal(); err != nil {
				t.Fatal(err)
			}
		}
		if !bytes.Equal(outputs[0], outputs[1]) {
			t.Fatalf("gob/pack packet differs for %q", task)
		}
	}
	t.Logf("IDX-SNAP-V0-7804 sources=%d symbols=%d gob_bytes=%d six packets byte-identical", len(built.Sources), len(built.Symbols), receipt.Bytes)
}
