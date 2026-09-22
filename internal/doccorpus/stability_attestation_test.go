package doccorpus

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/jstestprovider"
)

func TestPlaywrightStabilityAttestedProfile(t *testing.T) {
	for _, mode := range []string{"valid", "cross-repository", "revision-mismatch", "test-revision-mismatch", "invalid-attestation"} {
		t.Run(mode, func(t *testing.T) {
			root, manifest := stabilityFixture(t, func(_ int, receipt *jstestprovider.Receipt) {
				testRevision := receipt.External.DeclaredAppIdentity
				if mode == "cross-repository" || mode == "revision-mismatch" {
					receipt.External.DeclaredAppIdentity = strings.Repeat("9", 40)
				}
				addStabilityAttestation(receipt)
				receipt.TestRepositoryAtStart.Revision = testRevision
				receipt.TestRepositoryAtPublish.Revision = testRevision
				if mode == "invalid-attestation" {
					receipt.ApplicationAttestation.Before.OutputDigest = strings.Repeat("0", 64)
				}
				if mode == "test-revision-mismatch" {
					receipt.TestRepositoryAtStart.Revision = strings.Repeat("8", 40)
					receipt.TestRepositoryAtPublish.Revision = strings.Repeat("8", 40)
				}
			}, func(registry *StabilityRegistry) {
				if mode == "cross-repository" {
					for i := range registry.Aggregates[0].Contributions {
						registry.Aggregates[0].Contributions[i].Identity.ApplicationRevision = strings.Repeat("9", 40)
					}
				}
			})
			artifact, err := Build(context.Background(), root, manifest)
			if mode != "valid" && mode != "cross-repository" {
				if err == nil {
					t.Fatal("invalid attested aggregate accepted")
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if len(artifact.StabilityEvidence) != 1 || artifact.StabilityEvidence[0].Verdict != "clean" {
				t.Fatal("valid attested receipts did not aggregate")
			}
		})
	}
}

func addStabilityAttestation(r *jstestprovider.Receipt) {
	revision := r.External.DeclaredAppIdentity
	r.Profile = jstestprovider.AttestedExternalProfile
	r.External.DeclaredAppIdentity = ""
	repository := jstestprovider.ApplicationRepositoryIdentity{RootCommit: revision, Revision: revision, Tree: revision, DirtyState: "clean"}
	a := jstestprovider.ApplicationAttestation{
		Profile: jstestprovider.ApplicationAttestationProfile, Repository: repository,
		Build:         jstestprovider.ApplicationArtifactIdentity{Kind: "image", Digest: "sha256:" + strings.Repeat("4", 64)},
		Configuration: jstestprovider.ApplicationArtifactIdentity{Kind: "compose", Digest: "sha256:" + strings.Repeat("5", 64)},
		Instance:      jstestprovider.ApplicationInstanceIdentity{Kind: "docker-container", ID: "fixture", StartGeneration: "generation-1"},
		Health:        jstestprovider.ApplicationHealth{State: "healthy"},
	}
	expectation := jstestprovider.ApplicationAttestationExpectation{Repository: jstestprovider.ApplicationRepositoryExpectation{RootCommit: revision, Revision: revision, Tree: revision, DirtyPolicy: "require-clean"}, Build: a.Build, Configuration: a.Configuration, InstanceKind: a.Instance.Kind}
	config := struct {
		Profile     string                                           `json:"profile"`
		Expectation jstestprovider.ApplicationAttestationExpectation `json:"expectation"`
	}{"corvint-application-attestation-config/0", expectation}
	configData, _ := json.Marshal(config)
	aData, _ := json.Marshal(a)
	before := &jstestprovider.ApplicationAttestationObservation{Attestation: a, OutputDigest: strings.TrimPrefix(Digest(append(aData, '\n')), "sha256:")}
	after := *before
	r.ApplicationAttestation = &jstestprovider.ApplicationAttestationReceipt{
		Provider:    jstestprovider.ApplicationAttestationProviderIdentity{Profile: jstestprovider.ApplicationAttestationProviderProfile, Argv: []string{"/provider"}, ExecutablePath: "/provider", ExecutableDigest: strings.Repeat("6", 64), ConfigPath: "/provider.json", ConfigDigest: strings.TrimPrefix(Digest(append(configData, '\n')), "sha256:"), Environment: map[string]string{}},
		Expectation: expectation, Before: before, After: &after, Failures: []string{},
	}
	r.TestRepositoryAtStart = &repository
	end := repository
	r.TestRepositoryAtPublish = &end
}
