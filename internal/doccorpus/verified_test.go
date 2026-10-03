package doccorpus

import (
	"bytes"
	"context"
	json "encoding/json/v2"
	"testing"
)

func TestVerifiedCorpusIsolationAndReconstruction(t *testing.T) {
	t.Run("RCP-V0-007 DCP-V1-040 opaque immutable result and original source reconstruction", func(t *testing.T) {
		ctx := context.Background()
		root, m := shardAdoptionFixture(t, func(first, last *ProviderRecord) { first.Observations = nil })
		ids := []ShardIdentity{{"adapter", "shards/first.json"}, {"adapter", "shards/last.json"}}
		v, cache, _, err := BuildIncrementalVerified(ctx, root, m, ids, nil, "")
		if err != nil {
			t.Fatal(err)
		}
		raw, _ := v.Bytes()
		prior, _ := EncodeIncrementalCache(*cache)
		opened, stats, err := OpenIncrementalVerified(ctx, root, raw, ids, prior, Digest(prior))
		if err != nil || len(stats.Compiled) != 0 || len(stats.Reused) != 2 {
			t.Fatalf("prior rederivation: %+v %v", stats, err)
		}
		cold, err := OpenVerified(ctx, root, raw)
		if err != nil {
			t.Fatal(err)
		}
		coldRaw, _ := cold.Bytes()
		warmRaw, _ := opened.Bytes()
		if !bytes.Equal(raw, coldRaw) || !bytes.Equal(raw, warmRaw) {
			t.Fatal("cold and warm source reconstruction differ")
		}
		copy, _ := v.Bytes()
		copy[0] = '['
		snapshot, _ := v.Snapshot()
		snapshot.Subjects[0].Name = "caller mutation"
		after, _ := v.Bytes()
		fresh, _ := v.Snapshot()
		if !bytes.Equal(raw, after) || fresh.Subjects[0].Name == "caller mutation" {
			t.Fatal("returned bytes/snapshot alias token")
		}
		var zero VerifiedCorpus
		if _, err := zero.Bytes(); err == nil {
			t.Fatal("zero token bytes admitted")
		}
		if _, err := zero.Snapshot(); err == nil {
			t.Fatal("zero token snapshot admitted")
		}
		if err := json.Unmarshal([]byte(`{"canonical":"forged"}`), &zero, json.RejectUnknownMembers(true)); err == nil {
			t.Fatal("wire token admitted")
		}
		if err := json.Unmarshal([]byte(`{}`), &zero); err == nil {
			t.Fatal("opaque token unexpectedly decoded")
		}
		if _, err := zero.Snapshot(); err == nil {
			t.Fatal("empty decoded token admitted")
		}
		forged, _ := v.Snapshot()
		forged.Subjects[0].Name = "self-consistent source forgery"
		forged.SHA256 = ""
		hashed, _ := Encode(forged)
		forged.SHA256 = Digest(hashed)
		altered, _ := Encode(forged)
		if _, err := ParseArtifact(altered); err != nil {
			t.Fatal("forgery should pass byte-only identity", err)
		}
		if _, err := OpenVerified(ctx, root, altered); err == nil {
			t.Fatal("cold admitted invented source record")
		}
		if _, _, err := OpenIncrementalVerified(ctx, root, altered, ids, prior, Digest(prior)); err == nil {
			t.Fatal("warm admitted invented source record")
		}
	})
}

func TestIncrementalCompleteSourceCorrespondence(t *testing.T) {
	t.Run("RCP-V0-007 complete retained fields and unauthenticated reviewed trust", func(t *testing.T) {
		ctx := context.Background()
		root, m := shardAdoptionFixture(t, func(first, last *ProviderRecord) {
			first.Observations = nil
			ev := Evidence{Derivation: "declared", Trust: "reviewed", State: "unknown", Freshness: "unknown", Unknown: "synthetic declared scenario", Anchors: []Anchor{}, Limitations: []string{}}
			first.Subjects[0].Evidence = ev
			first.Journeys = []Journey{{ID: "adapter:scenario", Subject: first.Subjects[0].ID, Provider: "adapter", Status: "generated_not_verified", Preconditions: []string{"synthetic setup"}, Cleanup: "not-run", Evidence: ev, Steps: []Step{{ID: "adapter:step", Action: "read", Operation: "read", Expected: "declared result", Evidence: ev}}}}
			for i := range first.Capabilities {
				if first.Capabilities[i].Name == "journeys" {
					first.Capabilities[i].State, first.Capabilities[i].Reason = "present", "synthetic scenario"
				}
			}
		})
		ids := []ShardIdentity{{"adapter", "shards/first.json"}, {"adapter", "shards/last.json"}}
		a, cache, _, err := BuildIncremental(ctx, root, m, ids, nil, "")
		if err != nil {
			t.Fatal(err)
		}
		raw, _ := EncodeIncrementalCache(*cache)
		warm, _, _, err := BuildIncremental(ctx, root, m, ids, raw, Digest(raw))
		if err != nil {
			t.Fatal(err)
		}
		cold, err := Build(ctx, root, m)
		if err != nil {
			t.Fatal(err)
		}
		actual, _ := Encode(warm)
		original, _ := Encode(a)
		oracle, _ := Encode(cold)
		if !bytes.Equal(actual, original) || !bytes.Equal(actual, oracle) {
			t.Fatal("normalized reviewed evidence cold/warm differs")
		}
		var normalized *Subject
		for i := range warm.Subjects {
			if warm.Subjects[i].ID == cache.Shards[0].Contribution.Subjects[0].ID {
				normalized = &warm.Subjects[i]
			}
		}
		if normalized == nil || normalized.Evidence.Trust != "generated" || normalized.Evidence.ReportedTrust != "reviewed" {
			t.Fatal("provider review acquired authority")
		}
		cases := map[string]func(*ShardContribution){
			"subject name":           func(x *ShardContribution) { x.Subjects[0].Name = "invented" },
			"subject kind":           func(x *ShardContribution) { x.Subjects[0].Kind = "module" },
			"subject provider":       func(x *ShardContribution) { x.Subjects[0].Provider = "invented" },
			"subject evidence":       func(x *ShardContribution) { x.Subjects[0].Evidence.Unknown = "invented" },
			"reviewed normalization": func(x *ShardContribution) { x.Subjects[0].Evidence.ReportedTrust = "" },
			"claim text":             func(x *ShardContribution) { x.Claims[0].Text = "invented" },
			"claim subject":          func(x *ShardContribution) { x.Claims[0].Subject = "invented" },
			"claim evidence":         func(x *ShardContribution) { x.Claims[0].Evidence.Unknown = "invented" },
			"relation endpoints": func(x *ShardContribution) {
				x.Relations[0].From, x.Relations[0].To = x.Relations[0].To, x.Relations[0].From
			},
			"relation type": func(x *ShardContribution) {
				if x.Relations[0].Type == "related_to" {
					x.Relations[0].Type = "depends_on"
				} else {
					x.Relations[0].Type = "related_to"
				}
			},
			"relation evidence":     func(x *ShardContribution) { x.Relations[0].Evidence.Unknown = "invented" },
			"journey status":        func(x *ShardContribution) { x.Journeys[0].Status = "blocked" },
			"journey precondition":  func(x *ShardContribution) { x.Journeys[0].Preconditions[0] = "invented" },
			"journey cleanup":       func(x *ShardContribution) { x.Journeys[0].Cleanup = "invented" },
			"journey step":          func(x *ShardContribution) { x.Journeys[0].Steps[0].Expected = "invented" },
			"journey step evidence": func(x *ShardContribution) { x.Journeys[0].Steps[0].Evidence.Unknown = "invented" },
			"declaration":           func(x *ShardContribution) { x.Declarations[0].Reason = "invented" },
			"membership":            func(x *ShardContribution) { x.AdoptionProvider = false },
			"parity":                func(x *ShardContribution) { x.ImportParity[0].Admitted++ },
		}
		for name, edit := range cases {
			t.Run(name, func(t *testing.T) {
				var changed IncrementalCache
				if err := decodeBounded(raw, &changed, MaxCorpusBytes); err != nil {
					t.Fatal(err)
				}
				edit(&changed.Shards[0].Contribution)
				forged, _ := EncodeIncrementalCache(changed)
				if _, _, _, err := BuildIncremental(ctx, root, m, ids, forged, Digest(forged)); err == nil {
					t.Fatal("matching outer pin admitted fields absent from original source")
				}
			})
		}
	})
}
