// SPDX-License-Identifier: Apache-2.0
package dogfoodflow

import (
	"errors"
	"os"
	"path/filepath"
	"syscall"
	"testing"
)

// DWC-V0-7319: split Git/worktree mounts still publish an exact report atomically.
func TestCheckReportPublicationAcrossMounts(t *testing.T) {
	t.Parallel()
	for _, failure := range []string{"cross-device", "unrelated", "destination"} {
		t.Run(failure, func(t *testing.T) {
			root := t.TempDir()
			staged := filepath.Join(root, "git-stage")
			report := filepath.Join(root, "report.json")
			if err := os.WriteFile(report, []byte("previous"), 0o600); err != nil {
				t.Fatal(err)
			}
			calls := 0
			denied := errors.New("destination refused")
			rename := func(from, to string) error {
				calls++
				before, err := os.ReadFile(report)
				if err != nil || string(before) != "previous" {
					t.Fatalf("report changed before atomic publication: %q %v", before, err)
				}
				if calls == 1 {
					if failure == "unrelated" {
						return denied
					}
					return &os.LinkError{Op: "rename", Old: from, New: to, Err: syscall.EXDEV}
				}
				if failure == "destination" {
					return denied
				}
				return os.Rename(from, to)
			}
			err := publishCheckReport(staged, report, []byte("new exact report"), rename)
			want := "previous"
			if failure == "cross-device" {
				if err != nil {
					t.Fatal(err)
				}
				want = "new exact report"
			} else if !errors.Is(err, denied) {
				t.Fatalf("publication error=%v, want original failure", err)
			}
			got, readErr := os.ReadFile(report)
			if readErr != nil || string(got) != want {
				t.Fatalf("published=%q, %v, want %q", got, readErr, want)
			}
			entries, err := os.ReadDir(root)
			if err != nil || len(entries) != 1 || entries[0].Name() != "report.json" {
				t.Fatalf("temporary files leaked: %v %v", entries, err)
			}
		})
	}
}
