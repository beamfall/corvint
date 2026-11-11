// SPDX-License-Identifier: Apache-2.0
package runnerprotocol

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"os"
	"reflect"
	"testing"
	"time"

	"github.com/Beamfall/corvint/internal/execfile"
	"github.com/Beamfall/corvint/internal/testrunner/registry"
)

// hostPins installs the test binary as the reference plugin and returns the
// repository-shaped pin (source label only) and the operator store.
func hostPins(t testing.TB) (Pin, Discovery, string) {
	t.Helper()
	pin, source := selfPin(t)
	installed, err := Install(t.TempDir(), pin, source)
	if err != nil {
		t.Fatal(err)
	}
	pin.Executable = "./corvint-runner-reference"
	return pin, Discovery{Schema: DiscoverySchema, Plugins: []Pin{installed}}, installed.Executable
}

func hostOptions(t testing.TB, mode string, limits Limits) Options {
	t.Helper()
	return Options{Repository: fixtureRepo(t, mode), RunDir: t.TempDir(), CoreVersion: "test", Limits: limits}
}

func wantFail(t *testing.T, lines []Line, reason string) {
	t.Helper()
	want := []Line{{Runner: ReferenceRunner, Mode: ModeFull, Reason: reason, Run: RunFail, RunReason: RunnerAdapterUnavailable}}
	if !reflect.DeepEqual(lines, want) {
		t.Fatalf("lines=%+v want %+v", lines, want)
	}
}

// RSP-V0-063 positive: the clean twin through the host is the compiled-in
// runner's conformance result, one line per required runner, and an
// unrequired pin is never resolved or launched.
func TestRSPV0063CleanTwin(t *testing.T) {
	pin, store, _ := hostPins(t)
	unrequired := Pin{Name: "unrequired", Protocol: Protocol, Executable: "./corvint-runner-unrequired", SHA256: pin.SHA256, Runners: []string{"unrequired-runner"}}
	options := hostOptions(t, "reference", Limits{})
	h := OpenHost(context.Background(), HostInput{Required: []string{ReferenceRunner, ReferenceRunner, "go-test"}, Plan: DiscoveryPlan{Pins: []Pin{pin, unrequired}}, Store: store, Options: options})
	if h.Sessions() != 1 {
		t.Fatalf("sessions=%d, want 1 (no session for an unrequired pin)", h.Sessions())
	}
	plugin, err := h.Plugin(ReferenceRunner)
	if err != nil {
		t.Fatal(err)
	}
	c := corpus(t, options.RunDir)
	compiled, err := Conform(context.Background(), Reference{}, c, nil)
	if err != nil {
		t.Fatal(err)
	}
	var lines []Line
	process, err := Conform(context.Background(), plugin, c, func(ctx context.Context) error { lines = h.Close(ctx); return nil })
	if err != nil {
		t.Fatal(err)
	}
	if err = CompareConformance(compiled, process); err != nil {
		t.Fatal(err)
	}
	want := []Line{{Runner: ReferenceRunner, Mode: ModeFull, Reason: ReasonNotQualified, Run: RunOK}}
	if !reflect.DeepEqual(lines, want) {
		t.Fatalf("lines=%+v want %+v (compiled-in go-test is outside the host)", lines, want)
	}
	if again := h.Close(context.Background()); !reflect.DeepEqual(again, want) {
		t.Fatalf("second close changed lines: %+v", again)
	}
	if _, err = h.Plugin(ReferenceRunner); err == nil {
		t.Fatal("closed host served a plugin")
	}
}

// RSP-V0-063 negative: one fixture per RSP-V0-031 failure through the host,
// each FULL with its cause and the run FAIL runner-adapter-unavailable.
func TestRSPV0063FailureLines(t *testing.T) {
	short := Limits{Message: 4096, Hello: time.Second}
	for _, tc := range []struct{ mode, reason string }{
		{"crash", "crash"}, {"timeout", "timeout"}, {"oversize", "oversize-message"},
		{"protocol", "protocol-error"}, {"duplicate", "protocol-error"}, {"schema", "schema-violation"},
		{"error", "error-response"}, {"version", "version-mismatch"}, {"undeclared", "undeclared-runner"},
		{"untracked-mutation", "repository-mutation"}, {"late-mutation", "repository-mutation"},
	} {
		t.Run(tc.mode, func(t *testing.T) {
			pin, store, _ := hostPins(t)
			h := OpenHost(context.Background(), HostInput{Required: []string{ReferenceRunner}, Plan: DiscoveryPlan{Pins: []Pin{pin}}, Store: store, Options: hostOptions(t, tc.mode, short)})
			wantFail(t, h.Close(context.Background()), tc.reason)
		})
	}
	t.Run("schema-violation-mid-session", func(t *testing.T) {
		pin, store, _ := hostPins(t)
		h := OpenHost(context.Background(), HostInput{Required: []string{ReferenceRunner}, Plan: DiscoveryPlan{Pins: []Pin{pin}}, Store: store, Options: hostOptions(t, "wrong-response-runner", Limits{})})
		plugin, err := h.Plugin(ReferenceRunner)
		if err != nil {
			t.Fatal(err)
		}
		if _, err = plugin.Parse(context.Background(), registry.ParseRequest{Runner: ReferenceRunner, Report: "go.json"}); err == nil {
			t.Fatal("undeclared response runner accepted")
		}
		wantFail(t, h.Close(context.Background()), "schema-violation")
	})
	t.Run("digest-mismatch", func(t *testing.T) {
		pin, store, installed := hostPins(t)
		if err := os.Chmod(installed, 0700); err != nil {
			t.Fatal(err)
		}
		if err := execfile.WriteFile(installed, []byte("#!/bin/false\n"), 0700); err != nil {
			t.Fatal(err)
		}
		h := OpenHost(context.Background(), HostInput{Required: []string{ReferenceRunner}, Plan: DiscoveryPlan{Pins: []Pin{pin}}, Store: store, Options: hostOptions(t, "reference", Limits{})})
		if h.Sessions() != 0 {
			t.Fatal("launched a mismatched digest")
		}
		wantFail(t, h.Close(context.Background()), "digest-mismatch")
	})
	t.Run("missing-executable", func(t *testing.T) {
		pin, store, installed := hostPins(t)
		if err := os.Remove(installed); err != nil {
			t.Fatal(err)
		}
		h := OpenHost(context.Background(), HostInput{Required: []string{ReferenceRunner}, Plan: DiscoveryPlan{Pins: []Pin{pin}}, Store: store, Options: hostOptions(t, "reference", Limits{})})
		wantFail(t, h.Close(context.Background()), "missing-executable")
	})
	t.Run("not-installed", func(t *testing.T) {
		pin, _, _ := hostPins(t)
		h := OpenHost(context.Background(), HostInput{Required: []string{ReferenceRunner}, Plan: DiscoveryPlan{Pins: []Pin{pin}}, Store: Discovery{Schema: DiscoverySchema, Plugins: []Pin{}}, Options: hostOptions(t, "reference", Limits{})})
		wantFail(t, h.Close(context.Background()), "missing-executable")
	})
	t.Run("launch-failure", func(t *testing.T) {
		pin, store, installed := hostPins(t)
		bad := []byte("not executable format")
		if err := os.Chmod(installed, 0700); err != nil {
			t.Fatal(err)
		}
		if err := execfile.WriteFile(installed, bad, 0700); err != nil {
			t.Fatal(err)
		}
		pin.SHA256 = digestOf(bad)
		store.Plugins[0].SHA256 = pin.SHA256
		h := OpenHost(context.Background(), HostInput{Required: []string{ReferenceRunner}, Plan: DiscoveryPlan{Pins: []Pin{pin}}, Store: store, Options: hostOptions(t, "reference", Limits{})})
		wantFail(t, h.Close(context.Background()), "launch-failure")
	})
	t.Run("unpinned", func(t *testing.T) {
		_, store, _ := hostPins(t)
		h := OpenHost(context.Background(), HostInput{Required: []string{ReferenceRunner}, Plan: DiscoveryPlan{}, Store: store, Options: hostOptions(t, "reference", Limits{})})
		if h.Sessions() != 0 {
			t.Fatal("launched without a pin")
		}
		if _, err := h.Plugin(ReferenceRunner); err == nil {
			t.Fatal("unpinned runner served")
		}
		wantFail(t, h.Close(context.Background()), ReasonUnpinned)
	})
	t.Run("malformed-discovery", func(t *testing.T) {
		_, store, _ := hostPins(t)
		plan := PlanDiscovery([]byte(`{"schema":"corvint-runner-plugins/1","plugins":[],"extra":1}`), nil, false)
		h := OpenHost(context.Background(), HostInput{Required: []string{ReferenceRunner}, Plan: plan, Store: store, Options: hostOptions(t, "reference", Limits{})})
		lines := h.Close(context.Background())
		if len(lines) != 1 || lines[0].Run != RunFail || lines[0].RunReason != RunnerAdapterUnavailable || lines[0].Reason != plan.FullAll || plan.FullAll == "" {
			t.Fatalf("lines=%+v plan=%+v", lines, plan)
		}
	})
	t.Run("unsupported-schema", func(t *testing.T) {
		pin, store, _ := hostPins(t)
		plan := DiscoveryPlan{Pins: []Pin{pin}, Full: map[string]string{ReferenceRunner: "unsupported-schema"}}
		h := OpenHost(context.Background(), HostInput{Required: []string{ReferenceRunner}, Plan: plan, Store: store, Options: hostOptions(t, "reference", Limits{})})
		if h.Sessions() != 0 {
			t.Fatal("launched under an unsupported schema")
		}
		wantFail(t, h.Close(context.Background()), "unsupported-schema")
	})
	t.Run("pre-plugin-path", func(t *testing.T) {
		_, store, _ := hostPins(t)
		h := OpenHost(context.Background(), HostInput{Required: []string{ReferenceRunner}, Plan: DiscoveryPlan{}, Store: store, Options: hostOptions(t, "reference", Limits{}), CorePath: func(r string) bool { return r == ReferenceRunner }})
		want := []Line{{Runner: ReferenceRunner, Mode: ModeFull, Reason: ReasonUnpinned, Run: RunOK}}
		if lines := h.Close(context.Background()); !reflect.DeepEqual(lines, want) {
			t.Fatalf("lines=%+v want %+v", lines, want)
		}
	})
}

// RSP-V0-063 with RSP-V0-028: a candidate pin edit keeps the trusted pin's
// session, so the runner runs FULL rather than failing or narrowing.
func TestRSPV0063CandidatePinsChangedRunsTrustedPin(t *testing.T) {
	pin, store, _ := hostPins(t)
	trusted := []byte(`{"schema":"corvint-runner-plugins/1","plugins":[{"name":"reference","protocol":"corvint-runner/1","executable":"./corvint-runner-reference","sha256":"` + pin.SHA256 + `","runners":["` + ReferenceRunner + `"]}]}`)
	candidate := []byte(`{"schema":"corvint-runner-plugins/1","plugins":[]}`)
	plan := PlanDiscovery(trusted, candidate, true)
	h := OpenHost(context.Background(), HostInput{Required: []string{ReferenceRunner}, Plan: plan, Store: store, Options: hostOptions(t, "reference", Limits{})})
	if h.Sessions() != 1 {
		t.Fatalf("sessions=%d", h.Sessions())
	}
	want := []Line{{Runner: ReferenceRunner, Mode: ModeFull, Reason: "candidate-pins-changed", Run: RunOK}}
	if lines := h.Close(context.Background()); !reflect.DeepEqual(lines, want) {
		t.Fatalf("lines=%+v want %+v", lines, want)
	}
}

func digestOf(b []byte) string {
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}
