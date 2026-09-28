package opencodequalification

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/Beamfall/corvint/internal/procgroup"
)

func Probe(ctx context.Context, path string, args []string) error {
	cwd, e := os.Getwd()
	if e != nil {
		return e
	}
	config, e := readObject(path)
	if e != nil {
		return e
	}
	root := str(config["root"])
	if root == "" || str(config["binary"]) == "" {
		return errors.New("invalid capture probe configuration")
	}
	if truth(config["interrupt"]) {
		r := procgroup.Run(ctx, procgroup.Spec{Argv: []string{"/bin/sleep", "60"}, Dir: cwd, Env: os.Environ(), Timeout: 60 * time.Second, OutputLimit: 4096, ObserveDescendants: true, AfterStart: func(_ context.Context, pid int) error {
			return writeJSON(filepath.Join(root, "active.json"), Object{"wrapper": os.Getpid(), "child": pid})
		}})
		if ctx.Err() != nil {
			return ctx.Err()
		}
		return fmt.Errorf("interruption fixture unexpectedly finished: %d %v", r.ExitStatus, r.Err)
	}
	raw, e := io.ReadAll(io.LimitReader(os.Stdin, 2<<20))
	if e != nil {
		return e
	}
	if len(raw) == 2<<20 {
		return errors.New("probe input exceeds bound")
	}
	input := Object{}
	if len(raw) > 0 {
		input, e = decode(raw)
		if e != nil {
			return e
		}
	}
	start := time.Now()
	r := procgroup.Run(ctx, procgroup.Spec{Argv: append([]string{str(config["binary"])}, args...), Dir: cwd, Env: os.Environ(), Stdin: raw, Timeout: 45 * time.Second, OutputLimit: 4 << 20})
	leaked := []string{}
	for _, v := range os.Environ() {
		k, value, _ := strings.Cut(v, "=")
		if value == "qualification-sentinel" {
			leaked = append(leaked, k)
		}
	}
	e = appendJSON(filepath.Join(root, "invocations.jsonl"), Object{"argv": args, "input": input, "output": string(r.Stdout), "exit": r.ExitStatus, "milliseconds": float64(time.Since(start)) / float64(time.Millisecond), "leakedKeys": leaked})
	if e != nil {
		return e
	}
	if _, e = os.Stdout.Write(r.Stdout); e != nil {
		return e
	}
	if _, e = os.Stderr.Write(r.Stderr); e != nil {
		return e
	}
	if r.Err != nil {
		return r.Err
	}
	if r.ExitStatus != 0 {
		return fmt.Errorf("captured command exited %d", r.ExitStatus)
	}
	return nil
}
func interruptNative(ctx context.Context, c Config, root string) (Object, error) {
	out := filepath.Join(root, "interruption")
	args := []string{c.Self, "--native-only", "--interrupt-probe", "--source", c.Source, "--host", c.Host, "--corvint", c.Corvint, "--output", out}
	return interruptedCommand(ctx, c.Source, args, filepath.Join(root, "interrupted"), func() bool {
		matches, _ := filepath.Glob(filepath.Join(out, "run-*", "active.json"))
		for _, p := range matches {
			if x, e := readObject(p); e == nil && number(x["child"]) > 0 {
				return true
			}
		}
		return false
	})
}
func interruptedCommand(ctx context.Context, dir string, args []string, output string, ready func() bool) (Object, error) {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	started := make(chan int, 1)
	done := make(chan procgroup.Observation, 1)
	var witnessMu sync.Mutex
	var known map[int]procgroup.ObservedProcess
	var passiveErr error
	go func() {
		done <- procgroup.Run(ctx, procgroup.Spec{Argv: args, Dir: dir, Env: cleanEnvironment(), Timeout: 90 * time.Second, OutputLimit: 4 << 20, ObserveDescendants: true, AfterStart: func(_ context.Context, pid int) error { started <- pid; return nil }, BeforeStop: func(check context.Context, _ int) error {
			witnessMu.Lock()
			defer witnessMu.Unlock()
			passiveErr = witnessAbsent(check, dir, known)
			return passiveErr
		}})
	}()
	var result procgroup.Observation
	finished := false
	defer func() {
		if !finished {
			cancel()
			<-done
		}
	}()
	var pid int
	select {
	case pid = <-started:
	case result = <-done:
		finished = true
		return nil, fmt.Errorf("interruption helper failed to start: %v", result.Err)
	case <-ctx.Done():
		return nil, ctx.Err()
	}
	deadline := time.NewTimer(45 * time.Second)
	defer deadline.Stop()
	tick := time.NewTicker(20 * time.Millisecond)
	defer tick.Stop()
	for !ready() {
		select {
		case result = <-done:
			finished = true
			return nil, fmt.Errorf("interruption helper exited before readiness: %d %v %s", result.ExitStatus, result.Err, result.Stderr)
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-deadline.C:
			return nil, errors.New("interruption probe never had an active child")
		case <-tick.C:
		}
	}
	// Let the existing observer bind the deliberately long-lived fixture descendants.
	select {
	case <-time.After(100 * time.Millisecond):
	case <-ctx.Done():
		return nil, ctx.Err()
	}
	witnessMu.Lock()
	known, err := witnessDescendants(ctx, dir, pid)
	witnessMu.Unlock()
	if err != nil {
		return nil, err
	}
	if e := terminateProcess(pid); e != nil {
		return nil, e
	}
	select {
	case result = <-done:
		finished = true
	case <-ctx.Done():
		return nil, ctx.Err()
	}
	if e := os.WriteFile(output+".stderr", result.Stderr, 0600); e != nil {
		return nil, e
	}
	observed := []any{}
	if d := result.DescendantObservation; d != nil {
		for _, p := range d.Processes {
			if p.PID != pid {
				observed = append(observed, p.PID)
			}
		}
	}
	if passiveErr != nil {
		return nil, passiveErr
	}
	if result.ExitStatus != 143 || result.DescendantObservation == nil || !result.DescendantObservation.Absent || !result.OwnedProcessGroupCleanup || len(observed) == 0 {
		return nil, fmt.Errorf("interruption cleanup incomplete: exit=%d error=%v evidence=%+v", result.ExitStatus, result.Err, result.DescendantObservation)
	}
	return Object{"exit": 143, "observedDescendants": observed, "survivors": []any{}, "observation": result.DescendantObservation, "absentBeforeSupervisorCleanup": true, "passiveWitness": known}, nil
}
