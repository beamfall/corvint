package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// runSelect runs one selection and returns its exit code and streams. --exclude
// is written only when the caller names a manifest, so "no --exclude" and "an
// empty --exclude" stay distinguishable.
func runSelect(t *testing.T, repo, output, partition string, limit int, exclude ...string) (int, string, string) {
	t.Helper()
	corvintGo := buildCorvint(t)
	arguments := []string{"select",
		"--repo", repo, "--name", "fixture", "--since", "1970-01-01", "--limit", strconv.Itoa(limit),
		"--seed", "fixture", "--partition", partition, "--corvint", corvintGo, "--output", output,
	}
	if len(exclude) > 0 {
		arguments = append(arguments, "--exclude", strings.Join(exclude, ","))
	}
	var stdout, stderr bytes.Buffer
	code := run(context.Background(), arguments, &stdout, &stderr)
	return code, stdout.String(), stderr.String()
}

// selectHeldout selects a heldout set that excludes the changes named by the
// given prior manifests, and returns the selector's own summary output.
func selectHeldout(t *testing.T, repo, output string, exclude ...string) (*manifest, string) {
	t.Helper()
	code, stdout, stderr := runSelect(t, repo, output, "heldout", 1, exclude...)
	if code != 0 {
		t.Fatalf("select failed: %s", stderr)
	}
	raw, err := os.ReadFile(filepath.Join(output, "tasks.json"))
	if err != nil {
		t.Fatal(err)
	}
	var set manifest
	if err := json.Unmarshal(raw, &set); err != nil {
		t.Fatal(err)
	}
	return &set, stdout
}

// qualifyingHistory extends the two-commit fixtureRepo with two more changes of
// the same shape, so three commits pass the rule and a one-commit pilot leaves a
// non-empty heldout pool to check disjointness against.
func qualifyingHistory(t *testing.T) string {
	t.Helper()
	root := fixtureRepo(t)
	for revision := 2; revision <= 3; revision++ {
		writeFixtureFile(t, root, "internal/play/play.go", revisionSource(revision))
		writeFixtureFile(t, root, "internal/play/play_test.go",
			fmt.Sprintf("package play\n\nimport \"testing\"\n\nfunc TestOne(t *testing.T) {}\n\nfunc TestResume%d(t *testing.T) {}\n", revision))
		writeFixtureFile(t, root, "README.md", fmt.Sprintf("beamfall fixture, revision %d\n", revision))
		for _, arguments := range [][]string{{"add", "-A"}, {"commit", "-q", "-m", fmt.Sprintf("change %d", revision)}} {
			if _, err := git(context.Background(), root, arguments...); err != nil {
				t.Fatalf("git %s: %v", strings.Join(arguments, " "), err)
			}
		}
	}
	return root
}

// revisionSource edits the same three padded functions the fixture's first
// change edits, so every revision differs from the last in exactly three hunks.
func revisionSource(revision int) string {
	source := baseSource
	for _, edit := range [][2]string{
		{"return 0\n", fmt.Sprintf("return %d\n", 10*revision)},
		{"return 1\n", fmt.Sprintf("return %d\n", 11*revision)},
		{"return 3\n", fmt.Sprintf("return %d\n", 33*revision)},
	} {
		source = strings.Replace(source, edit[0], edit[1], 1)
	}
	return source
}

func writeFixtureFile(t *testing.T, root, name, content string) {
	t.Helper()
	full := filepath.Join(root, filepath.FromSlash(name))
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

// excludeManifest writes a prior manifest naming exactly the given commits.
func excludeManifest(t *testing.T, commits ...string) string {
	t.Helper()
	prior := manifest{Partition: "pilot", Tasks: []task{}}
	for _, commit := range commits {
		prior.Tasks = append(prior.Tasks, task{Commit: commit})
	}
	raw, err := json.Marshal(prior)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "tasks.json")
	if err := os.WriteFile(path, raw, 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func repoCommits(t *testing.T, repo string) []string {
	t.Helper()
	raw, err := git(context.Background(), repo, "log", "--format=%H")
	if err != nil {
		t.Fatal(err)
	}
	return strings.Fields(raw)
}

// CRT-V0-009: the pilot's changes are excluded from the heldout population, the
// heldout set is non-empty and disjoint, and its population is the qualifying
// count less the exclusions.
func TestExcludedPilotChangesAreDroppedAndCounted(t *testing.T) {
	repo := qualifyingHistory(t)
	pilot := t.TempDir()
	full := selectPilot(t, repo, pilot)
	if full.Population < 3 {
		t.Fatalf("the fixture offers %d qualifying changes, want at least 3", full.Population)
	}
	observed := full.Tasks[0].Commit

	set, summary := selectHeldout(t, repo, t.TempDir(), filepath.Join(pilot, "tasks.json"))
	if len(set.Tasks) == 0 {
		t.Fatal("the heldout selection is empty, so it never tested disjointness")
	}
	for _, item := range set.Tasks {
		if item.Commit == observed {
			t.Fatalf("the heldout set re-presented the pilot change %s", observed)
		}
	}
	if set.Population != full.Population-1 {
		t.Fatalf("heldout population %d, want %d qualifying less the 1 excluded", set.Population, full.Population)
	}
	if !strings.Contains(summary, "excluded 1,") {
		t.Fatalf("the selector did not report the excluded count: %s", summary)
	}
}

// CRT-V0-009: an exclude file that cannot supply identifiers is refused, so a
// silent no-op can never be mistaken for a disjoint selection.
func TestExcludeFileWithoutIdentifiersIsRefused(t *testing.T) {
	directory := t.TempDir()
	empty := filepath.Join(directory, "tasks.json")
	if err := os.WriteFile(empty, []byte(`{"partition":"pilot","tasks":[]}`), 0o644); err != nil {
		t.Fatal(err)
	}
	missing := filepath.Join(directory, "absent.json")
	for _, path := range []string{empty, missing} {
		_, err := excludedCommits([]string{path})
		var typed *excludeError
		if !errors.As(err, &typed) {
			t.Fatalf("%s: expected an excludeError, got %v", path, err)
		}
	}
	var stdout, stderr bytes.Buffer
	code := run(context.Background(), []string{"select",
		"--repo", directory, "--seed", "fixture", "--corvint", "corvint",
		"--exclude", empty, "--output", filepath.Join(directory, "out"),
	}, &stdout, &stderr)
	if code == 0 || !strings.Contains(stderr.String(), "--exclude") {
		t.Fatalf("select accepted an exclude file naming no change: %d %s", code, stderr.String())
	}
}

// CRT-V0-009: an identifier that is not a full lowercase object name would
// compare unequal to every candidate and drop nothing, so it is refused rather
// than counted by the named > 0 guard.
func TestExcludeIdentifiersMustBeFullLowercaseHex(t *testing.T) {
	full := "49a0484bdfc396f2abaaac5bb989c16447cc8c2e"
	for name, commit := range map[string]string{
		"uppercase":   strings.ToUpper(full),
		"abbreviated": full[:12],
		"absent":      "",
		"not hex":     strings.Repeat("z", 40),
	} {
		_, err := excludedCommits([]string{excludeManifest(t, commit)})
		var typed *excludeError
		if !errors.As(err, &typed) {
			t.Fatalf("%s identifier: expected an excludeError, got %v", name, err)
		}
	}
	if _, err := excludedCommits([]string{excludeManifest(t, full)}); err != nil {
		t.Fatalf("a full lowercase object name was refused: %v", err)
	}
}

// CRT-V0-009: a manifest from another or a rebased repository drops nothing;
// the run is refused instead of reporting "excluded 0" and exiting 0.
func TestExcludeThatDropsNothingIsRefused(t *testing.T) {
	repo := qualifyingHistory(t)
	foreign := excludeManifest(t, strings.Repeat("a", 40))
	code, _, stderr := runSelect(t, repo, t.TempDir(), "heldout", 1, foreign)
	if code == 0 || !strings.Contains(stderr, "none of its 1 changes") {
		t.Fatalf("an exclude that bit nothing was accepted: %d %s", code, stderr)
	}
}

// CRT-V0-009: exclusion that empties the pool is refused, and a pool short of
// --limit is reported rather than quietly under-filling the manifest.
func TestEmptyPoolIsRefusedAndAShortPoolWarns(t *testing.T) {
	repo := qualifyingHistory(t)
	everything := excludeManifest(t, repoCommits(t, repo)...)
	code, _, stderr := runSelect(t, repo, t.TempDir(), "heldout", 1, everything)
	if code == 0 || !strings.Contains(stderr, "no candidate change remains") {
		t.Fatalf("an emptied pool produced a manifest: %d %s", code, stderr)
	}
	code, _, stderr = runSelect(t, repo, t.TempDir(), "pilot", 30)
	if code != 0 || !strings.Contains(stderr, "short of --limit 30") {
		t.Fatalf("a pool short of --limit was not reported: %d %s", code, stderr)
	}
}

// CRT-V0-009: an --exclude the caller wrote but left empty is a mistake, not a
// request to exclude nothing.
func TestExplicitlyEmptyExcludeListIsRefused(t *testing.T) {
	base := []string{"--repo", "repo", "--seed", "fixture", "--corvint", "corvint", "--output", "out"}
	for _, value := range []string{"", ",", " , "} {
		if _, err := parseSelectOptions(append(append([]string{}, base...), "--exclude", value)); err == nil {
			t.Fatalf("--exclude %q was accepted as an empty list", value)
		}
	}
	if _, err := parseSelectOptions(base); err != nil {
		t.Fatalf("omitting --exclude was refused: %v", err)
	}
}
