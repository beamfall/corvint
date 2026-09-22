// SPDX-License-Identifier: AGPL-3.0-or-later
package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

func jsonLines(t *testing.T, events ...object) []byte {
	t.Helper()
	var raw []byte
	for _, event := range events {
		b, err := json.Marshal(event)
		if err != nil {
			t.Fatal(err)
		}
		raw = append(raw, b...)
		raw = append(raw, '\n')
	}
	return raw
}
func codexEvent(mode string, values ...any) object {
	usage := object{}
	for i, field := range codexFields {
		usage[field] = values[i]
	}
	return object{"type": "token_count", mode: usage}
}
func assertMetric(t *testing.T, metrics object, key string, want any) {
	t.Helper()
	if fmt.Sprint(metrics[key]) != fmt.Sprint(want) {
		t.Errorf("%s=%v, want %v", key, metrics[key], want)
	}
}

// BRAIN-DOG-013: a cumulative snapshot may subsume a delta, but a corrupt
// snapshot or an ambiguous tail cannot become a discount.
func TestCodexCumulativeAccounting(t *testing.T) {
	first := codexEvent("total_token_usage", 100, 10, 20)
	delta := codexEvent("last_token_usage", 5, 1, 2)
	large := codexEvent("usage", 1000, 100, 200)
	later := codexEvent("total_token_usage", 110, 12, 24)
	decrease := codexEvent("total_token_usage", 90, 10, 20)
	for _, test := range []struct {
		name   string
		events []object
		want   [3]any
	}{
		{"equal", []object{first, first}, [3]any{100, 10, 20}},
		{"monotonic", []object{first, later}, [3]any{110, 12, 24}},
		{"delta before snapshot", []object{delta, first}, [3]any{100, 10, 20}},
		{"duplicate-valued deltas", []object{delta, delta}, [3]any{10, 2, 4}},
		{"snapshot supersedes magnitude", []object{large, first}, [3]any{100, 10, 20}},
		{"ambiguous tail", []object{first, delta}, [3]any{unknown, unknown, unknown}},
		{"later snapshot", []object{first, delta, later}, [3]any{110, 12, 24}},
		{"equal subsumes tail", []object{first, delta, first}, [3]any{100, 10, 20}},
		{"only snapshots compared", []object{first, large, later}, [3]any{110, 12, 24}},
		{"decrease", []object{first, decrease}, [3]any{unknown, 10, 20}},
		{"decrease cannot recover", []object{first, decrease, later}, [3]any{unknown, 12, 24}},
		{"reset", []object{first, codexEvent("total_token_usage", 0, 0, 0)}, [3]any{unknown, unknown, unknown}},
		{"zero observed", []object{codexEvent("total_token_usage", 0, 0, 0)}, [3]any{0, 0, 0}},
		{"snapshot supersedes invalid delta", []object{codexEvent("last_token_usage", -1, 1, 1), first}, [3]any{100, 10, 20}},
	} {
		t.Run(test.name, func(t *testing.T) {
			got := parseCodex(jsonLines(t, test.events...))
			for i, key := range []string{"inputTokens", "cacheReadTokens", "outputTokens"} {
				assertMetric(t, got, key, test.want[i])
			}
		})
	}
	for _, mode := range []string{"total_token_usage", "last_token_usage", "usage"} {
		for field, metric := range map[string]string{"input_tokens": "inputTokens", "cached_input_tokens": "cacheReadTokens", "output_tokens": "outputTokens"} {
			for _, invalid := range []any{-3, true, "3", nil, 1.5} {
				initial, bad := codexEvent(mode, 10, 10, 10), codexEvent(mode, 14, 14, 14)
				asObject(bad[mode])[field] = invalid
				got := parseCodex(jsonLines(t, initial, bad))
				assertMetric(t, got, metric, unknown)
				for other, otherMetric := range map[string]string{"input_tokens": "inputTokens", "cached_input_tokens": "cacheReadTokens", "output_tokens": "outputTokens"} {
					if other != field {
						want := 24
						if mode == "total_token_usage" {
							want = 14
						}
						assertMetric(t, got, otherMetric, want)
					}
				}
				assertMetric(t, parseCodex(jsonLines(t, bad)), metric, unknown)
			}
		}
	}
	for _, invalid := range []any{-1, true, "100", nil} {
		bad := codexEvent("total_token_usage", invalid, 10, 20)
		got := parseCodex(jsonLines(t, first, bad, later))
		assertMetric(t, got, "inputTokens", unknown)
		assertMetric(t, got, "cacheReadTokens", 12)
	}
}
func manifestFixture(workers ...any) object {
	return object{"profile": "corvint-dogfood-workers-manifest/0", "task": "fixture", "pricingVersion": unknown, "workers": workers}
}
func workerFixture(id string) object {
	return object{"id": id, "role": "builder", "host": "claude-code", "status": "completed", "model": "fixture", "transcript": nil, "solved": false}
}
func writeManifest(t *testing.T, document object) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "manifest.json")
	if err := os.WriteFile(p, jsonLines(t, document), 0600); err != nil {
		t.Fatal(err)
	}
	return p
}
func receiptFixture(t *testing.T, document object) object {
	t.Helper()
	m, raw, err := loadManifest(writeManifest(t, document))
	if err != nil {
		t.Fatal(err)
	}
	r, err := buildReceipt(m, raw)
	if err != nil {
		t.Fatal(err)
	}
	return r
}
func TestManifestRefusalsAndWorkerBounds(t *testing.T) {
	for _, change := range []func(object){
		func(w object) { w["host"] = "unknown" }, func(w object) { w["ticket"] = 12 }, func(w object) { w["role"] = "unknown" }, func(w object) { w["solved"] = 0 }, func(w object) { w["extra"] = true }, func(w object) { w["id"] = "" }, func(w object) { w["model"] = nil },
	} {
		w := workerFixture("one")
		change(w)
		if _, _, err := loadManifest(writeManifest(t, manifestFixture(w))); err == nil {
			t.Fatalf("invalid worker accepted: %#v", w)
		}
	}
	for _, count := range []int{0, 1, 256, 257} {
		workers := []any{}
		for i := range count {
			workers = append(workers, workerFixture(fmt.Sprint(i)))
		}
		_, _, err := loadManifest(writeManifest(t, manifestFixture(workers...)))
		if (err == nil) != (count >= 1 && count <= 256) {
			t.Fatalf("count %d error %v", count, err)
		}
	}
	a, b := workerFixture("one"), workerFixture("two")
	a["transcript"] = filepath.Join(t.TempDir(), "shared")
	b["transcript"] = a["transcript"]
	if _, _, err := loadManifest(writeManifest(t, manifestFixture(a, b))); err == nil {
		t.Fatal("double transcript accepted")
	}
	directory := t.TempDir()
	target := filepath.Join(directory, "real")
	if err := os.WriteFile(target, nil, 0600); err != nil {
		t.Fatal(err)
	}
	alias := filepath.Join(directory, "alias")
	if err := os.Symlink(target, alias); err != nil {
		t.Fatal(err)
	}
	a["transcript"], b["transcript"] = target, alias
	if _, _, err := loadManifest(writeManifest(t, manifestFixture(a, b))); err == nil {
		t.Fatal("aliased transcript accepted")
	}
	path := writeManifest(t, manifestFixture(workerFixture("one")))
	raw, _ := os.ReadFile(path)
	raw = bytes.Replace(raw, []byte(`"task":"fixture"`), []byte(`"task":"fixture","task":"duplicate"`), 1)
	if err := os.WriteFile(path, raw, 0600); err != nil {
		t.Fatal(err)
	}
	if _, _, err := loadManifest(path); err == nil {
		t.Fatal("duplicate key accepted")
	}
}
func TestMissingUsageAndTicketPreservation(t *testing.T) {
	for _, host := range []string{"claude-code", "codex", "atm"} {
		w := workerFixture("one")
		w["host"] = host
		w["ticket"] = "AT-01"
		r := receiptFixture(t, manifestFixture(w))
		entry := asObject(r["workers"].([]any)[0])
		totals := asObject(r["totals"])
		for _, field := range metricFields {
			assertMetric(t, entry, field, unknown)
			assertMetric(t, totals, field, unknown)
		}
		assertMetric(t, totals, "observedWorkers", 0)
		assertMetric(t, totals, "solvedWorkers", 0)
		if entry["ticket"] != "AT-01" || r["pricingVersion"] != unknown || r["cost"] != unknown {
			t.Fatal(r)
		}
	}
	r := receiptFixture(t, manifestFixture(workerFixture("one")))
	if _, exists := asObject(r["workers"].([]any)[0])["ticket"]; exists {
		t.Fatal("absent ticket emitted")
	}
}
func TestFailedAndCancelledRemainInTotalsAndStableBytes(t *testing.T) {
	workers := []any{}
	for i, status := range []string{"failed", "cancelled"} {
		w := workerFixture(status)
		w["host"], w["status"] = "codex", status
		p := filepath.Join(t.TempDir(), "transcript.jsonl")
		count := 12 - i*4
		if err := os.WriteFile(p, jsonLines(t, codexEvent("total_token_usage", count, 0, count)), 0600); err != nil {
			t.Fatal(err)
		}
		w["transcript"] = p
		workers = append(workers, w)
	}
	r := receiptFixture(t, manifestFixture(workers...))
	totals := asObject(r["totals"])
	for key, want := range map[string]int{"inputTokens": 20, "outputTokens": 20, "cacheReadTokens": 0, "failedWorkers": 1, "cancelledWorkers": 1, "solvedWorkers": 0} {
		assertMetric(t, totals, key, want)
	}
	p := filepath.Join(t.TempDir(), "receipt.json")
	if err := writeReceipt(p, r); err != nil {
		t.Fatal(err)
	}
	first, _ := os.ReadFile(p)
	if err := writeReceipt(p, r); err != nil {
		t.Fatal(err)
	}
	second, _ := os.ReadFile(p)
	if !bytes.Equal(first, second) {
		t.Fatal("receipt bytes drifted")
	}
	info, _ := os.Stat(p)
	if info.Mode().Perm() != 0600 {
		t.Fatal("receipt is not private")
	}
}
func TestClaudeToolDedupeAndAbsentRecords(t *testing.T) {
	block := object{"type": "tool_use", "id": "same", "name": "Read", "input": object{}}
	event := object{"type": "assistant", "requestId": "one", "message": object{"usage": object{"input_tokens": 7, "cache_creation_input_tokens": 1, "cache_read_input_tokens": 2, "output_tokens": 3}, "content": []any{block}}}
	m := parseClaude(jsonLines(t, event, event))
	assertMetric(t, m, "sourceOpens", 1)
	assertMetric(t, m, "turns", 1)
	assertMetric(t, m, "inputTokens", 7)
	absent := parseClaude([]byte("bad\n{}\n"))
	assertMetric(t, absent, "turns", unknown)
	assertMetric(t, absent, "inputTokens", unknown)
	assertMetric(t, absent, "unparsedLines", 1)
}
