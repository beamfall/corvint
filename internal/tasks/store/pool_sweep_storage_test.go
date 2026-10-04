//go:build darwin || linux

package store_test

import (
	"archive/tar"
	"bytes"
	"errors"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/Beamfall/corvint/internal/tasks/archive"
	"github.com/Beamfall/corvint/internal/tasks/snapshot"
	"github.com/Beamfall/corvint/internal/tasks/store"
	"github.com/Beamfall/corvint/internal/tasks/transaction"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

func TestPSRNativeStorageAndArchive(t *testing.T) {
	t.Run("private-exact-export", func(t *testing.T) {
		s, _ := psrFixture(t, "printf reset; printf private-log >&2", "printf verified", "verified", "1", false)
		out, e := psrRun(s, "archive")
		if e != nil || out.Pending || len(psrPools(t, s).Entries) != 0 {
			t.Fatal(out, e)
		}
		observations := psrObservations(t, s)
		if len(observations) != 2 {
			t.Fatal("missing durable phase observations", len(observations))
		}
		expected := map[string][]byte{}
		for _, o := range observations {
			for _, digest := range []wire.Digest{wire.Sum(o.Encode()), o.Log} {
				path := "evidence/" + string(digest)
				raw, e := os.ReadFile(filepath.Join(s.repo.StateDir, path))
				if e != nil || wire.Sum(raw) != digest {
					t.Fatal(path, e)
				}
				info, e := os.Stat(filepath.Join(s.repo.StateDir, path))
				if e != nil || info.Mode().Perm() != 0600 {
					t.Fatal("not private", path, info, e)
				}
				expected[path] = raw
			}
		}
		var stream bytes.Buffer
		if _, e = archive.Export(archive.ExportOptions{Repo: s.repo, Staging: t.TempDir(), Stdout: &stream}); e != nil {
			t.Fatal(e)
		}
		if _, e = archive.Verify(bytes.NewReader(stream.Bytes())); e != nil {
			t.Fatal(e)
		}
		tr := tar.NewReader(bytes.NewReader(stream.Bytes()))
		found := 0
		for {
			h, e := tr.Next()
			if e == io.EOF {
				break
			}
			if e != nil {
				t.Fatal(e)
			}
			raw, e := io.ReadAll(tr)
			if e != nil {
				t.Fatal(e)
			}
			if want, ok := expected[h.Name]; ok {
				// Explicit export preserves its canonical format; native0600 was checked above.
				if h.Mode != 0644 || !bytes.Equal(raw, want) {
					t.Fatal("archive changed evidence", h.Name, h.Mode)
				}
				found++
			}
		}
		if found != len(expected) {
			t.Fatalf("archive evidence coverage %d/%d", found, len(expected))
		}
		before := storeDigest(t, s.repo)
		replay, e := psrRun(s, "archive")
		if e != nil || replay.Evidence != out.Evidence || before != storeDigest(t, s.repo) {
			t.Fatal("replay changed storage", replay, e)
		}
		auditOK(t, s.repo)
	})
	for _, phase := range []string{"before-receipt", "after-receipt"} {
		t.Run(phase, func(t *testing.T) {
			marker := filepath.Join(t.TempDir(), "runs")
			verify := marker + ".verify"
			s, _ := psrFixture(t, "printf x >> "+marker, "printf reached > "+verify+"; printf verified", "verified", "1", false)
			if len(psrPools(t, s).Entries) != 1 {
				t.Fatal("fixture missing quarantine")
			}
			fired := false
			injected := errors.New("injected sweep observation publication")
			restore := store.SetPublishFaultForTest(func(a transaction.Artifact) error {
				if _, e := os.Stat(marker); e != nil {
					return nil
				}
				target := a.Role == "RECEIPT"
				if phase == "after-receipt" {
					target = a.Role == "POST" && a.Target == "pools.json"
				}
				if target && !fired {
					fired = true
					return injected
				}
				return nil
			})
			_, e := psrRun(s, "interrupted")
			restore()
			if !fired || !errors.Is(e, injected) {
				t.Fatal("publication fault not reached", fired, e)
			}
			if raw, _ := os.ReadFile(marker); string(raw) != "x" {
				t.Fatal("reset not reached exactly once", string(raw))
			}
			if _, e = os.Stat(verify); !os.IsNotExist(e) {
				t.Fatal("verify ran without durable reset", e)
			}
			if p := psrPools(t, s); len(p.Entries) != 1 || p.Entries[0].Sweep == nil || p.Entries[0].State != "CLEANING" {
				t.Fatal("publication failure cleared owner", p)
			}
			if _, e = snapshot.Probe(s.repo.StateDir); phase == "after-receipt" && wire.CodeOf(e) != wire.CodeRedoPending {
				t.Fatal("committed interruption not fenced", e)
			}
			replay, e := psrRun(s, "interrupted")
			if e != nil || !replay.Pending {
				t.Fatal("original request lost pending identity", replay, e)
			}
			if raw, _ := os.ReadFile(marker); string(raw) != "x" {
				t.Fatal("pending replay reran reset", string(raw))
			}
			if p := psrPools(t, s); len(p.Entries) != 1 || p.Entries[0].Sweep == nil {
				t.Fatal("redo freed member", p)
			}
			auditOK(t, s.repo)
			// A redo publishes only the previously frozen observation; execution remains pending.
			if phase == "after-receipt" && len(psrObservations(t, s)) != 1 {
				t.Fatal("redo did not retain reset evidence")
			}
		})
	}
}
