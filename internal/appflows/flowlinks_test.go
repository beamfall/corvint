// SPDX-License-Identifier: Apache-2.0

package appflows

import (
	"math"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/liveverify/affected"
)

func fixtureFlowTestLink() FlowTestLink {
	return FlowTestLink{
		TestID: "playwright:e2e/cart.spec.ts::cart::buys", VariationID: "cart.buy",
		Evidence:   []string{"0123456789abcdef0123456789abcdef01234567:e2e/cart.spec.ts"},
		Confidence: 0.75, ConfidenceFormula: "fixture/0", AcceptedBy: "Fixture reviewer",
		AcceptedAt: "2026-11-18T12:00:00Z", PinnedCommit: "0123456789abcdef0123456789abcdef01234567",
		TestContentHash: strings.Repeat("1", 40), VariationDigest: strings.Repeat("2", 64),
	}
}

// FLP-V1-006: every field survives the G6 reader/writer and its frozen version
// 1 fixture; version 0 does not silently admit a link member.
func TestFLPV1006LinkRecordRoundTrip(t *testing.T) {
	want := fixtureFlowTestLink()
	golden, err := os.ReadFile(filepath.Join(flowsSchemaTestdata, "1.golden.json"))
	if err != nil {
		t.Fatal(err)
	}
	declared, err := decodeFlowsDeclaration(golden)
	if err != nil || declared == nil || declared.Version != 1 || !reflect.DeepEqual(declared.Links, []FlowTestLink{want}) {
		t.Fatalf("golden: %+v, %v", declared, err)
	}
	encoded, err := EncodeFlowsDeclaration(*declared)
	if err != nil {
		t.Fatal(err)
	}
	got, err := readFlowsBody(t, string(encoded))
	if err != nil || !reflect.DeepEqual(got, declared) {
		t.Fatalf("round trip: %+v, want %+v, error %v", got, declared, err)
	}
	if !got.Links[0].HasAcceptance() {
		t.Fatal("valid acceptance fields were lost")
	}
	old := strings.Replace(string(encoded), "corvint-flows/1", "corvint-flows/0", 1)
	if _, err := readFlowsBody(t, old); AsFlowsRefusal(err) == nil {
		t.Fatal("version 0 accepted a version 1 link record")
	}
}

// FLP-V1-006: acceptance is conjunctive, independent of confidence. These
// deliberately unaccepted records remain readable for later reporting.
func TestFLPV1006MissingAcceptance(t *testing.T) {
	for _, field := range []string{"accepted_by", "accepted_at", "pinned_commit"} {
		t.Run(field, func(t *testing.T) {
			link := fixtureFlowTestLink()
			link.Confidence = 1
			body, err := EncodeFlowsDeclaration(FlowsDeclaration{Dir: "flows", Links: []FlowTestLink{link}})
			if err != nil {
				t.Fatal(err)
			}
			start := strings.Index(string(body), `"`+field+`":`)
			if start < 0 {
				t.Fatal("missing fixture field", field)
			}
			end := start + len(field) + 4
			end += strings.Index(string(body[end:]), `"`) + 1
			// The three fields precede other record fields in the writer.
			without := string(body[:start]) + string(body[end+1:])
			got, err := readFlowsBody(t, without)
			if err != nil || got == nil || got.Links[0].HasAcceptance() {
				t.Fatalf("missing %s: %+v, %v; %s", field, got, err, without)
			}
		})
	}
	for _, edit := range []func(*FlowTestLink){
		func(l *FlowTestLink) { l.AcceptedBy = " \n\t" },
		func(l *FlowTestLink) { l.AcceptedAt = "yesterday" },
		func(l *FlowTestLink) { l.AcceptedAt = "2026-11-18T12:00:00-05:00" },
		func(l *FlowTestLink) { l.PinnedCommit = "HEAD" },
	} {
		link := fixtureFlowTestLink()
		edit(&link)
		if link.HasAcceptance() {
			t.Fatalf("invalid acceptance grants authority: %+v", link)
		}
	}
}

// FLP-V1-006: no test kind or unknown member can be smuggled through the
// closed record; resource and confidence bounds also apply to the writer.
func TestFLPV1006ClosedRecord(t *testing.T) {
	for _, field := range []string{"kind", "extra"} {
		body := `{"profile":"corvint-flows/1","flows":"flows","links":[{"test_id":"test","variation_id":"v","` + field + `":"e2e"}]}`
		if _, err := readFlowsBody(t, body); AsFlowsRefusal(err) == nil {
			t.Fatalf("accepted unknown %s: %v", field, err)
		}
	}
	for _, edit := range []func(*FlowTestLink){
		func(l *FlowTestLink) { l.TestID = "" },
		func(l *FlowTestLink) { l.VariationID = "../out" },
		func(l *FlowTestLink) { l.Confidence = -0.01 },
		func(l *FlowTestLink) { l.Confidence = 1.01 },
		func(l *FlowTestLink) { l.Confidence = math.NaN() },
		func(l *FlowTestLink) { l.Evidence = make([]string, maxFlowList+1) },
		func(l *FlowTestLink) { l.AcceptedBy = strings.Repeat("x", maxFlowText+1) },
	} {
		link := fixtureFlowTestLink()
		edit(&link)
		if body, err := EncodeFlowsDeclaration(FlowsDeclaration{Dir: "flows", Links: []FlowTestLink{link}}); err == nil {
			t.Fatalf("wrote invalid record: %s", body)
		}
	}
	link := fixtureFlowTestLink()
	link.Evidence = make([]string, 20)
	for i := range link.Evidence {
		link.Evidence[i] = strings.Repeat("x", maxFlowText)
	}
	if _, err := EncodeFlowsDeclaration(FlowsDeclaration{Dir: "flows", Links: []FlowTestLink{link}}); err == nil {
		t.Fatal("writer exceeded the reader's declaration byte cap")
	}
}

// FLP-V1-007 (partial): until the route and tier prerequisites land, a
// version 1 declaration must not enter the legacy narrowing path, even with
// acceptance fields and a high score. FULL is the existing runner fallback.
func TestFLPV1007PendingIntegrationNeverNarrows(t *testing.T) {
	f := flowPlanRepo(t, true)
	_, control := f.plan(t, "src/checkout.ts", "lib/lib.go")
	if control.E2E == nil || control.E2E.Scope != affected.E2EScopeBounded {
		t.Fatalf("version 0 control: %+v", control.E2E)
	}
	original, err := ReadFlowsDeclaration(f.root)
	if err != nil {
		t.Fatal(err)
	}
	for _, state := range []string{"absent", "accepted", "unaccepted"} {
		t.Run(state, func(t *testing.T) {
			declared := FlowsDeclaration{Dir: "flows", Kinds: original.Kinds}
			if state != "absent" {
				link := fixtureFlowTestLink()
				link.Confidence = 1
				if state == "unaccepted" {
					link.AcceptedBy = ""
				}
				declared.Links = []FlowTestLink{link}
			}
			body, err := EncodeFlowsDeclaration(declared)
			if err != nil {
				t.Fatal(err)
			}
			writeRaw(t, f.root, FlowsDeclarationPath, body)
			_, got := f.plan(t, "src/checkout.ts", "lib/lib.go")
			if !reflect.DeepEqual(control.Selected, got.Selected) {
				t.Fatal("version 1 changed non-E2E selections")
			}
			if got.E2E == nil || got.E2E.Scope != affected.E2EScopeFull || len(got.E2E.Selected) != 0 || len(got.E2E.Unknown) != 1 || got.E2E.Unknown[0].Reason != affected.UnknownE2EInventoryUnderivable {
				t.Fatalf("pending integration narrowed or trusted a partial inventory: %+v", got.E2E)
			}
		})
	}
}
