package stablefixture

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/cem/wire"
	"github.com/Beamfall/corvint/internal/cem/workflow"
)

// Change is one minimal verified cem/1.0 producer change, built by the real
// prepare and cite workflow, ready for Stabilize.
type Change struct {
	Root, Base, ProducerTarget string
	Producer                   []byte
	HunkID                     string
	// Git runs git in Root with automatic maintenance and gc disabled.
	Git func(args ...string) string
}

// NewChange builds a repository whose base holds docs/rule.txt and
// src/app.txt, whose target changes src/app.txt by one line, and whose
// cem/1.0 producer map cites docs/rule.txt line 1 as the specification of that hunk.
func NewChange(t testing.TB) Change {
	t.Helper()
	return NewChangeWithUnchanged(t, nil)
}

// NewChangeWithUnchanged is NewChange whose base also holds each path in
// unchanged with its content; the target leaves those files unchanged.
func NewChangeWithUnchanged(t testing.TB, unchanged map[string]string) Change {
	t.Helper()
	RequireStableHost(t)
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	home := t.TempDir()
	run := func(args ...string) string {
		t.Helper()
		command := exec.Command("git", append([]string{"-c", "maintenance.auto=false", "-c", "gc.auto=0"}, args...)...)
		command.Dir = root
		command.Env = append(os.Environ(),
			"GIT_CONFIG_NOSYSTEM=1", "HOME="+home, "XDG_CONFIG_HOME="+home,
			"GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@example.invalid",
			"GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@example.invalid",
			"GIT_AUTHOR_DATE=2000-01-01T00:00:00+0000", "GIT_COMMITTER_DATE=2000-01-01T00:00:00+0000")
		out, err := command.CombinedOutput()
		if err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
		return strings.TrimSpace(string(out))
	}
	run("init", "-q", "-b", "main")
	write(t, root, "docs/rule.txt", []byte("widget renderer authority\n"))
	write(t, root, "src/app.txt", []byte("alpha\n"))
	for path, content := range unchanged {
		write(t, root, path, []byte(content))
	}
	run("add", ".")
	run("commit", "-qm", "base")
	base := run("rev-parse", "HEAD")
	write(t, root, "src/app.txt", []byte("alpha\nwidget renderer\n"))
	run("add", ".")
	run("commit", "-qm", "target")
	target := run("rev-parse", "HEAD")
	ctx := context.Background()
	session, err := workflow.Open(root)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := session.Prepare(ctx, workflow.PrepareOptions{Base: base, Target: target}); err != nil {
		t.Fatal(err)
	}
	if session, err = workflow.Open(root); err != nil {
		t.Fatal(err)
	}
	if _, err := session.Cite(ctx, workflow.CiteOptions{
		MapPath: wire.ExcludedCEMPath, Hunk: "1", EvidencePath: "docs/rule.txt", Lines: "1:1", Relation: "specification",
	}); err != nil {
		t.Fatal(err)
	}
	producer, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(wire.ExcludedCEMPath)))
	if err != nil {
		t.Fatal(err)
	}
	var document struct {
		Hunks []struct {
			ID string `json:"id"`
		} `json:"hunks"`
	}
	if err := json.Unmarshal(producer, &document); err != nil || len(document.Hunks) != 1 {
		t.Fatalf("fixture expects one hunk: %v", err)
	}
	return Change{Root: root, Base: base, ProducerTarget: target, Producer: producer, HunkID: document.Hunks[0].ID, Git: run}
}

// Stable converts the change with Stabilize, linking its one hunk.
func (c Change) Stable(t testing.TB) Stable {
	t.Helper()
	return Stabilize(t, c.Root, c.Producer, c.HunkID, c.Git)
}
