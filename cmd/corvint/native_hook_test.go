package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"syscall"
	"testing"
	"time"
)

func TestNativeHookClosedProjection(t *testing.T) {
	t.Parallel()
	handle := strings.Repeat("a", 64)
	raw := []byte(`{"hook_event_name":"Stop","stop_hook_active":false,"session_id":"private","transcript_path":"/private/transcript","cwd":"/evil","FULL":true,"root":"/fake","consumer":"/fake","prompt":"private"}`)
	got, err := normalizeNativeHook(raw, false, handle)
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]any{"profile": "corvint-authority-event/0", "event": "stop", "stopHookActive": false, "enrollmentHandle": handle}
	if !reflect.DeepEqual(got, want) {
		t.Fatal(got)
	}
	for _, test := range []struct{ raw, handle string }{{`{"hook_event_name":"Stop"}`, handle}, {`{"hook_event_name":"Stop","stop_hook_active":null}`, handle}, {`{"hook_event_name":"Stop","stop_hook_active":false}`, ""}} {
		if _, err := normalizeNativeHook([]byte(test.raw), false, test.handle); err == nil {
			t.Fatal("invalid Stop admitted")
		}
	}
}
func TestNativeHookAllQualifiedEvents(t *testing.T) {
	t.Parallel()
	sum := sha256.Sum256([]byte("corvint-local-completion-session/0\x00private-session"))
	for event, want := range map[string]map[string]any{"SessionStart": {"startSource": "compact"}, "UserPromptSubmit": {"task": "AGENTS.md"}, "Stop": {"stopHookActive": false, "changedPaths": []string{}}, "SessionEnd": {"openedPaths": []string{}, "changedPaths": []string{}, "verification": []any{}}} {
		payload := map[string]any{"hook_event_name": event, "source": "compact", "prompt": " AGENTS.md ", "stop_hook_active": false, "session_id": "private-session", "transcript_path": "/private/transcript", "root": "/fake", "cwd": "/fake", "enrollmentHandle": strings.Repeat("a", 64), "support": "FULL", "tool_response": "private-tool"}
		raw, _ := json.Marshal(payload)
		got, err := normalizeNativeHook(raw, true, "")
		if err != nil {
			t.Fatal(err)
		}
		want["sessionIdSha256"] = hex.EncodeToString(sum[:])
		if len(got) != 3 || got["profile"] != qualifiedLifecycleProfile || !reflect.DeepEqual(got["input"], want) {
			t.Fatalf("%s: %#v", event, got)
		}
		encoded, _ := json.Marshal(got)
		if bytes.Contains(encoded, []byte("private-")) || bytes.Contains(encoded, []byte("/fake")) {
			t.Fatal("private caller data forwarded")
		}
	}
}
func TestNativeHookUnicodeAndInputBounds(t *testing.T) {
	t.Parallel()
	for _, prompt := range []string{"", "   ", strings.Repeat("x", 2001)} {
		raw, _ := json.Marshal(map[string]any{"hook_event_name": "UserPromptSubmit", "prompt": prompt})
		if _, err := normalizeNativeHook(raw, true, ""); err == nil {
			t.Fatal("unbounded prompt admitted")
		}
	}
	prompt := strings.Repeat("😀", 2000)
	raw, _ := json.Marshal(map[string]any{"hook_event_name": "UserPromptSubmit", "prompt": prompt})
	result, err := normalizeNativeHook(raw, true, "")
	if err != nil || result["input"].(map[string]any)["task"] != prompt {
		t.Fatal("valid Unicode prompt refused")
	}
	for _, raw := range [][]byte{[]byte(`{"hook_event_name":"Stop"}`), []byte(`{"hook_event_name":"Stop","stop_hook_active":1}`), []byte(`{"hook_event_name":"Stop","stop_hook_active":false,"stop_hook_active":true}`), {255}, []byte(`{"hook_event_name":"UserPromptSubmit","prompt":"\ud800"}`), bytes.Repeat([]byte("x"), nativeHookMaxInput+1)} {
		if _, err := normalizeNativeHook(raw, true, ""); err == nil {
			t.Fatalf("invalid input admitted %q", raw)
		}
	}
}
func TestNativeHookRepositoryCopyIsUnadmitted(t *testing.T) {
	t.Parallel()
	var out bytes.Buffer
	code := runNativeHook(context.Background(), []string{"--qualified-lifecycle"}, strings.NewReader(""), &out, &bytes.Buffer{})
	if code != 0 || !strings.Contains(out.String(), "FALLBACK") || strings.Contains(out.String(), `"decision"`) {
		t.Fatalf("code=%d output=%s", code, &out)
	}
	if nativeActiveEnrollment != "/Library/CorvintAuthority/public/active-enrollment.json" {
		t.Fatal("active pointer moved")
	}
}
func TestNativeHookEnvironmentIsClosed(t *testing.T) {
	t.Parallel()
	exe := "/Library/CorvintAuthority/versions/" + strings.Repeat("a", 64) + "/corvint"
	want := []string{"LANG=C.UTF-8", "PATH=" + filepath.Dir(exe) + "/bin"}
	if !reflect.DeepEqual(nativeHookEnvironment(exe), want) {
		t.Fatal("environment changed")
	}
	for _, actual := range [][]string{append(append([]string(nil), want...), "HOME=/attacker"), {want[0], "PATH=/usr/bin"}, {want[0], want[0]}} {
		if exactNativeHookEnvironment(actual, want) {
			t.Fatal("ambient environment admitted")
		}
	}
	if !exactNativeHookEnvironment([]string{want[1], want[0]}, want) {
		t.Fatal("ordering changed environment identity")
	}
}
func TestNativeHookExecHelper(t *testing.T) {
	marker := -1
	for i, arg := range os.Args {
		if arg == "native-exec-helper" {
			marker = i
			break
		}
	}
	if marker < 0 {
		return
	}
	mode, path := os.Args[marker+1], os.Args[marker+2]
	if mode == "replace" {
		exe, _ := os.Executable()
		args := []string{exe, "-test.run=^TestNativeHookExecHelper$", "--", "native-exec-helper", "child", path}
		if replaceNativeHook(exe, args, nativeHookEnvironment(exe)) != nil {
			os.Exit(2)
		}
		os.Exit(3)
	}
	exe, _ := os.Executable()
	if !exactNativeHookEnvironment(os.Environ(), nativeHookEnvironment(exe)) {
		os.Exit(4)
	}
	if err := os.WriteFile(path, []byte(fmt.Sprint(os.Getpid())), 0600); err != nil {
		os.Exit(5)
	}
	time.Sleep(time.Minute)
	os.Exit(0)
}

// GOC-V0-008: replacement keeps the leader PID and creates no child supervisor.
func TestNativeHookReplacementCanBeInterrupted(t *testing.T) {
	for _, signal := range []os.Signal{os.Interrupt, syscall.SIGTERM} {
		t.Run(signal.String(), func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "pid")
			cmd := exec.Command(os.Args[0], "-test.run=^TestNativeHookExecHelper$", "--", "native-exec-helper", "replace", path)
			if err := cmd.Start(); err != nil {
				t.Fatal(err)
			}
			done := make(chan struct{})
			go func() { _ = cmd.Wait(); close(done) }()
			t.Cleanup(func() {
				_ = cmd.Process.Kill()
				select {
				case <-done:
				case <-time.After(3 * time.Second):
				}
			})
			deadline := time.Now().Add(5 * time.Second)
			for {
				raw, err := os.ReadFile(path)
				if err == nil && len(raw) > 0 {
					if string(raw) != fmt.Sprint(cmd.Process.Pid) {
						t.Fatal("replacement changed PID")
					}
					break
				}
				if time.Now().After(deadline) {
					t.Fatal("replacement did not start")
				}
				time.Sleep(10 * time.Millisecond)
			}
			if err := cmd.Process.Signal(signal); err != nil {
				t.Fatal(err)
			}
			select {
			case <-done:
			case <-time.After(3 * time.Second):
				t.Fatal("replacement survived interruption")
			}
		})
	}
}
