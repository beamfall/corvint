package cli

import (
	"fmt"
	"path/filepath"
	"strconv"
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
	dryRun := false
	for i := 0; i < len(args); i++ {
		key := args[i]
		if key == "--dry-run" && !dryRun {
			dryRun = true
			continue
		}
		destination, ok := values[key]
		if !ok || seen[key] || i+1 >= len(args) {
			return usage(cmd, "policy update requires --request-id ID --expected-policy-version N --file PATH and optional --role OWNER|OPERATOR and --dry-run; duplicate and unknown flags refuse")
		}
		seen[key] = true
		i++
		*destination = args[i]
	}
	if (!dryRun && !seen["--request-id"]) || !seen["--expected-policy-version"] || !seen["--file"] {
		return usage(cmd, "policy update requires --request-id ID --expected-policy-version N --file PATH (--dry-run needs no --request-id)")
	}
	if seen["--request-id"] {
		if _, err := mutation.ParseRequestID("requestId", request); err != nil {
			return errorResult(cmd, err)
		}
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
	observed, pending, err := writerProbe(nil, repo.StateDir)
	if err != nil {
		return errorResult(cmd, err)
	}
	if dryRun {
		return policyDryRun(env, cmd, raw, version, pending)
	}
	now, err := wire.ParseTimestamp("recordedAt", time.Now().UTC().Format("2006-01-02T15:04:05Z"))
	if err != nil {
		return errorResult(cmd, err)
	}
	// The preview is a read: over a pending receipt it is not observed
	// rather than waiting out read patience the writer does not need.
	var fences wire.Value
	var fenceErr error = wire.Errorf(wire.CodeRedoPending, "head.json", "a committed receipt is not settled yet")
	if !pending {
		fences, fenceErr = handoffFencePreview(env, raw)
	}
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

// policyDryRun is `policy update --dry-run` (TCL-V0-821): it checks the file
// as the write would decode it, then returns the CAL-V0-124 fence preview
// and writes nothing. The later write repeats every check under the lock.
func policyDryRun(env Env, cmd []string, raw []byte, expected wire.Size, pending bool) *wire.Result {
	next, err := intent.DecodePolicy(raw)
	if err != nil {
		return errorResult(cmd, err)
	}
	if next.PolicyVersion.Uint64() != expected.Uint64()+1 {
		return failure(cmd, nil, wire.Errorf(wire.CodeMalformed, "/policyVersion", "policyVersion %s must be --expected-policy-version plus one (%d)", next.PolicyVersion, expected.Uint64()+1))
	}
	res := &wire.Result{Command: cmd, Outcome: wire.OutcomeOK}
	o := wire.NewObject().Set("dryRun", wire.Bool(true)).Set("newPolicySha256", wire.String(string(wire.Sum(raw))))
	var fences wire.Value
	var fenceErr error = wire.Errorf(wire.CodeRedoPending, "head.json", "a committed receipt is not settled yet")
	if !pending {
		fences, fenceErr = handoffFencePreview(env, raw)
	}
	if fenceErr != nil {
		o.Set("handoffFences", wire.Null())
		res.Warnings = append(res.Warnings, "handoff fence preview NOT_OBSERVED ("+wire.CodeOf(fenceErr)+"); dry run, nothing was written")
	} else {
		o.Set("handoffFences", fences)
		if n := len(fences.Arr); n > 0 {
			res.Warnings = append(res.Warnings, fmt.Sprintf("this policy change would fence or may fence the clean HANDOFF/REVIEW_RETURNED release of %d live attempt(s) listed in handoffFences (CAL-V0-124); dry run, nothing was written", n))
		}
	}
	res.Items = []wire.Value{wire.ObjectValue(o)}
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
// heartbeatTTLSeconds is the effective holder observation TTL every holder
// read uses (CAL-V0-278); it sits beside the policy, so the policy bytes and
// digest stay those of the stored file.
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
		item = wire.ObjectValue(wire.NewObject().Set("policy", p).Set("policyVersion", wire.String(string(rc.store.Policy.PolicyVersion))).Set("policySha256", wire.String(string(rc.store.Policy.PolicySha256()))).Set("heartbeatTTLSeconds", wire.String(strconv.FormatInt(rc.store.Policy.HeartbeatTTLSeconds(), 10))))
		return nil
	})
	if err != nil {
		return failure(cmd, rc, err)
	}
	res := success(cmd, rc)
	res.Items = []wire.Value{item}
	return res
}
