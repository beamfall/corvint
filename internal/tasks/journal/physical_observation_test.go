//go:build darwin || linux

package journal

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/tasks/fixture"
	"github.com/Beamfall/corvint/internal/tasks/snapshot"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

func TestCALV0026_PhysicalObservationParityAndBounds(t *testing.T) {
	t.Run("actual-native-paths", func(t *testing.T) {
		repo, r := setup(t)
		requestPath, _ := snapshot.RequestPath("physical-request")
		raw := requestBytes("physical-request", 2, wire.Sum([]byte("request")), false)
		appendReceipt(t, repo, "MUTATION", map[string][]byte{requestPath: raw}, "physical-request", true, true, true)
		baseline, err := r.AuditForWrite()
		if err != nil || baseline.Mode != ModeFull || baseline.IntentError != nil {
			t.Fatalf("fixture: %+v %v", baseline, err)
		}
		proof, observed, err := r.AuditForWriteObserved()
		if err != nil || observed.Cleanup != nil || proof.Identity != baseline.Identity || len(observed.Files) == 0 {
			t.Fatalf("observed: %+v %v", observed, err)
		}
		native := r.Source.(Native)
		for path, file := range observed.Files {
			actual, err := native.Read(path, wire.MaxEvidenceBlobBytes)
			if err != nil || file.Bytes != len(actual) || file.Sha256 != wire.Sum(actual) {
				t.Fatalf("physical parity %s: %+v %v", path, file, err)
			}
		}
		for _, path := range []string{requestPath, "receipts/000000000002.json", "evidence/" + string(wire.Sum(raw))} {
			if _, ok := observed.Files[path]; !ok {
				t.Fatalf("actual read missing: %s", path)
			}
		}
		// An inline/selected afterimage cannot manufacture a physical namespace key.
		if _, ok := observed.Files["physical-request"]; ok {
			t.Fatal("request ID used as physical path")
		}
	})
	t.Run("bounded-and-repeated", func(t *testing.T) {
		reads := newPhysicalReads()
		reads.limit = 1
		if err := reads.add("evidence/a", nil); err != nil {
			t.Fatal(err)
		}
		if reads.files["evidence/a"].Bytes != 0 {
			t.Fatal("empty file lost")
		}
		if err := reads.add("evidence/a", nil); err != nil {
			t.Fatal(err)
		}
		requireCode(t, reads.add("evidence/a", []byte("x")), wire.CodeSnapshotMoved)
		requireCode(t, reads.add("evidence/b", nil), wire.CodeLimitExceeded)
		if len(reads.files) != 1 {
			t.Fatal("bound insertion occurred")
		}
	})
	for _, kind := range []string{"divergent-intent", "pending", "corrupt-private"} {
		t.Run(kind, func(t *testing.T) {
			repo, r := setup(t)
			if p, _, err := r.AuditForWriteObserved(); err != nil || p.IntentError != nil {
				t.Fatalf("fixture %v", err)
			}
			switch kind {
			case "divergent-intent":
				fixture.Write(t, filepath.Join(repo.IntentDir, "tickets/A.json"), fixture.Ticket("A").Encode())
			case "pending":
				path, _ := snapshot.RequestPath("pending")
				appendReceipt(t, repo, "MUTATION", map[string][]byte{path: requestBytes("pending", 2, wire.Sum([]byte("pending")), false)}, "pending", false, false, false)
			case "corrupt-private":
				fixture.Write(t, filepath.Join(repo.StateDir, "reservations.json"), []byte("{}\n"))
			}
			proof, obs, err := r.AuditForWriteObserved()
			if obs.Files != nil || obs.Cleanup != nil {
				t.Fatalf("ineligible metadata leaked %+v", obs)
			}
			if kind == "divergent-intent" {
				if err != nil || proof == nil || proof.IntentError == nil {
					t.Fatalf("divergence fixture did not reach nil-error IntentError: %+v %v", proof, err)
				}
			} else if err == nil {
				t.Fatal("fault not reached")
			}
		})
	}
}

func TestCALV0026_PhysicalObservationLifetime(t *testing.T) {
	for _, kind := range []string{"file-close", "root-close", "file-close-and-movement"} {
		t.Run(kind, func(t *testing.T) {
			repo, r := setup(t)
			if _, err := r.AuditForWrite(); err != nil {
				t.Fatal(err)
			}
			oldFile, oldRoot := closeReadFile, closeReadRoot
			t.Cleanup(func() { closeReadFile, closeReadRoot = oldFile, oldRoot })
			injected, attempts := false, 0
			r.afterCapture = func() { attempts++ }
			sentinel := errors.New("injected physical lifetime close")
			closeReadFile = func(f *os.File) error {
				err := f.Close()
				if err != nil {
					return err
				}
				if kind != "root-close" && !injected && attempts > 0 && strings.HasSuffix(f.Name(), "000000000001.json") {
					injected = true
					if kind == "file-close-and-movement" {
						p := filepath.Join(repo.StateDir, "head.json")
						fixture.Write(t, p, read(t, p))
					}
					return sentinel
				}
				return nil
			}
			closeReadRoot = func(root *os.Root) error {
				err := root.Close()
				if err != nil {
					return err
				}
				if kind == "root-close" && !injected {
					injected = true
					return sentinel
				}
				return nil
			}
			proof, obs, err := r.AuditForWriteObserved()
			if !injected {
				t.Fatal("close injection not reached")
			}
			if proof != nil || obs.Files != nil || !errors.Is(obs.Cleanup, sentinel) || wire.CodeOf(err) != wire.CodeUnsupportedFilesystem {
				t.Fatalf("cleanup lost proof%v obs%+v err%v", proof, obs, err)
			}
			if attempts > 1 {
				t.Fatalf("cleanup retried %d attempts", attempts)
			}
		})
	}
	t.Run("retry-map-is-fresh", func(t *testing.T) {
		repo, r := setup(t)
		if _, err := r.AuditForWrite(); err != nil {
			t.Fatal(err)
		}
		seen := 0
		r.afterCapture = func() {
			seen++
			if seen == 1 {
				path, _ := snapshot.RequestPath("retry")
				appendReceipt(t, repo, "MUTATION", map[string][]byte{path: requestBytes("retry", 2, wire.Sum([]byte("retry")), false)}, "retry", true, true, false)
			}
		}
		proof, obs, err := r.AuditForWriteObserved()
		if err != nil || obs.Cleanup != nil || seen != 2 || proof.LastSeq != "2" {
			t.Fatalf("retry %+v %v seen%d", proof, err, seen)
		}
		head := read(t, filepath.Join(repo.StateDir, "head.json"))
		if obs.Files["head.json"].Sha256 != wire.Sum(head) {
			t.Fatal("failed attempt head survived")
		}
	})
}
