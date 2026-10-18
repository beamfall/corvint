package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"sync/atomic"
	"time"
)

// defaultInterruptGrace is the external runner's SIGINT grace on --timeout
// (PWP-V0-030): long enough for Playwright to write its interrupted report and
// stop its workers, short beside the default 5m run bound.
const defaultInterruptGrace = 10 * time.Second

// admitRunnerControl refuses the external-only runner flags in other modes
// and a negative grace (PWP-V0-028, PWP-V0-030).
func admitRunnerControl(fs *flag.FlagSet, externalServer bool, grace time.Duration) error {
	explicit := false
	fs.Visit(func(f *flag.Flag) {
		if f.Name == "runner-output" || f.Name == "interrupt-grace" {
			explicit = true
		}
	})
	if explicit && !externalServer {
		return errors.New("runner-control-requires-external-server")
	}
	if grace < 0 {
		return errors.New("interrupt-grace-invalid")
	}
	return nil
}

// runnerOutput is the --runner-output sink. After its first write error it
// accepts nothing more; the run is unaffected (PWP-V0-028).
type runnerOutput struct {
	w      io.Writer
	failed atomic.Bool
}

var errRunnerOutputStopped = errors.New("runner output stopped")

func (o *runnerOutput) Write(value []byte) (int, error) {
	if o.failed.Load() {
		return 0, errRunnerOutputStopped
	}
	n, err := o.w.Write(value)
	if err != nil {
		o.failed.Store(true)
	}
	return n, err
}

// openRunnerOutput opens path ("-" is stderr). The returned close reports
// runner-output-incomplete on stderr when a write failed.
func openRunnerOutput(path string, stderr io.Writer) (*runnerOutput, func(), error) {
	switch path {
	case "":
		return nil, func() {}, nil
	case "-":
		output := &runnerOutput{w: stderr}
		return output, func() { reportRunnerOutput(output, nil, stderr) }, nil
	}
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o600)
	if err != nil {
		return nil, nil, fmt.Errorf("runner-output-unavailable: %w", err)
	}
	output := &runnerOutput{w: file}
	return output, func() { reportRunnerOutput(output, file.Close(), stderr) }, nil
}

func reportRunnerOutput(output *runnerOutput, closeErr error, stderr io.Writer) {
	if output.failed.Load() || closeErr != nil {
		_, _ = fmt.Fprintln(stderr, "runner-output-incomplete: the runner console copy stopped early; the receipt is unaffected")
	}
}
