//go:build darwin || linux

package supervisor

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"
)

func TestMain(m *testing.M) {
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
