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
	// An omitted-event control removes a line and needs no assertion binding (CCE-V0-036).
	unbound := inv.Control.Kind != behaviorfalsify.OmittedEvent && (def["locator"] == "" || def["matcher"] == "" || def["marker"] == "")
	if err != nil || def["tree"] == "" || def["file"] == "" || def["replacement"] == "" || unbound {
		return r, errors.New("e2e-hook-definition-invalid")
	}
	var environment *Environment
	if raw, ok := def["environment"]; ok {
		environment = &Environment{}
		if json.Unmarshal([]byte(raw), environment) != nil || environment.validate() != nil ||
			(def["serve_bin"] != "" && !filepath.IsAbs(def["serve_bin"])) {
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
	perturbed, err := perturbLine(text, line, inv.Control.Kind, def["locator_form"], def["locator"], def["matcher"], def["value"], def["replacement"])
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
		PackageJSON: def["package_json"], Lockfile: def["lockfile"], Timeout: hookRunTimeout, Environment: environment,
		ServeBin: def["serve_bin"], ServePath: def["serve_path"], Grep: def["grep"], SessionEnv: def["session_env"]})
	digest, reason := qualify(receipt, runErr, 1, treeRoots(tree))
	if reason != "" {
		r.TestOutcome = "infrastructure_failed"
		obs.State = "not_run"
		r.TargetObservation = obs
		r.Infrastructure = &behaviorfalsify.InfrastructureFailure{Reason: infrastructureReason(ctx, receipt),
			Detail: hookInfrastructureDetail(reason, receipt, runErr, treeRoots(tree))}
		raw, _ := json.Marshal(r)
		r.NativeReceiptSHA256 = "sha256:" + sha256Hex(raw)
		return r, nil
	}
	r.NativeReceiptSHA256 = digest
	t := receipt.Tests[0]
	switch {
	case inv.Control.Kind == behaviorfalsify.OmittedEvent && (t.State == jstestprovider.StateFailed || t.State == jstestprovider.StateTimedOut):
		// The removed step or fixture setup was load-bearing: the scenario no longer completes
		// (CCE-V0-029). BBF reads only an assertion failure as killed.
		r.TestOutcome, obs.State, obs.FailureKind = "failed", "failed", "assertion"
	case t.State == jstestprovider.StateFailed:
		r.TestOutcome, obs.State, obs.FailureKind = "failed", "failed", "unexpected"
		// The template controls perturb the outcome assertion's own line; only a failure the
		// receipt locates there is an assertion failure (CCE-V0-009 step 6).
		if file, at := receiptLocation(treeRoots(tree), t); file == def["file"] && at == line {
			obs.FailureKind = "assertion"
		}
	case t.State == jstestprovider.StatePassed:
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
// (wrong-expected-value) on the 1-based line, keeping the literal's quote style; a typed locator's
// accessible name is its locator, and a count is unquoted (CCE-V0-109, CCE-V0-110). For
// omitted-event it replaces the awaited statement on the line with a comment, so every later
// line, including the outcome assertion, keeps its number.
func perturbLine(text []byte, line int, kind behaviorfalsify.ControlKind, form, locator, matcher, value, replacement string) ([]byte, error) {
	lines := strings.SplitAfter(string(text), "\n")
	if line < 1 || line > len(lines) {
		return nil, errors.New("e2e-hook-line-out-of-range")
	}
	var prefix, original string
	switch kind {
	case behaviorfalsify.OmittedEvent:
		l := lines[line-1]
		body := strings.TrimLeft(l, " ")
		indent := l[:len(l)-len(body)]
		if !strings.HasPrefix(body, "await ") || strings.Contains(body, "expect(") {
			return nil, errors.New("e2e-hook-event-not-found")
		}
		lines[line-1] = indent + "// corvint-test e2e prove: omitted-event control\n"
		return []byte(strings.Join(lines, "")), nil
	case behaviorfalsify.WrongLocator:
		prefix, original = "getByTestId(", locator
		switch form {
		case "role":
			prefix = "name: "
		case "label":
			prefix = "getByLabel("
		case "":
		default:
			return nil, fmt.Errorf("e2e-hook-locator-form-unsupported: %s", form)
		}
	case behaviorfalsify.WrongExpectedValue:
		prefix, original = "."+matcher+"(", value
	default:
		return nil, fmt.Errorf("e2e-hook-kind-unsupported: %s", kind)
	}
	l := lines[line-1]
	quotes := []func(string) string{jsString, jsonQuoted}
	if kind == behaviorfalsify.WrongExpectedValue && matcher == MatcherCount {
		quotes = []func(string) string{func(s string) string { return s + ")" }}
	}
	for _, quote := range quotes {
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
