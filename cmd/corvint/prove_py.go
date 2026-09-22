package main

import (
	"context"
	"path"
	"strings"

	"github.com/Beamfall/corvint/internal/liveverify/mutate"
	"github.com/Beamfall/corvint/internal/liveverify/pymutate"
)

// mutationTestPath reports whether a test path names a test the mutation
// falsifier can run: a Go _test.go file (FPK-V0-014) or a pytest test_*.py
// or *_test.py file (FPK-V0-019).
func mutationTestPath(test string) bool {
	return strings.HasSuffix(test, "_test.go") || pymutate.TestFile(test)
}

// mutationLanguage names the runner a path belongs to, by suffix; a path
// neither runner mutates has no language.
func mutationLanguage(relative string) string {
	switch path.Ext(relative) {
	case ".go":
		return "go"
	case ".py":
		return "python"
	}
	return ""
}

// mutationClaim reports whether test may claim to cover changed under
// test-kills-mutant: changed is source in the test's own language, is not a
// test, and is not the test itself. Mutating a test proves nothing about the
// code, and a runner judges one language.
func mutationClaim(test, changed string) bool {
	if changed == test || mutationTestPath(changed) {
		return false
	}
	language := mutationLanguage(changed)
	return language != "" && language == mutationLanguage(test)
}

// judgePythonMutation is judgeMutation's Python arm (FPK-V0-019): the same
// export, sandbox, budget, and confinement, with pymutate writing the
// mutants and pytest running the cited tests.
func judgePythonMutation(ctx context.Context, exported *mutate.Export, changed, test string, lines []mutate.LineSpan) (mutationVerdict, error) {
	report, err := pymutate.Judge(ctx, exported, pymutate.Request{ChangedPath: changed, TestPath: test, Lines: lines})
	if err != nil {
		return mutationVerdict{}, err
	}
	return mutationVerdict{falsified: mutationFalsified(report.Verdict), detail: report.Detail}, nil
}
