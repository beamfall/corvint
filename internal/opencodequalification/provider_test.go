package opencodequalification

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/procgroup"
)

// scriptedResponse posts one request body to the provider and returns the tool call it scripts
// (empty name when it answers with text) and whether the response carried compaction usage.
func scriptedResponse(t *testing.T, p *provider, body string) (name, arguments string, compact bool) {
	t.Helper()
	w := httptest.NewRecorder()
	p.ServeHTTP(w, httptest.NewRequest("POST", "/v1/chat/completions", strings.NewReader(body)))
	for _, line := range strings.Split(w.Body.String(), "\n") {
		data, ok := strings.CutPrefix(line, "data: ")
		if !ok || data == "[DONE]" {
			continue
		}
		chunk, e := decode([]byte(data))
		if e != nil {
			t.Fatalf("chunk %q: %v", data, e)
		}
		if _, ok := chunk["usage"]; ok {
			compact = true
		}
		for _, choice := range array(chunk["choices"]) {
			for _, call := range array(object(object(choice)["delta"])["tool_calls"]) {
				f := object(object(call)["function"])
				name, arguments = str(f["name"]), str(f["arguments"])
			}
		}
	}
	return name, arguments, compact
}

func TestAHI213ScriptedSourceQuerySuppliesExpansionHandle(t *testing.T) {
	const tree, blob = "a41cab2247b7e99d520b7f751a99e7865394e676", "f538972ef943fc087cef16a09d13c111fca7089d"
	sourceResult := `{"messages":[{"role":"tool","content":"BEGIN CORVINT REPOSITORY DATA {\"expansionHandles\":[\"cv1:` + tree + `:` + blob + `:all:add.go\"]}"}]}`
	t.Run("AHI-213 the source query result supplies the exact lines 1-3 handle", func(t *testing.T) {
		// GitHub #1900: since AHI-210 the hidden prompt frame carries no cv1 handle, so the
		// handle must come from a corvint_context result for the seeded source.
		p := &provider{root: t.TempDir(), ctx: context.Background()}
		want := []string{"corvint_context", "corvint_context", "execute", "edit", "execute", "execute", ""}
		for i, name := range want {
			body := `{"messages":[]}`
			if i >= 2 {
				body = sourceResult
			}
			got, args, compact := scriptedResponse(t, p, body)
			if got != name {
				t.Fatalf("request %d scripted %q, want %q", i, got, name)
			}
			switch i {
			case 0:
				if !strings.Contains(args, "required workflow gates") {
					t.Fatalf("governance query args = %s", args)
				}
			case 1:
				if !strings.Contains(args, `"task":"`+sourceTask+`"`) {
					t.Fatalf("source query args = %s", args)
				}
			case 2:
				if !strings.Contains(args, "cv1:"+tree+":"+blob+":1-3:add.go") || strings.Contains(args, "missing-supplied-handle") {
					t.Fatalf("expand args = %s", args)
				}
			}
			if compact != (i == 5) {
				t.Fatalf("request %d compaction usage = %v", i, compact)
			}
		}
		if _, e := os.Stat(filepath.Join(p.root, "provider-faults.jsonl")); !os.IsNotExist(e) {
			t.Fatalf("supplied handle recorded a provider fault: %v", e)
		}
	})
	t.Run("AHI-213 a missing handle is a typed fault, never a placeholder expansion", func(t *testing.T) {
		p := &provider{root: t.TempDir(), ctx: context.Background()}
		want := []string{"corvint_context", "corvint_context", "edit", "execute", "execute", ""}
		for i, name := range want {
			got, args, compact := scriptedResponse(t, p, `{"messages":[{"role":"tool","content":"{\"expansionHandles\":[]}"}]}`)
			if got != name || strings.Contains(args, "corvint_expand") {
				t.Fatalf("request %d scripted %q %s, want %q", i, got, args, name)
			}
			if compact != (i == 4) {
				t.Fatalf("request %d compaction usage = %v", i, compact)
			}
		}
		faults, e := readRows(filepath.Join(p.root, "provider-faults.jsonl"))
		if e != nil || len(faults) != 1 || faults[0]["check"] != "native-exact-expansion" || faults[0]["reason"] != reasonNoExpansionHandle {
			t.Fatalf("provider faults = %v, %v", faults, e)
		}
		failures := checkFailures(Object{"native-exact-expansion": false, "native-query": true}, faults, nil)
		if len(failures) != 1 || failures[0]["reason"] != reasonNoExpansionHandle || !strings.Contains(str(failures[0]["detail"]), "corvint_context") {
			t.Fatalf("failures = %v", failures)
		}
	})
	t.Run("AHI-213 benchmark and smoke scripts are unchanged", func(t *testing.T) {
		bench := &provider{root: t.TempDir(), ctx: context.Background(), benchmark: true}
		if got, args, _ := scriptedResponse(t, bench, `{}`); got != "execute" || !strings.Contains(args, "qualification_noop") {
			t.Fatalf("benchmark scripted %q %s", got, args)
		}
		smoke := &provider{root: t.TempDir(), ctx: context.Background(), smoke: true}
		if got, _, _ := scriptedResponse(t, smoke, `{}`); got != "" {
			t.Fatalf("smoke scripted %q", got)
		}
	})
}

func TestAHI213ObservationFailuresEncodeEmptyArray(t *testing.T) {
	t.Run("AHI-213 an observation retained with null failures encodes [] in sidecar and summary", func(t *testing.T) {
		var d procgroup.DescendantObservation
		if e := json.Unmarshal([]byte(`{"scope":"observed-pid-start-identities","interval_ms":20,"absent":true,"processes":[],"failures":null,"limitations":[]}`), &d); e != nil {
			t.Fatalf("retained evidence with null failures no longer decodes: %v", e)
		}
		path := filepath.Join(t.TempDir(), "probe.observation.json")
		summary, e := retainObservation(path, &d)
		if e != nil {
			t.Fatal(e)
		}
		encoded, e := jsonBytes(summary)
		if e != nil || !bytes.Contains(encoded, []byte(`"failures":[]`)) {
			t.Fatalf("summary %s, %v", encoded, e)
		}
		sidecar, e := os.ReadFile(path)
		if e != nil || !bytes.Contains(sidecar, []byte(`"failures": []`)) {
			t.Fatalf("sidecar %s, %v", sidecar, e)
		}
		if d.Failures != nil {
			t.Fatalf("caller's observation was mutated: %v", d.Failures)
		}
	})
	t.Run("AHI-213 recorded failures are kept", func(t *testing.T) {
		d := procgroup.DescendantObservation{Absent: true, Failures: []string{"snapshot failed"}}
		summary, e := retainObservation(filepath.Join(t.TempDir(), "o.json"), &d)
		if f, ok := summary["failures"].([]string); e != nil || !ok || len(f) != 1 || f[0] != "snapshot failed" {
			t.Fatalf("summary %v, %v", summary, e)
		}
	})
}

func TestAHI213EveryFalseCheckNamesAReason(t *testing.T) {
	// The complete native check set (integrations/opencode/evidence.test.mjs MANDATORY_NATIVE_CHECKS).
	mandatory := []string{
		"advisory-completion", "awaited-current-prompt", "bounded-prompt", "bounded-stops", "critical-recall-and-bytes",
		"direct-install", "dirty-path-recovery", "environment-filter", "explicit-outcome", "frozen-identities", "governance",
		"interruption-cleanup", "latency", "native-bundled-skill", "native-compaction", "native-discovery", "native-edit",
		"native-exact-expansion", "native-exit", "native-host-image", "native-query", "oversized-exclusion", "snapshot",
		"supplied-evidence", "timed-delivery", "uninstall", "verification-observation",
	}
	t.Run("AHI-213 every mandatory check has a typed reason", func(t *testing.T) {
		checks := Object{}
		for _, name := range mandatory {
			checks[name] = false
			if _, ok := checkFailureReasons[name]; !ok && name != "latency" {
				t.Errorf("no failure reason for %s", name)
			}
		}
		slow, _ := timingSample(latencyRows(series(30, 30, 0, 612), series(30, 0, 100, 0)))
		failures := checkFailures(checks, nil, []latencySample{slow})
		if len(failures) != len(mandatory) {
			t.Fatalf("failures = %d, want %d", len(failures), len(mandatory))
		}
		for i, f := range failures {
			if f["check"] != mandatory[i] || str(f["reason"]) == "" || str(f["detail"]) == "" {
				t.Fatalf("failure %d = %v", i, f)
			}
		}
		if f := failures[12]; f["reason"] != latencyOutcomeRegressed || f["detail"] != "attempt 1: query p95 612ms > 500ms" {
			t.Fatalf("latency failure = %v", f)
		}
	})
	t.Run("AHI-213 a passing report lists no failures as an empty array", func(t *testing.T) {
		got := checkFailures(Object{"native-query": true}, nil, nil)
		b, e := jsonBytes(Object{"failures": got})
		if e != nil || !bytes.Equal(bytes.TrimSpace(b), []byte(`{"failures":[]}`)) {
			t.Fatalf("encoded %s, %v", b, e)
		}
	})
	t.Run("AHI-213 an unmapped false check still names a reason", func(t *testing.T) {
		got := checkFailures(Object{"future-check": false}, nil, nil)
		if len(got) != 1 || got[0]["reason"] != "check-failed" {
			t.Fatalf("failures = %v", got)
		}
	})
}

func TestAHI218CampaignPromptEmitsAFrame(t *testing.T) {
	t.Run("AHI-218 the campaign prompt names no fixture path, so the AHI-215 hook does not withhold its frame", func(t *testing.T) {
		for _, path := range []string{"add.go", "AGENTS.md", "go.mod", "oversized.txt"} {
			if strings.Contains(framePrompt, path) {
				t.Fatalf("framePrompt %q names fixture path %s", framePrompt, path)
			}
		}
		if framePrompt == sourceTask || !strings.Contains(sourceTask, "add.go") {
			t.Fatalf("framePrompt %q and sourceTask %q must split orientation from source recall", framePrompt, sourceTask)
		}
	})
	t.Run("AHI-218 the probe prompts and the script queries with the frame prompt first", func(t *testing.T) {
		quoted, _ := jsonBytes(framePrompt)
		probe := template(probeTemplate, map[string]string{"EVENTS": "/e", "VERIFY": "v", "FRAME_PROMPT": framePrompt})
		if !strings.Contains(probe, "text:"+string(quoted)) || strings.Contains(probe, "FRAME_PROMPT") || strings.Contains(probe, sourceTask) {
			t.Fatalf("probe prompt not rendered from framePrompt:\n%s", probe)
		}
		calls := scriptedCalls("h", false)
		if calls[0].args["task"] != framePrompt || calls[1].args["task"] != sourceTask {
			t.Fatalf("scripted queries = %v, %v", calls[0].args, calls[1].args)
		}
	})
	t.Run("AHI-218 a run with no latency samples still encodes its report", func(t *testing.T) {
		// The V1-2804 live run lost report.json to "json: unsupported value: +Inf" when every
		// execute call was refused and no prompt was timed.
		empty := latencySample{query: []float64{}, lifecycle: []float64{}}
		evidence := Object{"queryP95Ms": p95Evidence(empty.query), "attempts": latencyAttemptEvidence([]latencySample{empty})}
		b, e := jsonBytes(evidence)
		if e != nil || !strings.Contains(string(b), `"queryP95Ms":"NOT_OBSERVED"`) {
			t.Fatalf("report = %s, %v", b, e)
		}
		if p95Evidence([]float64{3, 1, 2}) != float64(3) {
			t.Fatalf("p95Evidence of samples = %v", p95Evidence([]float64{3, 1, 2}))
		}
	})
}
