package gitauth

import (
	"context"
	"encoding/hex"
	"errors"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/cem/cemcode"
)

// literalTree writes a raw tree body that ordinary Git would refuse to build.
func literalTree(t *testing.T, root string, entries ...string) string {
	t.Helper()
	var body []byte
	for i := 0; i < len(entries); i += 2 {
		header, oid := entries[i], entries[i+1]
		raw, err := hex.DecodeString(oid)
		if err != nil {
			t.Fatal(err)
		}
		body = append(append(append(body, header...), 0), raw...)
	}
	command := exec.Command("git", "-c", "maintenance.auto=false", "-c", "gc.auto=0",
		"hash-object", "-t", "tree", "--literally", "-w", "--stdin")
	command.Dir = root
	command.Env = append(os.Environ(), "GIT_CONFIG_NOSYSTEM=1", "HOME="+t.TempDir(), "XDG_CONFIG_HOME="+t.TempDir())
	command.Stdin = strings.NewReader(string(body))
	out, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("hash-object: %v\n%s", err, out)
	}
	return strings.TrimSpace(string(out))
}

func malformedTreeRepo(t *testing.T) (string, string, string) {
	t.Helper()
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	gitCmd(t, root, "init", "-q", "-b", "main")
	writeFile(t, root, "real", "real\n")
	writeFile(t, root, "evil", "evil\n")
	real := gitCmd(t, root, "-c", "maintenance.auto=false", "-c", "gc.auto=0", "hash-object", "-w", "real")
	evil := gitCmd(t, root, "-c", "maintenance.auto=false", "-c", "gc.auto=0", "hash-object", "-w", "evil")
	commit := func(tree string) string {
		return gitCmd(t, root, "-c", "maintenance.auto=false", "-c", "gc.auto=0",
			"-c", "user.name=t", "-c", "user.email=t@example.invalid", "commit-tree", tree, "-m", "malformed")
	}
	// Slash variant: root lists a tree "a" holding b=real and a blob "a/b"=evil.
	a := literalTree(t, root, "100644 b", real)
	slash := commit(literalTree(t, root, "040000 a", a, "100644 a/b", evil))
	// Dot-dot variant: d/e lists ".." as a blob, which would rewrite "d".
	e := literalTree(t, root, "100644 ..", evil)
	d := literalTree(t, root, "040000 e", e, "100644 x", real)
	dotdot := commit(literalTree(t, root, "040000 d", d))
	return root, slash, dotdot
}

func requireMalformedTree(t *testing.T, label string, err error) {
	t.Helper()
	var coded *cemcode.Error
	if err == nil || errors.Is(err, fs.ErrNotExist) || !errors.As(err, &coded) || coded.Code != cemcode.RepositoryObjectUnavailable {
		t.Fatalf("%s: want fail-closed %s, got %v", label, cemcode.RepositoryObjectUnavailable, err)
	}
}

func TestRevisionFSRefusesUnsafeTreeEntryNames(t *testing.T) {
	t.Run("DLT-V0-002 unsafe tree entry names fail closed regardless of read order", func(t *testing.T) {
		root, slash, dotdot := malformedTreeRepo(t)
		ctx := context.Background()
		r := open(t, root)
		source := func(commit string) fs.FS {
			s, err := r.RevisionFS(ctx, commit, 4<<20)
			if err != nil {
				t.Fatal(err)
			}
			return s
		}
		direct := source(slash)
		body, err := fs.ReadFile(direct, "a/b")
		requireMalformedTree(t, "direct a/b read "+string(body), err)
		statFirst := source(slash)
		_, statErr := fs.Stat(statFirst, "a")
		body, err = fs.ReadFile(statFirst, "a/b")
		requireMalformedTree(t, "stat a", statErr)
		requireMalformedTree(t, "a/b after stat "+string(body), err)

		up := source(dotdot)
		_, err = fs.ReadDir(up, "d/e")
		requireMalformedTree(t, "list d/e", err)
		if info, err := fs.Stat(up, "d"); err != nil || !info.IsDir() {
			t.Fatalf("d rewritten by d/e/..: %v %v", info, err)
		}
		_, _, err = r.LookupTreeEntry(ctx, dotdot, "d/e/x")
		requireMalformedTree(t, "lookup below d/e", err)
	})
}
