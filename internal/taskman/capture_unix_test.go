//go:build darwin || linux

package taskman

import (
	"bytes"
	"context"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/Beamfall/corvint/internal/cem/wire"
)

type adapterFixture struct {
	root, executor, observations, responses string
	snapshot                                wire.Value
}

func writeTest(t *testing.T, path string, raw []byte, mode fs.FileMode) {
	t.Helper()
	if e := os.MkdirAll(filepath.Dir(path), 0755); e != nil {
		t.Fatal(e)
	}
	if e := os.WriteFile(path, raw, mode); e != nil {
		t.Fatal(e)
	}
}
func gitTest(t *testing.T, root string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-c", "core.hooksPath=/dev/null", "-c", "user.name=Fixture", "-c", "user.email=fixture@example.invalid", "-C", root}, args...)...)
	cmd.Env = append(os.Environ(), "GIT_CONFIG_GLOBAL=/dev/null", "GIT_CONFIG_NOSYSTEM=1")
	raw, e := cmd.CombinedOutput()
	if e != nil {
		t.Fatalf("git %v: %v %s", args, e, raw)
	}
	return strings.TrimSpace(string(raw))
}
func newAdapterFixture(t *testing.T) adapterFixture {
	t.Helper()
	base, e := filepath.EvalSymlinks(t.TempDir())
	if e != nil {
		t.Fatal(e)
	}
	f := adapterFixture{root: filepath.Join(base, "repo"), executor: filepath.Join(base, "executor"), observations: filepath.Join(base, "observations.json"), responses: filepath.Join(base, "responses")}
	if e = os.Mkdir(f.root, 0755); e != nil {
		t.Fatal(e)
	}
	for _, name := range []string{"policy", "queue"} {
		raw, e := os.ReadFile("testdata/" + name + ".json")
		if e != nil {
			t.Fatal(e)
		}
		writeTest(t, filepath.Join(f.root, ".taskman", name+".json"), raw, 0644)
	}
	exported := testDocument(t, "tickets")
	for _, item := range value(exported, "items").Arr {
		writeTest(t, filepath.Join(f.root, ".taskman", stringAt(item, "path")), append(canonical(value(item, "record")), '\n'), 0644)
	}
	writeTest(t, filepath.Join(f.root, "go.mod"), []byte("module example.com/fixture\n\ngo 1.27.1\n"), 0644)
	writeTest(t, filepath.Join(f.root, "a.go"), []byte("package fixture\nfunc A() {}\n"), 0644)
	writeTest(t, filepath.Join(f.root, "b.go"), []byte("package fixture\nfunc B() {}\n"), 0644)
	gitTest(t, f.root, "init", "-q", "-b", "main")
	gitTest(t, f.root, "add", "-A")
	gitTest(t, f.root, "commit", "-qm", "Fixture")
	for _, name := range []string{"audit", "status", "tickets"} {
		v := testDocument(t, name)
		snap := value(v, "snapshot")
		setString(snap, "primaryWorktreeSha256", sum([]byte(f.root)))
		f.snapshot = snap
		writeTest(t, filepath.Join(f.responses, name+".json"), append(canonical(v), '\n'), 0644)
	}
	script := "#!/bin/sh\ncase \"$1 $2\" in\n'receipt audit') cat '" + f.responses + "/audit.json';;\n'queue status') cat '" + f.responses + "/status.json';;\n'ticket export') cat '" + f.responses + "/tickets.json';;\n*) exit 1;;\nesac\n"
	writeTest(t, f.executor, []byte(script), 0755)
	c := testCapture(t)
	v := observationValue(t, c)
	setString(v, "sourceCommit", gitTest(t, f.root, "rev-parse", "HEAD"))
	setString(v, "sourceTree", gitTest(t, f.root, "rev-parse", "HEAD^{tree}"))
	writeTest(t, f.observations, append(canonical(v), '\n'), 0644)
	return f
}
func treeBytes(t *testing.T, root string) map[string]string {
	t.Helper()
	r := map[string]string{}
	e := filepath.WalkDir(root, func(p string, d fs.DirEntry, e error) error {
		if e != nil {
			return e
		}
		if !d.IsDir() {
			raw, e := os.ReadFile(p)
			if e != nil {
				return e
			}
			relative, _ := filepath.Rel(root, p)
			r[relative] = sum(raw)
		}
		return nil
	})
	if e != nil {
		t.Fatal(e)
	}
	return r
}
func TestNTPV0001AdapterCanonicalReadOnly(t *testing.T) {
	t.Run("NTP-V0-008 AdapterCanonicalReadOnly", func(t *testing.T) {
		f := newAdapterFixture(t)
		before := treeBytes(t, f.root)
		first, e := Preview(context.Background(), f.root, f.executor, f.observations)
		if e != nil {
			t.Fatal(e)
		}
		second, e := Preview(context.Background(), f.root, f.executor, f.observations)
		if e != nil || !bytes.Equal(first, second) {
			t.Fatal("replay", e)
		}
		after := treeBytes(t, f.root)
		if len(after) != len(before) {
			t.Fatal("file count changed")
		}
		for p, h := range before {
			if after[p] != h {
				t.Fatal("changed", p)
			}
		}
		v, e := document(first, 16<<20)
		if e != nil {
			t.Fatal(e)
		}
		entries := value(value(v, "plan"), "entries").Arr
		if len(entries) != 3 || stringAt(entries[0], "state") != "SELECTED" || stringAt(entries[1], "state") != "DEFERRED" {
			t.Fatal("priority proof", string(first))
		}
	})
}
func changeResponse(t *testing.T, f adapterFixture, name string, change func(wire.Value)) {
	t.Helper()
	path := filepath.Join(f.responses, name+".json")
	raw, e := os.ReadFile(path)
	if e != nil {
		t.Fatal(e)
	}
	v, e := document(raw, 16<<20)
	if e != nil {
		t.Fatal(e)
	}
	change(v)
	writeTest(t, path, append(canonical(v), '\n'), 0644)
}
func TestNTPV0001AdapterRefusals(t *testing.T) {
	t.Run("NTP-V0-001 AdapterRefusals", func(t *testing.T) {
		cases := []struct {
			name, response string
			change         func(wire.Value)
		}{
			{"non fixture", "status", func(v wire.Value) { setBool(value(v, "items").Arr[0], "fixture", false) }},
			{"moved head", "status", func(v wire.Value) { setString(value(v, "snapshot"), "headSeq", "5") }},
			{"pending", "audit", func(v wire.Value) { setBool(value(v, "snapshot"), "pendingRedo", true) }},
			{"staging", "audit", func(v wire.Value) { setBool(value(v, "items").Arr[0], "stagingPresent", true) }},
			{"unknown codec", "audit", func(v wire.Value) { setString(value(v, "items").Arr[0], "semanticCoverage", "UNKNOWN") }},
			{"bad record digest", "tickets", func(v wire.Value) { setString(value(v, "items").Arr[0], "sha256", strings.Repeat("0", 64)) }},
			{"bad record length", "tickets", func(v wire.Value) { setString(value(v, "items").Arr[0], "bytes", "1") }},
			{"wrong offset", "tickets", func(v wire.Value) { setString(value(v, "page"), "offset", "1") }},
			{"truncated no progress", "tickets", func(v wire.Value) {
				v.Obj.Values["items"] = testValue(t, []any{})
				setBool(value(v, "page"), "truncated", true)
			}},
			{"early final page", "tickets", func(v wire.Value) { v.Obj.Values["items"].Arr[2] = v.Obj.Values["items"].Arr[0] }},
			{"wrong policy", "status", func(v wire.Value) { setString(value(v, "items").Arr[0], "policySha256", strings.Repeat("0", 64)) }},
			{"foreign command", "tickets", func(v wire.Value) { v.Obj.Values["command"] = testValue(t, []string{"ticket", "list"}) }},
		}
		for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				f := newAdapterFixture(t)
				before := treeBytes(t, f.root)
				changeResponse(t, f, tc.response, tc.change)
				raw, e := Preview(context.Background(), f.root, f.executor, f.observations)
				if e == nil || len(raw) != 0 {
					t.Fatal("refusal leaked plan", string(raw), e)
				}
				after := treeBytes(t, f.root)
				for p, h := range before {
					if after[p] != h {
						t.Fatal("changed source", p)
					}
				}
			})
		}
	})
}
func TestNTPV0002AdapterFinalDrift(t *testing.T) {
	t.Run("NTP-V0-002 AdapterFinalDrift", func(t *testing.T) {
		f := newAdapterFixture(t)
		script, e := os.ReadFile(f.executor)
		if e != nil {
			t.Fatal(e)
		}
		s := strings.Replace(string(script), "'ticket export') cat ", "'ticket export') printf 'changed' >> '"+filepath.Join(f.root, "a.go")+"'; cat ", 1)
		writeTest(t, f.executor, []byte(s), 0755)
		if raw, e := Preview(context.Background(), f.root, f.executor, f.observations); e == nil || len(raw) > 0 {
			t.Fatal("source drift admitted")
		}
	})
}
func TestNTPV0001InterruptionRetiresDescendant(t *testing.T) {
	t.Run("NTP-V0-001 InterruptionRetiresDescendant", func(t *testing.T) {
		base, e := filepath.EvalSymlinks(t.TempDir())
		if e != nil {
			t.Fatal(e)
		}
		script, pidfile := filepath.Join(base, "runner"), filepath.Join(base, "pid")
		writeTest(t, script, []byte("#!/bin/sh\nsleep 30 &\necho $! > '"+pidfile+"'\nwait\n"), 0755)
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		done := make(chan error, 1)
		go func() { _, e := runRead(ctx, script, base, nil); done <- e }()
		var pid int
		for deadline := time.Now().Add(3 * time.Second); time.Now().Before(deadline); {
			raw, _ := os.ReadFile(pidfile)
			pid, _ = strconv.Atoi(strings.TrimSpace(string(raw)))
			if pid > 0 {
				break
			}
			time.Sleep(10 * time.Millisecond)
		}
		if pid == 0 {
			t.Fatal("descendant did not start")
		}
		cancel()
		select {
		case e := <-done:
			if e == nil {
				t.Fatal("cancel succeeded")
			}
		case <-time.After(3 * time.Second):
			t.Fatal("runner hung")
		}
		for deadline := time.Now().Add(3 * time.Second); time.Now().Before(deadline); {
			if syscall.Kill(pid, 0) == syscall.ESRCH {
				return
			}
			time.Sleep(10 * time.Millisecond)
		}
		t.Fatal("descendant survived interruption")
	})
}
func TestNTPV0001BoundedRegularInputs(t *testing.T) {
	t.Run("NTP-V0-001 BoundedRegularInputs", func(t *testing.T) {
		dir := t.TempDir()
		file := filepath.Join(dir, "regular")
		writeTest(t, file, []byte("12345"), 0644)
		if _, e := regular(file, 4); e == nil {
			t.Fatal("oversize")
		}
		link := filepath.Join(dir, "link")
		if e := os.Symlink(file, link); e != nil {
			t.Fatal(e)
		}
		if _, e := regular(link, 10); e == nil {
			t.Fatal("symlink")
		}
		fifo := filepath.Join(dir, "fifo")
		if e := syscall.Mkfifo(fifo, 0600); e != nil {
			t.Fatal(e)
		}
		if _, e := regular(fifo, 10); e == nil {
			t.Fatal("fifo")
		}
	})
}
