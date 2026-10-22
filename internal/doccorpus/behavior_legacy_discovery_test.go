package doccorpus

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"
)

// legacyRepo commits the legacy-suite fixtures under e2e/ and returns the root and revision.
func legacyRepo(t *testing.T) (string, string) {
	t.Helper()
	root := t.TempDir()
	git(t, root, "init", "-q")
	git(t, root, "config", "user.name", "Corpus test")
	git(t, root, "config", "user.email", "corpus@example.invalid")
	for _, name := range []string{"jasmine/summary.spec.js", "cypress/checkout.cy.js"} {
		data, err := os.ReadFile(filepath.Join("testdata", "legacy-suite", filepath.FromSlash(name)))
		if err != nil {
			t.Fatal(err)
		}
		legacyWrite(t, root, "e2e/"+name, string(data))
	}
	legacyWrite(t, root, "e2e/cypress/support.js", "Cypress.Commands.add('login', () => { expect(1).toBe(1); });\n")
	git(t, root, "add", ".")
	git(t, root, "commit", "-qm", "legacy suites")
	return root, git(t, root, "rev-parse", "HEAD")
}

func legacyWrite(t *testing.T, root, name, data string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(filepath.Join(root, name)), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, name), []byte(data), 0600); err != nil {
		t.Fatal(err)
	}
}

func legacyInventory(t *testing.T, root, runner, revision, path string, registry []byte) (LegacyInventory, []byte, error) {
	t.Helper()
	return legacyInventoryContext(context.Background(), t, root, runner, revision, path, registry)
}

func legacyInventoryContext(ctx context.Context, t *testing.T, root, runner, revision, path string, registry []byte) (LegacyInventory, []byte, error) {
	t.Helper()
	data, err := BuildLegacyInventory(ctx, LegacyInventoryInput{Root: root, Runner: runner, Revision: revision, Path: path, Registry: registry})
	var inventory LegacyInventory
	if data != nil {
		if decodeErr := decode(data, &inventory); decodeErr != nil {
			t.Fatal(decodeErr)
		}
	}
	return inventory, data, err
}

func legacyCaseNamed(t *testing.T, inventory LegacyInventory, name string) BehaviorLegacyCase {
	t.Helper()
	for _, legacy := range inventory.Legacy {
		if legacy.Case == name {
			return legacy
		}
	}
	t.Fatalf("case %q not inventoried: %+v", name, inventory.Legacy)
	return BehaviorLegacyCase{}
}

func legacyUnknownReasons(inventory LegacyInventory, caseID string) []string {
	reasons := []string{}
	for _, unknown := range inventory.Unknown {
		if unknown.Case == caseID {
			reasons = append(reasons, unknown.Reason)
		}
	}
	return reasons
}

func legacyTuples(legacy BehaviorLegacyCase) []string {
	tuples := []string{}
	for _, criterion := range legacy.Criteria {
		tuples = append(tuples, criterion.ID+" "+criterion.Matcher+" | "+criterion.Locator+" | "+criterion.Value)
	}
	return tuples
}

func TestLegacyDiscoveryInventory(t *testing.T) {
	t.Parallel()
	root, revision := legacyRepo(t)
	t.Run("DCP-V1-060 DCP-V1-061 jasmine every case once with disabled marked", func(t *testing.T) {
		inventory, _, err := legacyInventory(t, root, "jasmine", revision, "e2e/jasmine/summary.spec.js", nil)
		if err != nil {
			t.Fatal(err)
		}
		if inventory.Schema != LegacyInventorySchema || inventory.Status != "generated" || inventory.Runner != "jasmine" || inventory.Revision != revision || inventory.Crosswalk != nil {
			t.Fatalf("header: %+v", inventory)
		}
		source, _ := os.ReadFile(filepath.Join("testdata", "legacy-suite", "jasmine", "summary.spec.js"))
		calls := regexp.MustCompile(`(?m)^\s*(it|xit|fit)\(`).FindAll(source, -1)
		if len(inventory.Legacy) != len(calls) || len(calls) != 8 {
			t.Fatalf("cases=%d calls=%d", len(inventory.Legacy), len(calls))
		}
		states := map[string]string{}
		ids := []string{}
		for _, legacy := range inventory.Legacy {
			states[legacy.Case] = legacy.State
			ids = append(ids, legacy.ID)
			if legacy.Evidence.Authority != "external-provider" || legacy.Evidence.Kind != "source" || legacy.Evidence.Revision != revision || len(legacy.Roles) != 0 {
				t.Fatalf("anchor: %+v", legacy)
			}
			excerpt, err := span(source, legacy.Evidence.Start, legacy.Evidence.End)
			if err != nil || Digest(excerpt) != legacy.Evidence.SpanSHA256 || Digest(source) != legacy.Evidence.SHA256 {
				t.Fatalf("case span not pinned: %+v", legacy.Evidence)
			}
			for _, criterion := range legacy.Criteria {
				if criterion.Evidence.Start < legacy.Evidence.Start || criterion.Evidence.End > legacy.Evidence.End {
					t.Fatalf("criterion outside case span: %+v", criterion)
				}
			}
		}
		if !uniqueIdentities(ids) {
			t.Fatalf("duplicate ids: %v", ids)
		}
		want := map[string]string{"shows the order count": "executable", "lists paid orders": "executable", "exports a CSV": "disabled", "is not written yet": "disabled", "waits on the backend": "disabled", "filters by status": "executable", "hides archived rows": "disabled", "checks totals with a custom helper": "executable"}
		for name, state := range want {
			if states[name] != state {
				t.Fatalf("%s: state %q, want %q (%v)", name, states[name], state, states)
			}
		}
		count := legacyCaseNamed(t, inventory, "shows the order count")
		if count.ID != "legacy:e2e/jasmine/summary.spec.js::Order summary > shows the order count" || count.Suite != "Order summary" {
			t.Fatalf("identity: %+v", count)
		}
		if got := legacyCaseNamed(t, inventory, "hides archived rows").Suite; got != "Order summary > archived orders" {
			t.Fatalf("suite path: %q", got)
		}
		if reasons := legacyUnknownReasons(inventory, legacyCaseNamed(t, inventory, "filters by status").ID); !slices.Contains(reasons, "focused") {
			t.Fatalf("focused not reported: %v", reasons)
		}
	})
	t.Run("DCP-V1-062 assertion tuples and unparsed assertions", func(t *testing.T) {
		inventory, _, err := legacyInventory(t, root, "jasmine", revision, "e2e/jasmine/summary.spec.js", nil)
		if err != nil {
			t.Fatal(err)
		}
		got := map[string][]string{}
		for _, name := range []string{"shows the order count", "lists paid orders", "filters by status"} {
			got[name] = legacyTuples(legacyCaseNamed(t, inventory, name))
		}
		want := map[string][]string{
			"shows the order count": {"c1 toBe | element(by.css('.count')).getText() | 2 orders", "c2 toEqual | page.title | Orders"},
			"lists paid orders":     {"c1 toContain | rows() | paid", "c2 toBeTruthy | rows().length>0 | (no expected value)"},
			"filters by status":     {"c1 toEqual | filter('paid') | ['A-1','A-2']"},
		}
		for name, tuples := range want {
			if !slices.Equal(got[name], tuples) {
				t.Fatalf("%s tuples:\n got %q\nwant %q", name, got[name], tuples)
			}
		}
		helper := legacyCaseNamed(t, inventory, "checks totals with a custom helper")
		reasons := legacyUnknownReasons(inventory, helper.ID)
		if len(helper.Criteria) != 0 || !slices.Contains(reasons, "unparsed-assertion") || !slices.Contains(reasons, "no-static-assertion") {
			t.Fatalf("unparsed assertion dropped: %+v %v", helper.Criteria, reasons)
		}
		for _, unknown := range inventory.Unknown {
			if unknown.Reason == "unparsed-assertion" && (unknown.Evidence == nil || !strings.Contains(unknown.Detail, "expect")) {
				t.Fatalf("unparsed assertion lacks evidence: %+v", unknown)
			}
		}

		mocha, _, err := legacyInventory(t, root, "mocha", revision, "e2e/cypress/checkout.cy.js", nil)
		if err != nil {
			t.Fatal(err)
		}
		if len(mocha.Legacy) != 5 || legacyCaseNamed(t, mocha, "applies a coupon").State != "disabled" {
			t.Fatalf("mocha cases: %+v", mocha.Legacy)
		}
		total := legacyTuples(legacyCaseNamed(t, mocha, "shows the cart total"))
		wantTotal := []string{
			"c1 should.have.text | cy.get('[data-test=total]') | $42.00",
			"c2 contains | cy | Pay now",
			"c3 contains | cy.get('nav') 'a' | Orders",
			"c4 should.be.visible | cy.get('nav').contains('a','Orders') | (no expected value)",
			"c5 should.have.attr | cy.get('nav').contains('a','Orders') | 'href','/orders'",
		}
		if !slices.Equal(total, wantTotal) {
			t.Fatalf("cypress tuples:\n got %q\nwant %q", total, wantTotal)
		}
		login := legacyCaseNamed(t, mocha, "redirects to login")
		if !slices.Equal(legacyTuples(login), []string{"c1 should.eq | cy.location('pathname') | /login", "c2 to.have.length | $form | 1"}) || !slices.Contains(legacyUnknownReasons(mocha, login.ID), "should-callback") {
			t.Fatalf("login tuples: %q %v", legacyTuples(login), legacyUnknownReasons(mocha, login.ID))
		}
		if login.Suite != "Checkout > when signed out" {
			t.Fatalf("context suite: %q", login.Suite)
		}
		custom := legacyCaseNamed(t, mocha, "custom command assertion")
		if !slices.Contains(legacyUnknownReasons(mocha, custom.ID), "unparsed-assertion") {
			t.Fatalf("bare should dropped: %v", legacyUnknownReasons(mocha, custom.ID))
		}
		reload := legacyCaseNamed(t, mocha, "keeps the cart after reload")
		if !slices.Contains(legacyUnknownReasons(mocha, reload.ID), "focused") || !slices.Equal(legacyTuples(reload), []string{"c1 to.be.true | true | (no expected value)"}) {
			t.Fatalf("reload: %q %v", legacyTuples(reload), legacyUnknownReasons(mocha, reload.ID))
		}
	})
	t.Run("DCP-V1-063 precondition anchors are content-bound", func(t *testing.T) {
		inventory, _, err := legacyInventory(t, root, "mocha", revision, "e2e/cypress", nil)
		if err != nil {
			t.Fatal(err)
		}
		if inventory.Path != "e2e/cypress" || len(inventory.Legacy) != 5 {
			t.Fatalf("directory mode read non-test files or missed cases: %d", len(inventory.Legacy))
		}
		kinds := map[string]LegacyPrecondition{}
		for _, precondition := range inventory.Preconditions {
			kinds[precondition.Kind+" "+precondition.Callee+" "+precondition.Text] = precondition
			if precondition.Fixture != precondition.Kind+":"+precondition.Callee+"@sha256:"+precondition.SHA256 || precondition.SHA256 != Digest([]byte(precondition.Text)) || precondition.Evidence.Kind != "source" {
				t.Fatalf("precondition not content-bound: %+v", precondition)
			}
		}
		for _, key := range []string{
			"setup cy.task cy.task('db:seed',{users:'admin-users'})",
			"intercept cy.intercept cy.intercept('GET','/api/cart',{fixture:'cart.json'}).as('cart')",
			"visit cy.visit cy.visit('/checkout')",
			"visit cy.visit cy.visit('/logout')",
		} {
			if _, ok := kinds[key]; !ok {
				t.Fatalf("missing precondition %q in %v", key, inventory.Preconditions)
			}
		}
		total := legacyCaseNamed(t, inventory, "shows the cart total")
		login := legacyCaseNamed(t, inventory, "redirects to login")
		// Execution order: run-once hooks, then per-case hooks, then the case body.
		if len(total.Fixtures) != 3 || len(login.Fixtures) != 4 || !slices.Equal(login.Fixtures[:3], total.Fixtures) ||
			!strings.HasPrefix(total.Fixtures[0], "setup:cy.task@") || !strings.HasPrefix(total.Fixtures[1], "intercept:cy.intercept@") ||
			!strings.HasPrefix(total.Fixtures[2], "visit:cy.visit@") || !strings.HasPrefix(login.Fixtures[3], "visit:cy.visit@") {
			t.Fatalf("fixtures: %v / %v", total.Fixtures, login.Fixtures)
		}
		jasmine, _, err := legacyInventory(t, root, "jasmine", revision, "e2e/jasmine/summary.spec.js", nil)
		if err != nil {
			t.Fatal(err)
		}
		count := legacyCaseNamed(t, jasmine, "shows the order count")
		if len(count.Fixtures) != 2 || !strings.HasPrefix(count.Fixtures[0], "setup:seedOrders@sha256:") || !strings.HasPrefix(count.Fixtures[1], "visit:browser.get@sha256:") {
			t.Fatalf("jasmine fixtures: %v", count.Fixtures)
		}
	})
}

func TestLegacyDiscoveryDeterministicReadOnly(t *testing.T) {
	t.Parallel()
	t.Run("DCP-V1-064 pinned, deterministic and read-only", func(t *testing.T) {
		root, revision := legacyRepo(t)
		status := git(t, root, "status", "--porcelain", "--untracked-files=all")
		_, first, err := legacyInventory(t, root, "mocha", revision, ".", nil)
		if err != nil {
			t.Fatal(err)
		}
		// Working-tree edits never reach the inventory: it reads Git blobs at the revision.
		legacyWrite(t, root, "e2e/cypress/checkout.cy.js", "it('changed', () => {});\n")
		_, second, err := legacyInventory(t, root, "mocha", revision, ".", nil)
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(first, second) {
			t.Fatal("inventory bytes are not deterministic or read the working tree")
		}
		git(t, root, "checkout", "--", ".")
		if after := git(t, root, "status", "--porcelain", "--untracked-files=all"); after != status {
			t.Fatalf("discovery mutated the repository: %q", after)
		}
	})
}

// legacyRegistry is a schema 2 registry whose one reviewed same mapping retains the legacy tuple
// of "shows the cart total" with a reviewed annotation.
func legacyRegistry(t *testing.T, inventory LegacyInventory, edit func(*BehaviorRegistry)) []byte {
	t.Helper()
	total := legacyCaseNamed(t, inventory, "shows the cart total")
	c1 := total.Criteria[0]
	revision := inventory.Revision
	target := Anchor{Path: "e2e/cart.spec.ts", SHA256: Digest([]byte("cart spec"))}
	review := Anchor{Kind: "review", Revision: revision, Path: target.Path, SHA256: target.SHA256}
	registry := BehaviorRegistry{
		Schema: 2, ContractID: "legacy-crosswalk", ContractSHA256: Digest([]byte("contract")), SourceRevision: revision, DocumentationRevision: revision,
		Flows:     []BehaviorFlow{{ID: "checkout-cart-total", Criteria: []string{"total"}, RequiredPages: []string{"/checkout"}}, {ID: "account-settings", RequiredPages: []string{"/settings"}}},
		Behaviors: []BehaviorSource{{ID: "cart-total", Evidence: Anchor{Revision: revision}, Flows: []string{"checkout-cart-total"}}},
		Tests: []BehaviorTest{
			{ID: "e2e/cart.spec.ts:cart total", Title: "cart total is shown", Evidence: target, Flows: []string{"checkout-cart-total"}, Criteria: []string{"total"}, Fixtures: slices.Clone(total.Fixtures),
				Assertions: []BehaviorAssertion{{ID: "a1", Behavior: "cart-total", Criterion: "total", Annotation: review, Matcher: c1.Matcher, Locator: c1.Locator, Value: c1.Value}},
				Legacy:     []BehaviorLegacyMapping{{Criterion: "total", LegacyCase: total.ID, LegacyCriterion: c1.ID, Relation: "same", Review: review}}},
			{ID: "e2e/profile.spec.ts:profile", Title: "profile page renders", Criteria: []string{"profile"}},
		},
	}
	if edit != nil {
		edit(&registry)
	}
	data, err := Encode(registry)
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func TestLegacyCrosswalk(t *testing.T) {
	t.Parallel()
	root, revision := legacyRepo(t)
	base, _, err := legacyInventory(t, root, "mocha", revision, "e2e/cypress", nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Run("DCP-V1-065 generated deterministic candidates never reviewed", func(t *testing.T) {
		registry := legacyRegistry(t, base, nil)
		inventory, first, err := legacyInventory(t, root, "mocha", revision, "e2e/cypress", registry)
		if err != nil {
			t.Fatal(err)
		}
		_, second, err := legacyInventory(t, root, "mocha", revision, "e2e/cypress", registry)
		if err != nil || !bytes.Equal(first, second) {
			t.Fatalf("crosswalk is not deterministic: %v", err)
		}
		cw := inventory.Crosswalk
		if cw == nil || cw.Status != "generated" || cw.RegistrySHA256 != Digest(registry) || len(cw.Candidates) != len(inventory.Legacy) {
			t.Fatalf("crosswalk header: %+v", cw)
		}
		if bytes.Contains(first, []byte(`"legacy_criteria"`)) || bytes.Contains(first, []byte(`"review"`)) || bytes.Contains(first, []byte(`"reviewed_annotation"`)) {
			t.Fatal("generated crosswalk emitted a reviewed legacy relation")
		}
		total := legacyCaseNamed(t, inventory, "shows the cart total")
		var set LegacyCandidateSet
		for _, candidate := range cw.Candidates {
			if candidate.LegacyCase == total.ID {
				set = candidate
			}
		}
		if len(set.Targets) == 0 || set.Targets[0].Kind != "test" || set.Targets[0].ID != "e2e/cart.spec.ts:cart total" || !slices.Contains(set.Targets[0].Reasons, "tuple-equal:c1") || !slices.Contains(set.Targets[0].Reasons, "preconditions-equal") {
			t.Fatalf("top candidate: %+v", set)
		}
		flow := false
		for _, target := range set.Targets {
			if target.Kind == "flow" && target.ID == "checkout-cart-total" && slices.Contains(target.Reasons, "page-visited:/checkout") {
				flow = true
			}
			if target.ID == "account-settings" || target.Score <= 0 || len(target.Reasons) == 0 {
				t.Fatalf("unsupported candidate: %+v", target)
			}
		}
		if !flow {
			t.Fatalf("flow candidate missing: %+v", set.Targets)
		}
		if len(cw.ReviewedChecks) != 1 || cw.ReviewedChecks[0].Verdict != "parity-candidate" || !slices.Equal(cw.ReviewedChecks[0].Reasons, []string{"runtime-baseline-required"}) {
			t.Fatalf("reviewed check: %+v", cw.ReviewedChecks)
		}
	})
	t.Run("DCP-V1-066 same with a different expected value is refused", func(t *testing.T) {
		registry := legacyRegistry(t, base, func(r *BehaviorRegistry) { r.Tests[0].Assertions[0].Value = "$40.00" })
		inventory, data, err := legacyInventory(t, root, "mocha", revision, "e2e/cypress", registry)
		if !errors.Is(err, ErrLegacyMappingRefused) || len(data) == 0 {
			t.Fatalf("expected refusal with report, got %v", err)
		}
		check := inventory.Crosswalk.ReviewedChecks[0]
		if check.Verdict != "refused" || !slices.Contains(check.Reasons, "expected-value-differs") {
			t.Fatalf("check: %+v", check)
		}
		registry = legacyRegistry(t, base, func(r *BehaviorRegistry) {
			r.Tests[0].Legacy[0].Relation = "stronger"
			r.Tests[0].Assertions[0].Locator = "#other"
		})
		inventory, _, err = legacyInventory(t, root, "mocha", revision, "e2e/cypress", registry)
		if !errors.Is(err, ErrLegacyMappingRefused) || !slices.Contains(inventory.Crosswalk.ReviewedChecks[0].Reasons, "observable-dropped") {
			t.Fatalf("stronger drop not refused: %v %+v", err, inventory.Crosswalk)
		}
		for _, tc := range []struct {
			edit            func(*BehaviorRegistry)
			verdict, reason string
		}{
			{func(r *BehaviorRegistry) { r.Tests[0].Legacy[0].Relation = "new" }, "not-parity", "relation-new-is-not-parity"},
			{func(r *BehaviorRegistry) { r.Tests[0].Legacy[0].LegacyCase = "legacy:gone" }, "unresolved", "legacy-case-not-discovered"},
			{func(r *BehaviorRegistry) { r.Tests[0].Legacy[0].LegacyCriterion = "c99" }, "unresolved", "legacy-criterion-not-discovered"},
			{func(r *BehaviorRegistry) { r.Tests[0].Roles = []string{"admin"} }, "not-parity", "roles-not-discovered"},
			{func(r *BehaviorRegistry) { r.Tests[0].Legacy[0].Review = Anchor{} }, "not-parity", "mapping-review-missing-or-stale"},
			{func(r *BehaviorRegistry) { r.Tests[0].Assertions[0].Annotation = Anchor{} }, "not-parity", "target-assertion-not-reviewed"},
			{func(r *BehaviorRegistry) { r.Tests[0].Criteria = []string{"other"} }, "not-parity", "target-criterion-not-declared"},
			{func(r *BehaviorRegistry) { slices.Reverse(r.Tests[0].Fixtures) }, "not-parity", "precondition-order-differs"},
		} {
			inventory, _, err := legacyInventory(t, root, "mocha", revision, "e2e/cypress", legacyRegistry(t, base, tc.edit))
			if err != nil {
				t.Fatal(err)
			}
			if check := inventory.Crosswalk.ReviewedChecks[0]; check.Verdict != tc.verdict || !slices.Contains(check.Reasons, tc.reason) {
				t.Fatalf("check: %+v, want %s %s", check, tc.verdict, tc.reason)
			}
		}
	})
	t.Run("DCP-V1-067 a changed seed digest makes the mapping not parity", func(t *testing.T) {
		registry := legacyRegistry(t, base, nil)
		source, err := os.ReadFile(filepath.Join(root, "e2e", "cypress", "checkout.cy.js"))
		if err != nil {
			t.Fatal(err)
		}
		legacyWrite(t, root, "e2e/cypress/checkout.cy.js", strings.Replace(string(source), "admin-users", "viewer-users", 1))
		git(t, root, "commit", "-qam", "change seed")
		changed := git(t, root, "rev-parse", "HEAD")
		inventory, _, err := legacyInventory(t, root, "mocha", changed, "e2e/cypress", registry)
		if err != nil {
			t.Fatal(err)
		}
		check := inventory.Crosswalk.ReviewedChecks[0]
		if check.Verdict != "not-parity" || !slices.Contains(check.Reasons, "precondition-digest-differs:setup:cy.task") {
			t.Fatalf("seed change not detected: %+v", check)
		}
		// Only the seed changed: the tuples still match, so this is not a refusal.
		if !slices.Equal(legacyTuples(legacyCaseNamed(t, inventory, "shows the cart total")), legacyTuples(legacyCaseNamed(t, base, "shows the cart total"))) {
			t.Fatal("seed change altered tuples")
		}
	})
	t.Run("DCP-V1-068 a duplicate legacy mapping is refused", func(t *testing.T) {
		registry := legacyRegistry(t, base, func(r *BehaviorRegistry) { r.Tests[0].Legacy = append(r.Tests[0].Legacy, r.Tests[0].Legacy[0]) })
		if _, _, err := legacyInventory(t, root, "mocha", revision, "e2e/cypress", registry); err == nil || !strings.Contains(err.Error(), "duplicate legacy criterion mapping") {
			t.Fatalf("duplicate mapping accepted: %v", err)
		}
	})
}

// TestLegacyPreconditionDigestCompileGap shows the behavior compiler keeps a changed seed
// digest from becoming runtime parity (DCP-V1-067).
func TestLegacyPreconditionDigestCompileGap(t *testing.T) {
	t.Parallel()
	t.Run("DCP-V1-067 compiler reports a precondition mismatch for a different seed digest", func(t *testing.T) {
		seed := "setup:seedOrders@sha256:" + Digest([]byte("seedOrders({count:2})"))
		changed := "setup:seedOrders@sha256:" + Digest([]byte("seedOrders({count:3})"))
		root, m := behaviorFixtureWithRun(t, func(r *BehaviorRegistry) {
			legacyFixture(r)
			r.Tests[0].Fixtures, r.Legacy[0].Fixtures = []string{seed}, []string{changed}
		}, true)
		a, err := Build(context.Background(), root, m)
		if err != nil {
			t.Fatal(err)
		}
		if len(a.BehaviorContracts[0].LegacyRuntimeParity) != 0 {
			t.Fatal("a different seed digest became parity")
		}
		found := false
		for _, gap := range a.Gaps {
			found = found || gap.Kind == "legacy-precondition-mismatch"
		}
		if !found {
			t.Fatalf("missing legacy-precondition-mismatch: %+v", a.Gaps)
		}
	})
}

func TestLegacyDiscoveryRefusals(t *testing.T) {
	t.Parallel()
	root, revision := legacyRepo(t)
	t.Run("DCP-V1-068 named refusals and file unknowns", func(t *testing.T) {
		for _, tc := range []struct {
			name, runner, revision, path string
			registry                     []byte
		}{
			{"runner", "jest", revision, "e2e", nil},
			{"short revision", "mocha", revision[:12], "e2e", nil},
			{"absolute path", "mocha", revision, "/e2e", nil},
			{"parent path", "mocha", revision, "../e2e", nil},
			{"missing path", "mocha", revision, "e2e/none", nil},
			{"non-canonical path", "mocha", revision, "e2e/cypress/../cypress", nil},
			{"open registry", "mocha", revision, "e2e/cypress", []byte(`{"tests":[],"extra":1}`)},
			{"empty registry", "mocha", revision, "e2e/cypress", []byte(`{}`)},
			{"schema 1 registry", "mocha", revision, "e2e/cypress", []byte(`{"schema":1,"contract_id":"c","contract_sha256":"` + Digest(nil) + `","source_revision":"` + revision + `","documentation_revision":"` + revision + `","discovery":{},"migration_manifest":{},"flows":[],"source_behaviors":[],"tests":[]}`)},
			{"duplicate test registry", "mocha", revision, "e2e/cypress", []byte(`{"schema":2,"contract_id":"c","contract_sha256":"` + Digest(nil) + `","source_revision":"` + revision + `","documentation_revision":"` + revision + `","discovery":{},"migration_manifest":{},"flows":[],"source_behaviors":[],"tests":[{"id":"t","project":"","title":"","evidence":{},"flows":[],"criteria":[],"assertions":[]},{"id":"t","project":"","title":"","evidence":{},"flows":[],"criteria":[],"assertions":[]}]}`)},
		} {
			_, data, err := legacyInventory(t, root, tc.runner, tc.revision, tc.path, tc.registry)
			var refusal *Error
			if !errors.As(err, &refusal) || data != nil {
				t.Fatalf("%s: expected a named refusal, got %v", tc.name, err)
			}
		}
		// The compiler keeps flows, source behaviors and tests in one identity namespace.
		inventory, _, err := legacyInventory(t, root, "mocha", revision, "e2e/cypress", nil)
		if err != nil {
			t.Fatal(err)
		}
		for name, edit := range map[string]func(*BehaviorRegistry){
			"test named as a flow":     func(r *BehaviorRegistry) { r.Tests[0].ID = r.Flows[0].ID },
			"test named as a behavior": func(r *BehaviorRegistry) { r.Tests[1].ID = r.Behaviors[0].ID },
		} {
			_, data, err := legacyInventory(t, root, "mocha", revision, "e2e/cypress", legacyRegistry(t, inventory, edit))
			if data != nil || err == nil || !strings.Contains(err.Error(), "duplicate flow, source behavior or test identities") {
				t.Fatalf("%s: expected the shared-namespace refusal, got %v", name, err)
			}
		}
		// A test-named symlink in a directory walk is refused as it is when selected alone.
		if err := os.Symlink("checkout.cy.js", filepath.Join(root, "e2e", "cypress", "linked.cy.js")); err != nil {
			t.Fatal(err)
		}
		git(t, root, "add", ".")
		git(t, root, "commit", "-qm", "symlinked suite")
		linked := git(t, root, "rev-parse", "HEAD")
		for _, selected := range []string{"e2e", "e2e/cypress", "e2e/cypress/linked.cy.js"} {
			if _, data, err := legacyInventory(t, root, "mocha", linked, selected, nil); data != nil || err == nil || !strings.Contains(err.Error(), "not a regular Git blob") {
				t.Fatalf("%s: a symlinked test file was not refused: %v", selected, err)
			}
		}
		git(t, root, "rm", "-q", "e2e/cypress/linked.cy.js")
		git(t, root, "commit", "-qm", "drop symlink")
		legacyWrite(t, root, "e2e/broken/broken.spec.js", "describe('broken', () => {\n  it('never closes', () => {\n")
		legacyWrite(t, root, "e2e/broken/empty.spec.js", "// no cases here\nconst value = 1;\n")
		legacyWrite(t, root, "e2e/plain/helpers.js", "export const value = 1;\n")
		git(t, root, "add", ".")
		git(t, root, "commit", "-qm", "broken")
		broken := git(t, root, "rev-parse", "HEAD")
		if _, data, err := legacyInventory(t, root, "mocha", broken, "e2e/plain", nil); data != nil || err == nil || !strings.Contains(err.Error(), "no test files") {
			t.Fatalf("directory without test files: %v", err)
		}
		inventory, _, err = legacyInventory(t, root, "mocha", broken, "e2e/broken", nil)
		if err != nil {
			t.Fatal(err)
		}
		reasons := []string{}
		for _, unknown := range inventory.Unknown {
			reasons = append(reasons, unknown.Path+" "+unknown.Reason)
		}
		if len(inventory.Legacy) != 0 || !slices.Equal(reasons, []string{"e2e/broken/broken.spec.js lex-failed", "e2e/broken/empty.spec.js no-static-cases"}) {
			t.Fatalf("file unknowns: %v", reasons)
		}
	})
}

func legacyEdgeTuples(legacy BehaviorLegacyCase) []string {
	tuples := []string{}
	for _, criterion := range legacy.Criteria {
		tuples = append(tuples, criterion.Matcher+"|"+criterion.Locator+"|"+criterion.Value)
	}
	return tuples
}

var legacyMoreSuite = `describe('more', () => {
  it('bare and', () => {
    cy.get('.x').and('have.text', 'X');
    cy.get('.y').should('be.visible');
  });
  it('computed continuation', () => {
    expect(v).equal(1)['and']['equal'](2);
  });
  if (flag) it('conditional case', () => { expect(v).toBe(1); });
  it('asi one', () => {
    expect(a).toBe(1)
  })
  it('asi two', () => {
    const s = ` + "`${cy.task('seed', 'T')}`" + `
    expect(b).toBe(2)
  })
  it('twice', () => { expect(c).toBe(3); });
  it('twice', () => { expect(c).toBe(4); });
});
describe('nested hook', () => {
  beforeEach(() => { cy.then(() => { cy.task('seed', 'A') }) });
  it('under a nested hook', () => { cy.get('.n').should('be.visible'); });
});
`

var legacyEdgeSuite = `describe('edge', () => {
  beforeEach(seedHelper);
  it('regex after a condition', () => {
    if (ready) /it('phantom', () => {})/.test(value);
    expect(a).toBe('\x41\u0042\u{43}');
  });
  it('template and chains', () => {
    const v = ` + "`${expect(x).toBe(1)}`" + `;
    expect(y).withContext('why').toBe(2);
    expect(z).to.equal(3).and.equal(4);
  });
  it('ordered preconditions', () => {
    cy.visit('/before');
    cy.task('seed', 'A');
    cy.get('.ok').should('be.visible');
  });
  it('nested preconditions', () => {
    cy.get('.x').then(() => { cy.task('seed', 'B'); });
    cy.visit('/` + strings.Repeat("a", 300) + `');
    cy.get('.ok').should('be.visible');
  });
});
describe('conditional hook', () => {
  beforeEach(() => { if (flag) { cy.task('seed', 'C'); } });
  it('inherits the doubt', () => { cy.get('.ok').should('be.visible'); });
});
`

var legacyRoundThreeSuite = `describe('round three', () => {
  const hook = ` + "`${beforeEach(() => { cy.task('seed', 'T') })}`" + `;
  it('template cypress', () => {
    const s = ` + "`${cy.contains('hello')}`" + `;
    cy.get('.a').should('be.visible');
  });
  it('computed member', () => {
    cy.get('.x')['should']('be.visible');
    cy['contains']('hi');
  });
  it('wrapped options', () => { cy.contains('hello', ({ matchCase: false })); });
  it('literal content', () => { cy.contains('.sel', 'hello'); });
  it('helpers', () => {
    const check = () => expect(value).toBe(1);
    function other() { expect(w).toBe(2); }
    items.forEach((x) => expect(x).toBe(3));
  });
});
`

// TestLegacyDiscoveryStaticEdgeCases pins the static reader's handling of syntax it must not
// misread: regex/division context, string escapes, template substitutions, chained matchers,
// precondition order and nesting, and hooks it cannot bind (DCP-V1-061..063, DCP-V1-068).
func TestLegacyDiscoveryStaticEdgeCases(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	git(t, root, "init", "-q")
	git(t, root, "config", "user.name", "Corpus test")
	git(t, root, "config", "user.email", "corpus@example.invalid")
	legacyWrite(t, root, "e2e/edge.cy.js", legacyEdgeSuite)
	legacyWrite(t, root, "e2e/octal.cy.js", "it('octal', () => { expect(a).toBe('\\101'); });\n")
	legacyWrite(t, root, "e2e/more.cy.js", legacyMoreSuite)
	legacyWrite(t, root, "e2e/root.cy.js", "beforeEach(() => { cy.visit('/root') })\ndescribe('rooted', () => {\n  it('under a root hook', () => { cy.get('.r').should('be.visible') })\n})\n")
	legacyWrite(t, root, "e2e/brace.cy.js", "it('brace', () => { if (a) {} /it('x', () => {})/.test(b); });\n")
	legacyWrite(t, root, "e2e/three.cy.js", legacyRoundThreeSuite)
	git(t, root, "add", ".")
	git(t, root, "commit", "-qm", "edge")
	revision := git(t, root, "rev-parse", "HEAD")
	inventory, _, err := legacyInventory(t, root, "mocha", revision, "e2e", nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Run("DCP-V1-062 every earlier-round edge case is refused by the allowlist", func(t *testing.T) {
		for _, legacy := range inventory.Legacy {
			want := "not-allowlisted"
			if strings.HasPrefix(legacy.ID, "legacy:e2e/root.cy.js::") {
				// The root hook is an admitted precondition form; the case is unknown because a
				// root hook applies to every suite the runner loads.
				want = "hook-not-static"
			}
			if reasons := legacyUnknownReasons(inventory, legacy.ID); !slices.Contains(reasons, want) {
				t.Fatalf("%s: want %s, got %v", legacy.ID, want, reasons)
			}
		}
	})
	t.Run("DCP-V1-061 DCP-V1-068 regex context and invalid escapes never invent cases", func(t *testing.T) {
		names := []string{}
		for _, legacy := range inventory.Legacy {
			if strings.HasPrefix(legacy.ID, "legacy:e2e/edge.cy.js::") {
				names = append(names, legacy.Case)
			}
		}
		if !slices.Equal(names, []string{"regex after a condition", "template and chains", "ordered preconditions", "nested preconditions", "inherits the doubt"}) {
			t.Fatalf("cases: %v", names)
		}
		failed := []string{}
		for _, unknown := range inventory.Unknown {
			if unknown.Reason == "lex-failed" {
				failed = append(failed, unknown.Path)
			}
		}
		if !slices.Equal(failed, []string{"e2e/brace.cy.js", "e2e/octal.cy.js"}) {
			t.Fatalf("lex-failed files: %v", failed)
		}
		if tuples := legacyEdgeTuples(legacyCaseNamed(t, inventory, "regex after a condition")); !slices.Equal(tuples, []string{"toBe|a|ABC"}) {
			t.Fatalf("escaped value: %q", tuples)
		}
	})
	t.Run("DCP-V1-062 template substitutions and chained matchers are never dropped", func(t *testing.T) {
		chains := legacyCaseNamed(t, inventory, "template and chains")
		if tuples := legacyEdgeTuples(chains); !slices.Equal(tuples, []string{"toBe|y|2", "to.equal|z|3"}) {
			t.Fatalf("chain tuples: %q", tuples)
		}
		details := []string{}
		for _, unknown := range inventory.Unknown {
			if unknown.Case == chains.ID && unknown.Reason == "unparsed-assertion" {
				details = append(details, unknown.Detail)
			}
		}
		slices.Sort(details)
		if len(details) != 2 || !strings.Contains(details[0], "and.equal(4)") || !strings.Contains(details[1], "template substitution") {
			t.Fatalf("unparsed: %q", details)
		}
	})
	t.Run("DCP-V1-063 preconditions keep order, nesting doubt and full text", func(t *testing.T) {
		ordered := legacyCaseNamed(t, inventory, "ordered preconditions")
		if len(ordered.Fixtures) != 2 || !strings.HasPrefix(ordered.Fixtures[0], "visit:cy.visit@") || !strings.HasPrefix(ordered.Fixtures[1], "setup:cy.task@") {
			t.Fatalf("order: %v", ordered.Fixtures)
		}
		nested := legacyCaseNamed(t, inventory, "nested preconditions")
		if len(nested.Fixtures) != 2 || !strings.HasPrefix(nested.Fixtures[0], "setup:cy.task@") || !slices.Contains(legacyUnknownReasons(inventory, nested.ID), "nested-precondition") {
			t.Fatalf("nested: %v %v", nested.Fixtures, legacyUnknownReasons(inventory, nested.ID))
		}
		for _, precondition := range inventory.Preconditions {
			if precondition.SHA256 != Digest([]byte(precondition.Text)) {
				t.Fatalf("text does not match digest: %+v", precondition)
			}
		}
		for _, name := range []string{"regex after a condition", "inherits the doubt"} {
			if !slices.Contains(legacyUnknownReasons(inventory, legacyCaseNamed(t, inventory, name).ID), "hook-not-static") {
				t.Fatalf("%s did not inherit hook doubt: %v", name, legacyUnknownReasons(inventory, legacyCaseNamed(t, inventory, name).ID))
			}
		}
	})
	t.Run("DCP-V1-061 DCP-V1-062 statement context, chains and repeated titles stay uncertain", func(t *testing.T) {
		reasons := func(name string) []string {
			return legacyUnknownReasons(inventory, legacyCaseNamed(t, inventory, name).ID)
		}
		for name, want := range map[string]string{
			"bare and": "unparsed-assertion", "computed continuation": "unparsed-assertion", "conditional case": "case-in-dynamic-scope",
			"twice": "duplicate-title", "twice #2": "duplicate-title", "under a nested hook": "hook-not-static", "under a root hook": "hook-not-static",
		} {
			if !slices.Contains(reasons(name), want) {
				t.Fatalf("%s: want %s, got %v", name, want, reasons(name))
			}
		}
		for _, name := range []string{"asi one", "asi two"} {
			if slices.Contains(reasons(name), "case-in-dynamic-scope") {
				t.Fatalf("%s: a semicolon-free statement was treated as dynamic", name)
			}
		}
		if rooted := legacyCaseNamed(t, inventory, "under a root hook"); len(rooted.Fixtures) != 0 {
			t.Fatalf("a root hook became a precondition: %v", rooted.Fixtures)
		}
		asi := legacyCaseNamed(t, inventory, "asi two")
		if len(asi.Fixtures) != 1 || !strings.HasPrefix(asi.Fixtures[0], "setup:cy.task@") || !slices.Contains(reasons("asi two"), "nested-precondition") {
			t.Fatalf("template precondition: %v %v", asi.Fixtures, reasons("asi two"))
		}
	})
	t.Run("DCP-V1-062 DCP-V1-063 computed members, template hooks, wrapped options and helpers stay unknown", func(t *testing.T) {
		details := func(name string) []string {
			id, out := legacyCaseNamed(t, inventory, name).ID, []string{}
			for _, unknown := range inventory.Unknown {
				if unknown.Case == id && unknown.Reason == "unparsed-assertion" {
					out = append(out, unknown.Detail)
				}
			}
			return out
		}
		for _, name := range []string{"template cypress", "computed member", "wrapped options", "literal content", "helpers"} {
			if !slices.Contains(legacyUnknownReasons(inventory, legacyCaseNamed(t, inventory, name).ID), "hook-not-static") {
				t.Fatalf("%s did not inherit the template hook doubt: %v", name, legacyUnknownReasons(inventory, legacyCaseNamed(t, inventory, name).ID))
			}
		}
		if got := details("template cypress"); len(got) != 1 || !strings.Contains(got[0], "template substitution") {
			t.Fatalf("template cypress: %q", got)
		}
		if got := details("computed member"); len(got) != 2 || !strings.Contains(got[0], "computed member") || !strings.Contains(got[1], "computed member") {
			t.Fatalf("computed member: %q", got)
		}
		wrapped := legacyCaseNamed(t, inventory, "wrapped options")
		if got := details("wrapped options"); len(wrapped.Criteria) != 0 || len(got) != 1 || !strings.Contains(got[0], "not static") {
			t.Fatalf("wrapped options: %+v %q", wrapped.Criteria, got)
		}
		if literal := legacyCaseNamed(t, inventory, "literal content"); len(literal.Criteria) != 1 || literal.Criteria[0].Value != "hello" {
			t.Fatalf("literal content: %+v", literal.Criteria)
		}
		helpers := legacyCaseNamed(t, inventory, "helpers")
		if tuples := legacyEdgeTuples(helpers); !slices.Equal(tuples, []string{"toBe|x|3"}) {
			t.Fatalf("helper tuples: %q", tuples)
		}
		if got := details("helpers"); len(got) != 2 || !strings.Contains(got[0], "does not visibly call") || !strings.Contains(got[1], "does not visibly call") {
			t.Fatalf("helper unknowns: %q", got)
		}
	})
}

// TestLegacyLexerCanonicalAndEscapes pins operator boundaries in canonical text and the decoding
// of surrogate escapes (DCP-V1-062, DCP-V1-063, DCP-V1-068).
func TestLegacyLexerCanonicalAndEscapes(t *testing.T) {
	t.Parallel()
	canonical := func(src string) string {
		tokens, _, ok := lexLegacy(src)
		if !ok {
			t.Fatalf("lex refused %q", src)
		}
		return legacyCanonical(tokens, 0, len(tokens))
	}
	if a, b := canonical("seed(x + ++y)"), canonical("seed(x++ + y)"); a == b {
		t.Fatalf("distinct operators share canonical text %q", a)
	}
	if a, b := canonical("seed( x+ ++y )"), canonical("seed(x + ++ y)"); a != b {
		t.Fatalf("layout changed canonical text: %q / %q", a, b)
	}
	if a, b := canonical("return\n1"), canonical("return 1"); a == b {
		t.Fatalf("an ASI-terminated return shares canonical text %q", a)
	}
	if a, b := canonical("a\n++b"), canonical("a++b"); a == b {
		t.Fatalf("an ASI-separated prefix increment shares canonical text %q", a)
	}
	if a, b := canonical("return  x"), canonical("return x"); a != b {
		t.Fatalf("layout changed a return: %q / %q", a, b)
	}
	for src, want := range map[string]string{"'a\\\u2028b'": "ab", "'a\\\u2029b'": "ab", "`a\r\nb\rc`": "a\nb\nc"} {
		if tokens, _, ok := lexLegacy(src); !ok || len(tokens) != 1 || tokens[0].text != want {
			t.Fatalf("line terminator in %q: %v %+v", src, ok, tokens)
		}
	}
	tokens, _, ok := lexLegacy(`'\uD83D\uDE00'`)
	if !ok || len(tokens) != 1 || tokens[0].text != "\U0001F600" {
		t.Fatalf("surrogate pair: %v %+v", ok, tokens)
	}
	for _, src := range []string{`'\uD800'`, `'\uDC00\uD800'`, `'\u{D800}'`, `'\uD83Dx'`} {
		if _, _, ok := lexLegacy(src); ok {
			t.Fatalf("lone surrogate %s accepted", src)
		}
	}
}

// TestLegacyResidueFailsClosed pins the round-4 repairs: every JavaScript line terminator ends a
// comment and separates statements, and an assertion, command or precondition that the static
// reader did not consume becomes an unknown that keeps its case from parity (DCP-V1-062, DCP-V1-063).
func TestLegacyResidueFailsClosed(t *testing.T) {
	t.Parallel()
	root, revision := legacyRepo(t)
	base, _, err := legacyInventory(t, root, "mocha", revision, "e2e/cypress", nil)
	if err != nil {
		t.Fatal(err)
	}
	registry := legacyRegistry(t, base, nil)
	source, err := os.ReadFile(filepath.Join(root, "e2e", "cypress", "checkout.cy.js"))
	if err != nil {
		t.Fatal(err)
	}
	baseTuples := legacyTuples(legacyCaseNamed(t, base, "shows the cart total"))
	// with commits the case "shows the cart total" with snippet appended to its body.
	edit := func(t *testing.T, old, replacement string) (LegacyInventory, BehaviorLegacyCase) {
		t.Helper()
		if !strings.Contains(string(source), old) {
			t.Fatalf("fixture lacks %q", old)
		}
		legacyWrite(t, root, "e2e/cypress/checkout.cy.js", strings.Replace(string(source), old, replacement, 1))
		git(t, root, "commit", "-qam", "residue")
		inventory, _, err := legacyInventory(t, root, "mocha", git(t, root, "rev-parse", "HEAD"), "e2e/cypress", registry)
		if err != nil {
			t.Fatalf("%q: %v", replacement, err)
		}
		return inventory, legacyCaseNamed(t, inventory, "shows the cart total")
	}
	with := func(t *testing.T, snippet string) (LegacyInventory, BehaviorLegacyCase) {
		t.Helper()
		return edit(t, "    cy.contains('Pay now');\n", "    cy.contains('Pay now');\n    "+snippet+"\n")
	}
	blocked := func(t *testing.T, snippet string, inventory LegacyInventory, legacy BehaviorLegacyCase) {
		t.Helper()
		if !slices.Contains(legacyUnknownReasons(inventory, legacy.ID), "not-allowlisted") {
			t.Fatalf("%q: the allowlist admitted it: %v", snippet, legacyUnknownReasons(inventory, legacy.ID))
		}
		check := inventory.Crosswalk.ReviewedChecks[0]
		if check.Verdict == "parity-candidate" || !slices.Contains(check.Reasons, "legacy-case-has-unknowns") {
			t.Fatalf("%q reached parity: %+v", snippet, check)
		}
	}
	t.Run("DCP-V1-062 every line terminator ends a comment and an ASI statement", func(t *testing.T) {
		for _, terminator := range []string{"\r", "\u2028", "\u2029", "\r\n"} {
			snippet := "// hidden" + terminator + "expect(v).toBe(9);"
			_, legacy := with(t, snippet)
			if tuples := legacyTuples(legacy); !slices.Contains(tuples, "c3 toBe | v | 9") || len(tuples) != len(baseTuples)+1 {
				t.Fatalf("%q: assertion hidden behind a comment: %q", snippet, tuples)
			}
		}
		canonical := func(src string) string {
			tokens, _, ok := lexLegacy(src)
			if !ok {
				t.Fatalf("lex refused %q", src)
			}
			return legacyCanonical(tokens, 0, len(tokens))
		}
		for _, terminator := range []string{"\r", "\u2028", "\u2029", "\r\n"} {
			if canonical("seed(() => { return"+terminator+"1 })") == canonical("seed(() => { return 1 })") {
				t.Fatalf("return%q1 shares canonical text with return 1", terminator)
			}
		}
		if tokens, _, ok := lexLegacy("1..should.equal(1)"); !ok || tokens[0].text != "1." || tokens[2].text != "should" {
			t.Fatalf("number scan swallowed members: %v %+v", ok, tokens)
		}
		if _, _, ok := lexLegacy("1.toString()"); ok {
			t.Fatal("an identifier directly after a number was accepted")
		}
		if _, _, ok := lexLegacy("a = /x\u2028/"); ok {
			t.Fatal("a regex literal spanning U+2028 was accepted")
		}
	})
	t.Run("DCP-V1-062 a computed cypress command is never dropped", func(t *testing.T) {
		snippet := "cy['get']('nav').contains('Orders');"
		inventory, legacy := with(t, snippet)
		blocked(t, snippet, inventory, legacy)
	})
	t.Run("DCP-V1-062 assertions in unproven nested functions stay unknown", func(t *testing.T) {
		for _, snippet := range []string{
			"const check = (() => expect(x).toBe(1));",
			"const o = { check() { expect(x).toBe(1); } };",
			"const g = { get v() { return expect(x).toBe(1); } };",
			"class K { m() { expect(x).toBe(1); } }",
			"(function () { expect(x).toBe(1); })();",
			"(() => { expect(x).toBe(1); })();",
			"const o = { 'k'() { cy.get('.k').should('exist'); } };",
		} {
			inventory, legacy := with(t, snippet)
			if tuples := legacyTuples(legacy); !slices.Equal(tuples, baseTuples) {
				t.Fatalf("%q: a helper assertion became a criterion: %q", snippet, tuples)
			}
			blocked(t, snippet, inventory, legacy)
		}
	})
	t.Run("DCP-V1-063 a simple computed precondition resolves and any other stays unknown", func(t *testing.T) {
		a, legacyA := with(t, "cy['task']('seed', 'A');")
		_, legacyB := with(t, "cy['task']('seed', 'B');")
		if len(legacyA.Fixtures) != len(base.Legacy[0].Fixtures)+1 || !slices.Contains(legacyA.Fixtures, "setup:cy.task@sha256:"+Digest([]byte("cy['task']('seed','A')"))) {
			t.Fatalf("computed task: %v", legacyA.Fixtures)
		}
		if slices.Equal(legacyA.Fixtures, legacyB.Fixtures) {
			t.Fatal("different seeds share a fixture identity")
		}
		if check := a.Crosswalk.ReviewedChecks[0]; check.Verdict == "parity-candidate" {
			t.Fatalf("an unmatched seed reached parity: %+v", check)
		}
		if _, legacy := with(t, "cy[`task`]('seed', 'A');"); !slices.Contains(legacy.Fixtures, "setup:cy.task@sha256:"+Digest([]byte("cy[`task`]('seed','A')"))) {
			t.Fatalf("a substitution-free template member did not resolve: %v", legacy.Fixtures)
		}
		for _, snippet := range []string{"cy[m]('seed', 'A');", "cy['ta' + 'sk']('seed', 'A');", "cy[`ta${'sk'}`]('seed', 'A');"} {
			inventory, legacy := with(t, snippet)
			blocked(t, snippet, inventory, legacy)
		}
	})
	t.Run("DCP-V1-062 DCP-V1-063 novel forms leave residue", func(t *testing.T) {
		for _, snippet := range []string{
			"cy?.get('x').should('exist');",
			"cy.get('x')?.should('exist');",
			"(0, expect)(a).toBe(1);",
			"const c = cy; c.get('x');",
			"globalThis.expect(a).toBe(1);",
			"const e = expect; e(a).toBe(1);",
			"cy.request('/api/reset');",
			"const t = cy.task; t('seed');",
			"window['cy'].get('x');",
			"x['ex' + 'pect'](a).toBe(1);",
			"window.cy.task('seed');",
			"it('nested', () => {});",
		} {
			inventory, legacy := with(t, snippet)
			blocked(t, snippet, inventory, legacy)
		}
	})
	t.Run("DCP-V1-062 DCP-V1-063 round-5 variants fail closed", func(t *testing.T) {
		// A keyword property name ends an expression, so this is division and the assertion runs.
		snippet := "obj.return / (expect(hidden).toBe(1), 2) / 2;"
		inventory, legacy := with(t, snippet)
		if !slices.Contains(legacyTuples(legacy), "c3 toBe | hidden | 1") {
			t.Fatalf("keyword member hid an assertion: %q", legacyTuples(legacy))
		}
		blocked(t, snippet, inventory, legacy)
		for _, snippet := range []string{
			"1..should.equal(1);",
			"cy.task[method](null, 'seed', 'A');",
			"globalThis[('expect')](hidden).toBe(1);",
			"`${globalThis['expect'](hidden).toBe(1)}`;",
			"globalThis.browser.get('/seed-A');",
			"globalThis.page.goto('/seed-A');",
		} {
			inventory, legacy := with(t, snippet)
			blocked(t, snippet, inventory, legacy)
		}
		for _, snippet := range []string{
			"const helper = { return() { expect(hidden).toBe(1); } };",
			"const helper = { if() { expect(hidden).toBe(1); } };",
		} {
			inventory, legacy := with(t, snippet)
			if tuples := legacyTuples(legacy); !slices.Equal(tuples, baseTuples) {
				t.Fatalf("%q: an uncalled method became a criterion: %q", snippet, tuples)
			}
			blocked(t, snippet, inventory, legacy)
		}
		hook := "  beforeEach.call(null, () => { cy.task('seed', 'A'); });\n"
		inventory, legacy = edit(t, "describe('Checkout', () => {\n", "describe('Checkout', () => {\n"+hook)
		if !slices.Contains(legacyUnknownReasons(inventory, legacy.ID), "hook-not-static") {
			t.Fatalf("an uninventoried hook spelling left no doubt: %v", legacyUnknownReasons(inventory, legacy.ID))
		}
		blocked(t, hook, inventory, legacy)
	})
	t.Run("DCP-V1-062 DCP-V1-063 the closed-world allowlist refuses the round-6 variants", func(t *testing.T) {
		for _, snippet := range []string{
			"let of = 8; of / (expect(hidden).toBe(1), 2) / 2;",
			"c\\u0079.task('seed', 'A');",
			"cy.task[0]('seed', 'A');",
			"cy.task['x-y']('seed', 'A');",
			"globalThis['ex' + 'pect'](hidden)['toBe'](1);",
			"const p = page; p.goto('/seed-A');",
			"checkHidden(); expect(visible).toBe(2);",
			"cy.visit('/a')\n(seed)('A');",
			"cy.get('.n');",
			"expect(hidden).toThrow();",
			"expect(hidden).toBe;",
			"expect(rows()).toBe(1);",
			"cy.visit('/a', { onBeforeLoad: seed });",
			"await\ncy.visit('/a');",
		} {
			inventory, legacy := with(t, snippet)
			blocked(t, snippet, inventory, legacy)
		}
		for _, hook := range []string{
			"  globalThis.beforeEach(() => { seed('A'); });\n",
			"  globalThis['beforeEach'](() => { seed('A'); });\n",
			"  beforeEach(() => { seed('A'); });\n",
			"  afterEach(() => { expect(hidden).toBe(1); });\n",
			"  context('other', (cy) => {});\n",
		} {
			inventory, legacy := edit(t, "describe('Checkout', () => {\n", "describe('Checkout', () => {\n"+hook)
			blocked(t, hook, inventory, legacy)
		}
		for _, preamble := range []string{"import { seed } from './seed';\n", "seed('A');\n"} {
			inventory, legacy := edit(t, "describe('Checkout', () => {\n", preamble+"describe('Checkout', () => {\n")
			blocked(t, preamble, inventory, legacy)
		}
		inventory, legacy := edit(t, "it('shows the cart total', () => {", "it('shows the cart total', (done) => {")
		blocked(t, "a case callback parameter", inventory, legacy)
		// Admitted forms keep the case a parity candidate.
		admitted := "expect(a.b).toBe(-1);\n    expect(c).to.exist\n    cy.get('a').first().should('have.text', 'A').and('be.visible')"
		inventory, legacy = with(t, admitted)
		if reasons := legacyUnknownReasons(inventory, legacy.ID); len(reasons) != 0 || len(legacy.Criteria) != len(baseTuples)+4 || inventory.Crosswalk.ReviewedChecks[0].Verdict != "parity-candidate" {
			t.Fatalf("admitted forms were refused: %v %q %+v", reasons, legacyTuples(legacy), inventory.Crosswalk.ReviewedChecks[0])
		}
		inventory, legacy = with(t, "cy.visit('/x', { timeout: 1000 });")
		if reasons := legacyUnknownReasons(inventory, legacy.ID); len(reasons) != 0 || len(legacy.Fixtures) != len(base.Legacy[0].Fixtures)+1 {
			t.Fatalf("an admitted precondition was refused: %v %v", reasons, legacy.Fixtures)
		}
	})
	t.Run("DCP-V1-062 DCP-V1-063 the allowlist refuses the round-7 variants", func(t *testing.T) {
		for _, snippet := range []string{
			// A chainer that calls its subject, or any unlisted chainer word.
			"cy.get('body').its('0.ownerDocument.defaultView.checkHidden').should('not.throw');",
			"cy.get('a').should('satisfy', 1);",
			"cy.get('a').should('change', 'x');",
			"cy.get('a').should('increase', 'x');",
			"cy.get('a').should('be.calledWith', 'x');",
			// An options argument the criterion does not record.
			"cy.contains('capital sentence', { matchCase: false });",
			"cy.contains('a', 'Orders', { matchCase: false });",
			// A continuation after a chainer that yields a new subject.
			"cy.get('a').should('have.attr', 'href').and('eq', '/users');",
			"cy.get('a').should('have.prop', 'value').should('eq', 'x');",
			"cy.get('a').should('have.length', 2).and('be.visible');",
			// A string value that reads like another literal's canonical text.
			"expect(one).toBe('1');",
			"expect(flag).toBe('true');",
			"expect(nothing).toBe('(no expected value)');",
			"cy.get('a').should('have.text', '[1]');",
			"cy.contains('-1');",
			// Completion order the source order does not show.
			"await page.goto('/seed');",
			"await cy.visit('/a');",
		} {
			inventory, legacy := with(t, snippet)
			blocked(t, snippet, inventory, legacy)
		}
		for _, hook := range []string{
			"  beforeEach(() => cy.visit('/x'));\n",
			"  beforeEach(async () => { cy.visit('/x'); });\n",
		} {
			inventory, legacy := edit(t, "describe('Checkout', () => {\n", "describe('Checkout', () => {\n"+hook)
			blocked(t, hook, inventory, legacy)
		}
		inventory, legacy := edit(t, "it('shows the cart total', () => {", "it('shows the cart total', async () => {")
		blocked(t, "an async case callback", inventory, legacy)
		// Round 9: a query after contains with no should/and, and any negated Cypress chainer.
		for _, snippet := range []string{
			"cy.contains('Pay now').find('.required');",
			"cy.contains('Pay now').find('.required').first().should('be.visible');",
			"cy.contains('Pay now').should('not.exist');",
			"cy.get('a').should('not.be.visible');",
			"cy.get('a').should('be.visible').and('not.have.class', 'x');",
		} {
			inventory, legacy := with(t, snippet)
			blocked(t, snippet, inventory, legacy)
		}
		// A query after contains that a should directly follows stays admitted.
		inventory, legacy = with(t, "cy.contains('Pay now').find('.required').should('be.visible');")
		if reasons := legacyUnknownReasons(inventory, legacy.ID); len(reasons) != 0 {
			t.Fatalf("a checked query after contains was refused: %v", reasons)
		}
		// Preconditions over the record bound fail the run rather than emit an unloadable list.
		tasks := strings.Builder{}
		for n := 0; n <= MaxRecords; n++ {
			fmt.Fprintf(&tasks, "cy.task(%d);", n)
			if n%64 == 63 {
				tasks.WriteString("\n")
			}
		}
		legacyWrite(t, root, "e2e/cypress/checkout.cy.js", strings.Replace(string(source), "    cy.contains('Pay now');\n", "    cy.contains('Pay now');\n"+tasks.String()+"\n", 1))
		git(t, root, "commit", "-qam", "over-bound preconditions")
		if _, _, err := legacyInventory(t, root, "mocha", git(t, root, "rev-parse", "HEAD"), "e2e/cypress", registry); err == nil || !strings.Contains(err.Error(), "record bound") {
			t.Fatalf("over-bound preconditions did not fail the run: %v", err)
		}
		// Round 10: chainers are whole strings from a closed set of positive assertions; a
		// composed, negated, zero-length or unlisted chainer, in should/and or expect, is refused.
		for _, snippet := range []string{
			"cy.contains('Pay now').should('have.length', 0);",
			"cy.get('a').should('have.length', 1.5);",
			"cy.url().should('exist.and.be.a', 'number');",
			"cy.url().should('be.a', 'string');",
			"cy.get('a').should('be.visible.and.have.text', 'A');",
			"cy.get('a').should('have.text');",
			"cy.get('a').should('exist', 'A');",
			"expect(a.b).not.toBe(-1);",
			"expect(c).to.be.true;",
			"expect(c).to.not.exist;",
			"expect(c).to.equal(1);",
			"expect(c).to.exist.and.be.a('string');",
			"expect(c).to.have.length(0);",
			"expect(c).to.be.visible();",
			"expect(c).to.have.text;",
			"expect(rows).toHaveLength(0);",
			"expect(rows).toHaveSize(-1);",
		} {
			inventory, legacy := with(t, snippet)
			blocked(t, snippet, inventory, legacy)
		}
		inventory, legacy = with(t, "cy.get('a').should('have.length', 2);\n    expect(c).to.have.attr('href', '/a');\n    expect(rows).toHaveLength(3);")
		if reasons := legacyUnknownReasons(inventory, legacy.ID); len(reasons) != 0 {
			t.Fatalf("exact chainers were refused: %v", reasons)
		}
		inventory, legacy = with(t, "cy.get('a').should('have.length', 2).and('be.visible');")
		blocked(t, "and after have.length", inventory, legacy)
		// Round 8: a chainer or expect chain of language words alone asserts nothing.
		for _, snippet := range []string{
			"cy.url().should('to.be');",
			"cy.get('a').should('be.visible').and('not.have');",
			"cy.get('a').should('deep.nested.own');",
			"expect(a).to.be;",
			"expect(a).to.not.have;",
		} {
			inventory, legacy := with(t, snippet)
			blocked(t, snippet, inventory, legacy)
		}
		// An unambiguous string value stays admitted.
		inventory, legacy = with(t, "expect(label).toBe('Orders');")
		if reasons := legacyUnknownReasons(inventory, legacy.ID); len(reasons) != 0 || inventory.Crosswalk.ReviewedChecks[0].Verdict != "parity-candidate" {
			t.Fatalf("an unambiguous value was refused: %v %+v", reasons, inventory.Crosswalk.ReviewedChecks[0])
		}
		// A case over the registry's per-case criterion bound is unknown, never truncated. The
		// statements are short enough for the case span to stay within the anchor bound.
		inventory, legacy = with(t, strings.Repeat(strings.Repeat("expect(a).ok;", 64)+"\n", MaxRecords/64)+"expect(a).ok;")
		if reasons := legacyUnknownReasons(inventory, legacy.ID); !slices.Contains(reasons, "criteria-over-bound") || len(legacy.Criteria) != 0 || inventory.Crosswalk.ReviewedChecks[0].Verdict == "parity-candidate" {
			t.Fatalf("an over-bound case was not refused: %v %d %+v", reasons, len(legacy.Criteria), inventory.Crosswalk.ReviewedChecks[0])
		}
	})
}
