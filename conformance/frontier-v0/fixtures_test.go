// Copyright 2026 Russell Lewis
// Licensed under the Apache License, Version 2.0.

package main

import (
	"bytes"
	"fmt"
	"sort"
	"strings"
	"testing"
)

func loadFixturesT(t *testing.T) []Fixture {
	t.Helper()
	fx, err := LoadFixtures(".")
	if err != nil {
		t.Fatalf("load fixtures: %v", err)
	}
	if len(fx) == 0 {
		t.Fatal("no fixtures")
	}
	return fx
}

func TestFixturesValidate(t *testing.T) {
	for _, f := range loadFixturesT(t) {
		f := f
		t.Run(f.ID, func(t *testing.T) {
			if err := f.Validate(); err != nil {
				t.Fatal(err)
			}
		})
	}
}

// TestManifestCoversEveryLiveMatrixRow re-reads the matrix table out of the
// specification itself. The manifest is a snapshot; this is what makes it go
// stale loudly when the spec grows a row.
func TestManifestCoversEveryLiveMatrixRow(t *testing.T) {
	m, err := LoadManifest(".")
	if err != nil {
		t.Fatalf("load manifest: %v", err)
	}
	live, err := SpecMatrixRows(RepoRelativeSpecPath("."))
	if err != nil {
		t.Fatalf("read spec matrix: %v", err)
	}
	byRow := map[string]MatrixRow{}
	for _, r := range m.Matrix {
		byRow[r.MatrixRow] = r
	}
	fixtures := map[string]Fixture{}
	for _, f := range loadFixturesT(t) {
		fixtures[f.ID] = f
	}
	for _, lr := range live {
		row, ok := byRow[lr.MatrixRow]
		if !ok {
			t.Errorf("the spec declares matrix row %q with no manifest entry; author a fixture for it", lr.MatrixRow)
			continue
		}
		if row.RequiredResult != lr.RequiredResult {
			t.Errorf("row %q: the manifest records required result %q but the spec now says %q",
				lr.MatrixRow, row.RequiredResult, lr.RequiredResult)
		}
		if len(row.Fixtures) == 0 {
			t.Errorf("row %q is uncovered", lr.MatrixRow)
			continue
		}
		for _, id := range row.Fixtures {
			f, ok := fixtures[id]
			if !ok {
				t.Errorf("row %q names fixture %q, which does not exist", lr.MatrixRow, id)
				continue
			}
			found := false
			for _, mr := range f.MatrixRows {
				if mr == lr.MatrixRow {
					found = true
				}
			}
			if !found {
				t.Errorf("fixture %s does not itself claim row %q", id, lr.MatrixRow)
			}
		}
	}
	if len(m.Matrix) != len(live) {
		t.Errorf("manifest records %d rows, the spec has %d", len(m.Matrix), len(live))
	}
	// Every fixture's claimed rows must exist in the live spec verbatim, so a
	// fixture cannot invent coverage of a row nobody asked for.
	liveSet := map[string]bool{}
	for _, lr := range live {
		liveSet[lr.MatrixRow] = true
	}
	for _, f := range fixtures {
		for _, mr := range f.MatrixRows {
			if !liveSet[mr] {
				t.Errorf("fixture %s claims matrix row %q, which is not in the spec", f.ID, mr)
			}
		}
	}
}

// TestCanaryDeclarationsAreSelfConsistent checks the CF-V0-024 fixtures against
// themselves: a planted canary that is not also asserted absent proves nothing.
func TestCanaryDeclarationsAreSelfConsistent(t *testing.T) {
	planted := map[string]bool{}
	for _, f := range loadFixturesT(t) {
		for _, c := range f.Cases {
			forbidden := map[string]bool{}
			for _, s := range c.ForbiddenSubstrings {
				forbidden[s] = true
			}
			for _, can := range c.Declared.Canaries {
				planted[can.Where] = true
				if !forbidden[can.Value] {
					t.Errorf("%s/%s: canary %q is planted in %s but never asserted absent",
						f.ID, c.ID, can.Value, can.Where)
				}
			}
			// A forbidden substring must not appear in the expectation itself.
			blob := c.Expect.Stderr + c.Expect.ErrorCode + c.Expect.FrontierState
			for _, it := range c.Expect.Items {
				blob += it.Kind + it.SubjectID + it.AuthorityClass + it.ResolutionClass + it.NextAction
				blob += strings.Join(it.Reasons, " ") + strings.Join(it.RelatedIDs, " ")
			}
			for _, s := range c.ForbiddenSubstrings {
				if strings.Contains(blob, s) {
					t.Errorf("%s/%s: the expectation itself contains the forbidden substring %q",
						f.ID, c.ID, s)
				}
			}
		}
	}
	// CF-V0-024 names these bodies explicitly; each must be planted somewhere.
	for _, where := range []string{"source-body", "diff-body", "command-body", "report-body",
		"sibling-path", "exception-text"} {
		if !planted[where] {
			t.Errorf("no fixture plants a canary in %s, which CF-V0-024 names", where)
		}
	}
}

// TestEveryLimitHasAtNAndAtNPlusOne asserts the CF-V0-023 row is exhaustive.
func TestEveryLimitHasAtNAndAtNPlusOne(t *testing.T) {
	var limits Fixture
	for _, f := range loadFixturesT(t) {
		if f.ID == "resource-limits" {
			limits = f
		}
	}
	if limits.ID == "" {
		t.Fatal("the resource-limits fixture is missing")
	}
	byID := map[string]Case{}
	for _, c := range limits.Cases {
		byID[c.ID] = c
	}
	for _, l := range Limits {
		at := byID[l.Name+"-at-limit"]
		over := byID[l.Name+"-at-limit-plus-one"]
		if at.ID == "" {
			t.Errorf("no at-limit case for %s", l.Name)
		} else {
			if at.Expect.Kind != "result" {
				t.Errorf("%s at N must be a complete result, not %q", l.Name, at.Expect.Kind)
			}
			if got := at.Declared.Counts[l.Name]; got != fmt.Sprint(l.Value) {
				t.Errorf("%s at-limit declares count %q, want %d", l.Name, got, l.Value)
			}
		}
		if over.ID == "" {
			t.Errorf("no at-limit-plus-one case for %s", l.Name)
		} else {
			if over.Expect.Kind != "error" || over.Expect.ErrorCode != "frontier-resource-exhausted" {
				t.Errorf("%s at N+1 must fail frontier-resource-exhausted, got kind=%q code=%q",
					l.Name, over.Expect.Kind, over.Expect.ErrorCode)
			}
			if over.Expect.Stdout != "none" {
				t.Errorf("%s at N+1 must emit no partial result", l.Name)
			}
			if got := over.Declared.Counts[l.Name]; got != fmt.Sprint(l.Value+1) {
				t.Errorf("%s at-limit-plus-one declares count %q, want %d", l.Name, got, l.Value+1)
			}
		}
	}
	if len(limits.Cases) != 2*len(Limits) {
		t.Errorf("resource-limits has %d cases, want %d (one at N and one at N+1 per bound)",
			len(limits.Cases), 2*len(Limits))
	}
}

// TestNegativeRowsFailClosed pins the matrix rows whose whole point is that a
// permissive implementation would pass a naive test.
func TestNegativeRowsFailClosed(t *testing.T) {
	fx := map[string]Fixture{}
	for _, f := range loadFixturesT(t) {
		fx[f.ID] = f
	}
	// OPEN with empty items, EMPTY with items, and any extra stop-decision field
	// must all be noncanonical-frontier.
	nc := fx["noncanonical-frontier-refusals"]
	if nc.ID == "" {
		t.Fatal("the noncanonical-frontier-refusals fixture is missing")
	}
	needed := []string{"open-with-empty-items", "empty-with-items", "extra-stop-decision-field"}
	have := map[string]Case{}
	for _, c := range nc.Cases {
		have[c.ID] = c
	}
	for _, n := range needed {
		c, ok := have[n]
		if !ok {
			t.Errorf("noncanonical-frontier-refusals lacks case %q", n)
			continue
		}
		if c.Expect.ErrorCode != "noncanonical-frontier" {
			t.Errorf("case %q must yield noncanonical-frontier, got %q", n, c.Expect.ErrorCode)
		}
		if c.Expect.Stdout != "none" || c.Expect.ExitCode != 2 {
			t.Errorf("case %q must emit no stdout and exit 2", n)
		}
	}

	// An aggregate LRF relevance-bound-exceeded result is frontier-resource-
	// exhausted, and the issue string must never reach the envelope.
	lrf := fx["aggregate-lrf-bound-is-resource-exhaustion"]
	if lrf.ID == "" {
		t.Fatal("the aggregate LRF fixture is missing")
	}
	var agg *Case
	for i := range lrf.Cases {
		if lrf.Cases[i].ID == "aggregate-bound-document" {
			agg = &lrf.Cases[i]
		}
	}
	if agg == nil {
		t.Fatal("the aggregate-bound-document case is missing")
	}
	if agg.Expect.ErrorCode != "frontier-resource-exhausted" {
		t.Errorf("an aggregate LRF bound document must map to frontier-resource-exhausted, got %q",
			agg.Expect.ErrorCode)
	}
	forbidsIssue := false
	for _, s := range agg.ForbiddenSubstrings {
		if s == "relevance-bound-exceeded" {
			forbidsIssue = true
		}
	}
	if !forbidsIssue {
		t.Error("the aggregate LRF case must assert that relevance-bound-exceeded never appears in output")
	}
	if strings.Contains(agg.Expect.Stderr, "relevance-bound-exceeded") {
		t.Error("CF-V0-022 forbids relevance-bound-exceeded as an operational code")
	}

	// Forged TCQ authority is an upstream canonical rejection, not a Frontier item.
	forged := fx["forged-tcq-authority-rejected-upstream"]
	if forged.ID == "" {
		t.Fatal("the forged-TCQ fixture is missing")
	}
	for _, c := range forged.Cases {
		if c.Expect.Kind != "error" {
			t.Errorf("%s: forged authority must be rejected, not projected into an item", c.ID)
		}
		if frontierCodes[c.Expect.ErrorCode] {
			t.Errorf("%s: the row requires an UPSTREAM canonical rejection, but %q is Frontier-owned",
				c.ID, c.Expect.ErrorCode)
		}
	}
}

// TestNoFixtureClosesAnIntentTestItem is the release-blocker guard from the
// spec's kill criteria: "A lexical OCM candidate or caller-reported TCQ relation
// closing one item ... is an immediate release blocker." CF-V0-015 says every
// structurally linked obligation emits an INTENT_TEST item, so no fixture may
// expect a linked obligation without one.
func TestNoFixtureClosesAnIntentTestItem(t *testing.T) {
	for _, f := range loadFixturesT(t) {
		for _, c := range f.Cases {
			if c.Expect.Kind != "result" {
				continue
			}
			// Only reason about cases whose sample is the whole projection.
			if fmt.Sprint(len(c.Expect.Items)) != c.Expect.ItemCount {
				continue
			}
			kinds := map[string]map[string]bool{}
			for _, it := range c.Expect.Items {
				if kinds[it.SubjectID] == nil {
					kinds[it.SubjectID] = map[string]bool{}
				}
				kinds[it.SubjectID][it.Kind] = true
			}
			for _, o := range c.Declared.Obligations {
				switch o.Disposition {
				case "linked":
					if !kinds[o.ID]["INTENT_CHANGE"] {
						t.Errorf("%s/%s: CF-V0-012 requires an INTENT_CHANGE item for linked obligation %s",
							f.ID, c.ID, o.ID)
					}
					if !kinds[o.ID]["INTENT_TEST"] {
						t.Errorf("%s/%s: CF-V0-015 requires an INTENT_TEST item for linked obligation %s; "+
							"a closed test item is an immediate release blocker", f.ID, c.ID, o.ID)
					}
				case "unknown":
					if kinds[o.ID]["INTENT_TEST"] {
						t.Errorf("%s/%s: CF-V0-011 forbids a fabricated INTENT_TEST item for OCM-unknown %s",
							f.ID, c.ID, o.ID)
					}
				}
			}
			// Every CALLER_REPORTED item is an INTENT_TEST item and vice versa
			// (CF-V0-015 fixes the authority class for that kind exactly).
			for _, it := range c.Expect.Items {
				if it.Kind == "INTENT_TEST" && it.AuthorityClass != "CALLER_REPORTED" {
					t.Errorf("%s/%s: CF-V0-015 fixes INTENT_TEST authorityClass to CALLER_REPORTED, got %q",
						f.ID, c.ID, it.AuthorityClass)
				}
				if it.Kind != "INTENT_TEST" && it.AuthorityClass == "CALLER_REPORTED" {
					t.Errorf("%s/%s: only an INTENT_TEST item may be CALLER_REPORTED", f.ID, c.ID)
				}
			}
		}
	}
}

// TestSampledItemsRespectKindOrder checks CF-V0-020's kind order on every
// fully-sampled projection.
func TestSampledItemsRespectKindOrder(t *testing.T) {
	rank := map[string]int{"HUNK_BASIS": 0, "INTENT_CHANGE": 1, "INTENT_TEST": 2}
	for _, f := range loadFixturesT(t) {
		for _, c := range f.Cases {
			prev := -1
			for _, it := range c.Expect.Items {
				r := rank[it.Kind]
				if r < prev {
					t.Errorf("%s/%s: CF-V0-020 requires HUNK_BASIS, then INTENT_CHANGE, then INTENT_TEST",
						f.ID, c.ID)
					break
				}
				prev = r
			}
		}
	}
}

// TestErrorEnvelopeIsCanonicalForEveryCode proves the envelope shape once per
// admitted code rather than trusting the generator.
func TestErrorEnvelopeIsCanonicalForEveryCode(t *testing.T) {
	for code := range frontierCodes {
		env := ErrorEnvelope(code)
		ok, err := IsCompleteDocument([]byte(env))
		if err != nil {
			t.Fatalf("%s: %v", code, err)
		}
		if !ok {
			t.Fatalf("%s: envelope is not codec bytes plus exactly one LF: %q", code, env)
		}
		if !bytes.HasSuffix([]byte(env), []byte("\n")) || bytes.Count([]byte(env), []byte("\n")) != 1 {
			t.Fatalf("%s: CF-V0-022 requires exactly one LF", code)
		}
		if strings.Contains(env, "message") || strings.Contains(env, "detail") {
			t.Fatalf("%s: the envelope is exactly {code, profile}", code)
		}
	}
}

// TestRunFixturesAgainstImplementation is the whole point of the fixture tree.
// It skips until the binding exists, and then skips only the individual cases
// whose declared universe the bound runner cannot materialize, each with the
// specific reason that universe is out of reach.
func TestRunFixturesAgainstImplementation(t *testing.T) {
	if registeredRunner == nil {
		t.Skip(unboundReason)
	}
	for _, f := range loadFixturesT(t) {
		f := f
		for _, c := range f.Cases {
			c := c
			t.Run(f.ID+"/"+c.ID, func(t *testing.T) {
				for _, capability := range RequiredCapabilities(f, c) {
					if !registeredRunner.Supports(capability) {
						t.Skip(SkipReason(c, capability))
					}
				}
				got, err := registeredRunner.Run(f, c)
				if err != nil {
					t.Fatalf("runner: %v", err)
				}
				// CF-V0-024 first: a canary in ANY output is a release blocker,
				// whatever else the case expected.
				for _, s := range c.ForbiddenSubstrings {
					if bytes.Contains(got.Stdout, []byte(s)) {
						t.Errorf("CF-V0-024: canary %q leaked into stdout", s)
					}
					if bytes.Contains(got.Stderr, []byte(s)) {
						t.Errorf("CF-V0-024: canary %q leaked into stderr", s)
					}
				}
				switch c.Expect.Kind {
				case "error":
					if len(got.Stdout) != 0 {
						t.Errorf("CF-V0-022 forbids stdout on operational failure, got %d bytes", len(got.Stdout))
					}
					if got.ExitCode != 2 {
						t.Errorf("exit %d, want 2", got.ExitCode)
					}
					if string(got.Stderr) != c.Expect.Stderr {
						t.Errorf("stderr envelope differs\n want %q\n  got %q", c.Expect.Stderr, got.Stderr)
					}
				case "result":
					if got.ExitCode != c.Expect.ExitCode {
						t.Errorf("exit %d, want %d", got.ExitCode, c.Expect.ExitCode)
					}
					if len(got.Stderr) != 0 {
						t.Errorf("a valid result writes no error envelope, got %q", got.Stderr)
					}
					ok, err := IsCompleteDocument(got.Stdout)
					if err != nil {
						t.Fatalf("CF-V0-019: stdout does not parse under the codec: %v", err)
					}
					if !ok {
						t.Error("CF-V0-019: stdout is not codec(document) plus exactly one LF")
					}
					checkResultProjection(t, c, got)
				case "no-output-guarantee":
					// CF-V0-022 guarantees nothing here. The only thing that would
					// be a defect is a partial document presented as complete.
					if len(got.Stdout) > 0 {
						if ok, err := IsCompleteDocument(got.Stdout); err != nil || !ok {
							t.Error("a partial Frontier document was left on stdout")
						}
					}
				}
			})
		}
	}
}

// checkResultProjection compares the emitted item projection against the
// authored expectation. It reads the document with this suite's OWN codec
// (refcodec.go), never the implementation's, so a shared parsing defect cannot
// make a wrong document look right.
//
// Content-addressed identifiers are compared through Outcome.Symbols: a fixture
// authored before the implementation existed cannot name a real hunk or claim
// ID, so it names a symbol and the runner reports what it bound that symbol to.
// Obligation IDs are NOT symbolic — they are caller-authored and appear
// verbatim in both the fixture and the committed intent scope — so they are
// compared literally.
func checkResultProjection(t *testing.T, c Case, got Outcome) {
	t.Helper()
	items, err := documentItems(got.Stdout)
	if err != nil {
		t.Fatalf("read emitted items: %v", err)
	}
	want, err := decimalCount(c.Expect.ItemCount)
	if err != nil {
		t.Fatalf("itemCount: %v", err)
	}
	if len(items) != want {
		t.Errorf("emitted %d items, the fixture requires %d", len(items), want)
	}
	if len(c.Expect.Items) == 0 {
		return
	}
	// A fully sampled projection is compared positionally, which also checks
	// the CF-V0-020 order. A partial sample is matched by subject.
	if len(c.Expect.Items) == want && len(items) == want {
		for index, expected := range c.Expect.Items {
			compareItem(t, c, index, expected, items[index], got.Symbols)
		}
		return
	}
	for _, expected := range c.Expect.Items {
		subject := resolveSymbol(expected.SubjectID, got.Symbols)
		index := -1
		for at, item := range items {
			if item.Kind == expected.Kind && item.SubjectID == subject {
				index = at
			}
		}
		if index < 0 {
			t.Errorf("no %s item for subject %s", expected.Kind, subject)
			continue
		}
		compareItem(t, c, index, expected, items[index], got.Symbols)
	}
}

func compareItem(t *testing.T, c Case, index int, expected ExpectItem, got emittedItem, symbols map[string]string) {
	t.Helper()
	where := c.ID + " item " + fmt.Sprint(index)
	if got.Kind != expected.Kind {
		t.Errorf("%s: kind = %q, want %q", where, got.Kind, expected.Kind)
	}
	if subject := resolveSymbol(expected.SubjectID, symbols); got.SubjectID != subject {
		t.Errorf("%s: subjectId = %q, want %q", where, got.SubjectID, subject)
	}
	if got.AuthorityClass != expected.AuthorityClass {
		t.Errorf("%s: authorityClass = %q, want %q", where, got.AuthorityClass, expected.AuthorityClass)
	}
	if got.ResolutionClass != expected.ResolutionClass {
		t.Errorf("%s: resolutionClass = %q, want %q", where, got.ResolutionClass, expected.ResolutionClass)
	}
	if got.NextAction != expected.NextAction {
		t.Errorf("%s: nextAction = %q, want %q", where, got.NextAction, expected.NextAction)
	}
	// CF-V0-016 fixes the stored reason order, so this is a sequence check.
	if strings.Join(got.Reasons, ",") != strings.Join(expected.Reasons, ",") {
		t.Errorf("%s: reasons = %v, want %v", where, got.Reasons, expected.Reasons)
	}
	wantRelated := make([]string, 0, len(expected.RelatedIDs))
	for _, id := range expected.RelatedIDs {
		wantRelated = append(wantRelated, resolveSymbol(id, symbols))
	}
	sort.Strings(wantRelated)
	if strings.Join(got.RelatedIDs, ",") != strings.Join(wantRelated, ",") {
		t.Errorf("%s: relatedIds = %v, want %v", where, got.RelatedIDs, wantRelated)
	}
}

func resolveSymbol(value string, symbols map[string]string) string {
	if bound, found := symbols[value]; found {
		return bound
	}
	return value
}

// emittedItem is one item read out of the emitted document by this suite's own
// codec.
type emittedItem struct {
	AuthorityClass  string
	Kind            string
	NextAction      string
	Reasons         []string
	RelatedIDs      []string
	ResolutionClass string
	SubjectID       string
}

func documentItems(raw []byte) ([]emittedItem, error) {
	if len(raw) == 0 || raw[len(raw)-1] != '\n' {
		return nil, fmt.Errorf("document does not end with exactly one LF")
	}
	root, err := Parse(raw[:len(raw)-1])
	if err != nil {
		return nil, err
	}
	array, found := member(root, "items")
	if !found || array.Kind != KindArray {
		return nil, fmt.Errorf("document has no items array")
	}
	items := make([]emittedItem, 0, len(array.Arr))
	for _, entry := range array.Arr {
		items = append(items, emittedItem{
			AuthorityClass:  stringAt(entry, "authorityClass"),
			Kind:            stringAt(entry, "kind"),
			NextAction:      stringAt(entry, "nextAction"),
			Reasons:         stringsAt(entry, "reasons"),
			RelatedIDs:      stringsAt(entry, "relatedIds"),
			ResolutionClass: stringAt(entry, "resolutionClass"),
			SubjectID:       stringAt(entry, "subjectId"),
		})
	}
	return items, nil
}

func member(value Value, key string) (Value, bool) {
	if value.Kind != KindObject {
		return Value{}, false
	}
	for _, m := range value.Obj {
		if m.Key == key {
			return m.Val, true
		}
	}
	return Value{}, false
}

func stringAt(value Value, key string) string {
	found, ok := member(value, key)
	if !ok || found.Kind != KindString {
		return ""
	}
	return found.Str
}

func stringsAt(value Value, key string) []string {
	found, ok := member(value, key)
	if !ok || found.Kind != KindArray {
		return nil
	}
	out := make([]string, 0, len(found.Arr))
	for _, entry := range found.Arr {
		out = append(out, entry.Str)
	}
	return out
}

// TestEveryCapabilityNamesWhyItIsOutOfReach keeps the skip mechanism honest: a
// capability with no recorded reason would produce a blanket skip, which is the
// exact failure this mechanism exists to prevent.
func TestEveryCapabilityNamesWhyItIsOutOfReach(t *testing.T) {
	declared := []Capability{
		CapFaultInjection, CapLRFResultOverride, CapTCQResultOverride, CapDynamicTestTuple,
		CapTCQDiagnosticSet, CapDeclaredItemCounts, CapUnreachableItemCount,
		CapReasonUnionBeyondAlgebra,
		CapCandidateDocument, CapSiblingRepository, CapUncommittableOCMReferences,
		CapObligationFreeUniverse,
	}
	if len(declared) != len(capabilityReasons) {
		t.Errorf("%d capabilities are declared but %d have reasons", len(declared), len(capabilityReasons))
	}
	for _, capability := range declared {
		reason, found := capabilityReasons[capability]
		if !found {
			t.Errorf("capability %q has no reason", capability)
			continue
		}
		if len(reason) < 60 {
			t.Errorf("capability %q has a reason too short to name what is out of reach: %q", capability, reason)
		}
	}
	// Every required capability of every case must be one of the declared set.
	known := map[Capability]bool{}
	for _, capability := range declared {
		known[capability] = true
	}
	for _, f := range loadFixturesT(t) {
		for _, c := range f.Cases {
			for _, capability := range RequiredCapabilities(f, c) {
				if !known[capability] {
					t.Errorf("%s/%s requires undeclared capability %q", f.ID, c.ID, capability)
				}
			}
		}
	}
}

// TestReasonBoundIsUnreachableByTheReasonAlgebra is the recorded adjudication
// of the CF-V0-023 32-reasons-per-item pair. It derives the ceiling from the
// suite's own reading of the closed unions rather than from the implementation,
// so if a future clause widens a union this fails instead of quietly agreeing.
func TestReasonBoundIsUnreachableByTheReasonAlgebra(t *testing.T) {
	if got := ReachableReasonMaximum(); got >= LimitReasonsPerItem {
		t.Fatalf("the reason algebra now reaches %d reasons, which is at or above the CF-V0-023 bound "+
			"of %d: the resource-limits pair is materializable again and must stop being skipped",
			got, LimitReasonsPerItem)
	}
	for _, kind := range []string{"HUNK_BASIS", "INTENT_CHANGE", "INTENT_TEST"} {
		if _, found := ReasonAlgebraCeilings[kind]; !found {
			t.Errorf("no recorded reason ceiling for %s", kind)
		}
	}
}

// TestEmptyStateIsUnreachableThroughTheEntryPoint is the recorded adjudication
// of the sixteen obligation-free cases, and the reason their skip is a result
// rather than a claim. IntentItemsPerObligation writes the chain down; this drives
// its load-bearing link for real, against a real repository, so a relaxed OCM
// verifier fails here instead of quietly turning the skip message into a lie.
func TestEmptyStateIsUnreachableThroughTheEntryPoint(t *testing.T) {
	// Link 4, read from the clauses: every obligation costs at least one item.
	for disposition, items := range IntentItemsPerObligation {
		if items < 1 {
			t.Fatalf("a %q obligation now contributes %d items: an obligation-free result is no longer "+
				"the only way to reach CF-V0-004 EMPTY, and the derivation must be re-read",
				disposition, items)
		}
	}

	// The recorded cost of the gap must match the tree it describes.
	// A case counts here only when this is the reason it actually skips with:
	// RequiredCapabilities returns capabilityOrder order, and obligation-free
	// is last, so a case reported under it has no more specific blocker.
	blocked := []string{}
	for _, f := range loadFixturesT(t) {
		for _, c := range f.Cases {
			required := RequiredCapabilities(f, c)
			if len(required) > 0 && required[0] == CapObligationFreeUniverse {
				blocked = append(blocked, f.ID+"/"+c.ID)
			}
		}
	}
	if len(blocked) != ObligationFreeCaseCount {
		t.Errorf("%d cases are blocked by the obligation-free derivation, but %d is recorded: %v",
			len(blocked), ObligationFreeCaseCount, blocked)
	}

	if registeredRunner == nil {
		t.Skip(unboundReason)
	}
	// Link 2, driven rather than asserted: a real universe whose intent scope
	// enumerates no requirement is refused, with the exact upstream code.
	fixture, subject := findCase(t, "whitespace-and-line-ending-hunks-close", "whitespace-only")
	if len(subject.Declared.Obligations) != 0 {
		t.Fatalf("%s no longer declares an obligation-free universe", subject.ID)
	}
	got, err := registeredRunner.Run(fixture, subject)
	if err != nil {
		t.Fatalf("runner: %v", err)
	}
	const refusal = `{"code":"missing-requirements","profile":"frontier-error/0"}` + "\n"
	if got.ExitCode != 2 || string(got.Stderr) != refusal || len(got.Stdout) != 0 {
		t.Fatalf("an obligation-free universe now yields exit %d / stdout %d bytes / stderr %q, not the "+
			"`missing-requirements` refusal. CF-V0-004 EMPTY may be reachable again: re-read "+
			"IntentItemsPerObligation's derivation and stop skipping the %d obligation-free cases",
			got.ExitCode, len(got.Stdout), got.Stderr, ObligationFreeCaseCount)
	}
}

// findCase returns one fixture and one of its cases by ID.
func findCase(t *testing.T, fixtureID, caseID string) (Fixture, Case) {
	t.Helper()
	for _, f := range loadFixturesT(t) {
		if f.ID != fixtureID {
			continue
		}
		for _, c := range f.Cases {
			if c.ID == caseID {
				return f, c
			}
		}
	}
	t.Fatalf("no case %s/%s", fixtureID, caseID)
	return Fixture{}, Case{}
}

// TestCountBoundsAreDefenceInDepth is the recorded adjudication of the
// CF-V0-023 counts cases that skip as unreachable. Five of the seven bounds are
// fed by an artifact whose own ceiling equals or undercuts the Frontier one, so
// their at-N+1 case cannot be materialized at all. Those coincidences are what
// the skip messages assert, so they are checked here rather than trusted: if an
// upstream cap moves, a case becomes materializable again and must stop being
// skipped, and this fails instead of quietly agreeing with a stale reason.
func TestCountBoundsAreDefenceInDepth(t *testing.T) {
	for bound, holds := range CountBoundsAreDefenceInDepth() {
		if !holds {
			t.Errorf("the %s ceiling is no longer defence in depth over a narrower artifact algebra: "+
				"its at-N+1 case may be materializable now, and its recorded skip reason is stale", bound)
		}
	}
	// The outputBytes derivation rests on one measured document plus the
	// aggregate related-ID caps. If the measurement ever approached the bound,
	// the "roughly 3.0 MB against 4,194,304" arithmetic would stop holding.
	if MeasuredMaximalDocumentBytes >= LimitOutputBytes/2 {
		t.Errorf("the item-maximal document measures %d bytes against a %d-byte bound; the outputBytes "+
			"derivation assumes ample headroom for related IDs and must be re-read",
			MeasuredMaximalDocumentBytes, LimitOutputBytes)
	}
}

// TestEveryCountsCaseHasAPlanOrAReason pins the other half: a counts case is
// either buildable, with a plan whose own arithmetic reproduces the declared
// item count, or it is not, with a derivation long enough to say why. A silent
// third state — executed against a universe that is not the declared one, or
// skipped with a shrug — is what this rules out.
func TestEveryCountsCaseHasAPlanOrAReason(t *testing.T) {
	for _, f := range loadFixturesT(t) {
		for _, c := range f.Cases {
			if len(c.Declared.Counts) == 0 || c.Declared.Counts["reasonsPerItem"] != "" {
				continue
			}
			plan, unreachable := PlanCounts(c)
			if unreachable == "" {
				want, err := decimalCount(c.Expect.ItemCount)
				if err != nil || plan.items() != want {
					t.Errorf("%s: the derived universe projects %d items, the case declares %q",
						c.ID, plan.items(), c.Expect.ItemCount)
				}
				continue
			}
			if len(unreachable) < 120 {
				t.Errorf("%s: skips as unreachable with a reason too short to name what is out of "+
					"reach: %q", c.ID, unreachable)
			}
		}
	}
}

// TestMeasuredMaximalDocumentIsStillMeasured drives the number the outputBytes
// derivation rests on, rather than trusting a constant someone once observed.
// The item-maximal universe is the totalItems at-N case, which this suite now
// builds for real; a document that has drifted far from the recorded size means
// the derivation was written about a different document.
func TestMeasuredMaximalDocumentIsStillMeasured(t *testing.T) {
	if registeredRunner == nil {
		t.Skip(unboundReason)
	}
	fixture, subject := findCase(t, "resource-limits", "totalItems-at-limit")
	got, err := registeredRunner.Run(fixture, subject)
	if err != nil {
		t.Fatalf("runner: %v", err)
	}
	drift := len(got.Stdout) - MeasuredMaximalDocumentBytes
	if drift < 0 {
		drift = -drift
	}
	if drift*10 > MeasuredMaximalDocumentBytes {
		t.Errorf("the item-maximal document is %d bytes, more than a tenth away from the recorded "+
			"MeasuredMaximalDocumentBytes = %d: re-measure it and re-read the outputBytes derivation",
			len(got.Stdout), MeasuredMaximalDocumentBytes)
	}
}

// TestSynthesizedUniversesAreDeterministic runs one synthesized counts universe
// twice, in two independent temporary repositories, and requires byte-identical
// output.
//
// The universes in this file are DERIVED rather than enumerated, so their
// determinism is a property of the derivation and not of a checked-in fixture.
// Anything drawn from the clock, the filesystem, or Go's map iteration order
// would move a commit OID and with it every CF-V0-006 and CF-V0-007 identity in
// the document — which is exactly what this compares, since the two repositories
// share nothing but their content.
func TestSynthesizedUniversesAreDeterministic(t *testing.T) {
	if registeredRunner == nil {
		t.Skip(unboundReason)
	}
	fixture, subject := findCase(t, "resource-limits", "intentChangeItems-at-limit")
	first, err := registeredRunner.Run(fixture, subject)
	if err != nil {
		t.Fatalf("runner: %v", err)
	}
	second, err := registeredRunner.Run(fixture, subject)
	if err != nil {
		t.Fatalf("runner: %v", err)
	}
	if !bytes.Equal(first.Stdout, second.Stdout) {
		t.Errorf("two builds of the same derived universe produced different documents (%d and %d "+
			"bytes): the synthesis is reading something other than the case",
			len(first.Stdout), len(second.Stdout))
	}
}
