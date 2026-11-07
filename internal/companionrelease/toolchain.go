package companionrelease

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

// requiredGoVersion is the exact toolchain identity PUB-V0-003 pins the
// companion build to. It is deliberately not a prefix match: any other
// installed Go, including a newer patch release, refuses the build.
const requiredGoVersion = "go1.27.3"

// Toolchain records the exact, absolute Go and Git identities a companion
// build used, for the component manifest and the final report.
type Toolchain struct {
	GoPath     string
	GoVersion  string
	GitPath    string
	GitVersion string
}

// resolveToolchain locates absolute Go and Git executables on PATH and
// verifies the exact pinned Go version. It never falls back to a different
// toolchain and never mutates GOTOOLCHAIN state.
func resolveToolchain(ctx context.Context, scratch string) (Toolchain, error) {
	goPath, err := exec.LookPath("go")
	if err != nil {
		return Toolchain{}, fmt.Errorf("resolve go on PATH: %w", err)
	}
	gitPath, err := exec.LookPath("git")
	if err != nil {
		return Toolchain{}, fmt.Errorf("resolve git on PATH: %w", err)
	}
	out, _, err := runCaptured(ctx, scratch, closedGoEnv(scratch, ""), subprocessTimeout,
		goPath, "env", "GOVERSION")
	if err != nil {
		return Toolchain{}, fmt.Errorf("go env GOVERSION: %w", err)
	}
	goVersion := strings.TrimSpace(string(out))
	if goVersion != requiredGoVersion {
		return Toolchain{}, fmt.Errorf("go toolchain mismatch: want exact %q, got %q", requiredGoVersion, goVersion)
	}
	out, _, err = runCaptured(ctx, scratch, closedGitEnv(scratch), subprocessTimeout, gitPath, "--version")
	if err != nil {
		return Toolchain{}, fmt.Errorf("git --version: %w", err)
	}
	return Toolchain{GoPath: goPath, GoVersion: goVersion, GitPath: gitPath, GitVersion: strings.TrimSpace(string(out))}, nil
}

// closedGitEnv is the fixed, minimal environment for every Git invocation:
// no system/global config, no network prompts, no ambient locale.
func closedGitEnv(home string) []string {
	return []string{
		"PATH=/usr/bin:/bin",
		"HOME=" + home,
		"GIT_CONFIG_NOSYSTEM=1",
		"GIT_CONFIG_GLOBAL=/dev/null",
		"GIT_CONFIG_SYSTEM=/dev/null",
		"GIT_TERMINAL_PROMPT=0",
		"GIT_OPTIONAL_LOCKS=0",
		"GIT_NO_LAZY_FETCH=1",
		"GIT_NO_REPLACE_OBJECTS=1",
		"LANG=C",
		"LC_ALL=C",
	}
}

// closedGoEnv is the fixed, minimal build environment PUB-V0-003 requires:
// no workspace, no module proxy or checksum lookups, no cgo, an explicit
// target (empty targetGOOS_GOARCH means "host", used for `go env`).
func closedGoEnv(home, targetGOOSArch string) []string {
	env := []string{
		"PATH=/usr/bin:/bin",
		"HOME=" + home,
		"GOPATH=" + home + "/gopath",
		"GOCACHE=" + home + "/gocache",
		"GOMODCACHE=" + home + "/gomodcache",
		// PUB-V0-965: the build's $WORK beside its cold caches, never the
		// host's default temp directory; ensureGoTmp creates it.
		"GOTMPDIR=" + home + "/gotmp",
		"GOTOOLCHAIN=local",
		"GOWORK=off",
		"GOFLAGS=",
		"GOPROXY=off",
		"GOSUMDB=off",
		"GOFLAGS=",
		"CGO_ENABLED=0",
		"LANG=C",
		"LC_ALL=C",
	}
	if targetGOOSArch != "" {
		goos, arch, _ := splitTarget(targetGOOSArch)
		env = append(env, "GOOS="+goos, "GOARCH="+arch)
	}
	return dedupeEnv(env)
}

// goTmpDir is the GOTMPDIR an environment names, or "" when it names none.
func goTmpDir(env []string) string { return envValue(env, "GOTMPDIR") }

// envValue is the last value an environment gives name, or "" when it gives none.
func envValue(env []string, name string) string {
	for i := len(env) - 1; i >= 0; i-- {
		if value, ok := strings.CutPrefix(env[i], name+"="); ok {
			return value
		}
	}
	return ""
}

// ensureGoTmp creates the GOTMPDIR an environment names, if any, since Go
// refuses a GOTMPDIR that does not exist (PUB-V0-965). An environment
// without one is left alone. For a closedGoEnv environment it also turns
// the go command's telemetry off under the build's own home (PUB-V0-700).
func ensureGoTmp(env []string) error {
	if dir := goTmpDir(env); dir != "" {
		if err := os.MkdirAll(dir, 0o700); err != nil {
			return err
		}
		return quietGoTelemetry(env, dir)
	}
	return nil
}

// quietGoTelemetry writes telemetry mode "off" into the go command's config
// directory under a closedGoEnv home (PUB-V0-700, V1-3823). With the default
// mode the go command starts a detached child that writes counters there
// after the command exits, so removing the home races it ("directory not
// empty"); GOTELEMETRY in the environment does not change that mode. Only
// the closedGoEnv layout (GOTMPDIR is HOME/gotmp) is touched, never a
// caller's real home, and an existing mode file is kept.
func quietGoTelemetry(env []string, goTmp string) error {
	home := envValue(env, "HOME")
	if home == "" || goTmp != home+"/gotmp" || runtime.GOOS == "windows" {
		return nil
	}
	config := filepath.Join(home, ".config")
	if runtime.GOOS == "darwin" {
		config = filepath.Join(home, "Library", "Application Support")
	} else if xdg := envValue(env, "XDG_CONFIG_HOME"); xdg != "" {
		config = xdg
	}
	dir := filepath.Join(config, "go", "telemetry")
	mode := filepath.Join(dir, "mode")
	if _, err := os.Stat(mode); err == nil {
		return nil
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("turn go telemetry off: %w", err)
	}
	if err := os.WriteFile(mode, []byte("off"), 0o600); err != nil {
		return fmt.Errorf("turn go telemetry off: %w", err)
	}
	return nil
}

func dedupeEnv(env []string) []string {
	seen := make(map[string]int, len(env))
	out := make([]string, 0, len(env))
	for _, e := range env {
		name := e
		if i := strings.IndexByte(e, '='); i >= 0 {
			name = e[:i]
		}
		if idx, ok := seen[name]; ok {
			out[idx] = e
			continue
		}
		seen[name] = len(out)
		out = append(out, e)
	}
	return out
}
