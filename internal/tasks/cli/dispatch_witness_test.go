//go:build darwin || linux

package cli

import (
	"io"
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/Beamfall/corvint/internal/tasks/dispatch"
	"github.com/Beamfall/corvint/internal/tasks/intent"
)

// CAL-V0-139: the dispatch queue's witness is stable on an unchanged store
// and changes with the journal head and with any intent tree entry.
func TestCALV0139_DispatchQueueWitness(t *testing.T) {
	repo, root := psrCLIReady(t, "true")
	q := dispatchQueue{env: Env{Cwd: root, Stdout: io.Discard, Stderr: io.Discard}}
	w := func() string {
		t.Helper()
		v, err := q.Witness()
		if err != nil {
			t.Fatal(err)
		}
		return v
	}
	w1 := w()
	if w2 := w(); w2 != w1 {
		t.Fatal("the witness of an unchanged store changed")
	}
	if _, err := (dispatchQueue{env: Env{Cwd: t.TempDir()}}).Witness(); err == nil {
		t.Fatal("a directory without a store gave a witness")
	}

	tickets := filepath.Join(repo.IntentRoot(), intent.Dir, intent.TicketsDir)
	if err := os.WriteFile(filepath.Join(tickets, "NEW.json"), []byte("{}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	w2 := w()
	if w2 == w1 {
		t.Fatal("a new ticket file did not change the witness")
	}
	if err := os.WriteFile(filepath.Join(repo.IntentRoot(), intent.Dir, "stray.json"), []byte("{}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	w3 := w()
	if w3 == w2 {
		t.Fatal("a new top-level intent entry did not change the witness")
	}

	head := filepath.Join(repo.StateDir, "head.json")
	raw, err := os.ReadFile(head)
	if err != nil {
		t.Fatal(err)
	}
	fi, err := os.Stat(head)
	if err != nil {
		t.Fatal(err)
	}
	// Same length, same mtime, different bytes: only the content hash sees it.
	alt := slices.Clone(raw)
	alt[len(alt)-2] ^= 1
	if err := os.WriteFile(head, alt, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(head, fi.ModTime(), fi.ModTime()); err != nil {
		t.Fatal(err)
	}
	if w() == w3 {
		t.Fatal("a changed journal head did not change the witness")
	}
}

type admitAll struct{}

func (admitAll) Admit(string) (func(bool), error) { return func(bool) {}, nil }
func (admitAll) Boundary(bool, bool) bool         { return false }

// CAL-V0-139: the service-run dispatcher reads the same pools as the
// foreground dispatch command; before, it was opened without them.
func TestCALV0139_ServiceDispatcherReadsTicketPools(t *testing.T) {
	_, root := psrCLIReady(t, "true")
	c := psrNativeConfig(root, filepath.Join(t.TempDir(), "dispatch"), "true")
	c.Roles[0].Match = &dispatch.Match{Pool: "db"}
	ctl, err := serviceOpen(Env{Cwd: t.TempDir(), Stdout: io.Discard, Stderr: io.Discard})("svc", c, admitAll{}, io.Discard)
	if err != nil {
		t.Fatal(err)
	}
	d := ctl.(*dispatch.Dispatcher)
	defer d.Close()
	q, ok := d.Queue.(dispatchQueue)
	if !ok {
		t.Fatalf("queue %T", d.Queue)
	}
	if !slices.Equal(q.pools, []string{"db"}) || q.env.Cwd != root || q.reviews == nil {
		t.Fatalf("service queue pools %v cwd %q reviews %v", q.pools, q.env.Cwd, q.reviews != nil)
	}
}
