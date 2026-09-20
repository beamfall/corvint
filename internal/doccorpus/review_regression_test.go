package doccorpus

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/cem/wire"
)

func TestCorpusScopeAdditionFreshness(t *testing.T) {
	t.Run("DCP-V1-011 inventory drift", func(t *testing.T) {
		root, m := fixture(t)
		a, err := Build(context.Background(), root, m)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(root, "src/new.go"), []byte("package value\nfunc New() {}\n"), 0600); err != nil {
			t.Fatal(err)
		}
		for _, committed := range []bool{false, true} {
			if committed {
				git(t, root, "add", "src/new.go")
				git(t, root, "commit", "-qm", "additional scope input")
			}
			state, limits, err := Freshness(context.Background(), root, a)
			if err != nil || state != "stale" {
				t.Fatalf("scope addition ignored: %v %v %v", state, limits, err)
			}
		}
	})
}
func TestCorpusNativeExcludedInputs(t *testing.T) {
	t.Run("DCP-V1-003 DCP-V1-010 native admission", func(t *testing.T) {
		for _, test := range []struct {
			path, body string
			refuse     bool
		}{{"src/generated/reference.md", "# Generated\n", true}, {"src/header.md", "<!-- Code generated fixture DO NOT EDIT. -->\n# Generated\n", true}, {"src/vendor/manual.md", "# Vendor\n", false}} {
			root, _ := fixture(t)
			if err := os.MkdirAll(filepath.Dir(filepath.Join(root, test.path)), 0700); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(root, test.path), []byte(test.body), 0600); err != nil {
				t.Fatal(err)
			}
			git(t, root, "add", test.path)
			git(t, root, "commit", "-qm", "excluded input")
			m, err := Inventory(context.Background(), root, git(t, root, "rev-parse", "HEAD"), "src", "2026-09-19T00:00:00Z")
			if err != nil {
				t.Fatal(err)
			}
			a, err := Build(context.Background(), root, m)
			if test.refuse {
				if err == nil {
					t.Fatal("generated source admitted")
				}
				continue
			}
			if err != nil {
				t.Fatal(err)
			}
			for _, s := range a.Subjects {
				if subjectPath(s, test.path) {
					t.Fatal("excluded source became a subject")
				}
			}
			found := false
			for _, gap := range a.Gaps {
				if strings.Contains(gap.Subject, test.path) && gap.Reason == "vendor/build excluded" {
					found = true
				}
			}
			if !found {
				t.Fatal("exclusion gap missing")
			}
		}
	})
}
func TestCorpusCEMRejectsBaseSymlink(t *testing.T) {
	t.Run("DCP-V1-014 CEM mode admission", func(t *testing.T) {
		root, _ := fixture(t)
		p := filepath.Join(root, "source.md")
		if err := os.Symlink("original", p); err != nil {
			t.Skip(err)
		}
		git(t, root, "add", "source.md")
		git(t, root, "commit", "-qm", "symlink base")
		base := git(t, root, "rev-parse", "HEAD")
		if err := os.Remove(p); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte("original"), 0600); err != nil {
			t.Fatal(err)
		}
		git(t, root, "add", "source.md")
		git(t, root, "commit", "-qm", "regular source with same blob")
		m, err := Inventory(context.Background(), root, git(t, root, "rev-parse", "HEAD"), "source.md", "2026-09-19T00:00:00Z")
		if err != nil {
			t.Fatal(err)
		}
		a, err := Build(context.Background(), root, m)
		if err != nil {
			t.Fatal(err)
		}
		data, _ := Encode(map[string]any{"spec": wire.Spec01, "baseRevision": base, "patchSha256": strings.Repeat("a", 64), "evidence": []any{}, "hunks": []any{}})
		if _, err := wire.ParseMap(data); err != nil {
			t.Fatal(err)
		}
		if _, err = CEMProjection(context.Background(), root, a, data, ""); err == nil || !strings.Contains(err.Error(), "CEM base") {
			t.Fatal("base symlink projected as usable source")
		}
	})
}
func TestCorpusMaintenanceConcurrentPublication(t *testing.T) {
	t.Run("DCP-V1-017 DCP-V1-018 confined publication", func(t *testing.T) {
		for _, mode := range []string{"parent-replaced", "destination-replaced", "concurrent-publication", "open-writer"} {
			t.Run(mode, func(t *testing.T) {
				root := t.TempDir()
				if err := os.Mkdir(filepath.Join(root, "docs"), 0700); err != nil {
					t.Fatal(err)
				}
				page := filepath.Join(root, "docs/page.md")
				before := []byte("Human prose\n")
				if err := os.WriteFile(page, before, 0640); err != nil {
					t.Fatal(err)
				}
				writer, err := os.OpenFile(page, os.O_WRONLY, 0)
				if err != nil {
					t.Fatal(err)
				}
				defer writer.Close()
				outside := t.TempDir()
				hook := func(stage string) {
					if stage == "before-capture" && mode == "parent-replaced" {
						if err := os.Rename(filepath.Join(root, "docs"), filepath.Join(root, "old")); err != nil {
							t.Fatal(err)
						}
						if err := os.Symlink(outside, filepath.Join(root, "docs")); err != nil {
							t.Fatal(err)
						}
					}
					if stage == "before-capture" && mode == "destination-replaced" {
						if err := os.Remove(page); err != nil {
							t.Fatal(err)
						}
						if err := os.WriteFile(page, []byte("Concurrent prose\n"), 0600); err != nil {
							t.Fatal(err)
						}
					}
					if stage == "before-publish" && mode == "concurrent-publication" {
						if err := os.WriteFile(page, []byte("Concurrent prose\n"), 0600); err != nil {
							t.Fatal(err)
						}
					}
					if stage == "before-publish" && mode == "open-writer" {
						if _, err := writer.WriteAt([]byte("Concurrent prose\n"), 0); err != nil {
							t.Fatal(err)
						}
					}
				}
				recovery, err := applyCorpusPage(root, "docs/page.md", before, []byte("Generated update\n"), hook)
				if mode == "open-writer" {
					if err != nil {
						t.Fatal(err)
					}
					kept, err := os.ReadFile(filepath.Join(root, recovery))
					if err != nil || string(kept) != "Concurrent prose\n" {
						t.Fatalf("concurrent inode writes lost: %q %v", kept, err)
					}
					return
				}
				if err == nil {
					t.Fatal("concurrent replacement accepted")
				}
				if mode == "parent-replaced" {
					entries, err := os.ReadDir(outside)
					if err != nil || len(entries) != 0 {
						t.Fatal("outside directory written")
					}
					return
				}
				kept, err := os.ReadFile(page)
				if err != nil || string(kept) != "Concurrent prose\n" {
					t.Fatalf("concurrent page overwritten: %q %v", kept, err)
				}
			})
		}
	})
}

func TestCorpusMaintenanceTargetAdmission(t *testing.T) {
	t.Run("DCP-V1-017 DCP-V1-018 metadata protection", func(t *testing.T) {
		root, m := fixture(t)
		a, err := Build(context.Background(), root, m)
		if err != nil {
			t.Fatal(err)
		}
		for _, page := range []string{".git/config", ".git/notes.md", ".GIT/notes.md", "sub/.git/notes.md", "src/value.go", "../page.md"} {
			for _, apply := range []bool{false, true} {
				if _, err := Maintain(root, page, a, apply); err == nil {
					t.Fatalf("unsafe target admitted: %s apply=%v", page, apply)
				}
			}
			if _, err := applyCorpusPage(root, page, []byte("before"), []byte("after"), nil); err == nil {
				t.Fatal("publication helper admitted unsafe target")
			}
		}
	})
}
