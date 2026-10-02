package cli

import (
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
	report, err := store.PolicyUpdate(writerContext(), repo, actor, store.PolicyRequest{QueueID: observed.Head.QueueID.Raw, RequestID: request, ExpectedPolicyVersion: version, Policy: raw}, now)
	if err != nil {
		return errorResult(cmd, err)
	}
	res := mutateResult(cmd, report)
	o := res.Items[0].Obj
	o.Set("oldPolicySha256", digestOrNull(report.OldPolicySha256))
	o.Set("newPolicySha256", digestOrNull(report.NewPolicySha256))
	return res
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
