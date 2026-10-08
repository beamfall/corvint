//go:build darwin || linux

package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/stepnegation"
	"github.com/Beamfall/corvint/internal/testvalidity"
)

// negateProvider writes a fake provider script: it records its argv, prints
// the file named by $NEGATE_FIXTURE, and exits $NEGATE_EXIT.
func negateProvider(t *testing.T) (provider, argvFile string) {
	t.Helper()
	directory := t.TempDir()
	provider, argvFile = filepath.Join(directory, "provider"), filepath.Join(directory, "argv")
	script := "#!/bin/sh\nprintf '%s\\n' \"$@\" > " + argvFile + "\ncat \"$NEGATE_FIXTURE\"\nexit \"${NEGATE_EXIT:-0}\"\n"
	if err := os.WriteFile(provider, []byte(script), 0o700); err != nil {
		t.Fatal(err)
	}
	return provider, argvFile
}

func negateFixture(t *testing.T, data []byte, exit string) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "fixture")
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("NEGATE_FIXTURE", path)
	t.Setenv("NEGATE_EXIT", exit)
}

func negateDocument(steps ...stepnegation.Step) stepnegation.Document {
	return stepnegation.Document{
		Schema: stepnegation.Schema,
		Binding: stepnegation.Binding{
			TestRepository: stepnegation.Repository{RootCommit: strings.Repeat("a", 40), Revision: strings.Repeat("b", 40), Tree: strings.Repeat("c", 40)},
			ConfigFile:     "playwright.config.cjs", ConfigDigest: strings.Repeat("1", 64), SpecFile: "cart.spec.cjs", SpecDigest: strings.Repeat("2", 64),
			Test:                  stepnegation.TestBinding{File: "cart.spec.cjs", FullTitle: "cart > total", Project: "chromium", Browser: "chromium", Device: "none"},
			Runner:                stepnegation.Runner{Name: "playwright", Version: "1.63.0", NodeVersion: "v22.23.2", Tuple: "candidate"},
			Application:           stepnegation.Application{Profile: "corvint-playwright-external/0", Label: "fixture-app"},
			ReadinessOrigin:       "http://127.0.0.1:4000",
			InjectionModuleDigest: strings.Repeat("3", 64),
		},
		Mode:      stepnegation.ModeStep,
		Runs:      stepnegation.Runs{Budget: 3, Used: 2, BaselineRepeat: 1, BaselinePassed: 1},
		Inventory: stepnegation.Inventory{Steps: []stepnegation.InventoryStep{{Title: "total", Ordinal: 1, Assertions: 1}, {Title: "title", Ordinal: 2, Assertions: 1}}},
		Steps:     steps,
	}
}

func killedStep(ordinal int, title, plan string) stepnegation.Step {
	return stepnegation.Step{PlanDigest: plan, Title: title, Ordinal: ordinal, Witness: stepnegation.WitnessNetwork,
		Strength: testvalidity.Axis{State: testvalidity.StrengthKilled, Reason: stepnegation.ReasonKilled, Anchors: []string{"plan:" + plan}}}
}

func manualStep(ordinal int, title string) stepnegation.Step {
	return stepnegation.Step{Title: title, Ordinal: ordinal, Witness: stepnegation.WitnessNone, Requires: stepnegation.RequiresManualControl,
		Strength: testvalidity.Axis{State: testvalidity.StrengthNotMeasured, Reason: stepnegation.ReasonUnderivable, Anchors: []string{"reason:regex-expected"}}}
}

func encodedNegation(t *testing.T, document stepnegation.Document) []byte {
	t.Helper()
	data, err := stepnegation.Encode(document)
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func runNegateCommand(root string, arguments ...string) (int, string, string) {
	var stdout, stderr bytes.Buffer
	status := runTestValidity(root, append([]string{"negate"}, arguments...), &stdout, &stderr)
	return status, stdout.String(), stderr.String()
}

// LPCV-V0-057, LPCV-V0-065, LPCV-V0-067: the core execs only the named
// provider, prints the run document, renders UNPROVEN, and retains the merged
// document atomically per test identity.
func TestNegateExecsProviderPrintsAndRetains(t *testing.T) {
	worktree, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	provider, argvFile := negateProvider(t)
	planA, planB := strings.Repeat("5", 64), strings.Repeat("6", 64)
	selection := []string{"--provider", provider, "--spec=cart.spec.cjs", "--test", "cart > total", "--project=chromium", "--step=total", "--app-identity=fixture-app"}

	first := negateDocument(killedStep(1, "total", planA))
	negateFixture(t, encodedNegation(t, first), "0")
	status, stdout, stderr := runNegateCommand(worktree, selection...)
	if status != 0 || stdout != string(encodedNegation(t, first)) {
		t.Fatalf("status %d stdout %q stderr %q", status, stdout, stderr)
	}
	argv, _ := os.ReadFile(argvFile)
	if want := "negate\n--root=" + worktree + "\n--spec=cart.spec.cjs\n--test=cart > total\n--project=chromium\n--step=total\n--app-identity=fixture-app\n"; string(argv) != want {
		t.Fatalf("provider argv = %q, want %q", argv, want)
	}
	retainedPath := filepath.Join(worktree, stepnegation.EvidenceDirectory, stepnegation.FileName(first.Binding.Test.Key()))
	info, err := os.Lstat(retainedPath)
	if err != nil || info.Mode().Perm() != 0o600 {
		t.Fatalf("retained file = %v, %v", info, err)
	}
	if !strings.Contains(stderr, `step 1 "total": strength: KILLED`) || !strings.Contains(stderr, "diagnostic, not joined") {
		t.Fatalf("summary = %q", stderr)
	}

	// A second run with the same binding measures step 2 and re-derives the
	// same plan for step 1: the retained file keeps both entries and the
	// printed run reports planReused.
	second := negateDocument(killedStep(1, "total", planA), manualStep(2, "title"))
	negateFixture(t, encodedNegation(t, second), "0")
	status, stdout, stderr = runNegateCommand(worktree, selection...)
	reported, err := stepnegation.Decode([]byte(stdout))
	if status != 0 || err != nil || !reported.Steps[0].PlanReused || reported.Steps[1].PlanReused {
		t.Fatalf("status %d err %v stdout %q", status, err, stdout)
	}
	if !strings.Contains(stderr, `step 2 "title": strength: UNPROVEN (manual control needed)`) {
		t.Fatalf("summary = %q", stderr)
	}
	retained, err := stepnegation.Load(worktree, first.Binding.Test.Key())
	if err != nil || retained == nil || len(retained.Steps) != 2 {
		t.Fatalf("retained = %+v, %v", retained, err)
	}

	// Measuring only step 2 keeps the stored step 1 entry.
	third := negateDocument(killedStep(2, "title", planB))
	negateFixture(t, encodedNegation(t, third), "0")
	if status, _, stderr = runNegateCommand(worktree, selection...); status != 0 {
		t.Fatalf("status %d stderr %q", status, stderr)
	}
	retained, _ = stepnegation.Load(worktree, first.Binding.Test.Key())
	if retained == nil || len(retained.Steps) != 2 || retained.Steps[0].PlanDigest != planA || retained.Steps[1].PlanDigest != planB {
		t.Fatalf("merged = %+v", retained)
	}
	if aggregate := stepnegation.Aggregate(*retained); aggregate.State != testvalidity.StrengthKilled {
		t.Fatalf("aggregate = %+v", aggregate)
	}

	// Incomplete cleanup still prints the document but exits nonzero.
	unclean := negateDocument(killedStep(1, "total", planA))
	unclean.CleanupIncomplete = true
	negateFixture(t, encodedNegation(t, unclean), "0")
	status, stdout, stderr = runNegateCommand(worktree, selection...)
	if reported, err = stepnegation.Decode([]byte(stdout)); status != 1 || err != nil || !reported.CleanupIncomplete || !strings.Contains(stderr, "cleanup incomplete") {
		t.Fatalf("unclean: status %d stdout %q stderr %q", status, stdout, stderr)
	}
}

// LPCV-V0-057, LPCV-V0-065, LPCV-V0-067: refusals exit 2 with no stdout.
func TestNegateRefusalsAndBusyEvidence(t *testing.T) {
	worktree, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	provider, argvFile := negateProvider(t)
	selection := []string{"--provider=" + provider, "--spec=cart.spec.cjs", "--test=cart > total", "--project=chromium", "--step=total"}
	for _, test := range []struct {
		name, code string
		arguments  []string
	}{
		{"no provider", "invalid-arguments", []string{"--spec=cart.spec.cjs"}},
		{"relative provider", "invalid-arguments", []string{"--provider=provider"}},
		{"directory provider", "invalid-arguments", []string{"--provider=" + filepath.Dir(provider)}},
		{"provider twice", "invalid-arguments", append(append([]string{}, selection...), "--provider="+provider)},
		{"root passthrough", "invalid-arguments", append(append([]string{}, selection...), "--root=/elsewhere")},
		{"missing value", "invalid-arguments", []string{"--provider=" + provider, "--spec"}},
	} {
		t.Run(test.name, func(t *testing.T) {
			status, stdout, stderr := runNegateCommand(worktree, test.arguments...)
			if status != 2 || stdout != "" || !strings.Contains(stderr, `"`+test.code+`"`) {
				t.Fatalf("status %d stdout %q stderr %q", status, stdout, stderr)
			}
		})
	}
	if _, err := os.Stat(argvFile); err == nil {
		t.Fatal("an argument refusal executed the provider")
	}

	t.Run("typed provider refusal", func(t *testing.T) {
		negateFixture(t, []byte(`{"schema":"corvint-step-negation-refusal/0","code":"runtime-tuple-unqualified","message":"node"}`+"\n"), "2")
		status, stdout, stderr := runNegateCommand(worktree, selection...)
		if status != 2 || stdout != "" || !strings.Contains(stderr, `"runtime-tuple-unqualified"`) {
			t.Fatalf("status %d stdout %q stderr %q", status, stdout, stderr)
		}
	})
	for name, fixture := range map[string][2]string{
		"exit 2 without refusal":      {"garbage", "2"},
		"refusal with unknown member": {`{"schema":"corvint-step-negation-refusal/0","code":"x","message":"","extra":1}`, "2"},
		"crash":                       {"", "3"},
		"noncanonical document":       {`{"schema":"corvint-step-negation/0"}`, "0"},
	} {
		t.Run(name, func(t *testing.T) {
			negateFixture(t, []byte(fixture[0]), fixture[1])
			status, stdout, stderr := runNegateCommand(worktree, selection...)
			if status != 2 || stdout != "" || !strings.Contains(stderr, `"negate-provider-failed"`) {
				t.Fatalf("status %d stdout %q stderr %q", status, stdout, stderr)
			}
		})
	}
	t.Run("oversized output", func(t *testing.T) {
		negateFixture(t, bytes.Repeat([]byte("x"), stepnegation.MaxDocumentBytes+1), "0")
		if status, stdout, stderr := runNegateCommand(worktree, selection...); status != 2 || stdout != "" || !strings.Contains(stderr, "exceeds 4 MiB") {
			t.Fatalf("status %d stderr %q", status, stderr)
		}
	})

	document := negateDocument(killedStep(1, "total", strings.Repeat("5", 64)))
	release, err := stepnegation.Lock(worktree, document.Binding.Test.Key())
	if err != nil {
		t.Fatal(err)
	}
	defer release()
	negateFixture(t, encodedNegation(t, document), "0")
	t.Run("busy identity refuses before any run", func(t *testing.T) {
		_ = os.Remove(argvFile)
		status, stdout, stderr := runNegateCommand(worktree, selection...)
		if status != 2 || stdout != "" || !strings.Contains(stderr, `"negate-evidence-busy"`) {
			t.Fatalf("status %d stdout %q stderr %q", status, stdout, stderr)
		}
		if _, err := os.Stat(argvFile); err == nil {
			t.Fatal("a busy refusal executed the provider")
		}
	})
	t.Run("busy identity without a project is a retention failure", func(t *testing.T) {
		status, stdout, stderr := runNegateCommand(worktree, "--provider="+provider, "--spec=cart.spec.cjs", "--test=cart > total", "--step=total")
		if status != 1 || stdout != string(encodedNegation(t, document)) || !strings.Contains(stderr, "retention failure:") || !strings.Contains(stderr, "negate-evidence-busy") {
			t.Fatalf("status %d stdout %q stderr %q", status, stdout, stderr)
		}
	})
	t.Run("provider names another identity", func(t *testing.T) {
		other := negateDocument(killedStep(1, "total", strings.Repeat("5", 64)))
		other.Binding.Test.Project = "webkit"
		negateFixture(t, encodedNegation(t, other), "0")
		status, stdout, stderr := runNegateCommand(worktree, "--provider="+provider, "--spec=cart.spec.cjs", "--test=cart > total", "--project=firefox", "--step=total")
		if status != 2 || stdout != "" || !strings.Contains(stderr, "different test identity") {
			t.Fatalf("status %d stdout %q stderr %q", status, stdout, stderr)
		}
	})
}

// LPCV-V0-069: a secret-flagged title is retained only as its SHA-256, and the
// core locks and retains under that same screened identity.
func TestNegateLocksScreenedTitle(t *testing.T) {
	worktree, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	provider, _ := negateProvider(t)
	title := "cart --pass=synthetic123"
	document := negateDocument(killedStep(1, "total", strings.Repeat("5", 64)))
	document.Binding.Test.FullTitle = stepnegation.Screen(title)
	if document.Binding.Test.FullTitle == title {
		t.Fatal("fixture title is not secret-flagged")
	}
	negateFixture(t, encodedNegation(t, document), "0")
	if status, _, stderr := runNegateCommand(worktree, "--provider="+provider, "--spec=cart.spec.cjs", "--test="+title, "--project=chromium", "--step=total"); status != 0 {
		t.Fatalf("status %d stderr %q", status, stderr)
	}
	if retained, err := stepnegation.Load(worktree, document.Binding.Test.Key()); err != nil || retained == nil {
		t.Fatalf("retained = %v, %v", retained, err)
	}
}
