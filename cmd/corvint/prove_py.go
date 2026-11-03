package main

import (
	"context"
	"strings"

	"github.com/Beamfall/corvint/internal/langplugin"
	"github.com/Beamfall/corvint/internal/liveverify/mutate"
	"github.com/Beamfall/corvint/internal/liveverify/pymutate"
	"github.com/Beamfall/corvint/internal/liveverify/rbmutate"
)

// mutationTestPath reports whether a test path names a test the mutation
// falsifier can run: a Go _test.go file (FPK-V0-014), a pytest test_*.py or
// *_test.py file (FPK-V0-019), or an RSpec *_spec.rb file (FPK-V0-068).
func mutationTestPath(test string) bool {
	return strings.HasSuffix(test, "_test.go") || pymutate.TestFile(test) || rbmutate.TestFile(test)
}

// mutationLanguage names the plugin whose mutation engine mutates a path, by
// suffix (LGP-V0-018); a path no engine mutates has no language.
func mutationLanguage(relative string) string {
	return langplugin.MutationLanguage(relative)
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
