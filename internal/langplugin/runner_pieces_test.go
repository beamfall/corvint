// SPDX-License-Identifier: Apache-2.0

package langplugin_test

import (
	"reflect"
	"testing"

	"github.com/Beamfall/corvint/internal/langplugin"
	tr "github.com/Beamfall/corvint/internal/testrunner"
	"github.com/Beamfall/corvint/internal/testrunner/registry"
)

func runnerManifest() langplugin.Manifest {
	return langplugin.Manifest{
		Name: "fixture", Paths: []string{"internal/fixture"}, Runners: []string{"fixture-runner"},
		Absent:       map[langplugin.Piece]string{langplugin.Provider: "none", langplugin.Analyzer: "none", langplugin.TestProvider: "none", langplugin.Coverage: "none", langplugin.Mutation: "none"},
		RunnerPieces: map[string]tr.RunnerPieces{"fixture-runner": {Inventory: tr.RunnerPiece{Present: true}, TestEvidence: tr.RunnerPiece{Present: true}, Qualification: tr.RunnerPiece{Present: true}}},
	}
}

// RSP-V0-001: every piece is present or individually absent, per runner.
func TestRSPV0001PresentAndAbsent(t *testing.T) {
	m := runnerManifest()
	if err := langplugin.Validate(m); err != nil {
		t.Fatal(err)
	}
	if reason := m.RunnerPieces["fixture-runner"].FullReason(true); reason != "" {
		t.Fatal(reason)
	}
	if reason := m.RunnerPieces["fixture-runner"].FullReason(false); reason != "runner-not-qualified" {
		t.Fatal(reason)
	}
	for _, name := range []string{"inventory", "test-evidence", "qualification"} {
		p := m.RunnerPieces["fixture-runner"]
		switch name {
		case "inventory":
			p.Inventory = tr.RunnerPiece{Reason: "not yet available"}
		case "test-evidence":
			p.TestEvidence = tr.RunnerPiece{Reason: "not yet available"}
		case "qualification":
			p.Qualification = tr.RunnerPiece{Reason: "not yet available"}
		}
		m.RunnerPieces["fixture-runner"] = p
		if err := langplugin.Validate(m); err != nil {
			t.Fatal(err)
		}
		if p.FullReason(true) == "" {
			t.Fatal("absent piece narrowed")
		}
		m = runnerManifest()
	}
	for _, piece := range []langplugin.Piece{langplugin.Inventory, langplugin.TestEvidence, langplugin.Qualification} {
		found := false
		for _, p := range langplugin.Pieces {
			found = found || p == piece
		}
		if !found {
			t.Fatalf("piece %s missing from closed set", piece)
		}
	}
}

// RSP-V0-001: missing, contradictory and language-level declarations refuse.
func TestRSPV0001RefusesInvalidPieces(t *testing.T) {
	for name, mutate := range map[string]func(*langplugin.Manifest){
		"missing-runner": func(m *langplugin.Manifest) { m.RunnerPieces = nil },
		"missing-inventory": func(m *langplugin.Manifest) {
			p := m.RunnerPieces["fixture-runner"]
			p.Inventory = tr.RunnerPiece{}
			m.RunnerPieces["fixture-runner"] = p
		},
		"blank-evidence": func(m *langplugin.Manifest) {
			p := m.RunnerPieces["fixture-runner"]
			p.TestEvidence = tr.RunnerPiece{Reason: " \t"}
			m.RunnerPieces["fixture-runner"] = p
		},
		"missing-qualification": func(m *langplugin.Manifest) {
			p := m.RunnerPieces["fixture-runner"]
			p.Qualification = tr.RunnerPiece{}
			m.RunnerPieces["fixture-runner"] = p
		},
		"contradiction": func(m *langplugin.Manifest) {
			p := m.RunnerPieces["fixture-runner"]
			p.Inventory.Reason = "also absent"
			m.RunnerPieces["fixture-runner"] = p
		},
		"unknown-runner": func(m *langplugin.Manifest) { m.RunnerPieces["other"] = tr.AbsentRunnerPieces("other") },
		"language-level": func(m *langplugin.Manifest) { m.Absent[langplugin.Inventory] = "none" },
	} {
		t.Run(name, func(t *testing.T) {
			m := runnerManifest()
			mutate(&m)
			if err := langplugin.Validate(m); err == nil {
				t.Fatal("invalid declaration accepted")
			}
		})
	}
}

// RSP-V0-001: all existing runners keep their legacy path and explicit absence.
func TestRSPV0001EveryExistingRunnerDeclared(t *testing.T) {
	seen := map[string]bool{}
	for _, m := range langplugin.All() {
		if err := langplugin.Validate(m); err != nil {
			t.Fatal(err)
		}
		for _, runner := range m.Runners {
			seen[runner] = true
			p, ok := m.RunnerPieces[runner]
			if !ok {
				t.Fatalf("%s missing pieces", runner)
			}
			want := tr.AbsentRunnerPieces(runner)
			switch runner {
			case "rspec", "minitest", "test-unit", "rails-test":
				want = tr.RunnerPieces{Inventory: tr.RunnerPiece{Present: true}, TestEvidence: tr.RunnerPiece{Present: true}, Qualification: tr.RunnerPiece{Reason: "unqualified"}}
			}
			if !reflect.DeepEqual(p, want) {
				t.Fatalf("%s: got %+v want %+v", runner, p, want)
			}
			plugin, ok := registry.SelectionPlugin(runner)
			if !ok || !reflect.DeepEqual(plugin.Pieces(), p) {
				t.Fatalf("%s: registry/manifest differ", runner)
			}
		}
	}
	for _, r := range registry.Runners() {
		if !seen[r] {
			t.Fatalf("%s undeclared", r)
		}
	}
}
