package appflows

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"testing"

	"github.com/Beamfall/corvint/internal/jstestprovider"
)

func TestAFUV1012QualifiedReceiptIngest(t *testing.T) {
	t.Run("AFU-V1-012 retains earlier attempts from the external profile", func(t *testing.T) {
		r := qualifiedAttemptFixture(t)
		raw, err := jstestprovider.EncodeQualified(r)
		if err != nil {
			t.Fatal(err)
		}
		got, err := IngestRunEvidence(FormatPlaywrightReceipt, raw, attemptHeader())
		if err != nil || got.Incomplete != "" || len(got.Records) != 1 {
			t.Fatalf("ingest %v %+v", err, got)
		}
		record := got.Records[0]
		if record.TestKey != r.Tests[0].ID || record.Project != "chromium" || record.Authority != AuthorityIngested || Classify(record.Attempts) != "flaky" || len(record.Attempts) != 2 || record.Attempts[0].DurationMS != 20 || record.Attempts[0].Failure != "expected checkout" || len(record.Attempts[0].Attachments) != 1 || len(record.Attempts[0].AssertionAnchors) != 1 {
			t.Fatalf("lost attempt evidence: %+v", record)
		}
		r.Tests[0].Attempts[0].State = jstestprovider.StateInfrastructure
		r.Tests[0].AttemptDetails[0].State = jstestprovider.StateInfrastructure
		raw, err = jstestprovider.EncodeQualified(r)
		if err != nil {
			t.Fatal(err)
		}
		if _, err = IngestRunEvidence(FormatPlaywrightReceipt, raw, attemptHeader()); err == nil {
			t.Fatal("unsupported infrastructure converted into a test outcome")
		}
	})
}

func attemptHeader() RunHeader {
	h := runHeader()
	h.RunnerVersion = "1.63.0"
	return h
}

func qualifiedAttemptFixture(t *testing.T) jstestprovider.Receipt {
	t.Helper()
	a := &jstestprovider.Anchor{File: "checkout.spec.ts", Line: 36}
	r := jstestprovider.Receipt{Profile: jstestprovider.AttemptExternalProfile, Kind: "e2e",
		Identity: jstestprovider.Identity{RunnerName: "playwright", RunnerVersion: "1.63.0", NodeVersion: "v22.23.2", ConfigFile: "/fixture/config.cjs", ConfigDigest: "config", ConfigInputDigests: map[string]string{"/fixture/config.cjs": "config"}, TestFileDigests: map[string]string{a.File: "test"}, Argv: []string{"playwright", "test"}},
		External: &jstestprovider.ExternalLifecycle{Ownership: "external", CleanupResponsibility: "external", ServerDescendants: "unknown", ReadyAtStart: true, ReadyAtPublish: true, ReadyURL: "http://127.0.0.1:3002", DeclaredAppIdentity: "fixture", ConfigOverride: "controlled-fixture", InputsUnchanged: true, RunnerDescendantsGone: true}}
	r.Tests = []jstestprovider.TestOutcome{{Name: "checkout", FullName: "checkout", Project: &jstestprovider.ProjectIdentity{Name: "chromium", Browser: "chromium", Device: "unknown", ConfigDigest: "config", Use: json.RawMessage(`{"browserName":"chromium","channel":"","headless":true,"launchOptions":{},"corvintBrowser":{"platform":"darwin","arch":"arm64","nodeVersion":"v22.23.2","browserType":"chromium","browserVersion":"Google Chrome for Testing 153.0.8010.12","channel":"","executableSource":"playwright-bundled","executableName":"chromium-headless-shell","executablePath":"/portable/cache/ms-playwright/chromium_headless_shell-1243/chrome-headless-shell-mac-arm64/chrome-headless-shell","executableSha256":"a0bfe7b4da4787b66058477d696cd1d09065d25f06a548947722b9af77ee8282","browserRevision":"1243","manifestBrowserVersion":"153.0.8010.12","headlessShellAvailable":true}}`)}, State: jstestprovider.StateFlaky, Retries: 1, DurationMS: 10, Anchor: a,
		Attempts:       []jstestprovider.Attempt{{State: jstestprovider.StateFailed, Retry: 0}, {State: jstestprovider.StatePassed, Retry: 1}},
		AttemptDetails: []jstestprovider.AttemptDetail{{State: jstestprovider.StateFailed, Retry: 0, DurationMS: 20, FailureMessage: "expected checkout", Anchor: a, Artifacts: []jstestprovider.FailureArtifact{{Name: "screen", Path: "result.png"}}}, {State: jstestprovider.StatePassed, Retry: 1, DurationMS: 10, Anchor: a}}}}
	bindAttemptID(&r)
	return r
}

func bindAttemptID(r *jstestprovider.Receipt) {
	test := &r.Tests[0]
	raw, _ := json.Marshal(struct {
		Identity jstestprovider.Identity
		Project  *jstestprovider.ProjectIdentity
		Anchor   *jstestprovider.Anchor
		FullName string
	}{r.Identity, test.Project, test.Anchor, test.FullName})
	digest := sha256.Sum256(raw)
	test.ID = hex.EncodeToString(digest[:])
}

func TestAFUV1012QualifiedReceiptRefusals(t *testing.T) {
	for name, mutate := range map[string]func(*jstestprovider.Receipt){
		"readiness":          func(r *jstestprovider.Receipt) { r.External.ReadyAtStart = false },
		"ownership":          func(r *jstestprovider.Receipt) { r.External.Ownership = "unknown" },
		"unqualified runner": func(r *jstestprovider.Receipt) { r.Identity.RunnerVersion = "1.50.0"; bindAttemptID(r) },
		"legacy runner":      func(r *jstestprovider.Receipt) { r.Identity.RunnerVersion = "1.60.0"; bindAttemptID(r) },
		"forged test ID":     func(r *jstestprovider.Receipt) { r.Tests[0].ID = "forged" },
	} {
		t.Run(name, func(t *testing.T) {
			r := qualifiedAttemptFixture(t)
			mutate(&r)
			raw, err := jstestprovider.EncodeQualified(r)
			if err != nil {
				t.Fatal(err)
			}
			h := attemptHeader()
			h.RunnerVersion = r.Identity.RunnerVersion
			if _, err := IngestRunEvidence(FormatPlaywrightReceipt, raw, h); err == nil {
				t.Fatal("unqualified receipt became run evidence")
			}
		})
	}
	t.Run("genuine failed control survives", func(t *testing.T) {
		r := qualifiedAttemptFixture(t)
		o := &r.Tests[0]
		o.State = jstestprovider.StateFailed
		o.Retries = 0
		o.DurationMS = 20
		o.FailureMessage = o.AttemptDetails[0].FailureMessage
		o.Artifacts = o.AttemptDetails[0].Artifacts
		o.Attempts = o.Attempts[:1]
		o.AttemptDetails = o.AttemptDetails[:1]
		raw, err := jstestprovider.EncodeQualified(r)
		if err != nil {
			t.Fatal(err)
		}
		got, err := IngestRunEvidence(FormatPlaywrightReceipt, raw, attemptHeader())
		if err != nil || len(got.Records) != 1 || Classify(got.Records[0].Attempts) != "failed" {
			t.Fatalf("failed control lost: %+v %v", got, err)
		}
	})
}
