package main

import (
	"bytes"
	"encoding/json"
	"os"
	"testing"

	"github.com/Beamfall/corvint/internal/appflows"
	"github.com/Beamfall/corvint/internal/doccorpus"
)

func qualifyFixtureEvidence(t *testing.T, root, evidence string) {
	t.Helper()
	records, err := appflows.ReadRunEvidence([]string{evidence})
	if err != nil {
		t.Fatal(err)
	}
	before := shopGit(t, root, "rev-parse", "HEAD")
	registry := appflows.RunRegistry{Schema: appflows.RunRegistrySchema, Policy: appflows.RunPolicy{ID: "fixture-one-run"}, Aggregates: []appflows.RunAggregate{}}
	for _, scope := range []string{"one-spec", "feature-batch", "suite"} {
		registry.Policy.Thresholds = append(registry.Policy.Thresholds, doccorpus.StabilityThreshold{Scope: scope, RequiredRepetitions: 1, MinimumPassed: 1})
	}
	for _, r := range records {
		if r.Authority != appflows.AuthorityStatic {
			registry.Aggregates = append(registry.Aggregates, appflows.RunAggregate{ID: r.TestKey + ":" + r.Project, Scope: "one-spec", TestKey: r.TestKey, Project: r.Project, Planned: 1, Contributions: []appflows.RunContribution{{RunID: r.RunID, RunKind: "planned-repetition", Repetition: 1, InfrastructureAttempts: []int{}}}})
		}
	}
	raw, err := json.Marshal(registry)
	if err != nil {
		t.Fatal(err)
	}
	shopWrite(t, root, map[string]string{"runs/registry.json": string(raw)})
	after := shopCommit(t, root, "declare fixture stability policy")
	at := appflows.RunSource{Commit: after, Tree: shopGit(t, root, "rev-parse", "HEAD^{tree}"), Clean: true}
	var lines bytes.Buffer
	for _, r := range records {
		if r.Source.Commit == before {
			r.Source = at
		}
		line, err := appflows.EncodeRunEvidence(r)
		if err != nil {
			t.Fatal(err)
		}
		lines.Write(line)
	}
	if err := os.WriteFile(evidence, lines.Bytes(), 0600); err != nil {
		t.Fatal(err)
	}
}

func TestAFUV1MapAndGapsRegistryQualification(t *testing.T) {
	t.Run("AFU-V1-015 AFU-V1-016 committed registry qualification", func(t *testing.T) {
		fx := newShopFixture(t)
		qualifyFixtureEvidence(t, fx.root, fx.evidence)
		for _, command := range []string{"map", "gaps"} {
			code, out, diag := runFlowsCLI(fx.root, command, "--flows", "flows", "--evidence", fx.evidence, "--registry", "runs/registry.json")
			if code != 0 {
				t.Fatalf("%s: %s", command, diag)
			}
			if command == "map" {
				var report appflows.MapReport
				if err := json.Unmarshal([]byte(out), &report); err != nil {
					t.Fatal(err)
				}
				for _, f := range report.Flows {
					if f.FlowID == "checkout" && (!f.Variations[0].Verified || f.Status != "complete") {
						t.Fatalf("clean aggregate did not verify: %+v", f)
					}
				}
			} else {
				var report appflows.GapsReport
				if err := json.Unmarshal([]byte(out), &report); err != nil {
					t.Fatal(err)
				}
				for _, f := range report.Flows {
					if f.FlowID == "checkout" && len(f.Gaps) != 0 {
						t.Fatalf("qualified checkout gaps: %+v", f)
					}
				}
			}
		}
		code, out, diag := runFlowsCLI(fx.root, "gaps", "--flows", "flows", "--evidence", fx.evidence)
		if code != 0 || !bytes.Contains([]byte(out), []byte(`"code":"stability-missing"`)) {
			t.Fatalf("missing stability gap: %d %s %s", code, out, diag)
		}
	})
}
