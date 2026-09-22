package doccompiler

import (
	"bytes"
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

type boundedBuffer struct {
	data     bytes.Buffer
	limit    int
	exceeded bool
	cancel   context.CancelFunc
}

func (buffer *boundedBuffer) Write(value []byte) (int, error) {
	written := len(value)
	remaining := buffer.limit - buffer.data.Len()
	if remaining < 0 {
		remaining = 0
	}
	if len(value) > remaining {
		_, _ = buffer.data.Write(value[:remaining])
		buffer.exceeded = true
		if buffer.cancel != nil {
			buffer.cancel()
		}
		return written, nil
	}
	_, _ = buffer.data.Write(value)
	return written, nil
}

func (buffer *boundedBuffer) String() string { return buffer.data.String() }

type commandResult struct {
	stdout string
	stderr string
}

func isolatedEnvironment(toolPath, stateRoot string) []string {
	environment := []string{
		"PATH=" + filepath.Dir(toolPath),
		"HOME=" + stateRoot,
		"XDG_CACHE_HOME=" + filepath.Join(stateRoot, "cache"),
		"XDG_CONFIG_HOME=" + filepath.Join(stateRoot, "config"),
		"XDG_DATA_HOME=" + filepath.Join(stateRoot, "data"),
		"TMPDIR=" + filepath.Join(stateRoot, "tmp"),
		"TEMP=" + filepath.Join(stateRoot, "tmp"),
		"TMP=" + filepath.Join(stateRoot, "tmp"),
		"LANG=C.UTF-8", "LC_ALL=C.UTF-8", "TZ=UTC",
		"PYTHONNOUSERSITE=1", "PYTHONDONTWRITEBYTECODE=1",
		"PIP_NO_INDEX=1", "PIP_DISABLE_PIP_VERSION_CHECK=1",
		"HTTP_PROXY=", "HTTPS_PROXY=", "ALL_PROXY=", "NO_PROXY=*",
		"http_proxy=", "https_proxy=", "all_proxy=", "no_proxy=*",
	}
	if value, exists := os.LookupEnv("SystemRoot"); exists {
		environment = append(environment, "SystemRoot="+value)
	}
	return environment
}

func runCommand(ctx context.Context, executable string, arguments []string, directory string, environment []string, stdoutLimit, stderrLimit int) (commandResult, error) {
	commandContext, cancel := context.WithCancel(ctx)
	defer cancel()
	command := exec.CommandContext(commandContext, executable, arguments...)
	command.Dir = directory
	command.Env = environment
	configureProcess(command)
	stdout := &boundedBuffer{limit: stdoutLimit, cancel: cancel}
	stderr := &boundedBuffer{limit: stderrLimit, cancel: cancel}
	command.Stdout = stdout
	command.Stderr = stderr
	if err := command.Start(); err != nil {
		if ctx.Err() != nil {
			return commandResult{}, failure("command-cancelled", "command was cancelled before start")
		}
		return commandResult{}, failure("command-start-failed", "cannot start pinned command")
	}
	processID := command.Process.Pid
	defer terminateProcessGroup(processID)
	waitErr := command.Wait()
	result := commandResult{stdout: strings.TrimSpace(stdout.String()), stderr: strings.TrimSpace(stderr.String())}
	if stdout.exceeded || stderr.exceeded {
		return result, failure("command-output-too-large", "command output exceeds its stdout or stderr byte limit")
	}
	if ctx.Err() != nil {
		return result, failure("command-timeout", "command exceeded its deadline")
	}
	if waitErr != nil {
		var exitError *exec.ExitError
		if errors.As(waitErr, &exitError) {
			return result, failure("command-failed", "pinned command exited with status %d", exitError.ExitCode())
		}
		return result, failure("command-failed", "pinned command did not complete")
	}
	return result, nil
}
