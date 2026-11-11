// SPDX-License-Identifier: Apache-2.0
package affected_test

import (
	"bytes"
	"os"
	"reflect"
	"testing"

	"github.com/Beamfall/corvint/internal/liveverify/affected"
)

// AFP-V0-9964: a cached graph restores each unit's name-listing reads and
// name-only walk flag exactly and plans as the fresh graph does, and an entry
// written under the previous profile, which cannot hold them, is rebuilt.
func TestGraphCacheNameListings_AFPV09964(t *testing.T) {
	pinnable(t)
	root, dir := t.TempDir(), privateCacheDir(t)
	writeCacheFixture(t, root, map[string]string{
		"script/a.sh":    "true\n",
		"lister_test.sh": "test \"$(ls script | wc -l)\" -eq 1\n",
		"walker_test.sh": "find . -name '*.md' | wc -l\n",
		"docs/x.md":      "x\n",
	})
	fresh, outcome := buildCached(t, root, dir, graphCacheIdentity)
	if !outcome.Stored {
		t.Fatalf("fixture must store: %+v", outcome)
	}
	cached, outcome := buildCached(t, root, dir, graphCacheIdentity)
	if !outcome.Hit {
		t.Fatalf("fixture must hit: %+v", outcome)
	}
	lister, _ := cached.Unit("shell:lister_test.sh")
	walker, _ := cached.Unit("shell:walker_test.sh")
	if !reflect.DeepEqual(lister.NameListings, []string{"/script/"}) || !walker.UnboundedNamesOnly {
		t.Fatalf("cache lost name-listing reads: lister %v walker names-only %v", lister.NameListings, walker.UnboundedNamesOnly)
	}
	if fresh.Digest() != cached.Digest() {
		t.Fatalf("digest differs after restore: %s %s", fresh.Digest(), cached.Digest())
	}
	edit := map[string]affected.ChangeStatus{"script/a.sh": affected.ChangeModified, "docs/x.md": affected.ChangeModified}
	for _, status := range []map[string]affected.ChangeStatus{nil, edit} {
		dirty := []string{"docs/x.md", "script/a.sh"}
		if !reflect.DeepEqual(affected.Select(fresh.WithChangeStatus(status), dirty), affected.Select(cached.WithChangeStatus(status), dirty)) {
			t.Fatalf("cached plan differs from fresh under status %v", status)
		}
	}
	entry := affected.GraphCacheEntryPath(dir, root, allLanguages())
	body, err := os.ReadFile(entry)
	if err != nil {
		t.Fatal(err)
	}
	if affected.GraphCacheProfile != "corvint-affected-graph-cache/4" || !bytes.HasPrefix(body, []byte(affected.GraphCacheProfile+" ")) {
		t.Fatalf("entry profile: %q", body[:min(len(body), 40)])
	}
	body = bytes.Replace(body, []byte(affected.GraphCacheProfile+" "), []byte("corvint-affected-graph-cache/3 "), 1)
	if err := os.WriteFile(entry, body, 0o600); err != nil {
		t.Fatal(err)
	}
	rebuilt, outcome := buildCached(t, root, dir, graphCacheIdentity)
	if outcome.Hit || outcome.Reason != "entry checksum or profile mismatch" || !outcome.Stored {
		t.Fatalf("a /3 entry must rebuild: %+v", outcome)
	}
	if unit, _ := rebuilt.Unit("shell:lister_test.sh"); !reflect.DeepEqual(unit.NameListings, []string{"/script/"}) {
		t.Fatalf("rebuilt graph lost name-listing reads: %v", unit.NameListings)
	}
}
