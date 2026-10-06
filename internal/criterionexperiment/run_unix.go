//go:build darwin || linux

package criterionexperiment

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sync"
	"syscall"
	"time"

	"github.com/Beamfall/corvint/internal/groupreap"
)

type boundedBuffer struct {
	mu       sync.Mutex
	b        bytes.Buffer
	overflow bool
	cancel   context.CancelFunc
}

func (b *boundedBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	n := len(p)
	remaining := (1 << 20) - b.b.Len()
	if len(p) > remaining {
		b.overflow = true
		if b.cancel != nil {
			b.cancel()
		}
		p = p[:remaining]
	}
	_, _ = b.b.Write(p)
	return n, nil
}
func runScenario(ctx context.Context, r Request, c Criterion, files map[string][]byte, modes map[string]string, dir string) ([]byte, []byte, int, bool, error) {
	if !filepath.IsAbs(r.GoBinary) {
		return nil, nil, 255, false, fmt.Errorf("Go binary must be absolute")
	}
	binary, e := os.ReadFile(r.GoBinary)
	if e != nil || Digest(binary) != r.GoSha256 {
		return nil, nil, 255, false, fmt.Errorf("Go executable binding changed")
	}
	for p, b := range files {
		full := filepath.Join(dir, "src", filepath.FromSlash(p))
		if e = os.MkdirAll(filepath.Dir(full), 0700); e != nil {
			return nil, nil, 255, false, e
		}
		mode := os.FileMode(0644)
		if modes[p] == "100755" {
			mode = 0755
		}
		if e = os.WriteFile(full, b, mode); e != nil {
			return nil, nil, 255, false, e
		}
		if e = os.Chmod(full, mode); e != nil {
			return nil, nil, 255, false, e
		}
	}
	for _, p := range []string{"home", "cache", "tmp", "gopath"} {
		if e = os.Mkdir(filepath.Join(dir, p), 0700); e != nil {
			return nil, nil, 255, false, e
		}
	}
	limited, cancel := context.WithTimeout(ctx, time.Duration(r.TimeoutSeconds)*time.Second)
	defer cancel()
	pkg := "."
	if c.Package != "." {
		pkg = "./" + c.Package
	}
	command := exec.CommandContext(limited, r.GoBinary, "test", "-json", "-count=1", "-timeout", fmt.Sprintf("%ds", r.TimeoutSeconds), "-run", "^"+regexp.QuoteMeta(c.Test)+"$", pkg)
	command.Dir = filepath.Join(dir, "src")
	command.Env = []string{"PATH=" + filepath.Dir(r.GoBinary), "GOTOOLCHAIN=local", "GOWORK=off", "GOPROXY=off", "GOSUMDB=off", "GOENV=off", "CGO_ENABLED=0", "HOME=" + filepath.Join(dir, "home"), "GOPATH=" + filepath.Join(dir, "gopath"), "GOCACHE=" + filepath.Join(dir, "cache"), "TMPDIR=" + filepath.Join(dir, "tmp")}
	ownGroup(command)
	stdout, stderr := boundedBuffer{cancel: cancel}, boundedBuffer{cancel: cancel}
	command.Stdout = &stdout
	command.Stderr = &stderr
	e = groupreap.Run(command)
	exit := 255
	if command.ProcessState != nil && command.ProcessState.ExitCode() >= 0 {
		exit = command.ProcessState.ExitCode()
	}
	complete := limited.Err() == nil && !stdout.overflow && !stderr.overflow && (e == nil || exit == 1)
	return stdout.b.Bytes(), stderr.b.Bytes(), exit, complete, nil
}

// ownGroup puts a child in its own process group, kills that group on
// cancellation and bounds the pipe drain after the child exits.
func ownGroup(command *exec.Cmd) {
	command.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	command.Cancel = func() error {
		if command.Process != nil {
			return syscall.Kill(-command.Process.Pid, syscall.SIGKILL)
		}
		return nil
	}
	// The bound detects a descendant holding the output pipes, not a slow reader (V1-0391).
	command.WaitDelay = time.Minute
}
