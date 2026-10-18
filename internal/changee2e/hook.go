package changee2e

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/Beamfall/corvint/internal/behaviorfalsify"
	"github.com/Beamfall/corvint/internal/jstestprovider"
)

// Hook roles BBF sets in CORVINT_BEHAVIOR_COMMAND_ROLE.
const (
	HookRoleControl = "control"
	HookRoleCleanup = "cleanup"
)

// RunHook is the BBF control and cleanup command for `prove` (CCE-V0-009 steps 4-5). The
// companion runs it when invoked with no arguments and CORVINT_BEHAVIOR_COMMAND_ROLE set. The
// cleanup role is a no-op: the control role writes only under its own scratch directory, never
// under HOME or the disposable root BBF runs it in.
func RunHook(ctx context.Context, role string, stdin io.Reader, stdout io.Writer) error {
	switch role {
	case HookRoleCleanup:
		return nil
	case HookRoleControl:
	default:
		return fmt.Errorf("e2e-hook-role-invalid: %q", role)
	}
	data, err := io.ReadAll(io.LimitReader(stdin, 32<<20))
	if err != nil {
		return err
	}
	var inv behaviorfalsify.Invocation
	if err := behaviorfalsify.Decode(data, &inv); err != nil {
		return err
	}
	receipt, err := runControl(ctx, inv)
	if err != nil {
		return err
	}
	out, err := behaviorfalsify.Encode(receipt)
	if err != nil {
		return err
	}
	_, err = stdout.Write(out)
	return err
}

// runControl perturbs the outcome assertion in a scratch copy of the proof tree and runs it once.
// A perturbation that cannot be applied is an error (BBF reads the failed hook process as
// infrastructure_failed); an unqualified run is an infrastructure receipt.
func runControl(ctx context.Context, inv behaviorfalsify.Invocation) (behaviorfalsify.HookReceipt, error) {
	def := inv.Control.Definition
	r := behaviorfalsify.HookReceipt{Schema: behaviorfalsify.HookSchema, PlanDigest: inv.PlanDigest,
		PerturbationSHA256: inv.Control.PerturbationSHA256, Target: inv.Target, Runner: inv.Runner, Attempt: inv.Attempt,
		Unrelated: []behaviorfalsify.CriterionObservation{}, Setup: []behaviorfalsify.SetupObservation{}, Artifacts: []behaviorfalsify.Artifact{}}
	obs := behaviorfalsify.CriterionObservation{CriterionID: inv.Target.CriterionID, AssertionID: inv.Target.AssertionID}
	line, err := strconv.Atoi(def["line"])
	if err != nil || def["tree"] == "" || def["file"] == "" || def["locator"] == "" || def["matcher"] == "" || def["marker"] == "" || def["replacement"] == "" {
		return r, errors.New("e2e-hook-definition-invalid")
	}
	var environment *Environment
	if raw, ok := def["environment"]; ok {
		environment = &Environment{}
		if json.Unmarshal([]byte(raw), environment) != nil || environment.validate() != nil {
			return r, errors.New("e2e-hook-definition-invalid")
		}
	}
	nodeBin, modules := os.Getenv(envNodeBin), os.Getenv(envModules)
	if !filepath.IsAbs(nodeBin) || modules == "" {
		return r, errors.New("e2e-hook-toolchain-unavailable")
	}
	scratch, err := os.MkdirTemp("", "corvint-e2e-hook-")
	if err != nil {
		return r, err
	}
	defer os.RemoveAll(scratch)
	home := filepath.Join(scratch, "home")
	if err := os.MkdirAll(home, 0o755); err != nil {
		return r, err
	}
	restore := setEnv(map[string]string{"HOME": home,
		"PATH": filepath.Dir(nodeBin) + string(os.PathListSeparator) + os.Getenv("PATH")})
	defer restore()

	tree := filepath.Join(scratch, "tree")
	if err := extractRev(ctx, def["tree"], inv.Target.TestRevision, tree); err != nil {
		return r, err
	}
	file := filepath.Join(tree, filepath.FromSlash(def["file"]))
	text, err := os.ReadFile(file)
	if err != nil {
		return r, err
	}
	perturbed, err := perturbLine(text, line, inv.Control.Kind, def["locator"], def["matcher"], def["value"], def["replacement"])
	if err != nil {
		return r, err
	}
	if err := os.WriteFile(file, perturbed, 0o644); err != nil {
		return r, err
	}
	resolvedModules, err := filepath.EvalSymlinks(modules)
	if err != nil {
		return r, err
	}
	receipt, runErr := runPlaywright(ctx, runInput{Tree: tree, AppIdentity: inv.Target.ApplicationRevision, Placement: def["file"],
		Repeats: 1, NodeModules: resolvedModules, RunnerVersion: inv.Runner.RunnerVersion,
		PackageJSON: def["package_json"], Lockfile: def["lockfile"], Timeout: hookRunTimeout, Environment: environment})
	digest, reason := qualify(receipt, runErr, 1)
	if reason != "" {
		r.TestOutcome = "infrastructure_failed"
		obs.State = "not_run"
		r.TargetObservation = obs
		r.Infrastructure = &behaviorfalsify.InfrastructureFailure{Reason: infrastructureReason(ctx, receipt), Detail: reason}
		raw, _ := json.Marshal(r)
		r.NativeReceiptSHA256 = "sha256:" + sha256Hex(raw)
		return r, nil
	}
	r.NativeReceiptSHA256 = digest
	t := receipt.Tests[0]
	locator := def["locator"]
	if inv.Control.Kind == behaviorfalsify.WrongLocator {
		locator = def["replacement"]
	}
	switch t.State {
	case jstestprovider.StateFailed:
		r.TestOutcome, obs.State, obs.FailureKind = "failed", "failed", "unexpected"
		if failsAtAssertion(string(t.State), t.FailureMessage, def["marker"], def["matcher"], locator) {
			obs.FailureKind = "assertion"
		}
	case jstestprovider.StatePassed:
		r.TestOutcome, obs.State = "passed", "passed"
	default:
		r.TestOutcome, obs.State, obs.FailureKind = string(t.State), string(t.State), "unexpected"
	}
	r.TargetObservation = obs
	return r, nil
}

func infrastructureReason(ctx context.Context, r jstestprovider.Receipt) string {
	switch {
	case errors.Is(ctx.Err(), context.DeadlineExceeded):
		return "timeout"
	case r.Cancelled || ctx.Err() != nil:
		return "cancelled"
	}
	return "environment-unavailable"
}

// perturbLine replaces exactly one quoted locator (wrong-locator) or expected value
// (wrong-expected-value) on the 1-based line, keeping the literal's quote style.
func perturbLine(text []byte, line int, kind behaviorfalsify.ControlKind, locator, matcher, value, replacement string) ([]byte, error) {
	lines := strings.SplitAfter(string(text), "\n")
	if line < 1 || line > len(lines) {
		return nil, errors.New("e2e-hook-line-out-of-range")
	}
	var prefix, original string
	switch kind {
	case behaviorfalsify.WrongLocator:
		prefix, original = "getByTestId(", locator
	case behaviorfalsify.WrongExpectedValue:
		prefix, original = "."+matcher+"(", value
	default:
		return nil, fmt.Errorf("e2e-hook-kind-unsupported: %s", kind)
	}
	l := lines[line-1]
	for _, quote := range []func(string) string{jsString, jsonQuoted} {
		old, repl := prefix+quote(original), prefix+quote(replacement)
		if strings.Count(l, old) == 1 {
			lines[line-1] = strings.Replace(l, old, repl, 1)
			return []byte(strings.Join(lines, "")), nil
		}
	}
	return nil, errors.New("e2e-hook-assertion-not-found")
}

func jsonQuoted(s string) string {
	b, _ := json.Marshal(s)
	return string(b)
}
