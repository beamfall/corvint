package testvaliditydoc

import (
	"bytes"
	"encoding/json"
	"reflect"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/jstestprovider"
)

// anchorTestJSON is one receipt test whose single attempt carries the given request-outcome record
// (empty for none).
func anchorTestJSON(name, state, record string) string {
	attempt := `"state":"` + state + `","retry":0,"durationMs":1`
	if record != "" {
		attempt += `,"requestOutcomes":` + record
	}
	return `{"name":"` + name + `","fullName":"` + name + `","state":"` + state + `","attemptDetails":[{` + attempt + `}]}`
}

func anchorDocument(t *testing.T, tests ...string) Document {
	t.Helper()
	return projectDocument(t, `{"receipt":{"kind":"e2e","tests":[`+strings.Join(tests, ",")+`]},"testProjections":[],"runProjection":{}}`)
}

const (
	forbiddenWrongRecord   = `{"assertion":"rejection-only","actor":"tenant-a","requests":[{"method":"GET","route":"/api/records/:id","status":403}]}`
	forbiddenMissingRecord = `{"assertion":"rejection-only","actor":"tenant-a","requests":[{"method":"GET","route":"/api/records/:id","status":403}]}`
	allowedRejected        = `{"assertion":"positive","actor":"tenant-a","requests":[{"method":"GET","route":"/api/records/:id","status":403}]}`
	allowedSucceeded       = `{"assertion":"positive","actor":"tenant-a","requests":[{"method":"GET","route":"/api/records/:id","status":200}]}`
)

func anchorOf(t *testing.T, document Document, index int) RejectionAnchor {
	t.Helper()
	if document.Tests[index].RejectionAnchor == nil {
		t.Fatalf("test %d has no rejectionAnchor", index)
	}
	return *document.Tests[index].RejectionAnchor
}

// LPCV-V0-082 LPCV-V0-083: issue #1099 acceptance 1. The positive test on the route fails with 403
// while two rejection-only tests on it pass with 403: both are ANCHOR-FAILED and block credit.
func TestLPCVV0082PositiveSiblingFailedBlocksCredit(t *testing.T) {
	document := anchorDocument(t,
		anchorTestJSON("owner reads record", "failed", allowedRejected),
		anchorTestJSON("other tenant record is forbidden", "passed", forbiddenWrongRecord),
		anchorTestJSON("missing record is forbidden", "passed", forbiddenMissingRecord),
	)
	for _, index := range []int{1, 2} {
		got := anchorOf(t, document, index)
		want := RejectionAnchor{State: AnchorFailed, Reason: AnchorObservedSiblingFailed, BlocksCredit: true, Siblings: []string{"owner reads record"}}
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("test %d anchor = %+v, want %+v", index, got, want)
		}
	}
	if got := anchorOf(t, document, 0); got.State != AnchorNotApplicable || got.Reason != AnchorNotRejectionOnly {
		t.Fatalf("positive test anchor = %+v", got)
	}
}

// LPCV-V0-082: issue #1099 acceptance 2. The same run with the positive test passing anchors both.
func TestLPCVV0082PositiveSiblingPassedAnchors(t *testing.T) {
	document := anchorDocument(t,
		anchorTestJSON("owner reads record", "passed", allowedSucceeded),
		anchorTestJSON("other tenant record is forbidden", "passed", forbiddenWrongRecord),
		anchorTestJSON("missing record is forbidden", "passed", forbiddenMissingRecord),
	)
	for _, index := range []int{1, 2} {
		got := anchorOf(t, document, index)
		want := RejectionAnchor{State: AnchorAnchored, Reason: AnchorSuccessOnRoute, Siblings: []string{"owner reads record"}}
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("test %d anchor = %+v, want %+v", index, got, want)
		}
	}
}

// LPCV-V0-082: issue #1099 acceptance 3. A rejection-only test with no success on its route is
// UNANCHORED; declaring a positive sibling whose own record shows a success on the route for the
// same actor clears it, and a declared sibling that failed blocks it.
func TestLPCVV0082UnanchoredClearedByDeclaredSibling(t *testing.T) {
	alone := anchorDocument(t,
		anchorTestJSON("forbidden", "passed", forbiddenWrongRecord),
		anchorTestJSON("unrelated", "passed", `{"assertion":"positive","actor":"tenant-a","requests":[{"method":"GET","route":"/api/other","status":200}]}`),
	)
	if got := anchorOf(t, alone, 0); got.State != AnchorUnanchored || got.Reason != AnchorNoSuccessOnRoute || got.BlocksCredit {
		t.Fatalf("lone rejection = %+v", got)
	}
	declared := strings.Replace(forbiddenWrongRecord, `"actor"`, `"positiveSibling":"owner reads record","actor"`, 1)
	cleared := anchorDocument(t,
		anchorTestJSON("forbidden", "passed", declared),
		anchorTestJSON("owner reads record", "passed", allowedSucceeded),
	)
	if got := anchorOf(t, cleared, 0); !reflect.DeepEqual(got, RejectionAnchor{State: AnchorAnchored, Reason: AnchorDeclaredSiblingPassed, Siblings: []string{"owner reads record"}}) {
		t.Fatalf("declared passing sibling = %+v", got)
	}
	blocked := anchorDocument(t,
		anchorTestJSON("forbidden", "passed", declared),
		anchorTestJSON("owner reads record", "failed", ""),
	)
	if got := anchorOf(t, blocked, 0); !reflect.DeepEqual(got, RejectionAnchor{State: AnchorFailed, Reason: AnchorDeclaredSiblingFailed, BlocksCredit: true, Siblings: []string{"owner reads record"}}) {
		t.Fatalf("declared failing sibling = %+v", got)
	}
	notRun := anchorDocument(t, anchorTestJSON("forbidden", "passed", declared), anchorTestJSON("owner reads record", "skipped", ""))
	if got := anchorOf(t, notRun, 0); got.State != AnchorUnanchored || got.Reason != AnchorDeclaredSiblingMissing {
		t.Fatalf("declared skipped sibling = %+v", got)
	}
}

// LPCV-V0-082: a declared sibling anchors only through its own positive evidence. A passed sibling
// without a record, or whose record shows no success for the same actor on the same keys, is
// UNKNOWN; two rejection-only tests that declare each other stay UNANCHORED.
func TestLPCVV0082DeclaredSiblingNeedsPositiveEvidence(t *testing.T) {
	declared := strings.Replace(forbiddenWrongRecord, `"actor"`, `"positiveSibling":"owner reads record","actor"`, 1)
	for name, sibling := range map[string]struct {
		record string
		want   RejectionAnchor
	}{
		"no record":         {"", RejectionAnchor{State: AnchorUnknown, Reason: AnchorDeclaredSiblingNoEvidence}},
		"other route":       {`{"assertion":"positive","actor":"tenant-a","requests":[{"method":"GET","route":"/api/other","status":200}]}`, RejectionAnchor{State: AnchorUnknown, Reason: AnchorDeclaredSiblingNoEvidence}},
		"other actor":       {`{"assertion":"positive","actor":"tenant-b","requests":[{"method":"GET","route":"/api/records/:id","status":200}]}`, RejectionAnchor{State: AnchorUnknown, Reason: AnchorDeclaredSiblingNoEvidence}},
		"positive with 403": {allowedRejected, RejectionAnchor{State: AnchorUnknown, Reason: AnchorDeclaredSiblingNoEvidence}},
		"mixed with 200":    {`{"assertion":"mixed","actor":"tenant-a","requests":[{"method":"GET","route":"/api/records/:id","status":200}]}`, RejectionAnchor{State: AnchorAnchored, Reason: AnchorDeclaredSiblingPassed, Siblings: []string{"owner reads record"}}},
	} {
		document := anchorDocument(t, anchorTestJSON("forbidden", "passed", declared), anchorTestJSON("owner reads record", "passed", sibling.record))
		if got := anchorOf(t, document, 0); !reflect.DeepEqual(got, sibling.want) {
			t.Errorf("%s: anchor = %+v, want %+v", name, got, sibling.want)
		}
	}
	mutualA := strings.Replace(forbiddenWrongRecord, `"actor"`, `"positiveSibling":"b","actor"`, 1)
	mutualB := strings.Replace(forbiddenWrongRecord, `"actor"`, `"positiveSibling":"a","actor"`, 1)
	mutual := anchorDocument(t, anchorTestJSON("a", "passed", mutualA), anchorTestJSON("b", "passed", mutualB))
	for i := range mutual.Tests {
		if got := anchorOf(t, mutual, i); got.State != AnchorUnanchored || got.Reason != AnchorDeclaredSiblingRejectionOnly {
			t.Errorf("mutual rejection-only %d = %+v", i, got)
		}
	}
}

// LPCV-V0-082: the Playwright adapter names a test by its own title only, so two suites with the
// same test title make a declared sibling ambiguous; it is refused, never picked.
func TestLPCVV0082AmbiguousDeclaredSiblingIsUnknown(t *testing.T) {
	declared := strings.Replace(forbiddenWrongRecord, `"actor"`, `"positiveSibling":"owner reads record","actor"`, 1)
	for name, states := range map[string][2]string{
		"both passed": {"passed", "passed"},
		"one failed":  {"passed", "failed"},
		"one not run": {"passed", "skipped"},
	} {
		document := anchorDocument(t,
			anchorTestJSON("forbidden", "passed", declared),
			anchorTestJSON("owner reads record", states[0], ""),
			anchorTestJSON("owner reads record", states[1], ""),
		)
		if got := anchorOf(t, document, 0); !reflect.DeepEqual(got, RejectionAnchor{State: AnchorUnknown, Reason: AnchorDeclaredSiblingAmbiguous}) {
			t.Errorf("%s: anchor = %+v", name, got)
		}
	}
	report := `{"suites":[{"title":"a.spec.ts","file":"a.spec.ts","suites":[` +
		playwrightSuiteJSON("admin", "owner reads record", "") + `,` +
		playwrightSuiteJSON("member", "owner reads record", "") + `,` +
		playwrightSuiteJSON("guest", "forbidden", strings.ReplaceAll(declared, `"`, `\"`)) + `]}],"errors":[]}`
	outcomes, _, err := jstestprovider.ParsePlaywrightJSON([]byte(report))
	if err != nil || len(outcomes) != 3 {
		t.Fatalf("outcomes=%d err=%v", len(outcomes), err)
	}
	document := receiptDocument(t, jstestprovider.Receipt{Kind: "e2e", Tests: outcomes})
	if got := anchorOf(t, document, 2); got.State != AnchorUnknown || got.Reason != AnchorDeclaredSiblingAmbiguous {
		t.Fatalf("two suites with one title: %+v", got)
	}
}

// LPCV-V0-081 LPCV-V0-082: screening collapses distinct secret-bearing actors and routes to one
// placeholder, so screened text never joins: the rejection is UNKNOWN, never ANCHORED. A retained
// receipt that still carries an unscreened secret is treated the same way.
func TestLPCVV0081ScreenedEvidenceNeverJoins(t *testing.T) {
	report := `{"suites":[{"title":"a.spec.ts","file":"a.spec.ts","suites":[` +
		playwrightSuiteJSON("s1", "forbidden", `{\"assertion\":\"rejection-only\",\"actor\":\"ghp_aaaaaaaaaaaaaaaaaaaaaaaa\",\"requests\":[{\"method\":\"GET\",\"route\":\"/r\",\"status\":403}]}`) + `,` +
		playwrightSuiteJSON("s2", "allowed", `{\"assertion\":\"positive\",\"actor\":\"ghp_bbbbbbbbbbbbbbbbbbbbbbbb\",\"requests\":[{\"method\":\"GET\",\"route\":\"/r\",\"status\":200}]}`) + `]}],"errors":[]}`
	outcomes, _, err := jstestprovider.ParsePlaywrightJSON([]byte(report))
	if err != nil || len(outcomes) != 2 {
		t.Fatalf("outcomes=%d err=%v", len(outcomes), err)
	}
	document := receiptDocument(t, jstestprovider.Receipt{Kind: "e2e", Tests: outcomes})
	if got := anchorOf(t, document, 0); !reflect.DeepEqual(got, RejectionAnchor{State: AnchorUnknown, Reason: AnchorActorScreened}) {
		t.Fatalf("distinct screened actors = %+v", got)
	}
	routes := anchorDocument(t,
		anchorTestJSON("forbidden", "passed", `{"assertion":"rejection-only","actor":"tenant-a","requests":[{"method":"GET","route":"/r?key=[REDACTED]","status":403}]}`),
		anchorTestJSON("allowed", "passed", `{"assertion":"positive","actor":"tenant-a","requests":[{"method":"GET","route":"/r?key=[REDACTED]","status":200}]}`),
	)
	if got := anchorOf(t, routes, 0); !reflect.DeepEqual(got, RejectionAnchor{State: AnchorUnknown, Reason: AnchorRouteScreened}) {
		t.Fatalf("screened route = %+v", got)
	}
	unscreened := anchorDocument(t,
		anchorTestJSON("forbidden", "passed", `{"assertion":"rejection-only","actor":"ghp_aaaaaaaaaaaaaaaaaaaaaaaa","requests":[{"method":"GET","route":"/r","status":403}]}`),
		anchorTestJSON("allowed", "passed", `{"assertion":"positive","actor":"ghp_aaaaaaaaaaaaaaaaaaaaaaaa","requests":[{"method":"GET","route":"/r","status":200}]}`),
	)
	if got := anchorOf(t, unscreened, 0); !reflect.DeepEqual(got, RejectionAnchor{State: AnchorUnknown, Reason: AnchorActorScreened}) {
		t.Fatalf("unscreened secret actor = %+v", got)
	}
	siblingScreened := anchorDocument(t,
		anchorTestJSON("forbidden", "passed", `{"assertion":"rejection-only","actor":"tenant-a","requests":[{"method":"GET","route":"/r","status":403}]}`),
		anchorTestJSON("allowed", "passed", `{"assertion":"positive","actor":"[REDACTED]","requests":[{"method":"GET","route":"/r","status":200}]}`),
	)
	if got := anchorOf(t, siblingScreened, 0); got.State == AnchorAnchored {
		t.Fatalf("screened sibling anchored a clean actor: %+v", got)
	}
	screenedTitle := anchorDocument(t,
		anchorTestJSON("forbidden", "passed", `{"assertion":"rejection-only","positiveSibling":"[REDACTED]","actor":"tenant-a","requests":[{"method":"GET","route":"/r","status":403}]}`),
		anchorTestJSON("[REDACTED]", "passed", `{"assertion":"positive","actor":"tenant-a","requests":[{"method":"GET","route":"/other","status":200}]}`),
	)
	if got := anchorOf(t, screenedTitle, 0); !reflect.DeepEqual(got, RejectionAnchor{State: AnchorUnknown, Reason: AnchorDeclaredSiblingAmbiguous}) {
		t.Fatalf("screened sibling title = %+v", got)
	}
}

// playwrightSuiteJSON is one Playwright suite holding one passed spec, with an optional
// request-outcome annotation description (already escaped for the report string).
func playwrightSuiteJSON(suite, title, description string) string {
	annotations := `[]`
	if description != "" {
		annotations = `[{"type":"corvint-request-outcomes","description":"` + description + `"}]`
	}
	return `{"title":"` + suite + `","file":"a.spec.ts","specs":[{"title":"` + title + `","ok":true,"tests":[{"status":"expected","results":[{"status":"passed","retry":0,"duration":1,"annotations":` + annotations + `}]}]}]}`
}

func receiptDocument(t *testing.T, receipt jstestprovider.Receipt) Document {
	t.Helper()
	raw, err := json.Marshal(receipt)
	if err != nil {
		t.Fatal(err)
	}
	return projectDocument(t, `{"receipt":`+string(raw)+`,"testProjections":[],"runProjection":{}}`)
}

// LPCV-V0-081 LPCV-V0-082: missing route or actor evidence yields uncertainty, never ANCHORED; a
// success for another actor, method or route is no anchor; tests without a usable record state why.
func TestLPCVV0081MissingEvidenceIsNeverAnchored(t *testing.T) {
	document := anchorDocument(t,
		anchorTestJSON("ui denied", "passed", `{"assertion":"rejection-only","actor":"tenant-a","requests":[]}`),
		anchorTestJSON("no actor", "passed", `{"assertion":"rejection-only","requests":[{"method":"GET","route":"/api/records/:id","status":403}]}`),
		anchorTestJSON("success without actor", "passed", `{"assertion":"positive","requests":[{"method":"GET","route":"/api/records/:id","status":200}]}`),
		anchorTestJSON("other actor success", "passed", `{"assertion":"positive","actor":"tenant-b","requests":[{"method":"GET","route":"/api/records/:id","status":200}]}`),
		anchorTestJSON("other method success", "passed", `{"assertion":"positive","actor":"tenant-a","requests":[{"method":"PUT","route":"/api/records/:id","status":200}]}`),
		anchorTestJSON("forbidden", "passed", forbiddenWrongRecord),
		anchorTestJSON("no record", "passed", ""),
		anchorTestJSON("failed rejection", "failed", forbiddenWrongRecord),
	)
	want := []RejectionAnchor{
		{State: AnchorUnknown, Reason: AnchorRouteEvidenceMissing},
		{State: AnchorUnknown, Reason: AnchorActorEvidenceMissing},
		{State: AnchorNotApplicable, Reason: AnchorNotRejectionOnly},
		{State: AnchorNotApplicable, Reason: AnchorNotRejectionOnly},
		{State: AnchorNotApplicable, Reason: AnchorNotRejectionOnly},
		{State: AnchorUnanchored, Reason: AnchorNoSuccessOnRoute},
		{State: AnchorNotProduced, Reason: AnchorRecordAbsent},
		{State: AnchorNotApplicable, Reason: AnchorTestNotPassed},
	}
	for i, expected := range want {
		if got := anchorOf(t, document, i); !reflect.DeepEqual(got, expected) {
			t.Errorf("test %d (%s) = %+v, want %+v", i, document.Tests[i].Name, got, expected)
		}
	}
}

// LPCV-V0-081: a receipt without any record keeps every byte, and the member changes no other
// member when present.
func TestLPCVV0081ReceiptWithoutRecordKeepsBytes(t *testing.T) {
	legacy := anchorDocument(t, anchorTestJSON("a", "passed", ""))
	encoded, err := json.Marshal(legacy)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(encoded), "rejectionAnchor") {
		t.Fatalf("legacy document gained rejectionAnchor: %s", encoded)
	}
	withRecord := anchorDocument(t, anchorTestJSON("a", "passed", forbiddenWrongRecord))
	withRecord.Tests[0].RejectionAnchor = nil
	again, err := json.Marshal(withRecord)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(encoded, again) {
		t.Fatalf("rejectionAnchor changed another member:\n%s\n%s", encoded, again)
	}
}

// LPCV-V0-083: ANCHOR-FAILED never rewrites the five-axis projection of the test or the run; the
// block is carried only by the member.
func TestLPCVV0083AnchorLeavesAxesUnchanged(t *testing.T) {
	plain := anchorDocument(t, anchorTestJSON("owner reads record", "failed", ""), anchorTestJSON("forbidden", "passed", ""))
	anchored := anchorDocument(t, anchorTestJSON("owner reads record", "failed", allowedRejected), anchorTestJSON("forbidden", "passed", forbiddenWrongRecord))
	if got := anchorOf(t, anchored, 1); !got.BlocksCredit {
		t.Fatalf("no block: %+v", got)
	}
	for i := range plain.Tests {
		if !reflect.DeepEqual(plain.Tests[i].Projection, anchored.Tests[i].Projection) {
			t.Fatalf("test %d projection rewritten:\n%+v\n%+v", i, plain.Tests[i].Projection, anchored.Tests[i].Projection)
		}
	}
	if !reflect.DeepEqual(plain.Run, anchored.Run) {
		t.Fatalf("run projection rewritten")
	}
}

// LPCV-V0-080: a retained receipt whose record breaks the closed decoding is refused, and one whose
// rejection-only record names a non-4xx status is NOT_PRODUCED, never classified.
func TestLPCVV0080RetainedRecordIsClosed(t *testing.T) {
	for name, record := range map[string]string{
		"missing status":   `{"assertion":"positive","requests":[{"method":"GET","route":"/a"}]}`,
		"null status":      `{"assertion":"positive","requests":[{"method":"GET","route":"/a","status":null}]}`,
		"duplicate status": `{"assertion":"positive","requests":[{"method":"GET","route":"/a","status":403,"status":200}]}`,
		"unknown member":   `{"assertion":"positive","requests":[],"anchor":"x"}`,
	} {
		body := `{"receipt":{"kind":"e2e","tests":[` + anchorTestJSON("a", "passed", record) + `]},"testProjections":[],"runProjection":{}}`
		if _, err := Decode([]byte(body)); err == nil {
			t.Errorf("%s: retained record accepted", name)
		}
	}
	invalid := anchorDocument(t, anchorTestJSON("a", "passed", `{"assertion":"rejection-only","actor":"x","requests":[{"method":"GET","route":"/a","status":200}]}`))
	if got := anchorOf(t, invalid, 0); got.State != AnchorNotProduced || got.Reason != AnchorRecordInvalid {
		t.Fatalf("contradictory record = %+v", got)
	}
}
