package journal

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/tasks/fixture"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// requireRefusal asserts both the code and the path a refusal names.
func requireRefusal(t *testing.T, err error, code, where string) {
	t.Helper()
	e, ok := err.(*wire.Error)
	if !ok || e.Code != code || e.Where != where {
		t.Fatalf("got %v; want %s at %s", err, code, where)
	}
}

// CAL-V0-114: with several divergent projections the audit names the
// byte-smallest divergent path, and that path's code, on every run. Each case
// is repeated so map-order iteration fails it with near certainty.
func TestCALV0114_DivergentProjectionRefusalIsDeterministic(t *testing.T) {
	cases := map[string]struct {
		edit       func(t *testing.T, repo *fixture.Repo)
		code, path string
	}{
		"intent and state": {code: wire.CodeIntentDiverged, path: ticketPath("A"), edit: func(t *testing.T, repo *fixture.Repo) {
			fixture.Write(t, filepath.Join(repo.IntentDir, "tickets", "B.json"), fixture.Ticket("Y").Encode())
			fixture.Write(t, filepath.Join(repo.IntentDir, "tickets", "A.json"), fixture.Ticket("Z").Encode())
			fixture.Write(t, filepath.Join(repo.StateDir, "reservations.json"), []byte("{\"other\":\"1\"}\n"))
		}},
		"state only": {code: wire.CodeJournalForked, path: "VERSION", edit: func(t *testing.T, repo *fixture.Repo) {
			fixture.Write(t, filepath.Join(repo.StateDir, "reservations.json"), []byte("{\"other\":\"1\"}\n"))
			fixture.Write(t, filepath.Join(repo.StateDir, "VERSION"), []byte("other\n"))
		}},
		"strays": {code: wire.CodeIntentDiverged, path: ticketPath("C"), edit: func(t *testing.T, repo *fixture.Repo) {
			fixture.Write(t, filepath.Join(repo.IntentDir, "tickets", "D.json"), fixture.Ticket("D").Encode())
			fixture.Write(t, filepath.Join(repo.IntentDir, "tickets", "C.json"), fixture.Ticket("C").Encode())
		}},
		"unassigned stage slots": {code: wire.CodeMalformed, path: "staging/a01", edit: func(t *testing.T, repo *fixture.Repo) {
			for _, s := range []string{"a09", "a01", "a05", "a07", "a03"} {
				fixture.Write(t, filepath.Join(repo.StateDir, "staging", s), []byte("x"))
			}
		}},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			repo, r := setup(t)
			stubStageSleep(t, nil) // orphan slots exhaust the CTS-V0-008 wait
			appendReceipt(t, repo, "MUTATION", map[string][]byte{
				ticketPath("A"): fixture.Ticket("A").Encode(),
				ticketPath("B"): fixture.Ticket("B").Encode(),
			}, "", true, true, false)
			tc.edit(t, repo)
			for i := 0; i < 32; i++ {
				_, err := r.Audit()
				requireRefusal(t, err, tc.code, tc.path)
			}
		})
	}
}

// CAL-V0-114 leaves a single divergence and the success path unchanged.
func TestCALV0114_SingleDivergenceAndSuccessUnchanged(t *testing.T) {
	repo, r := setup(t)
	appendReceipt(t, repo, "MUTATION", map[string][]byte{
		ticketPath("A"): fixture.Ticket("A").Encode(),
	}, "", true, true, false)
	if _, err := r.Audit(ticketPath("A")); err != nil {
		t.Fatal(err)
	}
	fixture.Write(t, filepath.Join(repo.StateDir, "reservations.json"), []byte("{\"other\":\"1\"}\n"))
	_, err := r.Audit()
	requireRefusal(t, err, wire.CodeJournalForked, "reservations.json")
}

// CAL-V0-114: several close failures of the native read source join in path
// order, so the diagnostic text is the same on every run.
func TestCALV0114_CloseFailuresJoinInPathOrder(t *testing.T) {
	old := closeReadFile
	t.Cleanup(func() { closeReadFile = old })
	closeReadFile = func(f *os.File) error {
		if err := f.Close(); err != nil {
			return err
		}
		return errors.New(filepath.Base(f.Name()))
	}
	names := []string{"e", "b", "d", "a", "c"}
	for i := 0; i < 32; i++ {
		root := t.TempDir()
		s := newNativeRead(Native{})
		for _, n := range names {
			if err := os.Mkdir(filepath.Join(root, n), 0o700); err != nil {
				t.Fatal(err)
			}
			d, err := os.Open(filepath.Join(root, n))
			if err != nil {
				t.Fatal(err)
			}
			s.dirs[n] = d
		}
		if got := s.close(); got == nil || got.Error() != strings.Join([]string{"a", "b", "c", "d", "e"}, "\n") {
			t.Fatalf("got %q; want path order", got)
		}
	}
}
