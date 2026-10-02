package doccorpus

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"testing"
)

func updateIncrementalShard(t *testing.T, root string, m Manifest, path string, edit func(*ProviderRecord)) Manifest {
	t.Helper()
	raw, e := os.ReadFile(filepath.Join(root, path))
	if e != nil {
		t.Fatal(e)
	}
	var r ProviderRecord
	if e = decodeBounded(raw, &r, MaxProviderBytes); e != nil {
		t.Fatal(e)
	}
	edit(&r)
	raw, e = Encode(r)
	if e != nil {
		t.Fatal(e)
	}
	if e = os.WriteFile(filepath.Join(root, path), raw, 0600); e != nil {
		t.Fatal(e)
	}
	git(t, root, "add", path)
	git(t, root, "commit", "-qm", "changed shard")
	rev := git(t, root, "rev-parse", "HEAD")
	for i := range m.Providers {
		if m.Providers[i].ID == "adapter" {
			m.Providers[i].Revision = rev
		}
	}
	for i := range m.Inputs {
		if m.Inputs[i].Purpose == "provider" {
			in := &m.Inputs[i]
			in.Revision = rev
			in.Blob = git(t, root, "rev-parse", rev+":"+in.Path)
			b, _ := os.ReadFile(filepath.Join(root, in.Path))
			in.SHA256 = Digest(b)
		}
	}
	for i := range m.Scopes {
		if m.Scopes[i].Path == "shards" {
			m.Scopes[i].Revision = rev
		}
	}
	return m
}
func TestIncrementalShardParity(t *testing.T) {
	t.Run("RCP-V0-007 unchanged contribution reused changed contribution compiled with cold byte oracle", func(t *testing.T) {
		root, m := shardAdoptionFixture(t, func(first, last *ProviderRecord) { first.Observations = nil })
		ids := []ShardIdentity{{"adapter", "shards/first.json"}, {"adapter", "shards/last.json"}}
		_, cache, stats, e := BuildIncremental(context.Background(), root, m, ids, nil, "")
		if e != nil {
			t.Fatal(e)
		}
		if len(stats.Compiled) != 2 {
			t.Fatalf("first compilation %+v", stats)
		}
		raw, e := EncodeIncrementalCache(*cache)
		if e != nil {
			t.Fatal(e)
		}
		m = updateIncrementalShard(t, root, m, "shards/last.json", func(p *ProviderRecord) { p.Capabilities[0].Reason = "updated empty shard declaration" })
		a, _, stats, e := BuildIncremental(context.Background(), root, m, ids, raw, Digest(raw))
		if e != nil {
			t.Fatal(e)
		}
		if len(stats.Reused) != 1 || stats.Reused[0] != ids[0] || len(stats.Compiled) != 1 || stats.Compiled[0] != ids[1] {
			t.Fatalf("not discriminating %+v", stats)
		}
		cold, e := Build(context.Background(), root, m)
		if e != nil {
			t.Fatal(e)
		}
		got, _ := Encode(a)
		want, _ := Encode(cold)
		if !bytes.Equal(got, want) {
			t.Fatal("whole artifact differs from cold oracle")
		}
	})
}
func TestIncrementalCacheRefusals(t *testing.T) {
	t.Run("RCP-V0-007 closed contribution membership detail parity eligibility and digest", func(t *testing.T) {
		root, m := shardAdoptionFixture(t, func(first, last *ProviderRecord) { first.Observations = nil })
		ids := []ShardIdentity{{"adapter", "shards/first.json"}, {"adapter", "shards/last.json"}}
		_, cache, _, e := BuildIncremental(context.Background(), root, m, ids, nil, "")
		if e != nil {
			t.Fatal(e)
		}
		raw, _ := EncodeIncrementalCache(*cache)
		cases := map[string]func(*IncrementalCache){"membership": func(c *IncrementalCache) { c.Shards[0].Contribution.AdoptionProvider = false }, "typed detail": func(c *IncrementalCache) { delete(c.Shards[0].Contribution.Details, "adapter:claim") }, "import parity": func(c *IncrementalCache) {
			c.Shards[0].Contribution.ImportParity = c.Shards[0].Contribution.ImportParity[1:]
		}, "profile": func(c *IncrementalCache) { c.Schema = "unknown" }}
		for name, edit := range cases {
			t.Run(name, func(t *testing.T) {
				var altered IncrementalCache
				if e := decodeBounded(raw, &altered, MaxCorpusBytes); e != nil {
					t.Fatal(e)
				}
				edit(&altered)
				b, _ := EncodeIncrementalCache(altered)
				if _, _, _, e := BuildIncremental(context.Background(), root, m, ids, b, Digest(b)); e == nil {
					t.Fatal("altered contribution reused")
				}
			})
		}
		if _, _, _, e := BuildIncremental(context.Background(), root, m, ids[:1], raw, Digest(raw)); e == nil {
			t.Fatal("changed eligibility accepted")
		}
		if _, _, _, e := BuildIncremental(context.Background(), root, m, ids, raw, "bad"); e == nil {
			t.Fatal("wrong cache digest accepted")
		}
	})
	t.Run("RCP-V0-007 late restriction screened despite unchanged cached first shard", func(t *testing.T) {
		root, m := shardAdoptionFixture(t, func(first, last *ProviderRecord) {
			first.Observations = nil
			first.Subjects[0].Name = "new-secret-token"
		})
		ids := []ShardIdentity{{"adapter", "shards/first.json"}, {"adapter", "shards/last.json"}}
		_, cache, _, e := BuildIncremental(context.Background(), root, m, ids, nil, "")
		if e != nil {
			t.Fatal(e)
		}
		raw, _ := EncodeIncrementalCache(*cache)
		m = updateIncrementalShard(t, root, m, "shards/last.json", func(p *ProviderRecord) {
			p.RestrictedFindings = append(p.RestrictedFindings, RestrictedFinding{"new-secret-token", "high", "private/new.txt", "new restricted detail"})
		})
		if _, _, _, e := BuildIncremental(context.Background(), root, m, ids, raw, Digest(raw)); e == nil {
			t.Fatal("late restriction skipped by reuse")
		}
	})
	t.Run("RCP-V0-007 unsupported observation contribution has named cold fallback", func(t *testing.T) {
		root, m := shardAdoptionFixture(t, nil)
		ids := []ShardIdentity{{"adapter", "shards/first.json"}}
		_, _, stats, e := BuildIncremental(context.Background(), root, m, ids, nil, "")
		if e != nil {
			t.Fatal(e)
		}
		if len(stats.Reused) > 0 || len(stats.Fallback) != 1 || stats.FallbackReason == "" {
			t.Fatal("unsupported reuse reported success")
		}
	})
}
