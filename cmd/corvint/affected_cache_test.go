package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/Beamfall/corvint/internal/liveverify/affected"
)

// observeAffectedCache records every cache outcome and lets entries pin files
// the test just wrote. Tests that use it replace package state, so they do
// not run in parallel.
func observeAffectedCache(t *testing.T) *[]affected.CacheOutcome {
	t.Helper()
	if runtime.GOOS != "darwin" && runtime.GOOS != "linux" {
		t.Skip("the graph cache pins change times only on darwin and linux (AFP-V0-392)")
	}
	outcomes := &[]affected.CacheOutcome{}
	savedHook, savedWindow := affectedCacheObserved, affected.GraphCacheRacyWindow
	affectedCacheObserved = func(outcome affected.CacheOutcome) { *outcomes = append(*outcomes, outcome) }
	affected.GraphCacheRacyWindow = 0
	t.Cleanup(func() { affectedCacheObserved, affected.GraphCacheRacyWindow = savedHook, savedWindow })
	return outcomes
}

func affectedStatusSnapshot(t *testing.T, root string) string {
	t.Helper()
	return affectedGit(t, root, "status", "--porcelain", "--ignored", "--untracked-files=all")
}

// AFP-V0-390: --cache is an explicit caller-named directory outside the
// repository; every other placement or combination refuses before any read.
func TestAFPV0390CacheDirectoryMustBeOutsideTheRepository(t *testing.T) {
	t.Parallel()
	root := affectedFixtureRepository(t)
	outside := t.TempDir()
	link := filepath.Join(outside, "into-root")
	if err := os.Symlink(root, link); err != nil {
		t.Fatal(err)
	}
	before := affectedStatusSnapshot(t, root)
	refused := [][]string{
		{"--cache", root},
		{"--cache", filepath.Join(root, ".corvint", "cache")},
		{"--cache", filepath.Join(root, "core")},
		{"--cache", filepath.Join(link, "cache")},
		{"--cache", filepath.Join(outside, "elsewhere", ".git", "cache")},
		{"--cache", ""},
		{"--cache", outside, "--cache", outside},
		{"--cache", outside, "--snapshot", "receipt.json"},
		{"--cache", outside, "--playwright-config", "playwright.config.ts"},
		{"--cache", filepath.Join(outside, "elsewhere", ".GIT", "cache")},
	}
	// On a case-insensitive file system the root under another case is the
	// same directory, which only a same-file comparison catches.
	folded := filepath.Join(filepath.Dir(root), strings.ToUpper(filepath.Base(root)))
	if info, err := os.Stat(folded); err == nil {
		if rootInfo, _ := os.Stat(root); os.SameFile(info, rootInfo) {
			refused = append(refused, []string{"--cache", filepath.Join(folded, "cache")})
		}
	}
	for _, arguments := range refused {
		_, stdout, stderr, code := runAffectedCLI(t, root, arguments...)
		if code != 2 || len(stdout) != 0 || !strings.Contains(stderr, `"invalid-arguments"`) {
			t.Fatalf("%q must refuse with invalid-arguments and no stdout, got exit %d: %s", arguments, code, stderr)
		}
	}
	if after := affectedStatusSnapshot(t, root); after != before {
		t.Fatalf("a refused --cache changed the repository:\n%s\n%s", before, after)
	}
	if entries, _ := os.ReadDir(outside); len(entries) != 1 {
		t.Fatalf("a refused --cache wrote into the cache directory: %v", entries)
	}
}

// AFP-V0-393: every plan form writes the same bytes with no cache, with a
// cold cache and with a warm one, and a cached run leaves the repository's
// status, ignored files included, unchanged (GitHub #1601).
func TestAFPV0393ReceiptsAreByteIdenticalWithAndWithoutTheCache(t *testing.T) {
	outcomes := observeAffectedCache(t)
	root := affectedFixtureRepository(t)
	cache := filepath.Join(t.TempDir(), "cache")
	before := affectedStatusSnapshot(t, root)
	for _, form := range [][]string{
		nil,
		{"--full"},
		{"--path", "core/core.go"},
		{"--full", "--path", "core/core.go", "--path", "web/core.mjs"},
		{"--base", strings.TrimSpace(affectedGit(t, root, "rev-parse", "HEAD"))},
	} {
		_, want, stderr, code := runAffectedCLI(t, root, form...)
		if code != 0 {
			t.Fatalf("%q exit %d: %s", form, code, stderr)
		}
		for run := range 2 {
			*outcomes = nil
			_, got, stderr, code := runAffectedCLI(t, root, append(slices.Clone(form), "--cache", cache)...)
			if code != 0 || !bytes.Equal(got, want) {
				t.Fatalf("%q run %d with --cache: exit %d, stdout differs: %s\n%s\n%s", form, run, code, stderr, got, want)
			}
			if len(*outcomes) != 1 || !(*outcomes)[0].Hit && !(*outcomes)[0].Stored {
				t.Fatalf("%q run %d: every cached run must hit or store: %+v", form, run, *outcomes)
			}
		}
		if !(*outcomes)[0].Hit {
			t.Fatalf("%q: the repeated identical call must reuse the entry: %+v", form, *outcomes)
		}
	}
	if after := affectedStatusSnapshot(t, root); after != before {
		t.Fatalf("a cached run changed the repository status:\n%s\n%s", before, after)
	}
	// A dirty worktree changes what the graph read, so the entry misses and
	// the plan is the uncached plan of the dirty tree.
	corePath := filepath.Join(root, "core", "core.go")
	body, err := os.ReadFile(corePath)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(corePath, append(body, []byte("\n// edit\n")...), 0o644); err != nil {
		t.Fatal(err)
	}
	_, want, _, _ := runAffectedCLI(t, root)
	*outcomes = nil
	_, got, stderr, code := runAffectedCLI(t, root, "--cache", cache)
	if code != 0 || !bytes.Equal(got, want) {
		t.Fatalf("dirty tree with --cache: exit %d: %s\n%s\n%s", code, stderr, got, want)
	}
	if len(*outcomes) != 1 || (*outcomes)[0].Hit || (*outcomes)[0].Reason != "path changed: core/core.go" {
		t.Fatalf("an edited source must miss the entry: %+v", *outcomes)
	}
}

// AFP-V0-395: --paths-from batches many --path values into one plan, the
// plan of the same values given as --path, and refuses what --path refuses.
func TestAFPV0395PathsFromBatchesPathsIntoOnePlan(t *testing.T) {
	t.Parallel()
	root := affectedFixtureRepository(t)
	list := filepath.Join(t.TempDir(), "paths.txt")
	if err := os.WriteFile(list, []byte("web/core.mjs\n\ncore/core.go\ncore/core.go\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	_, want, stderr, code := runAffectedCLI(t, root, "--path", "core/core.go", "--path", "web/core.mjs")
	if code != 0 {
		t.Fatalf("--path exit %d: %s", code, stderr)
	}
	for _, form := range [][]string{{"--paths-from", list}, {"--paths-from=" + list, "--path", "web/core.mjs"}} {
		receipt, got, stderr, code := runAffectedCLI(t, root, form...)
		if code != 0 || !bytes.Equal(got, want) {
			t.Fatalf("%q: exit %d: %s\n%s\n%s", form, code, stderr, got, want)
		}
		if !slices.Equal(anyStrings(receipt["path_list"]), []string{"core/core.go", "web/core.mjs"}) {
			t.Fatalf("%q: path_list=%v", form, receipt["path_list"])
		}
	}
	directory := t.TempDir()
	write := func(name, body string) string {
		full := filepath.Join(directory, name)
		if err := os.WriteFile(full, []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
		return full
	}
	many := strings.Builder{}
	for index := range affectedMaxPathList + 1 {
		fmt.Fprintf(&many, "p%d.go\n", index)
	}
	for _, arguments := range [][]string{
		{"--paths-from", write("empty.txt", "\n\n")},
		{"--paths-from", write("absolute.txt", "core/core.go\n/etc/passwd\n")},
		{"--paths-from", write("parent.txt", "../core.go\n")},
		{"--paths-from", write("crlf.txt", "core/core.go\r\n")},
		{"--paths-from", write("bom.txt", "\ufeffcore/core.go\n")},
		{"--paths-from", write("many.txt", many.String())},
		{"--paths-from", write("oversized.txt", strings.Repeat("core/core.go\n", affectedPathsFromMaxBytes/len("core/core.go\n")+1))},
		{"--paths-from", filepath.Join(directory, "missing.txt")},
		{"--paths-from", directory},
		{"--paths-from", list, "--paths-from", list},
		{"--paths-from", list, "--base", strings.TrimSpace(affectedGit(t, root, "rev-parse", "HEAD"))},
		{"--paths-from", "-x"},
	} {
		_, stdout, stderr, code := runAffectedCLI(t, root, arguments...)
		if code != 2 || len(stdout) != 0 || !strings.Contains(stderr, `"invalid-arguments"`) {
			t.Fatalf("%q must refuse with invalid-arguments, got exit %d: %s", arguments, code, stderr)
		}
	}
}

// AFP-V0-396: the help names the batch form and the cache, so a caller that
// checks one path at a time knows to batch or to reuse state (GitHub #1601).
func TestAFPV0396HelpDocumentsTheBatchFormAndTheCache(t *testing.T) {
	t.Parallel()
	help := strings.Join(strings.Fields(helpText("affected")), " ")
	for _, want := range []string{
		"--paths-from FILE",
		"--cache DIR",
		"One call with many paths builds the graph once",
		"byte-identical",
		"outside the repository",
	} {
		if !strings.Contains(help, want) {
			t.Fatalf("affected help does not say %q", want)
		}
	}
}

// AFP-V0-397: on a generated Rails-shaped tree a repeated single-path call
// reuses the verified graph instead of reading every Ruby file again, and
// writes the cold call's bytes (GitHub #1601: calls shared nothing).
func TestAFPV0397RepeatedPathCallOnALargeRubyTreeReusesTheGraph(t *testing.T) {
	outcomes := observeAffectedCache(t)
	root := t.TempDir()
	files := map[string]string{
		"Gemfile":               "gem 'rails'\ngem 'rspec-rails'\n",
		"config/application.rb": "require 'rails/all'\nmodule App\n  class Application < Rails::Application\n  end\nend\n",
		"spec/spec_helper.rb":   "RSpec.configure do |config|\nend\n",
		"spec/rails_helper.rb":  "require 'spec_helper'\n",
	}
	for index := range 600 {
		files[fmt.Sprintf("app/models/thing%d.rb", index)] = fmt.Sprintf("class Thing%d < ApplicationRecord\n  def total\n    Thing%d.count\n  end\nend\n", index, (index+1)%600)
		files[fmt.Sprintf("spec/models/thing%d_spec.rb", index)] = fmt.Sprintf("require 'rails_helper'\n\nRSpec.describe Thing%d do\n  it 'counts' do\n    expect(Thing%d.new.total).to be >= 0\n  end\nend\n", index, index)
	}
	for relative, body := range files {
		full := filepath.Join(root, filepath.FromSlash(relative))
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	for _, arguments := range [][]string{
		{"init", "-q"}, {"config", "user.email", "corvint@example.test"}, {"config", "user.name", "Corvint Test"},
		{"config", "gc.auto", "0"}, {"add", "-A"}, {"commit", "-qm", "rails fixture"},
	} {
		affectedGit(t, root, arguments...)
	}
	// t.TempDir is 0777 less the umask, group-writable under umask 0002,
	// and the cache refuses a directory others may write (V1-3217).
	cache := t.TempDir()
	if err := os.Chmod(cache, 0o700); err != nil {
		t.Fatal(err)
	}
	started := time.Now()
	_, cold, stderr, code := runAffectedCLI(t, root, "--path", "app/models/thing5.rb", "--cache", cache)
	coldElapsed := time.Since(started)
	if code != 0 || len(*outcomes) != 1 || !(*outcomes)[0].Stored {
		t.Fatalf("cold call: exit %d %s %+v", code, stderr, *outcomes)
	}
	started = time.Now()
	_, warm, stderr, code := runAffectedCLI(t, root, "--path", "app/models/thing5.rb", "--cache", cache)
	warmElapsed := time.Since(started)
	if code != 0 || len(*outcomes) != 2 || !(*outcomes)[1].Hit || !bytes.Equal(cold, warm) {
		t.Fatalf("warm call must hit and write the cold bytes: exit %d %s %+v", code, stderr, *outcomes)
	}
	_, uncached, _, _ := runAffectedCLI(t, root, "--path", "app/models/thing5.rb")
	if !bytes.Equal(uncached, warm) {
		t.Fatal("the cached receipt differs from the uncached one")
	}
	t.Logf("1200 Ruby files: cold %v, warm %v", coldElapsed, warmElapsed)
}

// AFP-V0-390: a Git directory outside the worktree (`--separate-git-dir`) is
// repository state too, so --cache at, under or aliased to it refuses before
// anything is written there.
func TestAFPV0390CacheRefusesTheSeparateGitDirectory(t *testing.T) {
	t.Parallel()
	outside := t.TempDir()
	root, admin := filepath.Join(outside, "work"), filepath.Join(outside, "admin")
	if err := os.MkdirAll(filepath.Join(root, "core"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "core", "core.go"), []byte("package core\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "go.mod"), []byte("module example.test/work\n\ngo 1.22\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	for _, arguments := range [][]string{
		{"init", "-q", "--separate-git-dir=" + admin}, {"config", "user.email", "corvint@example.test"}, {"config", "user.name", "Corvint Test"},
		{"add", "-A"}, {"commit", "-qm", "fixture"},
	} {
		affectedGit(t, root, arguments...)
	}
	alias := filepath.Join(outside, "admin-alias")
	if err := os.Symlink(admin, alias); err != nil {
		t.Fatal(err)
	}
	snapshot := func() []string {
		names := []string{}
		_ = filepath.WalkDir(admin, func(current string, _ os.DirEntry, _ error) error {
			names = append(names, current)
			return nil
		})
		return names
	}
	before := snapshot()
	for _, cache := range []string{admin, filepath.Join(admin, "cache"), filepath.Join(alias, "cache"), filepath.Join(admin, "objects", "cache")} {
		_, stdout, stderr, code := runAffectedCLI(t, root, "--path", "core/core.go", "--cache", cache)
		if code != 2 || len(stdout) != 0 || !strings.Contains(stderr, `"invalid-arguments"`) || !strings.Contains(stderr, "Git directory") {
			t.Fatalf("--cache %s must refuse with invalid-arguments, got exit %d: %s", cache, code, stderr)
		}
	}
	if after := snapshot(); !slices.Equal(before, after) {
		t.Fatalf("a refused --cache changed the Git directory:\n%v\n%v", before, after)
	}
	if _, _, stderr, code := runAffectedCLI(t, root, "--path", "core/core.go", "--cache", filepath.Join(outside, "cache")); code != 0 {
		t.Fatalf("a cache beside the Git directory must be accepted: exit %d: %s", code, stderr)
	}
}

// AFP-V0-391: the program identity hashes the running executable. A pathname
// replaced at or after the process start names another build, so it yields no
// identity (no cache) rather than that build's hash; the running image itself
// (/proc/self/exe on linux) is hashed whatever its times.
func TestAFPV0391ProgramIdentityRefusesAReplacedExecutable(t *testing.T) {
	t.Parallel()
	if runtime.GOOS != "darwin" && runtime.GOOS != "linux" {
		t.Skip("the graph cache is used only on darwin and linux (AFP-V0-392)")
	}
	directory := t.TempDir()
	name := filepath.Join(directory, "corvint")
	if err := os.WriteFile(name, []byte("build A"), 0o755); err != nil {
		t.Fatal(err)
	}
	replacement := filepath.Join(directory, "corvint.new")
	if err := os.WriteFile(replacement, []byte("build B"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(replacement, name); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(name)
	if err != nil {
		t.Fatal(err)
	}
	replaced := affectedChangeTime(info)
	if replaced.Before(info.ModTime()) {
		replaced = info.ModTime()
	}
	sum := sha256.Sum256([]byte("build B"))
	want := "sha256:" + hex.EncodeToString(sum[:])
	if got := affectedFileIdentity(name, false, replaced); got != "" {
		t.Fatalf("a pathname replaced at the process start must yield no identity, got %s", got)
	}
	if got := affectedFileIdentity(name, false, replaced.Add(time.Second)); got != want {
		t.Fatalf("an executable installed before the start must hash to %s, got %s", want, got)
	}
	if got := affectedFileIdentity(name, true, replaced); got != want {
		t.Fatalf("the running image must hash whatever its times: %s", got)
	}
	if got := affectedFileIdentity(directory, false, replaced.Add(time.Second)); got != "" {
		t.Fatalf("a directory must yield no identity, got %s", got)
	}
	if executable, running := affectedExecutable(); runtime.GOOS == "linux" && (executable != "/proc/self/exe" || !running) {
		t.Fatalf("linux must hash the running image, got %q %v", executable, running)
	}
	if affectedProgramIdentity() == "" {
		t.Fatal("the test binary, built before it started, must have an identity")
	}
}

// AFP-V0-393: the forms the first parity test does not reach, --require-clean,
// --paths-from, the provider and app-map forms, and a cache directory that
// cannot be written, write the uncached bytes too.
func TestAFPV0393CacheParityAcrossTheRemainingForms(t *testing.T) {
	outcomes := observeAffectedCache(t)
	root, base, record := affectedSelectionRepository(t)
	list := filepath.Join(t.TempDir(), "paths.txt")
	if err := os.WriteFile(list, []byte("core/core.go\nweb/core.mjs\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	cache := filepath.Join(t.TempDir(), "cache")
	before := affectedStatusSnapshot(t, root)
	parity := func(cache string, form []string, wantHit bool) affected.CacheOutcome {
		t.Helper()
		_, want, stderr, code := runAffectedCLI(t, root, form...)
		if code != 0 {
			t.Fatalf("%q exit %d: %s", form, code, stderr)
		}
		*outcomes = nil
		_, got, stderr, code := runAffectedCLI(t, root, append(slices.Clone(form), "--cache", cache)...)
		if code != 0 || !bytes.Equal(got, want) {
			t.Fatalf("%q with --cache: exit %d, stdout differs: %s\n%s\n%s", form, code, stderr, got, want)
		}
		if len(*outcomes) != 1 || (*outcomes)[0].Hit != wantHit {
			t.Fatalf("%q: outcome %+v, want hit=%v", form, *outcomes, wantHit)
		}
		return (*outcomes)[0]
	}
	forms := [][]string{
		{"--require-clean"},
		{"--paths-from", list},
		{"--base", base, "--provider", record, "--selection-profile", "strict", "--full"},
		{"--base", base, "--provider", record, "--selection-profile", "coverage"},
	}
	for index, form := range forms {
		parity(cache, form, index > 0)
		parity(cache, form, true)
	}
	if after := affectedStatusSnapshot(t, root); after != before {
		t.Fatalf("a cached run changed the repository status:\n%s\n%s", before, after)
	}
	unwritable := t.TempDir()
	if err := os.Chmod(unwritable, 0o500); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(unwritable, 0o700) })
	for _, directory := range []string{unwritable, filepath.Join(unwritable, "nested")} {
		if outcome := parity(directory, forms[1], false); outcome.Stored || outcome.NotStored == "" {
			t.Fatalf("%s: an unwritable cache directory must fall back without storing: %+v", directory, outcome)
		}
	}
}

// AFP-V0-393: the app-map form writes the uncached bytes with a cold and a
// warm cache.
func TestAFPV0393CacheParityWithAnAppMapImpact(t *testing.T) {
	outcomes := observeAffectedCache(t)
	root := appmapCLIRepo(t)
	code, mapJSON, diag := runFlowsCLI(root, "appmap", "build", "--manifest", "appmap.json")
	if code != 0 {
		t.Fatal(diag)
	}
	directory := t.TempDir()
	mapFile := filepath.Join(directory, "map.json")
	if err := os.WriteFile(mapFile, []byte(mapJSON), 0o600); err != nil {
		t.Fatal(err)
	}
	base := strings.TrimSpace(affectedGit(t, root, "rev-parse", "HEAD"))
	routes, err := os.ReadFile(filepath.Join(root, "app", "routes.js"))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "app", "routes.js"), append(routes, '\n'), 0o644); err != nil {
		t.Fatal(err)
	}
	affectedGit(t, root, "add", "-A")
	affectedGit(t, root, "-c", "user.name=t", "-c", "user.email=t@example.invalid", "commit", "-qm", "routes")
	code, impact, diag := runFlowsCLI(root, "appmap", "impact", "--map", mapFile, "--base", base, "--full")
	if code != 0 {
		t.Fatal(diag)
	}
	impactFile := filepath.Join(directory, "impact.json")
	if err := os.WriteFile(impactFile, []byte(impact), 0o600); err != nil {
		t.Fatal(err)
	}
	form := []string{"--base", base, "--appmap-impact", impactFile}
	_, want, stderr, code := runAffectedCLI(t, root, form...)
	if code != 0 {
		t.Fatalf("exit %d: %s", code, stderr)
	}
	cache := filepath.Join(t.TempDir(), "cache")
	for run := range 2 {
		*outcomes = nil
		_, got, stderr, code := runAffectedCLI(t, root, append(slices.Clone(form), "--cache", cache)...)
		if code != 0 || !bytes.Equal(got, want) {
			t.Fatalf("run %d with --cache: exit %d, stdout differs: %s", run, code, stderr)
		}
		if len(*outcomes) != 1 || (*outcomes)[0].Hit != (run == 1) {
			t.Fatalf("run %d: outcome %+v", run, *outcomes)
		}
	}
}
