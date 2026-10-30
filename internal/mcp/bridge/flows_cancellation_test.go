//go:build darwin || linux

package bridge

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"
)

// impactWalkContext gates only the tenth callback of the real disk walk.
// This proves cancellation reaches graph construction rather than an earlier Git call.
type impactWalkContext struct {
	context.Context
	visits  int
	once    sync.Once
	reached chan struct{}
	release chan struct{}
}

func (c *impactWalkContext) Err() error {
	pcs := make([]uintptr, 16)
	n := runtime.Callers(2, pcs)
	frames := runtime.CallersFrames(pcs[:n])
	for {
		frame, more := frames.Next()
		if strings.Contains(frame.Function, "affected.walkSourceFilesContext.func") {
			c.visits++
			if c.visits == 10 {
				c.once.Do(func() { close(c.reached); <-c.release })
			}
			break
		}
		if !more {
			break
		}
	}
	return c.Context.Err()
}

// AFU-V1-059, MCPV0-035: cancelling an impact call mid-walk retires the call.
func TestAFUV1059FlowsImpactCancelledMidWalk(t *testing.T) {
	root := flowsRepository(t)
	for i := 0; i < 30; i++ {
		dir := filepath.Join(root, fmt.Sprintf("walk%02d", i))
		if err := os.Mkdir(dir, 0700); err != nil {
			t.Fatal(err)
		}
		writeFile(t, filepath.Join(dir, "source.txt"), "source")
	}
	gitOutput(t, root, "add", ".")
	gitOutput(t, root, "commit", "-qm", "walk fixture")
	base := strings.TrimSpace(string(gitOutput(t, root, "rev-parse", "HEAD")))
	registry, err := NewFlows(root)
	if err != nil {
		t.Fatal(err)
	}
	args, _ := json.Marshal(map[string]string{"flows": "flows", "base": base})
	// Prove canonical initialization and the healthy product path before injection.
	if result, err := registry.Call(context.Background(), ToolFlowsImpact, args); err != nil || result.State != "READY" {
		t.Fatalf("healthy=%#v %v", result, err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	injected := &impactWalkContext{Context: ctx, reached: make(chan struct{}), release: make(chan struct{})}
	done := make(chan *Error, 1)
	go func() {
		result, err := registry.Call(injected, ToolFlowsImpact, args)
		if result.State == "READY" {
			done <- failure("internal-error")
			return
		}
		done <- err
	}()
	defer func() {
		cancel()
		select {
		case <-injected.release:
		default:
			close(injected.release)
		}
	}()
	select {
	case <-injected.reached:
	case err := <-done:
		t.Fatalf("impact returned before walk injection: %#v", err)
	case <-time.After(2 * time.Minute):
		t.Fatal("impact never reached real walk")
	}
	cancel()
	close(injected.release)
	select {
	case err := <-done:
		if err == nil || err.Code != "cancelled" {
			t.Fatalf("error=%#v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("cancelled impact kept walking")
	}
	if injected.visits != 10 {
		t.Fatalf("walk continued: %d callbacks", injected.visits)
	}
	if result, err := registry.Call(context.Background(), ToolFlowsImpact, args); err != nil || result.State != "READY" {
		t.Fatalf("subsequent=%#v %v", result, err)
	}
}
