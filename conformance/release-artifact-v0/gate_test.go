package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const validManifest = `{
  "schema": "corvint.release-artifact-v0",
  "toolchain": {"goVersion": "go1.27.1", "goModDirective": "1.27.1", "forbiddenGoModDirective": "toolchain", "gotoolchain": "local"},
  "profile": {"package": "./cmd/corvint", "modulePath": "github.com/Beamfall/corvint", "binaryName": "corvint",
    "buildFlags": ["-trimpath"],
    "environment": {"CGO_ENABLED": "0", "GOENV": "off", "GOTOOLCHAIN": "local", "GOPROXY": "off", "GOFLAGS": "-mod=readonly", "GOSUMDB": "off"}},
  "targets": [
    {"goos": "darwin", "goarch": "amd64", "binarySuffix": ""},
    {"goos": "darwin", "goarch": "arm64", "binarySuffix": ""},
    {"goos": "linux", "goarch": "amd64", "binarySuffix": ""},
    {"goos": "linux", "goarch": "arm64", "binarySuffix": ""},
    {"goos": "windows", "goarch": "amd64", "binarySuffix": ".exe"}
  ],
  "legalFiles": [
    {"path": "LICENSE", "sha256": "cdb7dd035e8a8536a2b5c90ba6fb7a6270a1b989f7b52f3f87c4877e2fa6c893"},
    {"path": "LICENSE-APACHE-2.0", "sha256": "cdb7dd035e8a8536a2b5c90ba6fb7a6270a1b989f7b52f3f87c4877e2fa6c893"},
    {"path": "LICENSING.md", "sha256": "cdb7dd035e8a8536a2b5c90ba6fb7a6270a1b989f7b52f3f87c4877e2fa6c893"},
    {"path": "PROVENANCE.md", "sha256": "cdb7dd035e8a8536a2b5c90ba6fb7a6270a1b989f7b52f3f87c4877e2fa6c893"}],
  "smoke": {"versionArgument": "--version", "expectedVersion": "Corvint 0.5.0a4", "queryTask": "orient",
    "queryLimit": "1", "expectedIntent": "project-operations", "fixtureInstructions": "# Agents\n",
    "crossCompileNotRunReason": "cross-compilation is not execution evidence"},
  "pendingEvidence": ["W10-performance-GPK-V0-016-017"]
}`

func writeManifest(t *testing.T, body string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "manifest.json")
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestValidManifestLoads(t *testing.T) {
	manifest, err := loadManifest(writeManifest(t, validManifest))
	if err != nil {
		t.Fatalf("valid manifest rejected: %v", err)
	}
	if len(manifest.Targets) != len(requiredTargets) {
		t.Fatalf("target count %d want %d", len(manifest.Targets), len(requiredTargets))
	}
}

// The manifest is the profile authority, so every weakening of it must be a
// load failure rather than a quietly weaker gate.
func TestManifestRefusals(t *testing.T) {
	tests := []struct {
		name, old, new, want string
	}{
		{"wrong-schema", `"corvint.release-artifact-v0"`, `"corvint.release-artifact-v1"`, "schema must be"},
		{"missing-trimpath", `"buildFlags": ["-trimpath"]`, `"buildFlags": []`, "-trimpath"},
		{"surplus-build-flag", `"buildFlags": ["-trimpath"]`, `"buildFlags": ["-trimpath", "-x"]`, "exactly"},
		{"cgo-enabled", `"CGO_ENABLED": "0"`, `"CGO_ENABLED": "1"`, "CGO_ENABLED"},
		{"goenv-enabled", `"GOENV": "off"`, `"GOENV": "/tmp/goenv"`, "GOENV"},
		{"online-proxy", `"GOPROXY": "off"`, `"GOPROXY": "https://proxy.golang.org"`, "GOPROXY"},
		{"mutable-modules", `"GOFLAGS": "-mod=readonly"`, `"GOFLAGS": "-mod=mod"`, "GOFLAGS"},
		{"downloadable-toolchain", `"gotoolchain": "local"`, `"gotoolchain": "auto"`, "gotoolchain"},
		{"dropped-target", `{"goos": "windows", "goarch": "amd64", "binarySuffix": ".exe"}`, `{"goos": "js", "goarch": "wasm", "binarySuffix": ""}`, "row 4"},
		{"wrong-target-order", `{"goos": "darwin", "goarch": "amd64", "binarySuffix": ""}`, `{"goos": "linux", "goarch": "amd64", "binarySuffix": ""}`, "row 0"},
		{"short-legal-digest", `"cdb7dd035e8a8536a2b5c90ba6fb7a6270a1b989f7b52f3f87c4877e2fa6c893"`, `"cdb7dd"`, "sha256"},
		{"missing-not-run-reason", `"cross-compilation is not execution evidence"`, `""`, "crossCompileNotRunReason"},
		{"unknown-field", `"pendingEvidence"`, `"pendingEvidenceX"`, "unknown field"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			body := strings.Replace(validManifest, test.old, test.new, 1)
			if body == validManifest {
				t.Fatal("fixture did not change")
			}
			_, err := loadManifest(writeManifest(t, body))
			if err == nil {
				t.Fatal("expected refusal")
			}
			if !strings.Contains(err.Error(), test.want) {
				t.Fatalf("error %q does not mention %q", err, test.want)
			}
		})
	}
}

func TestDuplicateKeyIsRefused(t *testing.T) {
	body := strings.Replace(validManifest, `"schema": "corvint.release-artifact-v0",`, `"schema": "corvint.release-artifact-v0", "schema": "corvint.release-artifact-v0",`, 1)
	if _, err := loadManifest(writeManifest(t, body)); err == nil || !strings.Contains(err.Error(), "duplicate") {
		t.Fatalf("duplicate key accepted: %v", err)
	}
}

func TestOutputInsideRepositoryIsRefused(t *testing.T) {
	root := t.TempDir()
	report := Report{}
	err := checkOutputLocation(Options{Root: root, Output: filepath.Join(root, "artifacts")}, &report)
	if err == nil {
		t.Fatal("expected refusal")
	}
	if len(report.Reasons) != 1 || report.Reasons[0].Kind != reasonOutputInsideRepo {
		t.Fatalf("reasons %+v", report.Reasons)
	}
}

func TestBuildInfoMismatchIsNamed(t *testing.T) {
	manifest, err := loadManifest(writeManifest(t, validManifest))
	if err != nil {
		t.Fatal(err)
	}
	target := Target{GOOS: "linux", GOARCH: "arm64"}
	commit := strings.Repeat("a", 40)
	clean := map[string]string{
		"go": "go1.27.1", "path": "github.com/Beamfall/corvint/cmd/corvint",
		"GOOS": "linux", "GOARCH": "arm64", "CGO_ENABLED": "0", "-trimpath": "true",
		"vcs.revision": commit, "vcs.modified": "false",
	}
	if mismatches := verifyBuildInfo(manifest, target, commit, clean); len(mismatches) != 0 {
		t.Fatalf("clean build info rejected: %v", mismatches)
	}
	for key, bad := range map[string]string{"CGO_ENABLED": "1", "-trimpath": "false", "vcs.modified": "true", "go": "go1.26.0"} {
		dirty := map[string]string{}
		for k, v := range clean {
			dirty[k] = v
		}
		dirty[key] = bad
		mismatches := verifyBuildInfo(manifest, target, commit, dirty)
		if len(mismatches) != 1 || !strings.Contains(mismatches[0], key) {
			t.Fatalf("%s=%s produced %v", key, bad, mismatches)
		}
	}
}

func TestDivergenceLocatesFirstDifferingByte(t *testing.T) {
	directory := t.TempDir()
	first := filepath.Join(directory, "a")
	second := filepath.Join(directory, "b")
	if err := os.WriteFile(first, []byte{1, 2, 3, 4}, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(second, []byte{1, 2, 9, 4}, 0o600); err != nil {
		t.Fatal(err)
	}
	if detail := describeDivergence(first, second); !strings.Contains(detail, "offset 2") {
		t.Fatalf("divergence %q", detail)
	}
	if err := os.WriteFile(second, []byte{1, 2, 3}, 0o600); err != nil {
		t.Fatal(err)
	}
	if detail := describeDivergence(first, second); !strings.Contains(detail, "size differs") {
		t.Fatalf("divergence %q", detail)
	}
}

func TestQueryMustResolveExpectedIntentAndLeaveRepositoryUnchanged(t *testing.T) {
	body := `{"context":{"intent":{"id":"project-operations"}}}`
	passed := classifyQuery(SmokeReport{Status: statusPass, RepositoryUnchanged: true}, body, "project-operations")
	if passed.Status != statusPass || !passed.QueryOK {
		t.Fatalf("clean query failed: %+v", passed)
	}
	mutated := classifyQuery(SmokeReport{Status: statusPass, RepositoryUnchanged: false}, body, "project-operations")
	if mutated.Status != statusFail || !strings.Contains(mutated.Reason, "mutated") {
		t.Fatalf("mutation not caught: %+v", mutated)
	}
	wrong := classifyQuery(SmokeReport{Status: statusPass, RepositoryUnchanged: true}, `{"context":{"intent":{"id":"other"}}}`, "project-operations")
	if wrong.Status != statusFail {
		t.Fatalf("wrong intent accepted: %+v", wrong)
	}
	broken := classifyQuery(SmokeReport{Status: statusPass, RepositoryUnchanged: true}, "not json", "project-operations")
	if broken.Status != statusFail {
		t.Fatalf("non-JSON accepted: %+v", broken)
	}
}

// A cross-built target is NOT_RUN, never a pass. This keeps GPK-V0-018's
// "cross-compilation is not execution evidence" visible in the summary line.
func TestSummaryCountsNotRunSmokeSeparately(t *testing.T) {
	report := Report{
		Verdict: statusPass,
		Pending: []string{"W10-performance-GPK-V0-016-017"},
		Targets: []TargetReport{
			{ByteIdentical: true, Smoke: SmokeReport{Status: statusPass}},
			{ByteIdentical: true, Smoke: SmokeReport{Status: statusNotRun}},
		},
	}
	var buffer bytes.Buffer
	writeSummary(&buffer, report)
	line := buffer.String()
	// smoke-pass and smoke-not-run are checked as one joined token, not two
	// separate substrings: "smoke-pass=1" alone would also match a wrongly
	// double-digit "smoke-pass=10", since one string containing another is all
	// strings.Contains asks.
	for _, want := range []string{"targets=2", "byte-identical=2 of 2", "smoke-pass=1 smoke-not-run=1 ", "verdict=PASS", "pending-evidence=W10-performance-GPK-V0-016-017"} {
		if !strings.Contains(line, want) {
			t.Fatalf("summary %q missing %q", line, want)
		}
	}
}

func TestGoModDirectivesAreReadExactly(t *testing.T) {
	path := filepath.Join(t.TempDir(), "go.mod")
	if err := os.WriteFile(path, []byte("module example.com/x\n\ngo 1.27.0\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	directive, lines, err := readGoModDirectives(path)
	if err != nil || directive != "1.27.0" || lines != 0 {
		t.Fatalf("directive=%q lines=%d err=%v", directive, lines, err)
	}
	if err := os.WriteFile(path, []byte("module example.com/x\n\ngo 1.27.0\n\ntoolchain go1.27.1\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, lines, _ := readGoModDirectives(path); lines != 1 {
		t.Fatalf("toolchain directive not counted: %d", lines)
	}
}
