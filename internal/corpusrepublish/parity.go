package corpusrepublish

import (
	"bytes"
	"crypto/sha256"
	"fmt"
	"github.com/Beamfall/corvint/internal/doccorpus"
	"sort"
)

// encodedDigest streams the canonical encoding into a hash, so a member's size never
// decides parity; only a real encoding error is reported, and never as a change (DCP-V1-092).
func encodedDigest(v any) ([]byte, error) {
	h := sha256.New()
	if _, err := doccorpus.EncodeTo(h, v); err != nil {
		return nil, fmt.Errorf("republish parity encoding failed")
	}
	return h.Sum(nil), nil
}
func delta(before, after map[string]any) (Delta, error) {
	d := Delta{Added: []string{}, Retired: []string{}, Changed: []string{}}
	for k, v := range after {
		old, ok := before[k]
		if !ok {
			d.Added = append(d.Added, k)
			continue
		}
		x, err := encodedDigest(old)
		if err != nil {
			return d, err
		}
		y, err := encodedDigest(v)
		if err != nil {
			return d, err
		}
		if !bytes.Equal(x, y) {
			d.Changed = append(d.Changed, k)
		}
	}
	for k := range before {
		if _, ok := after[k]; !ok {
			d.Retired = append(d.Retired, k)
		}
	}
	sort.Strings(d.Added)
	sort.Strings(d.Retired)
	sort.Strings(d.Changed)
	return d, nil
}
func inventory(a *doccorpus.Artifact) [7]map[string]any {
	var x [7]map[string]any
	for i := range x {
		x[i] = map[string]any{}
	}
	if a == nil {
		return x
	}
	for _, r := range a.Subjects {
		x[0][r.ID] = r
	}
	for _, r := range a.Claims {
		x[1][r.ID] = r
	}
	for _, r := range a.Relations {
		x[2][r.ID] = r
	}
	for _, r := range a.Journeys {
		x[3][r.ID] = r
	}
	for _, r := range a.Observations {
		x[4][r.Link.ID] = r
	}
	for id, d := range a.Details {
		x[5][id] = d
	}
	for _, r := range a.Capabilities {
		x[6]["capability:"+r.Name] = r
	}
	for _, in := range a.Manifest.Inputs {
		key, _ := Encode(struct{ Provider, Path, Purpose string }{in.Provider, in.Path, in.Purpose})
		x[6]["input:"+string(key)] = in
	}
	for _, provider := range a.Manifest.Providers {
		x[6]["provider:"+provider.ID] = provider
	}
	x[6]["behavior"] = a.BehaviorContracts
	x[6]["stability"] = a.StabilityEvidence
	for _, j := range a.TestKnowledge {
		x[6]["test_knowledge:"+j.ID] = j
	}
	for _, j := range a.FlowRelations {
		x[6]["flow_relation:"+j.ID] = j
	}
	for _, j := range a.ClaimAudits {
		x[6]["claim_audit:"+j.ID] = j
	}
	for _, g := range a.SuppliedGaps {
		x[6]["supplied_gap:"+g.ID] = g
	}
	x[6]["gaps"] = a.Gaps
	x[6]["import_parity"] = a.ImportParity
	x[6]["restricted_summaries"] = a.RestrictedSummaries
	return x
}
func shardInventory(a *doccorpus.Artifact) map[string]doccorpus.ShardIdentity {
	out := map[string]doccorpus.ShardIdentity{}
	if a == nil {
		return out
	}
	for _, p := range a.Manifest.Providers {
		paths := p.Shards
		if paths == nil {
			if p.Kind == "native" {
				paths = []string{"@native"}
			} else {
				paths = []string{p.Record}
			}
		}
		for _, path := range paths {
			id := doccorpus.ShardIdentity{Provider: p.ID, Path: path}
			raw, _ := Encode(id)
			out[string(raw)] = id
		}
	}
	return out
}
func compare(previous, next *doccorpus.Artifact, allowed []doccorpus.ShardIdentity) (Parity, error) {
	x, y := inventory(previous), inventory(next)
	p := Parity{RetiredShards: []doccorpus.ShardIdentity{}}
	for i, out := range []*Delta{&p.Subjects, &p.Claims, &p.Relations, &p.Journeys, &p.Observations, &p.Details, &p.Semantics} {
		d, err := delta(x[i], y[i])
		if err != nil {
			return p, err
		}
		*out = d
	}
	prev, cur := shardInventory(previous), shardInventory(next)
	a, b := map[string]any{}, map[string]any{}
	for k, v := range prev {
		a[k] = v
	}
	for k, v := range cur {
		b[k] = v
	}
	shards, err := delta(a, b)
	if err != nil {
		return p, err
	}
	p.Shards = shards
	approved := map[doccorpus.ShardIdentity]bool{}
	for _, id := range allowed {
		approved[id] = true
	}
	for _, key := range p.Shards.Retired {
		id := prev[key]
		if !approved[id] {
			return p, fmt.Errorf("republish dropped shard without approved retirement")
		}
		p.RetiredShards = append(p.RetiredShards, id)
	}
	for id := range approved {
		found := false
		for _, v := range p.RetiredShards {
			found = found || v == id
		}
		if !found {
			return p, fmt.Errorf("republish retirement not in previous inventory")
		}
	}
	if _, err := Encode(p); err != nil {
		return p, err
	}
	return p, nil
}
func parityCounts(p Parity) (added, retired, changed int) {
	for _, d := range []Delta{p.Subjects, p.Claims, p.Relations, p.Journeys, p.Observations} {
		added += len(d.Added)
		retired += len(d.Retired)
		changed += len(d.Changed)
	}
	return
}
