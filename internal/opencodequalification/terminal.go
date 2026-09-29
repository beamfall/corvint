package opencodequalification

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"sync"
	"time"

	"github.com/Beamfall/corvint/internal/procgroup"
)

type terminal struct {
	input  io.WriteCloser
	mu     sync.Mutex
	done   chan procgroup.Observation
	cancel context.CancelFunc
	result *procgroup.Observation
}

func startTerminal(ctx context.Context, c Config, root, repo string, env []string) (*terminal, error) {
	config := root + "/terminal.json"
	if e := writeJSON(config, Object{"host": c.Host, "directory": repo, "env": env}); e != nil {
		return nil, e
	}
	ctx, cancel := context.WithCancel(ctx)
	t := &terminal{done: make(chan procgroup.Observation, 1), cancel: cancel}
	ready := make(chan io.WriteCloser, 1)
	go func() {
		t.done <- procgroup.Run(ctx, procgroup.Spec{Argv: []string{c.Self, "--terminal-config", config}, Dir: repo, Env: env, Timeout: 3 * time.Minute, OutputLimit: 16 << 20, ObserveDescendants: true, Dialogue: func(out io.Reader, in io.WriteCloser) error { ready <- in; _, e := io.Copy(io.Discard, out); return e }})
	}()
	select {
	case t.input = <-ready:
		return t, nil
	case r := <-t.done:
		cancel()
		return nil, fmt.Errorf("terminal start: %v %s", r.Err, r.Stderr)
	case <-ctx.Done():
		cancel()
		r := <-t.done
		return nil, fmt.Errorf("terminal cancelled: %v", r.Err)
	}
}
func (t *terminal) command(v Object) error {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.input == nil {
		return errors.New("terminal closed")
	}
	return json.NewEncoder(t.input).Encode(v)
}
func (t *terminal) send(s string) error { return t.command(Object{"send": s}) }
func (t *terminal) resize(width, height int) error {
	return t.command(Object{"resize": []int{width, height}})
}
func (t *terminal) close(root string) error {
	if t.result != nil {
		return nil
	}
	_ = t.command(Object{"stop": true})
	t.mu.Lock()
	if t.input != nil {
		_ = t.input.Close()
		t.input = nil
	}
	t.mu.Unlock()
	var r procgroup.Observation
	select {
	case r = <-t.done:
	case <-time.After(10 * time.Second):
		t.cancel()
		r = <-t.done
	}
	t.cancel()
	t.result = &r
	survivors := []int{}
	if r.DescendantObservation == nil || !r.DescendantObservation.Absent {
		survivors = append(survivors, -1)
	}
	if e := writeJSON(root+"/cleanup.json", Object{"survivors": survivors, "observed": r.DescendantObservation, "groupClean": r.OwnedProcessGroupCleanup}); e != nil {
		return e
	}
	if e := os.WriteFile(root+"/terminal.stderr", r.Stderr, 0600); e != nil {
		return e
	}
	if !r.OwnedProcessGroupCleanup || len(survivors) > 0 {
		return fmt.Errorf("terminal cleanup incomplete: %+v", r.DescendantObservation)
	}
	return nil
}
