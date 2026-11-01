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
		cmd := exec.Command("git", append([]string{"-c", "user.name=t", "-c", "user.email=t@example.invalid"}, args...)...)
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

// TestResultCacheEqualsReplay_ASDV0778 proves the fast path equals the slow
// path: a second run on byte-identical inputs replays nothing and writes
// byte-identical results; the cache off, a changed planner, a changed cost
// table and a changed change entry each replay again.
func TestResultCacheEqualsReplay_ASDV0778(t *testing.T) {
	root, corvint, selector, count, set := cacheFixture(t)
	ctx := context.Background()
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
	cache, off := openCache(ctx, root, corvint, selector)
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
		if err := os.WriteFile(corvint, []byte(fakeCorvint+"# rebuilt\n"), 0o755); err != nil {
			t.Fatal(err)
		}
	})
	replays("selector bytes", func() {
		if err := os.WriteFile(selector, []byte(fakeSelector+"# rebuilt\n"), 0o755); err != nil {
			t.Fatal(err)
		}
	})
	replays("environment", func() { t.Setenv("GOFLAGS", "-mod=mod") })
	replays("costs", func() {
		if err := os.WriteFile(filepath.Join(root, ".github", "cishards", "package-costs.json"), []byte(`{"milliseconds":{"example.com/fixture/x":50,"example.com/fixture/y":50}}`+"\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	})
	os.Remove(count)
	set.Changes[0].Recorded = &recorded{CIScope: "FULL", Verdict: "run"}
	if _, err := replay(ctx, root, set, "changes.json", corvint, selector); err != nil {
		t.Fatal(err)
	}
	if got := invocations(t, count); !reflect.DeepEqual(got, []string{"--version", "affected", "test"}) {
		t.Fatalf("a changed change entry: want only that change replayed, got %q", got)
	}
}

// TestResultCacheRefusesForeignRecords_ASDV0778: a record whose schema or key
// does not match, or that does not decode, is a miss, never a result.
func TestResultCacheRefusesForeignRecords_ASDV0778(t *testing.T) {
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

func TestCacheEnvironmentKeepsBehaviourVariables_ASDV0778(t *testing.T) {
	got := cacheEnvironment([]string{"PATH=/bin", "GOFLAGS=-x", "GIT_TRACE2_EVENT=/tmp/t", "GIT_DIR=/x", "CORVINT_A=1", "CGO_ENABLED=0", "SELECTION_BASELINE_JOBS=8", "SELECTION_BASELINE_CACHE=on", "HOME=/h"})
	want := []string{"CGO_ENABLED=0", "CORVINT_A=1", "GIT_DIR=/x", "GOFLAGS=-x"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("cacheEnvironment = %q, want %q", got, want)
	}
}
