//go:build darwin || linux

package intent_test

import (
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/Beamfall/corvint/internal/tasks/fixture"
	"github.com/Beamfall/corvint/internal/tasks/intent"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// loadTreeRoot writes queue.json, policy.json and n fixture tickets under a
// fresh primary worktree and returns its path.
func loadTreeRoot(tb testing.TB, n int) string {
	tb.Helper()
	root := tb.TempDir()
	if real, err := filepath.EvalSymlinks(root); err == nil {
		root = real
	}
	write := func(p string, raw []byte) {
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			tb.Fatal(err)
		}
		if err := os.WriteFile(p, raw, 0o644); err != nil {
			tb.Fatal(err)
		}
	}
	dir := filepath.Join(root, intent.Dir)
	write(filepath.Join(dir, intent.QueueFile), fixture.QueueBytes())
	write(filepath.Join(dir, intent.PolicyFile), fixture.PolicyBytes())
	for i := 0; i < n; i++ {
		rec := fixture.Ticket(fmt.Sprintf("T%04d", i))
		write(filepath.Join(dir, intent.TicketsDir, rec.TicketID.Local+".json"), rec.Encode())
	}
	return root
}

// TM-V0-008: LoadTree over a captured tree decodes the same store as
// LoadExpecting reading it again, refuses a pin it does not match as
// SNAPSHOT_MOVED, and still re-hashes every captured file before decoding.
func TestTMV0008_LoadTreeParity(t *testing.T) {
	root := loadTreeRoot(t, 12)
	tree, err := intent.TreeDigest(root)
	if err != nil {
		t.Fatal(err)
	}
	want, err := intent.LoadExpecting(root, tree.Sha256)
	if err != nil {
		t.Fatal(err)
	}
	got, err := intent.LoadTree(root, tree, tree.Sha256)
	if err != nil || !reflect.DeepEqual(got, want) {
		t.Fatalf("LoadTree differs: %v", err)
	}
	other := wire.Sum([]byte("other"))
	_, a := intent.LoadExpecting(root, other)
	_, b := intent.LoadTree(root, tree, other)
	if a == nil || b == nil || a.Error() != b.Error() || wire.CodeOf(b) != wire.CodeSnapshotMoved {
		t.Fatalf("pin refusal differs: %v / %v", a, b)
	}
	tampered := tree
	tampered.Files = append([]intent.File(nil), tree.Files...)
	last := len(tampered.Files) - 1
	tampered.Files[last].Raw = append(append([]byte(nil), tampered.Files[last].Raw...), '\n')
	if _, err := intent.LoadTree(root, tampered, tree.Sha256); wire.CodeOf(err) != wire.CodeSnapshotMoved {
		t.Fatalf("tampered capture decoded: %v", err)
	}
}

// TM-V0-008: TreeDigest reading tickets/ and releases/ records beneath one
// pinned descriptor per subdirectory captures the same tree, and reports the
// same refusal, as opening each record's whole path from the store root.
func TestTMV0008_PinnedTreeDigestParity(t *testing.T) {
	for _, kind := range []string{"valid", "empty", "release", "unreadable-ticket", "stray-entry"} {
		t.Run(kind, func(t *testing.T) {
			n := 9
			if kind == "empty" {
				n = 0
			}
			root := loadTreeRoot(t, n)
			dir := filepath.Join(root, intent.Dir)
			switch kind {
			case "empty":
				if err := os.MkdirAll(filepath.Join(dir, intent.TicketsDir), 0o755); err != nil {
					t.Fatal(err)
				}
			case "release":
				if err := os.MkdirAll(filepath.Join(dir, intent.ReleasesDir), 0o755); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(filepath.Join(dir, intent.ReleasesDir, "r1.json"), []byte("{}\n"), 0o644); err != nil {
					t.Fatal(err)
				}
			case "unreadable-ticket":
				if os.Geteuid() == 0 {
					t.Skip("root reads a mode-0 file")
				}
				p := filepath.Join(dir, intent.TicketsDir, fixture.Ticket("T0003").TicketID.Local+".json")
				if err := os.Chmod(p, 0); err != nil {
					t.Fatal(err)
				}
				t.Cleanup(func() { os.Chmod(p, 0o644) })
			case "stray-entry":
				if err := os.WriteFile(filepath.Join(dir, intent.TicketsDir, "stray"), []byte("x"), 0o644); err != nil {
					t.Fatal(err)
				}
			}
			restore := intent.SetPinTreeDirsForTest(false)
			want, wantErr := intent.TreeDigest(root)
			restore()
			got, gotErr := intent.TreeDigest(root)
			if fmt.Sprint(gotErr) != fmt.Sprint(wantErr) || !reflect.DeepEqual(got, want) {
				t.Fatalf("pinned %v / per-file %v", gotErr, wantErr)
			}
			if (kind == "unreadable-ticket" || kind == "stray-entry") != (gotErr != nil) {
				t.Fatalf("%s: %v", kind, gotErr)
			}
			t.Logf("%s: %d files, %v", kind, len(got.Files), gotErr)
		})
	}
}

// BenchmarkTMV0008_ReadIntentTree is a read's intent-tree work around its
// body: two probe digests plus the body's load, before (a third pass through
// LoadExpecting) and after (decoding the first probe's tree).
func BenchmarkTMV0008_ReadIntentTree(b *testing.B) {
	root := loadTreeRoot(b, 880)
	for _, probed := range []bool{false, true} {
		b.Run(map[bool]string{false: "third-pass", true: "probed-tree"}[probed], func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				tree, err := intent.TreeDigest(root)
				if err != nil {
					b.Fatal(err)
				}
				if probed {
					_, err = intent.LoadTree(root, tree, tree.Sha256)
				} else {
					_, err = intent.LoadExpecting(root, tree.Sha256)
				}
				if err != nil {
					b.Fatal(err)
				}
				if _, err := intent.TreeDigest(root); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

// BenchmarkTMV0008_TreeDigest is one probe's tree capture over 880 tickets,
// opening each record's whole path from the store root against one pinned
// descriptor per subdirectory.
func BenchmarkTMV0008_TreeDigest(b *testing.B) {
	root := loadTreeRoot(b, 880)
	for _, pinned := range []bool{false, true} {
		b.Run(map[bool]string{false: "per-file", true: "pinned-dir"}[pinned], func(b *testing.B) {
			defer intent.SetPinTreeDirsForTest(pinned)()
			for i := 0; i < b.N; i++ {
				if _, err := intent.TreeDigest(root); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}
