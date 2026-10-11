package doccorpus

import (
	"bytes"
	"context"
	"errors"
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
	data, err := BuildLegacyInventory(context.Background(), LegacyInventoryInput{Root: root, Runner: runner, Revision: revision, Path: path, Registry: registry})
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
		if len(total.Fixtures) != 3 || len(login.Fixtures) != 4 || !slices.IsSorted(login.Fixtures) {
			t.Fatalf("fixtures: %v / %v", total.Fixtures, login.Fixtures)
		}
		for _, fixture := range total.Fixtures {
			if !slices.Contains(login.Fixtures, fixture) {
				t.Fatalf("nested case lost inherited hook %s", fixture)
			}
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

func legacyRegistry(t *testing.T, inventory LegacyInventory, edit func(*BehaviorRegistry)) []byte {
	t.Helper()
	total := legacyCaseNamed(t, inventory, "shows the cart total")
	c1 := total.Criteria[0]
	registry := BehaviorRegistry{
		Flows: []BehaviorFlow{{ID: "checkout-cart-total", RequiredPages: []string{"/checkout"}}, {ID: "account-settings", RequiredPages: []string{"/settings"}}},
		Tests: []BehaviorTest{
			{ID: "e2e/cart.spec.ts:cart total", Title: "cart total is shown", Criteria: []string{"total"}, Fixtures: slices.Clone(total.Fixtures),
				Assertions: []BehaviorAssertion{{ID: "a1", Criterion: "total", Matcher: c1.Matcher, Locator: c1.Locator, Value: c1.Value}},
				Legacy:     []BehaviorLegacyMapping{{Criterion: "total", LegacyCase: total.ID, LegacyCriterion: c1.ID, Relation: "same"}}},
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
		if bytes.Contains(first, []byte(`"legacy_criteria"`)) || bytes.Contains(first, []byte(`"review"`)) {
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
}

// TestLegacyPreconditionDigestCompileGap shows the behavior compiler keeps a changed seed
// digest from becoming runtime parity (DCP-V1-067).
func TestLegacyPreconditionDigestCompileGap(t *testing.T) {
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
		} {
			_, data, err := legacyInventory(t, root, tc.runner, tc.revision, tc.path, tc.registry)
			var refusal *Error
			if !errors.As(err, &refusal) || data != nil {
				t.Fatalf("%s: expected a named refusal, got %v", tc.name, err)
			}
		}
		legacyWrite(t, root, "e2e/broken/broken.spec.js", "describe('broken', () => {\n  it('never closes', () => {\n")
		legacyWrite(t, root, "e2e/broken/empty.spec.js", "// no cases here\nconst value = 1;\n")
		legacyWrite(t, root, "e2e/plain/helpers.js", "export const value = 1;\n")
		git(t, root, "add", ".")
		git(t, root, "commit", "-qm", "broken")
		broken := git(t, root, "rev-parse", "HEAD")
		if _, data, err := legacyInventory(t, root, "mocha", broken, "e2e/plain", nil); data != nil || err == nil || !strings.Contains(err.Error(), "no test files") {
			t.Fatalf("directory without test files: %v", err)
		}
		inventory, _, err := legacyInventory(t, root, "mocha", broken, "e2e/broken", nil)
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
