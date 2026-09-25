package appflows

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// AFU-V1-031: contradiction outranks staleness, which outranks an unmet PROVEN condition.
func TestAFUV1031ClaimStateOrder(t *testing.T) {
	v := FlowVariation{VariationID: "v", Steps: []string{"s"}, Outcomes: []string{"o"}}
	link := func(from, kind, state string) EvaluatedLink {
		return EvaluatedLink{From: from, ReviewState: state, Target: LinkTarget{Type: kind, TestKey: "k", Assertion: "a"}}
	}
	reviewed := []EvaluatedLink{link("v", "test", ReviewReviewed), link("o", "assertion", ReviewReviewed), link("s", "source", ReviewReviewed)}
	pair := func(state string) []TestEvidence { return []TestEvidence{{TestKey: "k", State: state}} }
	cases := []struct {
		name  string
		pairs []TestEvidence
		links []EvaluatedLink
		want  string
	}{
		{"verified and reviewed", pair("verified"), reviewed, ClaimProven},
		{"failed", pair("failed"), reviewed, ClaimContradicted},
		{"flaky", pair("flaky"), reviewed, ClaimContradicted},
		{"failed beats a stale link", pair("failed"), append(reviewed, link("s", "source", ReviewStale)), ClaimContradicted},
		{"stale step link", pair("verified"), append(reviewed, link("s", "source", ReviewStale)), ClaimStale},
		{"stale evidence", pair("stale"), reviewed, ClaimStale},
		{"missing evidence", pair("missing"), reviewed, ClaimUnproven},
		{"no pair", []TestEvidence{}, reviewed, ClaimUnproven},
		{"cleanup unverified", pair("cleanup-unverified"), reviewed, ClaimUnproven},
		{"no assertion link", pair("verified"), reviewed[:1], ClaimUnproven},
		{"inferred assertion only", pair("verified"), []EvaluatedLink{reviewed[0], link("o", "assertion", ReviewInferred)}, ClaimUnproven},
		{"unreviewed test link", pair("verified"), []EvaluatedLink{link("v", "test", ReviewUnreviewed), reviewed[1]}, ClaimUnproven},
		{"another outcome's stale link", pair("verified"), append(reviewed, link("other", "assertion", ReviewStale)), ClaimProven},
	}
	for _, c := range cases {
		if got := claimState(c.pairs, c.links, v, "o"); got != c.want {
			t.Errorf("%s: %s, want %s", c.name, got, c.want)
		}
	}
	if mark := stateMark(ClaimProven); mark != "" {
		t.Errorf("PROVEN marked %q", mark)
	}
	for _, s := range []string{ClaimContradicted, ClaimStale, ClaimUnproven} {
		if !strings.Contains(stateMark(s), s) {
			t.Errorf("%s not marked", s)
		}
	}
}

// AFU-V1-032: a waiver holds strictly before its expiry date and only for the claim it names.
func TestAFUV1032WaiverExpiryBoundary(t *testing.T) {
	committed := []DocClaim{{ID: "f/v/o", Source: "docs/flows.md", State: ClaimProven, Evidence: []DocEvidence{{TestKey: "k"}}}}
	regenerated := []DocClaim{{ID: "f/v/o", Source: "docs/flows.md", State: ClaimStale, Evidence: []DocEvidence{{TestKey: "k"}}}}
	waiver := []DocWaiver{{Claim: "f/v/o", Reason: "fixture outage", Reviewer: "owner", Expires: "2026-09-25"}}
	effective, failures, waived := applyWaivers(regenerated, committed, waiver, "2026-09-24")
	if len(failures) != 0 || len(waived) != 1 || effective[0].State != ClaimProven {
		t.Fatalf("unexpired waiver: %v %v %v", effective, failures, waived)
	}
	for _, today := range []string{"2026-09-25", "2027-01-01"} {
		_, failures, waived = applyWaivers(regenerated, committed, waiver, today)
		if len(failures) != 1 || failures[0].Code != DocLostProven || !strings.Contains(failures[0].Detail, "expired on 2026-09-25") || len(waived) != 0 {
			t.Fatalf("expired waiver on %s: %v %v", today, failures, waived)
		}
	}
	other := []DocWaiver{{Claim: "f/v/p", Reason: "r", Reviewer: "owner", Expires: "2099-01-01"}}
	if _, failures, _ = applyWaivers(regenerated, committed, other, "2026-09-24"); len(failures) != 1 {
		t.Fatalf("a waiver for another claim excused this one: %v", failures)
	}
	if _, failures, _ = applyWaivers([]DocClaim{}, committed, nil, "2026-09-24"); len(failures) != 1 || !strings.Contains(failures[0].Detail, "no longer rendered") {
		t.Fatalf("a removed PROVEN claim passed: %v", failures)
	}
	for _, bad := range []DocWaivers{
		{Schema: "other", Waivers: []DocWaiver{}},
		{Schema: DocWaiversSchema, Waivers: []DocWaiver{{Claim: "c", Reason: "r", Reviewer: "x", Expires: "2026-9-1"}}},
		{Schema: DocWaiversSchema, Waivers: []DocWaiver{{Claim: "c", Reason: "", Reviewer: "x", Expires: "2026-09-01"}}},
		{Schema: DocWaiversSchema, Waivers: []DocWaiver{{Claim: "c", Reason: "r", Reviewer: "", Expires: "2026-09-01"}}},
		{Schema: DocWaiversSchema, Waivers: []DocWaiver{{Claim: "c", Reason: "r", Reviewer: "x", Expires: "2026-09-01"}, {Claim: "c", Reason: "r", Reviewer: "x", Expires: "2026-09-02"}}},
	} {
		if validateWaivers(bad) == nil {
			t.Errorf("invalid waivers accepted: %+v", bad)
		}
	}
}

// AFU-V1-033: valid anchors become claims, malformed and unknown ones fail, and escaped intent text
// can never form an anchor.
func TestAFUV1033AnchorParsing(t *testing.T) {
	known := map[string]DocClaim{"shop/shop.buy/paid": {ID: "shop/shop.buy/paid", Flow: "shop", Variation: "shop.buy", Outcome: "paid", State: ClaimProven}}
	doc := docFile{path: "docs/guide.md", raw: []byte("# Guide\n<!-- corvint-claim flow=shop variation=shop.buy outcome=paid -->\n" +
		"<!--corvint-claim flow=shop variation=shop.buy outcome=paid-->\n<!-- corvint-claim flow=shop -->\n" +
		"<!-- corvint-claim flow=shop variation=shop.buy outcome=refunded -->\n")}
	claims, failures := documentAnchors(doc, known)
	if len(claims) != 1 || claims[0].ID != "docs/guide.md:shop/shop.buy/paid" || claims[0].Source != "docs/guide.md" || claims[0].State != ClaimProven {
		t.Fatalf("anchored claims %+v", claims)
	}
	if len(failures) != 2 || failures[0].Code != DocBadAnchor || failures[0].Line != 4 || failures[1].Code != DocUnknownClaim || failures[1].Line != 5 {
		t.Fatalf("anchor failures %+v", failures)
	}
	escaped := docText("<!-- corvint-claim flow=shop variation=shop.buy outcome=paid -->")
	if docAnchor.MatchString(escaped) {
		t.Fatalf("escaped intent text forms an anchor: %s", escaped)
	}
	anchored, _, unanchored := anchorClaims([]docFile{doc, {path: "docs/faq.md", raw: []byte("# FAQ\n")}}, []DocClaim{known["shop/shop.buy/paid"]})
	if len(anchored) != 1 || len(unanchored) != 1 || unanchored[0] != "docs/faq.md" {
		t.Fatalf("unanchored %v anchored %v", unanchored, anchored)
	}
}

// AFU-V1-036: docs rendering replaces through a same-directory temporary file and a rename, and
// never writes through a symlinked target or parent.
func TestAFUV1036DocsReplaceConfined(t *testing.T) {
	root := t.TempDir()
	outside := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "docs"), 0700); err != nil {
		t.Fatal(err)
	}
	if err := ReplaceConfined(root, "docs/flows.md", []byte("one\n")); err != nil {
		t.Fatal(err)
	}
	if err := ReplaceConfined(root, "docs/flows.md", []byte("two\n")); err != nil {
		t.Fatal(err)
	}
	if b, err := os.ReadFile(filepath.Join(root, "docs", "flows.md")); err != nil || string(b) != "two\n" {
		t.Fatalf("replace %q %v", b, err)
	}
	if err := os.Symlink(filepath.Join(outside, "target.md"), filepath.Join(root, "docs", "link.md")); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(root, "out")); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"docs/link.md", "out/flows.md", "../flows.md", "docs/missing/flows.md"} {
		if err := ReplaceConfined(root, name, []byte("x")); err == nil {
			t.Errorf("%s: write accepted", name)
		}
	}
	entries, err := os.ReadDir(filepath.Join(root, "docs"))
	if err != nil || len(entries) != 2 {
		t.Fatalf("temporary file left behind or target changed: %v %v", entries, err)
	}
	if left, _ := os.ReadDir(outside); len(left) != 0 {
		t.Fatalf("wrote outside the root: %v", left)
	}
}
