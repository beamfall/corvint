// SPDX-License-Identifier: Apache-2.0
package dogfoodflow

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
)

func TestDWCV07501ReportPublish(t *testing.T) {
	for _, initial := range []error{nil, syscall.EXDEV, syscall.EACCES} {
		t.Run(initialName(initial), func(t *testing.T) {
			dir := t.TempDir()
			staged, target := filepath.Join(dir, "git-stage"), filepath.Join(dir, "report.json")
			if err := os.WriteFile(staged, []byte("complete report\n"), 0o640); err != nil {
				t.Fatal(err)
			}
			calls := 0
			err := publishReport(staged, target, func(from, to string) error {
				calls++
				if calls == 1 && initial != nil {
					return &os.LinkError{Op: "rename", Old: from, New: to, Err: initial}
				}
				if calls == 2 && filepath.Dir(from) != filepath.Dir(to) {
					t.Fatal("fallback is not same-directory")
				}
				return os.Rename(from, to)
			})
			if initial == syscall.EACCES {
				if !errors.Is(err, initial) || calls != 1 {
					t.Fatalf("error=%v calls=%d", err, calls)
				}
			} else {
				if err != nil {
					t.Fatal(err)
				}
				data, err := os.ReadFile(target)
				if err != nil || string(data) != "complete report\n" {
					t.Fatalf("data=%q err=%v", data, err)
				}
				info, err := os.Stat(target)
				if err != nil || info.Mode().Perm() != 0o640 {
					t.Fatalf("mode=%v err=%v", info, err)
				}
			}
			if _, err := os.Stat(staged); !os.IsNotExist(err) {
				t.Fatalf("staging file retained: %v", err)
			}
			left, _ := filepath.Glob(filepath.Join(dir, ".dogfood-report.*.tmp"))
			if len(left) != 0 {
				t.Fatalf("temporary files retained: %v", left)
			}
		})
	}
}

// DWC-V0-7501: failed same-directory publication cleans up both temporary files.
func TestDWCV07501ReportPublishFallbackCleanup(t *testing.T) {
	dir := t.TempDir()
	staged := filepath.Join(dir, "staged")
	if err := os.WriteFile(staged, []byte("report"), 0o640); err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(dir, "report")
	if err := os.WriteFile(target, []byte("previous report"), 0o640); err != nil {
		t.Fatal(err)
	}
	calls := 0
	err := publishReport(staged, target, func(string, string) error {
		calls++
		if calls == 1 {
			return syscall.EXDEV
		}
		return syscall.EACCES
	})
	if !errors.Is(err, syscall.EACCES) || calls != 2 {
		t.Fatalf("err=%v calls=%d", err, calls)
	}
	entries, _ := os.ReadDir(dir)
	if len(entries) != 1 || entries[0].Name() != "report" {
		t.Fatalf("leftover entries: %v", entries)
	}
	data, _ := os.ReadFile(target)
	if string(data) != "previous report" {
		t.Fatalf("published report changed after refusal: %q", data)
	}
	ignored, err := os.ReadFile("../../.gitignore")
	if err != nil || !strings.Contains(string(ignored), ".corvint/.dogfood-report.*.tmp") {
		t.Fatalf("fallback temp pattern is not ignored: %v", err)
	}
}

func initialName(err error) string {
	if err == nil {
		return "normal"
	}
	return err.Error()
}
