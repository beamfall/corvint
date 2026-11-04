// SPDX-License-Identifier: Apache-2.0
package dogfoodflow

import (
	"errors"
	"os"
	"path/filepath"
	"syscall"
	"testing"
)

func TestDWCV0048ReportPublicationAcrossMounts(t *testing.T) {
	for _, fail := range []bool{false, true} {
		name := "published"
		if fail {
			name = "refused-without-truncation"
		}
		t.Run(name, func(t *testing.T) {
			metadata, dir := t.TempDir(), t.TempDir()
			staged, target := filepath.Join(metadata, "report.tmp"), filepath.Join(dir, "report.json")
			if err := os.WriteFile(target, []byte("prior report\n"), 0600); err != nil {
				t.Fatal(err)
			}
			calls := 0
			rename := func(src, dst string) error {
				calls++
				if calls == 1 {
					return &os.LinkError{Op: "rename", Old: src, New: dst, Err: syscall.EXDEV}
				}
				if filepath.Dir(src) != filepath.Dir(dst) {
					t.Fatal("fallback is not same-directory")
				}
				if fail {
					return os.ErrPermission
				}
				return os.Rename(src, dst)
			}
			err := publishDogfoodReport(staged, target, []byte("checked report\n"), rename)
			want := "checked report\n"
			if fail {
				want = "prior report\n"
				if !errors.Is(err, os.ErrPermission) {
					t.Fatal(err)
				}
			} else if err != nil {
				t.Fatal(err)
			}
			got, err := os.ReadFile(target)
			if err != nil || string(got) != want {
				t.Fatalf("%q %v", got, err)
			}
			if calls != 2 {
				t.Fatal(calls)
			}
			if _, err := os.Stat(staged); !os.IsNotExist(err) {
				t.Fatalf("staging file retained: %v", err)
			}
			entries, err := os.ReadDir(dir)
			if err != nil || len(entries) != 1 {
				t.Fatalf("temporary files retained: %v %v", entries, err)
			}
		})
	}
}
