package doccorpus

import (
	"context"
	json "encoding/json/v2"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// extractBuild commits an inventory's provider member and builds it through a manifest of the
// given schema (DCX-V1-393, DCX-V1-560).
func extractBuild(t *testing.T, root, rev, schema string, inv ExtractorInventory) *Artifact {
	t.Helper()
	m, err := InventoryScopes(context.Background(), root, rev, []string{"app"}, schema, "2026-10-24T00:00:00Z")
	if err != nil {
		t.Fatal(err)
	}
	record, err := Encode(inv.Provider)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(root, "evidence"), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "evidence/extract.json"), record, 0600); err != nil {
		t.Fatal(err)
	}
	git(t, root, "add", "evidence/extract.json")
	git(t, root, "commit", "-qm", "extractor record")
	recordRevision := git(t, root, "rev-parse", "HEAD")
	m.Providers = append(m.Providers, Provider{ID: "app", Kind: "records", Version: "1", Revision: recordRevision, Record: "evidence/extract.json"})
	m.Scopes = append(m.Scopes, Scope{"evidence/extract.json", recordRevision})
	m.Inputs = append(m.Inputs, Input{Path: "evidence/extract.json", Revision: recordRevision, Blob: git(t, root, "rev-parse", recordRevision+":evidence/extract.json"), SHA256: Digest(record), Provider: "app", Purpose: "provider"})
	a, err := Build(context.Background(), root, m)
	if err != nil {
		t.Fatal(err)
	}
	return a
}

func extractSchema(t *testing.T, root, rev, schema string) (ExtractorInventory, error) {
	t.Helper()
	data, err := BuildExtractorInventory(context.Background(), ExtractorInput{Root: root, Revision: rev, Profile: "corvint/extract.json", Schema: schema})
	var inv ExtractorInventory
	if err == nil {
		if err := json.Unmarshal(data, &inv); err != nil {
			t.Fatal(err)
		}
	}
	return inv, err
}

// DCX-V1-560, DCX-V1-561 (GitHub #1700): 14,000 distinct definitions, past #1700's 13,051, cross the provider/1 record
// bound. The default refusal names the count reached, the bound, the extractor and the path, and
// the --schema 2 bound; --schema 2 emits an adoption provider record a manifest /2 builds.
func TestDCXV1560ExtractorSchemaTwoAdmitsMoreThanTheRecordBound(t *testing.T) {
	t.Parallel()
	more := map[string]string{}
	for f := 0; f < 14; f++ {
		var b strings.Builder
		for n := 0; n < 1000; n++ {
			b.WriteString("def m" + strconv.Itoa(f*1000+n) + "\n")
		}
		more["app/syms_"+strconv.Itoa(f)+".rb"] = b.String()
	}
	root, rev := extractFixtureFiles(t, false, more, nil)
	for _, schema := range []string{"", "1"} {
		_, err := extractSchema(t, root, rev, schema)
		tkRefused(t, err, `extractor inventory exceeds the record bound: 4097 sites at extractor "handlers" path app/syms_`)
		tkRefused(t, err, "over the bound of 4096; --schema 2 emits an adoption provider record for a manifest /2, which admits 100000")
	}
	if _, err := extractSchema(t, root, rev, "3"); err == nil || !strings.Contains(err.Error(), "--schema must be 1 or 2") {
		t.Fatalf("an unknown schema should be refused: %v", err)
	}
	inv, err := extractSchema(t, root, rev, "2")
	if err != nil {
		t.Fatal(err)
	}
	if inv.Provider.Schema != AdoptionProviderSchema || len(inv.Provider.Subjects) < 14000 {
		t.Fatalf("--schema 2 should emit an adoption provider record of every site: %s %d", inv.Provider.Schema, len(inv.Provider.Subjects))
	}
	again, err := BuildExtractorInventory(context.Background(), ExtractorInput{Root: root, Revision: rev, Profile: "corvint/extract.json", Schema: "2"})
	if err != nil {
		t.Fatal(err)
	}
	if first, err := Encode(inv); err != nil || string(first) != string(again) {
		t.Fatal("--schema 2 extraction is not deterministic")
	}
	a := extractBuild(t, root, rev, ManifestSchemaV2, inv)
	found := 0
	for _, s := range a.Subjects {
		if strings.HasPrefix(s.ID, "app:symbol:m") {
			found++
		}
	}
	if found != 14000 {
		t.Fatalf("the manifest /2 build admits %d of 14000 extracted definitions", found)
	}
	// The provider/1 output of a corpus within the bound is unchanged.
	small, smallRev := extractFixture(t, false, nil)
	one, err := BuildExtractorInventory(context.Background(), ExtractorInput{Root: small, Revision: smallRev, Profile: "corvint/extract.json"})
	if err != nil {
		t.Fatal(err)
	}
	explicit, err := BuildExtractorInventory(context.Background(), ExtractorInput{Root: small, Revision: smallRev, Profile: "corvint/extract.json", Schema: "1"})
	if err != nil || string(one) != string(explicit) || !strings.Contains(string(one), `"schema":"`+ProviderSchema+`"`) {
		t.Fatalf("--schema 1 should equal the default provider/1 output: %v", err)
	}
}

// DCX-V1-562: a provider record within the record bound but over its schema's input bound is
// refused at extraction, naming its bytes and the bound, instead of emitting a record build
// refuses; --schema 2 admits it. 200 enqueue sites each cite the job's 64 declaration anchors.
func TestDCXV1562ExtractorRefusesARecordOverTheInputBound(t *testing.T) {
	t.Parallel()
	more := map[string]string{
		"app/jobs/bulk_job.rb": strings.Repeat("class BulkJob\n", 64),
		"app/enqueue.rb":       strings.Repeat("BulkJob.perform_later\n", 200),
	}
	root, rev := extractFixtureFiles(t, false, more, nil)
	_, err := extractSchema(t, root, rev, "")
	tkRefused(t, err, "over the "+ProviderSchema+" input bound of 4194304 bytes; --schema 2 emits an adoption provider record, which admits 67108864 bytes")
	tkRefused(t, err, "extractor provider record is ")
	inv, err := extractSchema(t, root, rev, "2")
	if err != nil {
		t.Fatal(err)
	}
	if record, _ := Encode(inv.Provider); len(record) <= MaxBytes {
		t.Fatalf("the fixture record should exceed the provider/1 input bound: %d bytes", len(record))
	}
	extractBuild(t, root, rev, ManifestSchemaV2, inv)
}
