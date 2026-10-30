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
	ObservedBy        []string      `json:"observed_by,omitempty"`
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
	Observations []string       `json:"observations,omitempty"`
	Unmatched    int            `json:"observed_unmatched,omitempty"`
	Satisfied    bool           `json:"satisfied"`
	Unsatisfied  []string       `json:"unsatisfied"`
	Omitted      map[string]int `json:"omitted"`
	Digest       string         `json:"digest"`
}
type edgeOccurrence struct {
	flow     Flow
	from, to Step
}

// provenBy returns the test key of a PROVEN ledger row whose crossing is VERIFIED by a receipt
// with a valid test key; any other row proves nothing.
func provenBy(r LedgerEdge) (string, bool) {
	if r.Status != EdgeProven || r.Evidence == nil || r.Evidence.Verification.Status != Verified || r.Evidence.Verification.Receipt == nil {
		return "", false
	}
	key := r.Evidence.Verification.Receipt.TestKey
	return key, validTestKey(key)
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

const MaxLedgerObservations = 16

// observedTraceConsistent is true when the trace has the shape Observe emits: visits in sequence,
// one edge per adjacent visit pair whose endpoints are those visits' screens, resolved exactly when
// both visits resolve to map screens, with a trigger from the closed vocabulary.
func observedTraceConsistent(m *Map, tr ObservedTrace) bool {
	if !validTestKey(tr.TestKey) || tr.Visits == nil || tr.Edges == nil {
		return false
	}
	for i, v := range tr.Visits {
		if v.Seq != i || (v.Status == StatusResolved) != (v.Screen != "") || (v.Screen != "" && m.screen(v.Screen) == nil) {
			return false
		}
	}
	if len(tr.Edges) != max(len(tr.Visits)-1, 0) {
		return false
	}
	for i, e := range tr.Edges {
		a, b := tr.Visits[i], tr.Visits[i+1]
		resolved := a.Status == StatusResolved && b.Status == StatusResolved
		if e.FromSeq != i || e.ToSeq != i+1 || e.From != a.Screen || e.To != b.Screen || (e.Status == StatusResolved) != resolved ||
			(e.Trigger != StatusUnknown && !typedTrigger(e.Trigger)) {
			return false
		}
	}
	return true
}

// LoadObservations refuses more than MaxLedgerObservations names before reading any file.
func LoadObservations(m *Map, names []string) ([]*Observation, error) {
	if len(names) > MaxLedgerObservations {
		return nil, verifyError("appmap-invalid-query", "at most %d --observation", MaxLedgerObservations)
	}
	var out []*Observation
	for _, name := range names {
		obs, err := LoadObservation(m, name)
		if err != nil {
			return nil, err
		}
		out = append(out, obs)
	}
	return out, nil
}

// LoadObservation reads one complete application-navigation-observation/0 document built from m:
// same app and map digest, nothing omitted and an intact digest (ANL-V0-023).
func LoadObservation(m *Map, name string) (*Observation, error) {
	bad := verifyError("appmap-invalid-query", "--observation must name an untrimmed application-navigation-observation/0 file of at most 4 MiB built from --map")
	raw, ok := readRegular(name, 4<<20)
	if !ok {
		return nil, bad
	}
	var obs Observation
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	if dec.Decode(&obs) != nil || dec.Decode(new(any)) != io.EOF || obs.Schema != ObservationSchema || obs.App != m.App || obs.MapDigest != m.Digest || obs.Traces == nil {
		return nil, bad
	}
	for _, n := range obs.Omitted {
		if n != 0 {
			return nil, bad
		}
	}
	for _, tr := range obs.Traces {
		if !observedTraceConsistent(m, tr) {
			return nil, bad
		}
	}
	check := obs
	check.Digest = ""
	complete, _ := json.Marshal(check)
	if digest(complete) != obs.Digest {
		return nil, bad
	}
	return &obs, nil
}

// ProjectEdges returns a capped projection and the full-ledger assumption verdict. It does not
// mutate the map or verification input. All run qualification is delegated to RVN (ANL-V0-002).
func ProjectEdges(ctx context.Context, m *Map, v *Verification, d *EdgeDeclarations, required []string, o Options) ([]byte, bool, error) {
	return ProjectObservedEdges(ctx, m, v, d, nil, required, o)
}

// ProjectObservedEdges is ProjectEdges with navigation observations: an observed crossing binds its
// test key to the matching ledger edge, like a declared proves, and still needs a qualified receipt
// at the observation's revision to prove it (ANL-V0-023).
func ProjectObservedEdges(ctx context.Context, m *Map, v *Verification, d *EdgeDeclarations, observations []*Observation, required []string, o Options) ([]byte, bool, error) {
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
	assumers := map[string][]string{}
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
				assumers[id] = append(assumers[id], t.TestKey)
			}
		}
	}
	if len(observations) > MaxLedgerObservations {
		return nil, false, verifyError("appmap-invalid-query", "at most %d --observation", MaxLedgerObservations)
	}
	// observedAt[edge][key] holds the revisions at which key's trace crossed the edge.
	observedAt := map[string]map[string]map[string]bool{}
	for _, obs := range observations {
		if obs == nil || obs.App != m.App || obs.MapDigest != m.Digest {
			return nil, false, verifyError("appmap-invalid-query", "observation not built from --map")
		}
		ledger.Observations = append(ledger.Observations, obs.Digest)
		for _, tr := range obs.Traces {
			for _, oe := range tr.Edges {
				if oe.Status != StatusResolved {
					continue
				}
				matched := false
				for id, row := range rows {
					if row.From != oe.From || row.To != oe.To || (row.Trigger != StatusUnknown && row.Trigger != oe.Trigger) {
						continue
					}
					matched = true
					if observedAt[id] == nil {
						observedAt[id] = map[string]map[string]bool{}
					}
					if observedAt[id][tr.TestKey] == nil {
						observedAt[id][tr.TestKey] = map[string]bool{}
					}
					observedAt[id][tr.TestKey][obs.Revision] = true
				}
				if !matched {
					ledger.Unmatched++
				}
			}
		}
	}
	sort.Strings(ledger.Observations)
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
			declared := map[string]bool{}
			for k := range keys {
				declared[k] = true
			}
			for _, k := range proves[id] {
				keys[k] = true
				declared[k] = true
			}
			for k := range observedAt[id] {
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
				anchors := append(append(e.m.stepAnchors(occ.from), e.m.stepAnchors(occ.to)...), occ.flow.Anchor)
				// Isolate one test key while sharing the RVN freshness cache and receipt admission.
				single := &Verification{receipts: v.receipts, binds: map[string][]string{}}
				if !declared[key] {
					// An observation-only key is read only from its outcomes at an observed
					// revision; outcomes elsewhere are not evidence about this crossing.
					single.receipts = receiptsAt(e, v.receipts, key, observedAt[id][key], anchors)
				}
				ve := *e
				ve.v = single
				if keyUncertain(&ve, key, anchors) {
					blocked = true
				}
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
				if !declared[key] && sv.Status != Unverified && !observedAt[id][key][sv.Revision] {
					// An observation-only binding proves nothing unless the receipt ran at the
					// revision the trace was observed at.
					sv = StepVerification{Status: Unverified, Reason: "observation-revision-mismatch", Authority: AuthorityLearned}
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
		for k := range observedAt[id] {
			row.ObservedBy = append(row.ObservedBy, k)
		}
		sort.Strings(row.ObservedBy)
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
		if rows[id].Status != EdgeProven || selfProvenOnly(rows[id], assumers[id]) {
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

// receiptsAt keeps the receipts holding an outcome for key at one of the revisions or at a
// revision that does not resolve in the repository: only an outcome placed at another resolved
// commit is set aside, so uncertainty that might be at an observed revision still blocks.
func receiptsAt(e *verifier, receipts []receiptInput, key string, revisions map[string]bool, anchors []Anchor) []receiptInput {
	var kept []receiptInput
	for _, in := range receipts {
		for _, t := range in.r.Tests {
			if t.ID != key {
				continue
			}
			rev, _ := jstestprovider.QualifiedApplicationRevision(in.r, t)
			if revisions[rev] || !commitOID.MatchString(rev) || e.anchorsAt(rev, anchors).evaluated != rev {
				kept = append(kept, in)
				break
			}
		}
	}
	return kept
}

// selfProvenOnly is true when some test assumes a proven edge that no other test proves: an
// edge cannot be proven only by the test it is used to select (ANL-V0-024). The prover set is the
// full ledger's, read before any budget trimming.
func selfProvenOnly(row LedgerEdge, assumedBy []string) bool {
	if row.Evidence == nil || row.Evidence.Verification.Receipt == nil {
		return false
	}
	provers := append([]string{row.Evidence.Verification.Receipt.TestKey}, row.RedundantCrossers...)
	for _, t := range assumedBy {
		other := false
		for _, k := range provers {
			if k != t {
				other = true
				break
			}
		}
		if !other {
			return true
		}
	}
	return false
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
