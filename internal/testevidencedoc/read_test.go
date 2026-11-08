// SPDX-License-Identifier: Apache-2.0
package testevidencedoc

import (
	"context"
	"crypto/sha256"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/linecov"
	"github.com/Beamfall/corvint/internal/testevidence"
)

func repository(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	for _, args := range [][]string{{"init", "-q"}, {"-c", "user.name=Fixture", "-c", "user.email=fixture@example.test", "commit", "--allow-empty", "-qm", "fixture"}} {
		c := exec.Command("git", append([]string{"-C", root}, args...)...)
		if out, e := c.CombinedOutput(); e != nil {
			t.Fatalf("git: %v %s", e, out)
		}
	}
	return root
}
func snapshot(t *testing.T, root string) string {
	t.Helper()
	h := sha256.New()
	err := filepath.WalkDir(root, func(p string, e fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		info, err := e.Info()
		if err != nil {
			return err
		}
		fmt.Fprintf(h, "%s %v\n", strings.TrimPrefix(p, root), info.Mode())
		if info.Mode().IsRegular() {
			data, err := os.ReadFile(p)
			if err != nil {
				return err
			}
			h.Write(data)
		} else if info.Mode()&os.ModeSymlink != 0 {
			s, err := os.Readlink(p)
			if err != nil {
				return err
			}
			h.Write([]byte(s))
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return fmt.Sprintf("%x", h.Sum(nil))
}
func retainFixture(t *testing.T, root string) string {
	t.Helper()
	_, r := fixture(t)
	raw, err := testevidence.Encode(r)
	if err != nil {
		t.Fatal(err)
	}
	dir := filepath.Join(root, testevidence.EvidenceDirectory)
	if err = os.MkdirAll(dir, 0700); err != nil {
		t.Fatal(err)
	}
	p := filepath.Join(dir, "record.json")
	if err = os.WriteFile(p, raw, 0600); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestRSPV0051ReadNoWrite(t *testing.T) {
	root := repository(t)
	file := retainFixture(t, root)
	if err := os.MkdirAll(filepath.Join(root, ".context-corvint"), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".context-corvint", "trace"), []byte("trace state"), 0600); err != nil {
		t.Fatal(err)
	}
	for _, malformed := range []bool{false, true} {
		if malformed {
			if e := os.WriteFile(file, []byte("{"), 0600); e != nil {
				t.Fatal(e)
			}
		}
		before := snapshot(t, root)
		for _, q := range []Query{{Path: "src.go", Revision: "HEAD"}, {Path: "src.go", Start: 2, End: 3, Revision: "HEAD"}, {Test: "suite/test", Revision: "HEAD"}, {Explain: "suite/test", Base: "HEAD", Revision: "HEAD"}} {
			d := Read(context.Background(), root, q)
			if d.Status != "UNKNOWN" || !slices.Contains(d.Reasons, "inventory-unavailable") {
				t.Fatal(d)
			}
			if malformed && !slices.Contains(d.Reasons, "malformed-record") {
				t.Fatal(d)
			}
			if !malformed && (len(d.Records) != 1 || d.Records[0].Reason != "run-context-unresolved") {
				t.Fatal(d)
			}
			if q.Explain != "" && (d.Explanation.Runners[0].Mode != "full" || !slices.Contains(d.Reasons, "unqualified")) {
				t.Fatal(d)
			}
		}
		if after := snapshot(t, root); before != after {
			t.Fatal("repository or trace state changed")
		}
	}
}

func TestRSPV0051ReadMissingUnsafeAndIndex(t *testing.T) {
	root := repository(t)
	q := Query{Test: "suite/test", Revision: "HEAD"}
	if d := Read(context.Background(), root, q); !slices.Contains(d.Reasons, "no-fresh-record") {
		t.Fatal(d)
	}
	q.Revision = "missing"
	if d := Read(context.Background(), root, q); !slices.Contains(d.Reasons, "revision-unresolved") {
		t.Fatal(d)
	}
	q.Revision = "HEAD"
	q.Index = "missing.idx"
	if d := Read(context.Background(), root, q); !slices.Contains(d.Reasons, "index-unreadable") {
		t.Fatal(d)
	}
	in, r := fixture(t)
	ix := &linecov.Index{Format: linecov.Format, Module: "example.test", Commit: in.Base, Runner: "fixture", Evidence: []testevidence.Record{r}}
	f, e := os.Create(filepath.Join(root, "records.idx"))
	if e != nil {
		t.Fatal(e)
	}
	e = ix.Encode(f)
	f.Close()
	if e != nil {
		t.Fatal(e)
	}
	q.Index = "records.idx"
	if d := Read(context.Background(), root, q); len(d.Records) != 1 || d.Records[0].Reason != "run-context-unresolved" {
		t.Fatal(d)
	}
	if e = os.WriteFile(filepath.Join(root, "records.idx"), []byte("bad"), 0600); e != nil {
		t.Fatal(e)
	}
	if d := Read(context.Background(), root, q); !slices.Contains(d.Reasons, "index-malformed") {
		t.Fatal(d)
	}
	q.Index = ""
	p := retainFixture(t, root)
	if e = os.Remove(p); e != nil {
		t.Fatal(e)
	}
	outside := filepath.Join(t.TempDir(), "outside.json")
	if e = os.WriteFile(outside, []byte("secret"), 0600); e != nil {
		t.Fatal(e)
	}
	if e = os.Symlink(outside, p); e != nil {
		t.Skip("symlinks unavailable")
	}
	if d := Read(context.Background(), root, q); !slices.Contains(d.Reasons, "records-unreadable") || len(d.Records) != 0 {
		t.Fatal(d)
	}
}
