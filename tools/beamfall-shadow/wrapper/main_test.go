//go:build aix || darwin || dragonfly || freebsd || linux || netbsd || openbsd || solaris

package main

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
	"testing"
)

func TestEarlySignalsPreventOrReapEveryLaunchBoundary(t *testing.T) {
	for _, stage := range []string{"after-install", "before-start", "after-start"} {
		for _, signal := range []string{"TERM", "INT", "HUP"} {
			t.Run(stage+"-"+signal, func(t *testing.T) {
				marker := filepath.Join(t.TempDir(), "started")
				command := exec.Command(os.Args[0], "-test.run=^TestEarlySignalHelper$", "--", stage, signal, marker)
				command.Env = append(os.Environ(), "SHADOW_WRAPPER_EARLY_SIGNAL_HELPER=1")
				err := command.Run()
				var exitError *exec.ExitError
				if !errors.As(err, &exitError) || exitError.ExitCode() != 128 {
					t.Fatalf("stage=%s signal=%s err=%v", stage, signal, err)
				}
				if stage != "after-start" {
					if _, err := os.Stat(marker); !os.IsNotExist(err) {
						t.Fatalf("stage=%s launched child before signal handling: %v", stage, err)
					}
				}
			})
		}
	}
}

func TestEarlySignalHelper(t *testing.T) {
	if os.Getenv("SHADOW_WRAPPER_EARLY_SIGNAL_HELPER") != "1" {
		return
	}
	arguments := os.Args
	separator := 0
	for i, value := range arguments {
		if value == "--" {
			separator = i
			break
		}
	}
	if separator == 0 || len(arguments) != separator+4 {
		os.Exit(2)
	}
	stage, signalName, marker := arguments[separator+1], arguments[separator+2], arguments[separator+3]
	signal := map[string]syscall.Signal{"TERM": syscall.SIGTERM, "INT": syscall.SIGINT, "HUP": syscall.SIGHUP}[signalName]
	if signal == 0 {
		os.Exit(2)
	}
	send := func() { _ = syscall.Kill(os.Getpid(), signal) }
	hooks := launchHooks{}
	switch stage {
	case "after-install":
		hooks.afterSignalInstall = send
	case "before-start":
		hooks.beforeStart = send
	case "after-start":
		hooks.afterStart = send
	default:
		os.Exit(2)
	}
	code := run([]string{"--", "/bin/sh", "-c", "printf started > \"$1\"; sleep 30 & wait", "shadow", marker}, hooks)
	os.Exit(code)
}
