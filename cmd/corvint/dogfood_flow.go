package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/signal"
	"strings"
	"time"

	"github.com/Beamfall/corvint/internal/dogfoodflow"
	"github.com/Beamfall/corvint/internal/localcompletion"
)

// dogfoodFlowOptions names the options each daily-path subverb accepts
// (DCW-V0-020). The script wrappers pass the executables they built or
// resolved; the subverbs never build, resolve or version-check one.
var dogfoodFlowOptions = map[string]map[string]bool{
	"change": {"--corvint-bin": true},
	"check":  {"--base-verifier": true, "--tree-verifier": true, "--override-verifier": true},
	"seal":   {"--base-verifier": true, "--tree-verifier": true, "--override-verifier": true},
	// batch-check is read-only and verifies sealed constituents (DCW-V0-039).
	"batch-check": {"--verifier": true},
	// propose-citations is read-only and prints a draft plan (DCW-V0-150).
	"propose-citations": {},
	// finish-lane runs change, check and seal in turn, so it takes both
	// option sets plus the bind loop's inputs; BASE is --base (DCW-V0-060).
	"finish-lane": {"--base": true, "--citations": true, "--intents": true, "--verify": true, "--outcome": true,
		"--corvint-bin": true, "--base-verifier": true, "--tree-verifier": true, "--override-verifier": true},
	// inputs-check is finish-lane's input checks and prepare pass alone: it
	// binds, commits and seals nothing, so it takes no verifier (DCW-V0-380).
	"inputs-check": {"--base": true, "--citations": true, "--intents": true, "--verify": true, "--outcome": true,
		"--corvint-bin": true},
}

func dogfoodFlowArguments(action string, args []string) (map[string]string, string, error) {
	flags := map[string]string{}
	positional := []string{}
	for index := 0; index < len(args); index++ {
		if !strings.HasPrefix(args[index], "--") {
			positional = append(positional, args[index])
			continue
		}
		name, value, inline := strings.Cut(args[index], "=")
		if !dogfoodFlowOptions[action][name] {
			return nil, "", errors.New("invalid-local-completion-option")
		}
		if _, exists := flags[name]; exists {
			return nil, "", errors.New("duplicate-local-completion-option")
		}
		if !inline {
			index++
			if index >= len(args) || argparseOptionLike(args[index]) {
				return nil, "", errors.New("local-completion-option-value-required")
			}
			value = args[index]
		}
		if value == "" || len(value) > 4096 {
			return nil, "", errors.New("invalid-local-completion-option-value")
		}
		flags[name] = value
	}
	if action == "finish-lane" || action == "inputs-check" {
		if len(positional) != 0 || flags["--base"] == "" {
			return nil, "", errors.New("dogfood-base-required")
		}
		return flags, flags["--base"], nil
	}
	if len(positional) != 1 {
		return nil, "", errors.New("dogfood-base-required")
	}
	return flags, positional[0], nil
}

// runDogfoodFlow runs `dogfood change|check|seal|batch-check BASE` or
// `dogfood finish-lane|inputs-check --base BASE` with this executable
// unless an option names another one, reading the DOGFOOD_* inputs the
// make targets have always taken.
func runDogfoodFlow(ctx context.Context, root string, args []string, stdout, stderr io.Writer) int {
	flags, base, err := dogfoodFlowArguments(args[0], args[1:])
	if err != nil {
		return emitLocalCompletionFailure(stderr, err.Error())
	}
	self, err := os.Executable()
	if err != nil {
		return emitLocalCompletionFailure(stderr, "dogfood-executable-unavailable")
	}
	if args[0] != "propose-citations" {
		refusal, notes := dogfoodStaleBinary(ctx, root, self, flags)
		for _, note := range notes {
			_, _ = fmt.Fprintf(stderr, "dogfood-%s: NOTE %s\n", args[0], note)
		}
		if refusal != "" {
			_, _ = fmt.Fprintf(stderr, "dogfood-%s: REFUSE %s\n", args[0], refusal)
			return 2
		}
	}
	runner := func(name string) dogfoodflow.Runner {
		if path, ok := flags[name]; ok {
			return dogfoodflow.Exec(path)
		}
		return dogfoodflow.Exec(self)
	}
	codes := dogfoodSignalCodes()
	notified := make(chan os.Signal, 1)
	signals := make([]os.Signal, 0, len(codes))
	for received := range codes {
		signals = append(signals, received)
	}
	signal.Notify(notified, signals...)
	defer signal.Stop(notified)
	flowCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	received := make(chan os.Signal, 1)
	done := make(chan struct{})
	defer close(done)
	// The parent context may end first on the same signal; only the signal
	// itself chooses the exit status.
	go func() {
		select {
		case value := <-notified:
			received <- value
			cancel()
		case <-done:
		}
	}()
	code, err := runDogfoodAction(flowCtx, root, args[0], base, flags, runner, stdout, stderr)
	if !errors.Is(err, dogfoodflow.ErrInterrupted) {
		return code
	}
	// The former scripts' traps exited 128 plus the signal number.
	select {
	case value := <-received:
		return codes[value]
	case <-time.After(time.Second):
		return 2
	}
}

func runDogfoodAction(ctx context.Context, root, action, base string, flags map[string]string, runner func(string) dogfoodflow.Runner, stdout, stderr io.Writer) (int, error) {
	// finish-lane's input options take precedence over the DOGFOOD_* inputs.
	input := func(option, name string) string {
		if value, ok := flags[option]; ok {
			return value
		}
		return os.Getenv(name)
	}
	change := dogfoodflow.ChangeOptions{
		Root: root, Base: base, Steps: runner("--corvint-bin"),
		Task: os.Getenv("DOGFOOD_TASK"), Verify: os.Getenv("DOGFOOD_VERIFY"), VerifyFile: input("--verify", "DOGFOOD_VERIFY_FILE"),
		Outcome: input("--outcome", "DOGFOOD_OUTCOME"), IntentsFile: input("--intents", "DOGFOOD_INTENTS_FILE"),
		Citations: input("--citations", "DOGFOOD_CITATIONS"), OCMLinks: os.Getenv("DOGFOOD_OCM_LINKS"),
		CitationReplace:    os.Getenv("DOGFOOD_CITATION_REPLACE"),
		AggregateOverLimit: true,
	}
	if action == "change" {
		return dogfoodflow.Change(ctx, change, stderr)
	}
	if action == "propose-citations" {
		return dogfoodflow.ProposeCitations(ctx, dogfoodflow.ProposeOptions{Root: root, Base: base}, stdout, stderr)
	}
	if action == "inputs-check" {
		return dogfoodflow.FinishLane(ctx, dogfoodflow.FinishLaneOptions{Change: change, Preflight: true}, stdout, stderr)
	}
	if action == "batch-check" {
		return dogfoodflow.BatchCheck(ctx, dogfoodflow.BatchCheckOptions{Root: root, Base: base, Verifier: runner("--verifier")}, stdout, stderr)
	}
	options := dogfoodflow.CheckOptions{
		Root: root, Base: base, BaseVerifier: runner("--base-verifier"), TreeVerifier: runner("--tree-verifier"),
		Exception: os.Getenv("DOGFOOD_EXCEPTION"), Verbose: os.Getenv("DOGFOOD_VERBOSE") == "1", OutputChange: os.Getenv("DOGFOOD_OUTPUT_CHANGE"),
		ReusePass: os.Getenv("DOGFOOD_CHECK_REUSE") == "1",
	}
	if _, ok := flags["--override-verifier"]; ok {
		override := runner("--override-verifier")
		options.Override = &override
	}
	options = localcompletion.ConfigureAggregateCheck(options, false)
	if action == "finish-lane" {
		return dogfoodflow.FinishLane(ctx, dogfoodflow.FinishLaneOptions{Change: change, Check: options}, stdout, stderr)
	}
	if action == "seal" {
		return dogfoodflow.Seal(ctx, options, stdout, stderr)
	}
	return dogfoodflow.Check(ctx, options, stdout, stderr)
}

// dogfoodStaleBinary refuses a dogfood flow whose own binary, or a binary it
// would run for the change steps or as a verifier, is stale for root, so no
// change evidence is bound or checked by code that current source does not
// build (DCW-V0-396). A binary that could not be compared is a note, not a
// refusal. The base verifier is built from BASE on purpose and is not judged;
// the wrappers' builds carry no Git stamp and are never refused, so only an
// operator's CORVINT_BIN can be.
func dogfoodStaleBinary(ctx context.Context, root, self string, flags map[string]string) (string, []string) {
	var notes []string
	judge := func(label string, freshness binaryFreshness) string {
		switch freshness.Code {
		case selfBinaryStaleCode:
			return selfBinaryStaleCode + " " + label + ": " + freshness.Detail + "; or unset CORVINT_BIN to use the current-tree build"
		case selfBinaryUnverifiedCode:
			notes = append(notes, selfBinaryUnverifiedCode+" "+label+": "+freshness.Detail)
		}
		return ""
	}
	if refusal := judge(self, selfBinaryStale(ctx, root)); refusal != "" {
		return refusal, notes
	}
	for _, name := range []string{"--corvint-bin", "--tree-verifier", "--override-verifier", "--verifier"} {
		path, ok := flags[name]
		if !ok || path == self {
			continue
		}
		if refusal := judge(name+" "+path, executableStale(ctx, root, path)); refusal != "" {
			return refusal, notes
		}
	}
	return "", notes
}
