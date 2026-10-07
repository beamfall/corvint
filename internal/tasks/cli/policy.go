package cli

import (
	"fmt"
	"path/filepath"
	"time"

	"github.com/Beamfall/corvint/internal/tasks/intent"
	"github.com/Beamfall/corvint/internal/tasks/mutation"
	"github.com/Beamfall/corvint/internal/tasks/snapshot"
	"github.com/Beamfall/corvint/internal/tasks/store"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// policyCommand runs `corvint-tasks policy update` (ATM-V0-027, TM-V0-030):
// the only writer of intent/policy.json after init.
func policyCommand(env Env, args []string) *wire.Result {
	if len(args) > 0 && args[0] == "show" {
		return policyShow(env, args[1:])
	}
	cmd := []string{"policy", "update"}
	if len(args) == 0 || args[0] != "update" {
		return usage([]string{"policy"}, "policy needs the verb show or update")
	}
	args = args[1:]
	role, request, expected, file := "OWNER", "", "", ""
	values := map[string]*string{"--role": &role, "--request-id": &request, "--expected-policy-version": &expected, "--file": &file}
	seen := map[string]bool{}
	for i := 0; i < len(args); i++ {
		key := args[i]
		destination, ok := values[key]
		if !ok || seen[key] || i+1 >= len(args) {
			return usage(cmd, "policy update requires --request-id ID --expected-policy-version N --file PATH and optional --role OWNER|OPERATOR; duplicate and unknown flags refuse")
		}
		seen[key] = true
		i++
		*destination = args[i]
	}
	if !seen["--request-id"] || !seen["--expected-policy-version"] || !seen["--file"] {
		return usage(cmd, "policy update requires --request-id ID --expected-policy-version N --file PATH")
	}
	if _, err := mutation.ParseRequestID("requestId", request); err != nil {
		return errorResult(cmd, err)
	}
	version, err := wire.ParseSize("expectedPolicyVersion", expected)
	if err != nil {
		return errorResult(cmd, err)
	}
	actor, err := initActor(role)
	if err != nil {
		return errorResult(cmd, err)
	}
	if !filepath.IsAbs(file) {
		file = filepath.Join(env.Cwd, file)
	}
	raw, err := intent.ReadFile(file, wire.MaxPolicyFileBytes)
	if err != nil {
		return errorResult(cmd, err)
	}
	repo, err := intent.Resolve(env.Cwd)
	if err != nil {
		return errorResult(cmd, err)
	}
	observed, err := snapshot.Probe(repo.StateDir)
	if err != nil {
		return errorResult(cmd, err)
	}
	now, err := wire.ParseTimestamp("recordedAt", time.Now().UTC().Format("2006-01-02T15:04:05Z"))
	if err != nil {
		return errorResult(cmd, err)
	}
	fences, fenceErr := handoffFencePreview(env, raw)
	report, err := store.PolicyUpdate(writerContext(), repo, actor, store.PolicyRequest{QueueID: observed.Head.QueueID.Raw, RequestID: request, ExpectedPolicyVersion: version, Policy: raw}, now)
	if err != nil {
		return errorResult(cmd, err)
	}
	res := mutateResult(cmd, report)
	o := res.Items[0].Obj
	o.Set("oldPolicySha256", digestOrNull(report.OldPolicySha256))
	o.Set("newPolicySha256", digestOrNull(report.NewPolicySha256))
	if report.Outcome.Outcome == mutation.OutcomeCompleted {
		if fenceErr != nil {
			o.Set("handoffFences", wire.Null())
			res.Warnings = append(res.Warnings, "handoff fence preview NOT_OBSERVED ("+wire.CodeOf(fenceErr)+"); the policy update is committed")
		} else {
			o.Set("handoffFences", fences)
			if n := len(fences.Arr); n > 0 {
				res.Warnings = append(res.Warnings, fmt.Sprintf("this policy change fences or may fence the clean HANDOFF/REVIEW_RETURNED release of %d live attempt(s) listed in handoffFences (CAL-V0-124); the preview was read before the commit, outside the writer lock", n))
			}
		}
	}
	return res
}

// handoffFencePreview is the CAL-V0-124 advisory read taken before the
// writer runs: one pinned snapshot of the current policy and live attempts.
// It lists each live unsupervised external-agent attempt whose clean handoff
// the new policy would refuse STALE_POLICY (FENCED), or whose interval began
// under an earlier policy whose history this read does not audit
// (NOT_OBSERVED). It is outside the writer lock and never writes.
func handoffFencePreview(env Env, next []byte) (wire.Value, error) {
	var out []wire.Value
	_, err := withStore(env, func(rc *readCtx) error {
		out = []wire.Value{}
		live, err := liveAttempts(rc)
		if err != nil {
			return err
		}
		current := rc.store.Policy.Raw
		for _, a := range live {
			if a.RuntimeID != snapshot.RuntimeExternalAgent || a.Supervision != nil || a.ConfigSha256 != a.PolicySha256 {
				continue // never eligible for the CAL-V0-044 policy-compatible handoff
			}
			pool, member := wire.Null(), wire.Null()
			poolID, memberID := "", ""
			if a.PoolAllocation != nil {
				poolID, memberID = a.PoolAllocation.PoolID, a.PoolAllocation.MemberID
				pool, member = wire.String(poolID), wire.String(memberID)
			}
			status := "NOT_OBSERVED"
			if a.PolicySha256 == wire.Sum(current) {
				if ok, err := intent.HandoffPolicyCompatible(current, next, poolID, memberID); err == nil && ok {
					continue
				}
				status = "FENCED"
			}
			o := wire.NewObject().Set("attemptId", wire.String(a.AttemptID)).Set("ticketId", wire.String(a.TicketID.Raw)).Set("generation", wire.String(string(a.Generation))).Set("stage", wire.String(a.Stage)).Set("holder", wire.String(a.Lease.Holder)).Set("poolId", pool).Set("memberId", member).Set("handoff", wire.String(status))
			out = append(out, wire.ObjectValue(o))
		}
		return nil
	})
	return wire.Array(out...), err
}

func digestOrNull(d wire.Digest) wire.Value {
	if d == "" {
		return wire.Null()
	}
	return wire.String(string(d))
}

// policyShow binds the effective bytes, version and digest to one audited read.
func policyShow(env Env, args []string) *wire.Result {
	cmd := []string{"policy", "show"}
	if len(args) != 0 {
		return usage(cmd, "policy show takes no arguments")
	}
	var item wire.Value
	rc, err := withStore(env, func(rc *readCtx) error {
		p, err := wire.Parse(rc.store.Policy.Raw)
		if err != nil {
			return err
		}
		item = wire.ObjectValue(wire.NewObject().Set("policy", p).Set("policyVersion", wire.String(string(rc.store.Policy.PolicyVersion))).Set("policySha256", wire.String(string(rc.store.Policy.PolicySha256()))))
		return nil
	})
	if err != nil {
		return failure(cmd, rc, err)
	}
	res := success(cmd, rc)
	res.Items = []wire.Value{item}
	return res
}
