// SPDX-License-Identifier: Apache-2.0
package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

func releaseRun(t *testing.T, f fixture, arguments ...string) (int, string, string) {
	t.Helper()
	return runCLI(t, append([]string{"release", "--root", f.root, "--binaries", f.catalog}, arguments...)...)
}

// ARTIFACT-BIN-V0-523: one tag becomes a verified output directory holding the selected
// binaries and nothing else; a later tag for one binary carries the earlier records forward.
func TestReleaseTagBuildsOnlyTheSelectedBinariesAndVerifies(t *testing.T) {
	f := newFixture(t)
	output := filepath.Join(t.TempDir(), "core")
	code, stdout, stderr := releaseRun(t, f, "--tag", "core/v1.0.0", "--output", output)
	if code != 0 {
		t.Fatalf("release core/v1.0.0: exit %d: %s", code, stderr)
	}
	var run ReleaseRun
	if err := json.Unmarshal([]byte(stdout), &run); err != nil {
		t.Fatalf("release output %q: %v", stdout, err)
	}
	raw, err := os.ReadFile(filepath.Join(output, manifestName))
	if err != nil {
		t.Fatal(err)
	}
	if run.Schema != releaseRunSchema || run.Tag != "core/v1.0.0" || run.Scope != "set:core" || run.Version != "1.0.0" ||
		!slices.Equal(run.Binaries, []string{"corvint-alpha", "corvint-beta"}) || !slices.Equal(run.Rechecked, run.Binaries) ||
		run.ManifestSHA256 != digestOf(raw) {
		t.Fatalf("release run %+v (manifest %s)", run, digestOf(raw))
	}
	var names []string
	for _, entry := range listDirectory(t, output) {
		names = append(names, entry)
	}
	for _, name := range names {
		if strings.HasPrefix(name, "corvint-gamma") || strings.HasPrefix(name, "corvint-plain") || strings.HasPrefix(name, "corvint-tool") {
			t.Errorf("a core/ tag built %s", name)
		}
	}
	if !slices.Contains(names, "corvint-alpha.release-entry.json") || !slices.Contains(names, "corvint-beta.SHA256SUMS") || !slices.Contains(names, manifestName+".sha256") {
		t.Fatalf("output %v lacks the set's entries, checksums or manifest digest", names)
	}
	if code, _, stderr := runCLI(t, "verify", "--binaries", f.catalog, "--manifest", filepath.Join(output, manifestName), "--output", output); code != 0 {
		t.Fatalf("an independent verify of the release output refused: %s", stderr)
	}

	next := filepath.Join(t.TempDir(), "gamma")
	code, stdout, stderr = releaseRun(t, f, "--tag", "corvint-gamma/v2.1.0", "--output", next, "--base", filepath.Join(output, manifestName))
	if code != 0 {
		t.Fatalf("release corvint-gamma/v2.1.0: %s", stderr)
	}
	if err := json.Unmarshal([]byte(stdout), &run); err != nil {
		t.Fatal(err)
	}
	if run.Scope != "binary" || !slices.Equal(run.Binaries, []string{"corvint-gamma"}) || !slices.Equal(run.Rechecked, []string{"corvint-gamma"}) {
		t.Fatalf("one-binary release run %+v", run)
	}
	var manifest Manifest
	if err := readBoundedJSON(filepath.Join(next, manifestName), &manifest); err != nil {
		t.Fatal(err)
	}
	if manifest.Binaries["corvint-gamma"].Version != "2.1.0" || manifest.Binaries["corvint-alpha"].Version != "1.0.0" || len(manifest.Binaries) != 3 {
		t.Fatalf("manifest after a one-binary release: %+v", manifest.Binaries)
	}
	for _, name := range listDirectory(t, next) {
		if strings.HasPrefix(name, "corvint-alpha") || strings.HasPrefix(name, "corvint-beta") {
			t.Errorf("a one-binary tag rebuilt %s", name)
		}
	}
}

// ARTIFACT-BIN-V0-524: a non-empty output directory refuses before anything is built, and a
// failure after the first build leaves the directory as the run found it.
func TestReleaseTagRefusesNonEmptyOutputAndRollsBackOnFailure(t *testing.T) {
	f := newFixture(t)
	occupied := t.TempDir()
	if err := os.WriteFile(filepath.Join(occupied, "corvint-old.release-entry.json"), []byte("{}"), 0o644); err != nil {
		t.Fatal(err)
	}
	code, _, stderr := releaseRun(t, f, "--tag", "corvint-gamma/v1.0.0", "--output", occupied)
	requireRefusal(t, code, stderr, "output-not-empty")
	if got := listDirectory(t, occupied); !slices.Equal(got, []string{"corvint-old.release-entry.json"}) {
		t.Fatalf("a refused run changed the output: %v", got)
	}
	for tag, reason := range map[string]string{"corvint-tool/v1.0.0": "support-binary", "v1": "bad-tag"} {
		code, _, stderr := releaseRun(t, f, "--tag", tag, "--output", filepath.Join(t.TempDir(), "out"))
		requireRefusal(t, code, stderr, reason)
	}
	code, _, stderr = releaseRun(t, f, "--tag", "v1.0.0", "--output", filepath.Join(t.TempDir(), "out"), "--host-only")
	requireRefusal(t, code, stderr, "--host-only")

	// corvint-plain joins the core set without `stamp: none`, so it refuses stamp-unbound after
	// corvint-alpha and corvint-beta are already written; every file of the run is removed.
	raw, err := os.ReadFile(f.catalog)
	if err != nil {
		t.Fatal(err)
	}
	broken := strings.Replace(string(raw), `"role": "product", "stamp": "none"`, `"role": "product", "versionSet": "core"`, 1)
	if broken == string(raw) {
		t.Fatal("fixture catalog lacks the corvint-plain row")
	}
	if err := os.WriteFile(f.catalog, []byte(broken), 0o644); err != nil {
		t.Fatal(err)
	}
	created := filepath.Join(t.TempDir(), "created")
	code, _, stderr = releaseRun(t, f, "--tag", "core/v1.0.0", "--output", created)
	requireRefusal(t, code, stderr, "stamp-unbound")
	if _, err := os.Stat(created); !os.IsNotExist(err) {
		t.Fatalf("a failed run left the directory it created: %v %v", err, listDirectoryIfPresent(created))
	}
	empty := t.TempDir()
	code, _, stderr = releaseRun(t, f, "--tag", "core/v1.0.0", "--output", empty)
	requireRefusal(t, code, stderr, "stamp-unbound")
	if got := listDirectory(t, empty); len(got) != 0 {
		t.Fatalf("a failed run left files in an existing empty directory: %v", got)
	}
}

func listDirectoryIfPresent(directory string) []string {
	entries, err := os.ReadDir(directory)
	if err != nil {
		return nil
	}
	var names []string
	for _, entry := range entries {
		names = append(names, entry.Name())
	}
	return names
}
