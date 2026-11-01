package store

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
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
	plan, e := identityPlan(choice, policy, pools)
	if plan == nil || e != nil {
		return nil, e
	}
	return plan.run(ctx, repo, choice), nil
}

// identityRun is one identity command a confirm-safe needs, decided from the
// policy and pool state alone; deciding it runs nothing.
type identityRun struct {
	def *intent.PoolCommand
	id  snapshot.PoolIdentity
}

// identityPlan decides, without running anything, whether the state needs an
// identity observation: nil for a state without pools, an entry that is not
// current, or a member without an identity command.
func identityPlan(choice LeaseChoice, policy, pools []byte) (*identityRun, error) {
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
	return &identityRun{def: pool.MemberConfig[en.MemberID].Identity, id: snapshot.PoolIdentity{PoolID: en.PoolID, MemberID: en.MemberID, AllocationID: en.AllocationID, DefinitionSha256: en.DefinitionSha256, OutputSha256: wire.Sum(nil), ObservedSeq: "0"}}, nil
}

func (r *identityRun) run(ctx context.Context, repo *intent.Repository, choice LeaseChoice) []byte {
	def, id := r.def, r.id
	env, _ := gateEnvironment(def.Env)
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
	return id.Encode()
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
	if d.Decode(&out) != nil || out.Revision == nil || out.Tree == nil {
		return "MALFORMED_OUTPUT", nil
	}
	// Only white space may follow: a stray '}' or ']' is trailing content,
	// not the end of input, so More() alone cannot decide it (PSR-V0-084,
	// V1-3358). The check is inline because Tasks imports no Core package,
	// internal/jsonend included (TestImportDirection).
	if _, err := d.Token(); !errors.Is(err, io.EOF) {
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
//
// Under the writer lock the probe runs nothing (PSR-V0-168): locked serves
// only an observation memoized for byte-identical policy and pool state, and
// otherwise records that state and refuses with errIdentityDeferred. settle
// then runs the command for the recorded state with no lock or preparation
// gate held, and the retried write revalidates it under the lock by that
// byte-identical key.
type identityProbe struct {
	key     string
	raw     []byte
	pending *identityPending
}

// identityPending is the audited state a locked observer could not serve.
type identityPending struct {
	key           string
	policy, pools []byte
}

// errIdentityDeferred is a writer-locked observer's decline: the identity
// observation for the audited state was not yet made, and making it would
// run the member's command while the lock is held (V1-1656).
var errIdentityDeferred = errWriterRoute("identity", "pool identity probe deferred until the writer lock is released")

func identityKey(policy, pools []byte) string {
	return string(wire.Sum(append(append(append([]byte{}, policy...), 0), pools...)))
}

func (m *identityProbe) facts(ctx context.Context, repo *intent.Repository, choice LeaseChoice, policy, pools []byte) ([]byte, error) {
	key := identityKey(policy, pools)
	if m.raw != nil && m.key == key {
		return m.raw, nil
	}
	raw, e := poolIdentityFacts(ctx, repo, choice, policy, pools)
	if e == nil {
		m.key, m.raw = key, raw
	}
	return raw, e
}

// locked is facts for a caller holding the writer lock: it never runs the
// command.
func (m *identityProbe) locked(choice LeaseChoice, policy, pools []byte) ([]byte, error) {
	key := identityKey(policy, pools)
	if m.raw != nil && m.key == key {
		return m.raw, nil
	}
	plan, e := identityPlan(choice, policy, pools)
	if plan == nil || e != nil {
		return nil, e
	}
	m.pending = &identityPending{key: key, policy: append([]byte{}, policy...), pools: append([]byte{}, pools...)}
	return nil, errIdentityDeferred
}

// settle runs the command a locked observer deferred, against the state it
// recorded, and reports whether it ran. Its caller holds no lock and no
// preparation gate.
func (m *identityProbe) settle(ctx context.Context, repo *intent.Repository, choice LeaseChoice) bool {
	p := m.pending
	m.pending = nil
	if p == nil {
		return false
	}
	_, e := m.facts(ctx, repo, choice, p.policy, p.pools)
	return e == nil
}
