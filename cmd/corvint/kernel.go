package main

import (
	"context"
	"fmt"
	"io"
	"strings"

	"github.com/Beamfall/corvint/internal/compactionkernel"
	"github.com/Beamfall/corvint/internal/contextindex"
	"github.com/Beamfall/corvint/internal/gokernel"
)

// kernelGovernanceLimit and kernelGovernanceBudget are the bounded prompt-packet
// limits the kernel verb reuses purely to obtain the governance array; the
// packet's other members are discarded.
const (
	kernelGovernanceLimit  = 10
	kernelGovernanceBudget = 4_000
)

type kernelOptions struct {
	verify       bool
	requirements []string
}

// parseKernelInvocation intercepts `kernel` ahead of the shared argument
// parser, the way parseObservationsInvocation does, so the leading --root forms
// resolve before the verb word is matched. `kernel` is a listed top-level
// command, so it does appear in the vocabulary the invalid-choice error names.
func parseKernelInvocation(arguments []string) (string, []string, bool, error) {
	index, root := 0, ""
	for index < len(arguments) && (arguments[index] == "--root" || strings.HasPrefix(arguments[index], "--root=")) {
		if arguments[index] == "--root" {
			if !rootPreambleValue(arguments, index+1) {
				return "", nil, false, nil
			}
			root, index = arguments[index+1], index+2
			continue
		}
		root, index = strings.TrimPrefix(arguments[index], "--root="), index+1
	}
	if index >= len(arguments) || arguments[index] != "kernel" {
		return "", nil, false, nil
	}
	resolved, err := resolveKernelRoot(root)
	if err != nil {
		return "", nil, true, err
	}
	return resolved, arguments[index+1:], true, nil
}

func resolveKernelRoot(root string) (string, error) {
	if root == "" {
		return normalizeRoot(".")
	}
	return resolveExplicitRoot(root)
}

func parseKernelOptions(arguments []string) (kernelOptions, error) {
	options := kernelOptions{}
	start := 0
	if len(arguments) != 0 && arguments[0] == "verify" {
		options.verify, start = true, 1
	}
	for index := start; index < len(arguments); index++ {
		name, value, inline := strings.Cut(arguments[index], "=")
		if name != "--requirements" {
			return options, argumentError("unrecognized arguments: " + arguments[index])
		}
		if options.verify {
			return options, argumentError("kernel verify takes no --requirements")
		}
		if !inline {
			if index+1 >= len(arguments) || argparseOptionLike(arguments[index+1]) {
				return options, argumentError("argument --requirements: expected one argument")
			}
			value, index = arguments[index+1], index+1
		}
		for _, id := range strings.Split(value, ",") {
			trimmed := strings.TrimSpace(id)
			if trimmed == "" {
				return options, argumentError("invalid --requirements")
			}
			options.requirements = append(options.requirements, trimmed)
		}
	}
	return options, nil
}

// runKernel prints the compaction kernel, or verifies one recovered from stdin.
// Both paths are read-only.
func runKernel(ctx context.Context, root string, arguments []string, stdin io.Reader, stdout, stderr io.Writer) int {
	options, err := parseKernelOptions(arguments)
	if err != nil {
		emitError(stderr, err)
		return 2
	}
	index, err := snapshotOrBuild(ctx, root)
	if err != nil {
		emitError(stderr, err)
		return 2
	}
	if options.verify {
		return runKernelVerify(index, stdin, stdout, stderr)
	}
	return runKernelRender(ctx, index, options.requirements, stdout, stderr)
}

func runKernelRender(ctx context.Context, index *contextindex.Index, requirements []string, stdout, stderr io.Writer) int {
	governance, err := kernelGovernance(ctx, index)
	if err != nil {
		emitError(stderr, err)
		return 2
	}
	kernel, err := compactionkernel.Build(index, governance, requirements)
	if err != nil {
		emitError(stderr, err)
		return 2
	}
	if _, err := fmt.Fprint(stdout, compactionkernel.Render(kernel)); err != nil {
		emitError(stderr, &gokernel.Error{Code: "output-failed", Message: "cannot write compaction kernel"})
		return 2
	}
	return 0
}

func runKernelVerify(index *contextindex.Index, stdin io.Reader, stdout, stderr io.Writer) int {
	recovered, err := io.ReadAll(io.LimitReader(stdin, gokernel.MaxInputBytes+1))
	if err != nil {
		emitError(stderr, &gokernel.Error{Code: "invalid-kernel-input", Message: "cannot read kernel text from stdin"})
		return 2
	}
	if len(recovered) > gokernel.MaxInputBytes {
		emitError(stderr, &gokernel.Error{Code: "invalid-kernel-input", Message: "kernel input exceeds its byte limit"})
		return 2
	}
	verdict, err := compactionkernel.Verify(string(recovered), index)
	if err != nil {
		emitError(stderr, err)
		return 2
	}
	if err := emit(stdout, verdict); err != nil {
		emitError(stderr, &gokernel.Error{Code: "output-failed", Message: "cannot write kernel verdict"})
		return 2
	}
	if verdict.State != "intact" {
		return 1
	}
	return 0
}

// kernelGovernance reuses the prompt compiler's own authority selection rather
// than re-deriving precedence here, so the kernel pins exactly the authority
// the user-prompt packet names.
func kernelGovernance(ctx context.Context, index *contextindex.Index) ([]compactionkernel.Entry, error) {
	packet, err := contextindex.DogfoodPromptContext(ctx, index, "", nil, kernelGovernanceLimit, kernelGovernanceBudget)
	if err != nil {
		return nil, err
	}
	return compactionkernel.GovernanceFromPacket(packet), nil
}

const kernelHelp = `
Compaction kernel (experimental, CKN-V0 proposed), read-only, JSON/text:

  corvint [--root PATH] kernel [--requirements IDS]
  corvint [--root PATH] kernel verify < SUMMARY

Print the bounded envelope that pins the governing authority and the named
requirements to committed blobs, or recover one from arbitrary text on stdin
and report whether it is intact, stale, missing, or corrupt.

  --requirements IDS  Comma-separated requirement ids to pin.

Both paths are read-only. Verify exits 1 when the recovered verdict is
not intact; that exit status is a verdict, not a usage failure.
`
