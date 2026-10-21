package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// TestStableEnvelopeAdversarialShapes is the portable twin of the native test
// of the same name (V1-0595 U2, CEM-V1-363..366): repository shapes the public
// cells do not name, each built with real Git and verified through the CLI.
// An empty code means the full result must equal the named cell's own
// expected result; otherwise the answer is UNSUPPORTED at stage repository
// with exactly that code. A recipe calls accepted to prove its fixture is the
// cell's accepted repository before it introduces the one defect under test.
// Both consumers pin the same table.
func TestStableEnvelopeAdversarialShapes(t *testing.T) {
	var doc struct {
		Cases []s0eCell `json:"cases"`
	}
	s0eJSON(t, filepath.Join("inherited", "s0e", "REPOSITORY-ENVELOPE.cells.json"), &doc)
	cells := map[string]s0eCell{}
	for _, c := range doc.Cases {
		cells[c.ID] = c
	}
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	type shape struct {
		id, cell, code string
		recipe         func(t *testing.T, B, R, W string, c s0eCell, files map[string][]byte, accepted func())
	}
	admin := func(R string) string { return filepath.Join(R, ".git", "worktrees", "wt") }
	symlink := func(t *testing.T, target, link string) {
		if err := os.Symlink(target, link); err != nil {
			t.Fatal(err)
		}
	}
	// externalAdmin moves the worktree's real administrative directory to
	// dir, rebases its relative commondir onto the common directory, keeps
	// its absolute back-pointer and writes marker into the worktree's .git.
	externalAdmin := func(t *testing.T, R, W, dir, marker string) {
		if err := os.MkdirAll(filepath.Dir(dir), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.Rename(admin(R), dir); err != nil {
			t.Fatal(err)
		}
		common, err := filepath.Rel(dir, filepath.Join(R, ".git"))
		if err != nil {
			t.Fatal(err)
		}
		s0eWrite(t, filepath.Join(dir, "commondir"), []byte(common+"\n"), 0o644)
		s0eWrite(t, filepath.Join(W, ".git"), []byte("gitdir: "+marker+"\n"), 0o644)
	}
	shapes := []shape{
		{"gitdir-outside-reciprocal", "linked-sha1", "", func(t *testing.T, B, R, W string, c s0eCell, files map[string][]byte, accepted func()) {
			outside := filepath.Join(B, "outside-admin")
			externalAdmin(t, R, W, outside, outside)
		}},
		{"gitdir-outside-no-backpointer", "linked-sha1", "repository-object-unavailable", func(t *testing.T, B, R, W string, c s0eCell, files map[string][]byte, accepted func()) {
			outside := filepath.Join(B, "outside-admin")
			externalAdmin(t, R, W, outside, outside)
			accepted()
			if err := os.Remove(filepath.Join(outside, "gitdir")); err != nil {
				t.Fatal(err)
			}
		}},
		{"gitdir-outside-foreign-worktree", "linked-sha1", "repository-object-unavailable", func(t *testing.T, B, R, W string, c s0eCell, files map[string][]byte, accepted func()) {
			other := filepath.Join(B, "other")
			s0eRepo(t, files, other, "sha1", c.Target)
			s0eGit(t, other, "worktree", "add", "-q", "--detach", filepath.Join(B, "other-wt"), c.Target)
			s0eWrite(t, filepath.Join(W, ".git"), []byte("gitdir: "+filepath.Join(other, ".git", "worktrees", "other-wt")+"\n"), 0o644)
		}},
		{"gitdir-relative-symlinked-ancestor", "linked-sha1", "repository-object-unavailable", func(t *testing.T, B, R, W string, c s0eCell, files map[string][]byte, accepted func()) {
			externalAdmin(t, R, W, filepath.Join(B, "real-outside", "admin"), "../real-outside/admin")
			accepted()
			symlink(t, "real-outside", filepath.Join(B, "link"))
			s0eWrite(t, filepath.Join(W, ".git"), []byte("gitdir: ../link/admin\n"), 0o644)
		}},
		{"commondir-symlink-loop", "linked-sha1", "repository-object-unavailable", func(t *testing.T, B, R, W string, c s0eCell, files map[string][]byte, accepted func()) {
			symlink(t, "loop", filepath.Join(admin(R), "loop"))
			s0eWrite(t, filepath.Join(admin(R), "commondir"), []byte("loop\n"), 0o644)
		}},
		{"commondir-symlink", "linked-sha1", "repository-object-unavailable", func(t *testing.T, B, R, W string, c s0eCell, files map[string][]byte, accepted func()) {
			symlink(t, filepath.Join(R, ".git"), filepath.Join(admin(R), "common-link"))
			s0eWrite(t, filepath.Join(admin(R), "commondir"), []byte("common-link\n"), 0o644)
		}},
		{"commondir-file-symlink", "linked-sha1", "repository-object-unavailable", func(t *testing.T, B, R, W string, c s0eCell, files map[string][]byte, accepted func()) {
			s0eWrite(t, filepath.Join(B, "commondir-target"), []byte("../..\n"), 0o644)
			if err := os.Remove(filepath.Join(admin(R), "commondir")); err != nil {
				t.Fatal(err)
			}
			symlink(t, filepath.Join(B, "commondir-target"), filepath.Join(admin(R), "commondir"))
		}},
		{"unknown-extension-v1", "ordinary-sha1", "git-read-failed", func(t *testing.T, B, R, W string, c s0eCell, files map[string][]byte, accepted func()) {
			s0eGit(t, R, "config", "core.repositoryformatversion", "1")
			s0eGit(t, R, "config", "extensions.corvintUnknownExtension", "true")
		}},
		{"unknown-extension-v0", "ordinary-sha1", "", func(t *testing.T, B, R, W string, c s0eCell, files map[string][]byte, accepted func()) {
			s0eGit(t, R, "config", "extensions.corvintUnknownExtension", "true")
		}},
		{"sha256-extensions-first-mixed-case", "ordinary-sha256", "", func(t *testing.T, B, R, W string, c s0eCell, files map[string][]byte, accepted func()) {
			s0eWrite(t, filepath.Join(R, ".git", "config"), []byte("[Extensions]\n\tObjectFormat = sha256\n[CoRe]\n\tRepositoryFormatVersion = 1\n\tFileMode = true\n\tBare = false\n"), 0o644)
		}},
		{"sha1-store-claims-sha256", "ordinary-sha1", "git-read-failed", func(t *testing.T, B, R, W string, c s0eCell, files map[string][]byte, accepted func()) {
			s0eGit(t, R, "config", "core.repositoryformatversion", "1")
			s0eGit(t, R, "config", "extensions.objectFormat", "sha256")
		}},
		{"sha256-store-claims-sha1", "ordinary-sha256", "git-read-failed", func(t *testing.T, B, R, W string, c s0eCell, files map[string][]byte, accepted func()) {
			s0eWrite(t, filepath.Join(R, ".git", "config"), []byte("[core]\n\trepositoryformatversion = 0\n\tbare = false\n"), 0o644)
		}},
	}
	for _, s := range shapes {
		t.Run(s.id, func(t *testing.T) {
			if !stableOwnerSupported() {
				t.Skip("process containment is not qualified on this platform")
			}
			s0eRequireBoundTuple(t)
			c, ok := cells[s.cell]
			if !ok {
				t.Fatalf("no envelope cell %s", s.cell)
			}
			files := s0eFixtures(t)
			B := s0eTemp(t)
			R := filepath.Join(B, "repo")
			s0eRepo(t, files, R, c.ObjectFormat, c.Target)
			arts := filepath.Join(B, "artifacts")
			s0eArtifacts(t, files, arts)
			mapPath := filepath.Join(B, "map.json")
			s0eWrite(t, mapPath, files["fixtures/"+c.Map], 0o644)
			repo := R
			if c.ID == "linked-sha1" {
				repo = filepath.Join(B, "wt")
				s0eGit(t, R, "worktree", "add", "-q", "--detach", repo, c.Target)
			}
			run := func() (stableResult, int, []byte) {
				cmd := exec.Command(exe, "verify-stable-canonical", "--repository", repo, "--map", mapPath,
					"--expected-base", c.ExpectedBase, "--target", c.Target, "--artifacts", arts)
				cmd.Env = []string{"PATH=" + os.Getenv("PATH"), "HOME=" + filepath.Join(B, "home"), "TMPDIR=" + os.Getenv("TMPDIR"), "CEM01_TEST_CLI=1"}
				var stdout bytes.Buffer
				cmd.Stdout = &stdout
				code := 0
				var exitErr *exec.ExitError
				if err := cmd.Run(); errors.As(err, &exitErr) {
					code = exitErr.ExitCode()
				} else if err != nil {
					t.Fatal(err)
				}
				var got stableResult
				if err := json.Unmarshal(stdout.Bytes(), &got); err != nil {
					t.Fatalf("stdout is not a result: %v\n%s", err, stdout.Bytes())
				}
				return got, code, stdout.Bytes()
			}
			s.recipe(t, B, R, repo, c, files, func() {
				got, code, out := run()
				if key := s0eCompare(got, code, s0eExpect(c.ExpectedResult), c.ExpectedExit); key != "" {
					t.Fatalf("precondition: %s fixture is not %s's accepted repository before the defect; first differing key %q\n%s", s.id, s.cell, key, out)
				}
			})
			got, code, out := run()
			gotCode := ""
			if got.Code != nil {
				gotCode = *got.Code
			}
			t.Logf("ENVELOPE-ADVERSARIAL %s exit=%d outcome=%s stage=%s code=%s", s.id, code, got.Outcome, got.Stage, gotCode)
			if s.code == "" {
				if key := s0eCompare(got, code, s0eExpect(c.ExpectedResult), c.ExpectedExit); key != "" {
					t.Errorf("first differing key %q\n%s", key, out)
				}
				return
			}
			if code != 2 || got.Outcome != "UNSUPPORTED" || got.Stage != "repository" || gotCode != s.code || got.Accept != nil {
				t.Errorf("got exit=%d outcome=%s stage=%s code=%q, want exit=2 UNSUPPORTED repository %q accept=null", code, got.Outcome, got.Stage, gotCode, s.code)
			}
		})
	}
}
