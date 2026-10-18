package testvaliditydoc

import (
	"bytes"
	"encoding/json"
	"reflect"
	"strings"
	"testing"
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
// UNANCHORED; declaring a positive sibling that passed clears it, and one that failed blocks it.
func TestLPCVV0082UnanchoredClearedByDeclaredSibling(t *testing.T) {
	alone := anchorDocument(t,
		anchorTestJSON("forbidden", "passed", forbiddenWrongRecord),
		anchorTestJSON("unrelated", "passed", `{"assertion":"positive","actor":"tenant-a","requests":[{"method":"GET","route":"/api/other","status":200}]}`),
	)
	if got := anchorOf(t, alone, 0); got.State != AnchorUnanchored || got.Reason != AnchorNoSuccessOnRoute || got.BlocksCredit {
		t.Fatalf("lone rejection = %+v", got)
	}
	declared := strings.Replace(forbiddenWrongRecord, `"actor"`, `"positiveSibling":"owner sees settings page","actor"`, 1)
	cleared := anchorDocument(t,
		anchorTestJSON("forbidden", "passed", declared),
		anchorTestJSON("owner sees settings page", "passed", ""),
	)
	if got := anchorOf(t, cleared, 0); !reflect.DeepEqual(got, RejectionAnchor{State: AnchorAnchored, Reason: AnchorDeclaredSiblingPassed, Siblings: []string{"owner sees settings page"}}) {
		t.Fatalf("declared passing sibling = %+v", got)
	}
	blocked := anchorDocument(t,
		anchorTestJSON("forbidden", "passed", declared),
		anchorTestJSON("owner sees settings page", "failed", ""),
	)
	if got := anchorOf(t, blocked, 0); !reflect.DeepEqual(got, RejectionAnchor{State: AnchorFailed, Reason: AnchorDeclaredSiblingFailed, BlocksCredit: true, Siblings: []string{"owner sees settings page"}}) {
		t.Fatalf("declared failing sibling = %+v", got)
	}
	notRun := anchorDocument(t, anchorTestJSON("forbidden", "passed", declared), anchorTestJSON("owner sees settings page", "skipped", ""))
	if got := anchorOf(t, notRun, 0); got.State != AnchorUnanchored || got.Reason != AnchorDeclaredSiblingMissing {
		t.Fatalf("declared skipped sibling = %+v", got)
	}
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
