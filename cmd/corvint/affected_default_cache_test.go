package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// useAffectedUserCacheDir points the default graph cache at base (or at the
// failure err) for one test, never at the real user cache directory. Tests
// that use it replace package state, so they do not run in parallel.
func useAffectedUserCacheDir(t *testing.T, base string, err error) {
	t.Helper()
	saved := affectedUserCacheDir
	affectedUserCacheDir = func() (string, error) { return base, err }
	t.Cleanup(func() { affectedUserCacheDir = saved })
}

// affectedDefaultEntries lists the graph cache entries in dir.
func affectedDefaultEntries(t *testing.T, dir string) []string {
	t.Helper()
	entries, err := filepath.Glob(filepath.Join(dir, "affected-graph-*.json"))
	if err != nil {
		t.Fatal(err)
	}
	return entries
}

func resolvedTempDir(t *testing.T) string {
	t.Helper()
	dir, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	return dir
}

// AFP-V0-436, AFP-V0-438: without --cache or --no-cache the graph cache is
// os.UserCacheDir()/corvint/affected, created private to the user, one entry
// per root; a cold and a warm call print exactly the --no-cache bytes.
func TestAFPV0436DefaultCacheLivesUnderTheUserCacheDirectory(t *testing.T) {
	outcomes := observeAffectedCache(t)
	base := resolvedTempDir(t)
	useAffectedUserCacheDir(t, base, nil)
	root := affectedFixtureRepository(t)
	location := filepath.Join(base, "corvint", "affected")
	before := affectedStatusSnapshot(t, root)
	for _, form := range [][]string{
		nil,
		{"--full"},
		{"--path", "core/core.go"},
		{"--base", strings.TrimSpace(affectedGit(t, root, "rev-parse", "HEAD"))},
	} {
		*outcomes = nil
		_, want, stderr, code := runAffectedCLI(t, root, append(slices.Clone(form), "--no-cache")...)
		if code != 0 || len(*outcomes) != 0 {
			t.Fatalf("%q --no-cache: exit %d, outcomes %+v: %s", form, code, *outcomes, stderr)
		}
		for run := range 2 {
			*outcomes = nil
			_, got, stderr, code := runAffectedCLI(t, root, form...)
			if code != 0 || stderr != "" || !bytes.Equal(got, want) {
				t.Fatalf("%q default run %d: exit %d, stdout differs: %s\n%s\n%s", form, run, code, stderr, got, want)
			}
			if len(*outcomes) != 1 || !(*outcomes)[0].Hit && !(*outcomes)[0].Stored {
				t.Fatalf("%q default run %d must hit or store: %+v", form, run, *outcomes)
			}
		}
		if !(*outcomes)[0].Hit {
			t.Fatalf("%q: the repeated call must reuse the default entry: %+v", form, *outcomes)
		}
	}
	if entries := affectedDefaultEntries(t, location); len(entries) != 1 {
		t.Fatalf("the default cache must hold one entry for one root, got %q", entries)
	}
	for _, dir := range []string{filepath.Join(base, "corvint"), location} {
		info, err := os.Stat(dir)
		if err != nil || info.Mode().Perm() != 0o700 {
			t.Fatalf("%s must be created with mode 0700: %v %v", dir, info, err)
		}
	}
	if after := affectedStatusSnapshot(t, root); after != before {
		t.Fatalf("a default-cached run changed the repository status:\n%s\n%s", before, after)
	}
	// Only the plan forms use the default: a snapshot, Playwright, --no-cache
	// or status invocation resolves no cache location.
	for _, invocation := range []affectedInvocation{
		{Root: root, Snapshot: "snapshot.json"},
		{Root: root, PlaywrightConfig: "playwright.config.ts"},
		{Root: root, NoCache: true},
		{Root: root, CacheStatus: true},
	} {
		if err := resolveAffectedBatchAndCache(&invocation); err != nil || invocation.Cache != "" {
			t.Fatalf("%+v must resolve no cache: %q %v", invocation, invocation.Cache, err)
		}
	}
	plan := affectedInvocation{Root: root}
	if err := resolveAffectedBatchAndCache(&plan); err != nil || plan.Cache != location {
		t.Fatalf("a plan must resolve the default location %s: %q %v", location, plan.Cache, err)
	}
}

// AFP-V0-437: --cache DIR replaces the default location and --no-cache turns
// the cache off; the two cannot be combined and --no-cache takes no value.
func TestAFPV0437NoCacheAndCacheOverrideTheDefault(t *testing.T) {
	outcomes := observeAffectedCache(t)
	base := resolvedTempDir(t)
	useAffectedUserCacheDir(t, base, nil)
	root := affectedFixtureRepository(t)
	_, want, stderr, code := runAffectedCLI(t, root, "--no-cache", "--path", "core/core.go")
	if code != 0 || len(*outcomes) != 0 {
		t.Fatalf("--no-cache: exit %d, outcomes %+v: %s", code, *outcomes, stderr)
	}
	override := filepath.Join(t.TempDir(), "override")
	for run := range 2 {
		*outcomes = nil
		_, got, stderr, code := runAffectedCLI(t, root, "--path", "core/core.go", "--cache", override)
		if code != 0 || !bytes.Equal(got, want) || len(*outcomes) != 1 || (run == 1) != (*outcomes)[0].Hit {
			t.Fatalf("--cache run %d: exit %d, outcomes %+v: %s\n%s\n%s", run, code, *outcomes, stderr, got, want)
		}
	}
	if entries := affectedDefaultEntries(t, override); len(entries) != 1 {
		t.Fatalf("--cache must hold the entry: %q", entries)
	}
	if _, err := os.Lstat(filepath.Join(base, "corvint")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("--no-cache and --cache must not touch the default location: %v", err)
	}
	for _, refused := range []struct {
		args    []string
		message string
	}{
		{[]string{"--cache", override, "--no-cache"}, "--cache and --no-cache cannot be combined"},
		{[]string{"--no-cache", "--no-cache"}, "argument --no-cache: does not accept a value or repetition"},
		{[]string{"--no-cache=1"}, "argument --no-cache: does not accept a value or repetition"},
	} {
		*outcomes = nil
		_, _, stderr, code := runAffectedCLI(t, root, refused.args...)
		if code != 2 || !strings.Contains(stderr, refused.message) || len(*outcomes) != 0 {
			t.Fatalf("%q: exit %d, outcomes %+v, stderr %s", refused.args, code, *outcomes, stderr)
		}
	}
}

// AFP-V0-438: a default location that cannot be named, is placed where
// --cache would be refused, or cannot be written is not an error: the call
// runs uncached and prints the --no-cache bytes with nothing on stderr.
func TestAFPV0438DefaultCacheDegradesToUncached(t *testing.T) {
	outcomes := observeAffectedCache(t)
	root := affectedFixtureRepository(t)
	_, want, stderr, code := runAffectedCLI(t, root, "--no-cache", "--path", "core/core.go")
	if code != 0 {
		t.Fatalf("--no-cache: exit %d: %s", code, stderr)
	}
	before := affectedStatusSnapshot(t, root)
	unwritable := resolvedTempDir(t)
	if err := os.Chmod(unwritable, 0o500); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(unwritable, 0o700) })
	blocked := resolvedTempDir(t)
	if err := os.WriteFile(filepath.Join(blocked, "corvint"), []byte("a file, not a directory"), 0o600); err != nil {
		t.Fatal(err)
	}
	for _, failure := range []struct {
		name string
		base string
		err  error
	}{
		{"no user cache directory", "", errors.New("$HOME is not defined")},
		{"empty", "", nil},
		{"relative", "relative/cache", nil},
		{"inside the repository", root, nil},
		{"inside the Git directory", filepath.Join(root, ".git"), nil},
		{"unwritable", unwritable, nil},
		{"component is a file", blocked, nil},
	} {
		useAffectedUserCacheDir(t, failure.base, failure.err)
		*outcomes = nil
		_, got, stderr, code := runAffectedCLI(t, root, "--path", "core/core.go")
		if code != 0 || stderr != "" || !bytes.Equal(got, want) {
			t.Fatalf("%s: exit %d, stderr %q, stdout differs:\n%s\n%s", failure.name, code, stderr, got, want)
		}
		for _, outcome := range *outcomes {
			if outcome.Hit || outcome.Stored {
				t.Fatalf("%s: an unusable default must not read or store an entry: %+v", failure.name, outcome)
			}
		}
	}
	if after := affectedStatusSnapshot(t, root); after != before {
		t.Fatalf("a refused default location changed the repository status:\n%s\n%s", before, after)
	}
}

func runAffectedCacheStatusCLI(t *testing.T, root string, options ...string) ([]byte, string, int) {
	t.Helper()
	var stdout, stderr bytes.Buffer
	code := runContext(context.Background(), append([]string{"--root", root, "affected", "cache"}, options...), strings.NewReader(""), &stdout, &stderr)
	return stdout.Bytes(), stderr.String(), code
}

// AFP-V0-439: `affected cache` reports the default cache's location, entry
// count and bytes as JSON or as a doctor plugin item, and creates nothing.
func TestAFPV0439CacheStatusReportsLocationAndSize(t *testing.T) {
	observeAffectedCache(t)
	base := resolvedTempDir(t)
	useAffectedUserCacheDir(t, base, nil)
	root := affectedFixtureRepository(t)
	location := filepath.Join(base, "corvint", "affected")
	status := func(want string) affectedCacheStatus {
		t.Helper()
		stdout, stderr, code := runAffectedCacheStatusCLI(t, root)
		var report affectedCacheStatus
		if code != 0 || stderr != "" || json.Unmarshal(stdout, &report) != nil {
			t.Fatalf("affected cache: exit %d: %s\n%s", code, stderr, stdout)
		}
		if report.Profile != "affected-cache-status/0" || !report.OK || report.Mutates || report.State != want {
			t.Fatalf("affected cache: want state %s, got %s", want, stdout)
		}
		return report
	}
	if report := status("ABSENT"); report.Location != location || report.Entries != 0 || report.Bytes != 0 {
		t.Fatalf("an absent cache: %+v", report)
	}
	if _, err := os.Lstat(filepath.Join(base, "corvint")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("the status call must not create the cache directory: %v", err)
	}
	if _, _, stderr, code := runAffectedCLI(t, root, "--path", "core/core.go"); code != 0 {
		t.Fatalf("default call: exit %d: %s", code, stderr)
	}
	entries := affectedDefaultEntries(t, location)
	if len(entries) != 1 {
		t.Fatalf("one entry: %q", entries)
	}
	info, err := os.Stat(entries[0])
	if err != nil {
		t.Fatal(err)
	}
	report := status("PRESENT")
	if report.Location != location || report.Entries != 1 || report.Bytes != info.Size() || report.Reason != "" {
		t.Fatalf("a present cache: %+v, entry %d bytes", report, info.Size())
	}
	stdout, stderr, code := runAffectedCacheStatusCLI(t, root, "--format", "doctor-plugin")
	var items []map[string]string
	if code != 0 || json.Unmarshal(stdout, &items) != nil || len(items) != 1 {
		t.Fatalf("doctor-plugin: exit %d: %s\n%s", code, stderr, stdout)
	}
	if item := items[0]; item["kind"] != "AFFECTED_CACHE" || item["who"] != location || !strings.HasPrefix(item["detail"], "PRESENT: 1 entries, ") || item["remedy"] == "" || len(item) != 4 {
		t.Fatalf("doctor-plugin item: %v", item)
	}
	if stdout, _, code := runAffectedCacheStatusCLI(t, root, "--format=json"); code != 0 || !bytes.Contains(stdout, []byte(`"state":"PRESENT"`)) {
		t.Fatalf("--format=json: exit %d: %s", code, stdout)
	}
	// A directory the planner refuses as not private is reported, not PRESENT.
	if err := os.Chmod(location, 0o770); err != nil {
		t.Fatal(err)
	}
	if report := status("UNAVAILABLE"); report.Location != location || !strings.Contains(report.Reason, "not private") {
		t.Fatalf("a group-writable cache: %+v", report)
	}
	if err := os.Chmod(location, 0o700); err != nil {
		t.Fatal(err)
	}
	useAffectedUserCacheDir(t, root, nil)
	if report := status("UNAVAILABLE"); !strings.Contains(report.Reason, "outside the repository") {
		t.Fatalf("a default inside the repository: %+v", report)
	}
	useAffectedUserCacheDir(t, "", errors.New("$HOME is not defined"))
	if report := status("UNAVAILABLE"); report.Location != "" || !strings.Contains(report.Reason, "no user cache directory") {
		t.Fatalf("no user cache directory: %+v", report)
	}
	stdout, _, _ = runAffectedCacheStatusCLI(t, root, "--format", "doctor-plugin")
	if !bytes.Contains(stdout, []byte(`"who":"affected-cache"`)) {
		t.Fatalf("doctor-plugin with no location: %s", stdout)
	}
	for _, refused := range [][]string{{"--format", "text"}, {"--format", "json", "--format", "json"}, {"--format"}, {"--no-cache"}, {"extra"}} {
		if _, stderr, code := runAffectedCacheStatusCLI(t, root, refused...); code != 2 || stderr == "" {
			t.Fatalf("%q: exit %d: %s", refused, code, stderr)
		}
	}
}

// AFP-V0-436, AFP-V0-437, AFP-V0-439: help names the default location, the
// two overrides and the status form.
func TestAFPV0437HelpDocumentsTheDefaultCache(t *testing.T) {
	t.Parallel()
	help := strings.Join(strings.Fields(helpText("affected")), " ")
	for _, want := range []string{
		"[--cache DIR | --no-cache]",
		"corvint/affected under the user cache directory",
		"--no-cache turns the cache off",
		"the call runs uncached, with no error",
		"affected cache [--format json|doctor-plugin]",
		"it creates and writes nothing",
	} {
		if !strings.Contains(help, want) {
			t.Fatalf("affected help does not say %q", want)
		}
	}
}
