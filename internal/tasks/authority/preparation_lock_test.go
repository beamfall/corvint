//go:build darwin || linux

package authority_test

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"github.com/Beamfall/corvint/internal/tasks/authority"
	"github.com/Beamfall/corvint/internal/tasks/fixture"
	"github.com/Beamfall/corvint/internal/tasks/intent"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// CAL-V0-026: preparation is distinct cross-process admission, never writer
// authority. Existing writer identity and cancellation checks remain applicable.
func TestCALV0026_PreparationGateAuthority(t *testing.T) {
	if root := os.Getenv("CORVINT_PREPARATION_HELPER_ROOT"); root != "" {
		repo, err := intent.Resolve(root)
		if err != nil {
			t.Fatal(err)
		}
		lock, err := authority.AcquirePreparation(context.Background(), repo, authority.LockOptions{})
		if err != nil {
			t.Fatal(err)
		}
		fmt.Println("preparation-acquired")
		_, _ = io.Copy(io.Discard, os.Stdin)
		if err := lock.Close(); err != nil {
			t.Fatal(err)
		}
		return
	}
	t.Run("distinct-authority-and-observers", func(t *testing.T) {
		_, repo := resolved(t)
		var observations []authority.PreparationObservation
		writers := 0
		ctx := authority.WithLockObserver(context.Background(), func(time.Duration) { writers++ })
		ctx = authority.WithPreparationObserver(ctx, func(o authority.PreparationObservation) { observations = append(observations, o) })
		gate, err := authority.AcquirePreparation(ctx, repo, authority.LockOptions{})
		if err != nil {
			t.Fatal(err)
		}
		defer gate.Close()
		if gate.Path() != filepath.Join(repo.CommonDir, "taskman.prepare.lock") {
			t.Fatal(gate.Path())
		}
		if reflect.TypeOf(gate).ConvertibleTo(reflect.TypeOf((*authority.Lock)(nil))) {
			t.Fatal("preparation convertible to writer authority")
		}
		native, err := authority.AcquireLock(ctx, repo, authority.LockOptions{Wait: time.Second})
		if err != nil {
			t.Fatal(err)
		}
		if err := native.Close(); err != nil {
			t.Fatal(err)
		}
		_, other := resolved(t)
		independent, err := authority.AcquirePreparation(ctx, other, authority.LockOptions{Wait: time.Second})
		if err != nil {
			t.Fatal(err)
		}
		if err := independent.Close(); err != nil {
			t.Fatal(err)
		}
		if err := gate.Close(); err != nil {
			t.Fatal(err)
		}
		if err := gate.Close(); err != nil {
			t.Fatal(err)
		}
		if writers != 1 || len(observations) != 2 {
			t.Fatalf("writers=%d preparation=%+v", writers, observations)
		}
		for _, o := range observations {
			if !o.Acquired || !o.Released || o.Err != nil || o.Wait < 0 || o.Hold <= 0 {
				t.Fatalf("observation %+v", o)
			}
		}
		if _, err := os.Stat(gate.Path()); err != nil {
			t.Fatal("inert file removed", err)
		}
	})
	t.Run("cancel-and-timeout", func(t *testing.T) {
		_, repo := resolved(t)
		done, cancel := context.WithCancel(context.Background())
		cancel()
		if l, err := authority.AcquirePreparation(done, repo, authority.LockOptions{}); l != nil || !errors.Is(err, context.Canceled) {
			t.Fatalf("precancel %v %v", l, err)
		}
		if _, err := os.Stat(filepath.Join(repo.CommonDir, "taskman.prepare.lock")); !os.IsNotExist(err) {
			t.Fatal("precancel opened gate", err)
		}
		holder, err := authority.AcquirePreparation(context.Background(), repo, authority.LockOptions{})
		if err != nil {
			t.Fatal(err)
		}
		defer holder.Close()
		var observations []authority.PreparationObservation
		observed := authority.WithPreparationObserver(context.Background(), func(o authority.PreparationObservation) { observations = append(observations, o) })
		timeout, stop := context.WithTimeout(observed, 30*time.Millisecond)
		defer stop()
		if l, err := authority.AcquirePreparation(timeout, repo, authority.LockOptions{Wait: time.Second}); l != nil || !errors.Is(err, context.DeadlineExceeded) {
			t.Fatalf("wait cancel %v %v", l, err)
		}
		if l, err := authority.AcquirePreparation(observed, repo, authority.LockOptions{Wait: 30 * time.Millisecond, Poll: time.Millisecond}); l != nil || wire.CodeOf(err) != wire.CodeLockTimeout {
			t.Fatalf("wait timeout %v %v", l, err)
		}
		if len(observations) != 2 {
			t.Fatal(observations)
		}
		for _, o := range observations {
			if o.Acquired || o.Hold != 0 || o.Wait <= 0 {
				t.Fatal(o)
			}
		}
		if err := holder.Close(); err != nil {
			t.Fatal(err)
		}
		next, err := authority.AcquirePreparation(context.Background(), repo, authority.LockOptions{Wait: time.Second})
		if err != nil {
			t.Fatal(err)
		}
		if err := next.Close(); err != nil {
			t.Fatal(err)
		}
	})
	t.Run("unsafe-paths", func(t *testing.T) {
		for _, kind := range []string{"symlink", "directory", "wrong-tuple"} {
			t.Run(kind, func(t *testing.T) {
				_, repo := resolved(t)
				path := filepath.Join(repo.CommonDir, "taskman.prepare.lock")
				switch kind {
				case "symlink":
					if err := os.Symlink("HEAD", path); err != nil {
						t.Fatal(err)
					}
				case "directory":
					if err := os.Mkdir(path, 0700); err != nil {
						t.Fatal(err)
					}
				case "wrong-tuple":
					repo.LockPath = path
				}
				if l, err := authority.AcquirePreparation(context.Background(), repo, authority.LockOptions{}); l != nil || wire.CodeOf(err) != wire.CodeUnsupportedFilesystem {
					t.Fatalf("%s: %v %v", kind, l, err)
				}
			})
		}
	})
	// The replaced-while-waiting assertion uses the reached gate-open seam in
	// TestGH494PreparationOpen; registration adds guards before that boundary.
	t.Run("cross-process-death", func(t *testing.T) {
		_, repo := resolved(t)
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		cmd := exec.CommandContext(ctx, os.Args[0], "-test.run=^TestCALV0026_PreparationGateAuthority$")
		cmd.Env = append(os.Environ(), "CORVINT_PREPARATION_HELPER_ROOT="+repo.PrimaryWorktree)
		stdin, err := cmd.StdinPipe()
		if err != nil {
			t.Fatal(err)
		}
		stdout, err := cmd.StdoutPipe()
		if err != nil {
			t.Fatal(err)
		}
		cmd.Stderr = os.Stderr
		if err := cmd.Start(); err != nil {
			t.Fatal(err)
		}
		joined := false
		defer func() {
			stdin.Close()
			if !joined {
				cmd.Process.Kill()
				cmd.Wait()
			}
		}()
		line, err := bufio.NewReader(stdout).ReadString('\n')
		if err != nil || line != "preparation-acquired\n" {
			t.Fatalf("helper %q %v", line, err)
		}
		if l, err := authority.AcquirePreparation(context.Background(), repo, authority.LockOptions{Wait: 30 * time.Millisecond, Poll: time.Millisecond}); l != nil || wire.CodeOf(err) != wire.CodeLockTimeout {
			t.Fatalf("process exclusion %v %v", l, err)
		}
		// The helper has no children; its inherited outer process group is unchanged.
		if err := cmd.Process.Kill(); err != nil {
			t.Fatal(err)
		}
		err = cmd.Wait()
		joined = true
		if err == nil {
			t.Fatal("killed helper exited successfully")
		}
		next, err := authority.AcquirePreparation(context.Background(), repo, authority.LockOptions{Wait: time.Second})
		if err != nil {
			t.Fatal(err)
		}
		if err := next.Close(); err != nil {
			t.Fatal(err)
		}
	})
	t.Run("probe-and-first-file-outside-observed-state", func(t *testing.T) {
		fr, repo := resolved(t)
		fixture.WriteIntent(t, fr)
		fixture.WriteState(t, fr)
		guard, err := authority.WatchChanges(repo)
		if err != nil {
			t.Fatal(err)
		}
		defer guard.Close()
		if _, err := authority.Qualify(repo.CommonDir); err != nil {
			t.Fatal(err)
		}
		gate, err := authority.AcquirePreparation(context.Background(), repo, authority.LockOptions{})
		if err != nil {
			t.Fatal(err)
		}
		if err := gate.Close(); err != nil {
			t.Fatal(err)
		}
		if err := guard.Check(); err != nil {
			t.Fatalf("probe/first gate creation invalidated unrelated state: %v", err)
		}
	})
}
