package main

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/doccorpus"
)

const legacyCLISuite = `describe('cart', () => {
  beforeEach(() => { cy.task('seed', 'admin-users'); cy.visit('/cart'); });
  it('shows the total', () => { cy.get('[data-test=total]').should('have.text', '$10.00'); });
  it.skip('applies a coupon', () => { cy.get('#coupon').should('be.visible'); });
});
`

func legacyCLIRepository(t *testing.T) (string, string) {
	t.Helper()
	root := t.TempDir()
	cemGit(t, root, "init", "-q")
	cemWrite(t, root, "e2e/cart.cy.js", legacyCLISuite)
	cemGit(t, root, "add", ".")
	cemGit(t, root, "commit", "-qm", "legacy")
	return root, cemGit(t, root, "rev-parse", "HEAD")
}

func TestDocsCorpusDiscoverLegacyCLI(t *testing.T) {
	root, revision := legacyCLIRepository(t)
	t.Run("DCP-V1-069 CLI bytes equal the producer", func(t *testing.T) {
		code, out, stderr := corpusCLI(t, root, "docs", "corpus", "discover-legacy", "--runner", "mocha", "--revision", revision, "--path", "e2e")
		if code != 0 {
			t.Fatalf("exit %d: %s", code, stderr)
		}
		want, err := doccorpus.BuildLegacyInventory(context.Background(), doccorpus.LegacyInventoryInput{Root: root, Runner: "mocha", Revision: revision, Path: "e2e"})
		if err != nil || out != string(want) {
			t.Fatalf("CLI output differs from the producer: %v\n%s\n%s", err, out, want)
		}
		var inventory doccorpus.LegacyInventory
		if err := json.Unmarshal([]byte(out), &inventory); err != nil || inventory.Status != "generated" || len(inventory.Legacy) != 2 {
			t.Fatalf("inventory = %+v, %v", inventory, err)
		}
	})
	t.Run("DCP-V1-069 refused reviewed mapping exits 1 with the report", func(t *testing.T) {
		_, out, _ := corpusCLI(t, root, "docs", "corpus", "discover-legacy", "--runner", "mocha", "--revision", revision, "--path", "e2e")
		var inventory doccorpus.LegacyInventory
		if err := json.Unmarshal([]byte(out), &inventory); err != nil {
			t.Fatal(err)
		}
		total := inventory.Legacy[0]
		if total.Case != "shows the total" || len(total.Criteria) != 1 {
			t.Fatalf("first case = %+v", total)
		}
		criterion := total.Criteria[0]
		registry := doccorpus.BehaviorRegistry{Schema: 2, ContractID: "legacy-crosswalk", ContractSHA256: doccorpus.Digest([]byte("contract")), SourceRevision: revision, DocumentationRevision: revision,
			Tests: []doccorpus.BehaviorTest{{ID: "e2e/cart.spec.ts:total", Title: "cart total", Criteria: []string{"total"}, Fixtures: total.Fixtures,
				Assertions: []doccorpus.BehaviorAssertion{{ID: "a1", Criterion: "total", Matcher: criterion.Matcher, Locator: criterion.Locator, Value: "$11.00"}},
				Legacy:     []doccorpus.BehaviorLegacyMapping{{Criterion: "total", LegacyCase: total.ID, LegacyCriterion: criterion.ID, Relation: "same", Review: doccorpus.Anchor{Kind: "review", Revision: revision}}}}}}
		data, err := doccorpus.Encode(registry)
		if err != nil {
			t.Fatal(err)
		}
		cemWrite(t, root, "registry.json", string(data))
		code, out, stderr := corpusCLI(t, root, "docs", "corpus", "discover-legacy", "--runner", "mocha", "--revision", revision, "--path", "e2e", "--input", "registry.json")
		if code != 1 || !strings.Contains(out, `"verdict":"refused"`) || !strings.Contains(out, "expected-value-differs") {
			t.Fatalf("exit %d\n%s\n%s", code, out, stderr)
		}
	})
	t.Run("DCP-V1-069 usage refusals", func(t *testing.T) {
		for _, args := range [][]string{
			{"--runner", "jest", "--revision", revision, "--path", "e2e"},
			{"--revision", revision, "--path", "e2e"},
			{"--runner", "mocha", "--path", "e2e"},
			{"--runner", "mocha", "--revision", revision},
			{"--runner", "mocha", "--revision", revision, "--path", "e2e", "--report", "x.json"},
		} {
			code, out, _ := corpusCLI(t, root, append([]string{"docs", "corpus", "discover-legacy"}, args...)...)
			if code != 2 || out != "" {
				t.Fatalf("%v: exit %d out %q", args, code, out)
			}
		}
	})
	t.Run("DCP-V1-069 help names the producer", func(t *testing.T) {
		if !strings.Contains(docsHelp, "docs corpus discover-legacy --runner jasmine|mocha --revision FULL_COMMIT --path PATH [--input REGISTRY.json]") || !strings.Contains(docsHelp, "DCP-V1-060..069") {
			t.Fatal("docs help does not name discover-legacy")
		}
	})
}
