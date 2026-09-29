//go:build darwin || linux

package gitstatus

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestOpaqueGitlinksPrivateStatus(t *testing.T) {
	t.Run("EAF-V0-007", func(t *testing.T) {
		// EAF-V0-007: only gitlink OIDs, never nested contents, contribute to status.
		for _, format := range []string{"v1", "v2"} {
			t.Run(format, func(t *testing.T) {
				root := fixture(t)
				child := filepath.Join(root, "vendor space\tchild")
				if err := os.Mkdir(child, 0700); err != nil {
					t.Fatal(err)
				}
				gitTest(t, child, "init", "-q")
				gitTest(t, child, "-c", "user.name=Test", "-c", "user.email=test@example.invalid", "commit", "--allow-empty", "-qm", "child")
				oid := strings.TrimSpace(string(gitTest(t, child, "rev-parse", "HEAD")))
				gitTest(t, root, "update-index", "--add", "--cacheinfo", "160000,"+oid+",vendor space\tchild")
				compare := func() {
					t.Helper()
					args := []string{"status", "--porcelain=" + format, "-z", "--untracked-files=all", "--ignored=no", "--no-renames"}
					want := gitTest(t, root, append(args, "--ignore-submodules=dirty")...)
					got, err := Status(context.Background(), root, metadataLimit, testRun, args...)
					if err != nil || !bytes.Equal(got, want) {
						t.Fatalf("status=%q want=%q err=%v", got, want, err)
					}
				}
				compare() // staged addition
				gitTest(t, root, "commit", "-qm", "gitlink")
				compare() // clean
				writeTest(t, filepath.Join(child, "untracked"), "outside coverage")
				compare()
				gitTest(t, child, "-c", "user.name=Test", "-c", "user.email=test@example.invalid", "commit", "--allow-empty", "-qm", "new child")
				compare() // checked-out commit differs
				gitTest(t, root, "add", "vendor space\tchild")
				compare() // staged OID change
				gitTest(t, root, "commit", "-qm", "new gitlink")
				gitTest(t, root, "update-index", "--add", "--cacheinfo", "160000,"+oid+",other")
				if err := os.Mkdir(filepath.Join(root, "other"), 0700); err != nil {
					t.Fatal(err)
				}
				gitTest(t, root, "commit", "-qm", "second gitlink")
				gitTest(t, root, "update-index", "--force-remove", "other")
				compare() // staged deletion while another link remains
				if err := os.RemoveAll(child); err != nil {
					t.Fatal(err)
				}
				compare() // missing checkout
				if err := os.Mkdir(child, 0700); err != nil {
					t.Fatal(err)
				}
				compare() // uninitialized checkout
				if err := os.Remove(child); err != nil {
					t.Fatal(err)
				}
				writeTest(t, child, "replacement")
				compare() // unstaged type change
				gitTest(t, root, "add", "vendor space\tchild")
				compare() // staged type change, no live gitlinks
			})
		}
	})
}

func TestOpaqueGitlinkHostileMetadata(t *testing.T) {
	for _, kind := range []string{"filter", "head-symlink", "marker-symlink", "ref-cycle", "absorbed"} {
		t.Run(kind, func(t *testing.T) {
			root := fixture(t)
			child := filepath.Join(root, "child")
			if err := os.Mkdir(child, 0700); err != nil {
				t.Fatal(err)
			}
			gitTest(t, child, "init", "-q")
			gitTest(t, child, "-c", "user.name=Test", "-c", "user.email=test@example.invalid", "commit", "--allow-empty", "-qm", "child")
			oid := strings.TrimSpace(string(gitTest(t, child, "rev-parse", "HEAD")))
			gitTest(t, root, "update-index", "--add", "--cacheinfo", "160000,"+oid+",child")
			gitTest(t, root, "commit", "-qm", "gitlink")
			marker := filepath.Join(child, ".git")
			sentinel := filepath.Join(t.TempDir(), "executed")
			switch kind {
			case "filter":
				writeTest(t, filepath.Join(child, ".gitattributes"), "* filter=hostile\n")
				gitTest(t, child, "config", "filter.hostile.clean", "touch "+sentinel)
				gitTest(t, child, "config", "core.fsmonitor", "touch "+sentinel)
				gitTest(t, child, "config", "include.path", "/definitely/not/read")
			case "head-symlink":
				if err := os.Remove(filepath.Join(marker, "HEAD")); err != nil {
					t.Fatal(err)
				}
				if err := os.Symlink(filepath.Join(root, ".git", "HEAD"), filepath.Join(marker, "HEAD")); err != nil {
					t.Fatal(err)
				}
			case "marker-symlink":
				if err := os.Rename(marker, marker+".real"); err != nil {
					t.Fatal(err)
				}
				if err := os.Symlink(marker+".real", marker); err != nil {
					t.Fatal(err)
				}
			case "ref-cycle":
				writeTest(t, filepath.Join(marker, "HEAD"), "ref: refs/heads/loop\n")
				writeTest(t, filepath.Join(marker, "refs", "heads", "loop"), "ref: refs/heads/loop\n")
			case "absorbed":
				target := filepath.Join(root, ".git", "modules", "child")
				if err := os.MkdirAll(filepath.Dir(target), 0700); err != nil {
					t.Fatal(err)
				}
				if err := os.Rename(marker, target); err != nil {
					t.Fatal(err)
				}
				writeTest(t, marker, "gitdir: ../.git/modules/child\n")
				gitTest(t, child, "pack-refs", "--all")
			}
			_, err := Status(context.Background(), root, metadataLimit, testRun, "status", "--porcelain=v1", "-z", "--untracked-files=all")
			if kind == "filter" || kind == "absorbed" {
				if err != nil {
					t.Fatal(err)
				}
			} else if err == nil {
				t.Fatal("unsafe metadata accepted")
			}
			if _, err := os.Stat(sentinel); !os.IsNotExist(err) {
				t.Fatalf("nested command ran: %v", err)
			}
		})
	}
}

func TestOpaqueGitlinkSHA256AndDrift(t *testing.T) {
	root := fixture(t, "--object-format=sha256")
	child := filepath.Join(root, "child")
	if err := os.Mkdir(child, 0700); err != nil {
		t.Fatal(err)
	}
	gitTest(t, child, "init", "-q", "--object-format=sha256")
	gitTest(t, child, "-c", "user.name=Test", "-c", "user.email=test@example.invalid", "commit", "--allow-empty", "-qm", "child")
	oid := strings.TrimSpace(string(gitTest(t, child, "rev-parse", "HEAD")))
	gitTest(t, root, "update-index", "--add", "--cacheinfo", "160000,"+oid+",child")
	gitTest(t, root, "commit", "-qm", "gitlink")
	args := []string{"status", "--porcelain=v2", "-z", "--untracked-files=all"}
	raw, err := Status(context.Background(), root, metadataLimit, testRun, args...)
	if err != nil || len(raw) != 0 {
		t.Fatalf("SHA256=%q %v", raw, err)
	}
	run := func(ctx context.Context, dir string, limit int, arguments ...string) ([]byte, error) {
		raw, err := testRun(ctx, dir, limit, arguments...)
		if strings.Contains(strings.Join(arguments, " "), "--ignore-submodules=all") {
			writeTest(t, filepath.Join(child, ".git", "HEAD"), strings.Repeat("1", 64)+"\n")
		}
		return raw, err
	}
	if _, err := Status(context.Background(), root, metadataLimit, run, args...); err != errDrift {
		t.Fatalf("metadata drift accepted: %v", err)
	}
}

func TestOpaqueGitlinkRefusesUnsupportedStatusShape(t *testing.T) {
	root := fixture(t)
	oid := strings.TrimSpace(string(gitTest(t, root, "rev-parse", "HEAD")))
	gitTest(t, root, "update-index", "--add", "--cacheinfo", "160000,"+oid+",child")
	for _, args := range [][]string{{"status", "--porcelain"}, {"status", "--porcelain=v1", "-z", "--", "child"}} {
		if _, err := Status(context.Background(), root, metadataLimit, testRun, args...); RefusalClass(err) != "submodule" {
			t.Fatalf("unsupported shape=%v", err)
		}
	}
}
