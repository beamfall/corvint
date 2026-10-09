package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/jstestprovider"
	"github.com/Beamfall/corvint/internal/stepnegation"
	"github.com/Beamfall/corvint/internal/testvalidity"
)

// LPCV-V0-057, LPCV-V0-058: parse refusals are typed before any run.
func TestParseNegateRefusals(t *testing.T) {
	root := t.TempDir()
	for _, test := range []struct {
		name, code string
		args       []string
	}{
		{"keep reporters", jstestprovider.NegateModeUnsupported, []string{"--root=" + root, "--keep-reporters"}},
		{"profile /2", jstestprovider.NegateModeUnsupported, []string{"--root=" + root, "--sensitive-input-redaction=on"}},
		{"profile /3", jstestprovider.NegateModeUnsupported, []string{"-retain-attempt-details"}},
		{"owned server argv", jstestprovider.NegateModeUnsupported, []string{"--server-arg=node"}},
		{"watch", jstestprovider.NegateModeUnsupported, []string{"--watch"}},
		{"owned server", jstestprovider.NegateModeUnsupported, []string{"--root=" + root, "--external-server=false"}},
		{"unknown flag", jstestprovider.NegateInvalidArguments, []string{"--root=" + root, "--frobnicate"}},
		{"positional", jstestprovider.NegateInvalidArguments, []string{"--root=" + root, "extra"}},
		{"missing root", jstestprovider.NegateInvalidArguments, []string{"--spec=a.spec.cjs"}},
		{"explicit zero baseline repeat", jstestprovider.NegateInvalidArguments, []string{"--root=" + root, "--baseline-repeat=0"}},
		{"negative max runs", jstestprovider.NegateInvalidArguments, []string{"--root=" + root, "--max-runs=-1"}},
	} {
		t.Run(test.name, func(t *testing.T) {
			_, err := parseNegate(test.args)
			var refusal *jstestprovider.NegateRefusal
			if !errors.As(err, &refusal) || refusal.Code != test.code {
				t.Fatalf("parseNegate = %v, want %s", err, test.code)
			}
		})
	}
	cfg, err := parseNegate([]string{"--root=" + root, "--spec=a.spec.cjs", "--test=a > b", "--step=total", "--max-runs=4", "--baseline-repeat=2", "--app-identity=fixture-app"})
	if err != nil || !cfg.E2E.ExternalServer || cfg.Step != "total" || cfg.MaxRuns != 4 || cfg.BaselineRepeat != 2 || cfg.E2E.Dir != cfg.Root || cfg.E2E.AppIdentity != "fixture-app" {
		t.Fatalf("parseNegate = %+v, %v", cfg, err)
	}
}

func providerDocument() stepnegation.Document {
	plan := strings.Repeat("4", 64)
	return stepnegation.Document{
		Schema: stepnegation.Schema,
		Binding: stepnegation.Binding{
			TestRepository: stepnegation.Repository{RootCommit: strings.Repeat("a", 40), Revision: strings.Repeat("b", 40), Tree: strings.Repeat("c", 40)},
			ConfigFile:     "playwright.config.cjs", ConfigDigest: strings.Repeat("1", 64), SpecFile: "a.spec.cjs", SpecDigest: strings.Repeat("2", 64),
			Test:                  stepnegation.TestBinding{File: "a.spec.cjs", FullTitle: "a > b", Project: "chromium", Browser: "chromium", Device: "none"},
			Runner:                stepnegation.Runner{Name: "playwright", Version: "1.63.0", NodeVersion: "v22.23.2", Tuple: "candidate"},
			Application:           stepnegation.Application{Profile: "corvint-playwright-external/0", Label: "fixture-app"},
			ReadinessOrigin:       "http://127.0.0.1:4000",
			InjectionModuleDigest: strings.Repeat("3", 64),
		},
		Mode:      stepnegation.ModeStep,
		Runs:      stepnegation.Runs{Budget: 3, Used: 2, BaselineRepeat: 1, BaselinePassed: 1},
		Inventory: stepnegation.Inventory{Steps: []stepnegation.InventoryStep{{Title: "total", Ordinal: 1, Assertions: 1}}},
		Steps: []stepnegation.Step{{PlanDigest: plan, Title: "total", Ordinal: 1, Witness: stepnegation.WitnessNetwork,
			Strength: testvalidity.Axis{State: testvalidity.StrengthKilled, Reason: stepnegation.ReasonKilled, Anchors: []string{"plan:" + plan}}}},
	}
}

// LPCV-V0-065: the provider writes exactly the canonical document, or one
// closed refusal document and exit 2.
func TestRunNegateWritesDocumentOrClosedRefusal(t *testing.T) {
	previous := negateRunner
	t.Cleanup(func() { negateRunner = previous })
	root := t.TempDir()
	args := []string{"--root=" + root, "--spec=a.spec.cjs", "--test=a > b", "--step=total"}

	negateRunner = func(context.Context, jstestprovider.NegateConfig) (stepnegation.Document, error) {
		return providerDocument(), nil
	}
	var stdout bytes.Buffer
	if err := runNegate(context.Background(), args, &stdout); err != nil {
		t.Fatal(err)
	}
	want, _ := stepnegation.Encode(providerDocument())
	if !bytes.Equal(stdout.Bytes(), want) {
		t.Fatalf("stdout = %s", stdout.Bytes())
	}

	negateRunner = func(context.Context, jstestprovider.NegateConfig) (stepnegation.Document, error) {
		return stepnegation.Document{}, &jstestprovider.NegateRefusal{Code: jstestprovider.NegateTupleUnqualified, Message: "node v0"}
	}
	stdout.Reset()
	if err := runNegate(context.Background(), args, &stdout); !errors.Is(err, errRefused) {
		t.Fatalf("refusal error = %v", err)
	}
	var refusal map[string]any
	decoder := json.NewDecoder(&stdout)
	if err := decoder.Decode(&refusal); err != nil || len(refusal) != 3 || refusal["schema"] != RefusalSchema || refusal["code"] != jstestprovider.NegateTupleUnqualified || decoder.More() {
		t.Fatalf("refusal = %v, %v", refusal, err)
	}

	negateRunner = func(context.Context, jstestprovider.NegateConfig) (stepnegation.Document, error) {
		return stepnegation.Document{}, errors.New("runner crashed")
	}
	stdout.Reset()
	if err := runNegate(context.Background(), args, &stdout); err == nil || errors.Is(err, errRefused) || stdout.Len() != 0 {
		t.Fatalf("untyped failure = %v, stdout %q", err, stdout.Bytes())
	}

	stdout.Reset()
	if err := runNegate(context.Background(), []string{"--keep-reporters"}, &stdout); !errors.Is(err, errRefused) || !strings.Contains(stdout.String(), jstestprovider.NegateModeUnsupported) {
		t.Fatalf("parse refusal = %v, %q", err, stdout.Bytes())
	}
}
