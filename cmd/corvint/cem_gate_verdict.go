package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	cemcli "github.com/Beamfall/corvint/internal/cem/cli"
	"github.com/Beamfall/corvint/internal/cem/wire"
	"github.com/Beamfall/corvint/internal/cemcandidate"
	"github.com/Beamfall/corvint/internal/taskman"
)

// cemGateVerdictInvalidArguments is the one stderr code for an argv the
// command cannot read; it prints no document (TCB-V0-016).
const cemGateVerdictInvalidArguments = "cem-gate-verdict-invalid-arguments"

// cemGateVerdictHelp closes cemHelp. It is kept out of the help.go literal so
// adding the command moves no help.go line another document cites by number.
const cemGateVerdictHelp = `
Tasks gate:
  corvint [--root PATH] cem gate-verdict --base OID --candidate OID
    --predicate touched-anchored|all-anchored --touch-paths JSON
    --portable-consumer PATH

  Read the one sealed map --candidate adds under .corvint/changes/ from Git
  objects, recompute its verdict with the stable verifier and the absolute
  --portable-consumer (a cem01-go build), and print one
  taskman-cem-verdict-gate/0 document with both verdicts and each governed
  path's minimum tier. Exit 0 grants, 1 refuses (verifier-disagreement when
  the consumers differ), 2 is an unreadable argv. --base and --candidate are
  full commit IDs; --touch-paths is a JSON string array.

  When the consumers differ and their recorded Go builds show different
  revisions or a modified tree, the refusal is verifier-build-mismatch: the
  consumer is not known to share the verifier's source (TCB-V0-045). Rebuild
  it from the verifier's checkout and rerun the gate:
    (cd interop/cem01-go && GOTOOLCHAIN=local go build -o "$CONSUMER" .)
`

// runCEMInvocation runs the cem commands that own their whole argv before the
// general router: the stable verifier and the Tasks verdict gate.
func runCEMInvocation(ctx context.Context, args []string, stdout, stderr io.Writer) (int, bool) {
	// A help request for either action prints that action's help before the
	// closed argv parsers would refuse it (V1-2109).
	if topic, requested, err := parseHelpInvocation(args); requested && err == nil &&
		(topic == cemActionHelpPrefix+"verify-stable" || topic == cemActionHelpPrefix+"gate-verdict" ||
			topic == cemActionHelpPrefix+"bundle") {
		if _, err := io.WriteString(stdout, helpText(topic)); err != nil {
			return 1, true
		}
		return 0, true
	}
	if code, handled := cemcli.RunStableInvocation(ctx, args, stdout, stderr); handled {
		return code, true
	}
	// The out-of-tree bundle command owns its closed argv (CSB-V0-001).
	if code, handled := cemcandidate.RunBundleInvocation(ctx, args, stdout, stderr); handled {
		return code, true
	}
	return runCEMGateVerdictInvocation(ctx, args, stdout, stderr)
}

// runCEMGateVerdictInvocation is the Tasks COMMAND gate over a committed
// cem/1.0 sealed map (TCB-V0-016):
//
//	corvint [--root PATH] cem gate-verdict --base OID --candidate OID
//	  --predicate touched-anchored|all-anchored --touch-paths JSON
//	  --portable-consumer ABS
//
// Every flag is closed and required once. It prints only the canonical
// taskman-cem-verdict-gate/0 document and exits 0 on GRANT and 1 on REFUSE; an
// unreadable argv prints one stderr line and exits 2. The root is --root or the
// working directory, which the Tasks runner sets to the attempt worktree.
func runCEMGateVerdictInvocation(ctx context.Context, args []string, stdout, stderr io.Writer) (int, bool) {
	at := -1
	for i := 0; i+1 < len(args); i++ {
		if args[i] == "cem" && args[i+1] == "gate-verdict" {
			at = i
			break
		}
	}
	if at < 0 {
		return 0, false
	}
	// As for cem verify-stable, a bare word first means another command owns the
	// arguments and merely carries these two values.
	for lead := 0; lead < at; {
		switch {
		case args[lead] == "--root" && lead+1 < at:
			lead += 2
		case strings.HasPrefix(args[lead], "--"):
			lead++
		default:
			return 0, false
		}
	}
	invalid := func() (int, bool) {
		fmt.Fprintln(stderr, "corvint: cem gate-verdict: "+cemGateVerdictInvalidArguments)
		return 2, true
	}
	root := ""
	switch {
	case at == 2 && args[0] == "--root" && filepath.IsAbs(args[1]) && filepath.Clean(args[1]) == args[1]:
		root = args[1]
	case at != 0:
		return invalid()
	default:
		// The working directory resolves to its real path: the stable consumers
		// refuse an aliased repository, and $PWD may name one.
		wd, err := os.Getwd()
		if err == nil {
			wd, err = filepath.EvalSymlinks(wd)
		}
		if err != nil || !filepath.IsAbs(wd) {
			return invalid()
		}
		root = filepath.Clean(wd)
	}
	values := map[string]string{}
	for i := at + 2; i < len(args); i += 2 {
		key := args[i]
		switch key {
		case "--base", "--candidate", "--predicate", "--touch-paths", "--portable-consumer":
		default:
			if message, paired := cemcli.PairedChangeRefusal([]string{key}); paired {
				fmt.Fprintln(stderr, "corvint: cem gate-verdict: "+cemGateVerdictInvalidArguments+": "+message)
				return 2, true
			}
			return invalid()
		}
		if _, seen := values[key]; seen || i+1 >= len(args) || strings.HasPrefix(args[i+1], "--") {
			return invalid()
		}
		values[key] = args[i+1]
	}
	if len(values) != 5 {
		return invalid()
	}
	touch, ok := touchPathsArgument(values["--touch-paths"])
	if !ok {
		return invalid()
	}
	result, err := taskman.EvaluateCEMVerdictGate(ctx, taskman.CEMVerdictInput{
		Root:             root,
		BaseCommit:       values["--base"],
		CandidateCommit:  values["--candidate"],
		TouchPaths:       touch,
		Predicate:        values["--predicate"],
		PortableConsumer: values["--portable-consumer"],
	})
	if err != nil {
		fmt.Fprintln(stderr, "corvint: cem gate-verdict: "+err.Error())
		return 2, true
	}
	if _, err := stdout.Write(result.Document); err != nil {
		return 2, true
	}
	if result.Granted {
		return 0, true
	}
	return 1, true
}

// touchPathsArgument reads the runner's {claim.touchPaths} value: one JSON
// array of strings. The gate library owns path validity.
func touchPathsArgument(raw string) ([]string, bool) {
	value, err := wire.Parse([]byte(raw))
	if err != nil || value.Kind != wire.KindArray {
		return nil, false
	}
	paths := make([]string, 0, len(value.Arr))
	for _, entry := range value.Arr {
		if entry.Kind != wire.KindString {
			return nil, false
		}
		paths = append(paths, entry.Str)
	}
	return paths, true
}
