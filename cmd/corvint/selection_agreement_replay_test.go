// SPDX-License-Identifier: Apache-2.0

package main

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// The V1-4361 change (PR #74): base ea09e3db5 and head 2f4447b3c, nine files
// across internal/gateselect, internal/sessiontest, two specs and a build-log
// entry. On 2026-11-03 `corvint test --profile ci` selected 24 Go packages this
// range's affected plan did not (V1-4363).
const (
	v14361Base = "ea09e3db5565637336f482a308f18ae2b08f33b2"
	v14361Head = "2f4447b3ce53c30bd9e2ce35eb3280bc1fc6c103"
)

// AFP-V0-9732 (V1-4363): on the V1-4361 range, `corvint test --dry-run` runs
// exactly the Go packages `corvint affected --base` selects, no extra and no
// omission (AFP-V0-2402). The range is replayed in a private repository built
// from this repository's own history, so the test runs only where both
// commits are present; elsewhere it is skipped and the replay stays NOT_RUN.
func TestV14361RangeAffectedAndTestSelectTheSameGoPackages_AFPV09732(t *testing.T) {
	if testing.Short() {
		t.Skip("the historical replay builds two whole-repository graphs")
	}
	gitExecutable, err := exec.LookPath("git")
	if err != nil {
		t.Skip("git is not on PATH")
	}
	source, err := exec.Command(gitExecutable, "rev-parse", "--show-toplevel").Output()
	if err != nil {
		t.Skip("not inside a Git checkout")
	}
	history := strings.TrimSpace(string(source))
	for _, commit := range []string{v14361Base, v14361Head} {
		if exec.Command(gitExecutable, "-C", history, "cat-file", "-e", commit+"^{commit}").Run() != nil {
			t.Skipf("commit %s is not in this checkout's history; the V1-4361 replay is NOT_RUN here", commit[:12])
		}
	}
	root := t.TempDir()
	for _, arguments := range [][]string{
		{"init", "-q"}, {"config", "user.email", "corvint@example.test"}, {"config", "user.name", "Corvint Test"},
		{"config", "maintenance.auto", "false"}, {"config", "gc.auto", "0"},
	} {
		affectedGit(t, root, arguments...)
	}
	var base string
	for _, commit := range []string{v14361Base, v14361Head} {
		replayTree(t, gitExecutable, history, commit, root)
		affectedGit(t, root, "add", "-A")
		affectedGit(t, root, "commit", "-qm", "replay "+commit[:12])
		if commit == v14361Base {
			base = strings.TrimSpace(affectedGit(t, root, "rev-parse", "HEAD"))
		}
	}
	changed := strings.Fields(affectedGit(t, root, "diff", "--name-only", base, "HEAD"))
	if len(changed) != 9 {
		t.Fatalf("the replayed range changes %d paths, the V1-4361 diff has 9: %v", len(changed), changed)
	}

	plan, _, stderr, code := runAffectedCLI(t, root, "--base", base, "--accept-stale-binary")
	if code != 0 {
		t.Fatalf("affected exit %d: %s", code, stderr)
	}
	var planned []string
	for _, raw := range plan["plan"].(map[string]any)["selected"].([]any) {
		if id, _ := raw.(map[string]any)["unitId"].(string); strings.HasPrefix(id, "go:") {
			planned = append(planned, id)
		}
	}
	manifestPath := filepath.Join(t.TempDir(), "manifest.json")
	var stdout, testStderr bytes.Buffer
	code = runContext(context.Background(), []string{"--root", root, "test", "--base", base, "--dry-run", "--manifest", manifestPath, "--accept-stale-binary"}, strings.NewReader(""), &stdout, &testStderr)
	if code != 0 {
		t.Fatalf("test --dry-run exit %d:\n%s%s", code, stdout.String(), testStderr.String())
	}
	raw, err := os.ReadFile(manifestPath)
	if err != nil {
		t.Fatal(err)
	}
	var manifest struct {
		Scope string `json:"scope"`
		Units []struct {
			Unit string `json:"unit"`
		} `json:"units"`
	}
	if err := json.Unmarshal(raw, &manifest); err != nil {
		t.Fatal(err)
	}
	var run []string
	for _, unit := range manifest.Units {
		if strings.HasPrefix(unit.Unit, "go:") {
			run = append(run, unit.Unit)
		}
	}
	slices.Sort(planned)
	slices.Sort(run)
	if manifest.Scope != "NARROWED" || len(planned) == 0 {
		t.Fatalf("the replay is not a narrowed Go selection: scope %s, %d planned", manifest.Scope, len(planned))
	}
	if !slices.Equal(planned, run) {
		var extra, omitted []string
		for _, id := range run {
			if !slices.Contains(planned, id) {
				extra = append(extra, id)
			}
		}
		for _, id := range planned {
			if !slices.Contains(run, id) {
				omitted = append(omitted, id)
			}
		}
		t.Fatalf("corvint test runs %d Go packages, affected selects %d (AFP-V0-2402): run but not selected %v; selected but not run %v", len(run), len(planned), extra, omitted)
	}
	// The 2026-11-03 observation: 24 extras against a 91-unit plan. The
	// pinned range must stay far below that plan size too, or the parity is
	// trivially a FULL run.
	if len(planned) > 200 {
		t.Fatalf("%d Go packages planned for a nine-file range; the replay no longer narrows", len(planned))
	}
}

// replayTree replaces root's tracked content with the tree of commit in
// history, through `git archive`, so the private repository's history holds
// exactly the replayed trees and nothing of this checkout's state.
func replayTree(t *testing.T, gitExecutable, history, commit, root string) {
	t.Helper()
	for _, entry := range mustReadDir(t, root) {
		if entry.Name() == ".git" {
			continue
		}
		if err := os.RemoveAll(filepath.Join(root, entry.Name())); err != nil {
			t.Fatal(err)
		}
	}
	archive := exec.Command(gitExecutable, "-C", history, "archive", "--format=tar", commit)
	extract := exec.Command("tar", "-x", "-f", "-", "-C", root)
	pipe, err := archive.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	extract.Stdin = pipe
	if err := archive.Start(); err != nil {
		t.Fatal(err)
	}
	if out, err := extract.CombinedOutput(); err != nil {
		t.Fatalf("tar: %v\n%s", err, out)
	}
	if err := archive.Wait(); err != nil {
		t.Fatalf("git archive %s: %v", commit[:12], err)
	}
}

func mustReadDir(t *testing.T, directory string) []os.DirEntry {
	t.Helper()
	entries, err := os.ReadDir(directory)
	if err != nil {
		t.Fatal(err)
	}
	return entries
}
