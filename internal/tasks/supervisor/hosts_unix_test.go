//go:build darwin || linux

package supervisor

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestCALV0074_CapsuleHost proves the capsule names its host canonically and
// that the lane leader decodes a zero exit in that host's vocabulary.
func TestCALV0074_CapsuleHost(t *testing.T) {
	exe, e := filepath.EvalSymlinks("/bin/sh")
	if e != nil {
		t.Fatal(e)
	}
	binary, e := ReadBounded(exe, 256<<20)
	if e != nil {
		t.Fatal(e)
	}
	base := Capsule{Profile: "taskman-codex-supervisor/0", Effect: strings.Repeat("b", 64), Executable: exe, ExecutableSHA256: Digest(binary), Directory: t.TempDir(), Env: []string{"PATH=/usr/bin:/bin"}}
	for _, host := range []string{HostCodex, "opencode", "CLAUDE-CODE"} {
		c := base
		c.Host = host
		if e := ValidateCapsule(c); e == nil {
			t.Fatalf("capsule host %q admitted", host)
		}
	}
	codex, _ := json.Marshal(base)
	if strings.Contains(string(codex), `"host"`) {
		t.Fatalf("codex capsule bytes carry host: %s", codex)
	}
	result := `{"type":"result","subtype":"success","is_error":false,"session_id":"claude-session","result":"{\"kind\":\"BUILT\",\"summary\":\"done\",\"nextAction\":\"review\"}","usage":{"input_tokens":4,"output_tokens":2}}`
	for _, host := range []string{"", HostClaudeCode} {
		c := base
		c.Host = host
		c.Effect = Digest([]byte(host))
		c.Argv = []string{"-c", "printf '%s\\n' '" + result + "'"}
		if e := ValidateCapsule(c); e != nil {
			t.Fatalf("capsule host %q refused: %v", host, e)
		}
		dir := t.TempDir()
		out, e := Run(context.Background(), os.Args[0], dir, c, func(string, Boot, *Outcome) error { return nil })
		if e != nil && host == HostClaudeCode {
			t.Fatalf("claude-code run: %v", e)
		}
		if host == HostClaudeCode {
			if out.Class != "EXIT_ZERO" || out.SessionID != "claude-session" || out.Result.Kind != "BUILT" || !out.Clean {
				t.Fatalf("claude-code outcome %+v", out)
			}
		} else if out.Class != "INVALID_RESULT" {
			t.Fatalf("codex vocabulary admitted a Claude Code result: %+v", out)
		}
	}
	// A duplicate is_error (true then false) is not a successful result.
	duplicate := `{"type":"result","subtype":"success","is_error":true,"is_error":false,"session_id":"claude-session","result":"{\"kind\":\"BUILT\",\"summary\":\"done\",\"nextAction\":\"review\"}","usage":{"input_tokens":4,"output_tokens":2}}`
	c := base
	c.Host = HostClaudeCode
	c.Effect = Digest([]byte("duplicate"))
	c.Argv = []string{"-c", "printf '%s\\n' '" + duplicate + "'"}
	out, _ := Run(context.Background(), os.Args[0], t.TempDir(), c, func(string, Boot, *Outcome) error { return nil })
	if out.Class != "INVALID_RESULT" || out.Result.Kind != "" {
		t.Fatalf("duplicate is_error outcome %+v", out)
	}
}

// TestCALV0074_RuntimeReplacedAtAck proves the lane leader executes the bytes
// it verified: replacing the pinned runtime, in place or by rename, at the
// RUNNING journal boundary (after the leader's check, before ack:true) never
// runs the replacement, for either host. The private copy is removed after the
// host exits.
func TestCALV0074_RuntimeReplacedAtAck(t *testing.T) {
	original := "#!/bin/sh\nprintf original > ran\n"
	replacement := "#!/bin/sh\nprintf replacement > ran\n"
	for _, host := range []string{"", HostClaudeCode} {
		for _, mode := range []string{"in place", "rename"} {
			t.Run(host+" "+mode, func(t *testing.T) {
				bin := t.TempDir()
				exe := filepath.Join(bin, "runtime")
				if e := os.WriteFile(exe, []byte(original), 0o755); e != nil {
					t.Fatal(e)
				}
				work, dir := t.TempDir(), t.TempDir()
				c := Capsule{Profile: "taskman-codex-supervisor/0", Effect: Digest([]byte(host + mode)), Executable: exe, ExecutableSHA256: Digest([]byte(original)), Directory: work, Env: []string{"PATH=/usr/bin:/bin"}, Host: host}
				replaced := false
				j := func(phase string, _ Boot, _ *Outcome) error {
					if phase != "RUNNING" {
						return nil
					}
					var e error
					if mode == "in place" {
						e = os.WriteFile(exe, []byte(replacement), 0o755)
					} else {
						next := filepath.Join(bin, "next")
						if e = os.WriteFile(next, []byte(replacement), 0o755); e == nil {
							e = os.Rename(next, exe)
						}
					}
					replaced = e == nil
					return e
				}
				out, _ := Run(context.Background(), os.Args[0], dir, c, j)
				if !replaced {
					t.Fatal("runtime was not replaced at the ack boundary")
				}
				ran, e := os.ReadFile(filepath.Join(work, "ran"))
				if e != nil || string(ran) != "original" {
					t.Fatalf("ran %q (%v), outcome %+v", ran, e, out)
				}
				if _, e = os.Lstat(filepath.Join(dir, "runtime")); !os.IsNotExist(e) {
					t.Fatalf("private runtime copy left behind: %v", e)
				}
			})
		}
	}
	// Only a canonical, root-owned runtime under root-owned directories that
	// no group or other can write runs by its path.
	system, e := filepath.EvalSymlinks("/bin/sh")
	if e != nil {
		t.Fatal(e)
	}
	user := filepath.Join(t.TempDir(), "runtime")
	if e = os.WriteFile(user, []byte(original), 0o755); e != nil {
		t.Fatal(e)
	}
	for path, want := range map[string]bool{system: os.Getuid() != 0, user: false} {
		_, st, e := readRuntime(path)
		if e != nil {
			t.Fatal(e)
		}
		if got := protectedRuntime(path, st); got != want {
			t.Fatalf("protectedRuntime(%s) = %v", path, got)
		}
	}
}

// TestCALV0074_PrelaunchErrorOnlyBeforeSpawn proves only a refusal before the
// lane leader is forked is a PrelaunchError. A journal failure at RUNNING
// returns an empty class after a spawn, so it must never be read as NO_EXEC:
// the leader is drained and journaled FINISHED or BLOCKED_RECOVERY.
func TestCALV0074_PrelaunchErrorOnlyBeforeSpawn(t *testing.T) {
	exe, e := filepath.EvalSymlinks("/bin/sh")
	if e != nil {
		t.Fatal(e)
	}
	binary, e := ReadBounded(exe, MaxRuntime)
	if e != nil {
		t.Fatal(e)
	}
	base := Capsule{Profile: "taskman-codex-supervisor/0", Effect: strings.Repeat("c", 64), Executable: exe, ExecutableSHA256: Digest(binary), Argv: []string{"-c", "exit 0"}, Directory: t.TempDir(), Env: []string{"PATH=/usr/bin:/bin"}}
	var pre *PrelaunchError

	stale := base
	stale.ExecutableSHA256 = Digest([]byte("other"))
	if _, e := Run(context.Background(), os.Args[0], t.TempDir(), stale, func(string, Boot, *Outcome) error { return nil }); !errors.As(e, &pre) {
		t.Fatalf("capsule refusal is not prelaunch: %v", e)
	}
	dir := t.TempDir()
	_, e = Run(context.Background(), os.Args[0], dir, base, func(phase string, _ Boot, _ *Outcome) error {
		if phase == "SPAWNING" {
			return fmt.Errorf("journal refused")
		}
		return nil
	})
	if !errors.As(e, &pre) {
		t.Fatalf("SPAWNING journal refusal is not prelaunch: %v", e)
	}
	if _, e := os.Lstat(filepath.Join(dir, "boot")); !os.IsNotExist(e) {
		t.Fatal("a leader booted after a prelaunch refusal")
	}

	phases := []string{}
	out, e := Run(context.Background(), os.Args[0], t.TempDir(), base, func(phase string, _ Boot, o *Outcome) error {
		if o != nil {
			phase += ":" + o.Class
		}
		phases = append(phases, phase)
		if phase == "RUNNING" {
			return fmt.Errorf("journal refused")
		}
		return nil
	})
	if e == nil || errors.As(e, &pre) {
		t.Fatalf("RUNNING journal refusal: want a non-prelaunch error, got %v", e)
	}
	if out.Class != "" || out.Boot.PID <= 0 {
		t.Fatalf("outcome %+v", out)
	}
	last := phases[len(phases)-1]
	if last != "FINISHED:" && last != "BLOCKED_RECOVERY:" {
		t.Fatalf("phases %v", phases)
	}
	for _, p := range phases {
		if strings.HasSuffix(p, ":NO_EXEC") {
			t.Fatalf("spawned leader journaled NO_EXEC: %v", phases)
		}
	}
}
