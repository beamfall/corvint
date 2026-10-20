package main

import "github.com/Beamfall/corvint/internal/liveverify/affected"

const (
	// unsupportedWitnessStatus and unsupportedWitnessReason are the witness a
	// claim no mutation engine can judge carries under --mutate (FPK-V0-057).
	unsupportedWitnessStatus = "UNSUPPORTED"
	unsupportedWitnessReason = "mutation-language-unsupported"
)

// discloseUnsupportedMutations marks, under --mutate, every affected-test
// claim no mutation engine can judge (FPK-V0-057): the verdict stays NOT_RUN
// and the falsifier none, and the row carries an UNSUPPORTED witness and a
// detail naming the language, so "not attempted" is never read as "no
// mutants". The selected tests are the affected-test rows' own paths, which
// affectedTestRows already deduplicated. It returns rows, changed in place.
func discloseUnsupportedMutations(rows []proveRow, mutateRequested bool) []proveRow {
	if !mutateRequested {
		return rows
	}
	tests := map[string]struct{}{}
	for _, row := range rows {
		if row.Kind == kindAffectedTest {
			tests[row.Path] = struct{}{}
		}
	}
	providers := affectedLanguages()
	for index := range rows {
		changed, _ := testClaimTarget(rows[index].reason)
		if language := unsupportedMutationLanguage(rows[index], changed, tests, providers); language != "" {
			rows[index].Detail = "no mutation engine for " + language + " sources"
			rows[index].Witness = &proveMutationWitness{Status: unsupportedWitnessStatus, Reason: unsupportedWitnessReason}
		}
	}
	return rows
}

// unsupportedMutationLanguage names the language of an affected-test claim
// that no mutation engine can judge (FPK-V0-057): a row the claim gate left
// with falsifier none whose test and changed path are distinct, whose changed
// path is not itself a selected test, and whose two paths the same
// affected-selection provider owns, that provider being neither Go nor
// Python. Every other row gets "", so a Go or Python row, and a claim across
// languages, keeps its earlier output. An RSpec claim never reaches here with
// falsifier none: the claim gate gives it the Ruby engine (FPK-V0-068), so
// only a minitest claim is marked ruby.
func unsupportedMutationLanguage(row proveRow, changed string, tests map[string]struct{}, providers []affected.Language) string {
	if row.Kind != kindAffectedTest || row.Falsifier != falsifierNone || changed == "" || changed == row.Path {
		return ""
	}
	if _, isTest := tests[changed]; isTest {
		return ""
	}
	for _, provider := range providers {
		if !provider.Owns(changed) || !provider.Owns(row.Path) {
			continue
		}
		if name := provider.Name(); name != "go" && name != "python" {
			return name
		}
		return ""
	}
	return ""
}
