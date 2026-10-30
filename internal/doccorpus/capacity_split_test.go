package doccorpus

import (
	"errors"
	"strings"
	"testing"
)

// TestDCXV1472ProviderRecordBytesChargedApart pins that provider record shards and the other
// pinned inputs are charged to separate 128 MiB bounds, and that a refusal names both totals and
// the largest inputs of the class that is over (DCX-V1-472).
func TestDCXV1472ProviderRecordBytesChargedApart(t *testing.T) {
	t.Parallel()
	source := func(p string) Input { return Input{Path: p, Purpose: "source"} }
	shard := func(p string) Input { return Input{Path: p, Purpose: "provider"} }
	// GitHub #1664: about 39 MB of sources and 100 MB of provider shards now build.
	var within inputCharge
	within.add(source("app/models/user.rb"), 39<<20)
	for _, p := range []string{"corpus-provider/shard-001.json", "corpus-provider/shard-002.json"} {
		within.add(shard(p), 50<<20)
	}
	if err := within.check(); err != nil {
		t.Fatalf("sources and provider shards each within their bound refused: %v", err)
	}
	cases := []struct {
		name  string
		build func(*inputCharge)
		want  []string
	}{
		{"providers over", func(c *inputCharge) {
			c.add(source("app/models/user.rb"), 1<<20)
			c.add(shard("corpus-provider/shard-001.json"), 60<<20)
			c.add(shard("corpus-provider/shard-002.json"), 63<<20)
			c.add(shard("corpus-provider/shard-003.json"), 6<<20)
			c.add(shard("corpus-provider/shard-004.json"), 1<<20)
		}, []string{
			"aggregate provider record bytes 136314880 exceeds the limit of 134217728",
			"source and other inputs 1048576 bytes (limit 134217728), provider record shards 136314880 bytes (limit 134217728)",
			"largest: corpus-provider/shard-002.json (66060288), corpus-provider/shard-001.json (62914560), corpus-provider/shard-003.json (6291456)",
		}},
		{"sources over", func(c *inputCharge) {
			c.add(source("docs/a.md"), 100<<20)
			c.add(source("docs/b.md"), 29<<20)
			c.add(shard("corpus-provider/shard-001.json"), 64<<20)
		}, []string{
			"aggregate pinned input bytes 135266304 exceeds the limit of 134217728",
			"largest: docs/a.md (104857600), docs/b.md (30408704)",
		}},
	}
	for _, tc := range cases {
		var c inputCharge
		tc.build(&c)
		var refused *Error
		if err := c.check(); !errors.As(err, &refused) || refused.Code != CapacityCode {
			t.Fatalf("%s: %v, want %s", tc.name, err, CapacityCode)
		}
		for _, want := range tc.want {
			if !strings.Contains(refused.Message, want) {
				t.Fatalf("%s: %q lacks %q", tc.name, refused.Message, want)
			}
		}
		if tc.name == "sources over" && strings.Contains(refused.Message, "largest: corpus-provider") {
			t.Fatalf("source refusal named provider shards as its contributors: %q", refused.Message)
		}
	}
}
