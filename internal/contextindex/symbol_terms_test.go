package contextindex

import (
	"context"
	"encoding/json"
	"os"
	"testing"
)

// symbolTermQueries cover exact names, compact-name containment, multi-term
// overlap, context-only matches, version narrowing, test intent and an empty
// query.
var symbolTermQueries = []string{
	"demux", "Demux", "split the cache key", "cache demux keys", "where is the snapshot decoded",
	"chunk the term table", "v2 api handler", "fix the failing test for split", "", "zz no such words zz",
	"LoadSnapshot", "evalPrepareSymbols ranking", "index refresh worker profile", "docs readme install",
}

func symbolTermIndexes(t *testing.T) map[string]*Index {
	t.Helper()
	indexes := map[string]*Index{"fixture": taskContextFixture(t)}
	if root := os.Getenv("CORVINT_SNAPSHOT_PARITY_ROOT"); root != "" {
		built, err := Build(context.Background(), root)
		if err != nil {
			t.Fatal(err)
		}
		indexes["repository"] = built
	}
	return indexes
}

func mustJSON(t *testing.T, value any) string {
	t.Helper()
	encoded, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	return string(encoded)
}
