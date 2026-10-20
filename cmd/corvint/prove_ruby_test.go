package main

import (
	"strings"
	"testing"
)

// proveRubyChangeRepository commits the Rails-shaped RSpec fixture, then a
// second commit that changes app/api_clients/foo_client.rb, so `prove --base`
// judges a committed Ruby range.
func proveRubyChangeRepository(t *testing.T) (root, base string) {
	t.Helper()
	root = rspecAdviceRepository(t, true)
	affectedGit(t, root, "stash", "-q")
	base = strings.TrimSpace(affectedGit(t, root, "rev-parse", "HEAD"))
	affectedGit(t, root, "stash", "pop", "-q")
	affectedGit(t, root, "add", ".")
	affectedGit(t, root, "commit", "-qm", "change FooClient")
	return root, base
}

// TestFPKV0057RubyRowsCarryUnsupportedMutationWitness pins FPK-V0-057: under
// --mutate an RSpec affected-test row claiming a changed .rb source carries an
// UNSUPPORTED witness with the stable reason and a detail naming the language,
// keeps falsifier none and NOT_RUN, and without --mutate carries neither.
func TestFPKV0057RubyRowsCarryUnsupportedMutationWitness(t *testing.T) {
	t.Parallel()
	root, base := proveRubyChangeRepository(t)
	before := treeDigest(t, root)
	rubyRow := func(receipt map[string]any) map[string]any {
		t.Helper()
		for _, row := range proofRows(t, receipt) {
			if stringAt(row, "kind") == kindAffectedTest && stringAt(row, "path") == "spec/api_clients/foo_client_spec.rb" {
				return row
			}
		}
		t.Fatalf("no RSpec affected-test row: %v", receipt["proof"])
		return nil
	}
	receipt, _, stderr, code := runProveCLI(t, root, "--base", base)
	if code != 0 {
		t.Fatalf("exit %d: %s", code, stderr)
	}
	row := rubyRow(receipt)
	if row["falsifier"] != falsifierNone || row["falsified"] != falsifiedNotRun || row["witness"] != nil || row["detail"] != nil {
		t.Fatalf("without --mutate: %v", row)
	}
	receipt, _, stderr, code = runProveCLI(t, root, "--base", base, "--mutate")
	if code != 0 {
		t.Fatalf("--mutate exit %d: %s", code, stderr)
	}
	row = rubyRow(receipt)
	witness, _ := row["witness"].(map[string]any)
	if row["falsifier"] != falsifierNone || row["falsified"] != falsifiedNotRun || row["detail"] != "no mutation engine for ruby sources" {
		t.Fatalf("--mutate row: %v", row)
	}
	if len(witness) != 2 || witness["status"] != "UNSUPPORTED" || witness["reason"] != "mutation-language-unsupported" {
		t.Fatalf("--mutate witness: %v", witness)
	}
	if after := treeDigest(t, root); after != before {
		t.Fatal("prove --mutate wrote the repository")
	}
}

// TestFPKV0057UnsupportedLanguageLeavesEngineAndCrossLanguageRowsAlone pins
// FPK-V0-057's boundary: Go and Python claims, a changed path that is itself a
// selected test, a claim across languages, a row that already has a falsifier,
// and a packet row are never marked; a same-language claim without an engine
// names its provider.
func TestFPKV0057UnsupportedLanguageLeavesEngineAndCrossLanguageRowsAlone(t *testing.T) {
	t.Parallel()
	providers := affectedLanguages()
	tests := map[string]struct{}{"spec/a_spec.rb": {}, "spec/b_spec.rb": {}}
	cases := []struct {
		test, changed, want string
	}{
		{"spec/a_spec.rb", "lib/a.rb", "ruby"},
		{"src/a.test.ts", "src/a.ts", "typescript"},
		{"spec/a_spec.rb", "spec/b_spec.rb", ""},
		{"spec/a_spec.rb", "spec/a_spec.rb", ""},
		{"spec/a_spec.rb", "config/a.yml", ""},
		{"pkg/calc/calc_test.go", "pkg/calc/calc_test.go", ""},
		{"pkg/calc/calc_test.go", "pkg/calc/testdata/fixture.go", ""},
		{"tests/helpers.py", "calc.py", ""},
	}
	for _, item := range cases {
		row := affectedRow(item.test, item.changed)
		if got := unsupportedMutationLanguage(row, item.changed, tests, providers); got != item.want {
			t.Errorf("%s claiming %s: got %q, want %q", item.test, item.changed, got, item.want)
		}
	}
	mutant := affectedRow("pkg/calc/calc_test.go", "pkg/calc/calc.go")
	if got := unsupportedMutationLanguage(mutant, "pkg/calc/calc.go", tests, providers); got != "" {
		t.Errorf("a Go mutation row is marked %q", got)
	}
	packetRow := proveRow{Kind: "test", Falsifier: falsifierNone, Path: "spec/a_spec.rb"}
	if got := unsupportedMutationLanguage(packetRow, "lib/a.rb", tests, providers); got != "" {
		t.Errorf("a packet row is marked %q", got)
	}
}
