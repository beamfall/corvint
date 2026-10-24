package doccorpus

import (
	"context"
	json "encoding/json/v2"
	"fmt"
	"path"
	"strings"
	"testing"
)

// manyDirectoryFixture commits one Ruby file in each of dirs directories under app/models, the
// same number of unmatched directories under vendor, and an extractor profile over app/**/*.rb.
func manyDirectoryFixture(t *testing.T, dirs int) (string, string) {
	t.Helper()
	profile := map[string]any{"schema": ExtractorProfileSchema, "provider": "app", "version": "1", "extractors": []any{
		map[string]any{"id": "defs", "subject": "symbol", "files": []string{"app/**/*.rb"}, "pattern": `^def (?P<name>\w+)`},
	}}
	data, err := json.Marshal(profile, json.Deterministic(true))
	if err != nil {
		t.Fatal(err)
	}
	files := map[string]string{"corvint/extract.json": string(data)}
	for i := 0; i < dirs; i++ {
		files[fmt.Sprintf("app/models/m%04d/model.rb", i)] = fmt.Sprintf("def model%d\nend\n", i)
		files[fmt.Sprintf("vendor/v%04d/lib.rb", i)] = fmt.Sprintf("def vendored%d\nend\n", i)
	}
	root := t.TempDir()
	return root, writeBulkFixture(t, root, files)
}

// TestDCXV1520ExtractionEnumeratesMoreThan1024Directories pins DCX-V1-520 and DCX-V1-521
// (GitHub #1662): a revision with more directories than the default 1,024-operation Git budget
// extracts under that default budget and under the small phase bound, and the vendor
// directories no glob reaches contribute nothing.
func TestDCXV1520ExtractionEnumeratesMoreThan1024Directories(t *testing.T) {
	t.Parallel()
	const dirs = 1100
	root, rev := manyDirectoryFixture(t, dirs)
	for name, ctx := range map[string]context.Context{
		"default budget": context.Background(),
		"phase bound":    boundCorpusGitOps(context.Background(), phaseGitOps),
	} {
		data, err := BuildExtractorInventory(ctx, ExtractorInput{Root: root, Revision: rev, Profile: "corvint/extract.json"})
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		var inv ExtractorInventory
		if err := json.Unmarshal(data, &inv, json.RejectUnknownMembers(true)); err != nil {
			t.Fatal(err)
		}
		if len(inv.Provider.Subjects) != dirs || len(inv.Gaps) != 0 {
			t.Fatalf("%s: subjects=%d gaps=%d, want %d subjects", name, len(inv.Provider.Subjects), len(inv.Gaps), dirs)
		}
		for _, subject := range inv.Provider.Subjects {
			if strings.Contains(subject.ID, "vendored") {
				t.Fatalf("%s: unmatched directory extracted: %s", name, subject.ID)
			}
		}
	}
}

// TestDCXV1521ExtractorGlobPruningIsSound pins DCX-V1-521: every directory above a path a glob
// matches is descended, and directories no glob can reach are pruned.
func TestDCXV1521ExtractorGlobPruningIsSound(t *testing.T) {
	t.Parallel()
	globs := []string{"app/**/*.rb", "app/*.rb", "lib/**", "*/models/*.rb", "a/b/c.rb", "**/*.ts", "[ab]/x/*.go"}
	names := []string{"app/a.rb", "app/x/y/z.rb", "lib/q", "lib/q/r/s.txt", "app/models/m.rb", "pkg/models/m.rb",
		"a/b/c.rb", "a/b/d.rb", "x/y/z.ts", "z.ts", "a/x/y.go", "b/x/y.go", "c/x/y.go", "vendor/v/lib.rb"}
	descends := func(glob, dir string) bool {
		return extractorGlobBelow(strings.Split(glob, "/"), strings.Split(dir, "/"))
	}
	for _, glob := range globs {
		for _, name := range names {
			if !extractorMatch(glob, name) {
				continue
			}
			for dir := path.Dir(name); dir != "."; dir = path.Dir(dir) {
				if !descends(glob, dir) {
					t.Errorf("glob %q matches %q but prunes %q", glob, name, dir)
				}
			}
		}
	}
	for _, c := range []struct {
		glob, dir string
		want      bool
	}{
		{"app/**/*.rb", "app", true}, {"app/**/*.rb", "app/x/y", true}, {"app/**/*.rb", "vendor", false},
		{"app/*.rb", "app", true}, {"app/*.rb", "app/x", false}, {"*/models/*.rb", "pkg/models", true},
		{"*/models/*.rb", "pkg/views", false}, {"*/models/*.rb", "pkg/models/deeper", false},
		{"a/b/c.rb", "a/b", true}, {"a/b/c.rb", "a/c", false}, {"[ab]/x/*.go", "c", false},
		{"lib/**", "lib/q/r", true}, {"**/*.ts", "any/depth", true},
	} {
		if got := descends(c.glob, c.dir); got != c.want {
			t.Errorf("descend(%q, %q) = %t, want %t", c.glob, c.dir, got, c.want)
		}
	}
}
