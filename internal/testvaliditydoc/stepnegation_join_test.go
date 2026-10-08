//go:build darwin || linux

package testvaliditydoc

import (
	"bytes"
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Beamfall/corvint/internal/jstestprovider"
	"github.com/Beamfall/corvint/internal/stepnegation"
	"github.com/Beamfall/corvint/internal/testvalidity"
)

const (
	joinSpecBody   = "test('total', async () => {})\n"
	joinConfigBody = "module.exports = {}\n"
)

// joinFixture writes a spec and config into a fresh worktree and returns a
// qualified /0 receipt for one test plus the matching retained document.
func joinFixture(t *testing.T) (string, jstestprovider.Receipt, stepnegation.Document) {
	t.Helper()
	root := discoveryRoot(t)
	spec, config := filepath.Join(root, "e2e", "cart.spec.cjs"), filepath.Join(root, "playwright.config.cjs")
	writeAt(t, spec, joinSpecBody, time.Unix(1_700_000_000, 0))
	writeAt(t, config, joinConfigBody, time.Unix(1_700_000_000, 0))
	specDigest, configDigest := digestHex([]byte(joinSpecBody)), digestHex([]byte(joinConfigBody))
	receipt := jstestprovider.Receipt{
		Profile: jstestprovider.ExternalProfile,
		Kind:    "e2e",
		Identity: jstestprovider.Identity{RunnerName: "playwright", RunnerVersion: "1.63.0", NodeVersion: "v22.23.2",
			ConfigFile: config, ConfigDigest: configDigest, TestFileDigests: map[string]string{spec: specDigest}},
		External: &jstestprovider.ExternalLifecycle{Ownership: "external", CleanupResponsibility: "external", ServerDescendants: "unknown", DeclaredAppIdentity: "fixture-app"},
		Tests: []jstestprovider.TestOutcome{{
			Name: "total", FullName: " > chromium > e2e/cart.spec.cjs > cart > total", State: jstestprovider.StatePassed,
			Anchor: &jstestprovider.Anchor{File: spec, Line: 1}, Project: &jstestprovider.ProjectIdentity{Name: "chromium", Browser: "chromium", Device: "none"},
			Attempts: []jstestprovider.Attempt{{State: jstestprovider.StatePassed, FailureKind: "none"}},
		}},
	}
	plan := strings.Repeat("4", 64)
	retained := stepnegation.Document{
		Schema: stepnegation.Schema,
		Binding: stepnegation.Binding{
			TestRepository: stepnegation.Repository{RootCommit: strings.Repeat("a", 40), Revision: strings.Repeat("b", 40), Tree: strings.Repeat("c", 40)},
			ConfigFile:     "playwright.config.cjs", ConfigDigest: configDigest, SpecFile: "e2e/cart.spec.cjs", SpecDigest: specDigest,
			Test:                  stepnegation.TestBinding{File: "e2e/cart.spec.cjs", FullTitle: "cart > total", Project: "chromium", Browser: "chromium", Device: "none"},
			Runner:                stepnegation.Runner{Name: "playwright", Version: "1.63.0", NodeVersion: "v22.23.2", Tuple: jstestprovider.RuntimeTupleCandidate},
			Application:           stepnegation.Application{Profile: jstestprovider.ExternalProfile, Label: "fixture-app"},
			ReadinessOrigin:       "http://127.0.0.1:4000",
			InjectionModuleDigest: strings.Repeat("3", 64),
		},
		Mode:      stepnegation.ModeStep,
		Runs:      stepnegation.Runs{Budget: 3, Used: 2, BaselineRepeat: 1, BaselinePassed: 1},
		Inventory: stepnegation.Inventory{Steps: []stepnegation.InventoryStep{{Title: "open", Ordinal: 1}, {Title: "total", Ordinal: 2, Assertions: 1}}},
		Steps: []stepnegation.Step{{PlanDigest: plan, Title: "total", Ordinal: 2, Witness: stepnegation.WitnessNetwork,
			Strength: testvalidity.Axis{State: testvalidity.StrengthKilled, Reason: stepnegation.ReasonKilled, Anchors: []string{"plan:" + plan}}}},
	}
	return root, receipt, retained
}

func retainNegation(t *testing.T, root string, document stepnegation.Document) {
	t.Helper()
	if err := stepnegation.Retain(root, document); err != nil {
		t.Fatal(err)
	}
}

func documentBytes(t *testing.T, document Document) []byte {
	t.Helper()
	data, err := json.Marshal(document)
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func withJoinGate(t *testing.T, qualified bool) {
	t.Helper()
	previous := stepNegationJoinQualified
	stepNegationJoinQualified = qualified
	t.Cleanup(func() { stepNegationJoinQualified = previous })
}

// LPCV-V0-068, LPCV-V0-070: while the live matrix is unqualified the join is
// gated off, so a joinable document changes no output byte.
func TestStepNegationJoinGatedOffIsByteIdentical(t *testing.T) {
	withJoinGate(t, false)
	root, receipt, retained := joinFixture(t)
	retainNegation(t, root, retained)
	input := Input{js: &receipt}
	if got, want := documentBytes(t, JoinStepNegation(Project(input), input, root)), documentBytes(t, Project(input)); !bytes.Equal(got, want) {
		t.Fatalf("gated join changed bytes:\n%s\n%s", got, want)
	}
	if Project(input).Tests[0].Projection.Strength.State != testvalidity.StrengthNotMeasured {
		t.Fatal("receipt strength is not the unjoined NOT_MEASURED")
	}
}

// LPCV-V0-068: a joinable document sets the test's strength by the
// inventory aggregation; every mismatch, and no document, leaves the bytes
// unchanged.
func TestStepNegationJoinConditions(t *testing.T) {
	withJoinGate(t, true)
	t.Run("joinable", func(t *testing.T) {
		root, receipt, retained := joinFixture(t)
		retainNegation(t, root, retained)
		input := Input{js: &receipt}
		strength := JoinStepNegation(Project(input), input, root).Tests[0].Projection.Strength
		if strength.State != testvalidity.StrengthKilled || strength.Reason != stepnegation.ReasonStepControlsKilled || len(strength.Anchors) != 2 || !strings.HasPrefix(strength.Anchors[0], stepNegationAnchorPrefix+stepnegation.EvidenceDirectory+"/") {
			t.Fatalf("strength = %+v", strength)
		}
	})
	t.Run("no document", func(t *testing.T) {
		root, receipt, _ := joinFixture(t)
		input := Input{js: &receipt}
		if got, want := documentBytes(t, JoinStepNegation(Project(input), input, root)), documentBytes(t, Project(input)); !bytes.Equal(got, want) {
			t.Fatal("join without a document changed bytes")
		}
	})
	for _, test := range []struct {
		name           string
		mutateReceipt  func(*jstestprovider.Receipt)
		mutateRetained func(*stepnegation.Document)
		mutateTree     func(t *testing.T, root string)
	}{
		{name: "runner version", mutateRetained: func(d *stepnegation.Document) { d.Binding.Runner.Version = "1.60.0" }},
		{name: "node version", mutateRetained: func(d *stepnegation.Document) { d.Binding.Runner.NodeVersion = "v24.11.1" }},
		{name: "unqualified negate tuple", mutateRetained: func(d *stepnegation.Document) { d.Binding.Runner.Tuple = jstestprovider.RuntimeTupleUnqualified }},
		{name: "unqualified receipt tuple", mutateReceipt: func(r *jstestprovider.Receipt) { r.Identity.NodeVersion = "v22.23.3" }, mutateRetained: func(d *stepnegation.Document) { d.Binding.Runner.NodeVersion = "v22.23.3" }},
		{name: "application label", mutateRetained: func(d *stepnegation.Document) { d.Binding.Application.Label = "other-app" }},
		{name: "application profile", mutateRetained: func(d *stepnegation.Document) { d.Binding.Application.Profile = jstestprovider.AttestedExternalProfile }},
		{name: "browser", mutateRetained: func(d *stepnegation.Document) { d.Binding.Test.Browser = "webkit" }},
		{name: "receipt spec digest", mutateReceipt: func(r *jstestprovider.Receipt) {
			for path := range r.Identity.TestFileDigests {
				r.Identity.TestFileDigests[path] = strings.Repeat("9", 64)
			}
		}},
		{name: "receipt config digest", mutateReceipt: func(r *jstestprovider.Receipt) { r.Identity.ConfigDigest = strings.Repeat("9", 64) }},
		{name: "worktree spec changed", mutateTree: func(t *testing.T, root string) {
			writeAt(t, filepath.Join(root, "e2e", "cart.spec.cjs"), "changed\n", time.Unix(1_700_000_001, 0))
		}},
		{name: "worktree config changed", mutateTree: func(t *testing.T, root string) {
			writeAt(t, filepath.Join(root, "playwright.config.cjs"), "changed\n", time.Unix(1_700_000_001, 0))
		}},
		{name: "receipt project", mutateReceipt: func(r *jstestprovider.Receipt) { r.Tests[0].Project.Name = "webkit" }},
	} {
		t.Run(test.name, func(t *testing.T) {
			root, receipt, retained := joinFixture(t)
			if test.mutateRetained != nil {
				test.mutateRetained(&retained)
			}
			retainNegation(t, root, retained)
			if test.mutateReceipt != nil {
				test.mutateReceipt(&receipt)
			}
			if test.mutateTree != nil {
				test.mutateTree(t, root)
			}
			input := Input{js: &receipt}
			if got, want := documentBytes(t, JoinStepNegation(Project(input), input, root)), documentBytes(t, Project(input)); !bytes.Equal(got, want) {
				t.Fatalf("mismatched document joined: %s", got)
			}
		})
	}
	t.Run("incomplete and survived aggregates", func(t *testing.T) {
		root, receipt, retained := joinFixture(t)
		retained.Steps[0].Strength = testvalidity.Axis{State: testvalidity.StrengthSurvived, Reason: stepnegation.ReasonFaultSurvived, Anchors: []string{}}
		retainNegation(t, root, retained)
		input := Input{js: &receipt}
		if strength := JoinStepNegation(Project(input), input, root).Tests[0].Projection.Strength; strength.State != testvalidity.StrengthSurvived {
			t.Fatalf("survived = %+v", strength)
		}
		retained.Steps = []stepnegation.Step{}
		retainNegation(t, root, retained)
		if strength := JoinStepNegation(Project(input), input, root).Tests[0].Projection.Strength; strength.State != testvalidity.StrengthNotMeasured || strength.Reason != stepnegation.ReasonStepControlsIncomplete {
			t.Fatalf("incomplete = %+v", strength)
		}
	})
	t.Run("attested application", func(t *testing.T) {
		root, receipt, retained := joinFixture(t)
		receipt.Profile = jstestprovider.AttestedExternalProfile
		observed := jstestprovider.ApplicationAttestation{
			Repository: jstestprovider.ApplicationRepositoryIdentity{RootCommit: strings.Repeat("d", 40), Revision: strings.Repeat("e", 40), Tree: strings.Repeat("f", 40)},
			Instance:   jstestprovider.ApplicationInstanceIdentity{Kind: "process", ID: "fixture-1", StartGeneration: "1"},
		}
		receipt.ApplicationAttestation = &jstestprovider.ApplicationAttestationReceipt{Before: &jstestprovider.ApplicationAttestationObservation{Attestation: observed}, Failures: []string{}}
		retained.Binding.Application = stepnegation.Application{Profile: jstestprovider.AttestedExternalProfile, RootCommit: observed.Repository.RootCommit, Revision: observed.Repository.Revision, Tree: observed.Repository.Tree, InstanceKind: "process", InstanceID: "fixture-1", StartGeneration: "1"}
		retainNegation(t, root, retained)
		input := Input{js: &receipt}
		if strength := JoinStepNegation(Project(input), input, root).Tests[0].Projection.Strength; strength.State != testvalidity.StrengthKilled {
			t.Fatalf("attested join = %+v", strength)
		}
		receipt.ApplicationAttestation.Before.Attestation.Instance.StartGeneration = "2"
		if strength := JoinStepNegation(Project(input), input, root).Tests[0].Projection.Strength; strength.State == testvalidity.StrengthKilled {
			t.Fatal("a restarted application instance joined")
		}
	})
	t.Run("discover joins the selected receipt", func(t *testing.T) {
		root, receipt, retained := joinFixture(t)
		retainNegation(t, root, retained)
		data, err := jstestprovider.EncodeQualified(receipt)
		if err != nil {
			t.Fatal(err)
		}
		writeAt(t, evidencePath(root, "run.json"), string(data), time.Unix(1_800_000_000, 0))
		document := discover(t, root)
		if len(document.Tests) != 1 || document.Tests[0].Projection.Strength.State != testvalidity.StrengthKilled {
			t.Fatalf("discovered = %+v", document.Tests)
		}
	})
}
