package store

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/Beamfall/corvint/internal/tasks/intent"
	"github.com/Beamfall/corvint/internal/tasks/journal"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// The §1 intent bounds apply through archive names without changing identities.
func TestInventoryArchiveBounds(t *testing.T) {
	for _, tc := range []struct {
		path  string
		bound int
	}{
		{"intent/queue.json", wire.MaxQueueFileBytes},
		{"intent/policy.json", wire.MaxPolicyFileBytes},
		{"intent/import-map.json", wire.MaxImportMapBytes},
		{"intent/tickets/x.json", wire.MaxTicketFileBytes},
		{"intent/releases/x.json", wire.MaxReleaseFileBytes},
		{"evidence/blob", wire.MaxEvidenceBlobBytes},
		{"requests/queue.json", wire.MaxEvidenceBlobBytes},
		{"intent/other.json", wire.MaxEvidenceBlobBytes},
	} {
		t.Run(tc.path, func(t *testing.T) {
			_, _, err := fileEntryWithReader("unused", tc.path, func(_ string, bound int) ([]byte, error) {
				if bound != tc.bound {
					t.Errorf("effective bound=%d want=%d", bound, tc.bound)
				}
				return nil, nil
			})
			if err != nil {
				t.Fatal(err)
			}
			for _, n := range []int{tc.bound, tc.bound + 1} {
				t.Run(fmt.Sprint(n), func(t *testing.T) {
					digest := wire.Sum([]byte("observed"))
					entry, present, err := fileEntryWithReader("unused", tc.path, func(string, int) ([]byte, error) { t.Fatal("observed bytes reread"); return nil, nil }, map[string]journal.PhysicalFile{tc.path: {Bytes: n, Sha256: digest}})
					if n == tc.bound {
						if err != nil || !present || entry.Path != tc.path || entry.Sha256 != digest || entry.Bytes.Uint64() != uint64(n) {
							t.Fatalf("exact-bound metadata: %+v present=%v err=%v", entry, present, err)
						}
					} else if wire.CodeOf(err) != wire.CodeLimitExceeded || present {
						t.Errorf("overflow metadata accepted: present=%v err=%v", present, err)
					}
					// Also exercise the real bounded reader for each intent file.
					if tc.bound == wire.MaxEvidenceBlobBytes {
						return
					}
					raw := bytes.Repeat([]byte("x"), n)
					full := filepath.Join(t.TempDir(), "file")
					if err := os.WriteFile(full, raw, 0600); err != nil {
						t.Fatal(err)
					}
					entry, present, err = fileEntryWithReader(full, tc.path, intent.ReadFile)
					if n == tc.bound {
						if err != nil || !present || entry.Path != tc.path || entry.Sha256 != wire.Sum(raw) || entry.Bytes.Uint64() != uint64(n) {
							t.Fatalf("exact-bound read: %+v present=%v err=%v", entry, present, err)
						}
					} else if wire.CodeOf(err) != wire.CodeLimitExceeded || present {
						t.Fatalf("overflow read accepted: present=%v err=%v", present, err)
					}
				})
			}
		})
	}
}
