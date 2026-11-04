// SPDX-License-Identifier: Apache-2.0

package dogfoodflow

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
)

// DWC-V0-929: EXDEV alone permits a same-directory retry. Failures after
// CreateTemp preserve the report and clean every staged file, on both mounts.
func TestCheckDWCV0929ReportStaysOnWorktreeMount(t *testing.T) {
	for _, mode := range []os.FileMode{0644, 0600} {
		for _, failure := range []string{"", "first", "fallback"} {
			t.Run(mode.String()+failure, func(t *testing.T) {
				root := t.TempDir()
				gitDir := t.TempDir()
				report := filepath.Join(root, "dogfood-report.json")
				original := []byte("{\n  \"complete\": true\n  ,\"dogfoodCheck\": null\n}\n")
				if err := os.WriteFile(report, original, mode); err != nil {
					t.Fatal(err)
				}
				c := newCheck(context.Background(), "dogfood-check", CheckOptions{Root: root}, nil, nil)
				c.gitDir, c.report = gitDir, report
				c.baseSHA, c.treeSHA = strings.Repeat("a", 64), strings.Repeat("b", 64)
				calls := 0
				c.reportRename = func(from, to string) error {
					calls++
					if data, err := os.ReadFile(from); err != nil || !bytes.Contains(data, []byte(`"outputsAgree": true`)) {
						t.Fatalf("not staged before rename: %s %v", data, err)
					}
					if calls == 1 {
						if filepath.Dir(from) != gitDir {
							t.Fatal(from)
						}
						if failure == "first" {
							return syscall.EACCES
						}
						return &os.LinkError{Op: "rename", Old: from, New: to, Err: syscall.EXDEV}
					}
					if filepath.Dir(from) != root {
						t.Fatal(from)
					}
					if failure == "fallback" {
						return syscall.EACCES
					}
					return os.Rename(from, to)
				}
				err := c.record(true, 0)
				data, _ := os.ReadFile(report)
				if failure == "" {
					if err != nil || calls != 2 || bytes.Equal(data, original) {
						t.Fatalf("EXDEV fallback: calls=%d err=%v data=%s", calls, err, data)
					}
				} else if err == nil || !errors.Is(err, syscall.EACCES) || !bytes.Equal(data, original) || failure == "first" && calls != 1 {
					t.Fatalf("failed publish: calls=%d err=%v data=%s", calls, err, data)
				}
				info, err := os.Stat(report)
				if err != nil || info.Mode().Perm() != mode {
					t.Fatalf("report mode: %v %v", info, err)
				}
				for _, dir := range []string{root, gitDir} {
					left, err := filepath.Glob(filepath.Join(dir, ".dogfood-report.*.tmp"))
					if err != nil || len(left) != 0 {
						t.Fatalf("staging leak: %v %v", left, err)
					}
				}
			})
		}
	}
}

// DWC-V0-929 / V1-4488: the CLI failure includes the underlying error and
// names EXDEV if even the same-directory publish is refused by the filesystem.
func TestCheckDWCV0929ReportErrorNamesEXDEV(t *testing.T) {
	root := t.TempDir()
	var out bytes.Buffer
	c := newCheck(context.Background(), "dogfood-check", CheckOptions{Root: root}, &out, &out)
	c.report = filepath.Join(root, "report.json")
	if err := os.WriteFile(c.report, []byte("{\n  ,\"dogfoodCheck\": null\n}\n"), 0644); err != nil {
		t.Fatal(err)
	}
	c.reportRename = func(from, to string) error {
		return &os.LinkError{Op: "rename", Old: from, New: to, Err: syscall.EXDEV}
	}
	code, _ := boundary(context.Background(), func() int { c.recordOrFail(true, 0); return 0 }, func() {})
	if code != 1 || !strings.Contains(out.String(), "FAIL dogfood-report-write") || !strings.Contains(out.String(), "EXDEV") || !strings.Contains(out.String(), "rename") || strings.Contains(out.String(), "make .corvint") {
		t.Fatalf("lost filesystem diagnosis: %d %s", code, out.String())
	}
}
