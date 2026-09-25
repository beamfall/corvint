package archive

import (
	"bytes"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
	"testing"

	"github.com/Beamfall/corvint/internal/tasks/fixture"
	"github.com/Beamfall/corvint/internal/tasks/intent"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

func TestTMV0022_AS36_StagingDirectorySwap(t *testing.T) {
	r, repo := repoWithStore(t)
	outside := fixture.TempDirOutside(t)
	staging := filepath.Join(outside, "stage")
	moved := filepath.Join(outside, "moved")
	target := filepath.Join(r.Root, "export-target")
	for _, p := range []string{staging, target} {
		if err := os.Mkdir(p, 0700); err != nil {
			t.Fatal(err)
		}
	}
	before := fixture.TreeSnapshot(t, r.Root)
	var out bytes.Buffer
	_, err := Export(ExportOptions{Repo: repo, Staging: staging, Stdout: &out,
		afterProbe: func() {
			if err := os.Rename(staging, moved); err != nil {
				t.Fatal(err)
			}
			if err := os.Symlink(target, staging); err != nil {
				t.Fatal(err)
			}
		},
		afterStaged: func(_ *os.File) {
			entries, err := os.ReadDir(target)
			if err != nil {
				t.Fatal(err)
			}
			if len(entries) != 0 {
				t.Errorf("staging pathname swap wrote %d entries inside repository", len(entries))
			}
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Verify(bytes.NewReader(out.Bytes())); err != nil {
		t.Fatal(err)
	}
	after := fixture.TreeSnapshot(t, r.Root)
	if !fixture.SameTree(before, after) {
		t.Fatal("repository changed")
	}
	entries, err := os.ReadDir(moved)
	if err != nil || len(entries) != 0 {
		t.Fatalf("retained directory cleanup: %v %v", entries, err)
	}
}

func TestTMV0022_AS36_StagingFileReplacement(t *testing.T) {
	_, repo := repoWithStore(t)
	staging := fixture.TempDirOutside(t)
	var out bytes.Buffer
	var original []byte
	var replacement string
	_, err := Export(ExportOptions{Repo: repo, Staging: staging, Stdout: &out,
		afterStaged: func(f *os.File) {
			entries, err := os.ReadDir(staging)
			if err != nil {
				t.Fatal(err)
			}
			if len(entries) != 0 {
				t.Errorf("staging remains pathname-addressable after writing")
			}
			// On the old boundary the linked file reveals its name. On the fixed
			// boundary only the retained descriptor knows the already unlinked name.
			name := ""
			if f != nil {
				name = filepath.Base(f.Name())
			} else if len(entries) == 1 {
				name = entries[0].Name()
			}
			if name == "" {
				t.Fatal("no staging name witness")
			}
			replacement = filepath.Join(staging, name)
		},
		afterVerified: func() {
			// Replacing the formerly used name must affect neither delivery nor cleanup.
			if err := os.Remove(replacement); err != nil && !os.IsNotExist(err) {
				t.Fatal(err)
			}
			original = []byte("replacement must survive and never be delivered")
			if err := os.WriteFile(replacement, original, 0600); err != nil {
				t.Fatal(err)
			}
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Verify(bytes.NewReader(out.Bytes())); err != nil {
		t.Errorf("delivery reopened replacement: %v", err)
	}
	raw, err := os.ReadFile(replacement)
	if err != nil || !bytes.Equal(raw, original) {
		t.Errorf("cleanup removed/replaced another file: %v", err)
	}
}

func TestTMV0022_AS36_StagingFailuresBeforeDelivery(t *testing.T) {
	for _, phase := range []string{"unlink", "verify_seek", "delivery_seek"} {
		t.Run(phase, func(t *testing.T) {
			_, repo := repoWithStore(t)
			staging := fixture.TempDirOutside(t)
			var out bytes.Buffer
			opts := ExportOptions{Repo: repo, Staging: staging, Stdout: &out}
			var f *os.File
			old := removeStaging
			t.Cleanup(func() { removeStaging = old })
			switch phase {
			case "unlink":
				removeStaging = func(root *os.Root, name string) error { return syscall.EIO }
			case "verify_seek":
				opts.afterStaged = func(file *os.File) {
					if err := file.Close(); err != nil {
						t.Fatal(err)
					}
				}
			case "delivery_seek":
				opts.afterStaged = func(file *os.File) { f = file }
				opts.afterVerified = func() {
					if err := f.Close(); err != nil {
						t.Fatal(err)
					}
				}
			}
			res, err := Export(opts)
			if res != nil || err == nil || out.Len() != 0 {
				t.Fatalf("%s: result=%v err=%v stdout=%d", phase, res != nil, err, out.Len())
			}
			if phase == "unlink" && !strings.Contains(err.Error(), syscall.EIO.Error()) {
				t.Fatalf("unlink error lost: %v", err)
			}
			if phase != "unlink" && (!strings.Contains(err.Error(), "cannot seek") || !strings.Contains(err.Error(), "cleanup failed")) {
				t.Fatalf("seek/close error lost: %v", err)
			}
		})
	}
}

func TestTMV0022_AS36_StagingCommonDirectoryCaseAlias(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skip("real Darwin case-alias witness NOT_RUN on this platform")
	}
	r, repo := repoWithStore(t)
	alias := filepath.Join(filepath.Dir(r.Root), strings.ToUpper(filepath.Base(r.Root)), ".git")
	if alias == repo.CommonDir {
		t.Fatal("fixture has no distinct case spelling")
	}
	source, err := os.Stat(repo.CommonDir)
	if err != nil {
		t.Fatal(err)
	}
	other, err := os.Stat(alias)
	if os.IsNotExist(err) {
		t.Skip("filesystem is case-sensitive; real case-alias witness NOT_RUN")
	}
	if err != nil {
		t.Fatal(err)
	}
	if !os.SameFile(source, other) {
		t.Fatal("case alias exists but names another directory")
	}
	linked := fixture.TempDirOutside(t)
	wtGit := filepath.Join(alias, "worktrees", "staging")
	fixture.Write(t, filepath.Join(wtGit, "commondir"), []byte("../..\n"))
	fixture.Write(t, filepath.Join(linked, ".git"), []byte("gitdir: "+wtGit+"\n"))
	resolved, err := intent.Resolve(linked)
	if err != nil {
		t.Fatal(err)
	}
	if resolved.CommonDir == repo.CommonDir {
		t.Fatal("resolver erased alias; no spelling counterexample")
	}
	before, linkedBefore := fixture.TreeSnapshot(t, r.Root), fixture.TreeSnapshot(t, linked)
	out, res, err := export(t, repo, linked)
	if code(err) != wire.CodeUnsupportedFilesystem || res != nil || len(out) != 0 {
		t.Errorf("same identity through case alias accepted: err=%v result=%v stdout=%d", err, res != nil, len(out))
	}
	if !fixture.SameTree(before, fixture.TreeSnapshot(t, r.Root)) || !fixture.SameTree(linkedBefore, fixture.TreeSnapshot(t, linked)) {
		t.Fatal("refusal changed a repository")
	}
	t.Log("real Darwin case alias exercised; filesystem identity agrees and path spellings differ")
}

func TestTMV0022_AS36_StagingUnlinkFailureSurvivesMovement(t *testing.T) {
	for _, scenario := range []string{"intent", "head", "intent+close", "head+close"} {
		t.Run(scenario, func(t *testing.T) {
			movement := strings.Split(scenario, "+")[0]
			r, repo := repoWithStore(t)
			staging := fixture.TempDirOutside(t)
			old, oldClose := removeStaging, closeStaging
			t.Cleanup(func() { removeStaging, closeStaging = old, oldClose })
			closeCalls := 0
			closeStaging = func(f *os.File) error {
				closeCalls++
				if err := oldClose(f); err != nil {
					t.Fatal(err)
				}
				if strings.HasSuffix(scenario, "+close") && closeCalls == 1 {
					return syscall.EBUSY
				}
				return nil
			}
			calls := 0
			removeStaging = func(root *os.Root, name string) error {
				calls++
				if calls != 1 {
					return old(root, name)
				}
				if movement == "head" {
					fixture.Commit(t, r, "MUTATION")
				} else {
					x := fixture.Ticket("A")
					x.Title = "edited during failed unlink"
					fixture.Write(t, filepath.Join(r.IntentDir, "tickets", "A.json"), x.Encode())
				}
				return syscall.EIO
			}
			out, res, err := export(t, repo, staging)
			if closeCalls != 1 {
				t.Errorf("failed staging descriptor close calls=%d", closeCalls)
			}
			if strings.HasSuffix(scenario, "+close") && (err == nil || !strings.Contains(err.Error(), syscall.EBUSY.Error())) {
				t.Errorf("associated close failure lost: %v", err)
			}
			entries, readErr := os.ReadDir(staging)
			if readErr != nil || len(entries) != 1 {
				t.Fatalf("one failed unlink witness: entries=%d err=%v", len(entries), readErr)
			}
			if res != nil || len(out) != 0 || err == nil || !strings.Contains(err.Error(), syscall.EIO.Error()) || !strings.Contains(err.Error(), "cleanup failed") {
				t.Fatalf("unlink failure lost after %s movement: result=%v stdout=%d err=%v unlinkCalls=%d", movement, res != nil, len(out), err, calls)
			}
		})
	}
}

func TestTMV0022_AS36_StagingIdentityInspectionFailsClosed(t *testing.T) {
	_, repo := repoWithStore(t)
	staging := fixture.TempDirOutside(t)
	fixture.Write(t, filepath.Join(staging, ".git", "HEAD"), []byte("ref: refs/heads/main\n"))
	base := fixture.TempDirOutside(t)
	file := filepath.Join(base, "file")
	fixture.Write(t, file, []byte("not a directory"))
	link := filepath.Join(base, "link")
	if err := os.Symlink(repo.CommonDir, link); err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{filepath.Join(base, "absent"), file, link, "/" + strings.Repeat("x", wire.MaxPathTextBytes)} {
		copy := *repo
		copy.CommonDir = path
		before := fixture.TreeSnapshot(t, staging)
		out, res, err := export(t, &copy, staging)
		if err == nil || res != nil || len(out) != 0 {
			t.Errorf("uninspectable identity accepted: result=%v stdout=%d err=%v", res != nil, len(out), err)
		}
		if !fixture.SameTree(before, fixture.TreeSnapshot(t, staging)) {
			t.Fatal("failed identity inspection wrote staging")
		}
	}
	// A distinct, valid external Git authority remains an ordinary staging option.
	out, res, err := export(t, repo, staging)
	if err != nil || res == nil || len(out) == 0 {
		t.Fatalf("external Git staging: result=%v stdout=%d err=%v", res != nil, len(out), err)
	}
}
