package appflows

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"

	jsonv2 "encoding/json/v2"

	"github.com/Beamfall/corvint/internal/doccorpus"
	"github.com/Beamfall/corvint/internal/extevidence"
)

// Export identities (AFU-V1-003).
const (
	InventorySchema  = "application-flow-inventory/1"
	FlowsInputID     = "application-flows"
	ProviderID       = "corvint-application-flows"
	ProviderRepoID   = "repository"
	maxEntitySummary = 512
)

// InventoryFlow is one DCP-V1-027 flow record compiled from an intent.
type InventoryFlow struct {
	ID               string                    `json:"id"`
	Derivation       string                    `json:"derivation"`
	Evidence         doccorpus.Anchor          `json:"evidence"`
	RequiredPages    []string                  `json:"required_pages"`
	NegativeControls []string                  `json:"negative_controls"`
	OrderedEvents    []doccorpus.BehaviorEvent `json:"ordered_events"`
	MissingReview    *doccorpus.Anchor         `json:"missing_e2e_review,omitempty"`
}

// Inventory is the compiled flow and variation document the adapter request maps (AFU-V1-003).
type Inventory struct {
	Schema     string                               `json:"schema"`
	Flows      []InventoryFlow                      `json:"flows"`
	Variations []doccorpus.BehaviorAdapterVariation `json:"variations"`
}

// CompileInventory compiles the intent set to canonical inventory bytes. It carries no proposed marker.
func CompileInventory(set IntentSet) ([]byte, error) {
	inv := Inventory{Schema: InventorySchema, Flows: []InventoryFlow{}, Variations: []doccorpus.BehaviorAdapterVariation{}}
	for _, intent := range set.Flows {
		if intent.Adapter == nil {
			return nil, fmt.Errorf("flow %s: export requires an adapter evidence anchor", intent.FlowID)
		}
		inv.Flows = append(inv.Flows, inventoryFlow(intent))
		for _, v := range intent.Variations {
			inv.Variations = append(inv.Variations, adapterVariation(intent, v))
		}
	}
	slices.SortFunc(inv.Variations, func(a, b doccorpus.BehaviorAdapterVariation) int { return strings.Compare(a.ID, b.ID) })
	return doccorpus.Encode(inv)
}

func inventoryFlow(intent FlowIntent) InventoryFlow {
	a := intent.Adapter
	return InventoryFlow{
		ID: intent.FlowID, Derivation: a.Derivation, Evidence: a.Evidence,
		RequiredPages: nonNil(a.RequiredPages), NegativeControls: nonNil(a.NegativeControls),
		OrderedEvents: nonNil(a.OrderedEvents), MissingReview: a.MissingReview,
	}
}

func adapterVariation(intent FlowIntent, v FlowVariation) doccorpus.BehaviorAdapterVariation {
	actions := map[string]string{}
	for _, s := range intent.Steps {
		actions[s.StepID] = s.Action
	}
	outcomes := map[string]FlowOutcome{}
	for _, o := range intent.Outcomes {
		outcomes[o.OutcomeID] = o
	}
	out := doccorpus.BehaviorAdapterVariation{
		ID: v.VariationID, Flow: intent.FlowID,
		Preconditions:   sortedSet(append(slices.Clone(intent.Preconditions), v.Preconditions...)),
		Actions:         []string{},
		ObservableFacts: sortedSet(v.ObservableFacts),
		Projects:        sortedSet(v.Projects),
		Tests:           variationTests(intent, v.VariationID),
	}
	for _, id := range v.Steps {
		out.Actions = append(out.Actions, actions[id])
	}
	for _, id := range sortedSet(v.Outcomes) {
		o := outcomes[id]
		out.ExpectedOutcomes = append(out.ExpectedOutcomes, doccorpus.BehaviorAdapterOutcome{ID: id, Behavior: o.Behavior, Matcher: o.Matcher, Locator: o.Locator, Value: o.Value})
	}
	return out
}

func variationTests(intent FlowIntent, variation string) []string {
	tests := []string{}
	for _, l := range intent.Links {
		if l.From == variation && l.Target.Type == "test" {
			tests = append(tests, l.Target.TestKey)
		}
	}
	return sortedSet(tests)
}

// ExportRequest fills the envelope's application-flows input with the inventory, maps the flows and
// variations kinds to it, and runs the result through the existing DCP-V1 adapter (AFU-V1-003).
func ExportRequest(envelope, inventory []byte) ([]byte, error) {
	var req doccorpus.BehaviorAdapterRequest
	if err := jsonv2.Unmarshal(envelope, &req, jsonv2.RejectUnknownMembers(true)); err != nil {
		return nil, errors.New("envelope is not a closed behavior-adapter request")
	}
	digest := doccorpus.Digest(inventory)
	i := slices.IndexFunc(req.Inputs, func(in doccorpus.BehaviorAdapterInput) bool { return in.ID == FlowsInputID })
	if i < 0 || req.Inputs[i].Anchor.SHA256 != digest || req.Inputs[i].Anchor.SpanSHA256 != digest {
		return nil, fmt.Errorf("envelope input %q must anchor the exact inventory bytes (sha256 %s); commit the output of --emit inventory and anchor it", FlowsInputID, digest)
	}
	req.Inputs[i].Document = string(inventory)
	req.Mappings = slices.DeleteFunc(req.Mappings, func(m doccorpus.BehaviorAdapterMapping) bool {
		return m.Kind == "flows" || m.Kind == "variations"
	})
	req.Mappings = append(req.Mappings, inventoryMappings()...)
	slices.SortFunc(req.Mappings, func(a, b doccorpus.BehaviorAdapterMapping) int { return strings.Compare(a.Kind, b.Kind) })
	slices.SortFunc(req.Inputs, func(a, b doccorpus.BehaviorAdapterInput) int { return strings.Compare(a.ID, b.ID) })
	out, err := doccorpus.Encode(req)
	if err != nil {
		return nil, err
	}
	if _, err = doccorpus.BuildBehaviorAdapter(out, nil); err != nil {
		return nil, fmt.Errorf("behavior adapter refused the exported request: %v", err)
	}
	return out, nil
}

func inventoryMappings() []doccorpus.BehaviorAdapterMapping {
	flowFields := map[string]string{}
	for _, name := range []string{"id", "derivation", "evidence", "required_pages", "negative_controls", "ordered_events", "missing_e2e_review"} {
		flowFields[name] = "/" + name
	}
	variationFields := map[string]string{"id": "/variation_id"}
	for _, name := range []string{"flow", "preconditions", "actions", "observable_facts", "expected_outcomes", "projects", "tests"} {
		variationFields[name] = "/" + name
	}
	return []doccorpus.BehaviorAdapterMapping{
		{Kind: "flows", Input: FlowsInputID, Records: "/flows", Fields: flowFields},
		{Kind: "variations", Input: FlowsInputID, Records: "/variations", Fields: variationFields},
	}
}

// ExportProvider compiles one EEP-V1 provider record at revision (AFU-V1-003, AFU-V1-007).
func ExportProvider(ctx context.Context, root string, set IntentSet, revision string) ([]byte, error) {
	rev, err := ResolveRevision(ctx, root, revision)
	if err != nil {
		return nil, err
	}
	links, err := EvaluateLinks(ctx, root, set, rev)
	if err != nil {
		return nil, err
	}
	record := extevidence.Record1{
		Schema:       extevidence.Schema1,
		Provider:     extevidence.Identity{ID: ProviderID, Revision: rev},
		Repositories: []extevidence.Repository1{{ID: ProviderRepoID, Revision: rev, Role: "application"}},
		Entities:     flowEntities(set),
		Relations:    []extevidence.Relation1{},
	}
	owners := variationOwners(set)
	for _, l := range links {
		to, entity := relationTarget(l)
		if entity != nil && !slices.ContainsFunc(record.Entities, func(e extevidence.Entity) bool { return e.ID == entity.ID }) {
			record.Entities = append(record.Entities, *entity)
		}
		record.Relations = append(record.Relations, extevidence.Relation1{
			From: extevidence.Endpoint1{Provider: ProviderID, Entity: owners(l.Flow, l.From)}, To: to,
			Type: "flow-" + l.Target.Type, Evidence: storedEvidence(l), Rule: relationRule(l),
			Reference: set.IntentPath(l.Flow),
		})
	}
	slices.SortFunc(record.Entities, func(a, b extevidence.Entity) int { return strings.Compare(a.ID, b.ID) })
	out, err := doccorpus.Encode(record)
	if err != nil {
		return nil, err
	}
	if _, err = extevidence.Decode1(out); err != nil {
		return nil, fmt.Errorf("provider record refused: %v", err)
	}
	return out, nil
}

func flowEntities(set IntentSet) []extevidence.Entity {
	entities := []extevidence.Entity{}
	for _, f := range set.Flows {
		entities = append(entities, extevidence.Entity{ID: "flow/" + f.FlowID, Kind: "flow", Summary: fmt.Sprintf("%s flow %s revision %d", f.Kind, f.FlowID, f.Revision)})
		for _, v := range f.Variations {
			entities = append(entities, extevidence.Entity{ID: "variation/" + v.VariationID, Kind: "variation", Summary: "variation of flow " + f.FlowID})
		}
	}
	return entities
}

// variationOwners maps a link's from member to its variation entity, or to its flow entity.
func variationOwners(set IntentSet) func(flow, from string) string {
	variations := map[string]bool{}
	for _, f := range set.Flows {
		for _, v := range f.Variations {
			variations[f.FlowID+"\x00"+v.VariationID] = true
		}
	}
	return func(flow, from string) string {
		if variations[flow+"\x00"+from] {
			return "variation/" + from
		}
		return "flow/" + flow
	}
}

func relationTarget(l EvaluatedLink) (extevidence.Endpoint1, *extevidence.Entity) {
	if l.Target.Path != "" {
		return extevidence.Endpoint1{Repository: ProviderRepoID, Path: l.Target.Path, Blob: l.Blob}, nil
	}
	entity := extevidence.Entity{ID: "evidence/" + l.Target.Digest, Kind: "evidence", Summary: "evidence digest " + l.Target.Digest}
	if l.Target.Type == "test" {
		entity = extevidence.Entity{ID: "test/" + doccorpus.Digest([]byte(l.Target.TestKey)), Kind: "test", Summary: testSummary(l.Target.TestKey)}
	}
	return extevidence.Endpoint1{Provider: ProviderID, Entity: entity.ID}, &entity
}

func testSummary(key string) string {
	if len(key) <= maxEntitySummary {
		return key
	}
	return "test key longer than 512 bytes; identified by its sha256"
}

func storedEvidence(l EvaluatedLink) string {
	if l.ReviewState == ReviewInferred {
		return extevidence.EvidenceInferred
	}
	return extevidence.EvidenceDeclared
}

func relationRule(l EvaluatedLink) string {
	rule := fmt.Sprintf("%s link from %s; review_state=%s", storedEvidence(l), l.From, l.ReviewState)
	if l.ReviewAttestation != "" {
		rule += "; review_attestation=" + l.ReviewAttestation + "; review identity is not verified"
	}
	return rule
}

func sortedSet(values []string) []string {
	out := slices.Clone(values)
	slices.Sort(out)
	return nonNil(slices.Compact(out))
}

func nonNil[T any](values []T) []T {
	if values == nil {
		return []T{}
	}
	return values
}
