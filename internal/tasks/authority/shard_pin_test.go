//go:build darwin || linux

package authority

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/tasks/fixture"
)

func pinnedShards(s *fixtureSession) []string {
	var keys []string
	for key := range s.parents {
		if strings.HasPrefix(key, "requests/") {
			keys = append(keys, key)
		}
	}
	sort.Strings(keys)
	return keys
}

// TestCALV0234_RequestShardsPinnedOnFirstUse covers CAL-V0-234: opening a
// session pins no request shard, the first link under a shard pins exactly
// that shard and lands the bytes there, and a shard that is not a directory
// or does not exist refuses the link without pinning anything.
func TestCALV0234_RequestShardsPinnedOnFirstUse(t *testing.T) {
	s, r := fixtureHarnessWith(t, func(r *fixture.Repo) {
		for i := 0; i < 256; i++ {
			fixtureMust(t, os.MkdirAll(filepath.Join(r.StateDir, "requests", fmt.Sprintf("%02x", i)), 0700))
		}
	})
	if got := pinnedShards(s); len(got) != 0 {
		t.Fatalf("session construction pinned %d request shards: %v", len(got), got)
	}
	name := "ab" + strings.Repeat("0", 62) + ".json"
	stage := fixturePrepared(t, s, fixtureRequest, "request-bytes")
	fixtureMust(t, s.link(stage, fixtureTarget{fixtureRequest, name}))
	if got := pinnedShards(s); len(got) != 1 || got[0] != "requests/ab" {
		t.Fatalf("first link pinned %v, want only requests/ab", got)
	}
	f, err := s.parents["requests/ab"].root.Open(name)
	fixtureMust(t, err)
	b, err := io.ReadAll(f)
	fixtureMust(t, err)
	fixtureMust(t, f.Close())
	if string(b) != "request-bytes" {
		t.Fatalf("linked bytes %q", b)
	}

	notDir := filepath.Join(r.StateDir, "requests", "cd")
	fixtureMust(t, os.Remove(notDir))
	fixtureMust(t, os.WriteFile(notDir, []byte("x"), 0600))
	stage, err = s.prepare("a01", fixtureRequest, []byte("second"))
	fixtureMust(t, err)
	if err := s.link(stage, fixtureTarget{fixtureRequest, "cd" + strings.Repeat("0", 62) + ".json"}); err == nil {
		t.Fatal("link under a shard that is a regular file succeeded")
	}
	fixtureMust(t, os.Remove(filepath.Join(r.StateDir, "requests", "ef")))
	stage, err = s.prepare("a02", fixtureRequest, []byte("third"))
	fixtureMust(t, err)
	if err := s.link(stage, fixtureTarget{fixtureRequest, "ef" + strings.Repeat("0", 62) + ".json"}); err == nil {
		t.Fatal("link under a missing shard succeeded")
	}
	if got := pinnedShards(s); len(got) != 1 || got[0] != "requests/ab" {
		t.Fatalf("refused links changed the pinned shards to %v", got)
	}
}
