package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"slices"

	"github.com/Beamfall/corvint/internal/evalrepo"
	"github.com/Beamfall/corvint/internal/gokernel"
	"github.com/Beamfall/corvint/internal/slotlearn"
)

// runSlotWeightsInvocation is the explicit, operator-invoked learning step
// (learned-trace-admission-v0, LTA-V0-009..012): `eval --learn-slot-weights
// [--goldens FILE] [--admit]` and the rollback `eval --reset-slot-weights`.
// It is the only reader of the ledgers as labels; `context` never is.
func runSlotWeightsInvocation(ctx context.Context, root string, arguments []string, stdout, stderr io.Writer) (int, bool) {
	learn := slices.Contains(arguments, "--learn-slot-weights")
	reset := slices.Contains(arguments, "--reset-slot-weights")
	if !learn && !reset {
		return 0, false
	}
	if reset {
		return runSlotWeightsReset(root, arguments, stdout, stderr), true
	}
	rest := slices.DeleteFunc(slices.Clone(arguments), func(argument string) bool {
		return argument == "--learn-slot-weights" || argument == "--admit"
	})
	options, err := parseEvalFlags(root, rest)
	if err == nil && options.fixture != "" {
		err = argumentError("argument --trace-fixture: not allowed with --learn-slot-weights")
	}
	if err != nil {
		return emitSlotWeightsFlagError(stderr, err), true
	}
	result, admitted, err := slotlearn.Learn(ctx, root, options.golden, slices.Contains(arguments, "--admit"))
	if err != nil {
		emitEvalError(stderr, err)
		return 2, true
	}
	return writeSlotWeightsPayload(stdout, stderr, map[string]any{"ok": true, "mutates": admitted, "tool": "eval", "slot_weights": result}), true
}

func runSlotWeightsReset(root string, arguments []string, stdout, stderr io.Writer) int {
	if len(arguments) != 1 {
		return emitSlotWeightsFlagError(stderr, argumentError("argument --reset-slot-weights: takes no other arguments"))
	}
	removed, err := slotlearn.Reset(root)
	if err != nil {
		emitEvalError(stderr, err)
		return 2
	}
	return writeSlotWeightsPayload(stdout, stderr, map[string]any{"ok": true, "mutates": removed, "tool": "eval", "slot_weights": map[string]any{"reset": true, "removed": removed}})
}

func emitSlotWeightsFlagError(stderr io.Writer, err error) int {
	var argument *gokernel.Error
	if errors.As(err, &argument) {
		emitError(stderr, err)
	} else {
		emitEvalError(stderr, err)
	}
	return 2
}

func writeSlotWeightsPayload(stdout, stderr io.Writer, payload map[string]any) int {
	encoded, err := evalrepo.Encode(payload)
	if err != nil {
		emitEvalError(stderr, fmt.Errorf("cannot write eval output"))
		return 2
	}
	encoded = append(encoded, '\n')
	if written, err := stdout.Write(encoded); err != nil || written != len(encoded) {
		emitEvalError(stderr, fmt.Errorf("cannot write eval output"))
		return 2
	}
	return 0
}
