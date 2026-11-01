package doccorpus

import (
	"context"
	json "encoding/json/v2"
	"fmt"
	"strings"
	"testing"
)

// TestDCXV1812MatchedFilesBoundNamesScopeAndSchemaTwo pins DCX-V1-812 (V1-2561): a caller-style
// app/**/*.rb profile over more than 4,096 matching Ruby files, as in a large Rails repository,
// refuses under the default provider/1 bound with a message naming the matched-file count, the
// bound, the extractor, --schema 2 and how to scope the profile; --schema 2 extracts every file.
func TestDCXV1812MatchedFilesBoundNamesScopeAndSchemaTwo(t *testing.T) {
	t.Parallel()
	const files = 4200
	profile := map[string]any{"schema": ExtractorProfileSchema, "provider": "app", "version": "1", "extractors": []any{
		map[string]any{"id": "defs", "subject": "symbol", "files": []string{"app/**/*.rb"}, "pattern": `^def (?P<name>\w+)`},
	}}
	data, err := json.Marshal(profile, json.Deterministic(true))
	if err != nil {
		t.Fatal(err)
	}
	tree := map[string]string{"corvint/extract.json": string(data)}
	for i := 0; i < files; i++ {
		tree[fmt.Sprintf("app/models/d%02d/m%04d.rb", i%50, i)] = fmt.Sprintf("def model%d\nend\n", i)
	}
	root := t.TempDir()
	rev := writeBulkFixture(t, root, tree)
	for _, schema := range []string{"", "1"} {
		_, err := extractSchema(t, root, rev, schema)
		tkRefused(t, err, `extractor inventory exceeds the record bound: 4200 matched files at extractor "defs", over the bound of 4096`)
		tkRefused(t, err, "--schema 2 emits an adoption provider record for a manifest /2, which admits 100000")
		tkRefused(t, err, `or narrow extractor "defs" files globs to at most 4096 files`)
	}
	inv, err := BuildExtractorInventory(context.Background(), ExtractorInput{Root: root, Revision: rev, Profile: "corvint/extract.json", Schema: "2"})
	if err != nil {
		t.Fatalf("--schema 2 over %d matched files: %v", files, err)
	}
	if !strings.Contains(string(inv), `"model4199"`) {
		t.Fatal("--schema 2 did not extract the last matched file")
	}
}
