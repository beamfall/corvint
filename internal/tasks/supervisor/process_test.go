//go:build darwin || linux

package supervisor

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"
)

func TestMain(m *testing.M) {
	if len(os.Args) == 4 && os.Args[1] == "orphan-owner" {
		cmd := exec.Command(os.Args[0], "lane-leader", "--directory", os.Args[2], "--capsule", os.Args[3])
		cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
		if err := cmd.Start(); err != nil {
			os.Exit(2)
		}
		_ = cmd.Wait()
		os.Exit(0)
	}

	if len(os.Args) == 6 && os.Args[1] == "lane-leader" {
		if e := Leader(context.Background(), os.Args[3], os.Args[5]); e != nil {
			fmt.Fprintln(os.Stderr, e)
			os.Exit(1)
		}
		os.Exit(0)
	}
	os.Exit(m.Run())
}
func TestSupervisorDescendantCleanup(t *testing.T) {
	for _, mode := range []string{"normal", "interrupt", "term-ignored"} {
		t.Run(mode, func(t *testing.T) {
			dir := t.TempDir()
			exe, e := filepath.EvalSymlinks("/bin/sh")
			if e != nil {
				t.Fatal(e)
			}
			binary, e := ReadBounded(exe, 256<<20)
			if e != nil {
				t.Fatal(e)
			}
			script := `echo $$ > parent
 /bin/sh -c 'echo $$ > child; /bin/sh -c '\''echo $$ > grandchild; sleep 60'\'' & wait' &
 while [ ! -s child ] || [ ! -s grandchild ]; do sleep .01; done
 echo ready > ready
 while [ ! -f continue ]; do sleep .01; done
 `
			if mode == "term-ignored" {
				script = "trap '' TERM\n" + script
				script = strings.ReplaceAll(script, "echo $$ > child;", "trap \"\" TERM; echo $$ > child;")
			}
			if mode == "normal" {
				script += "exit 0\n"
			} else {
				script += "wait\n"
			}
			capsule := Capsule{Profile: "taskman-codex-supervisor/0", Effect: strings.Repeat("a", 64), Executable: exe, ExecutableSHA256: Digest(binary), Argv: []string{"-c", script}, Directory: dir, Env: []string{"PATH=/usr/bin:/bin"}}
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			owned := map[int]string{}
			t.Cleanup(func() {
				for pid, id := range owned {
					now, _ := ProcessIdentity(pid)
					if now == id {
						_ = syscall.Kill(pid, syscall.SIGKILL)
					}
				}
			})
			phases := []string{}
			j := func(phase string, b Boot, out *Outcome) error { phases = append(phases, phase); return nil }
			type answer struct {
				o Outcome
				e error
			}
			done := make(chan answer, 1)
			go func() { o, e := Run(ctx, os.Args[0], dir, capsule, j); done <- answer{o, e} }()
			deadline := time.Now().Add(10 * time.Second)
			for {
				if _, e = os.Stat(filepath.Join(dir, "ready")); e == nil {
					break
				}
				if time.Now().After(deadline) {
					cancel()
					<-done
					t.Fatal("descendants never ready")
				}
				time.Sleep(10 * time.Millisecond)
			}
			for _, name := range []string{"parent", "child", "grandchild"} {
				raw, e := os.ReadFile(filepath.Join(dir, name))
				if e != nil {
					t.Fatal(e)
				}
				pid, e := strconv.Atoi(strings.TrimSpace(string(raw)))
				if e != nil {
					t.Fatal(e)
				}
				id, e := ProcessIdentity(pid)
				if e != nil {
					t.Fatal(e)
				}
				owned[pid] = id
			}
			if e = os.WriteFile(filepath.Join(dir, "continue"), nil, 0600); e != nil {
				t.Fatal(e)
			}
			if mode == "interrupt" || mode == "term-ignored" {
				cancel()
			}
			var got answer
			select {
			case got = <-done:
			case <-time.After(20 * time.Second):
				cancel()
				t.Fatal("cleanup deadline")
			}
			if !got.o.Clean {
				t.Fatalf("unproved cleanup: %+v %v", got.o, got.e)
			}
			for pid, id := range owned {
				now, _ := ProcessIdentity(pid)
				if id != "" && now == id {
					t.Fatalf("owned descendant %d survives", pid)
				}
			}
			if mode == "interrupt" && got.o.Class != "INTERRUPTED" {
				t.Fatal(got.o.Class)
			}
			b, _ := json.Marshal(phases)
			t.Log(string(b))
		})
	}
}

// ATR-V0-7202: killed owners and removed protocol directories cannot leak leaders.
func TestATRV07202_LeaderRetiresWhenOwnerOrDirectoryDisappears(t *testing.T) {
	for _, mode := range []string{"owner", "directory"} {
		t.Run(mode, func(t *testing.T) {
			dir := t.TempDir()
			exe, err := filepath.EvalSymlinks("/bin/sh")
			if err != nil {
				t.Fatal(err)
			}
			raw, err := ReadBounded(exe, 256<<20)
			if err != nil {
				t.Fatal(err)
			}
			c := Capsule{Profile: "taskman-codex-supervisor/0", Effect: strings.Repeat("a", 64), Executable: exe, ExecutableSHA256: Digest(raw), Argv: []string{"-c", "sleep 600"}, Directory: dir, Env: []string{"PATH=/bin:/usr/bin"}}
			b, _ := json.Marshal(c)
			if err := os.WriteFile(filepath.Join(dir, "capsule"), b, 0600); err != nil {
				t.Fatal(err)
			}
			owner := exec.Command(os.Args[0], "orphan-owner", dir, Digest(b))
			if err := owner.Start(); err != nil {
				t.Fatal(err)
			}
			defer func() { _ = owner.Process.Kill(); _ = owner.Wait() }()
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			var boot Boot
			if err := await(ctx, filepath.Join(dir, "boot"), &boot); err != nil {
				t.Fatal(err)
			}
			defer syscall.Kill(-boot.PID, syscall.SIGKILL)
			if err := Publish(dir, "ack", Ack{Boot: boot, Accepted: true}); err != nil {
				t.Fatal(err)
			}
			if mode == "owner" {
				_ = owner.Process.Kill()
				_ = owner.Wait()
			} else {
				if err := os.RemoveAll(dir); err != nil {
					t.Fatal(err)
				}
			}
			deadline := time.Now().Add(3 * time.Second)
			for time.Now().Before(deadline) {
				now, _ := ProcessIdentity(boot.PID)
				if now == "" || now != boot.Started {
					return
				}
				time.Sleep(20 * time.Millisecond)
			}
			t.Fatal("leader outlived owner/directory for 3s")
		})
	}
}
