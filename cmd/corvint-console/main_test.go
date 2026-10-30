package main

import (
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestParseOptionsSelectsTaskCompatibilityFlags(t *testing.T) {
	tests := []struct {
		name      string
		arguments []string
		want      string
		wantError bool
	}{
		{name: "CRB-V0-011 primary", arguments: []string{"-tasks", "/current"}, want: "/current"},
		{name: "CRB-V0-011 legacy fallback", arguments: []string{"-atm", "/legacy"}, want: "/legacy"},
		{name: "CRB-V0-011 equal", arguments: []string{"-tasks", "/same", "-atm", "/same"}, want: "/same"},
		{name: "CRB-V0-011 conflict", arguments: []string{"-tasks", "/current", "-atm", "/legacy"}, wantError: true},
		{name: "CRB-V0-011 empty primary", arguments: []string{"-tasks", ""}, wantError: true},
		{name: "CRB-V0-011 empty legacy", arguments: []string{"-atm", ""}, wantError: true},
		{name: "CRB-V0-011 equal empty", arguments: []string{"-tasks", "", "-atm", ""}, wantError: true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := parseOptions(test.arguments, io.Discard)
			if test.wantError {
				if err == nil {
					t.Fatal("want error")
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if got.tasks != test.want {
				t.Fatalf("tasks = %q, want %q", got.tasks, test.want)
			}
		})
	}
}

func TestFindCompanionPrefersAdjacentBeforePath(t *testing.T) {
	dir := t.TempDir()
	executable := filepath.Join(dir, "corvint-console")
	if err := os.WriteFile(executable, []byte("console"), 0o755); err != nil {
		t.Fatal(err)
	}
	legacy := filepath.Join(dir, "atm")
	if err := os.WriteFile(legacy, []byte("legacy"), 0o755); err != nil {
		t.Fatal(err)
	}
	resolvedDir, err := filepath.EvalSymlinks(dir)
	if err != nil {
		t.Fatal(err)
	}
	lookedUp := false
	got := findCompanion(executable, []string{"corvint-tasks", "atm"}, func(name string) (string, error) {
		lookedUp = true
		return "/path/" + name, nil
	})
	if want := filepath.Join(resolvedDir, "atm"); got != want || lookedUp {
		t.Fatalf("companion = %q, PATH lookup = %v", got, lookedUp)
	}

	if err := os.WriteFile(filepath.Join(dir, "corvint-tasks"), []byte("current"), 0o755); err != nil {
		t.Fatal(err)
	}
	got = findCompanion(executable, []string{"corvint-tasks", "atm"}, func(string) (string, error) {
		return "", errors.New("not found")
	})
	if want := filepath.Join(resolvedDir, "corvint-tasks"); got != want {
		t.Fatalf("companion = %q, want %q", got, want)
	}
}

func TestFindCompanionUsesCurrentPathBeforeLegacy(t *testing.T) {
	t.Run("CRB-V0-015 current PATH before legacy", func(t *testing.T) {
		got := findCompanion("", []string{"corvint-tasks", "atm"}, func(name string) (string, error) {
			return "/path/" + name, nil
		})
		if got != "/path/corvint-tasks" {
			t.Fatalf("companion = %q", got)
		}

		got = findCompanion("", []string{"corvint-tasks", "atm"}, func(name string) (string, error) {
			if name == "corvint-tasks" {
				return "", errors.New("not found")
			}
			return "/path/atm", nil
		})
		if got != "/path/atm" {
			t.Fatalf("companion = %q", got)
		}
	})
}

func TestConsoleCommandRejectsFlagsBeforeChildrenWithExitTwo(t *testing.T) {
	marker := filepath.Join(t.TempDir(), "git-ran")
	binDir := t.TempDir()
	git := filepath.Join(binDir, "git")
	if err := os.WriteFile(git, []byte("#!/bin/sh\n: > \"$GIT_MARKER\"\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		name      string
		arguments []string
		want      string
	}{
		{name: "conflict", arguments: []string{"-tasks", "/current", "-atm", "/legacy"}, want: "-tasks and -atm conflict"},
		{name: "empty primary", arguments: []string{"-tasks", ""}, want: "-tasks must not be empty"},
		{name: "empty legacy", arguments: []string{"-atm", ""}, want: "-atm must not be empty"},
		{name: "unknown", arguments: []string{"-unknown"}, want: "flag provided but not defined"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_ = os.Remove(marker)
			arguments := []string{"-test.run=TestConsoleCommandHelper", "--"}
			arguments = append(arguments, test.arguments...)
			command := exec.Command(os.Args[0], arguments...)
			command.Env = append(os.Environ(), "CORVINT_CONSOLE_HELPER=1", "GIT_MARKER="+marker, "PATH="+binDir)
			output, err := command.CombinedOutput()
			var exit *exec.ExitError
			if !errors.As(err, &exit) || exit.ExitCode() != 2 {
				t.Fatalf("exit error = %v, output = %s", err, output)
			}
			if !strings.Contains(string(output), test.want) {
				t.Fatalf("output = %q, want %q", output, test.want)
			}
			if _, err := os.Stat(marker); !errors.Is(err, os.ErrNotExist) {
				t.Fatalf("child ran before refusal: %v", err)
			}
		})
	}
}

func TestConsoleCommandHelper(t *testing.T) {
	if os.Getenv("CORVINT_CONSOLE_HELPER") != "1" {
		return
	}
	for index, argument := range os.Args {
		if argument == "--" {
			os.Args = append([]string{"corvint-console"}, os.Args[index+1:]...)
			main()
			return
		}
	}
	os.Exit(99)
}
