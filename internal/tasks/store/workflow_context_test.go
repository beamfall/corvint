package store

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"runtime"
	"strconv"
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	"github.com/Beamfall/corvint/internal/tasks/supervisor"
	"github.com/Beamfall/corvint/internal/tasks/ticket"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// CAL-V0-036: exercise the real pinned-process boundary, not a query-string mock.
func contextWorkflow(t *testing.T, title string) (*Workflow, string, []byte) {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("POSIX controlled executable")
	}
	root := t.TempDir()
	contextGit(t, root, "init", "-q", "-b", "main")
	contextGit(t, root, "-c", "user.name=fixture", "-c", "user.email=fixture@example.invalid", "commit", "-q", "--allow-empty", "-m", "base")
	id, err := wire.ParseTicketID("ticket", "ticket:fixture:main:OSC-DSP-TV-TAIL")
	if err != nil {
		t.Fatal(err)
	}
	raw := []byte("{\"ok\":true,\"context\":{\"state\":\"READY\",\"revision\":\"" + contextGit(t, root, "rev-parse", "HEAD^{tree}") + "\",\"freshness\":{\"state\":\"fresh\"},\"coverage\":{\"uncertainty\":[\"syntax only; coverage unknown\"]}}}\n")
	if err := os.WriteFile(filepath.Join(root, "context-packet"), raw, 0600); err != nil {
		t.Fatal(err)
	}
	script := []byte("#!/bin/sh\nprintf '%s\\000' \"$@\" > context-args\npwd > context-cwd\ncat context-packet\n")
	path := filepath.Join(t.TempDir(), "core")
	if err := os.WriteFile(path, script, 0700); err != nil {
		t.Fatal(err)
	}
	return &Workflow{cfg: ProgramConfig{CoreExecutable: path, CoreSHA256: supervisor.Digest(script)}, record: &ticket.Record{Title: title, TicketID: id}}, root, raw
}

func TestWorkflowContextIdentityBoundary(t *testing.T) {
	title := "Investigate $(touch escaped) `touch escaped` ;\nquoted \"DSP\""
	w, root, want := contextWorkflow(t, title)
	got, err := w.context(context.Background(), root, "HEAD")
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, want) {
		t.Fatal("raw uncertainty packet changed")
	}
	args, err := os.ReadFile(filepath.Join(root, "context-args"))
	if err != nil {
		t.Fatal(err)
	}
	actual := strings.Split(strings.TrimSuffix(string(args), "\x00"), "\x00")
	expected := []string{"query", "--task", title + " " + w.record.TicketID.Raw, "--budget-bytes", "8000"}
	if !reflect.DeepEqual(actual, expected) {
		t.Fatalf("argv=%q want=%q", actual, expected)
	}
	cwd, err := os.ReadFile(filepath.Join(root, "context-cwd"))
	if err != nil {
		t.Fatal(err)
	}
	// Darwin's /var and /private/var aliases can differ in pwd output.
	resolved, err := filepath.EvalSymlinks(root)
	if err != nil {
		t.Fatal(err)
	}
	actualCwd, err := filepath.EvalSymlinks(strings.TrimSpace(string(cwd)))
	if err != nil {
		t.Fatal(err)
	}
	if resolved != actualCwd {
		t.Fatalf("cwd=%s want=%s", actualCwd, resolved)
	}
	if _, err := os.Stat(filepath.Join(root, "escaped")); !os.IsNotExist(err) {
		t.Fatalf("title evaluated by shell: %v", err)
	}
}

func TestWorkflowContextCombinedRuneBound(t *testing.T) {
	for _, unit := range []string{"a", "界"} {
		for _, size := range []int{8000, 8001} {
			t.Run(unit+"/"+strings.Repeat("over", size-8000), func(t *testing.T) {
				w, root, _ := contextWorkflow(t, "")
				w.record.Title = strings.Repeat(unit, size-1-utf8.RuneCountInString(w.record.TicketID.Raw))
				_, err := w.context(context.Background(), root, "HEAD")
				if (err != nil) != (size > 8000) {
					t.Fatalf("size=%d err=%v", size, err)
				}
				if size > 8000 {
					if _, err := os.Stat(filepath.Join(root, "context-args")); !os.IsNotExist(err) {
						t.Fatal("oversized task invoked Core")
					}
				} else {
					args, err := os.ReadFile(filepath.Join(root, "context-args"))
					if err != nil {
						t.Fatal(err)
					}
					if !bytes.Contains(args, []byte(w.record.Title+" "+w.record.TicketID.Raw)) {
						t.Fatal("task truncated")
					}
				}
			})
		}
	}
}

func TestWorkflowContextRefusesInvalidEvidence(t *testing.T) {
	for _, name := range []string{"missing-pin", "mismatch-pin", "command-failure", "oversized-output", "malformed", "out-of-scope", "stale", "wrong-tree", "invalid-utf8"} {
		t.Run(name, func(t *testing.T) {
			w, root, raw := contextWorkflow(t, "Investigate DSP")
			switch name {
			case "missing-pin":
				w.cfg.CoreExecutable = ""
			case "mismatch-pin":
				w.cfg.CoreSHA256 = strings.Repeat("0", 64)
			case "command-failure":
				script := []byte("#!/bin/sh\nexit 9\n")
				if err := os.WriteFile(w.cfg.CoreExecutable, script, 0700); err != nil {
					t.Fatal(err)
				}
				w.cfg.CoreSHA256 = supervisor.Digest(script)
			case "oversized-output":
				raw = append(raw, bytes.Repeat([]byte(" "), 65537)...)
			case "malformed":
				raw = []byte("{broken")
			case "out-of-scope":
				raw = bytes.ReplaceAll(raw, []byte("READY"), []byte("OUT_OF_SCOPE"))
			case "stale":
				raw = bytes.ReplaceAll(raw, []byte("\"fresh\""), []byte("\"stale\""))
			case "wrong-tree":
				raw = bytes.ReplaceAll(raw, []byte(contextGit(t, root, "rev-parse", "HEAD^{tree}")), []byte(strings.Repeat("0", 40)))
			case "invalid-utf8":
				w.record.Title = "bad\xff"
			}
			if err := os.WriteFile(filepath.Join(root, "context-packet"), raw, 0600); err != nil {
				t.Fatal(err)
			}
			if _, err := w.context(context.Background(), root, "HEAD"); err == nil {
				t.Fatal("invalid context accepted")
			}
		})
	}
}

// CAL-V0-036: opt-in actual Core replay; no fabricated READY provider or model execution.
func TestWorkflowContextActualCoreReplay(t *testing.T) {
	core, root := os.Getenv("CORVINT_CONTEXT_TEST_CORE"), os.Getenv("CORVINT_CONTEXT_TEST_REPO")
	if core == "" || root == "" {
		t.Skip("actual pinned Core/repository supplied by qualification runner")
	}
	raw, err := os.ReadFile(core)
	if err != nil {
		t.Fatal(err)
	}
	id, err := wire.ParseTicketID("ticket", "ticket:oscillux:main:OSC-DSP-TV-TAIL")
	if err != nil {
		t.Fatal(err)
	}
	w := &Workflow{cfg: ProgramConfig{CoreExecutable: core, CoreSHA256: supervisor.Digest(raw)}, record: &ticket.Record{Title: "Characterize and resolve TV DSP tail latency", TicketID: id}}
	original := exec.Command(core, "query", "--task", w.record.Title, "--budget-bytes", "8000")
	original.Dir = root
	originalRaw, err := original.Output()
	if err != nil {
		t.Fatal(err)
	}
	var refused struct {
		Context struct {
			State string `json:"state"`
		} `json:"context"`
	}
	if err = json.Unmarshal(originalRaw, &refused); err != nil || refused.Context.State != "OUT_OF_SCOPE" {
		t.Fatalf("original refusal changed: %v %s", err, originalRaw)
	}
	if absent := os.Getenv("CORVINT_CONTEXT_TEST_ABSENT_REPO"); absent != "" {
		if _, err := w.context(context.Background(), absent, "HEAD"); err == nil {
			t.Fatal("absent implementation became READY")
		}
	}
	packet, err := w.context(context.Background(), root, "HEAD")
	if err != nil {
		t.Fatal(err)
	}
	var result struct {
		Context struct {
			Results []struct {
				ID string `json:"id"`
			} `json:"results"`
			Coverage struct {
				Uncertainty []string `json:"uncertainty"`
			} `json:"coverage"`
		} `json:"context"`
	}
	if err = json.Unmarshal(packet, &result); err != nil {
		t.Fatal(err)
	}
	relevant := false
	for _, r := range result.Context.Results {
		if strings.HasPrefix(r.ID, "crates/oscillux-") {
			relevant = true
		}
	}
	if !relevant || len(result.Context.Coverage.Uncertainty) == 0 {
		t.Fatal("relevant source or retained uncertainty absent")
	}
}

func contextGit(t *testing.T, root string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-C", root}, args...)...)
	cmd.Env = append(os.Environ(), "GIT_CONFIG_GLOBAL="+os.DevNull, "GIT_CONFIG_NOSYSTEM=1")
	raw, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v %s", args, err, raw)
	}
	return strings.TrimSpace(string(raw))
}

// V1-0391: a Core query that emits a valid READY packet while a descendant
// keeps stdout open is refused promptly, not accepted after the descendant
// lets go past the 30-second context deadline.
func TestWorkflowContextRefusesHeldOutputPipe(t *testing.T) {
	w, root, _ := contextWorkflow(t, "Investigate DSP")
	pidPath := filepath.Join(t.TempDir(), "descendant.pid")
	script := []byte("#!/bin/sh\ncat context-packet\nsleep 45 &\necho $! > '" + pidPath + "'\n")
	if err := os.WriteFile(w.cfg.CoreExecutable, script, 0700); err != nil {
		t.Fatal(err)
	}
	w.cfg.CoreSHA256 = supervisor.Digest(script)
	t.Cleanup(func() {
		if raw, err := os.ReadFile(pidPath); err == nil {
			if pid, err := strconv.Atoi(strings.TrimSpace(string(raw))); err == nil && pid > 1 {
				if process, err := os.FindProcess(pid); err == nil {
					_ = process.Kill()
				}
			}
		}
	})
	began := time.Now()
	if _, err := w.context(context.Background(), root, "HEAD"); err == nil {
		t.Fatal("context accepted while a descendant held the output pipe")
	}
	if elapsed := time.Since(began); elapsed > 20*time.Second {
		t.Fatalf("held pipe refused only after %s", elapsed)
	}
}
