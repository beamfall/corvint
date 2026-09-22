//go:build darwin

package authoritystore

import (
	"context"
	"os"
	"path/filepath"
	"strconv"
	"testing"

	"github.com/Beamfall/corvint/internal/localauthority"
)

func filesystemEvidenceFixture(t *testing.T) (string, *evidenceView, string) {
	t.Helper()
	root := t.TempDir()
	m := manifestFixture()
	m.ReaderGID = strconv.Itoa(os.Getgid())
	for i := range m.Files {
		f := &m.Files[i]
		raw := []byte(f.Path)
		f.Size = strconv.Itoa(len(raw))
		f.SHA256 = localauthority.BytesDigest(raw)
		p := filepath.Join(root, "repo", f.Path)
		if err := os.MkdirAll(filepath.Dir(p), 0700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, raw, 0440); err != nil {
			t.Fatal(err)
		}
		if err := os.Chmod(p, 0440); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.MkdirAll(filepath.Join(root, "repo/.git/refs"), 0700); err != nil {
		t.Fatal(err)
	}
	raw := encode(t, m)
	if err := os.WriteFile(filepath.Join(root, "manifest.json"), raw, 0440); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(filepath.Join(root, "manifest.json"), 0440); err != nil {
		t.Fatal(err)
	}
	if err := filepath.WalkDir(root, func(p string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return os.Chmod(p, 0550)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = filepath.WalkDir(root, func(p string, d os.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if d.IsDir() {
				return os.Chmod(p, 0700)
			}
			return nil
		})
	})
	return root, &evidenceView{owner: uint32(os.Getuid()), gid: uint32(os.Getgid()), manifest: m}, localauthority.BytesDigest(raw)
}
func auditTestEvidence(t *testing.T, root string, v *evidenceView, digest string) ([]evidenceNode, error) {
	t.Helper()
	f, err := os.Open(root)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	return v.audit(context.Background(), f, true, digest)
}

func TestEvidenceFilesystemAuditRejectsTamperingAndExtraAccess(t *testing.T) {
	t.Run("PLE-V0-003 evidence audit rejects tampering and extra access", func(t *testing.T) {
		for _, kind := range []string{"valid", "content", "world-readable", "group-writable", "symlink", "hardlink", "extra", "missing"} {
			t.Run(kind, func(t *testing.T) {
				root, v, digest := filesystemEvidenceFixture(t)
				p := filepath.Join(root, "repo", v.manifest.Files[2].Path)
				dir := filepath.Dir(p)
				if kind != "valid" {
					if err := os.Chmod(dir, 0750); err != nil {
						t.Fatal(err)
					}
				}
				switch kind {
				case "content":
					if err := os.Chmod(p, 0640); err != nil {
						t.Fatal(err)
					}
					if err := os.WriteFile(p, []byte("tamper"), 0640); err != nil {
						t.Fatal(err)
					}
					if err := os.Chmod(p, 0440); err != nil {
						t.Fatal(err)
					}
				case "world-readable":
					if err := os.Chmod(p, 0444); err != nil {
						t.Fatal(err)
					}
				case "group-writable":
					if err := os.Chmod(p, 0460); err != nil {
						t.Fatal(err)
					}
				case "symlink":
					if err := os.Remove(p); err != nil {
						t.Fatal(err)
					}
					if err := os.Symlink("../../HEAD", p); err != nil {
						t.Fatal(err)
					}
				case "hardlink":
					if err := os.Link(p, filepath.Join(t.TempDir(), "link")); err != nil {
						t.Fatal(err)
					}
				case "extra":
					if err := os.WriteFile(filepath.Join(dir, "unexpected"), []byte("x"), 0440); err != nil {
						t.Fatal(err)
					}
				case "missing":
					if err := os.Remove(p); err != nil {
						t.Fatal(err)
					}
				}
				if kind != "valid" {
					if err := os.Chmod(dir, 0550); err != nil {
						t.Fatal(err)
					}
				}
				_, err := auditTestEvidence(t, root, v, digest)
				if (err == nil) != (kind == "valid") {
					t.Fatalf("%s: %v", kind, err)
				}
			})
		}

	})
}
func TestEvidenceRecheckDetectsReplacementAndCancellation(t *testing.T) {
	root, v, digest := filesystemEvidenceFixture(t)
	before, err := auditTestEvidence(t, root, v, digest)
	if err != nil {
		t.Fatal(err)
	}
	p := filepath.Join(root, "repo", v.manifest.Files[2].Path)
	dir := filepath.Dir(p)
	if err := os.Chmod(dir, 0750); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(p); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, raw, 0440); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(p, 0440); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(dir, 0550); err != nil {
		t.Fatal(err)
	}
	after, err := auditTestEvidence(t, root, v, digest)
	if err != nil {
		t.Fatal(err)
	}
	if sameEvidenceNodes(before, after) {
		t.Fatal("replacement hidden")
	}
	f, err := os.Open(root)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := v.audit(ctx, f, true, digest); err == nil {
		t.Fatal("cancelled audit")
	}
}
func TestEvidenceDescriptorBudgetUsesCurrentProcess(t *testing.T) {
	baseline, limit, err := requireEvidenceDescriptorHeadroom()
	if err != nil {
		t.Fatal(err)
	}
	if baseline < 3 || limit < baseline+48 {
		t.Fatalf("baseline=%d limit=%d", baseline, limit)
	}
}
