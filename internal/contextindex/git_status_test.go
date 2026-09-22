//go:build darwin || linux

package contextindex

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"
)

func TestStandaloneBuildRejectsExecutableGitFilters(t *testing.T) {
	t.Run("EAF-V0-007", func(t *testing.T) {
		for _, driver := range []string{"clean", "process"} {
			t.Run(driver, func(t *testing.T) {
				root := authorityRepository(t)
				marker := filepath.Join(root, "filter-executed")
				command := ": > '" + strings.ReplaceAll(marker, "'", "'\"'\"'") + "'; cat"
				testGit(t, root, "config", "filter.hostile."+driver, command)
				writeTestFile(t, root, ".gitattributes", "*.md filter=hostile\n")
				source := filepath.Join(root, "AGENTS.md")
				data, err := os.ReadFile(source)
				if err != nil {
					t.Fatal(err)
				}
				data[0] = '!'
				if err := os.WriteFile(source, data, 0o600); err != nil {
					t.Fatal(err)
				}
				old := time.Unix(1700000000, 0)
				if err := os.Chtimes(source, old, old); err != nil {
					t.Fatal(err)
				}
				_, err = BuildQuery(context.Background(), root, queryTaskFixture)
				var failure *Error
				if !errors.As(err, &failure) || failure.Code != "repository-probe-failed" {
					t.Fatalf("unsafe standalone build error=%v", err)
				}
				if _, err := os.Stat(marker); !os.IsNotExist(err) {
					t.Fatalf("standalone build executed filter: %v", err)
				}
			})
		}
	})
}

func TestStandaloneStatusPreservesCancellationError(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := git(ctx, "/nonexistent", maxStatusBytes, nil, "status", "--porcelain=v1", "-z")
	var failure *Error
	if !errors.As(err, &failure) || failure.Message != "Git repository index was cancelled" {
		t.Fatalf("cancelled standalone status error=%v", err)
	}
}

func TestStandaloneStatusPreservesOrdinaryGitErrors(t *testing.T) {
	root := authorityRepository(t)
	for _, test := range []struct {
		name, script, message string
		diagnostics           bool
	}{
		{"stdout-overflow", fmt.Sprintf("exec /usr/bin/head -c %d /dev/zero", maxStatusBytes+1), "Git output exceeds its byte limit", false},
		{"stderr-overflow", fmt.Sprintf("exec /usr/bin/head -c %d /dev/zero >&2", maxGitErrorBytes+1), "Git error exceeds its byte limit", false},
		{"exit", "printf 'status failed\\n' >&2; exit 23", "Git error: status failed", true},
		{"start", "", "Git error: cannot start Git", true},
	} {
		t.Run(test.name, func(t *testing.T) {
			executable := filepath.Join(t.TempDir(), "git")
			if test.script != "" {
				// The bounded emitter replaces the shell, so it cannot leave a child.
				script := "#!/bin/sh\nfor arg do\nif [ \"$arg\" = status ]; then\n" + test.script + "\nfi\ndone\nexit 99\n"
				if err := os.WriteFile(executable, []byte(script), 0o700); err != nil {
					t.Fatal(err)
				}
			}
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			ctx = context.WithValue(ctx, gitExecutionKey{}, gitExecution{executable: executable, environment: sanitizedGitEnvironment()})
			raw, err := git(ctx, root, maxStatusBytes, nil, "status", "--porcelain=v1", "-z")
			var failure *Error
			if raw != nil || !errors.As(err, &failure) || failure.Code != "" || failure.Message != test.message {
				t.Fatalf("ordinary status failure changed: output=%q error=%#v", raw, err)
			}
			if details, ok := GitFailureDetails(err); ok != test.diagnostics || ok && !slices.Contains(details.Arguments, "status") {
				t.Fatalf("status failure diagnostics=%#v found=%v", details, ok)
			}
		})
	}
}

func TestStandaloneBuildClassifiesMalformedPrivateIndex(t *testing.T) {
	root := authorityRepository(t)
	if err := os.WriteFile(filepath.Join(root, ".git", "index"), []byte("malformed index"), 0o600); err != nil {
		t.Fatal(err)
	}
	_, err := BuildQuery(context.Background(), root, queryTaskFixture)
	var failure *Error
	if !errors.As(err, &failure) || failure.Code != "repository-probe-failed" {
		t.Fatalf("malformed private index error=%#v", err)
	}
	details, ok := GitFailureDetails(err)
	if !ok || !slices.Contains(details.Arguments, "ls-files") || details.ExitCode == 0 || len(details.Stderr) == 0 {
		t.Fatalf("private index failure lost Git diagnostics: %#v, found=%v", details, ok)
	}
	if failure.Message != "Git error: "+strings.TrimSpace(string(details.Stderr)) {
		t.Fatalf("private index failure changed diagnostics: %q", failure.Message)
	}
}

func TestStandaloneStatusClassifiesMalformedPrivateConfig(t *testing.T) {
	root := authorityRepository(t)
	if err := os.WriteFile(filepath.Join(root, ".git", "config"), []byte("[malformed\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	_, err := git(context.Background(), root, maxStatusBytes, nil, "status", "--porcelain=v1", "-z")
	var failure *Error
	if !errors.As(err, &failure) || failure.Code != "repository-probe-failed" {
		t.Fatalf("malformed private config error=%#v", err)
	}
	details, ok := GitFailureDetails(err)
	if !ok || !slices.Contains(details.Arguments, "config") || details.ExitCode == 0 || len(details.Stderr) == 0 {
		t.Fatalf("private config failure lost Git diagnostics: %#v, found=%v", details, ok)
	}
	if failure.Message != "Git error: "+strings.TrimSpace(string(details.Stderr)) {
		t.Fatalf("private config failure changed diagnostics: %q", failure.Message)
	}
}
