// Copyright 2026 Russell Lewis
// Licensed under the Apache License, Version 2.0.

package main

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// The producer runs in a separate process; this verifier never imports it or
// recomposes its bytes. These are synthetic deadline composition witnesses,
// not evidence that Execute's 30-minute process deadline elapsed.
func TestProducerDeadlineCompositionRoundtrip(t *testing.T) {
	directory := t.TempDir()
	command := exec.Command("go", "test", "-count=1", "-run", "^TestDeadlineReceiptComposition$", "./internal/liveverify/provider")
	command.Dir = filepath.Clean(filepath.Join(sourceDir(t), "..", ".."))
	command.Env = append(os.Environ(), "GOTOOLCHAIN=local", "CORVINT_DEADLINE_FIXTURES="+directory)
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("produce deadline fixtures: %v: %s", err, output)
	}
	contextRaw, err := os.ReadFile(filepath.Join(sourceDir(t), "testdata", "producer-context.json"))
	if err != nil {
		t.Fatal(err)
	}
	var context transcriptContext
	if err := json.Unmarshal(contextRaw, &context); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"all-passed", "incomplete-stream", "nonzero-no-failed-terminal", "provider-failure", "cancellation", "source-drift", "toolchain-drift", "failed-terminals"} {
		t.Run(name, func(t *testing.T) {
			raw, err := os.ReadFile(filepath.Join(directory, name+".jsonl"))
			if err != nil {
				t.Fatal(err)
			}
			if err := VerifyTranscript(raw, context); err != nil {
				t.Fatalf("independent deadline verification: %v", err)
			}
		})
	}
}

func TestProducerCancellationProviderFailureRoundtrip(t *testing.T) {
	directory := t.TempDir()
	fixture := filepath.Join(directory, "cancelled-provider-failure.jsonl")
	command := exec.Command("go", "test", "-count=1", "-run", "^TestCancellationProviderFailureReceiptComposition$", "./internal/liveverify/provider")
	command.Dir = filepath.Clean(filepath.Join(sourceDir(t), "..", ".."))
	command.Env = append(os.Environ(), "GOTOOLCHAIN=local", "CORVINT_CANCELLATION_PROVIDER_FAILURE_FIXTURE="+fixture)
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("produce cancellation/provider-failure fixture: %v: %s", err, output)
	}
	raw, err := os.ReadFile(fixture)
	if err != nil {
		t.Fatal(err)
	}
	contextRaw, err := os.ReadFile(filepath.Join(sourceDir(t), "testdata", "producer-context.json"))
	if err != nil {
		t.Fatal(err)
	}
	var context transcriptContext
	if err := json.Unmarshal(contextRaw, &context); err != nil {
		t.Fatal(err)
	}
	if err := VerifyTranscript(raw, context); err != nil {
		t.Fatalf("independent cancellation/provider-failure verification: %v", err)
	}
}

func TestProducerCancellationDrainOutputLimitRoundtrip(t *testing.T) {
	directory := t.TempDir()
	fixture := filepath.Join(directory, "cancelled-output-limit.jsonl")
	command := exec.Command("go", "test", "-count=1", "-run", "^TestCancellationDrainOutputLimitReceiptComposition$", "./internal/liveverify/provider")
	command.Dir = filepath.Clean(filepath.Join(sourceDir(t), "..", ".."))
	command.Env = append(os.Environ(), "GOTOOLCHAIN=local", "CORVINT_CANCELLATION_DRAIN_OUTPUT_LIMIT_FIXTURE="+fixture)
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("produce cancellation/output-limit fixture: %v: %s", err, output)
	}
	raw, err := os.ReadFile(fixture)
	if err != nil {
		t.Fatal(err)
	}
	contextRaw, err := os.ReadFile(filepath.Join(sourceDir(t), "testdata", "producer-context.json"))
	if err != nil {
		t.Fatal(err)
	}
	var context transcriptContext
	if err := json.Unmarshal(contextRaw, &context); err != nil {
		t.Fatal(err)
	}
	if err := VerifyTranscript(raw, context); err != nil {
		t.Fatalf("independent cancellation/output-limit verification: %v", err)
	}
}

func TestProducerReceiptDominanceRoundtrips(t *testing.T) {
	directory := t.TempDir()
	command := exec.Command("go", "test", "-count=1", "-run", "^(TestReceiptCancellationAndUnknownPostIdentity|TestReceiptStaleDominatesCancellationAndLimit)$", "./internal/liveverify/provider")
	command.Dir = filepath.Clean(filepath.Join(sourceDir(t), "..", ".."))
	command.Env = append(os.Environ(), "GOTOOLCHAIN=local", "CORVINT_RECEIPT_DOMINANCE_FIXTURES="+directory)
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("produce receipt-dominance fixtures: %v: %s", err, output)
	}
	contextRaw, err := os.ReadFile(filepath.Join(sourceDir(t), "testdata", "producer-context.json"))
	if err != nil {
		t.Fatal(err)
	}
	var context transcriptContext
	if err := json.Unmarshal(contextRaw, &context); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"current-cancellation", "unknown-identity-cancellation", "stale-cancellation-output-limit"} {
		t.Run(name, func(t *testing.T) {
			raw, err := os.ReadFile(filepath.Join(directory, name+".jsonl"))
			if err != nil {
				t.Fatal(err)
			}
			if err := VerifyTranscript(raw, context); err != nil {
				t.Fatalf("independent receipt-dominance verification: %v", err)
			}
		})
	}
}
