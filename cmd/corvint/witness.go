package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/Beamfall/corvint/internal/gokernel"
	"github.com/Beamfall/corvint/internal/witness"
)

type witnessOptions struct {
	base   string
	head   string
	cem    string
	stable string
	json   bool
}

// parseWitnessInvocation intercepts `witness` ahead of the shared argument
// parser so the report adds no verb to the parity-compared top-level command
// vocabulary.
func parseWitnessInvocation(arguments []string) (string, []string, bool, error) {
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
	if index >= len(arguments) || arguments[index] != "witness" {
		return "", nil, false, nil
	}
	resolved, err := resolveWitnessRoot(root)
	if err != nil {
		return "", nil, true, err
	}
	return resolved, arguments[index+1:], true, nil
}

func resolveWitnessRoot(root string) (string, error) {
	if root == "" {
		workingDirectory, err := os.Getwd()
		if err != nil {
			return "", argumentError("cannot resolve current directory")
		}
		return workingDirectory, nil
	}
	return resolveExplicitRoot(root)
}

// splitWitnessFlag separates `--name=value` from `--name value`. It reports
// whether the value was inline so a valueless flag can refuse one.
func splitWitnessFlag(argument string) (string, string, bool) {
	if position := strings.Index(argument, "="); position > 0 && strings.HasPrefix(argument, "--") {
		return argument[:position], argument[position+1:], true
	}
	return argument, "", false
}

// witnessOptionField names the field a valued option sets, or nil for an
// unknown name, so the name is checked before a value is consumed.
func witnessOptionField(result *witnessOptions, name string) *string {
	return map[string]*string{"--base": &result.base, "--head": &result.head, "--cem": &result.cem,
		"--stable-artifact-root": &result.stable}[name]
}

func parseWitnessOptions(arguments []string) (witnessOptions, error) {
	result := witnessOptions{}
	for index := 0; index < len(arguments); index++ {
		name, value, inline := splitWitnessFlag(arguments[index])
		if name == "--json" {
			if inline {
				return result, argumentError("--json takes no value")
			}
			result.json = true
			continue
		}
		field := witnessOptionField(&result, name)
		if field == nil {
			return result, argumentError("unrecognized witness option: " + name)
		}
		if !inline {
			if index+1 >= len(arguments) || argparseOptionLike(arguments[index+1]) {
				return result, argumentError("missing value for " + name)
			}
			index++
			value = arguments[index]
		}
		*field = value
	}
	if result.base == "" {
		return result, argumentError("witness requires --base")
	}
	if result.stable != "" {
		root, err := filepath.Abs(result.stable)
		if err != nil {
			return result, argumentError("cannot resolve --stable-artifact-root")
		}
		result.stable = root
	}
	return result, nil
}

// runWitness compiles and prints the unwitnessed surface of one committed
// range. It only reads: the index is the committed tree's snapshot or a build
// from the Git object store (IDX-SNAP-V0-020), and no file is written.
func runWitness(ctx context.Context, root string, arguments []string, stdout, stderr io.Writer) int {
	options, err := parseWitnessOptions(arguments)
	if err != nil {
		emitError(stderr, err)
		return 2
	}
	index, err := snapshotOrBuild(ctx, root)
	if err != nil {
		emitError(stderr, err)
		return 2
	}
	if options.head != "" && options.head != index.CommitRevision {
		emitError(stderr, argumentError(
			"--head must name the captured HEAD commit "+index.CommitRevision+"; witness reports a range ending at the checked-out revision"))
		return 2
	}
	report, err := witness.Compile(ctx, index, witness.Options{Base: options.base, CEMPath: options.cem, StableArtifacts: options.stable})
	if err != nil {
		emitError(stderr, &gokernel.Error{Code: "invalid-arguments", Message: err.Error()})
		return 2
	}
	if err := admitStableWitness(report, options.stable != ""); err != nil {
		emitError(stderr, err)
		return 2
	}
	if options.json {
		if err := emit(stdout, report); err != nil {
			emitError(stderr, err)
			return 2
		}
		return 0
	}
	if _, err := fmt.Fprint(stdout, witness.Render(report)); err != nil {
		emitError(stderr, &gokernel.Error{Code: "output-failed", Message: "cannot write witness report"})
		return 2
	}
	return 0
}

// admitStableWitness fails closed for the stable witness route (CEM-V1-160,
// CEM-V1-161). A cem/1.0 source is printed only when the stable verifier
// accepted it; any other stable outcome, including a missing artifact root,
// refuses with no stdout. --stable-artifact-root over a map that is not
// cem/1.0 refuses rather than relabel a 0.x report as stable.
func admitStableWitness(report *witness.Report, requested bool) error {
	source := report.Sources[0]
	if source.Stable == nil {
		if requested {
			return &gokernel.Error{Code: "stable-witness-refused", Message: "--stable-artifact-root requires a " +
				"cem/1.0 map committed at " + source.Path + "; the source is " + source.Status + ", so no stable report is produced"}
		}
		return nil
	}
	if source.Status != "BOUND" || report.Profile != witness.StableProfile {
		return &gokernel.Error{Code: "stable-witness-refused", Message: "the cem/1.0 witness source is " + source.Status + ": " + source.Detail}
	}
	return nil
}
