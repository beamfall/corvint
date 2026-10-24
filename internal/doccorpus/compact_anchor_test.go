package doccorpus

import (
	"bytes"
	"context"
	"encoding/json/jsontext"
	json "encoding/json/v2"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// compactEncode is the test-side producer of the compact adoption encoding (DCX-V1-487): it
// writes record as a corvint-corpus-adoption-provider/2 record whose anchors index a file table
// built in first-appearance order, omitting a file's repository and revision when they are the
// record's source and an anchor's authority, kind and reason when they equal defaults.
func compactEncode(t testing.TB, record ProviderRecord, defaults *AnchorDefaults) []byte {
	t.Helper()
	type fileKey struct{ repository, revision, path, blob, sha256 string }
	index := map[fileKey]int{}
	files := []AnchorFile{}
	unlessEqual := func(value, implied string) *string {
		if value == implied {
			return nil
		}
		return &value
	}
	var d AnchorDefaults
	if defaults != nil {
		d = *defaults
	}
	compact := json.MarshalToFunc(func(enc *jsontext.Encoder, a Anchor) error {
		key := fileKey{a.Repository, a.Revision, a.Path, a.Blob, a.SHA256}
		i, ok := index[key]
		if !ok {
			i = len(files)
			index[key] = i
			files = append(files, AnchorFile{Repository: unlessEqual(a.Repository, record.Source.ID), Revision: unlessEqual(a.Revision, record.Source.Revision), Path: a.Path, Blob: a.Blob, SHA256: a.SHA256})
		}
		c := CompactAnchor{File: &i, Start: a.Start, End: a.End, SpanSHA256: a.SpanSHA256, Symbol: a.Symbol, Region: a.Region,
			Authority: &a.Authority, Kind: &a.Kind, Reason: &a.Reason}
		if defaults != nil {
			c.Authority, c.Kind, c.Reason = unlessEqual(a.Authority, d.Authority), unlessEqual(a.Kind, d.Kind), unlessEqual(a.Reason, d.Reason)
		}
		return json.MarshalEncode(enc, c)
	})
	options := json.JoinOptions(json.Deterministic(true), json.WithMarshalers(compact))
	// The first pass fills the file table; the second writes the record with it.
	if _, err := json.Marshal(record, options); err != nil {
		t.Fatal(err)
	}
	record.Schema = AdoptionProviderSchemaV2
	raw, err := json.Marshal(compactRecord{ProviderRecord: record, Files: files, AnchorDefaults: defaults}, options)
	if err != nil {
		t.Fatal(err)
	}
	return append(raw, '\n')
}

// repinProvider replaces the adapter's provider record with raw, committed at a new revision.
func repinProvider(t *testing.T, root string, m Manifest, raw []byte) Manifest {
	t.Helper()
	const path = "evidence/provider.json"
	if err := os.WriteFile(filepath.Join(root, path), raw, 0600); err != nil {
		t.Fatal(err)
	}
	git(t, root, "add", path)
	git(t, root, "commit", "-qm", "compact provider")
	rev := git(t, root, "rev-parse", "HEAD")
	m.Inputs = append([]Input(nil), m.Inputs...)
	m.Scopes = append([]Scope(nil), m.Scopes...)
	m.Providers = append([]Provider(nil), m.Providers...)
	for i := range m.Inputs {
		if m.Inputs[i].Path == path {
			m.Inputs[i].Revision, m.Inputs[i].Blob, m.Inputs[i].SHA256 = rev, git(t, root, "rev-parse", rev+":"+path), Digest(raw)
		}
	}
	for i := range m.Scopes {
		if m.Scopes[i].Path == path {
			m.Scopes[i].Revision = rev
		}
	}
	for i := range m.Providers {
		if m.Providers[i].ID == "adapter" {
			m.Providers[i].Revision = rev
		}
	}
	return m
}

func readProvider(t *testing.T, root string) ProviderRecord {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(root, "evidence/provider.json"))
	if err != nil {
		t.Fatal(err)
	}
	var record ProviderRecord
	if err = decodeBounded(raw, &record, MaxProviderBytes); err != nil {
		t.Fatal(err)
	}
	return record
}

// corpusRecords is the build's admitted records, the part a provider encoding may not change.
func corpusRecords(t *testing.T, a *Artifact) []byte {
	t.Helper()
	raw, err := Encode(struct {
		Subjects  []Subject
		Claims    []Claim
		Relations []Relation
		Journeys  []Journey
		Details   map[string]RecordDetails
	}{a.Subjects, a.Claims, a.Relations, a.Journeys, a.Details})
	if err != nil {
		t.Fatal(err)
	}
	return raw
}

func TestDCXV1487CompactAnchorTableEquivalent(t *testing.T) {
	t.Parallel()
	root, m := adoptionFixture(t, nil)
	legacy, err := Build(context.Background(), root, m)
	if err != nil {
		t.Fatal(err)
	}
	want := corpusRecords(t, legacy)
	record := readProvider(t, root)
	if len(record.Subjects) == 0 || len(record.Subjects[0].Evidence.Anchors) == 0 {
		t.Fatal("fixture lacks anchors")
	}
	a0 := record.Subjects[0].Evidence.Anchors[0]
	for _, tc := range []struct {
		name     string
		defaults *AnchorDefaults
	}{
		{"file table only", nil},
		{"file table and anchor defaults", &AnchorDefaults{Authority: a0.Authority, Kind: a0.Kind, Reason: a0.Reason}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			raw := compactEncode(t, record, tc.defaults)
			if bytes.Count(raw, []byte(a0.Blob)) != 1 {
				t.Fatal("compact record repeats a file identity")
			}
			if tc.defaults != nil && bytes.Count(raw, []byte(a0.Reason)) != 1 {
				t.Fatal("compact record repeats the default reason")
			}
			compact := repinProvider(t, root, m, raw)
			got, err := Build(context.Background(), root, compact)
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(corpusRecords(t, got), want) {
				t.Fatal("compact record admitted different records from its /1 form")
			}
			encoded, err := Encode(got)
			if err != nil {
				t.Fatal(err)
			}
			if _, err = Open(context.Background(), root, encoded); err != nil {
				t.Fatal(err)
			}
		})
	}
	t.Run("old /1 shards stay readable", func(t *testing.T) {
		root, m := shardAdoptionFixture(t, nil)
		if _, err := Build(context.Background(), root, m); err != nil {
			t.Fatal(err)
		}
	})
}

func TestDCXV1488CompactAnchorRefusals(t *testing.T) {
	t.Parallel()
	base := func(t *testing.T) (string, Manifest, ProviderRecord, map[string]any) {
		root, m := adoptionFixture(t, nil)
		record := readProvider(t, root)
		var doc map[string]any
		if err := json.Unmarshal(compactEncode(t, record, nil), &doc); err != nil {
			t.Fatal(err)
		}
		return root, m, record, doc
	}
	firstAnchor := func(doc map[string]any) map[string]any {
		subject := doc["subjects"].([]any)[0].(map[string]any)
		return subject["evidence"].(map[string]any)["anchors"].([]any)[0].(map[string]any)
	}
	for _, tc := range []struct {
		name, want string
		edit       func(doc map[string]any, record ProviderRecord)
	}{
		{"index out of range", "outside the file table", func(doc map[string]any, _ ProviderRecord) { firstAnchor(doc)["file"] = 99 }},
		{"negative index", "outside the file table", func(doc map[string]any, _ ProviderRecord) { firstAnchor(doc)["file"] = -1 }},
		{"missing index", "lacks its file index", func(doc map[string]any, _ ProviderRecord) { delete(firstAnchor(doc), "file") }},
		{"full anchor member in compact anchor", "unknown member", func(doc map[string]any, record ProviderRecord) {
			firstAnchor(doc)["path"] = record.Subjects[0].Evidence.Anchors[0].Path
		}},
		{"unknown file table member", "/files/0/start_line: unknown member", func(doc map[string]any, _ ProviderRecord) {
			doc["files"].([]any)[0].(map[string]any)["start_line"] = 1
		}},
		{"unknown default member", "/anchor_defaults/path: unknown member", func(doc map[string]any, _ ProviderRecord) {
			doc["anchor_defaults"] = map[string]any{"path": "src/value.go"}
		}},
		{"file table in a /1 record", "unknown member", func(doc map[string]any, record ProviderRecord) {
			raw, _ := Encode(record)
			var legacy map[string]any
			_ = json.Unmarshal(raw, &legacy)
			for k := range doc {
				delete(doc, k)
			}
			for k, v := range legacy {
				doc[k] = v
			}
			doc["files"] = []any{}
		}},
		{"anchor defaults in a /1 record", "unknown member", func(doc map[string]any, record ProviderRecord) {
			raw, _ := Encode(record)
			var legacy map[string]any
			_ = json.Unmarshal(raw, &legacy)
			for k := range doc {
				delete(doc, k)
			}
			for k, v := range legacy {
				doc[k] = v
			}
			doc["anchor_defaults"] = map[string]any{}
		}},
		{"wrong file revision", "", func(doc map[string]any, _ ProviderRecord) {
			doc["files"].([]any)[0].(map[string]any)["revision"] = strings.Repeat("0", 40)
		}},
		{"explicit empty reason keeps no default", "", func(doc map[string]any, _ ProviderRecord) {
			doc["anchor_defaults"] = map[string]any{"reason": "fallback"}
			firstAnchor(doc)["reason"] = ""
		}},
		{"explicit empty file revision keeps no source default", "", func(doc map[string]any, _ ProviderRecord) {
			doc["files"].([]any)[0].(map[string]any)["revision"] = ""
		}},
		{"file table bound", "/files: anchor file table bound exceeded", func(doc map[string]any, _ ProviderRecord) {
			table := make([]any, maxAnchorFiles+1)
			for i := range table {
				table[i] = map[string]any{}
			}
			doc["files"] = table
		}},
		{"compact record on a /1 manifest", "", nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root, m, record, doc := base(t)
			if tc.edit != nil {
				tc.edit(doc, record)
			}
			raw, err := json.Marshal(doc, json.Deterministic(true))
			if err != nil {
				t.Fatal(err)
			}
			m = repinProvider(t, root, m, raw)
			if tc.edit == nil {
				m.Schema = ManifestSchema
			}
			_, err = Build(context.Background(), root, m)
			if err == nil {
				t.Fatal("invalid compact record admitted")
			}
			if tc.want != "" && (!strings.Contains(err.Error(), tc.want) || !strings.Contains(err.Error(), "evidence/provider.json")) {
				t.Fatalf("refusal %q lacks %q at its located input", err, tc.want)
			}
			if anchorCase := tc.want == "outside the file table" || tc.want == "lacks its file index"; anchorCase && !strings.Contains(err.Error(), "/subjects/0/evidence/anchors/0") {
				t.Fatalf("refusal %q lacks the anchor's pointer", err)
			}
		})
	}
}

// syntheticAnchoredRecord is an adoption record shaped like #1664's corpus: one exact one-line
// anchor on each of subjects inventory units spread over files source files, the first files
// units file-level and the rest symbol units.
func syntheticAnchoredRecord(subjects, files int, anchored bool) ProviderRecord {
	hex := func(seed string, n int) string { return Digest([]byte(seed))[:n] }
	source := Repository{ID: "app", Revision: hex("revision", 40)}
	record := ProviderRecord{Schema: AdoptionProviderSchema, ID: "inventory", Version: "1", Source: source, Subjects: make([]Subject, 0, subjects), Claims: []Claim{}, Relations: []Relation{}, Journeys: []Journey{}, Observations: []ObservationLink{}}
	for i := 0; i < subjects; i++ {
		file := i % files
		path := fmt.Sprintf("src/module_%03d/component_%05d.go", file%300, file)
		id, kind, symbol := "inventory:file:"+path, "file", ""
		if i >= files {
			symbol = fmt.Sprintf("Component%05dHandler%d", file, i/files)
			id, kind = "inventory:symbol:"+path+":"+symbol, "symbol"
		}
		ev := Evidence{ReportedTrust: "declared", Derivation: "provider", Trust: "declared", State: "present", Freshness: "fresh", Anchors: []Anchor{}, Limitations: []string{}}
		if anchored {
			line := 1 + i/files*7
			ev.Anchors = []Anchor{{Repository: source.ID, Revision: source.Revision, Path: path, Blob: hex("blob"+path, 40), SHA256: hex("file"+path, 64), Start: line, End: line, SpanSHA256: hex(fmt.Sprint("span", i), 64), Symbol: symbol, Authority: "external-provider", Kind: "declared", Reason: "inventory unit declared at its exact source line"}}
		}
		record.Subjects = append(record.Subjects, Subject{ID: id, Kind: kind, Name: id, Provider: "inventory", Evidence: ev})
	}
	return record
}

// TestDCXV1489CompactAnchorSizeMeasured measures the anchor bytes of #1664's shape (about 28,500
// anchored inventory subjects over about 17,400 files) in both encodings and checks the /2 form
// expands to exactly the /1 records.
func TestDCXV1489CompactAnchorSizeMeasured(t *testing.T) {
	t.Parallel()
	if maxPinnedInputBytes != 128<<20 || maxProviderRecordBytes != 128<<20 {
		t.Fatal("DCX-V1-489 keeps both aggregate bounds at 128 MiB")
	}
	for _, shape := range []struct {
		subjects, files int
		maxRatio        float64
	}{{28500, 17400, 0.50}, {28500, 2000, 0.35}} {
		t.Run(fmt.Sprintf("%d anchors over %d files", shape.subjects, shape.files), func(t *testing.T) {
			bare, err := Encode(syntheticAnchoredRecord(shape.subjects, shape.files, false))
			if err != nil {
				t.Fatal(err)
			}
			record := syntheticAnchoredRecord(shape.subjects, shape.files, true)
			legacy, err := Encode(record)
			if err != nil {
				t.Fatal(err)
			}
			compact := compactEncode(t, record, &AnchorDefaults{Authority: "external-provider", Kind: "declared", Reason: "inventory unit declared at its exact source line"})
			legacyAnchors, compactAnchors := len(legacy)-len(bare), len(compact)-len(bare)
			ratio := float64(compactAnchors) / float64(legacyAnchors)
			t.Logf("record without anchors %d bytes; /1 %d bytes (+%d, %.0f per anchor); /2 %d bytes (+%d, %.0f per anchor); /2 anchor bytes are %.1f%% of /1",
				len(bare), len(legacy), legacyAnchors, float64(legacyAnchors)/float64(shape.subjects), len(compact), compactAnchors, float64(compactAnchors)/float64(shape.subjects), 100*ratio)
			if ratio > shape.maxRatio {
				t.Fatalf("compact anchors cost %.1f%% of /1 anchor bytes, above %.0f%%", 100*ratio, 100*shape.maxRatio)
			}
			var plain, expanded ProviderRecord
			began := time.Now()
			if err := decodeProviderRecord("shard.json", legacy, &plain); err != nil {
				t.Fatal(err)
			}
			legacyDecode, began := time.Since(began), time.Now()
			if err := decodeProviderRecord("shard.json", compact, &expanded); err != nil {
				t.Fatal(err)
			}
			t.Logf("decode /1 %s, /2 %s", legacyDecode, time.Since(began))
			again, err := Encode(expanded)
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(again, legacy) {
				t.Fatal("compact record does not expand to its /1 form")
			}
		})
	}
}
