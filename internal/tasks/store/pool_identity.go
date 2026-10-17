package store

import (
	"bytes"
	"context"
	"encoding/json"
	"io"

	"github.com/Beamfall/corvint/internal/tasks/intent"
	"github.com/Beamfall/corvint/internal/tasks/snapshot"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// poolIdentityFacts runs a guarded member's identity command for a
// POOL_CONFIRM_SAFE and returns the bound observation for the transaction
// (PSR-V0-083). It reads the policy and pool state the lease observer was
// handed, so the writer route is kept. A member without an identity command,
// or an entry that is not current, runs nothing (nil); the transaction then
// decides as before.
func poolIdentityFacts(ctx context.Context, repo *intent.Repository, choice LeaseChoice, policy, pools []byte) ([]byte, error) {
	if len(pools) == 0 {
		return nil, nil
	}
	p, e := intent.DecodePolicy(policy)
	if e != nil {
		return nil, e
	}
	state, e := snapshot.DecodePools(pools)
	if e != nil {
		return nil, e
	}
	var en *snapshot.PoolEntry
	for i := range state.Entries {
		x := &state.Entries[i]
		if x.MemberID == choice.Lease.Member && string(x.AllocationID) == choice.Lease.Allocation {
			en = x
		}
	}
	if en == nil || en.Sweep != nil {
		return nil, nil
	}
	pool := p.Pool(en.PoolID)
	if pool == nil || pool.DefaultSource == nil || pool.MemberConfig[en.MemberID].Identity == nil {
		return nil, nil
	}
	def := pool.MemberConfig[en.MemberID].Identity
	env, _ := gateEnvironment(def.Env)
	id := snapshot.PoolIdentity{PoolID: en.PoolID, MemberID: en.MemberID, AllocationID: en.AllocationID, DefinitionSha256: en.DefinitionSha256, OutputSha256: wire.Sum(nil), ObservedSeq: "0"}
	dir, guard, e := poolCommandDir(ctx, def, leaseRoot(repo, choice))
	if e != nil {
		id.Class = sweepSourceClass(e)
	} else {
		res := executePoolCapturedGuarded(ctx, def, dir, env, guard)
		id.Class, id.OutputSha256 = res.Class, wire.Sum(res.Stdout)
		if res.Class == "EXIT_ZERO" {
			id.Class, id.Source = identityClass(res.Stdout)
		}
	}
	id.ObservedAt = poolClock()
	return id.Encode(), nil
}

// identityClass reads the identity command's stdout: exactly one JSON object
// {"revision": OID, "tree": OID}, optionally followed by white space. Any
// other output is MALFORMED_OUTPUT and proves nothing (PSR-V0-084).
func identityClass(stdout []byte) (string, *intent.PoolSource) {
	var out struct {
		Revision *string `json:"revision"`
		Tree     *string `json:"tree"`
	}
	d := json.NewDecoder(bytes.NewReader(stdout))
	d.DisallowUnknownFields()
	if d.Decode(&out) != nil || out.Revision == nil || out.Tree == nil || d.More() {
		return "MALFORMED_OUTPUT", nil
	}
	// Only white space may follow: a stray '}' or ']' is a syntax error,
	// not the end of input (PSR-V0-084).
	var rest json.RawMessage
	if d.Decode(&rest) != io.EOF {
		return "MALFORMED_OUTPUT", nil
	}
	s, e := intent.ParsePoolSource("identity", *out.Revision+":"+*out.Tree)
	if e != nil {
		return "MALFORMED_OUTPUT", nil
	}
	return snapshot.PoolIdentityReported, &s
}

// identityProbe memoizes poolIdentityFacts over the observed policy and pool
// state, so an observer retry over unchanged state reuses the observation and
// the command runs again only when the state it was bound to changed.
type identityProbe struct {
	key string
	raw []byte
}

func (m *identityProbe) facts(ctx context.Context, repo *intent.Repository, choice LeaseChoice, policy, pools []byte) ([]byte, error) {
	key := string(wire.Sum(append(append(append([]byte{}, policy...), 0), pools...)))
	if m.raw != nil && m.key == key {
		return m.raw, nil
	}
	raw, e := poolIdentityFacts(ctx, repo, choice, policy, pools)
	if e == nil {
		m.key, m.raw = key, raw
	}
	return raw, e
}
