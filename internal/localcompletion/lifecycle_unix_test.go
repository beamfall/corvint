//go:build unix

package localcompletion

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"
)

func TestLocalStateBoundsAndContention(t *testing.T) {
	t.Run("LCP-V0-012 bound", func(t *testing.T) {
		root, key, plan := fixture(t, []string{"true"})
		repo := beginFixture(t, root, key, plan)
		if validPath("..") {
			t.Fatal("parent path accepted")
		}
		original, _ := os.ReadFile(repo.local("state.json"))
		if err := os.Remove(repo.local("state.json")); err != nil {
			t.Fatal(err)
		}
		if err := syscall.Mkfifo(repo.local("state.json"), 0600); err != nil {
			t.Fatal(err)
		}
		start := time.Now()
		if _, err := Evaluate(context.Background(), root, key); err == nil {
			t.Fatal("FIFO accepted")
		}
		if time.Since(start) > time.Second {
			t.Fatal("FIFO blocked")
		}
		os.Remove(repo.local("state.json"))
		os.WriteFile(repo.local("state.json"), original, 0600)
		var duplicate Plan
		if strictJSON([]byte(`{"base":"a","base":"b"}`), &duplicate, MaxPlanBytes) == nil {
			t.Fatal("duplicate key accepted")
		}
		if strictJSON(make([]byte, MaxPlanBytes+1), &duplicate, MaxPlanBytes) == nil {
			t.Fatal("oversize accepted")
		}
		if _, err := Cancel(context.Background(), root, key); err != nil {
			t.Fatal(err)
		}
		var group sync.WaitGroup
		results := make(chan error, 2)
		raw, _ := json.Marshal(plan)
		for _, session := range []string{HashSession("one"), HashSession("two")} {
			group.Add(1)
			go func(session string) {
				defer group.Done()
				_, err := Begin(context.Background(), root, session, raw)
				results <- err
			}(session)
		}
		group.Wait()
		close(results)
		success := 0
		for err := range results {
			if err == nil {
				success++
			}
		}
		if success != 1 {
			t.Fatalf("concurrent enrollment successes=%d", success)
		}
	})
}

func TestVerificationCancellationCleansDescendant(t *testing.T) {
	root, key, plan := fixture(t, []string{"sh", "-c", "sleep 30 & echo $! > .git/descendant.pid; wait"})
	beginFixture(t, root, key, plan)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan error, 1)
	go func() { _, err := Verify(ctx, root, key, "test"); done <- err }()
	var pid int
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		raw, err := os.ReadFile(filepath.Join(root, ".git/descendant.pid"))
		if err == nil {
			pid, _ = strconv.Atoi(strings.TrimSpace(string(raw)))
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if pid == 0 {
		cancel()
		<-done
		t.Fatal("descendant did not start")
	}
	t.Cleanup(func() { _ = syscall.Kill(pid, syscall.SIGKILL) })
	cancel()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("cancel did not finish")
	}
	if err := syscall.Kill(pid, 0); err == nil {
		t.Fatal("descendant survived interruption")
	}
}
