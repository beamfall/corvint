package main

import (
	"context"
	"errors"
	"io"
	"os"
	"os/signal"
	"strings"
	"time"

	"github.com/Beamfall/corvint/internal/dogfoodflow"
)

// dogfoodFlowOptions names the options each daily-path subverb accepts
// (DCW-V0-020). The script wrappers pass the executables they built or
// resolved; the subverbs never build, resolve or version-check one.
var dogfoodFlowOptions = map[string]map[string]bool{
	"change": {"--corvint-bin": true},
	"check":  {"--base-verifier": true, "--tree-verifier": true, "--override-verifier": true},
	"seal":   {"--base-verifier": true, "--tree-verifier": true, "--override-verifier": true},
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
	if len(positional) != 1 {
		return nil, "", errors.New("dogfood-base-required")
	}
	return flags, positional[0], nil
}

// runDogfoodFlow runs `dogfood change|check|seal BASE` with this executable
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
	if action == "change" {
		return dogfoodflow.Change(ctx, dogfoodflow.ChangeOptions{
			Root: root, Base: base, Steps: runner("--corvint-bin"),
			Task: os.Getenv("DOGFOOD_TASK"), Verify: os.Getenv("DOGFOOD_VERIFY"), VerifyFile: os.Getenv("DOGFOOD_VERIFY_FILE"),
			Outcome: os.Getenv("DOGFOOD_OUTCOME"), IntentsFile: os.Getenv("DOGFOOD_INTENTS_FILE"),
			Citations: os.Getenv("DOGFOOD_CITATIONS"), OCMLinks: os.Getenv("DOGFOOD_OCM_LINKS"),
		}, stderr)
	}
	options := dogfoodflow.CheckOptions{
		Root: root, Base: base, BaseVerifier: runner("--base-verifier"), TreeVerifier: runner("--tree-verifier"),
		Exception: os.Getenv("DOGFOOD_EXCEPTION"),
	}
	if _, ok := flags["--override-verifier"]; ok {
		override := runner("--override-verifier")
		options.Override = &override
	}
	if action == "seal" {
		return dogfoodflow.Seal(ctx, options, stdout, stderr)
	}
	return dogfoodflow.Check(ctx, options, stdout, stderr)
}
