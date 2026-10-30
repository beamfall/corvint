package witness

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/contextindex"
)

// TestCompileAtAHistoricalHeadMatchesTheCheckedOutReport pins AGW-V0-007: a
// report compiled over a commit-pinned index after HEAD moved on and the
// worktree went dirty equals the report compiled while that commit was the
// checked-out HEAD, except for the COMMIT-PINNED head binding and the
// admission packet, which sheds the status digest it no longer binds.
func TestCompileAtAHistoricalHeadMatchesTheCheckedOutReport(t *testing.T) {
	index, base := packetFixture(t, "example.test/fixture")
	checkedOut, err := Compile(context.Background(), index, Options{Base: base})
	if err != nil {
		t.Fatal(err)
	}
	if checkedOut.Range.HeadBinding != "" {
		t.Fatalf("a checked-out head carries head binding %q", checkedOut.Range.HeadBinding)
	}
	head, root := index.CommitRevision, index.Root
	git, err := exec.LookPath("git")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "README.md"), []byte("later\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	command := exec.Command(git, "-c", "user.name=Fixture", "-c", "user.email=fixture@example.test", "commit", "--quiet", "-am", "later")
	command.Dir, command.Env = root, gitEnvironment()
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("git commit: %v: %s", err, output)
	}
	if err := os.WriteFile(filepath.Join(root, "a/a.go"), []byte("package a\n\nfunc dirty() {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	pinnedIndex, err := contextindex.BuildRevisionContext(context.Background(), root, head)
	if err != nil {
		t.Fatal(err)
	}
	pinned, err := Compile(context.Background(), pinnedIndex, Options{Base: base, HeadPinned: true})
	if err != nil {
		t.Fatal(err)
	}
	if pinned.Range.HeadBinding != "COMMIT-PINNED" || pinned.Range.Head != head || pinned.Range.Admission != "COMPUTED" {
		t.Fatalf("pinned range = %+v", pinned.Range)
	}
	if !strings.Contains(Render(pinned), "head="+head+" head-binding=COMMIT-PINNED\n") {
		t.Fatalf("the rendered report does not name the commit-pinned head\n%s", Render(pinned))
	}
	got, want := pinned.PacketCoverage[0].PacketCounts, checkedOut.PacketCoverage[0].PacketCounts
	if got == nil || want == nil || got.PacketBytes >= want.PacketBytes {
		t.Fatalf("admission packet %+v must shed the status digest of %+v", got, want)
	}
	got.PacketBytes = want.PacketBytes
	pinned.Range.HeadBinding = ""
	left, _ := json.Marshal(checkedOut)
	right, _ := json.Marshal(pinned)
	if string(left) != string(right) {
		t.Fatalf("historical report differs from the checked-out report:\n%s\n%s", left, right)
	}

	// Without HeadPinned the worktree-bound admission refuses the same index.
	unpinned, err := Compile(context.Background(), pinnedIndex, Options{Base: base})
	if err != nil {
		t.Fatal(err)
	}
	if unpinned.Range.Admission != "REFUSED" {
		t.Fatalf("worktree-bound admission over a historical index = %s", unpinned.Range.Admission)
	}
}
