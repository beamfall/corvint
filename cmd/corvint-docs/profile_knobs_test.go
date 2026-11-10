// SPDX-License-Identifier: Apache-2.0

package main

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/doccorpus"
	"github.com/Beamfall/corvint/internal/procprofile"
)

func profileKnobLookup(env map[string]string) func(string) (string, bool) {
	return func(name string) (string, bool) {
		value, ok := env[name]
		return value, ok
	}
}

// profileCorpusFixture is a one-page committed repository with a built corpus artifact.
func profileCorpusFixture(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "source.md"), []byte("# Evidence\n\nOriginal source.\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	git := func(args ...string) string {
		t.Helper()
		c := fixtureGitCommand(append([]string{"-c", "user.name=t", "-c", "user.email=t@example.invalid"}, args...)...)
		c.Dir = root
		out, err := c.CombinedOutput()
		if err != nil {
			t.Fatalf("git %v: %v %s", args, err, out)
		}
		return strings.TrimSpace(string(out))
	}
	git("init", "-q")
	git("add", "source.md")
	git("commit", "-qm", "source")
	m, err := doccorpus.Inventory(context.Background(), root, git("rev-parse", "HEAD"), "source.md", "2026-10-23T00:00:00Z")
	if err != nil {
		t.Fatal(err)
	}
	a, err := doccorpus.Build(context.Background(), root, m)
	if err != nil {
		t.Fatal(err)
	}
	for name, value := range map[string]any{"corpus.json": a, "manifest.json": m} {
		data, err := doccorpus.Encode(value)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(root, name), data, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	// Keep the worktree clean so reads report fresh.
	if err := os.WriteFile(filepath.Join(root, ".git", "info", "exclude"), []byte("corpus.json\nmanifest.json\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	return root
}

// PRB-V0-001, PRB-V0-003, PRB-V0-008: every docs corpus command the issue names writes both
// profiles through main's single bracket, and the knobs never change a stdout byte or the exit
// status. The rows moved here from cmd/corvint with the verbs (decision 0594).
func TestPRBV0008EveryNamedDocsCommandWritesProfiles(t *testing.T) {
	root := profileCorpusFixture(t)
	corpus := func(op string, extra ...string) []string {
		return append([]string{"--root", root, "docs", "corpus", op}, extra...)
	}
	rows := map[string][]string{
		"docs corpus build":    corpus("build", "--manifest", "manifest.json"),
		"docs corpus info":     corpus("info", "--artifact", "corpus.json"),
		"docs corpus search":   corpus("search", "--artifact", "corpus.json", "--query", "Original"),
		"docs corpus get":      corpus("get", "--artifact", "corpus.json", "--id", "native:file:source.md"),
		"docs corpus flow":     corpus("flow", "--artifact", "corpus.json", "--id", "native:file:source.md"),
		"docs corpus coverage": corpus("coverage", "--artifact", "corpus.json"),
		"docs corpus locate":   corpus("locate", "--artifact", "corpus.json", "--path", "source.md"),
		"docs corpus delta":    corpus("delta", "--previous", "corpus.json", "--artifact", "corpus.json"),
		"help --json":          {"help", "--json"},
	}
	names := make([]string, 0, len(rows))
	for name := range rows {
		names = append(names, name)
	}
	slices.Sort(names)
	for _, name := range names {
		arguments := rows[name]
		dir := t.TempDir()
		cpu, mem := filepath.Join(dir, "cpu.pprof"), filepath.Join(dir, "heap.pprof")
		var profiled, plain, profiledErr, plainErr bytes.Buffer
		code := runProfiled(context.Background(), context.Background(), arguments, strings.NewReader(""), &profiled, &profiledErr, profileKnobLookup(map[string]string{"CORVINT_CPUPROFILE": cpu, "CORVINT_MEMPROFILE": mem}))
		for _, path := range []string{cpu, mem} {
			if err := procprofile.Verify(path); err != nil {
				t.Fatalf("%s: %v", name, err)
			}
		}
		if code != 0 {
			t.Fatalf("%s: exit %d stderr %.300s", name, code, profiledErr.String())
		}
		plainCode := runProfiled(context.Background(), context.Background(), arguments, strings.NewReader(""), &plain, &plainErr, profileKnobLookup(nil))
		if code != plainCode || !bytes.Equal(profiled.Bytes(), plain.Bytes()) {
			t.Fatalf("%s: knobs changed the result: exit %d vs %d, stdout %d vs %d bytes", name, code, plainCode, profiled.Len(), plain.Len())
		}
		if entries, _ := os.ReadDir(dir); len(entries) != 2 {
			t.Fatalf("%s: unprofiled run wrote into the profile directory: %v", name, entries)
		}
	}
}
