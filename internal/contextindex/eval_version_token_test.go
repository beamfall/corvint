package contextindex

import (
	"context"
	"testing"
)

// evalVersionTokenRepository is a Git repository whose only symbols are two
// copies of one function, at an unversioned path and, when versioned is set,
// at a `v2` path as well.
func evalVersionTokenRepository(t *testing.T, versioned bool) string {
	t.Helper()
	root := t.TempDir()
	testGit(t, root, "init", "-q")
	testGit(t, root, "config", "user.email", "corvint@example.test")
	testGit(t, root, "config", "user.name", "Corvint Test")
	writeTestFile(t, root, "go.mod", "module example.test/versions\n\ngo 1.27.0\n")
	writeTestFile(t, root, "internal/auth/session.go", "package auth\n\nfunc EnforceSessionRevocation() bool { return true }\n")
	if versioned {
		writeTestFile(t, root, "internal/v2/session.go", "package v2\n\nfunc EnforceSessionRevocation() bool { return true }\n")
	}
	testGit(t, root, "add", ".")
	testGit(t, root, "commit", "-qm", "session revocation")
	return root
}

func evalVersionTokenPaths(t *testing.T, root, text string) []string {
	t.Helper()
	index, err := Build(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
	queryText := pythonLower(text)
	intent := evalInferIntent(queryText)
	queryTerms := evalRelevanceTerms(terms(queryText), intent)
	paths := make([]string, 0)
	ranked, _ := evalRankSymbols(index, queryText, queryTerms, intent)
	for _, candidate := range ranked {
		paths = append(paths, candidate.symbol.symbol.Path)
	}
	return paths
}

// TestEvalRankSymbolsVersionTokenNarrowsOnlyVersionedPaths pins decision 0017:
// a `vN` token in the task narrows the symbol universe only when some path in
// it carries a version term, and is ignored where none does.
func TestEvalRankSymbolsVersionTokenNarrowsOnlyVersionedPaths(t *testing.T) {
	const text = "enforce session revocation v2"
	unversioned := evalVersionTokenPaths(t, evalVersionTokenRepository(t, false), text)
	if len(unversioned) != 1 || unversioned[0] != "internal/auth/session.go" {
		t.Fatalf("no versioned path: ranked %v, want the unversioned symbol alone", unversioned)
	}
	versioned := evalVersionTokenPaths(t, evalVersionTokenRepository(t, true), text)
	if len(versioned) != 1 || versioned[0] != "internal/v2/session.go" {
		t.Fatalf("versioned path present: ranked %v, want the v2 symbol alone", versioned)
	}
}
