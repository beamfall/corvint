package main

import (
	"bytes"
	"context"
	json "encoding/json/v2"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/liveverify/gorunner"
	"github.com/Beamfall/corvint/internal/liveverify/provider"
)

const driftFixtureSegment = "at14-production-drift"

// Only this fixture's bundle path selects fault injection. All semantic
// responses still originate from the production parent; markers record its
// actual output. The transport control fails DISCOVER only.
func runQualificationAuthority(ctx context.Context, encoded string, capability []byte, request attachmentRequest) int {
	root := filepath.Dir(request.Bundle.ModuleCacheDirectory)
	if filepath.Base(root) != driftFixtureSegment {
		return runProductionAuthority(ctx, encoded, capability, os.Stdout)
	}
	modeRaw, err := os.ReadFile(filepath.Join(root, "mode"))
	if err != nil {
		return 91
	}
	mode := string(modeRaw)
	if request.Operation == "DISCOVER" && mode == "transport" {
		return 92
	}
	var output bytes.Buffer
	code := runProductionAuthority(ctx, encoded, capability, &output)
	var response attachmentResponse
	if err := json.Unmarshal(output.Bytes(), &response); err != nil {
		return 93
	}
	if err := os.WriteFile(filepath.Join(root, request.Operation+".response"), output.Bytes(), 0600); err != nil {
		return 94
	}
	target := request.Bundle.GoExecutable
	if mode == "dependency" {
		target = filepath.Join(request.Bundle.ModuleCacheDirectory, "fixture.txt")
	}
	if mode != "transport" && request.Operation == "ACQUIRE" && response.Status == "OK" {
		original, err := os.ReadFile(target)
		if err != nil {
			return 95
		}
		if err := os.WriteFile(filepath.Join(root, "original"), original, 0600); err != nil {
			return 96
		}
		if err := os.Chmod(target, 0700); err != nil {
			return 97
		}
		if err := os.WriteFile(target, append(original, []byte("\nAT14 drift\n")...), 0700); err != nil {
			return 98
		}
		if mode == "dependency" {
			if err := os.Chmod(target, 0444); err != nil {
				return 99
			}
		}
	}
	if mode != "transport" && request.Operation == "DISCOVER" {
		original, err := os.ReadFile(filepath.Join(root, "original"))
		if err != nil {
			return 100
		}
		if err := os.Chmod(target, 0700); err != nil {
			return 101
		}
		if err := os.WriteFile(target, original, 0700); err != nil {
			return 102
		}
		if mode == "dependency" {
			if err := os.Chmod(target, 0444); err != nil {
				return 103
			}
		}
	}
	if _, err := os.Stdout.Write(output.Bytes()); err != nil {
		return 104
	}
	return code
}

func qualificationDriftBundle(t *testing.T, mode string) authorityBundle {
	t.Helper()
	root := filepath.Join(resolvedTemp(t), driftFixtureSegment)
	repository := filepath.Join(root, "repository")
	goroot := filepath.Join(root, "goroot")
	modules := filepath.Join(root, "module-cache")
	temporary := filepath.Join(root, "temporary")
	toolDir := filepath.Join(goroot, "pkg", "tool", runtime.GOOS+"_"+runtime.GOARCH)
	for _, path := range []string{repository, modules, temporary, filepath.Join(goroot, "src", "runtime"), filepath.Join(goroot, "bin"), toolDir} {
		if err := os.MkdirAll(path, 0700); err != nil {
			t.Fatal(err)
		}
	}
	for path, body := range map[string]string{
		filepath.Join(root, "mode"):                           mode,
		filepath.Join(repository, "go.mod"):                   "module example.test/drift\n\ngo 1.27.0\n",
		filepath.Join(repository, "drift_test.go"):            "package drift\nimport \"testing\"\nfunc TestDrift(t *testing.T) {}\n",
		filepath.Join(goroot, "src", "runtime", "runtime.go"): "package runtime\n",
		filepath.Join(modules, "fixture.txt"):                 "original dependency\n",
		filepath.Join(toolDir, "asm"):                         "asm", filepath.Join(toolDir, "compile"): "compile", filepath.Join(toolDir, "link"): "link",
	} {
		if err := os.WriteFile(path, []byte(body), 0600); err != nil {
			t.Fatal(err)
		}
	}
	goExecutable := filepath.Join(goroot, "bin", executableName(runtime.GOOS))
	fixtureGoRoot, err := providerFixtureGoRoot()
	if err != nil {
		t.Fatal(err)
	}
	original, err := os.ReadFile(filepath.Join(fixtureGoRoot, "bin", executableName(runtime.GOOS)))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(goExecutable, original, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(filepath.Join(modules, "fixture.txt"), 0444); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(modules, 0555); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(modules, 0700); _ = os.Chmod(filepath.Join(modules, "fixture.txt"), 0600) })
	git := resolvedGitExecutable(t)
	runFixtureGit(t, git, repository, "init", "-q")
	runFixtureGit(t, git, repository, "add", ".")
	runFixtureGit(t, git, repository, "-c", "user.name=Corvint Test", "-c", "user.email=corvint@example.invalid", "commit", "-qm", "fixture")
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	executable = resolvedPath(t, executable)
	verifierSHA, err := regularFileDigest(executable)
	if err != nil {
		t.Fatal(err)
	}
	gitSHA, err := regularFileDigest(git)
	if err != nil {
		t.Fatal(err)
	}
	return authorityBundle{Profile: attachmentProfile, VerifierExecutable: executable, VerifierExecutableSHA256: verifierSHA, GitExecutable: git, GitExecutableSHA256: gitSHA, RepositoryRoot: repository, TemporaryParent: temporary, GoExecutable: goExecutable, GOROOT: goroot, GOOS: runtime.GOOS, GOARCH: runtime.GOARCH, ModuleCacheDirectory: modules, ModuleMode: string(provider.ModuleReadonly), Packages: []string{"example.test/drift"}, OutputLimitBytes: gorunner.MaxOutputBytes, TimeoutMilliseconds: gorunner.MaxRunTime.Milliseconds()}
}

func TestProductionParentDriftAndTransportAreDistinct(t *testing.T) {
	for _, mode := range []string{"dependency", "toolchain", "transport"} {
		t.Run(mode, func(t *testing.T) {
			bundle := qualificationDriftBundle(t, mode)
			transcript, err := executeProvider(context.Background(), bundle)
			root := filepath.Dir(bundle.ModuleCacheDirectory)
			var release attachmentResponse
			raw, readErr := os.ReadFile(filepath.Join(root, "RELEASE.response"))
			if readErr != nil {
				t.Fatalf("release marker: %v; Execute: %v", readErr, err)
			}
			if json.Unmarshal(raw, &release) != nil || release.Status != "OK" {
				t.Fatalf("clean release precondition: %s; Execute: %v", raw, err)
			}
			want, other := provider.ErrAuthorityDrift, provider.ErrAuthorityUnavailable
			detail := "PARENT_AUTHORITY_DRIFT"
			if mode == "transport" {
				want, other = other, want
				detail = "PARENT_AUTHORITY_UNAVAILABLE"
			} else {
				raw, readErr := os.ReadFile(filepath.Join(root, "DISCOVER.response"))
				if readErr != nil {
					t.Fatal(readErr)
				}
				var response attachmentResponse
				if json.Unmarshal(raw, &response) != nil || response.ErrorCode != "AUTHORITY_DRIFT" {
					t.Fatalf("real parent drift = %s", raw)
				}
			}
			if !errors.Is(err, want) || errors.Is(err, other) {
				t.Fatalf("Execute chain = %v, want only %v", err, want)
			}
			if transcript.Runner.Started || len(transcript.Receipt.Transcript) != 0 {
				t.Fatal("prelaunch refusal launched or composed receipt")
			}
			code, phase, got := prelaunchDiagnostic(err)
			if code != "IDENTITY_MISMATCH" || phase != "IDENTITY" || got != detail {
				t.Fatalf("diagnostic = %s %s %s", code, phase, got)
			}
			var diagnostic bytes.Buffer
			emitProviderFailure(&diagnostic, transcript, err)
			digest := bareID("go-live-error-detail", "go-live-error-detail/0", []byte(fmt.Sprintf(`{"code":%q,"detail":%q,"phase":%q}`, code, detail, phase)))
			if bytes.Count(diagnostic.Bytes(), []byte{'\n'}) != 1 || !strings.Contains(diagnostic.String(), digest) || strings.Contains(diagnostic.String(), "go-live-run/0") {
				t.Fatalf("wire refusal = %s", diagnostic.String())
			}
		})
	}
}

func TestUnavailableSourceDiagnosticMapping(t *testing.T) {
	code, phase, detail := prelaunchDiagnostic(fmt.Errorf("source B differs from observation A: %w", provider.ErrAuthorityUnavailable))
	if code != "IDENTITY_MISMATCH" || phase != "IDENTITY" || detail != "PARENT_AUTHORITY_UNAVAILABLE" {
		t.Fatalf("mapping = %s %s %s", code, phase, detail)
	}
}
