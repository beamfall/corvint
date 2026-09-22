package main

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"
)

type processResult struct {
	DurationNS int64 `json:"durationNs"`
	ErrorCode  any   `json:"errorCode"`
	ExitCode   any   `json:"exitCode"`
	TimedOut   bool  `json:"timedOut"`
}
type boundedCapture struct {
	mu       sync.Mutex
	buf      bytes.Buffer
	limit    int
	overflow bool
}

func (c *boundedCapture) Write(p []byte) (int, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	remaining := c.limit + 1 - c.buf.Len()
	if remaining > len(p) {
		remaining = len(p)
	}
	if remaining > 0 {
		_, _ = c.buf.Write(p[:remaining])
	}
	if c.buf.Len() > c.limit {
		c.overflow = true
	}
	return len(p), nil
}
func (c *boundedCapture) bytes() []byte {
	c.mu.Lock()
	defer c.mu.Unlock()
	raw := c.buf.Bytes()
	if len(raw) > c.limit {
		raw = raw[:c.limit]
	}
	return append([]byte(nil), raw...)
}

func minimalEnvironment(home string, extra map[string]string) []string {
	m := map[string]string{"GIT_CONFIG_GLOBAL": "/dev/null", "GIT_CONFIG_NOSYSTEM": "1", "GIT_NO_LAZY_FETCH": "1", "GIT_OPTIONAL_LOCKS": "0", "GIT_TERMINAL_PROMPT": "0", "HOME": home, "LANG": "C", "LC_ALL": "C", "PATH": os.Getenv("PATH"), "TMPDIR": home}
	if m["PATH"] == "" {
		m["PATH"] = "/usr/bin:/bin"
	}
	for k, v := range extra {
		m[k] = v
	}
	out := make([]string, 0, len(m))
	for k, v := range m {
		out = append(out, k+"="+v)
	}
	sortStrings(out)
	return out
}
func sortStrings(v []string) {
	for i := 1; i < len(v); i++ {
		for j := i; j > 0 && v[j] < v[j-1]; j-- {
			v[j], v[j-1] = v[j-1], v[j]
		}
	}
}

func runProcess(parent context.Context, argv []string, cwd string, environment []string, timeout time.Duration, limit int) (processResult, []byte, []byte, error) {
	started := time.Now()
	if runtime.GOOS == "windows" {
		return processResult{}, nil, nil, fail("posix-required")
	}
	ctx, cancel := context.WithTimeout(parent, timeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, argv[0], argv[1:]...)
	cmd.Dir = cwd
	cmd.Env = append([]string(nil), environment...)
	cmd.Stdin = bytes.NewReader(nil)
	setProcessGroup(cmd)
	cmd.WaitDelay = 500 * time.Millisecond
	stdout, stderr := &boundedCapture{limit: limit}, &boundedCapture{limit: limit}
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	cmd.Cancel = func() error {
		if cmd.Process == nil {
			return nil
		}
		return killProcessGroup(cmd)
	}
	if err := cmd.Start(); err != nil {
		return processResult{DurationNS: time.Since(started).Nanoseconds(), ErrorCode: "spawn-failed", ExitCode: nil, TimedOut: false}, nil, nil, nil
	}
	err := cmd.Wait()
	_ = killProcessGroup(cmd)
	if parent.Err() != nil {
		return processResult{DurationNS: time.Since(started).Nanoseconds(), ErrorCode: "interrupted", ExitCode: cmd.ProcessState.ExitCode(), TimedOut: false}, stdout.bytes(), stderr.bytes(), fail("interrupted")
	}
	result := processResult{DurationNS: time.Since(started).Nanoseconds(), ErrorCode: nil, ExitCode: cmd.ProcessState.ExitCode(), TimedOut: false}
	if stdout.overflow {
		result.ErrorCode = "stdout-bound-exceeded"
	} else if stderr.overflow {
		result.ErrorCode = "stderr-bound-exceeded"
	} else if ctx.Err() == context.DeadlineExceeded {
		result.ErrorCode = "timeout"
		result.TimedOut = true
	} else if ctx.Err() == context.Canceled {
		result.ErrorCode = "interrupted"
	} else if errors.Is(err, exec.ErrWaitDelay) {
		result.ErrorCode = "capture-did-not-close"
	} else if err != nil {
		var exit *exec.ExitError
		if !errors.As(err, &exit) {
			return result, stdout.bytes(), stderr.bytes(), fail("process-wait")
		}
	}
	return result, stdout.bytes(), stderr.bytes(), nil
}

func gitCommand(ctx context.Context, repo, home string, dated *string, args ...string) (string, error) {
	extra := map[string]string{}
	if dated != nil {
		extra["GIT_AUTHOR_DATE"] = *dated
		extra["GIT_COMMITTER_DATE"] = *dated
	}
	argv := []string{"git", "-C", repo, "-c", "core.hooksPath=/dev/null", "-c", "credential.helper=", "-c", "protocol.file.allow=never"}
	argv = append(argv, args...)
	result, stdout, _, err := runProcess(ctx, argv, filepath.Dir(repo), minimalEnvironment(home, extra), 20*time.Second, captureLimit)
	if err != nil {
		return "", err
	}
	if result.ErrorCode != nil || asInt(result.ExitCode) != 0 {
		return "", fail("git-command")
	}
	if !utf8ASCII(stdout) {
		return "", fail("git-output")
	}
	return strings.TrimSpace(string(stdout)), nil
}
func asInt(v any) int {
	if i, ok := v.(int); ok {
		return i
	}
	return -999
}
func utf8ASCII(v []byte) bool {
	for _, b := range v {
		if b > 127 {
			return false
		}
	}
	return true
}
func copyReader(dst io.Writer, src io.Reader) { _, _ = io.Copy(dst, src) }
