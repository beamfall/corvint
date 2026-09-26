package cli

import (
	"context"
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
	cmd := []string{"policy", "update"}
	if len(args) == 0 || args[0] != "update" {
		return usage([]string{"policy"}, "policy needs the verb update")
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
	report, err := store.PolicyUpdate(context.Background(), repo, actor, store.PolicyRequest{QueueID: observed.Head.QueueID.Raw, RequestID: request, ExpectedPolicyVersion: version, Policy: raw}, now)
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
