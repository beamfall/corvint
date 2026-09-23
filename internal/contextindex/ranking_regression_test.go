package contextindex

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"strings"
	"testing"
)

func TestEvalQueryCamelSplitsTaskBeforeLowering(t *testing.T) {
	t.Run("GPK-V0-055", func(t *testing.T) {
		t.Setenv("GIT_AUTHOR_DATE", "2000-01-01T00:00:00Z")
		t.Setenv("GIT_COMMITTER_DATE", "2000-01-01T00:00:00Z")
		root := t.TempDir()
		testGit(t, root, "init", "-q")
		testGit(t, root, "config", "user.email", "corvint@example.test")
		testGit(t, root, "config", "user.name", "Corvint Test")
		for path, content := range map[string]string{
			"lib/io/strip-newline.js":          "export const stripNewline = value => value;\n",
			"lib/validate-file-object-mode.js": "export const validateFileObjectMode = value => value;\n",
		} {
			writeTestFile(t, root, path, content)
		}
		testGit(t, root, "add", ".")
		testGit(t, root, "commit", "-qm", "query fixture")

		index, err := Build(context.Background(), root)
		if err != nil {
			t.Fatal(err)
		}
		packet, err := EvalQuery(context.Background(), index,
			"How does stripFinalNewline behave for object-mode output?", 1, nil)
		if err != nil {
			t.Fatal(err)
		}
		results := mapsFromAny(packet["results"])
		if len(results) != 1 || results[0]["id"] != "lib/io/strip-newline.js:stripNewline" {
			t.Fatalf("results = %v, want stripNewline first", results)
		}
	})
}

func TestImpactRanksSamePackageTestsByDeclarationReferences(t *testing.T) {
	t.Run("GPK-V0-056", func(t *testing.T) {
		t.Setenv("GIT_AUTHOR_DATE", "2000-01-01T00:00:00Z")
		t.Setenv("GIT_COMMITTER_DATE", "2000-01-01T00:00:00Z")
		root := t.TempDir()
		testGit(t, root, "init", "-q")
		testGit(t, root, "config", "user.email", "corvint@example.test")
		testGit(t, root, "config", "user.name", "Corvint Test")
		for path, content := range map[string]string{
			"go.mod":                  "module example.test/ranking\n\ngo 1.27.0\n",
			"pkg/subject.go":          "package pkg\n\nfunc ImportantDeclaration() {}\n",
			"pkg/a_unrelated_test.go": "package pkg\n\nfunc TestUnrelated() {}\n",
			"pkg/z_relevant_test.go":  "package pkg\n\nvar _ = ImportantDeclaration\n\nfunc TestRelevant() { ImportantDeclaration() }\n",
		} {
			writeTestFile(t, root, path, content)
		}
		testGit(t, root, "add", ".")
		testGit(t, root, "commit", "-qm", "impact fixture")

		index, err := Build(context.Background(), root)
		if err != nil {
			t.Fatal(err)
		}
		packet, err := Impact(index, []string{"pkg/subject.go"}, 2)
		if err != nil {
			t.Fatal(err)
		}
		results := mapsFromAny(packet["results"])
		if len(results) != 2 || results[1]["kind"] != "test" || results[1]["id"] != "pkg/z_relevant_test.go" {
			t.Fatalf("results = %v, want relevant same-package test second", results)
		}
		if results[1]["score"] != 552 {
			t.Fatalf("score = %v, want 552", results[1]["score"])
		}
		evidence := mapsFromAny(results[1]["evidence"])
		if len(evidence) != 3 || !strings.HasPrefix(stringValue(evidence[1]["reason"]), "references ImportantDeclaration") {
			t.Fatalf("evidence = %v, want convention followed by declaration references", evidence)
		}
	})
	// A name declared twice by the changed file (two `String` methods) is one
	// declaration name: each code line matching it is one pair, not two.
	t.Run("GPK-V0-056 distinct declaration names", func(t *testing.T) {
		root := impactRepositoryWithFiles(t, map[string]string{
			"go.mod":             "module example.test/ranking\n\ngo 1.27.0\n",
			"pkg/subject.go":     "package pkg\n\ntype A struct{}\ntype B struct{}\n\nfunc (A) String() string { return \"a\" }\nfunc (B) String() string { return \"b\" }\n",
			"pkg/z_user_test.go": "package pkg\n\nfunc TestUser() { _ = A{}.String() }\n",
		})
		index, err := Build(context.Background(), root)
		if err != nil {
			t.Fatal(err)
		}
		packet, err := Impact(index, []string{"pkg/subject.go"}, 5)
		if err != nil {
			t.Fatal(err)
		}
		for _, result := range mapsFromAny(packet["results"]) {
			if result["id"] != "pkg/z_user_test.go" {
				continue
			}
			if result["score"] != 551 || len(mapsFromAny(result["evidence"])) != 2 {
				t.Fatalf("result = %v, want score 551 with one reference evidence", result)
			}
			return
		}
		t.Fatalf("results = %v, want pkg/z_user_test.go", packet["results"])
	})
}

// recipeFixtureTask names a path and a backticked identifier so every recipe R
// mechanism (path field, definition term, documentation gate, anchor
// inference) has something to act on in recipeFixtureIndex.
const recipeFixtureTask = "Why does `zqframe` in pkg/parser/parser.go drop the needle signal?"

func recipeFixtureIndex(t *testing.T) *Index {
	t.Helper()
	t.Setenv("GIT_AUTHOR_DATE", "2000-01-01T00:00:00Z")
	t.Setenv("GIT_COMMITTER_DATE", "2000-01-01T00:00:00Z")
	root := t.TempDir()
	testGit(t, root, "init", "-q")
	testGit(t, root, "config", "user.email", "corvint@example.test")
	testGit(t, root, "config", "user.name", "Corvint Test")
	long := strings.Repeat("\t// tokenised filler lines keep this body long for normalisation\n", 80)
	for path, content := range map[string]string{
		"go.mod":                      "module example.test/recipe\n\ngo 1.27.0\n",
		"AGENTS.md":                   "# Corvint agent contract\n\nRead the parser before changing it.\n",
		"pkg/parser/parser.go":        "package parser\n\nfunc Parse(value string) string { return Frobnicate(value) }\n",
		"pkg/parser/parser_test.go":   "package parser\n\nfunc TestParse() { Parse(\"needle\") }\n",
		"pkg/parser/helpers.go":       "package parser\n\nfunc trimSpace(value string) string { return value }\n",
		"lib/frob/frob.go":            "package frob\n\nfunc Frobnicate(value string) string { return value }\n",
		"lib/scan/def.go":             "package scan\n\nfunc zqframe(value string) string {\n" + long + "\treturn value\n}\n",
		"cmd/needle/main.go":          "package main\n\nfunc main() { run(\"needle\") }\n",
		"internal/other/short.go":     "package other\n\n// needle needle signal\n",
		"internal/other/unrelated.go": "package other\n\nfunc unrelated() {}\n",
		"docs/needle.md":              "# needle\n\nThe needle signal drop.\n",
		"docs/guide.md":               "# guide\n\nsignal handling\n",
	} {
		writeTestFile(t, root, path, content)
	}
	testGit(t, root, "add", ".")
	testGit(t, root, "commit", "-qm", "recipe fixture")
	index, err := Build(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
	return index
}

func recipePacket(t *testing.T, index *Index, mode, task string) map[string]any {
	t.Helper()
	t.Setenv("CORVINT_CONTEXT_RECIPE", mode)
	packet, err := TaskContext(context.Background(), index, task, "", 20)
	if err != nil {
		t.Fatal(err)
	}
	return packet
}

// TestContextRecipeDefaultPathIsByteIdentical pins the default path against a
// packet captured with the recipe unset (lane byte-identity rule). The golden
// was first captured at 2a76e40, before recipe R existed, and re-captured on
// the staged tree after TCP-V0-015 and TCP-V0-016 landed; that re-capture
// added only the `test` relation and the answerability member. The TCP-V0-016(b)
// support-disclosure repair later corrected required=0 to 1 and its reason for
// this one-known-term fixture; every other packet field stayed byte-identical.
// TCP-V0-011 subsequently corrects only pair examination/count for its named
// path: subject-absent/null becomes examined/0; all other bytes are retained.
// Decision 0146 then adds only the supported verdict's nearest_claims block.
// Decision 0346 (TCP-V0-023) then adds only the `trust` member on each evidence
// row and the empty `coverage.governance_refused` array.
func TestContextRecipeDefaultPathIsByteIdentical(t *testing.T) {
	t.Run("TCP-V0-018", func(t *testing.T) {
		packet := recipePacket(t, recipeFixtureIndex(t), "", recipeFixtureTask)
		encoded, err := json.MarshalIndent(packet, "", "  ")
		if err != nil {
			t.Fatal(err)
		}
		golden, err := os.ReadFile("testdata/context-recipe-default-golden.json")
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(append(encoded, '\n'), golden) {
			t.Fatalf("default packet drifted from the golden:\n%s", encoded)
		}
	})
}

// TestContextRecipeRetiredFlagsAreIgnored keeps retired experiment settings
// from selecting an unpromoted order (decision 0078).
func TestContextRecipeRetiredFlagsAreIgnored(t *testing.T) {
	t.Run("TCP-V0-018", func(t *testing.T) {
		index := recipeFixtureIndex(t)
		before, err := json.Marshal(recipePacket(t, index, "", recipeFixtureTask))
		if err != nil {
			t.Fatal(err)
		}
		for _, mode := range []string{"r", "r+anchor", "r+doctail", "r+anchor+doctail", "r+doctail+anchor"} {
			t.Run(mode, func(t *testing.T) {
				after, err := json.Marshal(recipePacket(t, index, mode, recipeFixtureTask))
				if err != nil {
					t.Fatal(err)
				}
				if !bytes.Equal(before, after) {
					t.Fatalf("retired recipe %q changed the default packet", mode)
				}
			})
		}
	})
}
