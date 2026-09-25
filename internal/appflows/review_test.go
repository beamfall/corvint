package appflows

import (
	"context"
	"reflect"
	"strings"
	"testing"
)

// reviewSet loads the fixture intents with reviewed_at set on the declared app.js span link.
func reviewSet(t *testing.T, root, anchor string) IntentSet {
	t.Helper()
	set, err := LoadIntentsAt(context.Background(), root, "flows", "HEAD")
	if err != nil {
		t.Fatal(err)
	}
	set.Flows[0].Links[1].ReviewedAt = anchor
	return set
}

func spanState(t *testing.T, root, anchor, revision string) EvaluatedLink {
	t.Helper()
	links, err := EvaluateLinks(context.Background(), root, reviewSet(t, root, anchor), revision)
	if err != nil {
		t.Fatal(err)
	}
	return links[1]
}

// AFU-V1-007 AFU-V1-008
func TestAFUV1ReviewAnchorValidAndStale(t *testing.T) {
	root := intentRepo(t)
	anchor := gitOut(t, root, "rev-parse", "HEAD")
	link := spanState(t, root, anchor, "HEAD")
	if link.ReviewState != ReviewReviewed || link.Basis != "reviewed" || link.ReviewAttestation != "self" {
		t.Fatalf("valid anchor not reviewed: %+v", link)
	}
	if link.Revision != anchor || link.Target.Path != "app.js" || link.Target.StartLine != 1 || link.Blob == "" {
		t.Fatalf("link lacks target identity or evaluated revision: %+v", link)
	}
	writeRaw(t, root, "app.js", []byte("function pay() {}\nconst unrelated = 2\n"))
	outside := commitAll(t, root, "edit outside the span")
	if got := spanState(t, root, anchor, outside); got.ReviewState != ReviewReviewed {
		t.Fatalf("edit outside the reviewed span made the link %s", got.ReviewState)
	}
	writeRaw(t, root, "app.js", []byte("function pay(amount) {}\nconst unrelated = 2\n"))
	inside := commitAll(t, root, "edit the reviewed span")
	got := spanState(t, root, anchor, inside)
	if got.ReviewState != ReviewStale || got.Basis != "declared" || got.ReviewAttestation != "" {
		t.Fatalf("changed target not stale: %+v", got)
	}
}

// AFU-V1-008
func TestAFUV1ReviewAnchorNotAncestor(t *testing.T) {
	root := intentRepo(t)
	base := gitOut(t, root, "rev-parse", "HEAD")
	gitTest(t, root, "checkout", "-qb", "side")
	writeIntent(t, root, "flows/checkout.json", func() FlowIntent { f := sampleIntent("checkout"); f.Revision = 2; return f }())
	side := commitAll(t, root, "side review")
	gitTest(t, root, "checkout", "-q", base)
	if got := spanState(t, root, side, base); got.ReviewState != ReviewNotAncestor {
		t.Fatalf("non-ancestor anchor: %+v", got)
	}
	if got := spanState(t, root, strings.Repeat("e", 40), base); got.ReviewState != ReviewAnchorUnknown {
		t.Fatalf("unknown anchor: %+v", got)
	}
}

// AFU-V1-008
func TestAFUV1ReviewAnchorMustChangeIntent(t *testing.T) {
	root := intentRepo(t)
	writeRaw(t, root, "notes.txt", []byte("unrelated\n"))
	unrelated := commitAll(t, root, "does not touch the intent")
	if got := spanState(t, root, unrelated, "HEAD"); got.ReviewState != ReviewNotIntentChange || got.Basis == "reviewed" {
		t.Fatalf("anchor that did not change the intent: %+v", got)
	}
}

// AFU-V1-009 AFU-V1-008
func TestAFUV1InferredExcludedFromReviewed(t *testing.T) {
	root := intentRepo(t)
	anchor := gitOut(t, root, "rev-parse", "HEAD")
	set := reviewSet(t, root, anchor)
	links, err := EvaluateLinks(context.Background(), root, set, "HEAD")
	if err != nil {
		t.Fatal(err)
	}
	if links[2].ReviewState != ReviewInferred || links[2].Basis != "inferred" {
		t.Fatalf("inferred link promoted: %+v", links[2])
	}
	s := Summarize(anchor, links)
	if s.Reviewed != 1 || s.ReviewedDenominator != 2 || s.Inferred != 1 || s.ReviewAttestation != "self" || !strings.Contains(s.Limitation, "not verified") {
		t.Fatalf("summary counted inferred links or lost the self attestation: %+v", s)
	}
	set.Flows[0].Links[2].ReviewedAt = anchor
	if err := ValidateIntent(set.Flows[0]); err == nil {
		t.Fatal("an inferred link accepted a review anchor")
	}
}

// AFU-V1-010
func TestAFUV1ReverseLookupsDerived(t *testing.T) {
	root := intentRepo(t)
	before := treeSnapshot(t, root)
	links, err := EvaluateLinks(context.Background(), root, reviewSet(t, root, ""), "HEAD")
	if err != nil {
		t.Fatal(err)
	}
	wantPath := []ReverseHit{{Flow: "checkout", From: "pay", Basis: "declared", ReviewState: ReviewUnreviewed}, {Flow: "checkout", From: "pay", Basis: "inferred", ReviewState: ReviewInferred}}
	if byPath := FlowsForPath(links, "app.js"); !reflect.DeepEqual(byPath, wantPath) {
		t.Fatalf("path lookup: %+v", byPath)
	}
	wantTest := []ReverseHit{{Flow: "checkout", From: "checkout.happy", Basis: "declared", ReviewState: ReviewUnreviewed}}
	if byTest := FlowsForTestKey(links, "checkout.spec.ts > pays"); !reflect.DeepEqual(byTest, wantTest) {
		t.Fatalf("test key lookup: %+v", byTest)
	}
	if len(FlowsForPath(links, "missing.js")) != 0 {
		t.Fatal("lookup invented a flow")
	}
	if after := treeSnapshot(t, root); !reflect.DeepEqual(before, after) {
		t.Fatal("reverse lookup stored state in the repository")
	}
}

// AFU-V1-008
func TestAFUV1ReviewLinkMustExistAtAnchor(t *testing.T) {
	root := intentRepo(t)
	anchor := gitOut(t, root, "rev-parse", "HEAD")
	set := reviewSet(t, root, anchor)
	set.Flows[0].Links = append(set.Flows[0].Links, FlowLink{From: "open", Basis: "declared", ReviewedAt: anchor,
		Target: LinkTarget{Type: "source", Path: "app.js", StartLine: 2, EndLine: 2}})
	links, err := EvaluateLinks(context.Background(), root, set, "HEAD")
	if err != nil {
		t.Fatal(err)
	}
	if links[1].ReviewState != ReviewReviewed || links[3].ReviewState != ReviewLinkNotAtAnchor || links[3].Basis != "declared" {
		t.Fatalf("a link absent from the intent at the anchor was reviewed: %+v", links[3])
	}
	relabelled := reviewSet(t, root, anchor)
	relabelled.Flows[0].Links[2].Basis, relabelled.Flows[0].Links[2].ReviewedAt = "declared", anchor
	links, err = EvaluateLinks(context.Background(), root, relabelled, "HEAD")
	if err != nil {
		t.Fatal(err)
	}
	if links[2].ReviewState != ReviewLinkNotAtAnchor {
		t.Fatalf("a link inferred at the anchor was reviewed after relabelling: %+v", links[2])
	}
}

// AFU-V1-008
func TestAFUV1ReviewEvidenceTargetUnavailable(t *testing.T) {
	root := intentRepo(t)
	f := sampleIntent("checkout")
	f.Revision = 2
	f.Links = append(f.Links, FlowLink{From: "pay", Basis: "declared", Target: LinkTarget{Type: "evidence", Digest: strings.Repeat("a", 64)}})
	writeIntent(t, root, "flows/checkout.json", f)
	anchor := commitAll(t, root, "declare an evidence link")
	set := reviewSet(t, root, anchor)
	set.Flows[0].Links[3].ReviewedAt = anchor
	links, err := EvaluateLinks(context.Background(), root, set, "HEAD")
	if err != nil {
		t.Fatal(err)
	}
	if links[3].ReviewState != ReviewEvidenceUnavailable || links[3].Basis != "declared" {
		t.Fatalf("an evidence target not shown to exist was reviewed: %+v", links[3])
	}
}

// AFU-V1-008
func TestAFUV1ReviewContentIdentityRestoredTarget(t *testing.T) {
	root := intentRepo(t)
	anchor := gitOut(t, root, "rev-parse", "HEAD")
	writeRaw(t, root, "app.js", []byte("function pay(amount) {}\nconst unrelated = 1\n"))
	changed := commitAll(t, root, "change the reviewed span")
	writeRaw(t, root, "app.js", []byte("function pay() {}\nconst unrelated = 1\n"))
	restored := commitAll(t, root, "restore the reviewed span")
	if got := spanState(t, root, anchor, changed); got.ReviewState != ReviewStale {
		t.Fatalf("changed span: %+v", got)
	}
	if got := spanState(t, root, anchor, restored); got.ReviewState != ReviewReviewed {
		t.Fatalf("A to B to A is unchanged by content identity: %+v", got)
	}
}
