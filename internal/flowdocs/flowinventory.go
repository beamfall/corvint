package flowdocs

import (
	json "encoding/json/v2"
	"fmt"
	"path/filepath"
	"strings"
	"unicode/utf8"

	"github.com/Beamfall/corvint/internal/doccorpus"
)

// FlowInventorySchema names the optional caller-declared flow inventory that
// `docs flows generate --flows FILE` accepts (AFU-V1-101..106).
const FlowInventorySchema = "corvint-flow-inventory/1"

// MaxInventoryBytes bounds the inventory file; MaxInventoryMembers bounds the
// members across all declared flows; MaxInventoryDeclarations bounds the
// in-scope declarations generation resolves members against.
const (
	MaxInventoryBytes        = 4 << 20
	MaxInventoryMembers      = 8192
	MaxInventoryDeclarations = 16384
)

// FlowInventory is a project's own reviewed flow list. Membership is
// caller-declared: it groups source observations and never establishes
// accepted intent or runtime behavior.
type FlowInventory struct {
	Schema string          `json:"schema"`
	Flows  []InventoryFlow `json:"flows"`
}
type InventoryFlow struct {
	ID          string            `json:"id"`
	Title       string            `json:"title"`
	Members     []InventoryMember `json:"members"`
	EntryPoints []InventoryMember `json:"entry_points,omitempty"`
}

// InventoryMember names one repository path and, optionally, one declaration
// in it by the name generation reports (for example `Billing::Charge#call`).
// A member without a symbol takes every unambiguous declaration in the path.
type InventoryMember struct {
	Path   string `json:"path"`
	Symbol string `json:"symbol,omitempty"`
}
type UnresolvedMember struct {
	Path   string `json:"path"`
	Symbol string `json:"symbol,omitempty"`
	Reason string `json:"reason"`
}

// InventoryReport is the generation manifest's record of the declared
// inventory and how it resolved at the source revision. Declared is retained
// so Open and check rederive the same identities.
type InventoryReport struct {
	Declared   FlowInventory         `json:"declared"`
	Flows      []InventoryFlowReport `json:"flows"`
	Unassigned []InventoryMember     `json:"unassigned"`
}
type InventoryFlowReport struct {
	ID         string             `json:"id"`
	Flow       string             `json:"flow"`
	Generated  bool               `json:"generated"`
	Resolved   []InventoryMember  `json:"resolved"`
	Unresolved []UnresolvedMember `json:"unresolved"`
}

func inventoryID(s string) bool {
	if s == "" || len(s) > 128 {
		return false
	}
	for i, r := range s {
		if r >= 'a' && r <= 'z' || r >= '0' && r <= '9' || i > 0 && (r == '.' || r == '_' || r == '-') {
			continue
		}
		return false
	}
	return true
}
func inventoryText(s string, limit int) bool {
	if s == "" || len(s) > limit || !utf8.ValidString(s) {
		return false
	}
	for _, r := range s {
		if r < 32 || r == 127 {
			return false
		}
	}
	return true
}

// ParseFlowInventory strictly decodes and validates an inventory. Every
// malformed, duplicate, overlapping or over-bound input is refused rather
// than repaired (AFU-V1-101).
func ParseFlowInventory(raw []byte) (*FlowInventory, error) {
	if len(raw) > MaxInventoryBytes {
		return nil, fail("flow inventory exceeds 4 MiB bound")
	}
	var inv FlowInventory
	if err := json.Unmarshal(raw, &inv, json.RejectUnknownMembers(true)); err != nil {
		return nil, fail("invalid closed flow inventory")
	}
	if inv.Schema != FlowInventorySchema {
		return nil, fail("unsupported flow inventory schema")
	}
	if len(inv.Flows) == 0 || len(inv.Flows) > MaxFlows {
		return nil, fail("flow inventory must declare 1 to 256 flows")
	}
	ids := map[string]bool{}
	total := 0
	for _, f := range inv.Flows {
		if !inventoryID(f.ID) || ids[f.ID] {
			return nil, fail("flow inventory id must be unique lowercase [a-z0-9._-], at most 128 bytes")
		}
		ids[f.ID] = true
		if !inventoryText(f.Title, 200) {
			return nil, fail("flow inventory title must be 1 to 200 bytes of printable UTF-8")
		}
		total += len(f.Members)
		if len(f.Members) == 0 || total > MaxInventoryMembers {
			return nil, fail("flow inventory members must be nonempty and at most 8192 in total")
		}
		members, wholePath, paths := map[InventoryMember]bool{}, map[string]bool{}, map[string]int{}
		for _, m := range f.Members {
			if !validScope(m.Path) || m.Symbol != "" && !inventoryText(m.Symbol, 512) || members[m] {
				return nil, fail("flow inventory member must be a unique confined path with an optional printable symbol")
			}
			members[m] = true
			paths[m.Path]++
			if m.Symbol == "" {
				wholePath[m.Path] = true
			}
		}
		for p := range wholePath {
			if paths[p] > 1 {
				return nil, fail("flow inventory member path without symbol overlaps another member of the same flow")
			}
		}
		seen := map[InventoryMember]bool{}
		for _, e := range f.EntryPoints {
			if !members[e] || seen[e] {
				return nil, fail("flow inventory entry point must name a distinct member of its flow")
			}
			seen[e] = true
		}
	}
	return &inv, nil
}

// ReadInventoryFile reads a bounded regular inventory file.
func ReadInventoryFile(name string) ([]byte, error) {
	r, err := openDirectory(filepath.Dir(name))
	if err != nil {
		return nil, err
	}
	defer r.Close()
	return readRoot(r, filepath.Base(name), MaxInventoryBytes)
}

// scopedInventory parses an optional inventory and refuses a member outside
// the generation scope: generation never reads it, so it could never resolve.
func scopedInventory(raw []byte, scope string) (*FlowInventory, error) {
	if len(raw) == 0 {
		return nil, nil
	}
	inv, err := ParseFlowInventory(raw)
	if err != nil {
		return nil, err
	}
	for _, f := range inv.Flows {
		for _, member := range f.Members {
			if !inScope(member.Path, scope) {
				return nil, fail("flow inventory member outside --scope")
			}
		}
	}
	return inv, nil
}

// inventoryInput re-encodes the retained declared inventory so Open and check
// rederive the same identities; nil when generation had no inventory.
func (m Manifest) inventoryInput() ([]byte, error) {
	if m.Inventory == nil {
		return nil, nil
	}
	return Encode(m.Inventory.Declared)
}

// buildInventory turns each declared flow into one flow aggregating its
// resolved members (AFU-V1-102). Members that do not resolve are reported,
// never dropped (AFU-V1-103); declarations no declared flow claims are listed
// as unassigned instead of becoming flows (AFU-V1-104).
func buildInventory(m *Manifest, p *doccorpus.ProviderRecord, inv *FlowInventory, decls []memberDecl, ambiguous map[string]bool) (*InventoryReport, error) {
	report := &InventoryReport{Declared: *inv, Flows: []InventoryFlowReport{}, Unassigned: []InventoryMember{}}
	byPath := map[string][]int{}
	for i, md := range decls {
		byPath[md.path] = append(byPath[md.path], i)
	}
	assigned := make([]bool, len(decls))
	type pick struct {
		index int
		entry bool
	}
	for _, declared := range inv.Flows {
		row := InventoryFlowReport{ID: declared.ID, Flow: identity("flow", "inventory:"+declared.ID), Resolved: []InventoryMember{}, Unresolved: []UnresolvedMember{}}
		entry := map[InventoryMember]bool{}
		for _, e := range declared.EntryPoints {
			entry[e] = true
		}
		picks := []pick{}
		for _, member := range declared.Members {
			found := false
			for _, i := range byPath[member.Path] {
				if member.Symbol == "" || decls[i].d.name == member.Symbol {
					picks = append(picks, pick{i, entry[member]})
					found = true
				}
			}
			if found {
				continue
			}
			reason := "path has no unambiguous supported declaration at this revision"
			if member.Symbol != "" {
				reason = "symbol is not a supported declaration in this path at this revision"
				if ambiguous[member.Path+":"+member.Symbol] {
					reason = "symbol declaration is ambiguous in this path at this revision"
				}
			}
			row.Unresolved = append(row.Unresolved, UnresolvedMember{Path: member.Path, Symbol: member.Symbol, Reason: reason})
		}
		if len(row.Unresolved) > 0 {
			m.Gaps = append(m.Gaps, "Declared inventory members that do not resolve are listed in generation.json inventory.flows[].unresolved and in their flow README.")
		}
		if len(picks) == 0 {
			m.Gaps = append(m.Gaps, "A declared inventory flow with no resolved member is reported, not generated.")
			report.Flows = append(report.Flows, row)
			continue
		}
		first := decls[picks[0].index]
		b := &flowBuilder{m: m, p: p, f: Flow{ID: row.Flow, Name: declared.Title, Path: first.path, Paragraphs: []Paragraph{}}, calls: map[string]bool{}}
		b.summaries(map[string]string{
			"readme":              "Generated source documentation for a declared inventory flow. Priority: unassigned. Confidence: lexical candidate. Trust: generated; membership is caller-declared and accepted intent is not established.",
			"entry-points":        "Declared entry points are listed below. Entry dispatch and callers are unresolved.",
			"technical-deep-dive": "Declared members are listed below; generated call observations do not establish executed behavior.",
		}, first.a)
		for _, pk := range picks {
			md := decls[pk.index]
			assigned[pk.index] = true
			row.Resolved = append(row.Resolved, InventoryMember{Path: md.path, Symbol: md.d.name})
			key := md.path + ":" + md.d.name
			section, text := "technical-deep-dive", "Declared member: source declares "+md.d.name+"; membership is caller-declared, not runtime evidence."
			if pk.entry {
				section, text = "entry-points", "Declared entry point: source declares "+md.d.name+"; entry dispatch and callers are unresolved."
			}
			from := len(b.f.Paragraphs)
			b.add(section, "member:"+key, text, md.a)
			if err := b.member(key+":", md, from); err != nil {
				return nil, err
			}
		}
		b.finish(first.a)
		row.Generated = true
		report.Flows = append(report.Flows, row)
	}
	for i, md := range decls {
		if !assigned[i] {
			report.Unassigned = append(report.Unassigned, InventoryMember{Path: md.path, Symbol: md.d.name})
		}
	}
	if len(report.Unassigned) > 0 {
		m.Gaps = append(m.Gaps, "In-scope declarations that belong to no declared inventory flow are listed in generation.json inventory.unassigned, not generated as flows.")
	}
	return report, nil
}

// renderInventoryMembers lists a declared flow's resolved and unresolved
// members in its README; it writes nothing without an inventory.
func renderInventoryMembers(b *strings.Builder, m Manifest, f Flow) {
	if m.Inventory == nil {
		return
	}
	for _, row := range m.Inventory.Flows {
		if row.Flow != f.ID {
			continue
		}
		fmt.Fprintf(b, "\nDeclared inventory flow %s: %d resolved, %d unresolved members.\n", quote(row.ID), len(row.Resolved), len(row.Unresolved))
		for _, u := range row.Unresolved {
			fmt.Fprintf(b, "\n- Unresolved member %s %s: %s.\n", quote(u.Path), quote(u.Symbol), quote(u.Reason))
		}
	}
}
