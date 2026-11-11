// SPDX-License-Identifier: Apache-2.0
package main

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/execfile"
	"github.com/Beamfall/corvint/internal/testplatform"
)

const fakeCorvint = `#!/bin/sh
echo "$1" >> "$FAKE_COUNT"
case $1 in
--version) echo fake planner ;;
affected) printf '{"plan":{"dirty":["a.txt"],"scope":"BOUNDED","selected":[{"unitId":"go:example.com/fixture/x","witness":{"kind":"import"}}],"excluded":{"count":2},"unknown":[{"reason":"LANGUAGE_FRONTIER","detail":"shell:exec:site"}]}}\n' ;;
test) printf '{"scope":"NARROW","units":[{"adapter":"go","unit":"x","slice":"whole","reason":"DIRECT_SOURCE_CHANGE; tests changed too"}]}\n' > "$6" ;;
esac
`

const fakeSelector = "#!/bin/sh\nprintf 'audit\\nrun example.com/fixture/x\\n'\n"

// cacheFixture is a repository with two commits, a cost table and the fake
// planner and selector, with a private cache directory and invocation log.
func cacheFixture(t *testing.T) (root, corvint, selector, count string, set changeSet) {
	t.Helper()
	testplatform.SkipPOSIXShell(t, "the fake planner and selector are sh scripts")
	root = t.TempDir()
	gitIn := func(args ...string) string {
		cmd := exec.Command("git", append([]string{"-c", "user.name=t", "-c", "user.email=t@example.invalid", "-c", "maintenance.auto=false", "-c", "gc.auto=0"}, args...)...)
		cmd.Dir = root
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
		return strings.TrimSpace(string(out))
	}
	writeFile := func(name, body string, mode os.FileMode) {
		if err := os.MkdirAll(filepath.Dir(name), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(name, []byte(body), mode); err != nil {
			t.Fatal(err)
		}
	}
	writeFile(filepath.Join(root, "go.mod"), "module example.com/fixture\n\ngo 1.27\n", 0o644)
	writeFile(filepath.Join(root, ".github", "cishards", "package-costs.json"), `{"milliseconds":{"example.com/fixture/x":30,"example.com/fixture/y":70}}`+"\n", 0o644)
	writeFile(filepath.Join(root, ".github", "forward-rows", "github-guard.sh"), "echo 0\n", 0o644)
	gitIn("init", "-q", "-b", "main")
	gitIn("add", "-A")
	gitIn("commit", "-qm", "base")
	base := gitIn("rev-parse", "HEAD")
	writeFile(filepath.Join(root, "a.txt"), "a\n", 0o644)
	gitIn("add", "-A")
	gitIn("commit", "-qm", "head")
	head := gitIn("rev-parse", "HEAD")
	bin := t.TempDir()
	corvint, selector = filepath.Join(bin, "corvint"), filepath.Join(bin, "selector")
	writeFile(corvint, fakeCorvint, 0o755)
	writeFile(selector, fakeSelector, 0o755)
	count = filepath.Join(t.TempDir(), "COUNT")
	t.Setenv("FAKE_COUNT", count)
	t.Setenv("SELECTION_BASELINE_CACHE", "")
	t.Setenv("CORVINT_GATE_LEDGER", "")
	t.Setenv("SELECTION_BASELINE_CACHE_DIR", filepath.Join(t.TempDir(), "cache"))
	set = changeSet{Profile: changesProfile, Floor: floor{NarrowMerged: 1}, Changes: []change{
		{ID: "merged-1", Kind: "merged", Base: base, Head: head},
		{ID: "failed-1", Kind: "failed-ci", Base: base, Head: head, Failures: []failure{{Run: 1, Job: 2, Package: "example.com/fixture/y"}}},
	}}
	return root, corvint, selector, count, set
}

func invocations(t *testing.T, count string) []string {
	t.Helper()
	data, err := os.ReadFile(count)
	if err != nil && !os.IsNotExist(err) {
		t.Fatal(err)
	}
	return strings.Fields(string(data))
}

func replayJSON(t *testing.T, r results) string {
	t.Helper()
	data, err := json.MarshalIndent(r, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

// TestResultCacheEqualsReplay_ASCV0778 proves the fast path equals the slow
// path: a second run on byte-identical inputs replays nothing and writes
// byte-identical results; the cache off, a changed planner, a changed cost
// table and a changed change entry each replay again.
func TestResultCacheEqualsReplay_ASCV0778(t *testing.T) {
	root, corvint, selector, count, set := cacheFixture(t)
	ctx := context.Background()
	goenv := filepath.Join(t.TempDir(), "go.env")
	if err := os.WriteFile(goenv, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("GOENV", goenv)
	cold, err := replay(ctx, root, set, "changes.json", corvint, selector)
	if err != nil {
		t.Fatal(err)
	}
	if got := invocations(t, count); len(got) != 5 || got[0] != "--version" || strings.Count(strings.Join(got, " "), "affected") != 2 {
		t.Fatalf("cold run invocations %q", got)
	}
	if len(cold.Results) != 2 || cold.Results[0].CIScope != "NARROW" || len(cold.Results[1].Misses) != 1 {
		t.Fatalf("cold results %+v", cold.Results)
	}
	os.Remove(count)
	warm, err := replay(ctx, root, set, "changes.json", corvint, selector)
	if err != nil {
		t.Fatal(err)
	}
	if got := invocations(t, count); !reflect.DeepEqual(got, []string{"--version"}) {
		t.Fatalf("warm run replayed: %q", got)
	}
	cache, off := openCache(ctx, root, corvint, selector, []byte(mustReadFile(t, filepath.Join(root, ".github", "cishards", "package-costs.json"))), []byte("echo 0\n"))
	if cache == nil || len(hex.EncodeToString(cache.stamp)) != 64 {
		t.Fatalf("no cache key: %s", off)
	}
	if replayJSON(t, warm) != replayJSON(t, cold) || !reflect.DeepEqual(warm, cold) {
		t.Fatalf("cached results differ from the replay:\n%s\n%s", replayJSON(t, warm), replayJSON(t, cold))
	}

	replays := func(name string, edit func()) {
		t.Helper()
		edit()
		os.Remove(count)
		again, err := replay(ctx, root, set, "changes.json", corvint, selector)
		if err != nil {
			t.Fatal(err)
		}
		if got := invocations(t, count); len(got) < 3 {
			t.Fatalf("%s: did not replay: %q", name, got)
		}
		if name != "costs" && replayJSON(t, again) != replayJSON(t, cold) {
			t.Fatalf("%s: replay differs:\n%s", name, replayJSON(t, again))
		}
	}
	replays("cache off", func() { t.Setenv("SELECTION_BASELINE_CACHE", "off") })
	t.Setenv("SELECTION_BASELINE_CACHE", "")
	replays("ledger off", func() { t.Setenv("CORVINT_GATE_LEDGER", "off") })
	t.Setenv("CORVINT_GATE_LEDGER", "")
	replays("planner bytes", func() {
		if err := execfile.WriteFile(corvint, []byte(fakeCorvint+"# rebuilt\n"), 0o755); err != nil {
			t.Fatal(err)
		}
	})
	replays("selector bytes", func() {
		if err := execfile.WriteFile(selector, []byte(fakeSelector+"# rebuilt\n"), 0o755); err != nil {
			t.Fatal(err)
		}
	})
	replays("environment", func() { t.Setenv("GOFLAGS", "-mod=mod") })
	// CI sets GOPROXY=off, and go env shows that variable over the file's
	// value, so only the file's bytes can key the edit below. Record the
	// cache under it first, whatever the caller's environment.
	t.Setenv("GOPROXY", "off")
	if _, err := replay(ctx, root, set, "changes.json", corvint, selector); err != nil {
		t.Fatal(err)
	}
	replays("go env -w", func() {
		if err := os.WriteFile(goenv, []byte("GOPROXY=off\n"), 0o600); err != nil {
			t.Fatal(err)
		}
	})
	replays("git config", func() {
		if out, err := exec.Command("git", "-C", root, "config", "core.abbrev", "12").CombinedOutput(); err != nil {
			t.Fatalf("git config: %v %s", err, out)
		}
	})
	replays("costs", func() {
		if err := os.WriteFile(filepath.Join(root, ".github", "cishards", "package-costs.json"), []byte(`{"milliseconds":{"example.com/fixture/x":50,"example.com/fixture/y":50}}`+"\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	})
	os.Remove(count)
	set.Changes[0].Failures = []failure{{Run: 3, Job: 4, Package: "example.com/fixture/x"}}
	if _, err := replay(ctx, root, set, "changes.json", corvint, selector); err != nil {
		t.Fatal(err)
	}
	if got := invocations(t, count); !reflect.DeepEqual(got, []string{"--version", "affected", "test"}) {
		t.Fatalf("a changed change entry: want only that change replayed, got %q", got)
	}
	// ASC-V0-9851: the recorded scope and verdict are not replay inputs.
	os.Remove(count)
	set.Changes[0].Recorded = &recorded{CIScope: "FULL", Verdict: "run"}
	if _, err := replay(ctx, root, set, "changes.json", corvint, selector); err != nil {
		t.Fatal(err)
	}
	if got := invocations(t, count); !reflect.DeepEqual(got, []string{"--version"}) {
		t.Fatalf("a re-recorded change replayed: %q", got)
	}
}

// TestResultCacheKeyMissesOnEveryInput_ASCV9851 (V1-5162): each field of the
// change entry that measure reads, and the run's stamp (planner, selector and
// tool bytes, costs, go, git and environment; openCache's inputs, each
// replayed in TestResultCacheEqualsReplay_ASCV0778), moves the record key;
// the recorded scope and verdict, which measure never reads, do not.
func TestResultCacheKeyMissesOnEveryInput_ASCV9851(t *testing.T) {
	cache := &resultCache{stamp: []byte("stamp")}
	ch := change{ID: "c", Kind: "merged", Base: strings.Repeat("a", 40), Head: strings.Repeat("b", 40), PR: 7, Failures: []failure{{Run: 1, Job: 2, Package: "p"}}}
	key := cache.key(ch)
	for name, edit := range map[string]func(*change){
		"id":          func(c *change) { c.ID = "d" },
		"kind":        func(c *change) { c.Kind = "failed-ci" },
		"base":        func(c *change) { c.Base = strings.Repeat("c", 40) },
		"head":        func(c *change) { c.Head = strings.Repeat("c", 40) },
		"pr":          func(c *change) { c.PR = 8 },
		"failures":    func(c *change) { c.Failures = []failure{{Run: 1, Job: 2, Package: "q"}} },
		"no failures": func(c *change) { c.Failures = nil },
	} {
		other := ch
		edit(&other)
		if cache.key(other) == key {
			t.Errorf("%s: key unchanged", name)
		}
	}
	if (&resultCache{stamp: []byte("stamp2")}).key(ch) == key {
		t.Error("stamp: key unchanged")
	}
	rerecorded := ch
	rerecorded.Recorded = &recorded{CIScope: "NARROW", Verdict: "run"}
	if cache.key(rerecorded) != key {
		t.Error("recorded: key moved")
	}
	if ch.Recorded != nil || rerecorded.Recorded == nil {
		t.Fatal("key mutated its argument")
	}
}

// TestResultCacheRefusesForeignRecords_ASCV0778: a record whose schema or key
// does not match, or that does not decode, is a miss, never a result.
func TestResultCacheRefusesForeignRecords_ASCV0778(t *testing.T) {
	cache := &resultCache{dir: t.TempDir(), stamp: []byte("stamp")}
	key := cache.key(change{ID: "c"})
	if err := cache.store(key, Result{ID: "c", CIScope: "NARROW"}); err != nil {
		t.Fatal(err)
	}
	if r, ok := cache.load(key); !ok || r.ID != "c" {
		t.Fatalf("stored record: %+v %v", r, ok)
	}
	for name, body := range map[string]string{
		"other key":    `{"schema":"` + cacheSchema + `","key":"0000","result":{"id":"c"}}`,
		"other schema": `{"schema":"other/1","key":"` + key + `","result":{"id":"c"}}`,
		"truncated":    `{"schema":"`,
	} {
		if err := os.WriteFile(cache.path(key), []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
		if _, ok := cache.load(key); ok {
			t.Errorf("%s: loaded", name)
		}
	}
	if other := (&resultCache{dir: cache.dir, stamp: []byte("other")}).key(change{ID: "c"}); other == key {
		t.Fatal("a different stamp kept the key")
	}
}

func TestCacheEnvironmentKeepsBehaviourVariables_ASCV0778(t *testing.T) {
	got := cacheEnvironment([]string{"PATH=/bin", "GOFLAGS=-x", "GIT_TRACE2_EVENT=/tmp/t", "GIT_DIR=/x", "CORVINT_A=1", "CGO_ENABLED=0", "SELECTION_BASELINE_JOBS=8", "SELECTION_BASELINE_CACHE=on", "SELECTION_CONTRACT_CHECK=1", "HOME=/h"})
	want := []string{"CGO_ENABLED=0", "CORVINT_A=1", "GIT_DIR=/x", "GOFLAGS=-x", "SELECTION_CONTRACT_CHECK=1"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("cacheEnvironment = %q, want %q", got, want)
	}
}

func mustReadFile(t *testing.T, name string) string {
	t.Helper()
	data, err := os.ReadFile(name)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

// TestResultCacheKeysTheCostBytesReplayRead_ASCV0778: the key is
// derived from the cost bytes replay parsed, not from a second read of the
// file, so a table that changes between the read and the key cannot store
// results measured with one table under the other's key.
func TestResultCacheKeysTheCostBytesReplayRead_ASCV0778(t *testing.T) {
	root, corvint, selector, _, _ := cacheFixture(t)
	ctx := context.Background()
	costsFile := filepath.Join(root, ".github", "cishards", "package-costs.json")
	a := []byte(mustReadFile(t, costsFile))
	b := []byte(`{"milliseconds":{"example.com/fixture/x":99}}` + "\n")
	stamp := func(costs []byte) string {
		t.Helper()
		cache, off := openCache(ctx, root, corvint, selector, costs, []byte("echo 0\n"))
		if cache == nil {
			t.Fatalf("cache off: %s", off)
		}
		return hex.EncodeToString(cache.stamp)
	}
	sa := stamp(a)
	if err := os.WriteFile(costsFile, b, 0o644); err != nil {
		t.Fatal(err)
	}
	if got := stamp(a); got != sa {
		t.Fatalf("a changed file moved the key of the bytes read: %s vs %s", got, sa)
	}
	if stamp(b) == sa {
		t.Fatal("other cost bytes kept the key")
	}
}

// TestResultCacheColdLine_ASCV9852 (V1-5162): an empty cache directory prints
// the cold line, never the off line, and a warm rerun prints REUSE.
func TestResultCacheColdLine_ASCV9852(t *testing.T) {
	root, corvint, selector, _, set := cacheFixture(t)
	stderr := func() string {
		t.Helper()
		f, err := os.CreateTemp(t.TempDir(), "stderr")
		if err != nil {
			t.Fatal(err)
		}
		saved := os.Stderr
		os.Stderr = f
		_, err = replay(context.Background(), root, set, "changes.json", corvint, selector)
		os.Stderr = saved
		f.Close()
		if err != nil {
			t.Fatal(err)
		}
		return mustReadFile(t, f.Name())
	}
	if got := stderr(); !strings.Contains(got, "result cache cold: 0 of 2 change results") || strings.Contains(got, "cache off") {
		t.Fatalf("cold run stderr:\n%s", got)
	}
	if got := stderr(); !strings.Contains(got, "REUSE 2 of 2") || strings.Contains(got, "cache cold") {
		t.Fatalf("warm run stderr:\n%s", got)
	}
}
