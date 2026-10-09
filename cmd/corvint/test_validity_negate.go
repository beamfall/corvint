package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strconv"
	"syscall"

	"github.com/Beamfall/corvint/internal/gokernel"
	"github.com/Beamfall/corvint/internal/stepnegation"
)

// negateRefusalSchema is the provider's closed refusal document.
const negateRefusalSchema = "corvint-step-negation-refusal/0"

// negateInvocation is the core's view of `test-validity negate`: the provider
// it execs, the canonical test identity it locks and retains, and the
// provider arguments it passes through unchanged.
type negateInvocation struct {
	provider string
	key      stepnegation.TestKey
	forward  []string
}

// parseNegateInvocation takes --provider out of arguments and reads the test
// identity flags without consuming them; every other argument belongs to the
// provider, which owns the remaining refusals (LPCV-V0-057, LPCV-V0-058).
func parseNegateInvocation(arguments []string) (negateInvocation, error) {
	invocation := negateInvocation{}
	identity := map[string]*string{"--spec": &invocation.key.File, "--test": &invocation.key.FullTitle, "--project": &invocation.key.Project}
	for index := 0; index < len(arguments); index++ {
		argument := arguments[index]
		name, value, inline := splitWitnessFlag(argument)
		target, isIdentity := identity[name]
		if name != "--provider" && !isIdentity {
			if name == "--root" || name == "--dir" {
				return negateInvocation{}, argumentError("negate takes the worktree from the core --root option, not " + name)
			}
			invocation.forward = append(invocation.forward, argument)
			continue
		}
		if !inline {
			if index+1 >= len(arguments) || argparseOptionLike(arguments[index+1]) {
				return negateInvocation{}, argumentError("missing value for " + name)
			}
			index++
			value = arguments[index]
		}
		if name == "--provider" {
			if invocation.provider != "" {
				return negateInvocation{}, argumentError("--provider may be given once")
			}
			invocation.provider = value
			continue
		}
		if *target != "" {
			return negateInvocation{}, argumentError(name + " may be given once")
		}
		*target = value
		invocation.forward = append(invocation.forward, name+"="+value)
	}
	// The retained identity carries the screened title (LPCV-V0-069).
	invocation.key.FullTitle = stepnegation.Screen(invocation.key.FullTitle)
	if invocation.provider == "" {
		return negateInvocation{}, argumentError("negate requires --provider ABSOLUTE_FILE")
	}
	// The provider is the named regular file only, never a PATH lookup
	// (LPCV-V0-042).
	if !filepath.IsAbs(invocation.provider) {
		return negateInvocation{}, argumentError("--provider must be an absolute path")
	}
	if info, err := os.Stat(invocation.provider); err != nil || !info.Mode().IsRegular() {
		return negateInvocation{}, argumentError("--provider must name a regular file")
	}
	return invocation, nil
}

// runTestValidityNegate is the only test-validity mode that executes tests or
// writes (LPCV-V0-057). It execs the provider, prints the run's canonical
// corvint-step-negation/0 document, and retains the merged document under
// .corvint/strength-evidence (LPCV-V0-067). A refusal exits 2 with no stdout.
func runTestValidityNegate(root string, arguments []string, stdout, stderr io.Writer) int {
	invocation, err := parseNegateInvocation(arguments)
	if err != nil {
		emitError(stderr, err)
		return 2
	}
	worktree, err := negateWorktree(root)
	if err != nil {
		emitError(stderr, err)
		return 2
	}
	// With the project named, the identity is known before any run, so a
	// concurrent writer refuses before the baseline.
	var release func()
	if invocation.key.Project != "" && invocation.key.File != "" && invocation.key.FullTitle != "" {
		if release, err = stepnegation.Lock(worktree, invocation.key); err != nil {
			emitError(stderr, negateLockError(err))
			return 2
		}
		defer release()
	}
	document, err := execNegateProvider(invocation, worktree, stderr)
	if err != nil {
		emitError(stderr, err)
		return 2
	}
	key := document.Binding.Test.Key()
	if release != nil && key != invocation.key {
		emitError(stderr, &gokernel.Error{Code: "negate-provider-failed", Message: "provider document names a different test identity"})
		return 2
	}
	retentionErr := error(nil)
	if release == nil {
		if release, retentionErr = stepnegation.Lock(worktree, key); retentionErr == nil {
			defer release()
		}
	}
	var previous *stepnegation.Document
	if retentionErr == nil {
		previous, retentionErr = stepnegation.Load(worktree, key)
	}
	retained := document
	if retentionErr == nil {
		retained = stepnegation.Merge(previous, cloneNegation(document))
		markPlanReuse(&document, retained)
	}
	data, err := stepnegation.Encode(document)
	if err != nil {
		emitError(stderr, &gokernel.Error{Code: "negate-provider-failed", Message: err.Error()})
		return 2
	}
	if _, err := stdout.Write(data); err != nil {
		emitError(stderr, err)
		return 1
	}
	writeNegateSummary(stderr, document)
	if retentionErr == nil {
		retentionErr = stepnegation.Retain(worktree, retained)
	}
	status := 0
	if retentionErr != nil {
		message := retentionErr.Error()
		if errors.Is(retentionErr, stepnegation.ErrBusy) {
			message = "negate-evidence-busy: another negate holds this test identity's strength evidence"
		}
		fmt.Fprintln(stderr, "retention failure:", message)
		status = 1
	}
	if document.CleanupIncomplete {
		fmt.Fprintln(stderr, "cleanup incomplete: a scratch directory or owned process group was not removed")
		status = 1
	}
	return status
}

// negateWorktree resolves the worktree root (default: the working directory).
func negateWorktree(root string) (string, error) {
	if root == "" {
		root = "."
	}
	absolute, err := filepath.Abs(root)
	if err == nil {
		absolute, err = filepath.EvalSymlinks(absolute)
	}
	if err != nil {
		return "", argumentError("worktree root cannot be resolved")
	}
	return absolute, nil
}

func negateLockError(err error) error {
	if errors.Is(err, stepnegation.ErrBusy) {
		return &gokernel.Error{Code: "negate-evidence-busy", Message: "another negate holds this test identity's strength evidence"}
	}
	return err
}

// execNegateProvider runs `PROVIDER negate --root WORKTREE ARGS...` in the
// worktree. Interrupt and termination are forwarded so the provider's own
// cancellation join (LPCV-V0-069) runs; the core waits for it to exit.
func execNegateProvider(invocation negateInvocation, worktree string, stderr io.Writer) (stepnegation.Document, error) {
	command := exec.Command(invocation.provider, append([]string{"negate", "--root=" + worktree}, invocation.forward...)...)
	command.Dir = worktree
	output := &negateOutput{limit: stepnegation.MaxDocumentBytes}
	command.Stdout, command.Stderr = output, stderr
	signals := make(chan os.Signal, 2)
	signal.Notify(signals, os.Interrupt, syscall.SIGTERM)
	defer signal.Stop(signals)
	if err := command.Start(); err != nil {
		return stepnegation.Document{}, &gokernel.Error{Code: "negate-provider-failed", Message: "provider cannot be started: " + err.Error()}
	}
	done := make(chan error, 1)
	go func() { done <- command.Wait() }()
	var waitErr error
	for waiting := true; waiting; {
		select {
		case waitErr = <-done:
			waiting = false
		case received := <-signals:
			_ = command.Process.Signal(received)
		}
	}
	if output.overrun {
		return stepnegation.Document{}, &gokernel.Error{Code: "negate-provider-failed", Message: "provider output exceeds 4 MiB"}
	}
	var exitErr *exec.ExitError
	if errors.As(waitErr, &exitErr) && exitErr.ExitCode() == 2 {
		return stepnegation.Document{}, decodeNegateRefusal(output.Bytes())
	}
	if waitErr != nil {
		return stepnegation.Document{}, &gokernel.Error{Code: "negate-provider-failed", Message: "provider failed: " + waitErr.Error()}
	}
	document, err := stepnegation.Decode(output.Bytes())
	if err != nil {
		return stepnegation.Document{}, &gokernel.Error{Code: "negate-provider-failed", Message: "provider document " + err.Error()}
	}
	return document, nil
}

// decodeNegateRefusal accepts exactly one closed refusal document and returns
// its typed code; anything else is a provider failure.
func decodeNegateRefusal(data []byte) error {
	var refusal struct {
		Schema  string `json:"schema"`
		Code    string `json:"code"`
		Message string `json:"message"`
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&refusal); err != nil || decoder.More() || refusal.Schema != negateRefusalSchema || refusal.Code == "" {
		return &gokernel.Error{Code: "negate-provider-failed", Message: "provider exited 2 without a refusal document"}
	}
	return &gokernel.Error{Code: refusal.Code, Message: refusal.Message}
}

// negateOutput is a bounded stdout capture.
type negateOutput struct {
	bytes.Buffer
	limit   int
	overrun bool
}

func (o *negateOutput) Write(data []byte) (int, error) {
	if o.Len()+len(data) > o.limit {
		o.overrun = true
		return len(data), nil
	}
	return o.Buffer.Write(data)
}

func cloneNegation(document stepnegation.Document) stepnegation.Document {
	document.Steps = append([]stepnegation.Step(nil), document.Steps...)
	return document
}

// markPlanReuse copies the merge's planReused flags onto the run's own
// entries (LPCV-V0-068).
func markPlanReuse(document *stepnegation.Document, merged stepnegation.Document) {
	reused := map[int]bool{}
	for _, step := range merged.Steps {
		if step.PlanReused {
			reused[step.Ordinal] = true
		}
	}
	for index, step := range document.Steps {
		if step.Ordinal != 0 && reused[step.Ordinal] {
			document.Steps[index].PlanReused = true
		}
	}
}

// writeNegateSummary renders the human summary on stderr; a step needing a
// hand-authored control reads UNPROVEN (LPCV-V0-065). The aggregate line is
// over this run's entries only and is diagnostic (LPCV-V0-070).
func writeNegateSummary(stderr io.Writer, document stepnegation.Document) {
	for _, step := range document.Steps {
		state := step.Strength.State + " (" + step.Strength.Reason + ")"
		if step.Requires == stepnegation.RequiresManualControl {
			state = "UNPROVEN (manual control needed)"
		}
		label := "unresolved"
		if step.Ordinal != 0 {
			label = strconv.Itoa(step.Ordinal)
		}
		fmt.Fprintf(stderr, "step %s %q: strength: %s\n", label, step.Title, state)
	}
	aggregate := stepnegation.Aggregate(document)
	fmt.Fprintf(stderr, "test %q: strength: %s (%s); diagnostic, not joined\n", document.Binding.Test.FullTitle, aggregate.State, aggregate.Reason)
}
