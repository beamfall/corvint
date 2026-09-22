package mutate

import (
	"context"
	"os/exec"
	"time"
)

// This file is the seam a sibling runner (internal/liveverify/pymutate) uses
// to judge claims on the same exported copy: the export directory, the
// sandbox prefix, the scratch reset, the bounded output, and the
// process-group kill are this package's and are reached only through these
// methods. Nothing here is used by the Go runner itself.

// Dir is the directory holding the exported revision. The runner that
// mutates a file there must restore it after every mutant.
func (exported *Export) Dir() string { return exported.space.export }

// Unsandboxed is non-empty when the host has no sandbox, and says why; a
// sibling runner reports Unsupported with it and runs nothing.
func (exported *Export) Unsandboxed() string { return exported.unsandboxed }

// Run executes argv inside the host sandbox from dir with exactly
// environment, after emptying the scratch directory, and kills the run's
// process group afterwards whatever happened. It reports the bounded
// combined output, whether the per-run timeout (not the caller's context)
// ended the run, and the command's error.
func (exported *Export) Run(ctx context.Context, timeout time.Duration, dir string, environment []string, argv ...string) (string, bool, error) {
	if err := resetScratch(exported.space); err != nil {
		return err.Error(), false, err
	}
	commandCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	confined := append(append([]string{}, exported.space.prefix...), argv...)
	command := exec.CommandContext(commandCtx, confined[0], confined[1:]...)
	command.Dir = dir
	command.Env = environment
	output := &boundedBuffer{limit: outputLimit}
	command.Stdout, command.Stderr = output, output
	configureProcess(command)
	err := command.Run()
	if command.Process != nil {
		terminateProcessGroup(command.Process.Pid)
	}
	timedOut := commandCtx.Err() != nil && ctx.Err() == nil
	return output.String(), timedOut, err
}

// Scratch is the one directory the sandbox lets a run write, emptied before
// every Run; a sibling runner points the interpreter's temporary files at it.
func (exported *Export) Scratch() string { return exported.space.scratch }

// PassthroughEnvironment copies the named host variables that are set.
func PassthroughEnvironment(names []string) []string { return passthroughEnvironment(names) }
