// SPDX-License-Identifier: Apache-2.0

package linecov

import "testing"

// MCJ-V0-007, MCJ-V0-040: a runner or build-tag declaration wins, then the
// longest declared path rule, then the family's runner and path conventions,
// else unknown; an invalid declaration refuses.
func TestMCJV0040ResolveLevel(t *testing.T) {
	rules := []LevelRule{{Prefix: "spec", Level: LevelUnit}, {Prefix: "spec/features/", Level: LevelIntegration}}
	for _, c := range []struct {
		name string
		in   LevelInput
		lvl  string
		src  LevelSource
	}{
		{"declared beats rules and runner", LevelInput{Declared: LevelUnit, Rules: rules, Family: "playwright", Path: "spec/features/a_spec.rb"}, LevelUnit, LevelDeclared},
		{"longest path rule", LevelInput{Rules: rules, Family: "rspec", RunnerType: "feature", Path: "./spec/features/a_spec.rb"}, LevelIntegration, LevelPathRule},
		{"shorter path rule", LevelInput{Rules: rules, Family: "rspec", Path: "spec/models/a_spec.rb"}, LevelUnit, LevelPathRule},
		{"rule is a directory, not a name prefix", LevelInput{Rules: rules, Family: "rspec", Path: "specs/a_spec.rb"}, LevelUnknown, LevelDefault},
		{"browser e2e family", LevelInput{Family: "playwright", Path: "tests/a.spec.ts"}, LevelE2E, LevelRunner},
		{"rspec feature type", LevelInput{Family: "rspec", RunnerType: "feature", Path: "spec/a_spec.rb"}, LevelE2E, LevelRunner},
		{"rspec request type", LevelInput{Family: "rspec", RunnerType: "request", Path: "spec/a_spec.rb"}, LevelIntegration, LevelRunner},
		{"rspec model type says nothing", LevelInput{Family: "rspec", RunnerType: "model", Path: "spec/system/a_spec.rb"}, LevelUnknown, LevelDefault},
		{"rspec system dir in a subproject", LevelInput{Family: "rspec", Path: "web/spec/system/a_spec.rb"}, LevelE2E, LevelPath},
		{"go has no convention", LevelInput{Family: "go", Path: "e2e/a_test.go"}, LevelUnknown, LevelDefault},
	} {
		lvl, src, err := ResolveLevel(c.in)
		if err != nil || lvl != c.lvl || src != c.src {
			t.Errorf("%s: %q %q %v, want %q %q", c.name, lvl, src, err, c.lvl, c.src)
		}
	}
	if _, _, err := ResolveLevel(LevelInput{Declared: "E2E"}); err == nil {
		t.Error("an invalid declared level resolved")
	}
	if _, _, err := ResolveLevel(LevelInput{Rules: []LevelRule{{Prefix: "x", Level: "smoke"}}, Path: "y"}); err == nil {
		t.Error("an invalid path rule resolved")
	}
}
