package journal

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/tasks/fixture"
	"github.com/Beamfall/corvint/internal/tasks/intent"
)

// intentReads counts the intent files an audit reads through its source.
type intentReads struct {
	Native
	n int
}

func (s *intentReads) Read(p string, max int) ([]byte, error) {
	if strings.HasPrefix(p, "intent/") {
		s.n++
	}
	return s.Native.Read(p, max)
}

// CAL-V0-140: an audit given the tree its caller's outer snapshot hashed takes
// the intent bytes from it instead of reading and hashing every file twice
// more, with the same result. A file whose size differs is read afresh, so
// the identity moves; a same-size rewrite keeps the shared digest, which is
// why only a caller that hashes the tree again after the audit may share it.
func TestCALV0140_AuditSharesTheOuterIntentTree(t *testing.T) {
	repo, r := setup(t)
	fixture.CommitPosts(t, repo, "MUTATION", "", map[string][]byte{"intent/tickets/A.json": fixture.Ticket("A").Encode()})
	tree, err := intent.TreeDigest(repo.Root)
	if err != nil {
		t.Fatal(err)
	}
	src := &intentReads{Native: r.Source.(Native)}
	r.Source = src
	plain, err := r.Audit()
	if err != nil {
		t.Fatal(err)
	}
	if src.n == 0 {
		t.Fatal("the plain audit read no intent file")
	}
	src.n = 0
	r.IntentTree = &tree
	shared, err := r.Audit()
	if err != nil {
		t.Fatal(err)
	}
	if src.n != 0 {
		t.Fatalf("the shared audit read %d intent files", src.n)
	}
	if shared.Identity != plain.Identity || shared.Identity.IntentTreeSha256 != tree.Sha256 {
		t.Fatalf("shared identity %+v, plain %+v", shared.Identity, plain.Identity)
	}

	path := filepath.Join(repo.IntentDir, intent.TicketsDir, "A.json")
	raw := read(t, path)
	grown := append(append([]byte{}, raw[:len(raw)-1]...), ' ', '\n')
	if err := os.WriteFile(path, grown, 0o644); err != nil {
		t.Fatal(err)
	}
	moved, err := r.Audit()
	if err == nil && moved.Identity.IntentTreeSha256 == tree.Sha256 {
		t.Fatal("a resized intent file kept the shared digest")
	}
	same := append([]byte{}, raw...)
	same[len(same)-2] = ' '
	if err := os.WriteFile(path, same, 0o644); err != nil {
		t.Fatal(err)
	}
	stale, err := r.Audit()
	if err != nil {
		t.Fatal(err)
	}
	fresh, err := intent.TreeDigest(repo.Root)
	if err != nil {
		t.Fatal(err)
	}
	if stale.Identity.IntentTreeSha256 != tree.Sha256 || fresh.Sha256 == tree.Sha256 {
		t.Fatal("a same-size rewrite must keep the shared digest and move the outer tree")
	}
}
