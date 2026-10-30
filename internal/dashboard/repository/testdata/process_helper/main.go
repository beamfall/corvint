package main

import (
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"
)

func main() {
	mode := os.Getenv("CORVINT_PROCESS_HELPER_MODE")
	switch mode {
	case "changed-executable":
		executable, err := os.Executable()
		if err != nil || os.Chmod(executable, 0600) != nil {
			os.Exit(69)
		}
	case "normal-residue":
		spawnDescendant()
	case "hostile-tree":
		signal.Ignore(syscall.SIGTERM)
		spawnDescendant()
		for {
			time.Sleep(time.Hour)
		}
	case "hostile-output":
		signal.Ignore(syscall.SIGTERM)
		spawnDescendant()
		_, _ = os.Stdout.WriteString(strings.Repeat("x", 8192))
		for {
			time.Sleep(time.Hour)
		}
	case "descendant":
		signal.Ignore(syscall.SIGTERM)
		for {
			time.Sleep(time.Hour)
		}
	default:
		os.Exit(64)
	}
}

func spawnDescendant() {
	executable, err := os.Executable()
	if err != nil {
		os.Exit(65)
	}
	null, err := os.OpenFile(os.DevNull, os.O_RDWR, 0)
	if err != nil {
		os.Exit(66)
	}
	defer null.Close()
	command := exec.Command(executable)
	command.Env = replaceEnvironment(os.Environ(), "CORVINT_PROCESS_HELPER_MODE", "descendant")
	command.Stdin = null
	command.Stdout = null
	command.Stderr = null
	if err := command.Start(); err != nil {
		os.Exit(67)
	}
	if err := appendPID(command.Process.Pid); err != nil {
		_ = command.Process.Kill()
		_, _ = command.Process.Wait()
		os.Exit(68)
	}
}

func appendPID(pid int) error {
	path := os.Getenv("CORVINT_PROCESS_HELPER_PID_FILE")
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0o600)
	if err != nil {
		return err
	}
	defer file.Close()
	_, err = fmt.Fprintln(file, strconv.Itoa(pid))
	return err
}

func replaceEnvironment(environment []string, name, value string) []string {
	prefix := name + "="
	result := make([]string, 0, len(environment)+1)
	for _, entry := range environment {
		if !strings.HasPrefix(entry, prefix) {
			result = append(result, entry)
		}
	}
	return append(result, prefix+value)
}
