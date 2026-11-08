// SPDX-License-Identifier: Apache-2.0
package runnerpolicy

import (
	"context"
	"reflect"
	"strings"
	"testing"
	"time"

	q "github.com/Beamfall/corvint/internal/runnerqualification"
)

// RSP-V0-057: a qualification presented with its own declaration can clear
// only the exact retained scope, even when its sequence is newer than the miss.
func TestRSPV0057RequalificationExactRetainedIdentity(t *testing.T) {
	for _, scope := range []string{"same", "repository", "os", "architecture", "configuration", "environment"} {
		t.Run(scope, func(t *testing.T) {
			req := fixture()
			p := retained(t, req)
			findings := Detect(context.Background(), req.Sampling, true, p, 10, captureFixture(p, false)).Findings
			if len(findings) != 1 || findings[0].Kind != "miss" || findings[0].Sequence != 10 {
				t.Fatalf("expected confirmed miss at 10: %+v", findings)
			}
			d, records := qualificationFixture(req, 11)
			switch scope {
			case "repository":
				d.Corpus.Repository = "other"
			case "os":
				d.Identity.Execution.OS = "other"
			case "architecture":
				d.Identity.Execution.Architecture = "other"
			case "configuration":
				d.Identity.Runner.ConfigurationDigest = strings.Repeat("e", 64)
			case "environment":
				d.Identity.Execution.EnvironmentDigest = strings.Repeat("e", 64)
			}
			records[0].Record.Declaration = d
			if plan := q.PlanFor(d, records, time.Minute, true); plan.Status != "qualified" || plan.Sequence != 11 {
				t.Fatalf("qualification must pass with its own declaration: %+v", plan)
			}
			kept, err := Requalify(findings, d, records, time.Minute)
			if err != nil {
				t.Fatal(err)
			}
			if scope == "same" {
				if len(kept) != 0 {
					t.Fatalf("same-scope qualification did not clear miss: %+v", kept)
				}
			} else if !reflect.DeepEqual(kept, findings) {
				t.Fatalf("different scope changed retained miss: %+v", kept)
			}
			req.Findings = kept
			if full := runOf(ResolveCI(req)).Full; full != (scope != "same") {
				t.Fatalf("ResolveCI FULL = %v for scope %s", full, scope)
			}
		})
	}
}

// RSP-V0-057: incomplete legacy plans and unknown identities cannot lose a
// block. Test both sides, including equal unknown fields, not just inequality.
func TestRSPV0057RequalificationMissingOrUnknownIdentity(t *testing.T) {
	for _, field := range []string{"repository", "os", "architecture", "configuration", "environment"} {
		for _, value := range []string{"", "unknown", "UNKNOWN"} {
			for _, side := range []string{"miss", "qualification", "both"} {
				t.Run(field+"/"+value+"/"+side, func(t *testing.T) {
					req := fixture()
					p := retained(t, req)
					findings := Detect(context.Background(), req.Sampling, true, p, 10, captureFixture(p, false)).Findings
					if len(findings) != 1 || findings[0].Kind != "miss" {
						t.Fatalf("expected confirmed miss: %+v", findings)
					}
					d, records := qualificationFixture(req, 11)
					plan := &findings[0].Plan
					var missField, qualificationField *string
					switch field {
					case "repository":
						missField, qualificationField = &plan.repository, &d.Corpus.Repository
					case "os":
						missField, qualificationField = &plan.inventory.Key.OS, &d.Identity.Execution.OS
					case "architecture":
						missField, qualificationField = &plan.inventory.Key.Arch, &d.Identity.Execution.Architecture
					case "configuration":
						missField, qualificationField = &plan.inventory.Key.ConfigDigest, &d.Identity.Runner.ConfigurationDigest
					case "environment":
						missField, qualificationField = &plan.environmentDigest, &d.Identity.Execution.EnvironmentDigest
					}
					if side != "qualification" {
						*missField = value
					}
					if side != "miss" {
						*qualificationField = value
					}
					records[0].Record.Declaration = d
					kept, err := Requalify(findings, d, records, time.Minute)
					if err != nil {
						if side == "miss" || err.Error() != "requalification-not-admitted" {
							t.Fatal(err)
						}
						// A refused qualification leaves the caller's history active.
						kept = findings
					}
					if !reflect.DeepEqual(kept, findings) {
						t.Fatalf("incomplete identity changed retained miss: %+v", kept)
					}
					req.Findings = kept
					if !runOf(ResolveCI(req)).Full {
						t.Fatal("incomplete identity allowed narrowing")
					}
				})
			}
		}
	}
}
