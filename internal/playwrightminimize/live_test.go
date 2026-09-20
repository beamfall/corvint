//go:build darwin || linux

package playwrightminimize

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/Beamfall/corvint/internal/jstestprovider"
)

func TestPSMLiveQualifiedEvidenceAndNegativeControls(t *testing.T) {
	for _, mode := range []string{"valid", "schedule", "workers", "retry", "application", "browser", "seed", "test-revision", "boolean-only"} {
		t.Run(mode, func(t *testing.T) {
			r, b := liveBaselineFixture()
			id := r.Tests[0].ID
			switch mode {
			case "schedule":
				r.Schedule.Starts[0].FullName = "different test"
			case "workers":
				r.Schedule.Workers++
			case "retry":
				r.Schedule.Starts[0].Retry++
			case "application":
				r.ApplicationAttestation.Before.OutputDigest = strings.Repeat("0", 64)
			case "browser":
				b.Identity.Fixed.BrowserVersion = "different browser"
			case "seed":
				b.Identity.Fixed.SeedIdentity = digestBytes([]byte("other seed"))
			case "test-revision":
				b.Identity.Fixed.TestRevision = strings.Repeat("9", 40)
			case "boolean-only":
				r = jstestprovider.Receipt{Profile: jstestprovider.AttestedExternalProfile}
			}
			err := validateLiveBaseline(&r, b, id, nil)
			if (err == nil) != (mode == "valid") {
				t.Fatalf("mode %s: %v", mode, err)
			}
		})
	}
}

func TestPSMLiveAuthorizationPrecedesAllMutation(t *testing.T) {
	_, err := ExecuteLive(context.Background(), LivePlan{}, "")
	if err == nil || err.Error() != "operator-authorization-required" {
		t.Fatalf("authorization: %v", err)
	}
	var input LiveRequest
	for _, data := range []string{`{"unknown":true}`, `{} garbage`, `{} {}`} {
		if DecodeLiveRequest([]byte(data), &input) == nil {
			t.Fatalf("admitted %s", data)
		}
	}
}

func TestPSMLiveResetFailureStillCleansUp(t *testing.T) {
	root := t.TempDir()
	reset := commandFixture(t, root, "reset", "#!/bin/sh\nexit 7\n")
	marker := filepath.Join(root, "cleaned")
	cleanup := commandFixture(t, root, "cleanup", "#!/bin/sh\nprintf cleaned > '"+marker+"'\n")
	runner := liveRunner{input: LiveRequest{Reset: reset, Cleanup: cleanup, Config: jstestprovider.E2EConfig{Config: jstestprovider.Config{Dir: root}}}}
	result, err := runner.Run(context.Background(), Trial{ID: "reset-failure"})
	if err == nil || result.ResetSucceeded || !result.CleanupSucceeded || result.Live.Reset.Exit != 7 {
		t.Fatalf("result=%+v err=%v", result, err)
	}
	if _, err := os.Stat(marker); err != nil {
		t.Fatal(err)
	}
	if result.Digest != receiptDigest(result) {
		t.Fatal("cleanup evidence was not bound")
	}
}

func TestPSMLiveCancellationReapsDescendants(t *testing.T) {
	root := t.TempDir()
	pidPath := filepath.Join(root, "child.pid")
	command := commandFixture(t, root, "reset", "#!/bin/sh\nsleep 60 &\nchild=$!\ntrap 'kill \"$child\" 2>/dev/null; wait \"$child\" 2>/dev/null' EXIT INT TERM\nprintf '%s' \"$child\" > '"+pidPath+"'\nwait \"$child\"\n")
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan ProcessEvidence, 1)
	go func() { done <- runCommand(ctx, command, root, Trial{ID: "cancel"}) }()
	deadline := time.Now().Add(3 * time.Second)
	var pidData []byte
	for time.Now().Before(deadline) {
		pidData, _ = os.ReadFile(pidPath)
		if len(pidData) > 0 {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	cancel()
	observation := <-done
	if len(pidData) == 0 || processSucceeded(observation) || !observation.Cleanup {
		t.Fatalf("cancellation lost cleanup: %+v", observation)
	}
	pid, err := strconv.Atoi(string(pidData))
	if err != nil {
		t.Fatal(err)
	}
	if err := syscall.Kill(pid, 0); err != syscall.ESRCH {
		t.Fatalf("descendant %d remains: %v", pid, err)
	}
}

func TestPSMLiveCommandDriftRefusesExecution(t *testing.T) {
	root := t.TempDir()
	command := commandFixture(t, root, "reset", "#!/bin/sh\nexit 0\n")
	if err := os.WriteFile(command.Argv[0], []byte("#!/bin/sh\nexit 1\n"), 0700); err != nil {
		t.Fatal(err)
	}
	if runCommand(context.Background(), command, root, Trial{}).Started {
		t.Fatal("changed executable ran")
	}
}

func commandFixture(t *testing.T, root, name, source string) Command {
	t.Helper()
	path := filepath.Join(root, name)
	if err := os.WriteFile(path, []byte(source), 0700); err != nil {
		t.Fatal(err)
	}
	return Command{Argv: []string{path}, ExecutableSHA256: digestBytes([]byte(source))}
}

func liveBaselineFixture() (jstestprovider.Receipt, Baseline) {
	revision := strings.Repeat("1", 40)
	hash := strings.Repeat("2", 64)
	seed := digestBytes([]byte("seed"))
	repository := jstestprovider.ApplicationRepositoryIdentity{RootCommit: revision, Revision: revision, Tree: revision, DirtyState: "clean"}
	attestation := jstestprovider.ApplicationAttestation{Profile: jstestprovider.ApplicationAttestationProfile, Repository: repository, Build: jstestprovider.ApplicationArtifactIdentity{Kind: "image", Digest: "sha256:" + hash}, Configuration: jstestprovider.ApplicationArtifactIdentity{Kind: "compose", Digest: "sha256:" + hash}, Instance: jstestprovider.ApplicationInstanceIdentity{Kind: "docker-container", ID: "fixture", StartGeneration: "1"}, Health: jstestprovider.ApplicationHealth{State: "healthy"}}
	expectation := jstestprovider.ApplicationAttestationExpectation{Repository: jstestprovider.ApplicationRepositoryExpectation{RootCommit: revision, Revision: revision, Tree: revision, DirtyPolicy: "require-clean"}, Build: attestation.Build, Configuration: attestation.Configuration, InstanceKind: "docker-container"}
	configData, _ := json.Marshal(struct {
		Profile     string                                           `json:"profile"`
		Expectation jstestprovider.ApplicationAttestationExpectation `json:"expectation"`
	}{"corvint-application-attestation-config/0", expectation})
	attestationData, _ := json.Marshal(attestation)
	observation := jstestprovider.ApplicationAttestationObservation{Attestation: attestation, OutputDigest: strings.TrimPrefix(digestBytes(append(attestationData, '\n')), "sha256:")}
	after := observation
	r := jstestprovider.Receipt{Profile: jstestprovider.AttestedExternalProfile, Kind: "e2e", Identity: jstestprovider.Identity{ConfigFile: "/repo/config.cjs", ConfigDigest: hash, ConfigInputDigests: map[string]string{"/repo/config.cjs": hash}, TestFileDigests: map[string]string{"/repo/test.cjs": hash}, RunnerName: "playwright", RunnerVersion: "1.63.0", NodeVersion: "v22.23.2", Argv: []string{"playwright", "test"}, Environment: map[string]string{"CORVINT_MINIMIZER_SEED": seed}}, External: &jstestprovider.ExternalLifecycle{ReadyURL: "http://127.0.0.1:1234", Ownership: "external", CleanupResponsibility: "external", ServerDescendants: "unknown", ReadyAtStart: true, ReadyAtPublish: true, RunnerDescendantsGone: true, InputsUnchanged: true, ConfigOverride: "qualified"}, TestRepositoryAtStart: &repository, TestRepositoryAtPublish: &repository,
		ApplicationAttestation: &jstestprovider.ApplicationAttestationReceipt{Provider: jstestprovider.ApplicationAttestationProviderIdentity{Profile: jstestprovider.ApplicationAttestationProviderProfile, Argv: []string{"/provider"}, ExecutablePath: "/provider", ExecutableDigest: hash, ConfigPath: "/provider.json", ConfigDigest: strings.TrimPrefix(digestBytes(append(configData, '\n')), "sha256:"), Environment: map[string]string{}}, Expectation: expectation, Before: &observation, After: &after, Failures: []string{}}}
	test := jstestprovider.TestOutcome{Name: "target", FullName: "project > target", State: jstestprovider.StateFailed, Anchor: &jstestprovider.Anchor{File: "/repo/test.cjs", Line: 1}, Project: &jstestprovider.ProjectIdentity{Name: "project", Browser: "chromium", Device: "unknown", Use: json.RawMessage(`{"corvintBrowser":{"browserVersion":"Google Chrome 153.0.8010.48"}}`), ConfigDigest: hash}, Attempts: []jstestprovider.Attempt{{State: jstestprovider.StateFailed, Retry: 0, FailureKind: "assertion-or-test"}}}
	test.Project.Use = json.RawMessage(`{"browserName":"chromium","channel":"","launchOptions":{"executablePath":"/Applications/Google Chrome.app/Contents/MacOS/Google Chrome"},"corvintBrowser":{"platform":"darwin","arch":"arm64","nodeVersion":"v22.23.2","browserType":"chromium","browserVersion":"Google Chrome 153.0.8010.48","channel":"","executablePath":"/Applications/Google Chrome.app/Contents/MacOS/Google Chrome","headlessShellAvailable":true}}`)
	test.ID = strings.TrimPrefix(digestJSON(struct {
		Identity jstestprovider.Identity
		Project  *jstestprovider.ProjectIdentity
		Anchor   *jstestprovider.Anchor
		FullName string
	}{r.Identity, test.Project, test.Anchor, test.FullName}), "sha256:")
	r.Tests = []jstestprovider.TestOutcome{test}
	r.Schedule = &jstestprovider.ExecutionSchedule{Workers: 1, Starts: []jstestprovider.ExecutionStart{{FullName: test.FullName, File: test.Anchor.File, Line: 1, Project: "project", Worker: 0}}}
	b := Baseline{Outcome: OutcomeFailed, Identity: RunIdentity{Fixed: FixedIdentity{TestRevision: revision, ApplicationRevision: revision, ConfigDigest: "sha256:" + hash, Runner: "playwright", RunnerVersion: "1.63.0", Browser: "chromium", BrowserVersion: "Google Chrome 153.0.8010.48", Project: "project", FixtureSchema: "playwright-use", FixtureDigest: digestBytes(test.Project.Use), SeedIdentity: seed, ApplicationAttestationDigest: "sha256:" + observation.OutputDigest}, Order: []string{test.ID}, WorkerTopology: WorkerTopology{Workers: 1, Policy: "fixed"}}}
	return r, b
}
