package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/Beamfall/corvint/internal/contextindex"
	"github.com/Beamfall/corvint/internal/gokernel"
	"github.com/Beamfall/corvint/internal/witness"
)

type witnessOptions struct {
	base   string
	head   string
	cem    string
	stable string
	json   bool
	check  bool
	// checkMode is "" for the valueless --check, else admitted or candidate
	// (AGW-V0-034).
	checkMode string
	// evidence holds each --evidence document as a clean absolute path, in
	// the order given (AGW-V0-008).
	evidence []string
	// repositoryEvidence holds each --evidence-in-repo document as given,
	// relative to the checkout root; witness admits it (AGW-V0-011).
	repositoryEvidence []string
	// ranges holds each --range NAME=BASE..HEAD by name and rangeRoots each
	// --range-root NAME=PATH; a paired report needs both app and tests
	// (AGW-V0-013).
	ranges     map[string]witnessRange
	rangeRoots map[string]string
	// pairedFrom is the --paired-from tests commit whose Paired-Range
	// trailer names the application range (AGW-V0-036).
	pairedFrom string
}

// witnessRange is one named side of a paired report, as the caller wrote it.
type witnessRange struct {
	base, head string
}

// parseWitnessRange splits NAME=BASE..HEAD or, with paths, NAME=PATH, and
// refuses an unknown or repeated name.
func parseWitnessRange(flag, value string, seen map[string]bool) (string, string, error) {
	name, rest, ok := strings.Cut(value, "=")
	if !ok || rest == "" {
		return "", "", argumentError(flag + " takes NAME=VALUE: " + value)
	}
	if name != witness.AppRange && name != witness.TestsRange {
		return "", "", argumentError(flag + " names " + name + "; the names are " + witness.AppRange + " and " + witness.TestsRange)
	}
	if seen[name] {
		return "", "", argumentError(flag + " " + name + " is given twice")
	}
	return name, rest, nil
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
	// Paired ranges name each repository, so only a single-range witness refuses a discovered
	// workspace (WSD-V0-146).
	rest := arguments[index+1:]
	paired := slices.ContainsFunc(rest, func(a string) bool {
		return a == "--range" || strings.HasPrefix(a, "--range=") || a == "--paired-from" || strings.HasPrefix(a, "--paired-from=")
	})
	rest, err = guardWorkspace("witness", resolved, rest, !paired)
	return resolved, rest, true, err
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
		"--stable-artifact-root": &result.stable, "--paired-from": &result.pairedFrom}[name]
}

func parseWitnessOptions(arguments []string) (witnessOptions, error) {
	result := witnessOptions{}
	for index := 0; index < len(arguments); index++ {
		name, value, inline := splitWitnessFlag(arguments[index])
		if name == "--check" && inline {
			if value != witnessCheckAdmitted && value != witnessCheckCandidate {
				return result, argumentError("--check takes no value, or =" + witnessCheckAdmitted + " or =" + witnessCheckCandidate + ": " + value)
			}
			if result.check && result.checkMode != value {
				return result, argumentError("--check is given with two modes")
			}
			result.check, result.checkMode = true, value
			continue
		}
		if name == "--json" || name == "--check" {
			if inline {
				return result, argumentError(name + " takes no value")
			}
			if name == "--check" && result.checkMode != "" {
				return result, argumentError("--check is given with two modes")
			}
			result.json = result.json || name == "--json"
			result.check = result.check || name == "--check"
			continue
		}
		field := witnessOptionField(&result, name)
		if field == nil && name != "--evidence" && name != "--evidence-in-repo" && name != "--range" && name != "--range-root" {
			return result, argumentError("unrecognized witness option: " + name)
		}
		if !inline {
			if index+1 >= len(arguments) || argparseOptionLike(arguments[index+1]) {
				return result, argumentError("missing value for " + name)
			}
			index++
			value = arguments[index]
		}
		if name == "--paired-from" && value == "" {
			return result, argumentError("--paired-from takes the tests commit that carries the " + witness.PairingTrailerKey + " trailer")
		}
		if name == "--range" || name == "--range-root" {
			if err := result.addRange(name, value); err != nil {
				return result, err
			}
			continue
		}
		if field == nil {
			if len(result.evidence)+len(result.repositoryEvidence) == witness.MaxEvidenceFiles {
				return result, argumentError(fmt.Sprintf("--evidence and --evidence-in-repo are repeatable at most %d times together", witness.MaxEvidenceFiles))
			}
			if name == "--evidence-in-repo" {
				result.repositoryEvidence = append(result.repositoryEvidence, value)
				continue
			}
			file, err := filepath.Abs(value)
			if value == "" || err != nil {
				return result, argumentError("cannot resolve --evidence " + value)
			}
			result.evidence = append(result.evidence, file)
			continue
		}
		*field = value
	}
	if err := result.checkRanges(); err != nil {
		return result, err
	}
	if result.base == "" && result.ranges == nil && result.pairedFrom == "" {
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

// addRange records one --range or --range-root value (AGW-V0-013).
func (result *witnessOptions) addRange(flag, value string) error {
	seen := map[string]bool{}
	if flag == "--range-root" {
		for name := range result.rangeRoots {
			seen[name] = true
		}
	} else {
		for name := range result.ranges {
			seen[name] = true
		}
	}
	name, spec, err := parseWitnessRange(flag, value, seen)
	if err != nil {
		return err
	}
	if flag == "--range-root" {
		if result.rangeRoots == nil {
			result.rangeRoots = map[string]string{}
		}
		result.rangeRoots[name] = spec
		return nil
	}
	if result.ranges == nil {
		result.ranges = map[string]witnessRange{}
	}
	base, head, ok := strings.Cut(spec, "..")
	if !ok || base == "" || head == "" || strings.HasPrefix(head, ".") || strings.Contains(head, "..") {
		return argumentError("--range " + name + " takes BASE..HEAD with both commits named: " + spec)
	}
	result.ranges[name] = witnessRange{base: base, head: head}
	return nil
}

// checkRanges refuses an incomplete pair and every single-range option a
// paired report cannot honour (AGW-V0-013).
func (result *witnessOptions) checkRanges() error {
	if result.pairedFrom != "" {
		return result.checkPairedFrom()
	}
	if result.ranges == nil {
		if result.rangeRoots != nil {
			return argumentError("--range-root requires --range " + witness.AppRange + "=BASE..HEAD and --range " + witness.TestsRange + "=BASE..HEAD")
		}
		return nil
	}
	switch {
	case len(result.ranges) != 2:
		return argumentError("a paired witness requires both --range " + witness.AppRange + "=BASE..HEAD and --range " + witness.TestsRange + "=BASE..HEAD")
	case result.base != "" || result.head != "":
		return argumentError("--range replaces --base and --head; give each range its own BASE..HEAD")
	case result.cem != "" || result.stable != "":
		return argumentError("--cem and --stable-artifact-root name one repository; a paired witness reads each range's committed Change Evidence Map")
	}
	return nil
}

// checkPairedFrom refuses every option --paired-from cannot honour. The
// explicit application checkout is required once the trailer is read, so the
// refusal can name the trailer's repository (AGW-V0-036, AGW-V0-037).
func (result *witnessOptions) checkPairedFrom() error {
	switch {
	case result.ranges != nil:
		return argumentError("--paired-from reads both ranges from the commit's " + witness.PairingTrailerKey + " trailer; it replaces --range")
	case result.base != "" || result.head != "":
		return argumentError("--paired-from replaces --base and --head; the trailer and its commit name both ranges")
	case result.cem != "" || result.stable != "":
		return argumentError("--cem and --stable-artifact-root name one repository; a paired witness reads each range's committed Change Evidence Map")
	}
	return nil
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
	if options.ranges != nil || options.pairedFrom != "" {
		return runPairedWitness(ctx, root, options, stdout, stderr)
	}
	// --base and --head accept any revision Git resolves; the report carries
	// the full commit object ID (AGW-V0-005).
	if options.base, err = witness.ResolveCommit(ctx, root, options.base); err != nil {
		emitError(stderr, argumentError("--base: "+err.Error()))
		return 2
	}
	head := ""
	if options.head != "" {
		if head, err = witness.ResolveCommit(ctx, root, options.head); err != nil {
			emitError(stderr, argumentError("--head: "+err.Error()))
			return 2
		}
	}
	index, pinned, err := witnessIndex(ctx, root, head, "--head")
	if err != nil {
		emitError(stderr, err)
		return 2
	}
	report, err := witness.Compile(ctx, index, witness.Options{Base: options.base, CEMPath: options.cem, StableArtifacts: options.stable, HeadPinned: pinned, Evidence: options.evidence, RepositoryEvidence: options.repositoryEvidence})
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
		return witnessExit(report, options, stderr)
	}
	if _, err := fmt.Fprint(stdout, witness.Render(report)); err != nil {
		emitError(stderr, &gokernel.Error{Code: "output-failed", Message: "cannot write witness report"})
		return 2
	}
	return witnessExit(report, options, stderr)
}

// witnessIndex builds the index a report reads at head. A head other than the
// checked-out HEAD is a historical head: the index is built from the object
// store at that commit and admission skips the worktree checks, binding the
// range COMMIT-PINNED (AGW-V0-007). A head equal to HEAD, or none, keeps the
// snapshot path and its bytes. label names the caller's head option.
func witnessIndex(ctx context.Context, root, head, label string) (*contextindex.Index, bool, error) {
	pinned := false
	if head != "" {
		current, err := witness.ResolveCommit(ctx, root, "HEAD")
		pinned = err != nil || current != head
	}
	var index *contextindex.Index
	var err error
	if pinned {
		index, err = contextindex.BuildRevisionContext(ctx, root, head)
	} else {
		index, err = snapshotOrBuild(ctx, root)
	}
	if err != nil {
		return nil, false, err
	}
	if head != "" && head != index.CommitRevision {
		return nil, false, argumentError(label + " resolves to " + head + " but the index was built at " + index.CommitRevision)
	}
	return index, pinned, nil
}

// runPairedWitness compiles the app and tests ranges as one logical change
// (AGW-V0-013). Each range resolves in its own root, --root unless
// --range-root names another checkout; a range whose root, base or head does
// not resolve refuses the whole report with exit 2, naming the range.
func runPairedWitness(ctx context.Context, root string, options witnessOptions, stdout, stderr io.Writer) int {
	roots := map[string]string{}
	for _, name := range []string{witness.AppRange, witness.TestsRange} {
		roots[name] = root
		if value, ok := options.rangeRoots[name]; ok {
			resolved, err := resolveExplicitRoot(value)
			if err != nil {
				emitError(stderr, argumentError("--range-root "+name+": "+err.Error()))
				return 2
			}
			roots[name] = resolved
		}
	}
	var pairing *witness.Pairing
	if options.pairedFrom != "" {
		_, mapped := options.rangeRoots[witness.AppRange]
		trailer, err := pairedRanges(ctx, roots, options.pairedFrom, mapped)
		if err != nil {
			emitError(stderr, argumentError("--paired-from: "+err.Error()))
			return 2
		}
		options.ranges = map[string]witnessRange{
			witness.AppRange:   {base: trailer.Base, head: trailer.Head},
			witness.TestsRange: {base: trailer.Parent, head: trailer.Commit},
		}
		pairing = &trailer.Pairing
	}
	inputs := map[string]witness.PairedInput{}
	for _, name := range []string{witness.AppRange, witness.TestsRange} {
		label := "--range " + name
		rangeRoot := roots[name]
		spec := options.ranges[name]
		base, err := witness.ResolveCommit(ctx, rangeRoot, spec.base)
		if err != nil {
			emitError(stderr, argumentError(label+": base: "+err.Error()))
			return 2
		}
		head, err := witness.ResolveCommit(ctx, rangeRoot, spec.head)
		if err != nil {
			emitError(stderr, argumentError(label+": head: "+err.Error()))
			return 2
		}
		index, pinned, err := witnessIndex(ctx, rangeRoot, head, label+" head")
		if err != nil {
			emitError(stderr, argumentError(label+": "+err.Error()))
			return 2
		}
		input := witness.PairedInput{Root: rangeRoot, Index: index, Options: witness.Options{Base: base, HeadPinned: pinned}}
		if name == witness.TestsRange {
			input.Options.Evidence, input.Options.RepositoryEvidence = options.evidence, options.repositoryEvidence
		}
		inputs[name] = input
	}
	report, err := witness.CompilePaired(ctx, inputs[witness.AppRange], inputs[witness.TestsRange])
	if err != nil {
		emitError(stderr, &gokernel.Error{Code: "invalid-arguments", Message: err.Error()})
		return 2
	}
	report.Pairing = pairing
	if options.json {
		err = emit(stdout, report)
	} else {
		_, err = fmt.Fprint(stdout, witness.RenderPaired(report))
	}
	if err != nil {
		emitError(stderr, &gokernel.Error{Code: "output-failed", Message: "cannot write witness report"})
		return 2
	}
	if options.check {
		for _, item := range report.Ranges {
			if !witnessAnalysed(item.Report.Range, item.Report.Summary, options.checkMode) {
				witnessAdmissionNote(stderr, item.Report.Range)
				return 1
			}
		}
		if witnessGateFails(report.Summary, options.checkMode) {
			return 1
		}
	}
	return 0
}

// pairedRanges reads the --paired-from commit's trailer in the tests root and
// checks its application range in the caller's application root
// (AGW-V0-036): the tests range is the commit's parent to the commit. The
// trailer names a repository identity, never a path, so without an explicit
// --range-root app (mapped false) it refuses naming that repository and the
// trailer instead of guessing a checkout (AGW-V0-037).
func pairedRanges(ctx context.Context, roots map[string]string, revision string, mapped bool) (*witness.TrailerPairing, error) {
	trailer, err := witness.ReadPairingTrailer(ctx, roots[witness.TestsRange], revision)
	if err != nil {
		return nil, err
	}
	if !mapped {
		return nil, fmt.Errorf("commit %s: %s: %s names repository %s, which has no checkout here; pass --range-root %s=PATH, the checkout of that repository; witness never guesses a checkout", trailer.Commit, witness.PairingTrailerKey, trailer.Trailer, trailer.Repository, witness.AppRange)
	}
	if err := witness.CheckPairedApplication(ctx, roots[witness.AppRange], trailer); err != nil {
		return nil, fmt.Errorf("%s %s (repository %s, --range-root %s): %w", witness.PairingTrailerKey, witness.AppRange, trailer.Repository, witness.AppRange, err)
	}
	return trailer, nil
}

// The --check modes that gate on analysis instead of closure (AGW-V0-034).
const (
	witnessCheckAdmitted  = "admitted"
	witnessCheckCandidate = "candidate"
)

// witnessExit is 0 without --check. With the valueless --check it is 1 when
// the report is not a complete witness of the range: the admission engine did
// not admit the whole range, or any opened obligation is not closed
// (AGW-V0-006). In V0 no authority closes, so a range with any obligation
// exits 1. --check=admitted and --check=candidate gate on analysis and
// candidates instead, and never claim closure (AGW-V0-034).
func witnessExit(report *witness.Report, options witnessOptions, stderr io.Writer) int {
	if options.check && (!witnessAnalysed(report.Range, report.Summary, options.checkMode) || witnessGateFails(report.Summary, options.checkMode)) {
		witnessAdmissionNote(stderr, report.Range)
		return 1
	}
	return 0
}

// witnessAdmissionNote names why a failing gate's admission is not COMPUTED,
// on stderr only, so the report bytes stay those of the run without --check
// (AGW-V0-038). A PARTIAL admission's detail names each bound it hit.
func witnessAdmissionNote(stderr io.Writer, binding witness.Range) {
	if binding.Admission == "COMPUTED" || binding.AdmissionDetail == "" {
		return
	}
	_, _ = fmt.Fprintf(stderr, "corvint witness: --check: admission is %s: %s\n", binding.Admission, binding.AdmissionDetail)
}

// witnessAnalysed reports whether one range passes the analysis half of a
// gate: admission is COMPUTED and, in the admitted and candidate modes, the
// closure is COMPUTED (or EMPTY, when admission admitted no path) and no
// obligation is NOT_RUN. A TRUNCATED closure hides dependents, so it fails.
func witnessAnalysed(binding witness.Range, summary witness.Summary, mode string) bool {
	if binding.Admission != "COMPUTED" {
		return false
	}
	return mode == "" || ((binding.Closure == "COMPUTED" || binding.Closure == "EMPTY") && summary.NotRun == 0)
}

// witnessGateFails reports whether a summary fails the obligation half of a
// gate: any open obligation for the valueless --check, any open obligation
// with no candidate witness for candidate, and nothing for admitted.
func witnessGateFails(summary witness.Summary, mode string) bool {
	switch mode {
	case witnessCheckAdmitted:
		return false
	case witnessCheckCandidate:
		return summary.Unwitnessed > 0
	}
	return summary.Closed < summary.Opened
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
