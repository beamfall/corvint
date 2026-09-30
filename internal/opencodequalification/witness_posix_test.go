//go:build darwin || linux

package opencodequalification

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/Beamfall/corvint/internal/procgroup"
)

func TestGateInterruptionHelper(t *testing.T) {
	mode := os.Getenv("CORVINT_QUALIFICATION_HELPER")
	if mode == "" {
		return
	}
	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGTERM)
	defer cancel()
	dir := os.Getenv("CORVINT_QUALIFICATION_DIR")
	if mode == "gate" {
		env := []string{}
		for _, v := range os.Environ() {
			if !strings.HasPrefix(v, "CORVINT_QUALIFICATION_HELPER=") {
				env = append(env, v)
			}
		}
		env = append(env, "CORVINT_QUALIFICATION_HELPER=broken")
		_, _ = runCommand(ctx, dir, env, []string{os.Args[0], "-test.run=^TestGateInterruptionHelper$"}, dir+"/gate", time.Minute)
	} else {
		child, e := os.StartProcess("/bin/sleep", []string{"sleep", "60"}, &os.ProcAttr{Dir: dir, Env: os.Environ(), Files: []*os.File{os.Stdin, os.Stdout, os.Stderr}, Sys: &syscall.SysProcAttr{Setsid: true}})
		if e != nil {
			fmt.Fprintln(os.Stderr, e)
			os.Exit(1)
		}
		_ = writeJSON(dir+"/active.json", Object{"child": child.Pid})
		<-ctx.Done()
	}
	os.Exit(143)
}
func TestGateInterruptionWitness(t *testing.T) {
	t.Run("GOC-V0-008 AHI-032 detached gate cleanup and negative control", func(t *testing.T) {
		for _, mode := range []string{"gate", "broken"} {
			t.Run(mode, func(t *testing.T) {
				dir := t.TempDir()
				t.Setenv("CORVINT_QUALIFICATION_HELPER", mode)
				t.Setenv("CORVINT_QUALIFICATION_DIR", dir)
				ctx, cancel := context.WithTimeout(t.Context(), time.Minute)
				defer cancel()
				_, e := interruptedCommand(ctx, dir, []string{os.Args[0], "-test.run=^TestGateInterruptionHelper$"}, filepath.Join(dir, "outer"), func() bool { x, e := readObject(dir + "/active.json"); return e == nil && number(x["child"]) > 0 })
				if mode == "gate" && e != nil {
					t.Fatal(e)
				}
				if mode == "broken" && (e == nil || !strings.Contains(e.Error(), "before supervisor rescue")) {
					t.Fatalf("broken cleanup was not rejected: %v", e)
				}
				active, _ := readObject(dir + "/active.json")
				rows, e := processSnapshot(ctx, dir)
				if e != nil {
					t.Fatal(e)
				}
				if p, ok := rows[int(number(active["child"]))]; ok && !strings.HasPrefix(p.State, "Z") {
					t.Fatal("rescue left negative-control child alive", p)
				}
			})
		}
	})
}

func TestPassiveWitnessSettle(t *testing.T) {
	for _, transient := range []bool{true, false} {
		name := "AHI-032 persistent live survivor rejected"
		if transient {
			name = "AHI-032 transient descendant exit accepted"
		}
		t.Run(name, func(t *testing.T) {
			cmd := exec.Command("/bin/sleep", "60")
			if err := cmd.Start(); err != nil {
				t.Fatal(err)
			}
			defer cmd.Wait()
			defer cmd.Process.Kill()
			ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
			defer cancel()
			rows, err := processSnapshot(ctx, t.TempDir())
			if err != nil {
				t.Fatal(err)
			}
			owned, ok := rows[cmd.Process.Pid]
			if !ok {
				t.Fatal("child identity missing")
			}
			if transient {
				done := make(chan struct{})
				timer := time.AfterFunc(150*time.Millisecond, func() { defer close(done); _ = cmd.Process.Kill() })
				defer func() {
					if !timer.Stop() {
						<-done
					}
				}()
			}
			err = witnessAbsent(ctx, t.TempDir(), map[int]procgroup.ObservedProcess{owned.PID: owned})
			if transient && err != nil {
				t.Fatal(err)
			}
			if !transient && (err == nil || !strings.Contains(err.Error(), "before supervisor rescue")) {
				t.Fatalf("survivor was not rejected: %v", err)
			}
		})
	}
}
