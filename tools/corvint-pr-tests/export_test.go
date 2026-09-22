//go:build darwin || linux

package main

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"encoding/base64"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"
)

// Actual GNU tar1.35 output from the pinned image's tmpfs probe. Raw SHA256
// d4e22fbb007656977a1121bdb286defacda1ac83e7028e0b1c7d55b1e4c8a84e.
const actualExportArchive = "H4sIAAAAAAAC/+3RQQrCMBRF0T92FdlAJT9t0vVE6aAgpqRf6fLNRGjnIoL3TO7gDd+11Od8t27allKtW2q5TGfbTD7IN2kYxOvY7PoWetEYoib10ae2h1F7cV6+4LFars7Jn8p2y+vh/ZMAAAAAAAAAAAAAAAAAAH7fC99Q10UAKAAA"

func exportArchive(t *testing.T) []byte {
	t.Helper()
	b, err := base64.StdEncoding.DecodeString(actualExportArchive)
	if err != nil {
		t.Fatal(err)
	}
	z, err := gzip.NewReader(bytes.NewReader(b))
	if err != nil {
		t.Fatal(err)
	}
	defer z.Close()
	out, err := io.ReadAll(io.LimitReader(z, 10241))
	if err != nil || len(out) != 10240 {
		t.Fatal("invalid frozen archive")
	}
	return out
}
func TestContainerExportPadding(t *testing.T) {
	t.Run("AFP-V0-015", func(t *testing.T) {
		archive := exportArchive(t)
		var pax bytes.Buffer
		writer := tar.NewWriter(&pax)
		if err := writer.WriteHeader(&tar.Header{Name: "empty", Mode: 0600, Size: 0, Format: tar.FormatPAX, PAXRecords: map[string]string{"comment": strings.Repeat("x", 20<<10)}}); err != nil {
			t.Fatal(err)
		}
		if err := writer.Close(); err != nil {
			t.Fatal(err)
		}
		if err := copyRegularTar(io.Discard, bytes.NewReader(pax.Bytes()), "empty", 8<<20); err == nil {
			t.Fatal("oversized PAX framing admitted for empty member")
		}
		r, w := io.Pipe()
		defer r.Close()
		defer w.Close()
		done := make(chan error, 1)
		go func() { _, err := io.Copy(w, bytes.NewReader(archive)); w.Close(); done <- err }()
		var out bytes.Buffer
		if err := copyRegularTar(&out, r, "corvint-export-probe.txt", 19); err != nil {
			t.Fatal(err)
		}
		select {
		case err := <-done:
			if err != nil {
				t.Fatal(err)
			}
		case <-time.After(time.Second):
			r.Close()
			<-done
			t.Fatal("tar padding not drained before completion")
		}
		for _, tail := range [][]byte{{1}, make([]byte, 32<<10)} {
			if err := copyRegularTar(io.Discard, bytes.NewReader(append(append([]byte{}, archive...), tail...)), "corvint-export-probe.txt", 19); err == nil {
				t.Fatal("unsafe trailing archive admitted")
			}
		}
	})
}
func TestContainerExportTransport(t *testing.T) {
	t.Run("AFP-V0-015", func(t *testing.T) {
		dir := t.TempDir()
		archive := filepath.Join(dir, "archive")
		os.WriteFile(archive, exportArchive(t), 0600)
		script := "#!/bin/sh\nif [ \"$3\" != exec ]; then echo tmpfs-cp-unavailable >&2; exit 1; fi\ncat '" + archive + "'\n"
		if err := os.WriteFile(filepath.Join(dir, "docker"), []byte(script), 0700); err != nil {
			t.Fatal(err)
		}
		t.Setenv("PATH", dir+":"+os.Getenv("PATH"))
		dest := filepath.Join(dir, "evidence")
		if err := copyContainerFile(context.Background(), "fixture", "owned", "/work/corvint-export-probe.txt", dest, 19); err != nil {
			t.Fatal(err)
		}
		b, err := os.ReadFile(dest)
		if err != nil || len(b) != 19 {
			t.Fatal("evidence not retained")
		}
		os.WriteFile(filepath.Join(dir, "docker"), []byte("#!/bin/sh\necho diagnostic-witness >&2\nexit 1\n"), 0700)
		err = copyContainerFile(context.Background(), "fixture", "owned", "/work/corvint-export-probe.txt", filepath.Join(dir, "failed"), 19)
		if err == nil || !strings.Contains(err.Error(), "diagnostic-witness") {
			t.Fatalf("lost export stderr: %v", err)
		}
		if _, e := os.Stat(filepath.Join(dir, "failed")); !os.IsNotExist(e) {
			t.Fatal("partial evidence survived")
		}
		os.WriteFile(filepath.Join(dir, "docker"), []byte("#!/bin/sh\nhead -c 2097152 /dev/zero >&2\n"), 0700)
		if err := copyContainerFile(context.Background(), "fixture", "owned", "/work/corvint-export-probe.txt", filepath.Join(dir, "overflow"), 19); err == nil || len(err.Error()) > (1<<20)+1024 {
			t.Fatal("export stderr not bounded")
		}

	})
}

func TestContainerRunResult(t *testing.T) {
	t.Run("AFP-V0-015", func(t *testing.T) {
		dir := t.TempDir()
		o := options{base: strings.Repeat("a", 40), head: strings.Repeat("b", 40), target: strings.Repeat("c", 40), source: strings.Repeat("d", 40)}
		p := containerProfile{Source: o.source, Tools: map[string]string{"corvint": "planner", "gate-affected-select": "selector", "corvint-pr-tests": "driver"}}
		id := identity{GoVersion: "go1.27.0", OS: "linux", Arch: "amd64", GoBinary: "go-hash", Compiler: "cc-hash", Source: o.source, Container: &p, Driver: "driver", Planner: "planner", Selector: "selector", Args: testArgs, Env: []string{"GIT_CONFIG_GLOBAL=/dev/null"}}
		s := selection{Schema: schema, Base: o.base, Head: o.head, Target: o.target, Tree: strings.Repeat("e", 40), ObservedTarget: o.target, ObservedTree: strings.Repeat("e", 40), Identity: id, Packages: []string{"./..."}, Reason: "qualification unavailable"}
		e := execution{Selection: s, Exit: 1, Env: id.Env, Args: append(append([]string{}, testArgs...), s.Packages...)}
		failure := exec.Command("sh", "-c", "exit 1").Run()
		driverError := exec.Command("sh", "-c", "exit 2").Run()
		write := func() {
			t.Helper()
			writeJSON(filepath.Join(dir, "selection.json"), s)
			writeJSON(filepath.Join(dir, "execution.json"), e)
			os.WriteFile(filepath.Join(dir, "go.stderr"), nil, 0600)
			os.WriteFile(filepath.Join(dir, "go.json"), []byte("{\"Action\":\"fail\",\"Package\":\"example.org/fixture/a\"}\n"), 0600)
		}
		write()
		if code, err := containerRunResult(o, p, dir, nil, failure); code != 1 || err != nil {
			t.Fatalf("test failure misclassified: %d %v", code, err)
		}
		for _, mutate := range []func(){
			func() { os.Remove(filepath.Join(dir, "execution.json")) },
			func() { os.WriteFile(filepath.Join(dir, "go.json"), []byte("invalid"), 0600) },
			func() {
				os.WriteFile(filepath.Join(dir, "go.json"), []byte("{\"Action\":\"build-fail\",\"Package\":\"example.org/fixture/a\"}\n"), 0600)
			},
			func() { wrong := e; wrong.Error = "cancelled"; writeJSON(filepath.Join(dir, "execution.json"), wrong) },
			func() {
				wrong := e
				wrong.Args = []string{"test", "./..."}
				writeJSON(filepath.Join(dir, "execution.json"), wrong)
			},
			func() { wrong := s; wrong.Target = o.base; writeJSON(filepath.Join(dir, "selection.json"), wrong) },
		} {
			write()
			mutate()
			if code, err := containerRunResult(o, p, dir, nil, failure); code != 2 || err == nil {
				t.Fatal("invalid evidence admitted")
			}
		}
		write()
		for _, err := range []error{driverError, context.Canceled, context.DeadlineExceeded} {
			if code, _ := containerRunResult(o, p, dir, nil, err); code != 2 {
				t.Fatal("infrastructure exit admitted")
			}
		}
		if code, _ := containerRunResult(o, p, dir, map[string]string{"go.stderr": "missing"}, failure); code != 2 {
			t.Fatal("export error admitted")
		}
		e.Exit = 0
		write()
		os.WriteFile(filepath.Join(dir, "go.json"), []byte("{\"Action\":\"pass\",\"Package\":\"example.org/fixture/a\"}\n"), 0600)
		if code, err := containerRunResult(o, p, dir, nil, nil); code != 0 || err != nil {
			t.Fatalf("success: %d %v", code, err)
		}
		os.Remove(filepath.Join(dir, "execution.json"))
		if code, _ := containerRunResult(o, p, dir, nil, nil); code != 2 {
			t.Fatal("unproven success admitted")
		}
	})
}

func TestContainerExportInterruption(t *testing.T) {
	t.Run("AFP-V0-015", func(t *testing.T) {
		dir := t.TempDir()
		pidfile := filepath.Join(dir, "pid")
		script := "#!/bin/sh\nsleep 60 &\necho $! > '" + pidfile + "'\nwait\n"
		os.WriteFile(filepath.Join(dir, "docker"), []byte(script), 0700)
		t.Setenv("PATH", dir+":"+os.Getenv("PATH"))
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		done := make(chan error, 1)
		dest := filepath.Join(dir, "out")
		go func() { done <- copyContainerFile(ctx, "fixture", "owned", "/work/one", dest, 19) }()
		pid := 0
		deadline := time.Now().Add(5 * time.Second)
		for time.Now().Before(deadline) {
			b, _ := os.ReadFile(pidfile)
			pid, _ = strconv.Atoi(strings.TrimSpace(string(b)))
			if pid > 0 {
				break
			}
			time.Sleep(10 * time.Millisecond)
		}
		if pid == 0 {
			cancel()
			<-done
			t.Fatal("no export descendant")
		}
		defer syscall.Kill(pid, syscall.SIGKILL)
		cancel()
		select {
		case err := <-done:
			if err == nil {
				t.Fatal("interruption passed")
			}
		case <-time.After(5 * time.Second):
			t.Fatal("export survived cancellation")
		}
		if _, err := os.Stat(dest); !os.IsNotExist(err) {
			t.Fatal("partial interrupted export retained")
		}
		// The group kill is delivered before Wait returns, but the descendant can
		// still be exiting: poll, and count an unreaped zombie as gone.
		gone := func() bool {
			if syscall.Kill(pid, 0) == syscall.ESRCH {
				return true
			}
			b, _ := exec.Command("ps", "-o", "stat=", "-p", strconv.Itoa(pid)).Output()
			return strings.HasPrefix(strings.TrimSpace(string(b)), "Z")
		}
		deadline = time.Now().Add(5 * time.Second)
		for !gone() && time.Now().Before(deadline) {
			time.Sleep(10 * time.Millisecond)
		}
		if !gone() {
			t.Fatal("export descendant survived")
		}
	})
}
