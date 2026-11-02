// SPDX-License-Identifier: Apache-2.0

package intent_test

import (
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strconv"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/tasks/fixture"
	"github.com/Beamfall/corvint/internal/tasks/intent"
	"github.com/Beamfall/corvint/internal/tasks/ticket"
)

// parallelStore writes a store of n open tickets T0..T(n-1) under root.
func parallelStore(t *testing.T, n int) string {
	t.Helper()
	root := t.TempDir()
	dir := filepath.Join(root, intent.Dir)
	fixture.Write(t, filepath.Join(dir, intent.QueueFile), fixture.QueueBytes())
	fixture.Write(t, filepath.Join(dir, intent.PolicyFile), fixture.PolicyBytes())
	for i := 0; i < n; i++ {
		rec := fixture.Ticket("T" + strconv.Itoa(i))
		if i%50 == 7 {
			rec.KnowHow = []ticket.KnowHowEntry{{Seq: "1", Operation: ticket.KnowHowAdd, Text: "note " + strconv.Itoa(i),
				Anchors: []ticket.KnowHowAnchor{{Path: "src/a.go", Blob: strings.Repeat("a", 40)}}, Routes: []string{},
				Commit: strings.Repeat("c", 40), ActorID: "russell", ActorRole: "OWNER", RecordedAt: "2026-09-06T13:00:00Z"}}
		}
		fixture.Write(t, filepath.Join(dir, intent.TicketsDir, rec.TicketID.Local+".json"), rec.Encode())
	}
	return root
}

// TCX-V0-974: the tree capture reads and stats concurrently and gives the
// same tree, byte for byte, as the serial walk; a refused record is refused
// with the same error on both.
func TestTCXV0974_ParallelCaptureMatchesSerial(t *testing.T) {
	root := parallelStore(t, 300)
	parallel, err := intent.TreeDigest(root)
	if err != nil {
		t.Fatal(err)
	}
	serial, serialErr := func() (intent.Tree, error) {
		old := runtime.GOMAXPROCS(1)
		defer runtime.GOMAXPROCS(old)
		return intent.TreeDigest(root)
	}()
	if serialErr != nil {
		t.Fatal(serialErr)
	}
	if !reflect.DeepEqual(parallel, serial) {
		t.Fatalf("parallel capture differs from the serial one: %s vs %s", parallel.Sha256, serial.Sha256)
	}
	if len(parallel.Files) != 302 || parallel.Files[2].Raw == nil {
		t.Fatalf("files %d, raw %v", len(parallel.Files), parallel.Files[2].Raw != nil)
	}
	// The first refusal in listing order is the same under both walks.
	bad := filepath.Join(root, intent.Dir, intent.TicketsDir, "T150.json")
	if err := os.WriteFile(bad, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	_, perr := intent.TreeDigest(root)
	_, serr := func() (intent.Tree, error) {
		old := runtime.GOMAXPROCS(1)
		defer runtime.GOMAXPROCS(old)
		return intent.TreeDigest(root)
	}()
	if perr == nil || serr == nil || perr.Error() != serr.Error() {
		t.Fatalf("refusals differ: parallel %v, serial %v", perr, serr)
	}
}

// TCX-V0-975: LoadSummarizedTree over a captured tree is LoadSummarized.
func TestTCXV0975_LoadSummarizedTreeMatchesLoadSummarized(t *testing.T) {
	root := parallelStore(t, 120)
	tree, err := intent.TreeDigest(root)
	if err != nil {
		t.Fatal(err)
	}
	fromTree, err := intent.LoadSummarizedTree(root, tree, nil)
	if err != nil {
		t.Fatal(err)
	}
	direct, err := intent.LoadSummarized(root, nil)
	if err != nil {
		t.Fatal(err)
	}
	if fromTree.Inventory.Len() != direct.Inventory.Len() || !reflect.DeepEqual(fromTree.Tree, direct.Tree) || !reflect.DeepEqual(fromTree.Digests, direct.Digests) {
		t.Fatalf("loads differ: %d vs %d records", fromTree.Inventory.Len(), direct.Inventory.Len())
	}
	for _, id := range direct.Inventory.IDs() {
		a, _ := fromTree.Inventory.Get(id)
		b, _ := direct.Inventory.Get(id)
		if !reflect.DeepEqual(a, b) {
			t.Fatalf("%s differs", id)
		}
	}
}
