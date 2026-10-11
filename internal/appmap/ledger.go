package appmap

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"sort"

	"github.com/Beamfall/corvint/internal/jstestprovider"
)

const EdgeLedgerSchema = "application-edge-ledger/0"
const EdgeDeclarationsSchema = "application-edge-declarations/0"
const EdgeProven = "PROVEN"
const EdgeUnproven = "UNPROVEN"

// EdgeDeclarations are caller-owned bindings, never execution evidence (ANL-V0-004).
type EdgeDeclarations struct {
	Schema string            `json:"schema"`
	Tests  []EdgeDeclaration `json:"tests"`
}
type EdgeDeclaration struct {
	TestKey string   `json:"test_key"`
	Proves  []string `json:"proves"`
	Assumes []string `json:"assumes"`
}

func LoadEdgeDeclarations(name string) (*EdgeDeclarations, error) {
	raw, err := readReceipt(name)
	if err != nil {
		return nil, err
	}
	var d EdgeDeclarations
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	if dec.Decode(&d) != nil || dec.Decode(new(any)) != io.EOF || d.Schema != EdgeDeclarationsSchema || len(d.Tests) > 64 {
		return nil, verifyError("appmap-invalid-query", "invalid edge declarations")
	}
	for _, t := range d.Tests {
		if !validTestKey(t.TestKey) || len(t.Proves) > 64 || len(t.Assumes) > 64 {
			return nil, verifyError("appmap-invalid-query", "invalid edge declaration bounds or test key")
		}
	}
	return &d, nil
}

type EdgeCrossing struct {
	Flow         string           `json:"flow"`
	FromStep     string           `json:"from_step"`
	Step         string           `json:"step"`
	Steps        int              `json:"steps"`
	Verification StepVerification `json:"verification"`
}
type LedgerEdge struct {
	ID                string        `json:"id"`
	From              string        `json:"from"`
	To                string        `json:"to"`
	Trigger           string        `json:"trigger"`
	Selector          string        `json:"selector,omitempty"`
	Status            string        `json:"status"`
	Evidence          *EdgeCrossing `json:"evidence,omitempty"`
	RedundantCrossers []string      `json:"redundant-crossers"`
	RedundantCount    int           `json:"redundant_count"`
}
type EdgeLedger struct {
	Schema       string         `json:"schema"`
	App          string         `json:"app"`
	Revision     string         `json:"revision"`
	MapDigest    string         `json:"map_digest"`
	Authority    string         `json:"authority"`
	Edges        []LedgerEdge   `json:"edges"`
	Total        int            `json:"total"`
	Proven       int            `json:"proven"`
	UnknownPairs int            `json:"unknown_pairs"`
	Satisfied    bool           `json:"satisfied"`
	Unsatisfied  []string       `json:"unsatisfied"`
	Omitted      map[string]int `json:"omitted"`
	Digest       string         `json:"digest"`
}
type edgeOccurrence struct {
	flow     Flow
	from, to Step
}

func ledgerIdentity(from, to Step) LedgerEdge {
	trigger := to.Action
	switch trigger {
	case "goto", "click", "redirect", "form submit":
	default:
		trigger = StatusUnknown
	}
	selector := ""
	if to.Selector != nil {
		selector = to.Selector.ID
	}
	b, _ := json.Marshal([]string{from.Screen, to.Screen, trigger, selector})
	return LedgerEdge{ID: "edge:" + digest(b), From: from.Screen, To: to.Screen, Trigger: trigger, Selector: selector, Status: EdgeUnproven, RedundantCrossers: []string{}}
}

// ProjectEdges returns a capped projection and the full-ledger assumption verdict. It does not
// mutate the map or verification input. All run qualification is delegated to RVN (ANL-V0-002).
func ProjectEdges(ctx context.Context, m *Map, v *Verification, d *EdgeDeclarations, required []string, o Options) ([]byte, bool, error) {
	cap, err := o.budget(DefaultFlowBudget)
	if err != nil {
		return nil, false, err
	}
	if len(required) > 64 {
		return nil, false, verifyError("appmap-invalid-query", "at most 64 --require")
	}
	if v == nil {
		v = &Verification{binds: map[string][]string{}}
	}
	e := newVerifier(ctx, m, v, o)
	ledger := EdgeLedger{Schema: EdgeLedgerSchema, App: m.App, Revision: e.head, MapDigest: m.Digest, Authority: AuthorityLearned, Edges: []LedgerEdge{}, Unsatisfied: []string{}, Omitted: map[string]int{"edges": 0, "redundant-crossers": 0, "unsatisfied": 0}}
	occurrences := map[string][]edgeOccurrence{}
	rows := map[string]LedgerEdge{}
	pairs := 0
	for _, f := range m.Flows {
		for i := 1; i < len(f.Steps); i++ {
			pairs++
			if pairs > 4096 {
				return nil, false, verifyError("appmap-invalid-query", "at most 4096 adjacent pairs")
			}
			a, b := f.Steps[i-1], f.Steps[i]
			if a.Status != StatusResolved || b.Status != StatusResolved || m.screen(a.Screen) == nil || m.screen(b.Screen) == nil {
				ledger.UnknownPairs++
				continue
			}
			row := ledgerIdentity(a, b)
			rows[row.ID] = row
			occurrences[row.ID] = append(occurrences[row.ID], edgeOccurrence{f, a, b})
		}
	}
	proves := map[string][]string{}
	needs := map[string]bool{}
	for _, id := range required {
		needs[id] = true
	}
	if d != nil {
		if d.Schema != EdgeDeclarationsSchema || len(d.Tests) > 64 {
			return nil, false, verifyError("appmap-invalid-query", "invalid edge declarations")
		}
		for _, t := range d.Tests {
			if !validTestKey(t.TestKey) || len(t.Proves) > 64 || len(t.Assumes) > 64 {
				return nil, false, verifyError("appmap-invalid-query", "invalid edge declaration")
			}
			for _, id := range t.Proves {
				if _, ok := rows[id]; !ok {
					return nil, false, verifyError("appmap-invalid-query", "proves names an absent edge")
				}
				proves[id] = append(proves[id], t.TestKey)
			}
			for _, id := range t.Assumes {
				needs[id] = true
			}
		}
	}
	for id := range needs {
		if len(id) != 69 || id[:5] != "edge:" {
			return nil, false, verifyError("appmap-invalid-query", "edge IDs must be edge:SHA256")
		}
		for _, c := range id[5:] {
			if !(c >= '0' && c <= '9' || c >= 'a' && c <= 'f') {
				return nil, false, verifyError("appmap-invalid-query", "invalid edge ID")
			}
		}
	}
	ids := make([]string, 0, len(rows))
	for id := range rows {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	for _, id := range ids {
		row := rows[id]
		var all []EdgeCrossing
		blocked := false
		for _, occ := range occurrences[id] {
			keysA := map[string]bool{}
			keys := map[string]bool{}
			for _, k := range append(append([]string{}, occ.from.Tests...), v.binds[occ.from.ID]...) {
				keysA[k] = true
			}
			for _, k := range append(append([]string{}, occ.to.Tests...), v.binds[occ.to.ID]...) {
				if keysA[k] {
					keys[k] = true
				}
			}
			for _, k := range proves[id] {
				keys[k] = true
			}
			if len(keys) > 256 {
				return nil, false, verifyError("appmap-invalid-query", "at most 256 test keys per edge occurrence")
			}
			ordered := make([]string, 0, len(keys))
			for k := range keys {
				ordered = append(ordered, k)
			}
			sort.Strings(ordered)
			for _, key := range ordered {
				if keyUncertain(e, key, append(e.m.stepAnchors(occ.from), e.m.stepAnchors(occ.to)...)) {
					blocked = true
				}
				// Isolate one test key while sharing the RVN freshness cache and receipt admission.
				single := &Verification{receipts: v.receipts, binds: map[string][]string{}}
				ve := *e
				ve.v = single
				a, b := occ.from, occ.to
				a.Tests = []string{key}
				b.Tests = []string{key}
				av, _ := boundedVerification(*ve.verify(a))
				bv, _ := boundedVerification(*ve.verify(b))
				sv := bv
				switch {
				case av.Status == Unverified || bv.Status == Unverified:
					sv = StepVerification{Status: Unverified, Reason: "endpoint-unverified", Authority: AuthorityLearned}
					for _, endpoint := range []StepVerification{av, bv} {
						// A suite need not supply every declared test's receipt. Once an
						// outcome was evaluated, however, uncertainty blocks edge proof.
						if endpoint.Status == Unverified && endpoint.Reason != "no-binding" && endpoint.Reason != "no-receipt-outcome" {
							blocked = true
						}
					}
				case av.Status == UnverifiedAtHead:
					sv = av
				case bv.Status == UnverifiedAtHead:
					sv = bv
				case av.Status == Contradicted:
					sv = av
				case bv.Status == Contradicted:
					sv = bv
				case av.Revision != bv.Revision || av.Receipt == nil || bv.Receipt == nil || *av.Receipt != *bv.Receipt:
					sv = StepVerification{Status: Unverified, Reason: "crossing-evidence-mismatch", Authority: AuthorityLearned}
					blocked = true
				}
				if sv.Status != Unverified {
					anchors := []Anchor{occ.flow.Anchor}
					at := e.anchorsAt(sv.Revision, anchors)
					head := e.anchorsAt(e.head, anchors)
					switch {
					case head.state == Stale && at.state == Fresh:
						sv.Status = UnverifiedAtHead
					case head.state != Fresh || at.state != Fresh:
						sv = StepVerification{Status: Unverified, Reason: "flow-anchor-unverified", Authority: AuthorityLearned}
						blocked = true
					}
				}
				all = append(all, EdgeCrossing{Flow: occ.flow.ID, FromStep: a.ID, Step: b.ID, Steps: len(occ.flow.Steps), Verification: sv})
			}
		}
		sort.Slice(all, func(i, j int) bool {
			a, b := all[i], all[j]
			if a.Steps != b.Steps {
				return a.Steps < b.Steps
			}
			if ka, kb := crossingTestKey(a), crossingTestKey(b); ka != kb {
				return ka < kb
			}
			ab, _ := json.Marshal(a)
			bb, _ := json.Marshal(b)
			return string(ab) < string(bb)
		})
		// Conservative aggregation: stale/unknown evidence cannot be laundered by another pass.
		for _, status := range []string{UnverifiedAtHead, Contradicted, Verified} {
			for i := range all {
				if all[i].Verification.Status == status {
					if status == Verified && blocked {
						continue
					}
					row.Evidence = &all[i]
					row.Status = status
					if status == Verified {
						row.Status = EdgeProven
					}
					break
				}
			}
			if row.Evidence != nil {
				break
			}
		}
		if row.Status == EdgeProven {
			ledger.Proven++
			seen := map[string]bool{row.Evidence.Verification.Receipt.TestKey: true}
			for _, cross := range all {
				sv := cross.Verification
				if sv.Status == Verified && sv.Receipt != nil && !seen[sv.Receipt.TestKey] {
					seen[sv.Receipt.TestKey] = true
					row.RedundantCrossers = append(row.RedundantCrossers, sv.Receipt.TestKey)
				}
			}
			sort.Strings(row.RedundantCrossers)
			row.RedundantCount = len(row.RedundantCrossers)
		}
		rows[id] = row
		ledger.Edges = append(ledger.Edges, row)
	}
	ledger.Total = len(ledger.Edges)
	for id := range needs {
		if rows[id].Status != EdgeProven {
			ledger.Unsatisfied = append(ledger.Unsatisfied, id)
		}
	}
	sort.Strings(ledger.Unsatisfied)
	ledger.Satisfied = len(ledger.Unsatisfied) == 0
	complete, _ := json.Marshal(ledger)
	ledger.Digest = digest(complete)
	for {
		raw, err := json.Marshal(ledger)
		if err != nil {
			return nil, false, err
		}
		raw = append(raw, '\n')
		if len(raw) <= cap {
			return raw, ledger.Satisfied, nil
		}
		if n := len(ledger.Edges); n > 0 {
			row := &ledger.Edges[n-1]
			if len(row.RedundantCrossers) > 0 {
				row.RedundantCrossers = row.RedundantCrossers[:len(row.RedundantCrossers)-1]
				ledger.Omitted["redundant-crossers"]++
				continue
			}
			ledger.Edges = ledger.Edges[:n-1]
			ledger.Omitted["edges"]++
			continue
		}
		if n := len(ledger.Unsatisfied); n > 0 {
			ledger.Unsatisfied = ledger.Unsatisfied[:n-1]
			ledger.Omitted["unsatisfied"]++
			continue
		}
		return nil, false, budgetError("edge ledger header exceeds --budget")
	}
}

func crossingTestKey(c EdgeCrossing) string {
	if c.Verification.Receipt != nil {
		return c.Verification.Receipt.TestKey
	}
	return ""
}

// keyUncertain is true when any receipt outcome for key is not a conclusive pass or failure at a
// resolvable application revision: a pass elsewhere does not launder it (ANL-V0-003).
func keyUncertain(e *verifier, key string, anchors []Anchor) bool {
	for _, in := range e.v.receipts {
		for _, t := range in.r.Tests {
			if t.ID != key {
				continue
			}
			if !jstestprovider.QualifiedReceiptBindingReady(in.r, t) {
				return true
			}
			switch t.State {
			case jstestprovider.StatePassed, jstestprovider.StateFailed, jstestprovider.StateTimedOut:
			default:
				return true
			}
			if rev, _ := jstestprovider.QualifiedApplicationRevision(in.r, t); !commitOID.MatchString(rev) || e.anchorsAt(rev, anchors).evaluated != rev || e.anchorsAt(rev, anchors).state != Fresh {
				return true
			}
		}
	}
	return false
}
