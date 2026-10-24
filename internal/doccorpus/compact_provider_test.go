package doccorpus

import (
	"bytes"
	"context"
	json "encoding/json/v2"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

// TestDCXV1530CompactEncoderMatchesReference: the shipped compact encoder writes, byte for byte,
// what the DCX-V1-487 reference encoder writes for the same record and defaults, and refuses a
// record that is not /1 (DCX-V1-530).
func TestDCXV1530CompactEncoderMatchesReference(t *testing.T) {
	t.Parallel()
	root, _ := adoptionFixture(t, nil)
	fixture := readProvider(t, root)
	a0 := fixture.Subjects[0].Evidence.Anchors[0]
	synthetic := syntheticAnchoredRecord(600, 250, true)
	for name, record := range map[string]ProviderRecord{"adoption fixture": fixture, "synthetic inventory": synthetic, "no anchors": syntheticAnchoredRecord(40, 10, false)} {
		for _, defaults := range []*AnchorDefaults{nil, {Authority: a0.Authority, Kind: a0.Kind, Reason: a0.Reason}, {Authority: "other", Kind: "declared", Reason: ""}} {
			got, err := EncodeCompactProvider(record, defaults)
			if err != nil {
				t.Fatalf("%s: %v", name, err)
			}
			if want := compactEncode(t, record, defaults); !bytes.Equal(got, want) {
				t.Fatalf("%s %+v: compact encoder differs from the reference encoder", name, defaults)
			}
		}
	}
	// An empty member that differs from its default, and an empty file repository or revision
	// that differs from the source's, is kept present, so it expands to itself, not the default.
	empty := syntheticAnchoredRecord(4, 4, true)
	empty.Subjects[0].Evidence.Anchors[0].Reason, empty.Subjects[0].Evidence.Anchors[0].Authority = "", ""
	empty.Subjects[1].Evidence.Anchors[0].Repository, empty.Subjects[1].Evidence.Anchors[0].Revision = "", ""
	out, err := EncodeCompactProvider(empty, &AnchorDefaults{Authority: "external-provider", Kind: "declared", Reason: "fallback"})
	if err != nil {
		t.Fatal(err)
	}
	var expanded ProviderRecord
	if err := decodeProviderRecord("empty.json", out, &expanded); err != nil {
		t.Fatal(err)
	}
	want, err := json.Marshal(empty.Subjects, json.Deterministic(true))
	if err != nil {
		t.Fatal(err)
	}
	if got, err := json.Marshal(expanded.Subjects, json.Deterministic(true)); err != nil || !bytes.Equal(got, want) {
		t.Fatalf("explicit empty members did not survive the compact encoding:\n got %s\nwant %s", got, want)
	}
	compact := fixture
	compact.Schema = AdoptionProviderSchemaV2
	if _, err := EncodeCompactProvider(compact, nil); err == nil || !strings.Contains(err.Error(), "compact encoding needs a "+AdoptionProviderSchema+" record") {
		t.Fatalf("a /2 record was encoded again: %v", err)
	}
}

// referenceDefaults is the most frequent anchor authority, kind and reason in raw, ties to the
// smallest value, read from the generic JSON tree instead of the typed record.
func referenceDefaults(t *testing.T, raw []byte) *AnchorDefaults {
	t.Helper()
	var tree any
	if err := json.Unmarshal(raw, &tree); err != nil {
		t.Fatal(err)
	}
	counts := [3]map[string]int{{}, {}, {}}
	anchors := 0
	var walk func(v any)
	walk = func(v any) {
		switch v := v.(type) {
		case map[string]any:
			if _, ok := v["span_sha256"]; ok {
				anchors++
				for i, member := range []string{"authority", "evidence_kind", "reason"} {
					s, _ := v[member].(string)
					counts[i][s]++
				}
			}
			for _, child := range v {
				walk(child)
			}
		case []any:
			for _, child := range v {
				walk(child)
			}
		}
	}
	walk(tree)
	if anchors == 0 {
		return nil
	}
	var top [3]string
	for i, values := range counts {
		keys := make([]string, 0, len(values))
		for k := range values {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		best := 0
		for _, k := range keys {
			if values[k] > best {
				top[i], best = k, values[k]
			}
		}
	}
	return &AnchorDefaults{Authority: top[0], Kind: top[1], Reason: top[2]}
}

// TestDCXV1531CompactProviderProducer: docs corpus compact-provider rewrites a /1 record or shard
// as the reference encoding with the most frequent anchor members as defaults, the build admits
// the same records from either form, the output is deterministic, and a /2, non-adoption or
// malformed input is refused (DCX-V1-531).
func TestDCXV1531CompactProviderProducer(t *testing.T) {
	t.Parallel()
	root, m := adoptionFixture(t, nil)
	legacy, err := Build(context.Background(), root, m)
	if err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(filepath.Join(root, "evidence/provider.json"))
	if err != nil {
		t.Fatal(err)
	}
	got, err := CompactProvider("evidence/provider.json", raw)
	if err != nil {
		t.Fatal(err)
	}
	defaults := referenceDefaults(t, raw)
	if defaults == nil {
		t.Fatal("fixture lacks anchors")
	}
	if want := compactEncode(t, readProvider(t, root), defaults); !bytes.Equal(got, want) {
		t.Fatal("compact-provider output differs from the reference encoding")
	}
	if again, err := CompactProvider("evidence/provider.json", raw); err != nil || !bytes.Equal(again, got) {
		t.Fatalf("compact-provider is not deterministic: %v", err)
	}
	compact, err := Build(context.Background(), root, repinProvider(t, root, m, got))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(corpusRecords(t, compact), corpusRecords(t, legacy)) {
		t.Fatal("the /2 output admitted different records from its /1 input")
	}

	t.Run("shard", func(t *testing.T) {
		root, m := shardAdoptionFixture(t, nil)
		legacy, err := Build(context.Background(), root, m)
		if err != nil {
			t.Fatal(err)
		}
		for _, p := range m.Providers {
			for _, shard := range p.Shards {
				data, err := os.ReadFile(filepath.Join(root, shard))
				if err != nil {
					t.Fatal(err)
				}
				var record ProviderRecord
				if err := decodeBounded(data, &record, MaxProviderBytes); err != nil {
					t.Fatal(err)
				}
				out, err := CompactProvider(shard, data)
				if err != nil {
					t.Fatal(err)
				}
				if want := compactEncode(t, record, referenceDefaults(t, data)); !bytes.Equal(out, want) {
					t.Fatalf("%s: compact-provider output differs from the reference encoding", shard)
				}
				m = repinShard(t, root, m, shard, out)
			}
		}
		compact, err := Build(context.Background(), root, m)
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(corpusRecords(t, compact), corpusRecords(t, legacy)) {
			t.Fatal("the /2 shards admitted different records from their /1 inputs")
		}
	})

	t.Run("ties and records without anchors", func(t *testing.T) {
		record := syntheticAnchoredRecord(4, 4, true)
		record.Capabilities = []CapabilityDeclaration{}
		for i, reason := range []string{"b", "b", "a", "a"} {
			record.Subjects[i].Evidence.Anchors[0].Reason = reason
		}
		record.Subjects[0].Evidence.Anchors[0].Kind = "x"
		data, err := Encode(record)
		if err != nil {
			t.Fatal(err)
		}
		out, err := CompactProvider("tie.json", data)
		if err != nil {
			t.Fatal(err)
		}
		if want := compactEncode(t, record, &AnchorDefaults{Authority: "external-provider", Kind: "declared", Reason: "a"}); !bytes.Equal(out, want) {
			t.Fatal("a tie did not go to the smallest value")
		}
		bare := syntheticAnchoredRecord(3, 3, false)
		bare.Capabilities = []CapabilityDeclaration{}
		if data, err = Encode(bare); err != nil {
			t.Fatal(err)
		}
		if out, err = CompactProvider("bare.json", data); err != nil {
			t.Fatal(err)
		}
		if bytes.Contains(out, []byte(`"anchor_defaults"`)) || !bytes.Equal(out, compactEncode(t, bare, nil)) {
			t.Fatal("a record without anchors gained anchor defaults")
		}
	})

	for name, input := range map[string][]byte{
		"compact input":      got,
		"non-adoption input": bytes.Replace(raw, []byte(AdoptionProviderSchema), []byte("corvint-corpus-other-provider/1"), 1),
		"malformed input":    raw[:len(raw)/2],
	} {
		if out, err := CompactProvider("evidence/provider.json", input); err == nil || out != nil {
			t.Fatalf("%s was not refused", name)
		}
	}
}

// repinShard commits data as shard and repins the manifest's provider, scope and input to it.
func repinShard(t *testing.T, root string, m Manifest, shard string, data []byte) Manifest {
	t.Helper()
	if err := os.WriteFile(filepath.Join(root, shard), data, 0600); err != nil {
		t.Fatal(err)
	}
	git(t, root, "add", shard)
	git(t, root, "commit", "-qm", "compact shard")
	rev := git(t, root, "rev-parse", "HEAD")
	m.Inputs = append([]Input(nil), m.Inputs...)
	m.Scopes = append([]Scope(nil), m.Scopes...)
	m.Providers = append([]Provider(nil), m.Providers...)
	for i, in := range m.Inputs {
		if in.Purpose == "provider" {
			current, err := os.ReadFile(filepath.Join(root, in.Path))
			if err != nil {
				t.Fatal(err)
			}
			m.Inputs[i].Revision, m.Inputs[i].Blob, m.Inputs[i].SHA256 = rev, git(t, root, "rev-parse", rev+":"+in.Path), Digest(current)
		}
	}
	for i, s := range m.Scopes {
		for _, in := range m.Inputs {
			if in.Purpose == "provider" && (s.Path == in.Path || strings.HasPrefix(in.Path, strings.TrimSuffix(s.Path, "/")+"/")) {
				m.Scopes[i].Revision = rev
			}
		}
	}
	for i, p := range m.Providers {
		if len(p.Shards) > 0 || p.Record != "" {
			m.Providers[i].Revision = rev
		}
	}
	return m
}
