package jstestprovider

import (
	"context"
	"encoding/json"
	"flag"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/testvalidity"
)

func TestApplicationAttestationProviderHelper(t *testing.T) {
	if os.Getenv("CORVINT_APPLICATION_ATTESTATION_HELPER") != "1" {
		return
	}
	separator := slices.Index(os.Args, "--")
	if separator < 0 || separator+1 >= len(os.Args) {
		os.Exit(2)
	}
	data, err := os.ReadFile(os.Args[separator+1])
	if err != nil {
		os.Exit(3)
	}
	_, _ = os.Stdout.Write(data)
	os.Exit(0)
}

func TestApplicationAttestationCommandProvider(t *testing.T) {
	t.Setenv("CORVINT_APPLICATION_ATTESTATION_HELPER", "1")
	root := t.TempDir()
	configPath := filepath.Join(root, "config.json")
	statePath := filepath.Join(root, "state.json")
	expectation := applicationExpectationFixture()
	writeCanonicalFixture(t, configPath, applicationAttestationConfig{Profile: applicationAttestationConfigProfile, Expectation: expectation})
	before := applicationAttestationFixture()
	writeCanonicalFixture(t, statePath, before)
	provider, err := prepareApplicationAttestationProvider(ApplicationAttestationProvider{
		Argv:       []string{os.Args[0], "-test.run=TestApplicationAttestationProviderHelper", "--", statePath},
		ConfigFile: configPath,
	}, root, filepath.Join(root, "scratch"), map[string]string{"FIXTURE": "bound", "CORVINT_APPLICATION_ATTESTATION_HELPER": "1"})
	if err == nil {
		// prepare stages into an existing caller-owned scratch directory.
		t.Fatal("provider unexpectedly accepted a missing scratch directory")
	}
	if err := os.Mkdir(filepath.Join(root, "scratch"), 0o700); err != nil {
		t.Fatal(err)
	}
	provider, err = prepareApplicationAttestationProvider(ApplicationAttestationProvider{
		Argv:       []string{os.Args[0], "-test.run=TestApplicationAttestationProviderHelper", "--", statePath},
		ConfigFile: configPath,
	}, root, filepath.Join(root, "scratch"), map[string]string{"FIXTURE": "bound", "CORVINT_APPLICATION_ATTESTATION_HELPER": "1"})
	if err != nil {
		t.Fatal(err)
	}
	first, failure := provider.observe(context.Background())
	if failure != "" || first == nil || first.OutputDigest == "" || provider.receipt.Provider.ExecutableDigest == "" || provider.receipt.Provider.ConfigDigest == "" {
		t.Fatalf("first=%+v failure=%q provider=%+v", first, failure, provider.receipt.Provider)
	}
	after := before
	after.Instance.StartGeneration = "generation-2"
	writeCanonicalFixture(t, statePath, after)
	second, failure := provider.observe(context.Background())
	if failure != "" || second == nil || !slices.Contains(compareApplicationAttestations(first.Attestation, second.Attestation), "application-restarted") {
		t.Fatalf("restart was not detected: second=%+v failure=%q", second, failure)
	}
}

func TestApplicationAttestationNegativeControls(t *testing.T) {
	expectation := applicationExpectationFixture()
	healthy := applicationAttestationFixture()
	for name, row := range map[string]struct {
		mutate func(*ApplicationAttestation)
		want   string
	}{
		"wrong revision":      {func(actual *ApplicationAttestation) { actual.Repository.Revision = oid('9') }, "application-revision-mismatch"},
		"wrong image":         {func(actual *ApplicationAttestation) { actual.Build.Digest = digest('9') }, "application-build-mismatch"},
		"unhealthy":           {func(actual *ApplicationAttestation) { actual.Health.State = "unhealthy" }, "application-attestation-unhealthy"},
		"missing instance":    {func(actual *ApplicationAttestation) { actual.Instance.ID = "" }, "application-attestation-identity-unavailable"},
		"contradictory clean": {func(actual *ApplicationAttestation) { actual.Repository.DirtyDigest = digest('8') }, "application-attestation-contradictory"},
	} {
		t.Run(name, func(t *testing.T) {
			actual := healthy
			row.mutate(&actual)
			reason := validateAttestationShape(actual)
			if reason == "" {
				reason = compareExpectation(expectation, actual)
			}
			if reason != row.want {
				t.Fatalf("reason=%q want=%q", reason, row.want)
			}
		})
	}
}

func TestAttestedReceiptNeverPassesWrongOrRestartedApplication(t *testing.T) {
	for name, mutate := range map[string]func(*Receipt){
		"wrong revision": func(receipt *Receipt) {
			receipt.ApplicationAttestation.Before.Attestation.Repository.Revision = oid('9')
			refreshObservation(receipt.ApplicationAttestation.Before)
		},
		"wrong image": func(receipt *Receipt) {
			receipt.ApplicationAttestation.Before.Attestation.Build.Digest = digest('9')
			refreshObservation(receipt.ApplicationAttestation.Before)
		},
		"restarted": func(receipt *Receipt) {
			receipt.ApplicationAttestation.After.Attestation.Instance.StartGeneration = "generation-2"
			refreshObservation(receipt.ApplicationAttestation.After)
		},
		"provider unavailable": func(receipt *Receipt) {
			receipt.ApplicationAttestation.After = nil
			receipt.ApplicationAttestation.Failures = []string{"application-attestation-provider-unavailable"}
		},
		"malformed provider digest": func(receipt *Receipt) {
			receipt.ApplicationAttestation.Provider.ExecutableDigest = "x"
		},
		"contradictory provider argv": func(receipt *Receipt) {
			receipt.ApplicationAttestation.Provider.Argv[0] = "/other-provider"
		},
		"relative provider config": func(receipt *Receipt) {
			receipt.ApplicationAttestation.Provider.ConfigPath = "provider.json"
		},
		"changed expectation with stale config digest": func(receipt *Receipt) {
			receipt.ApplicationAttestation.Expectation.Repository.Revision = oid('9')
		},
	} {
		t.Run(name, func(t *testing.T) {
			receipt := qualifiedAttestedFixture(t)
			mutate(&receipt)
			if ReceiptTestProjection(receipt, receipt.Tests[0]).Execution.State == testvalidity.ExecutionPassed {
				t.Fatal("unqualified application projected passing evidence")
			}
		})
	}
}

func qualifiedAttestedFixture(t *testing.T) Receipt {
	t.Helper()
	receipt := qualifiedFixture(t)
	receipt.Profile = AttestedExternalProfile
	receipt.External.DeclaredAppIdentity = ""
	attestation := applicationAttestationFixture()
	before := &ApplicationAttestationObservation{Attestation: attestation}
	after := &ApplicationAttestationObservation{Attestation: attestation}
	refreshObservation(before)
	refreshObservation(after)
	receipt.ApplicationAttestation = &ApplicationAttestationReceipt{
		Provider: ApplicationAttestationProviderIdentity{
			Profile: ApplicationAttestationProviderProfile, Argv: []string{"/provider"}, ExecutablePath: "/provider", ExecutableDigest: sha256Hex([]byte("provider")), ConfigPath: "/provider.json", ConfigDigest: canonicalConfigDigest(applicationExpectationFixture()), Environment: map[string]string{},
		},
		Expectation: applicationExpectationFixture(), Before: before, After: after, Failures: []string{},
	}
	testRepository := ApplicationRepositoryIdentity{RootCommit: oid('6'), Revision: oid('7'), Tree: oid('8'), DirtyState: "clean"}
	receipt.TestRepositoryAtStart = &testRepository
	testRepositoryAtPublish := testRepository
	receipt.TestRepositoryAtPublish = &testRepositoryAtPublish
	return receipt
}

func TestObserveTestRepositoryIgnoresAmbientGitRedirects(t *testing.T) {
	git, err := exec.LookPath("git")
	if err != nil {
		t.Fatal(err)
	}
	wanted := initAttestationGitRepository(t, git, "wanted")
	redirect := initAttestationGitRepository(t, git, "redirect")
	wantedIdentity := gitTestOutput(t, git, wanted, "rev-parse", "HEAD")
	redirectIdentity := gitTestOutput(t, git, redirect, "rev-parse", "HEAD")
	if wantedIdentity == redirectIdentity {
		t.Fatal("fixture repositories unexpectedly share a revision")
	}
	t.Setenv("GIT_DIR", filepath.Join(redirect, ".git"))
	t.Setenv("GIT_WORK_TREE", redirect)
	observed, failure := observeTestRepository(context.Background(), wanted)
	if failure != "" || observed == nil || observed.Revision != wantedIdentity {
		t.Fatalf("observed=%+v failure=%q wanted=%s redirect=%s", observed, failure, wantedIdentity, redirectIdentity)
	}
}

func TestQualifiedProfilesRejectMixedIdentityShapes(t *testing.T) {
	legacy := qualifiedFixture(t)
	attested := qualifiedAttestedFixture(t)
	legacy.ApplicationAttestation = attested.ApplicationAttestation
	legacy.TestRepositoryAtStart = attested.TestRepositoryAtStart
	legacy.TestRepositoryAtPublish = attested.TestRepositoryAtPublish
	if _, err := EncodeQualified(legacy); err == nil || ReceiptTestProjection(legacy, legacy.Tests[0]).Execution.State == testvalidity.ExecutionPassed {
		t.Fatal("legacy profile accepted attested identity fields")
	}
	attested.External.DeclaredAppIdentity = "legacy-identity"
	if _, err := EncodeQualified(attested); err == nil || ReceiptTestProjection(attested, attested.Tests[0]).Execution.State == testvalidity.ExecutionPassed {
		t.Fatal("attested profile accepted legacy application identity")
	}
}

func canonicalConfigDigest(expectation ApplicationAttestationExpectation) string {
	data, _ := json.Marshal(applicationAttestationConfig{Profile: applicationAttestationConfigProfile, Expectation: expectation})
	return sha256Hex(append(data, '\n'))
}

func initAttestationGitRepository(t *testing.T, git, content string) string {
	t.Helper()
	repository := t.TempDir()
	if err := os.WriteFile(filepath.Join(repository, "bound.txt"), []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	for _, arguments := range [][]string{{"init", "-q"}, {"config", "user.email", "fixture@example.invalid"}, {"config", "user.name", "Corvint Fixture"}, {"add", "bound.txt"}, {"commit", "-q", "-m", content}} {
		command := exec.Command(git, append([]string{"-C", repository}, arguments...)...)
		command.Env = gitObservationEnvironment()
		if output, err := command.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v: %s", arguments, err, output)
		}
	}
	return repository
}

func gitTestOutput(t *testing.T, git, repository string, arguments ...string) string {
	t.Helper()
	command := exec.Command(git, append([]string{"-C", repository}, arguments...)...)
	command.Env = gitObservationEnvironment()
	output, err := command.Output()
	if err != nil {
		t.Fatal(err)
	}
	return strings.TrimSpace(string(output))
}

func applicationExpectationFixture() ApplicationAttestationExpectation {
	return ApplicationAttestationExpectation{
		Repository:    ApplicationRepositoryExpectation{RootCommit: oid('1'), Revision: oid('2'), Tree: oid('3'), DirtyPolicy: "require-clean"},
		Build:         ApplicationArtifactIdentity{Kind: "image", Digest: digest('4')},
		Configuration: ApplicationArtifactIdentity{Kind: "compose", Digest: digest('5')},
		InstanceKind:  "docker-container",
	}
}

func applicationAttestationFixture() ApplicationAttestation {
	return ApplicationAttestation{
		Profile:       ApplicationAttestationProfile,
		Repository:    ApplicationRepositoryIdentity{RootCommit: oid('1'), Revision: oid('2'), Tree: oid('3'), DirtyState: "clean"},
		Build:         ApplicationArtifactIdentity{Kind: "image", Digest: digest('4')},
		Configuration: ApplicationArtifactIdentity{Kind: "compose", Digest: digest('5')},
		Instance:      ApplicationInstanceIdentity{Kind: "docker-container", ID: "container-1", StartGeneration: "generation-1"},
		Health:        ApplicationHealth{State: "healthy"},
	}
}

func refreshObservation(observation *ApplicationAttestationObservation) {
	data, _ := json.Marshal(observation.Attestation)
	observation.OutputDigest = sha256Hex(append(data, '\n'))
}

func writeCanonicalFixture(t *testing.T, path string, value any) {
	t.Helper()
	data, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, append(data, '\n'), 0o600); err != nil {
		t.Fatal(err)
	}
}

func oid(value byte) string    { return string(makeFilled(40, value)) }
func digest(value byte) string { return "sha256:" + string(makeFilled(64, value)) }

func makeFilled(length int, value byte) []byte {
	result := make([]byte, length)
	for index := range result {
		result[index] = value
	}
	return result
}

var dockerQualification = flag.Bool("corvint-docker-qualification", false, "run the explicit Docker Compose application-attestation qualification")
