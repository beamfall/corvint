package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
)

// isNative reports whether this host can genuinely execute the target.
// GPK-V0-018 states cross-compilation is not execution evidence, so this is
// computed from the running host and never declared by the manifest.
func isNative(target Target) bool {
	return target.GOOS == runtime.GOOS && target.GOARCH == runtime.GOARCH
}

// smokeTest runs the two required executions: the version banner and one real
// read-only query, proving the query left the repository unchanged.
func smokeTest(ctx context.Context, binary string, smoke Smoke, build, workspace string) SmokeReport {
	banner := smoke.ExpectedVersion + " (build " + build + ")"
	version, err := runCaptured(ctx, binary, smoke.VersionArgument)
	if err != nil {
		return SmokeReport{Status: statusFail, Reason: fmt.Sprintf("%s failed: %v", smoke.VersionArgument, err)}
	}
	if strings.TrimSpace(version) != banner {
		return SmokeReport{Status: statusFail, Reason: fmt.Sprintf("version banner %q want %q", strings.TrimSpace(version), banner)}
	}
	fixture, err := makeQueryFixture(ctx, workspace, smoke)
	if err != nil {
		return SmokeReport{Status: statusFail, VersionOutput: strings.TrimSpace(version), Reason: "query fixture: " + err.Error()}
	}
	return querySmoke(ctx, binary, smoke, fixture, strings.TrimSpace(version))
}

func querySmoke(ctx context.Context, binary string, smoke Smoke, fixture, version string) SmokeReport {
	before, err := gitStatus(ctx, fixture)
	if err != nil {
		return SmokeReport{Status: statusFail, VersionOutput: version, Reason: "fixture status: " + err.Error()}
	}
	output, err := runCaptured(ctx, binary, "--root", fixture, "query", "--task", smoke.QueryTask, "--limit", smoke.QueryLimit)
	if err != nil {
		return SmokeReport{Status: statusFail, VersionOutput: version, Reason: fmt.Sprintf("query failed: %v: %s", err, truncate(output))}
	}
	after, statusErr := gitStatus(ctx, fixture)
	if statusErr != nil {
		return SmokeReport{Status: statusFail, VersionOutput: version, Reason: "fixture status: " + statusErr.Error()}
	}
	report := SmokeReport{Status: statusPass, VersionOutput: version, RepositoryUnchanged: before == after && after == ""}
	return classifyQuery(report, output, smoke.ExpectedIntent)
}

// classifyQuery requires a real successful receipt: the resolved intent must be
// the expected one and the repository must be provably unchanged.
func classifyQuery(report SmokeReport, output, expectedIntent string) SmokeReport {
	var decoded struct {
		Context struct {
			Intent struct {
				ID string `json:"id"`
			} `json:"intent"`
		} `json:"context"`
	}
	if err := json.Unmarshal([]byte(output), &decoded); err != nil {
		report.Status = statusFail
		report.Reason = "query output is not JSON: " + truncate(output)
		return report
	}
	report.QueryIntent = decoded.Context.Intent.ID
	report.QueryOK = decoded.Context.Intent.ID == expectedIntent
	if !report.QueryOK {
		report.Status = statusFail
		report.Reason = "query resolved intent " + strconv.Quote(decoded.Context.Intent.ID) + " want " + strconv.Quote(expectedIntent)
		return report
	}
	if !report.RepositoryUnchanged {
		report.Status = statusFail
		report.Reason = "read-only query mutated the fixture repository"
	}
	return report
}

// makeQueryFixture builds a throwaway repository with the uniquely
// highest-ranked root AGENTS.md the authority-start query requires.
func makeQueryFixture(ctx context.Context, workspace string, smoke Smoke) (string, error) {
	fixture := filepath.Join(workspace, "smoke-fixture")
	if err := os.RemoveAll(fixture); err != nil {
		return "", err
	}
	if err := os.MkdirAll(fixture, 0o700); err != nil {
		return "", err
	}
	if err := os.WriteFile(filepath.Join(fixture, "AGENTS.md"), []byte(smoke.FixtureInstruction), 0o600); err != nil {
		return "", err
	}
	commands := [][]string{
		{"init", "-q", "."},
		{"add", "-A"},
		{"-c", "user.email=release-artifact@corvint.invalid", "-c", "user.name=release-artifact", "commit", "-qm", "fixture"},
	}
	for _, arguments := range commands {
		command := exec.CommandContext(ctx, "git", arguments...)
		command.Dir = fixture
		command.Env = append(os.Environ(), "GIT_CONFIG_GLOBAL=/dev/null", "GIT_CONFIG_SYSTEM=/dev/null")
		if output, err := command.CombinedOutput(); err != nil {
			return "", fmt.Errorf("git %s: %w: %s", arguments[0], err, strings.TrimSpace(string(output)))
		}
	}
	return fixture, nil
}

func gitStatus(ctx context.Context, root string) (string, error) {
	command := exec.CommandContext(ctx, "git", "status", "--porcelain")
	command.Dir = root
	output, err := command.Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(output)), nil
}

func runCaptured(ctx context.Context, binary string, arguments ...string) (string, error) {
	command := exec.CommandContext(ctx, binary, arguments...)
	var stdout, stderr bytes.Buffer
	command.Stdout = &stdout
	command.Stderr = &stderr
	if err := command.Run(); err != nil {
		return stdout.String() + stderr.String(), err
	}
	return stdout.String(), nil
}

func truncate(value string) string {
	value = strings.TrimSpace(value)
	if len(value) > 240 {
		return value[:240] + "..."
	}
	return value
}
