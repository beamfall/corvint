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

	"github.com/Beamfall/corvint/internal/versionline"
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
	version, err := runCaptured(ctx, binary, smoke.VersionArgument)
	if err != nil {
		return SmokeReport{Status: statusFail, Reason: fmt.Sprintf("%s failed: %v", smoke.VersionArgument, err)}
	}
	// PUB-V0-053: read the version prefix and the build member tolerantly; the
	// stamped build must still be exact.
	if line, ok := versionline.Parse(version); !ok || !versionline.SameRelease(line.Name+" "+line.Version, smoke.ExpectedVersion) || line.Build != build {
		return SmokeReport{Status: statusFail, Reason: fmt.Sprintf("version banner %q want %s (build %s, ...)", strings.TrimSpace(version), smoke.ExpectedVersion, build)}
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
	output, err := runCaptured(ctx, binary, "--root", fixture, "context", "--path", smoke.QueryPath)
	if err != nil {
		return SmokeReport{Status: statusFail, VersionOutput: version, Reason: fmt.Sprintf("query failed: %v: %s", err, truncate(output))}
	}
	after, statusErr := gitStatus(ctx, fixture)
	if statusErr != nil {
		return SmokeReport{Status: statusFail, VersionOutput: version, Reason: "fixture status: " + statusErr.Error()}
	}
	report := SmokeReport{Status: statusPass, VersionOutput: version, RepositoryUnchanged: before == after && after == ""}
	return classifyQuery(report, output, smoke.ExpectedState)
}

// classifyQuery requires a real path brief: the instructions section of the first path must
// have the expected state and the repository must be provably unchanged. Decision 0594 removed
// `query` and decision 0597 removed the `context --task` packet, so the smoke reads
// `context --path`; QueryIntent records the observed instructions state.
func classifyQuery(report SmokeReport, output, expectedState string) SmokeReport {
	var decoded struct {
		Profile string `json:"profile"`
		Paths   []struct {
			Sections []struct {
				Name  string `json:"name"`
				State string `json:"state"`
			} `json:"sections"`
		} `json:"paths"`
	}
	if err := json.Unmarshal([]byte(output), &decoded); err != nil {
		report.Status = statusFail
		report.Reason = "query output is not JSON: " + truncate(output)
		return report
	}
	if decoded.Profile == "corvint-path-brief/1" && len(decoded.Paths) > 0 {
		for _, section := range decoded.Paths[0].Sections {
			if section.Name == "instructions" {
				report.QueryIntent = section.State
			}
		}
	}
	report.QueryOK = report.QueryIntent == expectedState
	if !report.QueryOK {
		report.Status = statusFail
		report.Reason = "path brief instructions state " + strconv.Quote(report.QueryIntent) + " want " + strconv.Quote(expectedState)
		return report
	}
	if !report.RepositoryUnchanged {
		report.Status = statusFail
		report.Reason = "read-only query mutated the fixture repository"
	}
	return report
}

// makeQueryFixture builds a throwaway repository whose root AGENTS.md the path
// brief reads as its instructions section.
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
		if err := configureLooseCommand(command); err != nil {
			return "", err
		}
		output, err := command.CombinedOutput()
		cleanupErr := cleanupLooseCommand(command)
		if err == nil {
			err = cleanupErr
		}
		if err != nil {
			return "", fmt.Errorf("git %s: %w: %s", arguments[0], err, strings.TrimSpace(string(output)))
		}
	}
	return fixture, nil
}

func gitStatus(ctx context.Context, root string) (string, error) {
	command := exec.CommandContext(ctx, "git", "status", "--porcelain")
	command.Dir = root
	if err := configureLooseCommand(command); err != nil {
		return "", err
	}
	output, err := command.Output()
	cleanupErr := cleanupLooseCommand(command)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(output)), cleanupErr
}

func runCaptured(ctx context.Context, binary string, arguments ...string) (string, error) {
	command := exec.CommandContext(ctx, binary, arguments...)
	var stdout, stderr bytes.Buffer
	command.Stdout = &stdout
	command.Stderr = &stderr
	if err := configureLooseCommand(command); err != nil {
		return "", err
	}
	runErr := command.Run()
	cleanupErr := cleanupLooseCommand(command)
	if runErr != nil {
		return stdout.String() + stderr.String(), runErr
	}
	return stdout.String(), cleanupErr
}

func truncate(value string) string {
	value = strings.TrimSpace(value)
	if len(value) > 240 {
		return value[:240] + "..."
	}
	return value
}
