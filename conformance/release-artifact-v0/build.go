package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"debug/buildinfo"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
)

// buildOnce runs one same-profile build into its own output path with its own
// build cache and temporary directory. Two builds sharing a cache would prove
// only that the cache was reused, so each build gets a cold cache.
// buildNumber is the first-parent commit count of revision: every commit on main
// carries a new, larger build number (PUB-V0-021).
func buildNumber(ctx context.Context, root, revision string) (string, error) {
	return gitOutput(ctx, root, "rev-list", "--count", "--first-parent", revision)
}

// buildArguments is the go build argv for the profile, stamping build into main.build.
func buildArguments(manifest Manifest, build, output string) []string {
	arguments := append([]string{"build"}, manifest.Profile.BuildFlags...)
	return append(arguments, "-ldflags=-X main.build="+build, "-o", output, manifest.Profile.Package)
}

func buildOnce(ctx context.Context, root string, manifest Manifest, target Target, build, output, cache, temporary string) error {
	for _, directory := range []string{cache, temporary} {
		if err := os.MkdirAll(directory, 0o700); err != nil {
			return err
		}
	}
	command := exec.CommandContext(ctx, "go", buildArguments(manifest, build, output)...)
	command.Dir = root
	command.Env = buildEnvironment(manifest, target, cache, temporary)
	var stderr bytes.Buffer
	command.Stderr = &stderr
	command.Stdout = io.Discard
	if err := command.Run(); err != nil {
		return fmt.Errorf("%w: %s", err, strings.TrimSpace(stderr.String()))
	}
	return nil
}

// buildEnvironment is a closed allowlist. An inherited GOFLAGS, GOPROXY, or
// GOTOOLCHAIN must not be able to change the profile.
func buildEnvironment(manifest Manifest, target Target, cache, temporary string) []string {
	environment := map[string]string{
		"HOME":    os.Getenv("HOME"),
		"PATH":    os.Getenv("PATH"),
		"GOOS":    target.GOOS,
		"GOARCH":  target.GOARCH,
		"GOCACHE": cache,
		"TMPDIR":  temporary,
		"GOSUMDB": "off",
	}
	for key, value := range manifest.Profile.Environment {
		environment[key] = value
	}
	pairs := make([]string, 0, len(environment))
	for key, value := range environment {
		pairs = append(pairs, key+"="+value)
	}
	sort.Strings(pairs)
	return pairs
}

func fileDigest(path string) (string, int64, error) {
	handle, err := os.Open(path)
	if err != nil {
		return "", 0, err
	}
	defer handle.Close()
	digest := sha256.New()
	written, err := io.Copy(digest, handle)
	if err != nil {
		return "", 0, err
	}
	return hex.EncodeToString(digest.Sum(nil)), written, nil
}

// describeDivergence names the exact first differing byte offset so a
// non-reproducible target is a located finding, not an assertion.
func describeDivergence(first, second string) string {
	left, leftErr := os.ReadFile(first)
	right, rightErr := os.ReadFile(second)
	if leftErr != nil || rightErr != nil {
		return "build outputs could not be compared"
	}
	if len(left) != len(right) {
		return fmt.Sprintf("size differs: %d vs %d bytes", len(left), len(right))
	}
	for index := range left {
		if left[index] != right[index] {
			return fmt.Sprintf("first differing byte at offset %d: 0x%02x vs 0x%02x (equal length %d)", index, left[index], right[index], len(left))
		}
	}
	return "digests differ with no located byte difference"
}

// readBuildInfo extracts the embedded provenance. It works on a cross-built
// artifact, so every target is checked, not only the native one.
func readBuildInfo(path string) (map[string]string, error) {
	info, err := buildinfo.ReadFile(path)
	if err != nil {
		return nil, err
	}
	settings := map[string]string{"go": info.GoVersion, "path": info.Path, "mod": info.Main.Version}
	for _, setting := range info.Settings {
		settings[setting.Key] = setting.Value
	}
	dependencies := make([]string, 0, len(info.Deps))
	for _, dependency := range info.Deps {
		dependencies = append(dependencies, dependency.Path)
	}
	settings["deps"] = strings.Join(dependencies, ",")
	return settings, nil
}

// verifyBuildInfo binds each artifact to the exact clean commit, the exact
// toolchain, CGO_ENABLED=0, and -trimpath, from bytes inside the artifact.
func verifyBuildInfo(manifest Manifest, target Target, commit string, info map[string]string) []string {
	expected := map[string]string{
		"go":           manifest.Toolchain.GoVersion,
		"path":         strings.TrimPrefix(manifest.Profile.Package, "./"),
		"GOOS":         target.GOOS,
		"GOARCH":       target.GOARCH,
		"CGO_ENABLED":  "0",
		"-trimpath":    "true",
		"vcs.revision": commit,
		"vcs.modified": "false",
	}
	expected["path"] = manifest.Profile.ModulePath + "/" + expected["path"]
	keys := make([]string, 0, len(expected))
	for key := range expected {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	var mismatches []string
	for _, key := range keys {
		if info[key] != expected[key] {
			mismatches = append(mismatches, fmt.Sprintf("%s=%q want %q", key, info[key], expected[key]))
		}
	}
	return mismatches
}

func writeChecksums(path string, targets []TargetReport) error {
	var builder strings.Builder
	for _, target := range targets {
		builder.WriteString(target.SHA256 + "  " + filepath.Base(target.Artifact) + "\n")
	}
	return os.WriteFile(path, []byte(builder.String()), 0o600)
}
