package changee2e

import (
	"bytes"
	"strconv"
	"strings"
	"testing"
)

const harvestExtraSpec = `import { test, expect } from '@playwright/test';

test('extra', async ({ page }) => {
  await page.goto('/#!/login');
  await expect(page.getByTestId('slot-status')).toHaveText('Booked');
  await expect(page.getByTestId('api-field')).toHaveValue('token-123');
  await expect(page.getByTestId('note')).toHaveText('');
  await expect(page.getByTestId('note'), 'with a message').toHaveText('Open');
});
`

func harvestByID(p *Proposals) map[string]Proposal {
	m := map[string]Proposal{}
	for _, pr := range p.Proposals {
		m[pr.Block.Kind+" "+pr.Block.Locator+pr.Block.URL] = pr
	}
	return m
}

// CCE-V0-041, CCE-V0-042: the harvester proposes only literal assertions and navigations from
// tracked test files, is byte-identical across runs, collapses repeats to the first occurrence and
// withholds every other harvestable-looking line with its reason, never emitting a secret.
func TestCCEV0041HarvestDeterministicLiteralOnly(t *testing.T) {
	root, _ := fixtureRepo(t)
	rev := commitFiles(t, root, map[string]*string{"e2e/specs/zz-extra.spec.ts": text(harvestExtraSpec)}, "extra spec")
	a, err := Harvest(bg, HarvestOptions{Root: root, Rev: rev})
	if err != nil {
		t.Fatal(err)
	}
	b, err := Harvest(bg, HarvestOptions{Root: root, Rev: "main"})
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(a.Encoded(), b.Encoded()) || a.Schema != ProposalsSchema || a.Harvester != HarvesterVersion || a.Revision != rev {
		t.Fatalf("harvest not deterministic or not pinned:\n%s", a.Encoded())
	}
	if len(a.Proposals) != 2 {
		t.Fatalf("want exactly the two literal proposals, got:\n%s", a.Encoded())
	}
	got := harvestByID(a)
	oc, ok := got[KindOutcome+" slot-status"]
	if !ok || oc.Block.Matcher != "toHaveText" || oc.Block.Value != "Booked" || oc.Block.Span.Path != "e2e/specs/alias.spec.ts" ||
		oc.Block.Span.Start != 7 || oc.Block.Provenance != "harvested:"+oc.Block.Blob+":7-7" || len(oc.Block.Span.SHA256) != 64 ||
		!strings.HasPrefix(oc.ID, "outcome:harvested:") || oc.ID != oc.Block.ID {
		t.Fatalf("outcome proposal %+v", oc)
	}
	if pth, ok := got[KindPath+" /#!/login"]; !ok || pth.Block.Span.Path != "e2e/specs/login.spec.ts" || pth.Block.Span.Start != 5 {
		t.Fatalf("path proposal %+v", pth)
	}
	withheld := map[string]string{}
	for _, w := range a.Withheld {
		withheld[w.Path+":"+strconv.Itoa(w.Line)] = w.Reason
	}
	want := map[string]string{
		"e2e/specs/login.spec.ts:6":    "harvest-not-literal",
		"e2e/specs/zz-extra.spec.ts:6": "harvest-secret-shaped",
		"e2e/specs/zz-extra.spec.ts:7": "outcome-vacuous",
		"e2e/specs/zz-extra.spec.ts:8": "harvest-not-literal",
	}
	if len(withheld) != len(want) {
		t.Fatalf("withheld %v, want %v", withheld, want)
	}
	for k, v := range want {
		if withheld[k] != v {
			t.Fatalf("withheld %v, want %v", withheld, want)
		}
	}
	if bytes.Contains(a.Encoded(), []byte("token-123")) {
		t.Fatal("a secret-shaped value reached the proposals document")
	}
}

// CCE-V0-043: only a proposal the ledger accepts at the catalog revision becomes a block; an
// unaccepted proposal never reaches the catalog or the compile report, a stale acceptance is
// unadmitted, an invalid ledger refuses, and without a ledger the catalog is unchanged; an
// acceptance the change under test adds is flagged (CCE-V0-044).
func TestCCEV0043UnacceptedProposalNeverCompiles(t *testing.T) {
	root, base := fixtureRepo(t)
	before, err := BuildCatalog(bg, CatalogOptions{Root: root, Rev: base})
	if err != nil {
		t.Fatal(err)
	}
	p, err := Harvest(bg, HarvestOptions{Root: root, Rev: base})
	if err != nil {
		t.Fatal(err)
	}
	got := harvestByID(p)
	accept, other := got[KindOutcome+" slot-status"].ID, got[KindPath+" /#!/login"].ID
	if accept == "" || other == "" {
		t.Fatalf("fixture proposals missing:\n%s", p.Encoded())
	}
	if bytes.Contains(before.Encoded(), []byte(":harvested:")) {
		t.Fatal("a catalog without a ledger carries harvested blocks")
	}
	stale := "path:harvested:0000000000000000"
	ledger := `{"schema":"e2e-acceptances/0","acceptances":[{"proposal_id":"` + accept + `","reason":"reviewed"},` +
		`{"proposal_id":"` + stale + `","reason":"reviewed earlier"}]}` + "\n"
	head := commitFiles(t, root, map[string]*string{AcceptancesPath: text(ledger)}, "accept one proposal")
	cat, err := BuildCatalog(bg, CatalogOptions{Root: root, Rev: head})
	if err != nil {
		t.Fatal(err)
	}
	b := cat.Block(accept)
	if b == nil || b.Relation != "accepted-proposal" || b.Provenance == "" || b.Kind != KindOutcome || b.Blob == "" {
		t.Fatalf("accepted proposal not admitted: %+v\n%s", b, cat.Encoded())
	}
	if cat.Block(other) != nil || bytes.Contains(cat.Encoded(), []byte(other)) {
		t.Fatalf("unaccepted proposal %s reached the catalog", other)
	}
	if reason := cat.unadmittedReason(KindPath, stale, ""); reason != "acceptance-stale" {
		t.Fatalf("stale acceptance reason %q", reason)
	}
	r, err := Compile(bg, DiffOptions{Root: root, Base: base, Head: head})
	if err != nil {
		t.Fatal(err)
	}
	if r.CatalogDigest != cat.Digest || bytes.Contains(r.Encoded(), []byte(other)) {
		t.Fatalf("compile read a catalog other than the head catalog or names %s", other)
	}
	// CCE-V0-044: the acceptance the change itself adds is flagged; once at base it is not.
	if len(r.AcceptancesInDiff) != 1 || r.AcceptancesInDiff[0] != accept {
		t.Fatalf("acceptances in diff %v, want [%s]", r.AcceptancesInDiff, accept)
	}
	later := commitFiles(t, root, map[string]*string{"e2e/specs/zz-later.spec.ts": text("// later\n")}, "later")
	if r2, err := Compile(bg, DiffOptions{Root: root, Base: head, Head: later}); err != nil || len(r2.AcceptancesInDiff) != 0 {
		t.Fatalf("an acceptance already at base was flagged: %v %v", err, r2.AcceptancesInDiff)
	}
	for name, bad := range map[string]string{
		"unknown field":    `{"schema":"e2e-acceptances/0","acceptances":[],"extra":1}`,
		"wrong schema":     `{"schema":"e2e-acceptances/1","acceptances":[]}`,
		"empty reason":     `{"schema":"e2e-acceptances/0","acceptances":[{"proposal_id":"` + accept + `","reason":" "}]}`,
		"not harvested id": `{"schema":"e2e-acceptances/0","acceptances":[{"proposal_id":"input:slot-note","reason":"x"}]}`,
		"duplicate": `{"schema":"e2e-acceptances/0","acceptances":[{"proposal_id":"` + accept + `","reason":"x"},` +
			`{"proposal_id":"` + accept + `","reason":"y"}]}`,
	} {
		rev := commitFiles(t, root, map[string]*string{AcceptancesPath: text(bad + "\n")}, name)
		if _, err := BuildCatalog(bg, CatalogOptions{Root: root, Rev: rev}); refusalCode(err) != "e2e-acceptances-invalid" {
			t.Fatalf("%s: err %v", name, err)
		}
	}
}
