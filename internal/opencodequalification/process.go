package opencodequalification

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/Beamfall/corvint/internal/procgroup"
)

type Config struct {
	Source, Host, Corvint, Self, Output, Theme string
	InterruptProbe                             bool
}

func capture(ctx context.Context, dir string, env []string, argv []string) (string, error) {
	var lookupErr error
	argv, lookupErr = absoluteArgv(argv)
	if lookupErr != nil {
		return "", lookupErr
	}
	if env == nil {
		env = os.Environ()
	}
	r := procgroup.Run(ctx, procgroup.Spec{Argv: argv, Dir: dir, Env: env, Timeout: 45 * time.Second, OutputLimit: 4 << 20})
	if r.Err != nil || r.ExitStatus != 0 {
		return "", fmt.Errorf("%v: exit %d: %v: %s", argv, r.ExitStatus, r.Err, r.Stderr)
	}
	return string(r.Stdout), nil
}
func runCommand(ctx context.Context, dir string, env []string, argv []string, output string, timeout time.Duration) (Object, error) {
	var lookupErr error
	argv, lookupErr = absoluteArgv(argv)
	if lookupErr != nil {
		return nil, lookupErr
	}
	r := procgroup.Run(ctx, procgroup.Spec{Argv: argv, Dir: dir, Env: env, Timeout: timeout, OutputLimit: 4 << 20, ObserveDescendants: true})
	e := os.WriteFile(output+".stdout", r.Stdout, 0600)
	if e != nil {
		return nil, e
	}
	if e = os.WriteFile(output+".stderr", r.Stderr, 0600); e != nil {
		return nil, e
	}
	if e = writeJSON(output+".cleanup.json", Object{"groupClean": r.OwnedProcessGroupCleanup, "descendants": r.DescendantObservation, "cancelled": r.Cancelled, "timedOut": r.TimedOut}); e != nil {
		return nil, e
	}
	result := Object{"exitCode": r.ExitStatus, "stdout": string(r.Stdout), "stderr": string(r.Stderr), "argv": argv}
	if !r.OwnedProcessGroupCleanup || r.DescendantObservation == nil || !r.DescendantObservation.Absent {
		return result, fmt.Errorf("observed command cleanup incomplete: %+v", r.DescendantObservation)
	}
	if r.Err != nil {
		return result, fmt.Errorf("command %v: %w", argv, r.Err)
	}
	if r.ExitStatus != 0 {
		return result, fmt.Errorf("command %v exited %d", argv, r.ExitStatus)
	}
	return result, nil
}
func cleanEnvironment() []string {
	out := []string{}
	for _, item := range os.Environ() {
		k, _, _ := strings.Cut(item, "=")
		if strings.HasPrefix(k, "CORVINT_TEST_") || k == "NODE_OPTIONS" || k == "NODE_PATH" || k == "GOTOOLCHAIN" || k == "GOWORK" || k == "GOFLAGS" || k == "GOPROXY" || k == "GOSUMDB" {
			continue
		}
		out = append(out, item)
	}
	return append(out, "GOTOOLCHAIN=local", "GOWORK=off", "GOFLAGS=", "GOPROXY=off", "GOSUMDB=off")
}
func appendJSON(path string, value any) error {
	f, e := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0600)
	if e != nil {
		return e
	}
	defer f.Close()
	b, e := jsonBytes(value)
	if e != nil {
		return e
	}
	_, e = f.Write(append(b, '\n'))
	return e
}
func mkdir(path string) error { return os.MkdirAll(path, 0700) }
func rootPath() (string, error) {
	cwd, e := os.Getwd()
	if e != nil {
		return "", e
	}
	s, e := capture(context.Background(), cwd, nil, []string{"git", "rev-parse", "--show-toplevel"})
	return strings.TrimSpace(s), e
}
func resolve(p string) (string, error) {
	a, e := filepath.Abs(p)
	if e != nil {
		return "", e
	}
	return filepath.EvalSymlinks(a)
}

func absoluteArgv(argv []string) ([]string, error) {
	if len(argv) == 0 {
		return nil, fmt.Errorf("missing command")
	}
	exe, e := exec.LookPath(argv[0])
	if e != nil {
		return nil, e
	}
	exe, e = filepath.Abs(exe)
	if e != nil {
		return nil, e
	}
	out := append([]string(nil), argv...)
	out[0] = exe
	return out, nil
}
