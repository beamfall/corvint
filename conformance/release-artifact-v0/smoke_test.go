package main

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// TestWriteChecksumsUsesCanonicalSHA256SUMSFormat is a GOC-V0-006 falsifying
// assertion: writeChecksums had no direct test anywhere in this package, so a
// format regression (wrong separator, missing newline, full-path instead of
// basename) would have gone undetected.
func TestWriteChecksumsUsesCanonicalSHA256SUMSFormat(t *testing.T) {
	path := filepath.Join(t.TempDir(), "SHA256SUMS")
	targets := []TargetReport{
		{SHA256: "aaaa", Artifact: "/build/out/corvint-darwin-arm64"},
		{SHA256: "bbbb", Artifact: "/build/out/corvint-linux-amd64"},
	}
	if err := writeChecksums(path, targets); err != nil {
		t.Fatal(err)
	}
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	want := "aaaa  corvint-darwin-arm64\nbbbb  corvint-linux-amd64\n"
	if string(body) != want {
		t.Fatalf("SHA256SUMS format = %q want %q", body, want)
	}
}

// TestSmokeTestExecutesRealSubprocessAndDetectsFailures is a GOC-V0-006
// falsifying assertion: the only prior coverage of smokeTest/querySmoke drove
// the pure classifyQuery sub-function with canned strings, so a smoke check
// that stopped actually invoking the candidate binary (a no-op regression)
// would still have passed every existing test. This drives smokeTest against
// a real subprocess and requires it to both PASS a genuine run and FAIL when
// the version banner, resolved intent, or fixture cleanliness regresses.
func TestSmokeTestExecutesRealSubprocessAndDetectsFailures(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not on PATH")
	}
	if _, err := exec.LookPath("sh"); err != nil {
		t.Skip("sh not on PATH")
	}
	smoke := Smoke{
		VersionArgument:    "--version",
		ExpectedVersion:    "Stub 1.0.0",
		QueryTask:          "task",
		QueryLimit:         "1",
		ExpectedIntent:     "fixture-intent",
		FixtureInstruction: "# Agents\n",
	}
	binary := writeStubBinary(t)

	cases := []struct {
		name    string
		version string
		output  string
		mutate  bool
		check   func(t *testing.T, report SmokeReport)
	}{
		{
			name: "genuine pass", version: "Stub 1.0.0 (build 7)",
			output: `{"context":{"intent":{"id":"fixture-intent"}}}`,
			check: func(t *testing.T, report SmokeReport) {
				if report.Status != statusPass || !report.RepositoryUnchanged || !report.QueryOK {
					t.Fatalf("expected a genuine pass, got %+v", report)
				}
			},
		},
		{
			name: "missing build number fails", version: "Stub 1.0.0",
			output: `{"context":{"intent":{"id":"fixture-intent"}}}`,
			check: func(t *testing.T, report SmokeReport) {
				if report.Status != statusFail || !strings.Contains(report.Reason, "version banner") {
					t.Fatalf("expected a version-banner failure, got %+v", report)
				}
			},
		},
		{
			name: "wrong build number fails", version: "Stub 1.0.0 (build 6)",
			output: `{"context":{"intent":{"id":"fixture-intent"}}}`,
			check: func(t *testing.T, report SmokeReport) {
				if report.Status != statusFail || !strings.Contains(report.Reason, "version banner") {
					t.Fatalf("expected a version-banner failure, got %+v", report)
				}
			},
		},
		{
			name: "wrong version banner fails", version: "Stub 9.9.9 (build 7)",
			output: `{"context":{"intent":{"id":"fixture-intent"}}}`,
			check: func(t *testing.T, report SmokeReport) {
				if report.Status != statusFail || !strings.Contains(report.Reason, "version banner") {
					t.Fatalf("expected a version-banner failure, got %+v", report)
				}
			},
		},
		{
			name: "wrong resolved intent fails", version: "Stub 1.0.0 (build 7)",
			output: `{"context":{"intent":{"id":"other"}}}`,
			check: func(t *testing.T, report SmokeReport) {
				if report.Status != statusFail || report.QueryOK {
					t.Fatalf("expected a wrong-intent failure, got %+v", report)
				}
			},
		},
		{
			name: "repository mutation fails", version: "Stub 1.0.0 (build 7)",
			output: `{"context":{"intent":{"id":"fixture-intent"}}}`, mutate: true,
			check: func(t *testing.T, report SmokeReport) {
				if report.Status != statusFail || !strings.Contains(report.Reason, "mutated") {
					t.Fatalf("expected a repository-mutation failure, got %+v", report)
				}
			},
		},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Setenv("SMOKE_VERSION", testCase.version)
			t.Setenv("SMOKE_QUERY_OUTPUT", testCase.output)
			if testCase.mutate {
				t.Setenv("SMOKE_MUTATE", "1")
			} else {
				t.Setenv("SMOKE_MUTATE", "")
			}
			report := smokeTest(context.Background(), binary, smoke, "7", t.TempDir())
			testCase.check(t, report)
		})
	}
}

func writeStubBinary(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "stub-corvint")
	script := `#!/bin/sh
if [ "$1" = "--version" ]; then
  echo "$SMOKE_VERSION"
  exit 0
fi
root="$2"
if [ "$SMOKE_MUTATE" = "1" ]; then
  echo mutated > "$root/mutated.txt"
fi
echo "$SMOKE_QUERY_OUTPUT"
`
	if err := os.WriteFile(path, []byte(script), 0o700); err != nil {
		t.Fatal(err)
	}
	return path
}
