package main

import (
	"context"
	"errors"
	"io"
	"os"
	"strings"

	"github.com/Beamfall/corvint/internal/cem/cli"
	"github.com/Beamfall/corvint/internal/localcompletion"
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
			return "", nil, true, argumentError("invalid local completion root")
		}
		root = resolved
	}
	return root, arguments[index+1:], true, nil
}

func runLocalCompletion(ctx context.Context, root string, args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	if len(args) > 0 && args[0] == "event" {
		return runLocalCompletionEvent(ctx, root, args[1:], stdin, stdout, stderr)
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
		result, err = localcompletion.Finish(ctx, root, key, localCompletionPublicCommand)
	case "review":
		result, err = localcompletion.Review(ctx, root, key, flags["--report-set"])
	case "cancel":
		result, err = localcompletion.Cancel(ctx, root, key)
	}
	if err != nil {
		if result.Lifecycle != "" {
			_ = emit(stdout, map[string]any{"ok": false, "profile": "corvint-local-completion/0", "tool": "dogfood-" + args[0], "policy": result})
		}
		return emitLocalCompletionFailure(stderr, err.Error())
	}
	payload := map[string]any{"ok": true, "profile": "corvint-local-completion/0", "tool": "dogfood-" + args[0], "mutates": args[0] != "status", "policy": result, "claim": "caller-owned-selected-workflow-only"}
	if err = emit(stdout, payload); err != nil {
		return emitLocalCompletionFailure(stderr, "output-failed")
	}
	if args[0] == "finish" && !result.Satisfied {
		return 1
	}
	if args[0] == "verify" {
		for _, check := range result.Checks {
			if check.ID == flags["--check"] && !check.Qualified {
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
	case "status", "finish", "cancel":
	case "handoff":
		allowed["--anchors"], allowed["--receipt"] = true, true
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
	return flags, nil
}

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
	return 2
}

func emitLocalCompletionFailure(stderr io.Writer, code string) int {
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
	_ = emit(stderr, map[string]any{"ok": false, "error": map[string]string{"code": code, "message": code}})
	return 2
}
