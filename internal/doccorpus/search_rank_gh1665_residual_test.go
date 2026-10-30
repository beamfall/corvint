package doccorpus

import (
	"fmt"
	"strings"
	"testing"
)

// residualSources reproduces the shape of the GitHub #1665 exact-file residual (22/24 on
// ec41e4e and b63dd0d): the method a query names sits in a records-owned file whose path does
// not spell it, and a bulk file earlier in manifest input order (the reporter's
// `app/models/club.rb`) exhausts the derived declaration bound first, so the method's
// declaration is dropped. The descriptive claims are the reporter's claims-suite shape.
var residualSources = map[string]string{
	"src/app/models/club.rb":            residualBulk(30),
	"src/app/services/pos/reconcile.rb": "class Reconcile\n  def sync_with_pos\n    true\n  end\nend\n",
}

var residualClaims = []string{
	"Lightspeed payments are validated before the batch posts",
	"The PLI check validates each Lightspeed payment",
}

func residualBulk(n int) string {
	var b strings.Builder
	b.WriteString("class Club\n")
	for i := range n {
		fmt.Fprintf(&b, "  def member_%02d\n  end\n", i)
	}
	b.WriteString("end\n")
	return b.String()
}

// residualCorpus builds the fixture with a records provider that leaves room for only a few
// derived declarations, and with filter, when not nil, on the native provider (DCS-V1-573).
func residualCorpus(t *testing.T, filter *DerivationFilter) *Artifact {
	t.Helper()
	owned := []string{"src/app/models/club.rb", "src/app/services/pos/reconcile.rb"}
	root, m := ownedSourceCorpusWith(t, residualSources, owned, false, func(p *ProviderRecord) {
		ev := p.Subjects[0].Evidence
		for i := range MaxRecords - 24 {
			p.Subjects = append(p.Subjects, Subject{fmt.Sprintf("adapter:r%04d", i), "capability", "r", "adapter", ev, nil})
		}
	}, residualClaims...)
	for i := range m.Providers {
		if m.Providers[i].Kind == "native" {
			m.Providers[i].Derive = filter
		}
	}
	return buildOwned(t, root, m)
}

// GitHub #1665 residual (DCS-V1-482, DCS-V1-568, DCS-V1-570, DCS-V1-573): without a filter the
// bound drops the declaration of sync_with_pos in manifest input order and the query misses it,
// as the reporter's truncated build did; a derivation filter naming the services directory keeps
// it, so it ranks first, and the claims query still orders the descriptive claims first.
func TestGH1665ResidualTruncatedDeclarationRecoveredByFilter(t *testing.T) {
	t.Parallel()
	const want = "native:symbol:src/app/services/pos/reconcile.rb:2:sync_with_pos"
	t.Run("unfiltered: dropped by the bound", func(t *testing.T) {
		t.Parallel()
		a := residualCorpus(t, nil)
		if derived := len(subjectsWithPrefix(a, "native:symbol:src/app/models/club.rb:")); derived == 0 || derived >= 31 {
			t.Fatalf("the bulk file should take the whole bound: %d derived", derived)
		}
		if got := subjectsWithPrefix(a, "native:symbol:src/app/services/"); len(got) != 0 {
			t.Fatalf("the services declarations should be dropped: %v", got)
		}
		if gap := derivedBoundGap(t, a); !strings.Contains(gap.Reason, "src/app/models/club.rb:") {
			t.Fatalf("dropping should begin in the bulk file: %s", gap.Reason)
		}
		if order := searchOrders(t, a, Request{Operation: "search", Query: "sync_with_pos", Limit: 10}); rankOf(order, want) != 0 {
			t.Fatalf("a dropped declaration cannot be found: %v", order)
		}
	})
	t.Run("filtered: kept and ranked first", func(t *testing.T) {
		t.Parallel()
		a := residualCorpus(t, &DerivationFilter{Paths: []string{"src/app/services/"}})
		if got := subjectsWithPrefix(a, "native:symbol:src/app/models/"); len(got) != 0 {
			t.Fatalf("the filter should exclude the bulk file: %v", got)
		}
		for _, query := range []string{"sync_with_pos", "syncWithPos"} {
			if order := searchOrders(t, a, Request{Operation: "search", Query: query, Limit: 10}); len(order) == 0 || order[0] != want {
				t.Fatalf("search %q: the kept declaration should rank first: %v", query, order)
			}
		}
		if order := claimOrder(t, a, "validate PLI lightspeed payments"); len(order) < 2 || !sameSet(order[:2], []string{"adapter:owner:claim-1", "adapter:owner:claim-2"}) {
			t.Fatalf("claims: the descriptive claims should order first: %v", order)
		}
	})
}

// DCS-V1-575 (V1-2793): when the records anchor the services file rather than the bulk file
// earlier in manifest input order, the services file derives first, so the bound keeps the
// sync_with_pos declaration without a filter; the dropped declarations are the bulk file's,
// which the gap names, and the kept declarations are still added in manifest input order.
func TestDCSV1575AnchoredInputsDeriveFirst(t *testing.T) {
	t.Parallel()
	const want = "native:symbol:src/app/services/pos/reconcile.rb:2:sync_with_pos"
	owned := []string{"src/app/services/pos/reconcile.rb", "src/app/models/club.rb"}
	root, m := ownedSourceCorpusWith(t, residualSources, owned, false, func(p *ProviderRecord) {
		ev := p.Subjects[0].Evidence
		for i := range MaxRecords - 24 {
			p.Subjects = append(p.Subjects, Subject{fmt.Sprintf("adapter:r%04d", i), "capability", "r", "adapter", ev, nil})
		}
	}, residualClaims...)
	a := buildOwned(t, root, m)
	services := subjectsWithPrefix(a, "native:symbol:src/app/services/pos/reconcile.rb:")
	if len(services) != 2 || services[1].ID != want {
		t.Fatalf("the anchored file should derive every declaration first: %v", services)
	}
	club := subjectsWithPrefix(a, "native:symbol:src/app/models/club.rb:")
	if len(club) == 0 || len(club) >= 31 {
		t.Fatalf("the bulk file should fill the rest of the bound: %d derived", len(club))
	}
	gap := derivedBoundGap(t, a)
	if !strings.Contains(gap.Reason, "dropped from src/app/models/club.rb:") || !strings.Contains(gap.Reason, "in derivation order (inputs records anchor first, then manifest input order)") {
		t.Fatalf("the gap should name the first bulk declaration dropped in derivation order: %s", gap.Reason)
	}
	firstServices, lastClub := -1, -1
	for i, s := range a.Subjects {
		switch {
		case strings.HasPrefix(s.ID, "native:symbol:src/app/models/club.rb:"):
			lastClub = i
		case strings.HasPrefix(s.ID, "native:symbol:src/app/services/") && firstServices < 0:
			firstServices = i
		}
	}
	if firstServices < lastClub {
		t.Fatalf("kept declarations should be added in manifest input order: services at %d, club up to %d", firstServices, lastClub)
	}
	if order := searchOrders(t, a, Request{Operation: "search", Query: "sync_with_pos", Limit: 10}); len(order) == 0 || order[0] != want {
		t.Fatalf("the kept declaration should rank first: %v", order)
	}
}
