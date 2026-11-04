// SPDX-License-Identifier: Apache-2.0
package dogfoodflow

import (
	"bytes"
	"context"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDTLV0016ReportPublication(t *testing.T) {
	t.Run("DTL-V0-016", func(t *testing.T) {
		root := t.TempDir()
		report := filepath.Join(root, "dogfood-report.json")
		old := []byte("{\n  \"profile\": \"test\"\n  ,\"dogfoodCheck\": null\n}\n")
		if err := os.WriteFile(report, old, 0600); err != nil {
			t.Fatal(err)
		}
		unrelated := filepath.Join(root, ".dogfood-report-unrelated.tmp")
		if err := os.WriteFile(unrelated, []byte("another invocation\n"), 0600); err != nil {
			t.Fatal(err)
		}
		c := newCheck(context.Background(), "dogfood-check", CheckOptions{Root: root}, io.Discard, io.Discard)
		// Negative control for Git-directory staging: this directory cannot be written.
		// The real lane additionally exercises separate sandbox mounts (EXDEV on BASE).
		c.gitDir = filepath.Join(root, "missing", "git")
		c.report = report
		if err := c.record(true, 1); err != nil {
			t.Fatal(err)
		}
		published, err := os.ReadFile(report)
		if err != nil || !bytes.Contains(published, []byte(`"outputsAgree": true`)) {
			t.Fatalf("report=%s err=%v", published, err)
		}
		info, err := os.Stat(report)
		if err != nil || info.Mode().Perm()&0077 != 0 {
			t.Fatalf("report is not private: info=%v err=%v", info, err)
		}

		// Fault injection: a nonempty directory cannot be replaced by a report.
		blocked := filepath.Join(root, "blocked-report")
		if err := os.Mkdir(blocked, 0700); err != nil {
			t.Fatal(err)
		}
		marker := filepath.Join(blocked, "keep")
		if err := os.WriteFile(marker, old, 0600); err != nil {
			t.Fatal(err)
		}
		if err := replaceCheckReport(blocked, []byte("must not publish")); err == nil {
			t.Fatal("rename fault was swallowed")
		}
		if kept, err := os.ReadFile(marker); err != nil || !bytes.Equal(kept, old) {
			t.Fatalf("existing destination changed: %q %v", kept, err)
		}
		if err := replaceCheckReport(filepath.Join(root, "absent", "report"), old); err == nil {
			t.Fatal("temporary creation fault was swallowed")
		}
		if kept, err := os.ReadFile(report); err != nil || !bytes.Equal(kept, published) {
			t.Fatalf("published report changed after failed writes: %q %v", kept, err)
		}
		entries, err := os.ReadDir(root)
		if err != nil {
			t.Fatal(err)
		}
		for _, entry := range entries {
			if strings.HasPrefix(entry.Name(), ".dogfood-report-") && entry.Name() != filepath.Base(unrelated) {
				t.Fatalf("temporary not cleaned: %s", entry.Name())
			}
		}
		if kept, err := os.ReadFile(unrelated); err != nil || string(kept) != "another invocation\n" {
			t.Fatalf("another invocation's temporary changed: %q %v", kept, err)
		}
	})
}
