// SPDX-License-Identifier: Apache-2.0
package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"
)

// treesRecord is the read-scope trees record that declares this guard's
// content predicates (AFP-V0-3300).
const treesRecord = ".corvint/test-read-scope-trees.json"

// licenceHeaders are lines every licensed source carries; a predicate that
// matches one alone selects the guard on every edit (V1-5364).
var licenceHeaders = []string{
	"// SPDX-License-Identifier: Apache-2.0",
	"# SPDX-License-Identifier: Apache-2.0",
	// Split so the license-headers check does not read the input as a header.
	"// SPDX-License-Identifier: A" + "GPL-3.0-or-later",
}

// refuseCalls are the call shapes unclassifiedCounts counts.
var refuseCalls = []string{`return refuse("work-not-found")`, `c.RefusePending()`, "refuseWith (\n\tcode)"}

// guardPredicates returns the content predicate the record declares for each
// of this guard's trees.
func guardPredicates(t *testing.T, root string) map[string]string {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(treesRecord)))
	if err != nil {
		t.Fatal(err)
	}
	var record struct {
		Trees map[string]map[string]json.RawMessage `json:"trees"`
	}
	if err := json.Unmarshal(raw, &record); err != nil {
		t.Fatalf("%s: %v", treesRecord, err)
	}
	predicates := map[string]string{}
	for tree, value := range record.Trees["tools/flow-guide-check"] {
		var object struct {
			ContentPredicate string `json:"contentPredicate"`
		}
		if json.Unmarshal(value, &object) == nil && object.ContentPredicate != "" {
			predicates[tree] = object.ContentPredicate
		}
	}
	return predicates
}

// predicateGaps names, per tree, each guide code its predicate does not
// match, a refuse call it does not match, and a licence header line it
// matches alone (AFP-V0-9981).
func predicateGaps(predicates map[string]string, codes []string) []string {
	var gaps []string
	for tree, source := range predicates {
		predicate, err := regexp.Compile(source)
		if err != nil {
			gaps = append(gaps, fmt.Sprintf("%s: predicate does not compile: %v", tree, err))
			continue
		}
		for _, code := range codes {
			if !predicate.MatchString(code) {
				gaps = append(gaps, fmt.Sprintf("%s: predicate misses guide code %s", tree, code))
			}
		}
		for _, call := range refuseCalls {
			if !predicate.MatchString(call) {
				gaps = append(gaps, fmt.Sprintf("%s: predicate misses refuse call %q", tree, call))
			}
		}
		for _, header := range licenceHeaders {
			if predicate.MatchString(header) {
				gaps = append(gaps, fmt.Sprintf("%s: predicate matches licence header %q", tree, header))
			}
		}
	}
	sort.Strings(gaps)
	return gaps
}

// AFP-V0-9981 (V1-5364): every refusal code the guide's recovery table maps,
// which checkGuide looks up in the sources under cmd/, internal/ and tools/,
// is matched by each declared predicate of those trees, as are refuse calls;
// no predicate matches a licence header alone. A guide edit selects this
// package through its plain docs/LANE-ORCHESTRATION.md entry, so a new code
// fails here before the predicate can fall behind it.
func TestGuidePredicateCoversGuardCodes_AFPV09981(t *testing.T) {
	root := filepath.Join("..", "..")
	repo, err := load(root)
	if err != nil {
		t.Fatal(err)
	}
	predicates := guardPredicates(t, root)
	for _, tree := range []string{"cmd/", "internal/", "tools/"} {
		if predicates[tree] == "" {
			t.Errorf("%s declares no content predicate for %s", treesRecord, tree)
		}
	}
	if len(repo.guideCodes) == 0 {
		t.Fatal("the guide's recovery table maps no code")
	}
	for _, gap := range predicateGaps(predicates, repo.guideCodes) {
		t.Error(gap)
	}
	// The check fires: a predicate missing a code, a word-class predicate
	// (the one V1-5364 replaced) and a predicate without the call shape.
	fixtures := map[string]string{
		"missing code": `[rR]efuse\w*\s*\(|whole-reason-missing`,
		"word class":   `[rR]efuse|[a-z][a-z0-9]*(?:-[a-z0-9]+)+|[A-Z][A-Z0-9]*(?:_[A-Z0-9]+)+`,
		"no refuse":    `whole-reason-missing|OUT_OF_SCOPE`,
	}
	want := map[string]string{
		"missing code": "internal/: predicate misses guide code OUT_OF_SCOPE",
		"word class":   "internal/: predicate matches licence header \"// SPDX-License-Identifier: Apache-2.0\"",
		"no refuse":    "internal/: predicate misses refuse call",
	}
	for name, predicate := range fixtures {
		gaps := strings.Join(predicateGaps(map[string]string{"internal/": predicate}, []string{"whole-reason-missing", "OUT_OF_SCOPE"}), "\n")
		if !strings.Contains(gaps, want[name]) {
			t.Errorf("%s: gaps %q lack %q", name, gaps, want[name])
		}
	}
}

// AFP-V0-9980 (V1-5364): the declared predicates do not match a licensed
// source whose only change is a comment, and match a source that adds or
// keeps each code the guard looks up or a refuse call it counts.
func TestGuidePredicateMatchesOnlyGuardReads_AFPV09980(t *testing.T) {
	root := filepath.Join("..", "..")
	repo, err := load(root)
	if err != nil {
		t.Fatal(err)
	}
	for tree, source := range guardPredicates(t, root) {
		predicate := regexp.MustCompile(source)
		for _, header := range licenceHeaders {
			file := header + "\n\npackage example\n\n// A comment-only edit with hyphen-words and UPPER_CASE words.\nfunc F() error { return nil }\n"
			if predicate.MatchString(file) {
				t.Errorf("%s: predicate matches a comment-only licensed file: %q", tree, predicate.FindString(file))
			}
		}
		for _, code := range repo.guideCodes {
			if file := "package example\n\nconst code = \"" + code + "\"\n"; !predicate.MatchString(file) {
				t.Errorf("%s: predicate misses a file holding guide code %s", tree, code)
			}
		}
		for _, call := range refuseCalls {
			if file := "package example\n\nfunc f() {\n\t" + call + "\n}\n"; !predicate.MatchString(file) {
				t.Errorf("%s: predicate misses a file holding %q", tree, call)
			}
		}
	}
}
