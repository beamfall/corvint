package extevidence

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"slices"
	"sort"
	"strings"

	"github.com/Beamfall/corvint/internal/gokernel"
)

// ObligationsSchema names the Change Frontier sidecar this file composes (EFO-V0-003).
const ObligationsSchema = "external-frontier-obligations/0"

// Sidecar states (EFO-V0-006).
const (
	ObligationsComplete = "complete"
	ObligationsPartial  = "partial"
	ObligationsUnknown  = "unknown"
)

// Association kinds in their output order (EFO-V0-004).
var associationKinds = []string{"entity", "obligation", "test", "unknown"}

// obligationsUntrusted names the provider-authored free text in the sidecar (EFO-V0-009).
var obligationsUntrusted = []string{
	"hunks[].associations[].summary", "hunks[].associations[].relation.rule", "hunks[].associations[].relation.reference",
}

const obligationsNote = "reference-only: each association proves identity, integrity, and freshness of an external reference; none states that an external record justifies the hunk; the frontier computes its own state from the CEM and never reads this sidecar"

// cemHunks is the structural slice of a cem/0.2 document the sidecar cites.
// It is not a verification: the frontier verifies the CEM (EFO-V0-002).
type cemHunks struct {
	Spec         string `json:"spec"`
	BaseRevision string `json:"baseRevision"`
	ExcludedPath string `json:"excludedPath"`
	PatchSha256  string `json:"patchSha256"`
	Hunks        []struct {
		ID          string `json:"id"`
		Path        string `json:"path"`
		Disposition string `json:"disposition"`
	} `json:"hunks"`
}

// impactReceipt is the structural slice of an impact receipt the sidecar joins.
type impactReceipt struct {
	Tool    string `json:"tool"`
	Context struct {
		External *externalSection `json:"external"`
	} `json:"context"`
}

type externalSection struct {
	Providers    []map[string]any `json:"providers"`
	Results      []map[string]any `json:"results"`
	Downstream   []map[string]any `json:"downstream"`
	Verification []map[string]any `json:"verification"`
}

// association is one hunk row member before it is rendered.
type association struct {
	kind, provider, entity, entityKind, summary, path, verification, freshness, reason, code string
	relation                                                                                 map[string]any
}

// Obligations composes the external-frontier-obligations/0 sidecar from raw
// CEM bytes and a raw `corvint impact` receipt (EFO-V0-002..EFO-V0-006). It
// reads no repository and never verifies the CEM: it binds both inputs by
// digest so a frontier document over the same CEM can be joined to it.
func Obligations(cem, impact []byte, limit int) (map[string]any, error) {
	var document cemHunks
	if err := json.Unmarshal(cem, &document); err != nil || document.Spec != "cem/0.2" {
		return nil, &gokernel.Error{Code: "invalid-obligations-input", Message: "cem input is not a cem/0.2 document"}
	}
	var receipt impactReceipt
	if err := json.Unmarshal(impact, &receipt); err != nil || receipt.Tool != "impact" {
		return nil, &gokernel.Error{Code: "invalid-obligations-input", Message: "impact input is not a corvint impact receipt"}
	}
	external := receipt.Context.External
	providers, freshness, missing := providerSummary(external)
	state, reason := obligationsState(external, missing)
	hunks := make([]any, 0, len(document.Hunks))
	for _, hunk := range document.Hunks {
		rows := associate(external, hunk.Path, freshness, missing)
		kept, omitted := boundAssociations(rows, limit)
		hunks = append(hunks, map[string]any{"hunk": hunk.ID, "path": hunk.Path, "disposition": hunk.Disposition, "associations": kept, "omitted": omitted})
	}
	out := map[string]any{
		"schema":       ObligationsSchema,
		"authority":    Authority,
		"state":        state,
		"state_reason": reason,
		"binding": map[string]any{
			"cem_sha256": digest(cem), "patch_sha256": document.PatchSha256, "base_revision": document.BaseRevision,
			"excluded_path": document.ExcludedPath, "impact_sha256": digest(impact), "impact_tool": receipt.Tool,
		},
		"providers":             providers,
		"hunks":                 hunks,
		"note":                  obligationsNote,
		"untrusted_text_fields": anyStrings(obligationsUntrusted),
	}
	encoded, err := gokernel.CanonicalJSON(out)
	if err != nil {
		return nil, err
	}
	out["id"] = "obligations:sha256:" + digest(encoded)
	return out, nil
}

func digest(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

func anyStrings(values []string) []any {
	out := make([]any, 0, len(values))
	for _, value := range values {
		out = append(out, value)
	}
	return out
}

// providerSummary echoes every provider row's identity, state, and freshness
// and names the providers that did not load (EFO-V0-005, EFO-V0-006).
func providerSummary(external *externalSection) ([]any, map[string]string, []association) {
	rows, freshness, missing := []any{}, map[string]string{}, []association{}
	if external == nil {
		return rows, freshness, missing
	}
	for _, row := range external.Providers {
		id, state, fresh := text(row["id"]), text(row["state"]), providerFreshness(row)
		rows = append(rows, map[string]any{"id": id, "revision": text(row["revision"]), "state": state, "freshness": fresh})
		freshness[id] = fresh
		if state != StateLoaded {
			missing = append(missing, association{kind: "unknown", provider: id, code: "provider-" + state, reason: text(row["reason"])})
		}
	}
	return rows, freshness, missing
}

// providerFreshness is the V0 record freshness or the V1 root repository's.
func providerFreshness(row map[string]any) string {
	if fresh := text(row["freshness"]); fresh != "" {
		return fresh
	}
	root := text(row["root_repository"])
	repositories, _ := row["repositories"].([]any)
	for _, entry := range repositories {
		repository, _ := entry.(map[string]any)
		if text(repository["id"]) == root {
			return text(repository["freshness"])
		}
	}
	return FreshnessRevisionUnavailable
}

func obligationsState(external *externalSection, missing []association) (string, string) {
	if external == nil {
		return ObligationsUnknown, "no-external-section"
	}
	if len(missing) > 0 {
		return ObligationsPartial, "provider-not-loaded"
	}
	return ObligationsComplete, "every-provider-loaded"
}

// associate joins one hunk path to the external section: results on the path
// are entities, downstream rows one hop from those entities are obligations,
// verification rows on those entities are tests, and a hunk with no entity
// or a provider that did not load is an explicit unknown (EFO-V0-004, EFO-V0-006).
func associate(external *externalSection, path string, freshness map[string]string, missing []association) []association {
	if external == nil {
		return []association{{kind: "unknown", code: "no-external-section", reason: "the impact receipt carries no context.external member"}}
	}
	rows := []association{}
	entities := map[string]struct{}{}
	for _, row := range external.Results {
		if text(row["path"]) != path {
			continue
		}
		rows = append(rows, fromRow("entity", row, freshness))
		entities[text(row["entity"])] = struct{}{}
	}
	for _, row := range external.Downstream {
		if touches(row, entities) {
			rows = append(rows, fromRow("obligation", row, freshness))
		}
	}
	for _, row := range external.Verification {
		if _, listed := entities[text(row["entity"])]; listed {
			rows = append(rows, fromRow("test", row, freshness))
		}
	}
	if len(entities) == 0 {
		rows = append(rows, association{kind: "unknown", code: "no-external-evidence", reason: "no loaded provider relates this hunk's path to an entity"})
	}
	return append(rows, missing...)
}

func fromRow(kind string, row map[string]any, freshness map[string]string) association {
	relation, _ := row["relation"].(map[string]any)
	return association{
		kind: kind, provider: text(row["provider"]), entity: text(row["entity"]), entityKind: text(row["kind"]),
		summary: text(row["summary"]), path: text(row["path"]), relation: relation,
		verification: text(row["verification"]), freshness: freshness[text(row["provider"])], reason: text(row["reason"]),
	}
}

// touches reports whether a downstream row's relation has a hunk entity on
// either side; V0 endpoints are strings and V1 endpoints are maps.
func touches(row map[string]any, entities map[string]struct{}) bool {
	relation, _ := row["relation"].(map[string]any)
	for _, side := range []any{relation["from"], relation["to"]} {
		if _, listed := entities[endpointText(side)]; listed {
			return true
		}
	}
	return false
}

func endpointText(side any) string {
	if structured, ok := side.(map[string]any); ok {
		return text(structured["provider"]) + ":" + text(structured["entity"])
	}
	return text(side)
}

func text(value any) string {
	out, _ := value.(string)
	return out
}

// boundAssociations sorts rows by kind order then a total key, keeps at most
// limit, and counts the rest (EFO-V0-008).
func boundAssociations(rows []association, limit int) ([]any, int) {
	sort.SliceStable(rows, func(i, j int) bool { return associationKey(rows[i]) < associationKey(rows[j]) })
	kept := min(len(rows), max(limit, 0))
	out := make([]any, 0, kept)
	for _, row := range rows[:kept] {
		out = append(out, row.toMap())
	}
	return out, len(rows) - kept
}

func associationKey(row association) string {
	order := fmt.Sprintf("%d", slices.Index(associationKinds, row.kind))
	return strings.Join([]string{order, row.provider, row.entity, row.path, row.code, text(row.relation["type"]), text(row.relation["reference"]), text(row.relation["from"]), text(row.relation["to"])}, "\x00")
}

// toMap renders one association. A non-unknown row always carries the
// provider evidence reference, verification, freshness, and authority, and
// no row carries a closure, score, or justification member (EFO-V0-005).
func (row association) toMap() map[string]any {
	if row.kind == "unknown" {
		out := map[string]any{"kind": row.kind, "code": row.code, "reason": row.reason}
		if row.provider != "" {
			out["provider"] = row.provider
		}
		return out
	}
	out := map[string]any{
		"kind": row.kind, "authority": Authority, "provider": row.provider, "entity": row.entity, "entity_kind": row.entityKind,
		"summary": row.summary, "relation": row.relation, "verification": row.verification, "freshness": row.freshness, "reason": row.reason,
	}
	if row.path != "" {
		out["path"] = row.path
	}
	return out
}
