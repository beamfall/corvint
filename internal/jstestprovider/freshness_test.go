package jstestprovider

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/procgroup"
	"github.com/Beamfall/corvint/internal/testvalidity"
)

func freshFixture(t *testing.T) Receipt {
	t.Helper()
	r := qualifiedAttestedFixture(t)
	r.Profile = FreshnessProfile
	r.Identity.NodeVersion = "v22.23.3"
	r.Identity.RunnerVersion = "1.63.0"
	r.Identity.Environment = map[string]string{"PATH": "/bin", "LANG": "C", "LC_ALL": "C", "TMPDIR": "/tmp"}
	r.Identity.PackageDigest = sha256Hex([]byte("package"))
	r.Identity.ConfigDigest = sha256Hex([]byte("config"))
	r.Identity.TestFileDigests[r.Tests[0].Anchor.File] = sha256Hex([]byte("test"))
	r.Identity.ConfigInputDigests = map[string]string{r.Identity.ConfigFile: r.Identity.ConfigDigest}
	r.Tests[0].Project.ConfigDigest = r.Identity.ConfigDigest
	r.Tests[0].Project.Use = json.RawMessage(`{"baseURL":"http://127.0.0.1:3002","browserName":"chromium","channel":"","headless":true,"corvintBrowser":{"platform":"darwin","arch":"arm64","nodeVersion":"v22.23.3","browserType":"chromium","browserVersion":"Google Chrome for Testing 153.0.8010.12","channel":"","executableSource":"playwright-bundled","executableName":"chromium-headless-shell","executablePath":"/cache/chromium_headless_shell-1243/chrome-headless-shell-mac-arm64/chrome-headless-shell","executableSha256":"a0bfe7b4da4787b66058477d696cd1d09065d25f06a548947722b9af77ee8282","browserRevision":"1243","manifestBrowserVersion":"153.0.8010.12","headlessShellAvailable":true}}`)
	cmd := FreshCommandIdentity{Argv: []string{"/node", "/entry.cjs"}, ExecutableDigest: sha256Hex([]byte("node")), EntrypointDigest: sha256Hex([]byte("entry"))}
	r.Identity.Argv = append([]string{}, cmd.Argv...)
	bind := FreshCommandBinding{Expected: cmd, Before: &cmd, After: &cmd}
	source := []byte("<button>one</button>")
	digest := sha256Hex(source)
	serverInput, _ := json.Marshal(struct {
		Response []byte `json:"response"`
	}{source})
	gone := true
	product := r.ApplicationAttestation.Before.Attestation.Repository
	process := FreshProcessIdentity{PID: 42, Start: "observed-start"}
	served := FreshServeObservation{URL: r.External.ReadyURL, Process: process, Body: source}
	r.Freshness = &FreshnessBinding{ServerInputDigest: sha256Hex(append(serverInput, '\n')), ProductExpected: product, TestExpected: *r.TestRepositoryAtStart, ProductBefore: &product, ProductAfter: &product, ArtifactPath: "app.html", Source: source, SourceDigest: digest, ArtifactBefore: digest, ArtifactAfter: digest, DocumentURL: r.External.ReadyURL, Leader: &process, ServedBefore: &served, ServedAfter: &served, Runner: bind, Server: bind, Observer: bind, ObserverConfigBefore: sha256Hex([]byte("observer-config")), ObserverConfigAfter: sha256Hex([]byte("observer-config")), EnvironmentBefore: r.Identity.Environment, EnvironmentAfter: r.Identity.Environment, IdentityAfter: &r.Identity, ServerGone: &gone, Failures: []string{}}
	r.ApplicationAttestation.Expectation.Build = ApplicationArtifactIdentity{Kind: "source", Digest: "sha256:" + digest}
	r.ApplicationAttestation.Expectation.Configuration = ApplicationArtifactIdentity{Kind: "source", Digest: "sha256:" + r.Identity.ConfigDigest}
	r.ApplicationAttestation.Expectation.InstanceKind = "process"
	r.ApplicationAttestation.Provider.ConfigDigest = canonicalConfigDigest(r.ApplicationAttestation.Expectation)
	for _, o := range []*ApplicationAttestationObservation{r.ApplicationAttestation.Before, r.ApplicationAttestation.After} {
		o.Attestation.Configuration = r.ApplicationAttestation.Expectation.Configuration
		o.Attestation.Build = r.ApplicationAttestation.Expectation.Build
		o.Attestation.Instance = ApplicationInstanceIdentity{Kind: "process", ID: "42", StartGeneration: process.Start}
		refreshObservation(o)
	}
	r.DescendantObservation = &procgroup.DescendantObservation{Scope: "fixture", IntervalMS: 20, Absent: true, Limitations: []string{"sampled"}}
	r.Freshness.ServerDescendants = r.DescendantObservation
	manifest := &FreshDependencyManifest{Roots: []string{"/cache/node_modules/playwright"}, Files: map[string]string{"/cache/node_modules/playwright/lib/impl.js": sha256Hex([]byte("implementation")), cmd.Argv[1]: cmd.EntrypointDigest, r.Tests[0].Anchor.File: r.Identity.TestFileDigests[r.Tests[0].Anchor.File], r.Identity.ConfigFile: r.Identity.ConfigDigest}}
	observed := &FreshDependencyObservation{Files: manifest.Files, Failures: []string{}}
	r.Freshness.Dependencies = manifest
	r.Freshness.DependencyDigest = FreshDependencyDigest(manifest)
	r.Freshness.DependenciesBefore = observed
	r.Freshness.DependenciesAfter = observed
	r.Tests[0].ImportedFiles = map[string]string{r.Tests[0].Anchor.File: manifest.Files[r.Tests[0].Anchor.File], "/cache/node_modules/playwright/lib/impl.js": manifest.Files["/cache/node_modules/playwright/lib/impl.js"]}
	for _, served := range []*FreshServeObservation{r.Freshness.ServedBefore, r.Freshness.ServedAfter} {
		served.ServerImports = map[string]string{cmd.Argv[1]: cmd.EntrypointDigest}
		served.ObserverImports = map[string]string{cmd.Argv[1]: cmd.EntrypointDigest}
	}

	r.Tests[0].ID = qualifiedTestID(r.Identity, r.Tests[0])
	return r
}

// PTF-V0-001/002/003/004: distinguish actual mismatches from absent facts and
// retain the first mismatch when a later observation is unavailable.
func TestPTFV0ObservedCurrency(t *testing.T) {
	for name, row := range map[string]struct {
		mutate func(*Receipt)
		want   string
	}{
		"complete":                           {func(*Receipt) {}, testvalidity.FreshnessCurrent},
		"manifest unavailable":               {func(r *Receipt) { r.Freshness.Dependencies = nil }, testvalidity.FreshnessUnknown},
		"dependency publication unavailable": {func(r *Receipt) { r.Freshness.DependenciesAfter = nil }, testvalidity.FreshnessUnknown},
		"dependency observed drift": {func(r *Receipt) {
			o := *r.Freshness.DependenciesAfter
			o.Files = map[string]string{}
			for path, digest := range r.Freshness.Dependencies.Files {
				o.Files[path] = digest
			}
			o.Files["/cache/node_modules/playwright/lib/impl.js"] = sha256Hex([]byte("changed"))
			r.Freshness.DependenciesAfter = &o
		}, testvalidity.FreshnessStale},
		"worker import unsupported": {func(r *Receipt) {
			r.Tests[0].ImportedFiles = map[string]string{"/unapproved/import.js": sha256Hex([]byte("module"))}
		}, testvalidity.FreshnessUnknown},
		"worker import drift": {func(r *Receipt) {
			r.Tests[0].ImportedFiles = map[string]string{r.Tests[0].Anchor.File: r.Identity.TestFileDigests[r.Tests[0].Anchor.File], "/cache/node_modules/playwright/lib/impl.js": sha256Hex([]byte("changed"))}
		}, testvalidity.FreshnessStale},
		"observer import unavailable": {func(r *Receipt) {
			o := *r.Freshness.ServedBefore
			o.ObserverImports = nil
			r.Freshness.ServedBefore = &o
		}, testvalidity.FreshnessUnknown},
		"server dependency unsupported": {func(r *Receipt) {
			o := *r.Freshness.ServedBefore
			o.ServerImports = map[string]string{"/entry.cjs": r.Freshness.Server.Expected.EntrypointDigest, "/other/import.js": sha256Hex([]byte("module"))}
			r.Freshness.ServedBefore = &o
		}, testvalidity.FreshnessUnknown},
		"missing binding": {func(r *Receipt) { r.Freshness = nil }, testvalidity.FreshnessUnknown},
		"missing publish": {func(r *Receipt) { r.Freshness.ProductAfter = nil }, testvalidity.FreshnessUnknown},
		"product changed": {func(r *Receipt) { p := *r.Freshness.ProductAfter; p.Revision = oid('9'); r.Freshness.ProductAfter = &p }, testvalidity.FreshnessStale},
		"test dirty": {func(r *Receipt) {
			p := *r.TestRepositoryAtPublish
			p.DirtyState = "dirty"
			r.TestRepositoryAtPublish = &p
		}, testvalidity.FreshnessStale},
		"old served artifact": {func(r *Receipt) {
			s := *r.Freshness.ServedBefore
			s.Body = []byte("old")
			r.Freshness.ServedBefore = &s
		}, testvalidity.FreshnessStale},
		"foreign healthy server": {func(r *Receipt) { s := *r.Freshness.ServedBefore; s.Process.PID++; r.Freshness.ServedBefore = &s }, testvalidity.FreshnessStale},
		"source drift then unavailable": {func(r *Receipt) {
			r.Freshness.ArtifactBefore = sha256Hex([]byte("old"))
			r.Freshness.ProductAfter = nil
		}, testvalidity.FreshnessStale},
		"config drift": {func(r *Receipt) {
			i := r.Identity
			i.ConfigDigest = sha256Hex([]byte("changed"))
			r.Freshness.IdentityAfter = &i
		}, testvalidity.FreshnessStale},
		"missing runner bytes": {func(r *Receipt) { r.Freshness.Runner.After = nil }, testvalidity.FreshnessUnknown},
		"runner changed": {func(r *Receipt) {
			c := *r.Freshness.Runner.After
			c.EntrypointDigest = sha256Hex([]byte("changed"))
			r.Freshness.Runner.After = &c
		}, testvalidity.FreshnessStale},
		"environment changed": {func(r *Receipt) {
			r.Freshness.EnvironmentAfter = map[string]string{"PATH": "/other", "LANG": "C", "LC_ALL": "C", "TMPDIR": "/tmp"}
		}, testvalidity.FreshnessStale},
		"cleanup unavailable": {func(r *Receipt) { r.Freshness.ServerGone = nil }, testvalidity.FreshnessUnknown},
		"old node tuple": {func(r *Receipt) {
			r.Identity.NodeVersion = "v22.23.2"
			r.Freshness.IdentityAfter = &r.Identity
			r.Tests[0].ID = qualifiedTestID(r.Identity, r.Tests[0])
		}, testvalidity.FreshnessUnknown},
		"native row changed": {func(r *Receipt) { r.Tests[0].ID = "different" }, testvalidity.FreshnessUnknown},
	} {
		t.Run(name, func(t *testing.T) {
			r := freshFixture(t)
			row.mutate(&r)
			got := ReceiptTestProjection(r, r.Tests[0])
			if got.Freshness.State != row.want {
				t.Fatalf("got %+v want %s", got, row.want)
			}
			if got.Strength.State == testvalidity.StrengthKilled {
				t.Fatal("currency invented strength")
			}
		})
	}
}

// PTF-V0-005: canonical byte equality rejects duplicate and unknown members,
// contradictory projections and base64-hidden source secrets.
func TestPTFV0ClosedCodec(t *testing.T) {
	r := freshFixture(t)
	data, err := EncodeFreshness(r)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := DecodeFreshness(data); err != nil {
		t.Fatal(err)
	}
	for name, mutated := range map[string][]byte{
		"duplicate":     bytes.Replace(data, []byte(`"profile":"corvint-playwright-freshness/0"`), []byte(`"profile":"corvint-playwright-freshness/0","profile":"corvint-playwright-freshness/0"`), 1),
		"unknown":       bytes.Replace(data, []byte(`{"receipt":{`), []byte(`{"receipt":{"unrecognized":true,`), 1),
		"projection":    bytes.Replace(data, []byte(`"state":"CURRENT"`), []byte(`"state":"STALE"`), 1),
		"whitespace":    append([]byte(" "), data...),
		"wrong profile": bytes.Replace(data, []byte(FreshnessProfile), []byte(AttestedExternalProfile), 1),
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := DecodeFreshness(mutated); err == nil {
				t.Fatal("accepted altered envelope")
			}
		})
	}
	if _, err := EncodeQualified(r); err == nil {
		t.Fatal("legacy codec admitted new profile")
	}
	r.Freshness.Source = []byte("ghp_" + strings.Repeat("a", 36))
	if _, err := EncodeFreshness(r); err == nil {
		t.Fatal("decoded source secret escaped screening")
	}
}

func TestPTFV0ResponseDeltaAndAssertion(t *testing.T) {
	source := []byte("<p>one</p>")
	mutant := []byte("<p>two</p>")
	d := ResponseMutationDefinition{ArtifactPath: "app.html", SourceSHA256: sha256Hex(source), From: "one", To: "two", MutantSHA256: sha256Hex(mutant)}
	if got, e := ResponseMutation(source, "app.html", d); e != nil || !bytes.Equal(got, mutant) {
		t.Fatal("approved response derivation failed")
	}
	for name, change := range map[string]func(*ResponseMutationDefinition){"path": func(d *ResponseMutationDefinition) { d.ArtifactPath = "other" }, "source": func(d *ResponseMutationDefinition) { d.SourceSHA256 = sha256Hex([]byte("old")) }, "mutant": func(d *ResponseMutationDefinition) { d.MutantSHA256 = sha256Hex([]byte("other")) }, "absent": func(d *ResponseMutationDefinition) { d.From = "absent" }, "no-op": func(d *ResponseMutationDefinition) { d.To = d.From }} {
		t.Run(name, func(t *testing.T) {
			altered := d
			change(&altered)
			if _, e := ResponseMutation(source, "app.html", altered); e == nil {
				t.Fatal("unbound delta accepted")
			}
		})
	}
	if _, e := ResponseMutation([]byte("one one"), "app.html", d); e == nil {
		t.Fatal("ambiguous delta accepted")
	}
	row := TestOutcome{State: StateFailed, FailureMessage: "Error: PTF-ASSERTION:counter-one\n\x1b[31mexpect(locator).toHaveText(expected) failed\x1b[0m", Attempts: []Attempt{{State: StateFailed, FailureKind: "assertion-or-test"}}}
	if !TargetAssertionFailure(row, "counter-one") {
		t.Fatal("native target assertion not recognized")
	}
	for _, message := range []string{"Error: PTF-ASSERTION:other\nexpect(locator).toHaveText(expected) failed", "Error: PTF-ASSERTION:counter-one\nharness failed", "Error: other\nPTF-ASSERTION:counter-one\nexpect(locator).toHaveText(expected) failed"} {
		row.FailureMessage = message
		if TargetAssertionFailure(row, "counter-one") {
			t.Fatal("unrelated failure counted as target kill")
		}
	}
}

// PTF-V0-001/003/007: an approved complete tree is reread independently; a
// changed imported implementation, an extra input, or a symlink cannot match.
func TestPTFV0DependencyManifestBound(t *testing.T) {
	root, e := filepath.EvalSymlinks(t.TempDir())
	if e != nil {
		t.Fatal(e)
	}
	path := filepath.Join(root, "implementation.cjs")
	source := []byte("module.exports=1;\n")
	if e := os.WriteFile(path, source, 0600); e != nil {
		t.Fatal(e)
	}
	manifest := &FreshDependencyManifest{Roots: []string{root}, Files: map[string]string{path: sha256Hex(source)}}
	if runtime.GOOS != "darwin" && runtime.GOOS != "linux" {
		if observed := ObserveFreshDependencies(manifest); len(observed.Failures) == 0 || FreshDependenciesMatch(manifest, observed) {
			t.Fatal("unsupported platform admitted manifest")
		}
		return
	}
	if !FreshDependenciesMatch(manifest, ObserveFreshDependencies(manifest)) {
		t.Fatal("complete approved manifest not observed")
	}
	if e := os.WriteFile(path, []byte("module.exports=2;\n"), 0600); e != nil {
		t.Fatal(e)
	}
	observed := ObserveFreshDependencies(manifest)
	if FreshDependenciesMatch(manifest, observed) || observed.Files[path] == manifest.Files[path] {
		t.Fatal("actual imported file drift missed")
	}
	os.WriteFile(path, source, 0600)
	extra := filepath.Join(root, "extra.cjs")
	os.WriteFile(extra, source, 0600)
	if FreshDependenciesMatch(manifest, ObserveFreshDependencies(manifest)) {
		t.Fatal("undeclared installed input ignored")
	}
	os.Remove(extra)
	if e := os.Symlink(path, extra); e != nil {
		t.Fatal(e)
	}
	if observed := ObserveFreshDependencies(manifest); len(observed.Failures) == 0 || FreshDependenciesMatch(manifest, observed) {
		t.Fatal("unsupported dependency symlink admitted")
	}
}
