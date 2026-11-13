package contextindex

import (
	"context"
	"strings"
	"testing"
)

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
// inference) has something to act on.
const recipeFixtureTask = "Why does `zqframe` in pkg/parser/parser.go drop the needle signal?"
