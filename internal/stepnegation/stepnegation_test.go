package stepnegation

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/testvalidity"
)

// Synthetic fixtures only: a made-up test identity and a loopback origin.
var (
	testKey   = TestKey{File: "e2e/cart.spec.cjs", FullTitle: "cart > total", Project: "chromium"}
	appOrigin = "http://127.0.0.1:4000"
)

func text(value string) *string { return &value }

// traceArchive writes a minimal Playwright 1.63-shaped trace.zip: the
// test-runner events in test.trace, library Frame.expect calls in
// 0-trace.trace, and resource snapshots plus bodies for the network log.
type traceArchive struct {
	events, frames, network []map[string]any
	resources               map[string][]byte
}

func (a *traceArchive) before(callID, parentID, method, title string, line int, start float64) {
	a.events = append(a.events, map[string]any{"type": "before", "callId": callID, "parentId": parentID, "class": "Test", "method": method, "title": title, "startTime": start,
		"stack": []map[string]any{{"file": "/repo/e2e/cart.spec.cjs", "line": line, "column": 5}}})
}

func (a *traceArchive) after(callID, message string) {
	event := map[string]any{"type": "after", "callId": callID}
	if message != "" {
		event["error"] = map[string]any{"message": message}
	}
	a.events = append(a.events, event)
}

func (a *traceArchive) frame(stepID, selector, expression string, expected []ExpectedText, start float64) {
	a.frames = append(a.frames, map[string]any{"type": "before", "class": "Frame", "method": "expect", "stepId": stepID, "startTime": start,
		"params": map[string]any{"selector": selector, "expression": expression, "expectedText": expected, "isNot": false}})
}

func (a *traceArchive) response(method, url, mime string, body []byte, start, duration float64) {
	name := "resources/" + digestHex(body)
	if a.resources == nil {
		a.resources = map[string][]byte{}
	}
	a.resources[name] = body
	a.network = append(a.network, map[string]any{"type": "resource-snapshot", "snapshot": map[string]any{
		"request": map[string]any{"method": method, "url": url}, "response": map[string]any{"content": map[string]any{"mimeType": mime, "_file": name}},
		"_monotonicTime": start, "time": duration}})
}

func (a *traceArchive) bytes(t *testing.T) []byte {
	t.Helper()
	var buffer bytes.Buffer
	writer := zip.NewWriter(&buffer)
	write := func(name string, data []byte) {
		entry, err := writer.Create(name)
		if err == nil {
			_, err = entry.Write(data)
		}
		if err != nil {
			t.Fatal(err)
		}
	}
	lines := func(events []map[string]any) []byte {
		var out []byte
		for _, event := range events {
			data, err := json.Marshal(event)
			if err != nil {
				t.Fatal(err)
			}
			out = append(append(out, data...), '\n')
		}
		return out
	}
	write("test.trace", lines(a.events))
	write("0-trace.trace", lines(a.frames))
	write("0-trace.network", lines(a.network))
	for name, data := range a.resources {
		write(name, data)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	return buffer.Bytes()
}

// baselineArchive is one passing test: step "open" with no assertion, step
// "total" whose toHaveText reads a value fetched from the application, and
// step "title" whose text exists only in the DOM.
func baselineArchive() *traceArchive {
	a := &traceArchive{}
	a.response("GET", appOrigin+"/", "text/html", []byte("<div id=total></div>"), 1, 2)
	a.response("GET", appOrigin+"/api/total?x=1", "application/json", []byte(`{"total":"42 apples"}`), 10, 5)
	a.before("s1", "", "test.step", "open", 3, 5)
	a.before("p1", "s1", "pw:api", "page.goto", 4, 6)
	a.after("p1", "")
	a.after("s1", "")
	a.before("s2", "", "test.step", "total", 6, 50)
	a.before("e1", "s2", "expect", `Expect "toHaveText"`, 7, 100)
	a.frame("e1", "#total", "to.have.text", []ExpectedText{{String: text("42 apples")}}, 100)
	a.after("e1", "")
	a.after("s2", "")
	a.before("s3", "", "test.step", "title", 9, 150)
	a.before("e2", "s3", "expect", `Expect "soft toHaveText"`, 10, 160)
	a.frame("e2", "h1", "to.have.text", []ExpectedText{{String: text("Static Title"), NormalizeWhiteSpace: true}}, 160)
	a.after("e2", "")
	a.after("s3", "")
	a.before("e3", "", "expect", `Expect "toBeVisible"`, 12, 170)
	a.frame("e3", "#footer", "to.be.visible", nil, 170)
	a.after("e3", "")
	return a
}

func parsed(t *testing.T, a *traceArchive) *Trace {
	t.Helper()
	trace, err := ParseTrace(a.bytes(t), "/repo")
	if err != nil {
		t.Fatal(err)
	}
	return trace
}

// LPCV-V0-059, LPCV-V0-065: the trace parser recovers the pre-order step
// tree, assertion ownership, locator calls and response bodies, and the
// inventory counts assertions outside every step.
func TestParseTraceRecoversStepTreeAndInventory(t *testing.T) {
	trace := parsed(t, baselineArchive())
	if len(trace.Steps) != 3 || trace.Steps[1].Title != "total" || trace.Steps[1].Ordinal != 2 || trace.Steps[1].Location != "e2e/cart.spec.cjs:6:5" {
		t.Fatalf("steps = %+v", trace.Steps)
	}
	assertions := trace.StepAssertions(2)
	if len(assertions) != 1 || assertions[0].Matcher != "toHaveText" || assertions[0].Frame == nil || assertions[0].Frame.Selector != "#total" || assertions[0].Frame.Ordinal != 1 {
		t.Fatalf("total assertions = %+v", assertions)
	}
	if soft := trace.StepAssertions(3); len(soft) != 1 || !soft[0].Soft {
		t.Fatalf("title assertions = %+v", soft)
	}
	if len(trace.Responses) != 2 || string(trace.Responses[1].Body) != `{"total":"42 apples"}` || trace.Responses[1].Completed != 15 {
		t.Fatalf("responses = %+v", trace.Responses)
	}
	want := Inventory{Steps: []InventoryStep{{"open", 1, 0}, {"total", 2, 1}, {"title", 3, 1}}, AssertionsOutsideSteps: 1}
	if got := trace.Inventory(); !reflect.DeepEqual(got, want) {
		t.Fatalf("inventory = %+v, want %+v", got, want)
	}
	if _, err := ParseTrace([]byte("not a zip"), "/repo"); err == nil {
		t.Fatal("a non-archive parsed")
	}
}

// LPCV-V0-061, LPCV-V0-062: derivation reasons for each closed case.
func TestDeriveNetworkAndDOMReasons(t *testing.T) {
	derive := func(t *testing.T, mutate func(*traceArchive)) Derivation {
		a := baselineArchive()
		if mutate != nil {
			mutate(a)
		}
		return Derive(parsed(t, a), testKey, 2, appOrigin)
	}
	t.Run("network fault on the unique application response", func(t *testing.T) {
		d := derive(t, nil)
		if d.Network == nil || d.DOM == nil || d.Witnessed == nil {
			t.Fatalf("derivation = %+v", d)
		}
		install := d.Network.Install
		if install.OriginPath != appOrigin+"/api/total" || install.RequestOrdinal != 1 || install.Search != "42 apples" || install.Method != "GET" {
			t.Fatalf("network install = %+v", install)
		}
		plan := d.Network.Plan
		if plan.OriginPath != digestHex([]byte(appOrigin+"/api/total")) || plan.ValueLength != 9 || plan.Marker != Marker(testKey, "total", "e2e/cart.spec.cjs:7:5", KindNetwork) {
			t.Fatalf("network plan = %+v", plan)
		}
		if d.Network.Digest != plan.Digest() || plan.BodyDigestAfter != digestHex([]byte(`{"total":"`+plan.Marker+`"}`)) {
			t.Fatal("plan digest or rewritten body digest is wrong")
		}
		if d.DOM.Install.Action != ActionText || d.DOM.Install.Selector != "#total" {
			t.Fatalf("dom install = %+v", d.DOM.Install)
		}
	})
	t.Run("json-escaped value", func(t *testing.T) {
		d := derive(t, func(a *traceArchive) {
			a.frames[0]["params"].(map[string]any)["expectedText"] = []ExpectedText{{String: text(`say "hi"`)}}
			a.response("GET", appOrigin+"/api/quote", "application/json", []byte(`{"q":"say \"hi\""}`), 20, 1)
		})
		if d.Network == nil || d.Network.Install.Search != `say \"hi\"` {
			t.Fatalf("derivation = %+v", d)
		}
	})
	for _, test := range []struct {
		name, network, dom string
		mutate             func(*traceArchive)
	}{
		{"value twice is ambiguous", ReasonNetworkDependencyAmbiguous, "", func(a *traceArchive) {
			a.response("GET", appOrigin+"/api/other", "application/json", []byte(`["42 apples"]`), 20, 1)
		}},
		{"value only after the assertion started", ReasonNoNetworkDependency, "", func(a *traceArchive) {
			a.network[1]["snapshot"].(map[string]any)["time"] = 200.0
		}},
		{"value only on another origin", ReasonOutsideApplicationOrigin, "", func(a *traceArchive) {
			a.network[1]["snapshot"].(map[string]any)["request"] = map[string]any{"method": "GET", "url": "http://127.0.0.1:4001/api/total"}
		}},
		{"binary body is not a candidate", ReasonNoNetworkDependency, "", func(a *traceArchive) {
			a.network[1]["snapshot"].(map[string]any)["response"].(map[string]any)["content"].(map[string]any)["mimeType"] = "image/png"
		}},
		{"regular expression", ReasonExpectedValueNotLiteral, ReasonExpectedValueNotLiteral, func(a *traceArchive) {
			a.frames[0]["params"].(map[string]any)["expectedText"] = []ExpectedText{{RegexSource: text(`\d+ apples`)}}
		}},
		{"negated", ReasonNegatedAssertion, ReasonNegatedAssertion, func(a *traceArchive) {
			a.events[5]["title"] = `Expect "not toHaveText"`
		}},
		{"marker would satisfy a substring expectation", "", ReasonFaultDoesNotFalsify, func(a *traceArchive) {
			a.events[5]["title"] = `Expect "toContainText"`
			a.frames[0]["params"].(map[string]any)["expectedText"] = []ExpectedText{{String: text("corvint-fault"), MatchSubstring: true}}
		}},
		{"unsupported matcher", ReasonExpectedValueNotLiteral, ReasonNoDOMFault, func(a *traceArchive) {
			a.events[5]["title"] = `Expect "toHaveCount"`
		}},
		{"locator not recovered", ReasonExpectedValueNotLiteral, ReasonNoDOMFault, func(a *traceArchive) {
			a.frames = a.frames[1:]
		}},
	} {
		t.Run(test.name, func(t *testing.T) {
			d := derive(t, test.mutate)
			if test.network != "" && (d.Network != nil || d.NetworkReason != test.network) {
				t.Errorf("network = %v reason %q, want reason %q", d.Network != nil, d.NetworkReason, test.network)
			}
			if test.dom != "" && (d.DOM != nil || d.DOMReason != test.dom) {
				t.Errorf("dom = %v reason %q, want reason %q", d.DOM != nil, d.DOMReason, test.dom)
			}
		})
	}
	t.Run("visibility gives a hide fault and no network fault", func(t *testing.T) {
		a := baselineArchive()
		a.events = a.events[:len(a.events)-2]
		a.before("s4", "", "test.step", "footer", 12, 165)
		a.before("e4", "s4", "expect", `Expect "toBeVisible"`, 13, 170)
		a.after("e4", "")
		a.after("s4", "")
		a.frames[2]["stepId"] = "e4"
		d := Derive(parsed(t, a), testKey, 4, appOrigin)
		if d.Network != nil || d.DOM == nil || d.DOM.Install.Action != ActionHide {
			t.Fatalf("derivation = %+v", d)
		}
	})
	t.Run("assertion-free step", func(t *testing.T) {
		d := Derive(parsed(t, baselineArchive()), testKey, 1, appOrigin)
		entry := Underivable(d)
		if !d.NoAssertion || entry.Strength.Reason != ReasonStepHasNoAssertion || entry.Requires != RequiresManualControl {
			t.Fatalf("entry = %+v", entry)
		}
	})
	t.Run("marker never depends on the plan", func(t *testing.T) {
		first := derive(t, nil)
		second := derive(t, func(a *traceArchive) {
			a.network[1]["snapshot"].(map[string]any)["response"] = map[string]any{"content": map[string]any{"mimeType": "application/json", "_file": "resources/other"}}
			a.resources["resources/other"] = []byte(`{"unit":"fruit","total":"42 apples"}`)
		})
		if first.Network.Plan.Marker != second.Network.Plan.Marker || first.Network.Digest == second.Network.Digest {
			t.Fatal("the marker changed with the plan, or the plan digest ignored the body")
		}
		if marker := first.Network.Plan.Marker; !strings.HasPrefix(marker, "corvint-fault-") || len(marker) != len("corvint-fault-")+8 {
			t.Fatalf("marker = %q", marker)
		}
	})
}

// faultedArchive returns baselineArchive with mutate applied.
func faultedArchive(mutate func(*traceArchive)) *traceArchive {
	a := baselineArchive()
	if mutate != nil {
		mutate(a)
	}
	return a
}

// failWitnessed fails the witnessed step-2 assertion showing marker as the
// received value; a hard expect ends the test there.
func failWitnessed(marker string) func(*traceArchive) {
	return func(a *traceArchive) {
		message := "expect(locator).toHaveText(expected) failed\n\nExpected: \"42 apples\"\n\x1b[31mReceived: \"" + marker + "\"\x1b[39m"
		a.events[6] = map[string]any{"type": "after", "callId": "e1", "error": map[string]any{"message": message}}
		a.events[7] = map[string]any{"type": "after", "callId": "s2", "error": map[string]any{"message": message}}
		a.events = a.events[:8]
		a.frames = a.frames[:1]
	}
}

// LPCV-V0-060, LPCV-V0-063: the single-step pass rule.
func TestEvaluateSingleRule(t *testing.T) {
	baseline := parsed(t, baselineArchive())
	d := Derive(baseline, testKey, 2, appOrigin)
	fault := d.Network
	marker := fault.Install.Marker
	applied := map[string]int{fault.Digest: 1}
	failed := func(t *testing.T, mutate func(*traceArchive)) *Trace {
		return parsed(t, faultedArchive(func(a *traceArchive) {
			failWitnessed(marker)(a)
			if mutate != nil {
				mutate(a)
			}
		}))
	}
	for _, test := range []struct {
		name   string
		run    func(t *testing.T) Run
		state  string
		reason string
	}{
		{"killed", func(t *testing.T) Run {
			return Run{Status: "failed", Attempts: 1, Trace: failed(t, nil), Applied: applied}
		}, testvalidity.StrengthKilled, ReasonKilled},
		{"survived", func(t *testing.T) Run { return Run{Status: "passed", Attempts: 1, Trace: baseline, Applied: applied} }, testvalidity.StrengthSurvived, ReasonFaultSurvived},
		{"not applied is never survived", func(t *testing.T) Run {
			return Run{Status: "passed", Attempts: 1, Trace: baseline, Applied: map[string]int{}}
		}, testvalidity.StrengthNotMeasured, ReasonFaultNotApplied},
		{"infrastructure", func(t *testing.T) Run {
			return Run{Status: "failed", Attempts: 1, Trace: failed(t, nil), Applied: applied, Infrastructure: true}
		}, testvalidity.StrengthNotMeasured, ReasonFaultedRunInfrastructure},
		{"interrupted", func(t *testing.T) Run {
			return Run{Status: "interrupted", Attempts: 1, Trace: failed(t, nil), Applied: applied}
		}, testvalidity.StrengthNotMeasured, ReasonFaultedRunInfrastructure},
		{"no trace", func(t *testing.T) Run { return Run{Status: "failed", Attempts: 1, Applied: applied} }, testvalidity.StrengthNotMeasured, ReasonFaultedRunInfrastructure},
		{"timeout", func(t *testing.T) Run {
			return Run{Status: "timedOut", Attempts: 1, Trace: failed(t, nil), Applied: applied}
		}, testvalidity.StrengthNotMeasured, ReasonFaultedRunTimeout},
		{"retried", func(t *testing.T) Run {
			return Run{Status: "failed", Attempts: 2, Trace: failed(t, nil), Applied: applied}
		}, testvalidity.StrengthNotMeasured, ReasonFaultedRunRetried},
		{"failure without the marker", func(t *testing.T) Run {
			return Run{Status: "failed", Attempts: 1, Trace: parsed(t, faultedArchive(failWitnessed("something else"))), Applied: applied}
		}, testvalidity.StrengthNotMeasured, ReasonUnattributedFailure},
		{"earlier step failed", func(t *testing.T) Run {
			return Run{Status: "failed", Attempts: 1, Applied: applied, Trace: failed(t, func(a *traceArchive) {
				a.events[3] = map[string]any{"type": "after", "callId": "s1", "error": map[string]any{"message": "boom"}}
			})}
		}, testvalidity.StrengthNotMeasured, ReasonFaultNotStepIsolated},
		{"another step's assertion failed", func(t *testing.T) Run {
			return Run{Status: "failed", Attempts: 1, Applied: applied, Trace: failed(t, func(a *traceArchive) {
				a.before("s3", "", "test.step", "title", 9, 150)
				a.before("e2", "s3", "expect", `Expect "soft toHaveText"`, 10, 160)
				a.after("e2", "collateral")
				a.after("s3", "collateral")
			})}
		}, testvalidity.StrengthNotMeasured, ReasonFaultNotStepIsolated},
		{"api failure in another step", func(t *testing.T) Run {
			return Run{Status: "failed", Attempts: 1, Applied: applied, Trace: failed(t, func(a *traceArchive) {
				a.events[2] = map[string]any{"type": "after", "callId": "p1", "error": map[string]any{"message": "net"}}
			})}
		}, testvalidity.StrengthNotMeasured, ReasonFaultNotStepIsolated},
		{"passed with a failed step", func(t *testing.T) Run {
			return Run{Status: "passed", Attempts: 1, Applied: applied, Trace: failed(t, nil)}
		}, testvalidity.StrengthNotMeasured, ReasonUnattributedFailure},
	} {
		t.Run(test.name, func(t *testing.T) {
			axis := EvaluateSingle(baseline, d, fault, test.run(t), 1)
			if axis.State != test.state || axis.Reason != test.reason {
				t.Fatalf("axis = %+v, want %s %s", axis, test.state, test.reason)
			}
			if !reflect.DeepEqual(axis.Anchors, []string{"plan:" + fault.Digest, "assertion:e2e/cart.spec.cjs:7:5#1"}) {
				t.Fatalf("anchors = %v", axis.Anchors)
			}
		})
	}
	t.Run("visibility needs the hidden observation and two baseline passes", func(t *testing.T) {
		a := baselineArchive()
		a.events = a.events[:len(a.events)-2]
		a.before("s4", "", "test.step", "footer", 12, 165)
		a.before("e4", "s4", "expect", `Expect "toBeVisible"`, 13, 170)
		a.after("e4", "")
		a.after("s4", "")
		a.frames[2]["stepId"] = "e4"
		visible := parsed(t, a)
		d := Derive(visible, testKey, 4, appOrigin)
		a.events[len(a.events)-2] = map[string]any{"type": "after", "callId": "e4", "error": map[string]any{"message": "element is not visible"}}
		a.events[len(a.events)-1] = map[string]any{"type": "after", "callId": "s4", "error": map[string]any{"message": "element is not visible"}}
		run := Run{Status: "failed", Attempts: 1, Trace: parsed(t, a), Applied: map[string]int{d.DOM.Digest: 1}, Hidden: map[string]bool{d.DOM.Digest: true}}
		if axis := EvaluateSingle(visible, d, d.DOM, run, 1); axis.Reason != ReasonUnattributedFailure {
			t.Fatalf("one baseline pass: %+v", axis)
		}
		if axis := EvaluateSingle(visible, d, d.DOM, run, 2); axis.State != testvalidity.StrengthKilled {
			t.Fatalf("two baseline passes: %+v", axis)
		}
		run.Hidden = nil
		if axis := EvaluateSingle(visible, d, d.DOM, run, 2); axis.Reason != ReasonUnattributedFailure {
			t.Fatalf("not observed hidden: %+v", axis)
		}
	})
}

// LPCV-V0-058, LPCV-V0-063: network first, DOM after a non-kill, and the
// network-survival-alone rule.
func TestResolveSingleOrder(t *testing.T) {
	d := Derive(parsed(t, baselineArchive()), testKey, 2, appOrigin)
	killed := &testvalidity.Axis{State: testvalidity.StrengthKilled, Reason: ReasonKilled, Anchors: []string{"plan:n"}}
	survived := &testvalidity.Axis{State: testvalidity.StrengthSurvived, Reason: ReasonFaultSurvived, Anchors: []string{"plan:x"}}
	notApplied := &testvalidity.Axis{State: testvalidity.StrengthNotMeasured, Reason: ReasonFaultNotApplied, Anchors: []string{"plan:x"}}
	networkOnly := d
	networkOnly.DOM = nil
	domOnly := d
	domOnly.Network = nil
	for _, test := range []struct {
		name               string
		derivation         Derivation
		network, dom       *testvalidity.Axis
		state, reason      string
		witness, requires  string
		networkSurvivedTag bool
	}{
		{"network kill is final", d, killed, nil, testvalidity.StrengthKilled, ReasonKilled, WitnessNetwork, "", false},
		{"dom kill after network survival", d, survived, killed, testvalidity.StrengthKilled, ReasonKilled, WitnessDOM, "", true},
		{"dom survival", d, survived, survived, testvalidity.StrengthSurvived, ReasonFaultSurvived, WitnessDOM, "", false},
		{"dom unmeasured after network survival", d, survived, notApplied, testvalidity.StrengthNotMeasured, ReasonFaultNotApplied, WitnessNone, RequiresManualControl, true},
		{"network survival alone", networkOnly, survived, nil, testvalidity.StrengthNotMeasured, ReasonNetworkFaultSurvived, WitnessNone, RequiresManualControl, false},
		{"network not run for budget", d, nil, nil, testvalidity.StrengthNotMeasured, ReasonRunBudgetExhausted, WitnessNone, "", false},
		{"dom not run for budget", d, notApplied, nil, testvalidity.StrengthNotMeasured, ReasonRunBudgetExhausted, WitnessNone, "", false},
		{"dom only", domOnly, nil, killed, testvalidity.StrengthKilled, ReasonKilled, WitnessDOM, "", false},
	} {
		t.Run(test.name, func(t *testing.T) {
			step := ResolveSingle(test.derivation, test.network, test.dom)
			if step.Strength.State != test.state || step.Strength.Reason != test.reason || step.Witness != test.witness || step.Requires != test.requires {
				t.Fatalf("step = %+v", step)
			}
			tagged := false
			for _, anchor := range step.Strength.Anchors {
				tagged = tagged || anchor == "reason:"+ReasonNetworkFaultSurvived
			}
			if tagged != test.networkSurvivedTag {
				t.Fatalf("network-fault-survived tag = %v, anchors %v", tagged, step.Strength.Anchors)
			}
			if step.PlanDigest == "" || step.Title != "total" || step.Ordinal != 2 {
				t.Fatalf("step identity = %+v", step)
			}
		})
	}
	underivable := Derive(parsed(t, faultedArchive(func(a *traceArchive) {
		a.frames[0]["params"].(map[string]any)["expectedText"] = []ExpectedText{{RegexSource: text("x")}}
	})), testKey, 2, appOrigin)
	if step := ResolveSingle(underivable, nil, nil); step.Strength.Reason != ReasonUnderivable || step.Requires != RequiresManualControl ||
		!reflect.DeepEqual(step.Strength.Anchors, []string{"derivation:network:" + ReasonExpectedValueNotLiteral, "derivation:dom:" + ReasonExpectedValueNotLiteral}) {
		t.Fatalf("underivable = %+v", step)
	}
}

// LPCV-V0-064: joint planning and marker attribution.
func TestJointPlanAndAttribution(t *testing.T) {
	baseline := parsed(t, baselineArchive())
	derivations := []Derivation{Derive(baseline, testKey, 1, appOrigin), Derive(baseline, testKey, 2, appOrigin), Derive(baseline, testKey, 3, appOrigin)}
	faults, entries := PlanJoint(derivations)
	if len(faults) != 2 || faults[0].Fault.Install.Kind != KindNetwork || faults[1].Fault.Install.Kind != KindDOM {
		t.Fatalf("joint faults = %+v", faults)
	}
	if entries[1].Strength.Reason != ReasonStepHasNoAssertion {
		t.Fatalf("entries = %+v", entries)
	}
	t.Run("collision and visibility need single-step runs", func(t *testing.T) {
		duplicate := derivations[2]
		duplicate.Ordinal = 4
		hide := derivations[2]
		hide.Ordinal, hide.Network = 5, nil
		hideFault := *hide.DOM
		hideFault.Install.Action, hideFault.Install.Selector = ActionHide, "#footer"
		hide.DOM = &hideFault
		_, entries := PlanJoint([]Derivation{derivations[2], duplicate, hide})
		if entries[4].Strength.Reason != ReasonRequiresSingleStepRun || entries[4].Requires != RequiresSingleStepRun || entries[5].Strength.Reason != ReasonRequiresSingleStepRun {
			t.Fatalf("entries = %+v", entries)
		}
	})
	networkMarker, domMarker := faults[0].Fault.Install.Marker, faults[1].Fault.Install.Marker
	applied := map[string]int{faults[0].Fault.Digest: 1, faults[1].Fault.Digest: 1}
	failBoth := func(a *traceArchive, first, second string) {
		a.events[6] = map[string]any{"type": "after", "callId": "e1", "error": map[string]any{"message": "Received: \"" + first + "\""}}
		a.events[10] = map[string]any{"type": "after", "callId": "e2", "error": map[string]any{"message": "Received: \"" + second + "\""}}
	}
	for _, test := range []struct {
		name    string
		run     func(t *testing.T) Run
		reasons [2]string
		witness [2]string
	}{
		{"both killed by their own markers", func(t *testing.T) Run {
			return Run{Status: "failed", Attempts: 1, Applied: applied, Trace: parsed(t, faultedArchive(func(a *traceArchive) { failBoth(a, networkMarker, domMarker) }))}
		}, [2]string{ReasonKilled, ReasonKilled}, [2]string{WitnessJointNetwork, WitnessJointDOM}},
		{"a crossed marker fails attribution for every step", func(t *testing.T) Run {
			return Run{Status: "failed", Attempts: 1, Applied: applied, Trace: parsed(t, faultedArchive(func(a *traceArchive) { failBoth(a, networkMarker, networkMarker) }))}
		}, [2]string{ReasonJointAttributionFailed, ReasonJointAttributionFailed}, [2]string{WitnessNone, WitnessNone}},
		{"hard failure leaves later steps not reached", func(t *testing.T) Run {
			return Run{Status: "failed", Attempts: 1, Applied: applied, Trace: parsed(t, faultedArchive(failWitnessed(networkMarker)))}
		}, [2]string{ReasonKilled, ReasonStepNotReached}, [2]string{WitnessJointNetwork, WitnessNone}},
		{"applied and passing is unconfirmed", func(t *testing.T) Run {
			return Run{Status: "passed", Attempts: 1, Applied: applied, Trace: baseline}
		}, [2]string{ReasonJointSurvivalUnconfirmed, ReasonJointSurvivalUnconfirmed}, [2]string{WitnessNone, WitnessNone}},
		{"unapplied", func(t *testing.T) Run {
			return Run{Status: "passed", Attempts: 1, Applied: map[string]int{}, Trace: baseline}
		}, [2]string{ReasonFaultNotApplied, ReasonFaultNotApplied}, [2]string{WitnessNone, WitnessNone}},
		{"retried run", func(t *testing.T) Run {
			return Run{Status: "failed", Attempts: 2, Applied: applied, Trace: baseline}
		}, [2]string{ReasonFaultedRunRetried, ReasonFaultedRunRetried}, [2]string{WitnessNone, WitnessNone}},
	} {
		t.Run(test.name, func(t *testing.T) {
			result := EvaluateJoint(faults, test.run(t))
			for index, ordinal := range []int{2, 3} {
				step := result[ordinal]
				if step.Strength.Reason != test.reasons[index] || step.Witness != test.witness[index] || step.PlanDigest != faults[index].Fault.Digest {
					t.Errorf("step %d = %+v, want %s witness %s", ordinal, step, test.reasons[index], test.witness[index])
				}
			}
		})
	}
}

func sampleDocument() Document {
	return Document{
		Schema: Schema,
		Binding: Binding{
			TestRepository: Repository{RootCommit: strings.Repeat("a", 40), Revision: strings.Repeat("b", 40), Tree: strings.Repeat("c", 40)},
			ConfigFile:     "playwright.config.cjs", ConfigDigest: strings.Repeat("1", 64), SpecFile: testKey.File, SpecDigest: strings.Repeat("2", 64),
			Test:                  TestBinding{File: testKey.File, FullTitle: testKey.FullTitle, Project: testKey.Project, Browser: "chromium", Device: "none"},
			Runner:                Runner{Name: "playwright", Version: "1.63.0", NodeVersion: "v22.23.2", Tuple: "candidate"},
			Application:           Application{Profile: "corvint-playwright-external/0", Label: "fixture-app"},
			ReadinessOrigin:       appOrigin,
			InjectionModuleDigest: strings.Repeat("3", 64),
		},
		Mode:      ModeStep,
		Runs:      Runs{Budget: 3, Used: 2, BaselineRepeat: 1, BaselinePassed: 1},
		Inventory: Inventory{Steps: []InventoryStep{{"open", 1, 0}, {"total", 2, 1}, {"title", 3, 1}}},
		Steps: []Step{
			{PlanDigest: strings.Repeat("4", 64), Title: "total", Ordinal: 2, Witness: WitnessNetwork, Strength: testvalidity.Axis{State: testvalidity.StrengthKilled, Reason: ReasonKilled, Anchors: []string{"plan:" + strings.Repeat("4", 64)}}},
		},
	}
}

// LPCV-V0-065: the closed canonical codec.
func TestDocumentCodecIsClosedAndCanonical(t *testing.T) {
	document := sampleDocument()
	data, err := Encode(document)
	if err != nil {
		t.Fatal(err)
	}
	if data[len(data)-1] != '\n' || bytes.Count(data, []byte("\n")) != 1 {
		t.Fatal("encoding is not compact JSON plus LF")
	}
	decoded, err := Decode(data)
	if err != nil || !reflect.DeepEqual(decoded, document) {
		t.Fatalf("round trip: %v", err)
	}
	refused := map[string][]byte{
		"unknown field": bytes.Replace(data, []byte(`{"schema"`), []byte(`{"extra":1,"schema"`), 1),
		"noncanonical":  bytes.Replace(data, []byte(`,"mode"`), []byte(`, "mode"`), 1),
		"trailing data": append(append([]byte{}, data...), []byte("{}\n")...),
		"missing LF":    data[:len(data)-1],
		"oversized":     make([]byte, MaxDocumentBytes+1),
	}
	for name, bad := range refused {
		if _, err := Decode(bad); err == nil {
			t.Errorf("%s decoded", name)
		}
	}
	for name, mutate := range map[string]func(*Document){
		"schema":                 func(d *Document) { d.Schema = "corvint-step-negation/1" },
		"mode":                   func(d *Document) { d.Mode = "watch" },
		"runs over budget":       func(d *Document) { d.Runs.Used = 4 },
		"baseline repeat":        func(d *Document) { d.Runs.BaselineRepeat = 6 },
		"nil steps":              func(d *Document) { d.Steps = nil },
		"step outside inventory": func(d *Document) { d.Steps[0].Title = "other" },
		"new axis state":         func(d *Document) { d.Steps[0].Strength.State = "UNPROVEN" },
		"killed without witness": func(d *Document) { d.Steps[0].Witness = WitnessNone },
		"witness outside set":    func(d *Document) { d.Steps[0].Witness = "source" },
		"requires outside set":   func(d *Document) { d.Steps[0].Requires = "human" },
		"unordered": func(d *Document) {
			d.Steps = append(d.Steps, Step{Title: "open", Ordinal: 1, Witness: WitnessNone, Strength: notMeasured(ReasonStepHasNoAssertion)})
		},
		"unresolved entry with a plan": func(d *Document) {
			d.Steps = []Step{{PlanDigest: strings.Repeat("4", 64), Title: "missing", Witness: WitnessNone, Strength: notMeasured(ReasonStepNotFound)}}
		},
	} {
		bad := sampleDocument()
		mutate(&bad)
		if _, err := Encode(bad); err == nil {
			t.Errorf("%s encoded", name)
		}
	}
	unresolved := sampleDocument()
	unresolved.Steps = []Step{{Title: "missing", Witness: WitnessNone, Strength: notMeasured(ReasonStepNotFound)}}
	if _, err := Encode(unresolved); err != nil {
		t.Fatalf("unresolved entry: %v", err)
	}
	if got := Screen("cmd --pass=synthetic123"); !strings.HasPrefix(got, "sha256:") || Screen("total") != "total" {
		t.Fatalf("Screen = %q", got)
	}
}

// LPCV-V0-068: the PTF-V0-006 aggregation over the inventory denominator.
func TestAggregateOverInventory(t *testing.T) {
	killedStep := func(ordinal int, title string) Step {
		digest := strings.Repeat(string(rune('0'+ordinal)), 64)
		return Step{PlanDigest: digest, Title: title, Ordinal: ordinal, Witness: WitnessDOM, Strength: testvalidity.Axis{State: testvalidity.StrengthKilled, Reason: ReasonKilled, Anchors: []string{}}}
	}
	document := sampleDocument()
	if axis := Aggregate(document); axis.State != testvalidity.StrengthNotMeasured || axis.Reason != ReasonStepControlsIncomplete || !reflect.DeepEqual(axis.Anchors, []string{"step:3"}) {
		t.Fatalf("incomplete = %+v", axis)
	}
	document.Steps = append(document.Steps, killedStep(3, "title"))
	if axis := Aggregate(document); axis.State != testvalidity.StrengthKilled || axis.Reason != ReasonStepControlsKilled || len(axis.Anchors) != 2 {
		t.Fatalf("killed = %+v", axis)
	}
	outside := document
	outside.Inventory.AssertionsOutsideSteps = 1
	if axis := Aggregate(outside); axis.Reason != ReasonAssertionsOutsideSteps {
		t.Fatalf("outside = %+v", axis)
	}
	survived := document
	survived.Steps = []Step{document.Steps[0], {PlanDigest: strings.Repeat("5", 64), Title: "title", Ordinal: 3, Witness: WitnessDOM, Strength: testvalidity.Axis{State: testvalidity.StrengthSurvived, Reason: ReasonFaultSurvived, Anchors: []string{}}}}
	survived.Inventory.AssertionsOutsideSteps = 1
	if axis := Aggregate(survived); axis.State != testvalidity.StrengthSurvived || !reflect.DeepEqual(axis.Anchors, []string{"step:3"}) {
		t.Fatalf("survived = %+v", axis)
	}
	empty := sampleDocument()
	empty.Inventory = Inventory{Steps: []InventoryStep{{"open", 1, 0}}}
	empty.Steps = []Step{}
	if axis := Aggregate(empty); axis.Reason != ReasonStepControlsIncomplete {
		t.Fatalf("no assertion-bearing step = %+v", axis)
	}
}

// LPCV-V0-067: atomic confined retention, merge and the writer lock.
func TestRetentionMergeLockAndConfinement(t *testing.T) {
	worktree := t.TempDir()
	if document, err := Load(worktree, testKey); err != nil || document != nil {
		t.Fatalf("Load on an empty worktree = %v, %v", document, err)
	}
	if _, err := os.Lstat(filepath.Join(worktree, ".corvint")); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("Load created the retention directory")
	}
	release, err := Lock(worktree, testKey)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Lock(worktree, testKey); !errors.Is(err, ErrBusy) {
		t.Fatalf("second Lock = %v, want ErrBusy", err)
	}
	first := sampleDocument()
	if err := Retain(worktree, Merge(nil, first)); err != nil {
		t.Fatal(err)
	}
	release()
	if again, err := Lock(worktree, testKey); err != nil {
		t.Fatalf("Lock after release: %v", err)
	} else {
		again()
	}
	path := filepath.Join(worktree, EvidenceDirectory, FileName(testKey))
	for name, want := range map[string]os.FileMode{path: 0o600, filepath.Dir(path): 0o700, filepath.Join(worktree, ".corvint"): 0o700} {
		info, err := os.Stat(name)
		if err != nil || info.Mode().Perm() != want {
			t.Fatalf("%s mode = %v, %v; want %v", name, info.Mode().Perm(), err, want)
		}
	}
	stored, err := Load(worktree, testKey)
	if err != nil || stored == nil || !reflect.DeepEqual(*stored, first) {
		t.Fatalf("Load = %+v, %v", stored, err)
	}

	second := sampleDocument()
	second.Steps = []Step{{PlanDigest: strings.Repeat("5", 64), Title: "title", Ordinal: 3, Witness: WitnessDOM, Strength: testvalidity.Axis{State: testvalidity.StrengthKilled, Reason: ReasonKilled, Anchors: []string{}}}}
	merged := Merge(stored, second)
	if len(merged.Steps) != 2 || merged.Steps[0].Ordinal != 2 || merged.Steps[1].Ordinal != 3 {
		t.Fatalf("equal binding merge = %+v", merged.Steps)
	}
	rerun := Merge(stored, sampleDocument())
	if !rerun.Steps[0].PlanReused {
		t.Fatal("an equal plan digest is not reported reused")
	}
	drifted := sampleDocument()
	drifted.Binding.SpecDigest = strings.Repeat("9", 64)
	drifted.Steps = second.Steps
	if replaced := Merge(stored, drifted); len(replaced.Steps) != 1 || replaced.Steps[0].Ordinal != 3 {
		t.Fatalf("different binding merge = %+v", replaced.Steps)
	}
	if err := Retain(worktree, merged); err != nil {
		t.Fatal(err)
	}
	entries, _ := os.ReadDir(filepath.Dir(path))
	for _, entry := range entries {
		if strings.HasSuffix(entry.Name(), ".tmp") {
			t.Fatalf("temporary left behind: %s", entry.Name())
		}
	}

	t.Run("symlinked component is refused", func(t *testing.T) {
		linked := t.TempDir()
		elsewhere := t.TempDir()
		if err := os.Symlink(elsewhere, filepath.Join(linked, ".corvint")); err != nil {
			t.Fatal(err)
		}
		if err := Retain(linked, sampleDocument()); err == nil {
			t.Fatal("Retain followed a symlinked .corvint")
		}
		if _, err := Lock(linked, testKey); err == nil {
			t.Fatal("Lock followed a symlinked .corvint")
		}
		if _, err := Load(linked, testKey); err == nil {
			t.Fatal("Load followed a symlinked .corvint")
		}
		if entries, _ := os.ReadDir(elsewhere); len(entries) != 0 {
			t.Fatal("a write escaped through the symlink")
		}
	})
	t.Run("undecodable retained bytes load as absent", func(t *testing.T) {
		if err := os.WriteFile(path, []byte("{}\n"), 0o600); err != nil {
			t.Fatal(err)
		}
		if document, err := Load(worktree, testKey); err != nil || document != nil {
			t.Fatalf("Load = %v, %v", document, err)
		}
	})
}

// LPCV-V0-059, LPCV-V0-067: the lock and retained files stay out of `git
// status`, so evidence never makes the clean test repository that a later
// negate run requires dirty.
func TestRetentionStaysOutOfGitStatus(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("NOT_RUN: git is not installed")
	}
	worktree := t.TempDir()
	git := func(args ...string) string {
		t.Helper()
		command := exec.Command("git", append([]string{"-C", worktree, "-c", "user.name=fixture", "-c", "user.email=fixture@example.invalid", "-c", "commit.gpgsign=false"}, args...)...)
		command.Env = []string{"HOME=" + worktree, "PATH=" + os.Getenv("PATH")}
		output, err := command.CombinedOutput()
		if err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, output)
		}
		return string(output)
	}
	git("init", "-q")
	if err := os.WriteFile(filepath.Join(worktree, "cart.spec.cjs"), []byte("// fixture\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	git("add", "cart.spec.cjs")
	git("commit", "-q", "-m", "fixture")
	release, err := Lock(worktree, testKey)
	if err != nil {
		t.Fatal(err)
	}
	if status := git("status", "--porcelain=v1", "--untracked-files=all"); status != "" {
		t.Fatalf("a held lock makes the repository dirty:\n%s", status)
	}
	if err := Retain(worktree, sampleDocument()); err != nil {
		t.Fatal(err)
	}
	release()
	if status := git("status", "--porcelain=v1", "--untracked-files=all"); status != "" {
		t.Fatalf("retained evidence makes the repository dirty:\n%s", status)
	}
	if data, err := os.ReadFile(filepath.Join(worktree, EvidenceDirectory, ".gitignore")); err != nil || string(data) != retentionIgnore {
		t.Fatalf(".gitignore = %q, %v", data, err)
	}
}
