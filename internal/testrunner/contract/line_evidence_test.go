// SPDX-License-Identifier: Apache-2.0
package contract

import (
	"fmt"
	"slices"
	"strings"
	"testing"
)

func TestLEVV1014GoNamesWidenToPackages(t *testing.T) {
	tests := make([]string, 5000)
	for i := range tests {
		tests[i] = fmt.Sprintf("go:pkg::Test%05d%s", i, strings.Repeat("x", 30))
	}
	request := Request{Instance: Instance{Runner: "go", Root: "."}, Full: []string{"go", "test", "./..."}, Tests: tests, Tier: TierLine}
	got, err := (goRunner{}).Command(request)
	if err != nil {
		t.Fatal(err)
	}
	if got.Tier != TierFile || got.Exact || got.Reason != ReasonArgvLimit || slices.Contains(got.Argv, "-run") || !slices.Contains(got.Argv, "./pkg") || !ArgvFits(got.Argv) {
		t.Fatalf("%+v", got)
	}
}

func TestLEVV1014FileOverflowWidens(t *testing.T) {
	for _, name := range []string{"vitest", "pytest", "rspec", "playwright"} {
		t.Run(name, func(t *testing.T) {
			tests := make([]string, 2000)
			for i := range tests {
				tests[i] = fmt.Sprintf("%s:tests/%04d%s.test.js", name, i, strings.Repeat("x", 150))
			}
			runner, _ := For(name)
			got, err := runner.Command(Request{Instance: Instance{Runner: name, Root: "."}, Full: runner.Default(Instance{Runner: name, Root: "."}, func(string) bool { return false }), Tests: tests, Report: "report.json"})
			if err != nil {
				t.Fatal(err)
			}
			if got.Tier != TierFull || got.Exact || got.Reason != ReasonArgvLimit || !ArgvFits(got.Argv) {
				t.Fatalf("%+v", got)
			}
		})
	}
}
