// SPDX-License-Identifier: Apache-2.0

package registry_test

import (
	"context"
	"testing"

	"github.com/Beamfall/corvint/internal/rubylinecov"
	"github.com/Beamfall/corvint/internal/testevidence"
	tr "github.com/Beamfall/corvint/internal/testrunner"
	"github.com/Beamfall/corvint/internal/testrunner/registry"
)

func TestRSPV0048RubyGloballyRegistered(t *testing.T) {
	for _, runner := range rubyRunners {
		_, key, report := rubyFixture(t, runner, nil)
		p, ok := registry.SelectionPlugin(runner)
		if !ok || !p.Pieces().Inventory.Present || !p.Pieces().TestEvidence.Present || p.Pieces().Qualification.Present || p.Pieces().FullReason(true) == "" {
			t.Fatalf("%s: %+v", runner, p)
		}
		in, err := p.Inventory(context.Background(), registry.InventoryRequest{Key: key, Output: report})
		if err != nil || !in.Complete || len(in.Tests) != 2 {
			t.Fatalf("%s: %+v %v", runner, in, err)
		}
		results, err := p.Parse(context.Background(), registry.ParseRequest{Runner: runner, Report: report})
		if err != nil || len(results) != 2 {
			t.Fatalf("%s: %+v %v", runner, results, err)
		}
		plan := registry.NewSelectionSession(p, key, true).Plan(in, []tr.Exclusion{{ID: in.Tests[0].ID, Fresh: true}}, nil)
		if !plan.Full || len(plan.Tests) != 2 || plan.Reason == "" {
			t.Fatal(plan)
		}
	}
}

func TestRSPV0005RubyRSpecDotSlashEvidence(t *testing.T) {
	const id = "./spec/x_spec.rb[1:1]"
	native := testevidence.Native{Executed: testevidence.Component[testevidence.ExecutedFile]{State: testevidence.Present}, Regions: testevidence.Missing[testevidence.RegionsFile]("fixture"), Loaded: testevidence.Missing[testevidence.Blob]("fixture")}
	capture := func(context.Context, tr.InventoryKey, []string) ([]rubylinecov.SelectionNative, error) {
		return []rubylinecov.SelectionNative{{Package: "./spec/x_spec.rb", Name: id, Native: native}}, nil
	}
	p, _, _ := rubyFixture(t, "rspec", capture)
	got, err := p.Evidence(context.Background(), registry.EvidenceRequest{Runner: "rspec", Artifacts: []string{"capture"}})
	if err != nil || len(got) != 1 || got[0].Test != id {
		t.Fatal(got, err)
	}
}
