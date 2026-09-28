//go:build darwin || linux

package opencodequalification

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"
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
