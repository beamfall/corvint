// SPDX-License-Identifier: Apache-2.0
package affected_test

import (
	"bytes"
	"os"
	"reflect"
	"testing"

	"github.com/Beamfall/corvint/internal/liveverify/affected"
)

// AFP-V0-9995: a cached graph restores each unit's walk facts exactly and
// plans a deleted script as the fresh graph does, the walk facts are part of
// the digest, and an entry written under the previous profile, which cannot
// hold them, is rebuilt rather than read as holding none.
func TestGraphCacheWalkFacts_AFPV09995(t *testing.T) {
	pinnable(t)
	root, dir := t.TempDir(), privateCacheDir(t)
	writeCacheFixture(t, root, map[string]string{
		"script/a.sh":          "true\n",
		"tools/helper.sh":      "ls script | wc -l\n",
		"tools/runner_test.sh": "bash \"$dir/check-$1.sh\"\nfind \"$1\" -name x\n",
	})
	fresh, outcome := buildCached(t, root, dir, graphCacheIdentity)
	if !outcome.Stored {
		t.Fatalf("fixture must store: %+v", outcome)
	}
	cached, outcome := buildCached(t, root, dir, graphCacheIdentity)
	if !outcome.Hit {
		t.Fatalf("fixture must hit: %+v", outcome)
	}
	helper, _ := cached.Unit("shell:tools/helper.sh")
	runner, _ := cached.Unit("shell:tools/runner_test.sh")
	if !reflect.DeepEqual(helper.WalkDirectories, []string{"script"}) || runner.WalkOpaque != "computed-walk-operand" || !reflect.DeepEqual(runner.RunNames, []string{"check-*.sh"}) {
		t.Fatalf("cache lost walk facts: helper %v runner %q %v", helper.WalkDirectories, runner.WalkOpaque, runner.RunNames)
	}
	if fresh.Digest() != cached.Digest() {
		t.Fatalf("digest differs after restore: %s %s", fresh.Digest(), cached.Digest())
	}
	const deleted = "script/check-old.sh"
	if !reflect.DeepEqual(affected.Select(fresh, []string{deleted}), affected.Select(cached, []string{deleted})) {
		t.Fatal("cached plan of a deleted script differs from fresh")
	}
	entry := affected.GraphCacheEntryPath(dir, root, allLanguages())
	body, err := os.ReadFile(entry)
	if err != nil {
		t.Fatal(err)
	}
	if affected.GraphCacheProfile != "corvint-affected-graph-cache/5" || !bytes.HasPrefix(body, []byte(affected.GraphCacheProfile+" ")) {
		t.Fatalf("entry profile: %q", body[:min(len(body), 40)])
	}
	body = bytes.Replace(body, []byte(affected.GraphCacheProfile+" "), []byte("corvint-affected-graph-cache/4 "), 1)
	if err := os.WriteFile(entry, body, 0o600); err != nil {
		t.Fatal(err)
	}
	rebuilt, outcome := buildCached(t, root, dir, graphCacheIdentity)
	if outcome.Hit || outcome.Reason != "entry checksum or profile mismatch" || !outcome.Stored {
		t.Fatalf("a /4 entry must rebuild: %+v", outcome)
	}
	if unit, _ := rebuilt.Unit("shell:tools/helper.sh"); !reflect.DeepEqual(unit.WalkDirectories, []string{"script"}) {
		t.Fatalf("rebuilt graph lost walk facts: %v", unit.WalkDirectories)
	}
}
