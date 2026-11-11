// SPDX-License-Identifier: Apache-2.0

package appflows

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"slices"
	"strings"

	"github.com/Beamfall/corvint/internal/liveverify/affected"
	"github.com/Beamfall/corvint/internal/liveverify/affected/typescript"
)

// Unlinked-test members of application-flow-gaps/1 (AFU-V1-150 to AFU-V1-154, V1-5255). The code
// appears only in the report's unlinked_tests list, never inside a flow's gaps, so the AFU-V1-016
// per-flow code set is unchanged.
const (
	GapUnlinkedTest  = "unlinked-test"
	MaxUnlinkedTests = 1024

	InventoryAbsent     = "absent"
	InventoryComplete   = "complete"
	InventoryPartial    = "partial"
	InventoryStale      = "stale"
	InventoryUnverified = "unverified"
)

// InventoryState says which E2E inventory the gaps report compared against and how complete it is
// (AFU-V1-153). An absent inventory lists no unlinked test and is never a complete answer.
type InventoryState struct {
	State   string `json:"state"`
	Source  string `json:"source"`
	Tests   int    `json:"tests"`
	Reason  string `json:"reason"`
	BoundTo string `json:"bound_to,omitempty"`
}

// InventoryProvider is the application-flow-selection-provider/1 document the gaps report reads
// its E2E inventory from, with the repository-relative name it was read by.
type InventoryProvider struct {
	Name     string
	Provider SelectionProvider
}

// ReadInventoryProvider reads the repository-relative provider under root as `corvint affected
// --provider` does (closed, bounded, secret-screened) and refuses one that declares a flows
// directory other than dir, because its inventory would then be compared against the wrong flows.
func ReadInventoryProvider(root, name, dir string) (*InventoryProvider, error) {
	var p SelectionProvider
	if err := readSelectionDocument(root, name, &p, maskFlowTierKeys); err != nil {
		return nil, fmt.Errorf("--provider %s: %v", name, err)
	}
	if err := validSelectionProvider(p); err != nil {
		return nil, fmt.Errorf("--provider %s: %v", name, errInvalidContent)
	}
	if strings.TrimSuffix(filepath.ToSlash(p.Flows), "/") != strings.TrimSuffix(filepath.ToSlash(dir), "/") {
		return nil, fmt.Errorf("--provider %s declares flows %s, not --flows %s", name, p.Flows, dir)
	}
	return &InventoryProvider{Name: name, Provider: p}, nil
}

// absentInventory is the state of a gaps report produced without --provider.
func absentInventory() InventoryState {
	return InventoryState{State: InventoryAbsent, Reason: "no --provider was given, so no E2E inventory was compared and unlinked tests were not evaluated"}
}

// state binds the provider's inventory to HEAD through its declared discovery receipt, when it has
// one; without one, completeness is the provider author's attestation (AFU-V1-153).
func (p *InventoryProvider) state(root string, at head) InventoryState {
	s := InventoryState{Source: "provider:" + p.Name, Tests: len(p.Provider.Inventory)}
	receipt := p.Provider.Discovery
	if receipt == "" {
		s.State = InventoryComplete
		s.Reason = "the provider declares no discovery receipt; the inventory's completeness is the provider author's attestation"
		return s
	}
	raw, err := ReadFile(filepath.Join(root, filepath.FromSlash(receipt)))
	if err != nil {
		raw = nil
	}
	// The declared generated paths are pruned from the source walk, as the e2e-safe selection
	// prunes them, so the receipt binds the same way here (AFP-V0-444).
	if _, generated, err := affected.ReadGeneratedDeclaration(root); err == nil && len(generated) != 0 {
		if release, err := affected.HoldIgnoredExclusion(root, generated); err == nil {
			defer release()
		}
	}
	units, state := typescript.VerifyPlaywrightDiscovery(root, p.Provider.RunnerConfig, at.commit, raw)
	switch state {
	case "MATCHED":
		differences := inventoryDifferences(p.Provider.Inventory, units)
		if len(differences) == 0 {
			s.State = InventoryComplete
			s.Reason = "discovery receipt " + receipt + " is bound to HEAD and lists exactly the inventory"
			return s
		}
		s.State = InventoryPartial
		s.Reason = fmt.Sprintf("discovery receipt %s is bound to HEAD but differs from the inventory at %d tests: %s", receipt, len(differences), firstFew(differences, 3))
	case "BINDING_MISMATCH":
		if bound := receiptRevision(raw); bound != "" && bound != at.commit {
			s.State, s.BoundTo = InventoryStale, bound
			s.Reason = "discovery receipt " + receipt + " is bound to revision " + bound + ", not HEAD " + at.commit
			return s
		}
		s.State = InventoryUnverified
		s.Reason = "discovery receipt " + receipt + " names HEAD but does not bind to its runner config and sources; completeness is not verified"
	default:
		s.State = InventoryUnverified
		s.Reason = "discovery receipt " + receipt + " is " + state + "; completeness is not verified"
	}
	return s
}

// receiptRevision reads only the revision a discovery receipt claims; raw is already bounded by
// ReadFile and an unreadable receipt has none.
func receiptRevision(raw []byte) string {
	var receipt struct {
		Revision string `json:"revision"`
	}
	if json.Unmarshal(raw, &receipt) != nil {
		return ""
	}
	return receipt.Revision
}

// inventoryDifferences names each discovered test the inventory lacks and each inventoried test
// discovery lacks, as the e2e-safe selection's e2e-inventory-incomplete subjects do.
func inventoryDifferences(inventory []InventoryTest, discovered []typescript.PlaywrightDiscoveryUnit) []string {
	declared := map[typescript.PlaywrightDiscoveryUnit]bool{}
	for _, t := range inventory {
		declared[typescript.PlaywrightDiscoveryUnit{Project: t.Project, Test: t.Path}] = true
	}
	differences := []string{}
	for _, u := range discovered {
		if !declared[u] {
			differences = append(differences, "not-inventoried:"+u.Project+":"+u.Test)
		}
		delete(declared, u)
	}
	for u := range declared {
		differences = append(differences, "not-discovered:"+u.Project+":"+u.Test)
	}
	slices.Sort(differences)
	return differences
}

func firstFew(values []string, n int) string {
	if len(values) <= n {
		return strings.Join(values, ", ")
	}
	return strings.Join(values[:n], ", ") + fmt.Sprintf(" and %d more", len(values)-n)
}

// unlinkedTests lists the inventory tests no flow variation links with a non-inferred test link,
// in test-key then project order, bounded at MaxUnlinkedTests with the full count (AFU-V1-150,
// AFU-V1-152). An inferred link never counts as a link, the rule no-test applies (AFU-V1-009), so a
// test whose only links are inferred is listed and its detail says so (AFU-V1-252).
func unlinkedTests(set IntentSet, links []EvaluatedLink, inventory []InventoryTest) ([]Gap, int) {
	declared, inferred := map[string]bool{}, map[string]bool{}
	for _, intent := range set.Flows {
		variations := map[string]bool{}
		for _, v := range intent.Variations {
			variations[v.VariationID] = true
		}
		for _, l := range links {
			if l.Flow != intent.FlowID || l.Target.Type != "test" || !variations[l.From] {
				continue
			}
			if l.ReviewState == ReviewInferred {
				inferred[l.Target.TestKey] = true
			} else {
				declared[l.Target.TestKey] = true
			}
		}
	}
	gaps := []Gap{}
	for _, t := range inventory {
		if declared[t.TestKey] {
			continue
		}
		detail := "no flow variation links this test"
		if inferred[t.TestKey] {
			detail = "only inferred test links, which count once reviewed"
		}
		gaps = append(gaps, Gap{Code: GapUnlinkedTest, TestKey: t.TestKey, Project: t.Project, Detail: detail + "; file " + t.Path})
	}
	slices.SortFunc(gaps, func(a, b Gap) int {
		if c := strings.Compare(a.TestKey, b.TestKey); c != 0 {
			return c
		}
		return strings.Compare(a.Project, b.Project)
	})
	total := len(gaps)
	if total > MaxUnlinkedTests {
		gaps = gaps[:MaxUnlinkedTests]
	}
	return gaps, total
}
