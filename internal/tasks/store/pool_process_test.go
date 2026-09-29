//go:build unix

package store

import (
	"context"
	"fmt"
	"github.com/Beamfall/corvint/internal/tasks/intent"
	"github.com/Beamfall/corvint/internal/tasks/wire"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"
)

// CAL-V0-033.
func TestPoolProcessDescendants(t *testing.T) {
	for _, scenario := range []string{"success", "timeout", "interrupt"} {
		t.Run(scenario, func(t *testing.T) {
			root := t.TempDir()
			pids := filepath.Join(root, "pids")
			script := `trap 'kill "$child" 2>/dev/null || :; wait "$child" 2>/dev/null || :' INT TERM
/bin/sh -c 'sleep 60 & child=$!; echo $$ >> "$1"; echo $child >> "$1"; trap '\''kill "$child" 2>/dev/null || :; wait "$child" 2>/dev/null || :'\'' EXIT INT TERM; wait "$child"' sh "$1" &
child=$!
echo $$ >> "$1"
while [ "$(/usr/bin/wc -l < "$1")" -lt 3 ]; do sleep 0.01; done
if [ "$2" = success ]; then exit 0; fi
wait "$child"
`
			file := filepath.Join(root, "probe.sh")
			if e := os.WriteFile(file, []byte(script), 0600); e != nil {
				t.Fatal(e)
			}
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			timeout := wire.Count("1")
			if scenario != "timeout" {
				timeout = "5"
			}
			def := &intent.PoolCommand{Argv: []string{"/bin/sh", file, pids, scenario}, TimeoutSeconds: timeout}
			done := make(chan struct{})
			var class string
			var clean bool
			go func() { class, clean, _ = executePool(ctx, def, root, nil); close(done) }()
			owned := map[int]string{}
			t.Cleanup(func() {
				cancel()
				select {
				case <-done:
				case <-time.After(8 * time.Second):
					t.Error("runner did not stop")
				}
				for pid, identity := range owned {
					current, e := poolRunnerIdentity(pid)
					if e == nil && current != "" && current == identity {
						_ = syscall.Kill(pid, syscall.SIGKILL)
					}
				}
			})
			{
				deadline := time.Now().Add(3 * time.Second)
				for {
					raw, _ := os.ReadFile(pids)
					if len(strings.Fields(string(raw))) >= 3 {
						break
					}
					if time.Now().After(deadline) {
						t.Fatal("descendants did not start")
					}
					time.Sleep(10 * time.Millisecond)
				}
				raw, _ := os.ReadFile(pids)
				for _, p := range strings.Fields(string(raw)) {
					pid, _ := strconv.Atoi(p)
					identity, _ := poolRunnerIdentity(pid)
					if identity != "" {
						owned[pid] = identity
					}
				}
				if scenario == "interrupt" {
					cancel()
				}
			}
			select {
			case <-done:
			case <-time.After(8 * time.Second):
				t.Fatal("runner hung")
			}
			want := map[string]string{"success": "EXIT_ZERO", "timeout": "TIMEOUT", "interrupt": "INTERRUPTED"}[scenario]
			if class != want || !clean {
				t.Fatalf("%s clean=%v", class, clean)
			}
			raw, e := os.ReadFile(pids)
			if e != nil {
				t.Fatal(e)
			}
			for _, p := range strings.Fields(string(raw)) {
				pid, _ := strconv.Atoi(p)
				state, e := exec.Command("/bin/ps", "-p", fmt.Sprint(pid), "-o", "stat=").Output()
				if e == nil && !strings.HasPrefix(strings.TrimSpace(string(state)), "Z") {
					t.Errorf("surviving descendant %d: %s", pid, state)
				}
			}
		})
	}
}
