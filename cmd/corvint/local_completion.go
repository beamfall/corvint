package main

import (
	"context"
	"errors"
	"io"
	"os"
	"strings"

	"github.com/Beamfall/corvint/internal/cem/cli"
	"github.com/Beamfall/corvint/internal/localcompletion"
	"github.com/Beamfall/corvint/internal/tracerecordrepo"
)

func parseLocalCompletionInvocation(arguments []string) (string, []string, bool, error) {
	index := 0
	root := ""
	for index < len(arguments) && (arguments[index] == "--root" || strings.HasPrefix(arguments[index], "--root=")) {
		if arguments[index] == "--root" {
			if !rootPreambleValue(arguments, index+1) {
				return "", nil, false, nil
			}
			root = arguments[index+1]
			index += 2
		} else {
			root = strings.TrimPrefix(arguments[index], "--root=")
			index++
		}
	}
	if index >= len(arguments) || arguments[index] != "dogfood" {
		return "", nil, false, nil
	}
	if root == "" {
		current, err := os.Getwd()
		if err != nil {
			return "", nil, true, argumentError("current directory unavailable")
		}
		root = current
	} else {
		resolved, err := resolveExplicitRoot(root)
		if err != nil {
			return "", nil, true, err
		}
		root = resolved
	}
	return root, arguments[index+1:], true, nil
}

func runLocalCompletion(ctx context.Context, root string, args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	if len(args) > 0 && args[0] == "event" {
		return runLocalCompletionEvent(ctx, root, args[1:], stdin, stdout, stderr)
	}
	if len(args) > 0 && dogfoodFlowOptions[args[0]] != nil {
		return runDogfoodFlow(ctx, root, args, stdout, stderr)
	}
	if len(args) == 0 {
		return emitLocalCompletionFailure(stderr, "local-completion-action-required")
	}
	flags, err := localCompletionFlags(args)
	if err != nil {
		return emitLocalCompletionFailure(stderr, err.Error())
	}
	key, err := localcompletion.SessionKey(flags["--session-key"])
	if err != nil {
		return emitLocalCompletionFailure(stderr, err.Error())
	}
	if err = requireRepositoryRoot(root); err != nil {
		emitError(stderr, err)
		return 2
	}
	if args[0] == "handoff" {
		return runDogfoodHandoff(ctx, root, key, flags, stdout, stderr)
	}
	var result localcompletion.Evaluation
	switch args[0] {
	case "begin":
		raw, readErr := localcompletion.ReadPlan(flags["--plan"])
		if readErr != nil {
			return emitLocalCompletionFailure(stderr, readErr.Error())
		}
		result, err = localcompletion.Begin(ctx, root, key, raw)
	case "status":
		result, err = localcompletion.Evaluate(ctx, root, key)
	case "verify":
		result, err = localcompletion.Verify(ctx, root, key, flags["--check"])
	case "finish":
		if flags["--transport-adapted-recovery"] != "" {
			return runTransportAdaptedRecovery(ctx, root, key, flags, stdout, stderr)
		}
		command := localcompletion.PublicCommand(localCompletionPublicCommand)
		if wrapFinishCommand != nil {
			command = wrapFinishCommand(command)
		}
		result, err = localcompletion.FinishWithAggregateProfile(ctx, root, key, flags["--aggregate-outcome-profile"], command)
	case "review":
		result, err = localcompletion.Review(ctx, root, key, flags["--report-set"])
	case "cancel":
		result, err = localcompletion.Cancel(ctx, root, key, flags["--reason"])
	case "hold":
		result, err = localcompletion.Hold(ctx, root, key, flags["--reason"], flags["--release-authorization"])
	}
	if err != nil {
		if result.Lifecycle != "" {
			if len(result.NextActions) == 0 && result.NoActionReason == "" {
				result.NoActionReason = "evaluation-unavailable"
			}
			_ = emit(stdout, map[string]any{"ok": false, "profile": "corvint-local-completion/0", "tool": "dogfood-" + args[0], "policy": result})
		}
		var bound *localcompletion.CheckBoundError
		if errors.As(err, &bound) {
			return emitLocalCompletionFailureDetail(stderr, err.Error(), bound.Detail())
		}
		var reasonless *localcompletion.CancelReasonError
		if errors.As(err, &reasonless) {
			return emitLocalCompletionFailureDetail(stderr, err.Error(), reasonless.Detail())
		}
		var held *localcompletion.HoldError
		if errors.As(err, &held) {
			return emitLocalCompletionFailureDetail(stderr, err.Error(), held.Detail())
		}
		var drift *localcompletion.SealedDriftError
		if errors.As(err, &drift) {
			return emitLocalCompletionFailureDetail(stderr, err.Error(), drift.Detail())
		}
		var final *localcompletion.FinalCheckError
		if errors.As(err, &final) {
			return emitLocalCompletionFailureDetail(stderr, err.Error(), final.Detail())
		}
		return emitLocalCompletionFailure(stderr, err.Error())
	}
	code := localCompletionExit(args[0], flags["--check"], result)
	// ok mirrors the exit status (LCP-V0-017): a caller reading only ok never
	// sees an unqualified selected check or an unsatisfied finish as success.
	payload := map[string]any{"ok": code == 0, "profile": "corvint-local-completion/0", "tool": "dogfood-" + args[0], "mutates": args[0] != "status", "policy": result, "claim": "caller-owned-selected-workflow-only"}
	if reason := flags["--reason"]; reason != "" && args[0] == "cancel" {
		// LCP-V0-038: the caller's reason is echoed, never persisted in the
		// closed enrollment state.
		payload["cancelReason"] = reason
	}
	if err = emit(stdout, payload); err != nil {
		return emitLocalCompletionFailure(stderr, "output-failed")
	}
	return code
}

// localCompletionExit is 1 for an unsatisfied finish and for a verify whose
// selected check is not qualified, and 0 otherwise.
func localCompletionExit(action, checkID string, result localcompletion.Evaluation) int {
	if action == "finish" && !result.Satisfied {
		return 1
	}
	if action == "verify" {
		for _, check := range result.Checks {
			if check.ID == checkID && !check.Qualified {
				return 1
			}
		}
	}
	return 0
}

func localCompletionFlags(args []string) (map[string]string, error) {
	allowed := map[string]bool{"--session-key": true}
	required := ""
	switch args[0] {
	case "begin":
		required = "--plan"
	case "verify":
		required = "--check"
	case "review":
		required = "--report-set"
	case "finish":
		allowed["--aggregate-outcome-profile"] = true
		allowed["--transport-adapted-recovery"] = true
	case "status":
	case "cancel":
		allowed["--reason"] = true
	case "hold":
		// LCP-V0-880/881: exactly one of the two is given.
		allowed["--reason"], allowed["--release-authorization"] = true, true
	case "handoff":
		allowed["--anchors"], allowed["--receipt"] = true, true
		allowed["--task-state"], allowed["--bundle"] = true, true
	default:
		return nil, errors.New("invalid-local-completion-action")
	}
	if required != "" {
		allowed[required] = true
	}
	flags := map[string]string{}
	for index := 1; index < len(args); index++ {
		name, value, inline := strings.Cut(args[index], "=")
		if !allowed[name] {
			return nil, errors.New("invalid-local-completion-option")
		}
		if _, exists := flags[name]; exists {
			return nil, errors.New("duplicate-local-completion-option")
		}
		if !inline {
			index++
			if index >= len(args) || argparseOptionLike(args[index]) {
				return nil, errors.New("local-completion-option-value-required")
			}
			value = args[index]
		}
		if value == "" || len(value) > 4096 {
			return nil, errors.New("invalid-local-completion-option-value")
		}
		flags[name] = value
	}
	if required != "" && flags[required] == "" {
		return nil, errors.New("local-completion-option-required")
	}
	if flags["--anchors"] != "" && flags["--receipt"] != "" {
		return nil, errors.New("invalid-local-completion-option")
	}
	if flags["--task-state"] != "" || flags["--bundle"] != "" {
		if flags["--session-key"] == "" || flags["--receipt"] != "" || (flags["--bundle"] != "" && (flags["--task-state"] != "" || flags["--anchors"] != "")) {
			return nil, errors.New("invalid-local-completion-option")
		}
	}
	if flags["--transport-adapted-recovery"] != "" && flags["--aggregate-outcome-profile"] != tracerecordrepo.AggregateOutcomeProfile {
		return nil, errors.New("invalid-local-completion-option")
	}
	return flags, nil
}

// runTransportAdaptedRecovery is the recovery-only aggregate finish route
// (ALO-V0-023). Ordinary finish never reaches it.
func runTransportAdaptedRecovery(ctx context.Context, root, key string, flags map[string]string, stdout, stderr io.Writer) int {
	raw, err := localcompletion.ReadTransportRecoveryRequest(flags["--transport-adapted-recovery"])
	if err != nil {
		return emitLocalCompletionFailure(stderr, err.Error())
	}
	result, provenance, err := localcompletion.FinishAggregateRecovery(ctx, root, key, raw, localCompletionPublicCommand)
	if err != nil {
		if result.Lifecycle != "" {
			if len(result.NextActions) == 0 && result.NoActionReason == "" {
				result.NoActionReason = "evaluation-unavailable"
			}
			_ = emit(stdout, map[string]any{"ok": false, "profile": "corvint-local-completion/0", "tool": "dogfood-finish", "policy": result, "transportAdaptedRecovery": provenance})
		}
		return emitLocalCompletionFailure(stderr, err.Error())
	}
	payload := map[string]any{"ok": result.Satisfied, "profile": "corvint-local-completion/0", "tool": "dogfood-finish", "mutates": true, "policy": result, "claim": "caller-owned-selected-workflow-only", "transportAdaptedRecovery": provenance}
	if err = emit(stdout, payload); err != nil {
		return emitLocalCompletionFailure(stderr, "output-failed")
	}
	if !result.Satisfied {
		return 1
	}
	return 0
}

// wrapFinishCommand is nil in the shipped binary. The LCP-V0-060 signal
// regression's re-executed helper sets it to hold the final strict check open
// until a real SIGINT or SIGTERM reaches the CLI's signal context; a default
// value would form an initialization cycle through runContext.
var wrapFinishCommand func(localcompletion.PublicCommand) localcompletion.PublicCommand

func localCompletionPublicCommand(ctx context.Context, root string, args []string, stdout, stderr io.Writer) int {
	if len(args) < 2 {
		return 2
	}
	switch args[0] {
	case "cem":
		return cli.Run(ctx, root, args[1:], stdout, stderr)
	case "ocm":
		return runOCM(ctx, root, args[1:], stdout, stderr)
	}
	// The in-process dogfood flow runs every other public step as this binary
	// would (LCP-V0-014).
	return runContext(ctx, append([]string{"--root", root}, args...), strings.NewReader(""), stdout, stderr)
}

func emitLocalCompletionFailure(stderr io.Writer, code string) int {
	return emitLocalCompletionFailureDetail(stderr, code, "")
}

// emitLocalCompletionFailureDetail appends a fixed, typed detail (LCP-V0-032)
// to the remedy; callers pass only text built from bounded, validated values.
func emitLocalCompletionFailureDetail(stderr io.Writer, code, detail string) int {
	// Operational errors can contain paths or command text. Only fixed, bounded
	// reason codes belong on this new public surface.
	for _, r := range code {
		if !(r >= 'a' && r <= 'z') && r != '-' {
			code = "local-completion-failed"
			break
		}
	}
	if len(code) > 96 || code == "" {
		code = "local-completion-failed"
	}
	message := code
	if remedy, ok := localCompletionRemedies[code]; ok {
		message = code + ": " + remedy
	}
	if detail != "" && code != "local-completion-failed" {
		message += "; " + detail
	}
	// The top-level code is the CCF-V1-004 member every Core refusal carries; the nested error
	// object stays for readers of the earlier envelope.
	_ = emit(stderr, map[string]any{"ok": false, "code": code, "error": map[string]string{"code": code, "message": message}})
	return 2
}

// localCompletionRemedies names the fix for a plan refusal whose bare code hides
// the rule (LCP-V0-018, V1-1043). Each text is fixed, so the message still
// carries no path or command text; the code stays the stable member.
var localCompletionRemedies = map[string]string{
	"cancel-reason-required": "an active, unsatisfied enrollment is cancelled only with --reason TEXT; to move to a new base, leave it ACTIVE and begin a new enrollment in a fresh worktree or clone instead (docs/DOGFOOD.md)",
	"sealed-evidence-drift":  "a sealed continuation's pinned evidence no longer matches; the detail names the artifact, the reason and, for a byte change, the pinned and current digests. Only the lane closeout's own outcome record is re-pinned, so run the enrolled finish, then the lane closeout, then dogfood verify and finish (docs/DOGFOOD.md); restore any other named artifact instead of editing private evidence",
	"finish-interrupted":     "the caller's signal or cancellation ended finish during the named step; its logs are retained, no outcome or terminal was written, and rerunning the named finish reuses the unchanged bound results",
	"final-check-failed":     "the final strict dogfood check did not pass; read the named logs under the Git directory, fix the cause, and rerun finish",
	"prior-completion-stale": "this session's satisfied enrollment is bound to an earlier target and no longer evaluates as satisfied; begin the next stage of a staged binding in a fresh worktree or clone, and leave this one as it is (docs/DOGFOOD.md, staged binding)",
	"invalid-check-bound":    "each plan check takes 1-64 argv elements and timeoutSeconds 1-3600; split a longer run into several checks",
	"invalid-intent-scope":   "the plan intents array must list 1-16 repository-relative spec paths, each clean, sorted in byte order with no duplicates (the rule DOGFOOD_INTENTS_FILE follows for dogfood change); sort and de-duplicate it, for example with LC_ALL=C sort -u",
	"plan-bound-exceeded":    "the plan intents array must list 1-16 repository-relative spec paths, sorted in byte order with no duplicates, and the checks array 1-16 checks; an empty or longer array is refused",
}
