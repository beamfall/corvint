package cli_test

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/Beamfall/corvint/internal/tasks/cli"
	"github.com/Beamfall/corvint/internal/tasks/fixture"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// privateCache points the user cache directory at a temporary directory
// and returns the preflight cache directory under it.
func privateCache(t *testing.T) string {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CACHE_HOME", filepath.Join(home, ".cache"))
	base, err := os.UserCacheDir()
	if err != nil {
		t.Fatal(err)
	}
	return filepath.Join(base, "corvint-tasks", "preflight")
}

func cacheStatus(t *testing.T, x run) string {
	t.Helper()
	if len(x.res.Items) != 1 {
		t.Fatalf("no preflight item: %s", x.stdout)
	}
	c := field(x.res.Items[0], "cache")
	if c.Kind != wire.KindObject {
		t.Fatalf("no cache member: %s", x.stdout)
	}
	return field(c, "status").Str
}

func warned(x run, prefix string) bool {
	for _, w := range x.res.Warnings {
		if strings.HasPrefix(w, prefix) {
			return true
		}
	}
	return false
}

// TestTOLV0047_PreflightCacheReplaysIdenticalInputs: with --cache an
// identical second preflight replays the stored report and says so, and a
// change to the commit, a tracked file, an untracked file or the check
// implementation misses the cache (TOL-V0-047, TOL-V0-048); a run without
// --cache has no cache member.
func TestTOLV0047_PreflightCacheReplaysIdenticalInputs(t *testing.T) {
	privateCache(t)
	r, id, _ := preflightRepo(t, cleanSpec, cleanSeed)
	if x := atm(t, r.Root, nil, "preflight", id); len(x.res.Items) != 1 || func() bool { _, ok := x.res.Items[0].Obj.Get("cache"); return ok }() {
		t.Fatalf("a run without --cache carries a cache member: %s", x.stdout)
	}
	step := func(want string) run {
		t.Helper()
		x := atm(t, r.Root, nil, "preflight", id, "--cache")
		if got := cacheStatus(t, x); got != want {
			t.Fatalf("cache status %s, want %s: %s", got, want, x.stdout)
		}
		if want == "HIT" != warned(x, "PREFLIGHT_CACHE_HIT:") {
			t.Fatalf("hit warning mismatch for %s: %s", want, x.stdout)
		}
		return x
	}
	first := step("MISS")
	hit := step("HIT")
	strip := func(x run) string {
		v := x.res.Items[0]
		o := wire.NewObject()
		for _, k := range v.Obj.SortedKeys() {
			if k != "cache" {
				val, _ := v.Obj.Get(k)
				o.Set(k, val)
			}
		}
		return string(wire.Encode(wire.ObjectValue(o)))
	}
	if strip(first) != strip(hit) || hit.res.Outcome != first.res.Outcome {
		t.Fatalf("replayed report differs\nfirst %s\nhit   %s", first.stdout, hit.stdout)
	}
	if field(field(hit.res.Items[0], "cache"), "key").Str != field(field(first.res.Items[0], "cache"), "key").Str {
		t.Fatal("identical inputs keyed differently")
	}
	// An untracked file Git does not ignore, then a change to its content.
	fixture.Write(t, filepath.Join(r.Root, "notes.txt"), []byte("one\n"))
	step("MISS")
	step("HIT")
	fixture.Write(t, filepath.Join(r.Root, "notes.txt"), []byte("two\n"))
	step("MISS")
	// A tracked file changed but not committed.
	fixture.Write(t, filepath.Join(r.Root, "e2e", "helpers.ts"), []byte("// AC-6 helper, edited\n"))
	step("MISS")
	step("HIT")
	// A new commit.
	git(t, r.Root, "add", "-A")
	git(t, r.Root, "-c", "user.name=t", "-c", "user.email=t@example.invalid", "commit", "-m", "edit")
	step("MISS")
	// Another build of the check implementation.
	restore := cli.SetImplementationDigest(func() (string, error) { return strings.Repeat("ab", 32), nil })
	step("MISS")
	step("HIT")
	restore()
	step("HIT")
}

// TestTOLV0048_PreflightFreshBypassesTheCache: --fresh runs the checks and
// replaces the entry, --fresh needs --cache and --cache refuses --deep, a failing report replays as refused, and an
// unavailable implementation digest skips the cache with the reason.
func TestTOLV0048_PreflightFreshBypassesTheCache(t *testing.T) {
	privateCache(t)
	r, id, _ := preflightRepo(t, preflightSpec, preflightSeed)
	x := atm(t, r.Root, nil, "preflight", id, "--fresh")
	if x.res.Outcome != wire.OutcomeError || !warned(x, "--fresh bypasses the cache") {
		t.Fatalf("--fresh without --cache: %s", x.stdout)
	}
	if x := atm(t, r.Root, nil, "preflight", id, "--cache", "--deep", "--gate", "lint"); x.res.Outcome != wire.OutcomeError || !warned(x, "--cache covers the static checks only") {
		t.Fatalf("--cache with --deep: %s", x.stdout)
	}
	miss := atm(t, r.Root, nil, "preflight", id, "--cache")
	if cacheStatus(t, miss) != "MISS" || miss.res.Outcome != wire.OutcomeRefused {
		t.Fatalf("failing miss: %s", miss.stdout)
	}
	hit := atm(t, r.Root, nil, "preflight", id, "--cache")
	if cacheStatus(t, hit) != "HIT" || hit.res.Outcome != wire.OutcomeRefused || hit.code == 0 || !warned(hit, "PREFLIGHT_FAILED") {
		t.Fatalf("failing report did not replay refused: %s", hit.stdout)
	}
	later := time.Now().Add(time.Hour)
	defer cli.SetLocalNow(func() time.Time { return later })()
	fresh := atm(t, r.Root, nil, "preflight", id, "--cache", "--fresh")
	if cacheStatus(t, fresh) != "FRESH" || warned(fresh, "PREFLIGHT_CACHE_HIT:") {
		t.Fatalf("--fresh: %s", fresh.stdout)
	}
	again := atm(t, r.Root, nil, "preflight", id, "--cache")
	if cacheStatus(t, again) != "HIT" || field(field(again.res.Items[0], "cache"), "storedAt").Str != later.UTC().Truncate(time.Second).Format(time.RFC3339) {
		t.Fatalf("--fresh did not replace the entry: %s", again.stdout)
	}
	defer cli.SetImplementationDigest(func() (string, error) { return "", os.ErrPermission })()
	skipped := atm(t, r.Root, nil, "preflight", id, "--cache")
	if cacheStatus(t, skipped) != "SKIPPED" || field(field(skipped.res.Items[0], "cache"), "reason").Kind != wire.KindString {
		t.Fatalf("unkeyable inputs: %s", skipped.stdout)
	}
}

// TestTOLV0049_PreflightCacheIsBounded: entries older than the age bound
// miss and are pruned, and the directory keeps at most the count bound,
// dropping the oldest.
func TestTOLV0049_PreflightCacheIsBounded(t *testing.T) {
	dir := privateCache(t)
	r, id, _ := preflightRepo(t, cleanSpec, cleanSeed)
	if cacheStatus(t, atm(t, r.Root, nil, "preflight", id, "--cache")) != "MISS" {
		t.Fatal("first run is not a miss")
	}
	restore := cli.SetLocalNow(func() time.Time { return time.Now().Add(cli.PreflightCacheMaxAge + time.Hour) })
	if got := cacheStatus(t, atm(t, r.Root, nil, "preflight", id, "--cache")); got != "MISS" {
		t.Fatalf("an entry past the age bound replayed: %s", got)
	}
	restore()
	// Fill the directory past the count bound with aged files.
	base := time.Now().Add(-time.Hour)
	for i := 0; i < cli.PreflightCacheMaxEntries+10; i++ {
		name := filepath.Join(dir, strings.Repeat("0", 56)+strconv.FormatInt(int64(10000000+i), 10)+".json")
		fixture.Write(t, name, []byte("{}\n"))
		mod := base.Add(time.Duration(i) * time.Second)
		if err := os.Chtimes(name, mod, mod); err != nil {
			t.Fatal(err)
		}
	}
	old := filepath.Join(dir, strings.Repeat("1", 64)+".json")
	fixture.Write(t, old, []byte("{}\n"))
	stale := time.Now().Add(-cli.PreflightCacheMaxAge - time.Hour)
	if err := os.Chtimes(old, stale, stale); err != nil {
		t.Fatal(err)
	}
	foreign := filepath.Join(dir, "keep.txt")
	fixture.Write(t, foreign, []byte("not an entry\n"))
	cli.PruneCacheDir(dir, cli.PreflightCacheMaxEntries, cli.PreflightCacheMaxAge)
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	count := 0
	for _, e := range entries {
		if strings.HasSuffix(e.Name(), ".json") {
			count++
		}
	}
	if count != cli.PreflightCacheMaxEntries {
		t.Fatalf("%d entries after pruning, want %d", count, cli.PreflightCacheMaxEntries)
	}
	if _, err := os.Stat(old); !os.IsNotExist(err) {
		t.Fatal("an entry past the age bound survived pruning")
	}
	if _, err := os.Stat(filepath.Join(dir, strings.Repeat("0", 56)+"10000000.json")); !os.IsNotExist(err) {
		t.Fatal("the oldest entry survived the count bound")
	}
	if _, err := os.Stat(foreign); err != nil {
		t.Fatal("pruning removed a file the cache did not write")
	}
}

// TestTOLV0050_PreflightCacheStaysOutsideTheRepository: a cached run
// writes its entry under the user cache directory and changes nothing in
// the repository, the intent store or the state directory.
func TestTOLV0050_PreflightCacheStaysOutsideTheRepository(t *testing.T) {
	dir := privateCache(t)
	r, id, _ := preflightRepo(t, cleanSpec, cleanSeed)
	state, intents := fixture.TreeSnapshot(t, r.StateDir), fixture.TreeSnapshot(t, r.IntentDir)
	before := gitIn(t, r.Root, "", "status", "--porcelain", "--untracked-files=all")
	atm(t, r.Root, nil, "preflight", id, "--cache")
	atm(t, r.Root, nil, "preflight", id, "--cache")
	if gitIn(t, r.Root, "", "status", "--porcelain", "--untracked-files=all") != before {
		t.Fatal("a cached preflight changed the working tree")
	}
	if !fixture.SameTree(state, fixture.TreeSnapshot(t, r.StateDir)) || !fixture.SameTree(intents, fixture.TreeSnapshot(t, r.IntentDir)) {
		t.Fatal("a cached preflight wrote store state")
	}
	entries, err := os.ReadDir(dir)
	if err != nil || len(entries) != 1 {
		t.Fatalf("cache directory holds %d entries (%v)", len(entries), err)
	}
	if info, err := os.Stat(dir); err != nil || info.Mode().Perm() != 0o700 {
		t.Fatalf("cache directory mode: %v %v", info, err)
	}
}
