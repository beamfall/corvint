package benchmarks_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestDogfoodWorkersMissingUsageCLI(t *testing.T) {
	t.Run("BRAIN-DOG-013 observed requests without usage remain visible through receipt CLI", func(t *testing.T) {
		_, source, _, ok := runtime.Caller(0)
		if !ok {
			t.Fatal("source root unavailable")
		}
		root := filepath.Dir(filepath.Dir(source))
		dir := workerFixtureDirectory(t)
		binary := buildWorkerCLI(t, root)
		env := []string{"PATH=" + os.Getenv("PATH"), "LC_ALL=C", "TMPDIR=" + dir}
		t.Logf("native-cli-sha256=%s test-sha256=%s", hashFile(t, binary), hashFile(t, source))

		missing := func(id string) map[string]any {
			return map[string]any{"type": "assistant", "requestId": id, "message": map[string]any{"content": []any{}}}
		}
		fixtures := []struct {
			id     string
			events []map[string]any
		}{
			{"missing-request", []map[string]any{usageEvent("A", false, usage(10)), missing("B")}},
			{"mixed-missing", []map[string]any{missing("M"), usageEvent("M", true, usage(10))}},
		}
		workers := []map[string]any{}
		hashes := map[string]string{}
		for _, fixture := range fixtures {
			raw := []byte{}
			for _, event := range fixture.events {
				raw = append(raw, jsonBytes(t, event)...)
				raw = append(raw, '\n')
			}
			path := filepath.Join(dir, fixture.id+".jsonl")
			writeFixture(t, path, raw)
			hashes[fixture.id] = digest(raw)
			workers = append(workers, map[string]any{"id": fixture.id, "role": "builder", "host": "claude-code", "status": "completed", "model": "synthetic-fixture", "transcript": path, "solved": "NOT_OBSERVED"})
		}
		manifest := jsonBytes(t, map[string]any{"profile": "corvint-dogfood-workers-manifest/0", "task": "synthetic missing usage CLI boundary", "pricingVersion": "NOT_OBSERVED", "workers": workers})
		manifestPath, outputPath := filepath.Join(dir, "missing-usage-manifest.json"), filepath.Join(dir, "missing-usage-receipt.json")
		writeFixture(t, manifestPath, manifest)
		t.Logf("manifest-sha256=%s transcript-sha256=%s", digest(manifest), jsonBytes(t, hashes))
		if err := os.Remove(outputPath); err != nil && !os.IsNotExist(err) {
			t.Fatal(err)
		}
		stdout := runWorkerCLI(t, root, env, binary, "--manifest", manifestPath, "--output", outputPath)
		if len(stdout) != 0 {
			t.Fatalf("unexpected CLI stdout: %q", stdout)
		}
		raw, err := os.ReadFile(outputPath)
		if err != nil {
			t.Fatal(err)
		}
		var receipt struct {
			Profile        string           `json:"profile"`
			Task           string           `json:"task"`
			PricingVersion string           `json:"pricingVersion"`
			ManifestSHA256 string           `json:"manifestSha256"`
			Workers        []map[string]any `json:"workers"`
			Totals         map[string]any   `json:"totals"`
			Cost           any              `json:"cost"`
		}
		if err := json.Unmarshal(raw, &receipt); err != nil {
			t.Fatal(err)
		}
		if receipt.Profile != "corvint-dogfood-workers/0" || receipt.Task != "synthetic missing usage CLI boundary" || receipt.PricingVersion != "NOT_OBSERVED" || receipt.ManifestSHA256 != digest(manifest) || receipt.Cost != "NOT_OBSERVED" {
			t.Fatalf("receipt identity or cost mismatch: %s", raw)
		}
		if len(receipt.Workers) != 2 {
			t.Fatalf("worker count=%d", len(receipt.Workers))
		}
		for index, worker := range receipt.Workers {
			id := fixtures[index].id
			if worker["id"] != id || worker["transcriptSha256"] != hashes[id] {
				t.Fatalf("worker identity/hash mismatch: %v", worker)
			}
			turns := float64(2 - index)
			if worker["turns"] != turns {
				t.Errorf("%s turns=%v, want %v", id, worker["turns"], turns)
			}
			for _, field := range receiptFields {
				if worker[field] != "NOT_OBSERVED" {
					t.Errorf("%s %s=%v, want NOT_OBSERVED", id, field, worker[field])
				}
			}
		}
		side, ok := receipt.Workers[1]["sidechain"].(map[string]any)
		if !ok {
			t.Error("mixed main with missing usage was replaced by its observed sidechain")
		} else {
			checkMetrics(t, "mixed-missing sidechain", side, 10, 1, -1)
		}
		for _, field := range receiptFields {
			if receipt.Totals[field] != "NOT_OBSERVED" {
				t.Errorf("totals %s=%v, want NOT_OBSERVED", field, receipt.Totals[field])
			}
		}
		if receipt.Totals["turns"] != float64(3) {
			t.Errorf("total turns=%v, want 3", receipt.Totals["turns"])
		}
	})
}
