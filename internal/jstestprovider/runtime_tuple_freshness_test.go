package jstestprovider

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/testvalidity"
)

// fresh164Fixture is the freshness fixture moved onto the 1.64.0 bundled
// headless shell and the given Node version.
func fresh164Fixture(t *testing.T, node string) Receipt {
	t.Helper()
	r := freshFixture(t)
	r.Identity.RunnerVersion = "1.64.0"
	r.Identity.NodeVersion = node
	r.Freshness.IdentityAfter = &r.Identity
	use := string(r.Tests[0].Project.Use)
	for _, pair := range [][2]string{
		{`"nodeVersion":"v22.23.3"`, `"nodeVersion":"` + node + `"`},
		{"153.0.8010.12", "156.0.8078.4"},
		{"chromium_headless_shell-1243", "chromium_headless_shell-1248"},
		{"a0bfe7b4da4787b66058477d696cd1d09065d25f06a548947722b9af77ee8282", "77d8278b1ac9d0687fad4e817ea83023ede09b1deef36e1864b49ba194ac9634"},
		{`"browserRevision":"1243"`, `"browserRevision":"1248"`},
	} {
		if !strings.Contains(use, pair[0]) {
			t.Fatalf("fixture lacks %s", pair[0])
		}
		use = strings.ReplaceAll(use, pair[0], pair[1])
	}
	r.Tests[0].Project.Use = []byte(use)
	r.Tests[0].ID = qualifiedTestID(r.Identity, r.Tests[0])
	return r
}

// PWP-V0-050: one exact tuple, Playwright 1.64.0 on Node v22.23.2 with the
// bundled browser, admits the external receipt and the freshness receipt
// from the same table; freshness on v24.11.1 stays unadmitted.
func TestPWPV0050OneTupleAdmitsExternalAndFreshness(t *testing.T) {
	for _, profile := range []string{ExternalProfile, AttemptExternalProfile, SensitiveExternalProfile, FreshnessProfile} {
		if m := AdmittedRuntimeTupleMismatch(profile, "1.64.0", qualifiedNodeVersion, BrowserSourceBundled); m != nil {
			t.Fatalf("%s refused the shared tuple: %+v", profile, m)
		}
	}
	if got := ReceiptRuntimeTuple(Receipt{Profile: ExternalProfile, Identity: Identity{RunnerVersion: "1.64.0", NodeVersion: qualifiedNodeVersion}}); got != RuntimeTupleCandidate {
		t.Fatalf("external receipt on the shared tuple read %s", got)
	}
	current := func(r Receipt) string { return ReceiptTestProjection(r, r.Tests[0]).Freshness.State }
	if got := current(fresh164Fixture(t, qualifiedNodeVersion)); got != testvalidity.FreshnessCurrent {
		t.Fatalf("freshness on the shared tuple read %s", got)
	}
	if got := current(fresh164Fixture(t, admittedBundledNodeVersion)); got != testvalidity.FreshnessUnknown {
		t.Fatalf("freshness on 1.64.0/v24.11.1 without a live run read %s", got)
	}
	if got := current(fresh164Fixture(t, freshnessNodeVersion)); got != testvalidity.FreshnessUnknown {
		t.Fatalf("freshness on 1.64.0/v22.23.3 without a row read %s", got)
	}
	if !runtimeTupleAdmitted(FreshnessProfile, "1.63.0", freshnessNodeVersion, BrowserSourceBundled) {
		t.Fatal("the 1.63.0 freshness row was dropped")
	}
}

// PWP-V0-051: a freshness command executable is digested in a stream up to
// its own bound, so an official Node binary above the 64 MiB attestation
// read bound still binds, while an oversized or non-regular file refuses.
func TestPWPV0051FreshCommandExecutableStreamDigest(t *testing.T) {
	dir := t.TempDir()
	big := filepath.Join(dir, "node")
	size := int64(applicationAttestationExecutableLimit) + 1024
	f, err := os.Create(big)
	if err != nil {
		t.Fatal(err)
	}
	chunk := bytes.Repeat([]byte("corvint"), 1<<16)
	h := sha256.New()
	for written := int64(0); written < size; {
		n := min(int64(len(chunk)), size-written)
		if _, err := f.Write(chunk[:n]); err != nil {
			t.Fatal(err)
		}
		h.Write(chunk[:n])
		written += n
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
	want := hex.EncodeToString(h.Sum(nil))
	if got, err := digestBoundedRegularFile(big, freshCommandExecutableLimit); err != nil || got != want {
		t.Fatalf("over-64MiB executable: %q %v", got, err)
	}
	if _, err := digestBoundedRegularFile(big, size-1); err == nil {
		t.Fatal("file above the bound was digested")
	}
	if got, err := digestBoundedRegularFile(big, size); err != nil || got != want {
		t.Fatalf("file at the bound: %q %v", got, err)
	}
	small := filepath.Join(dir, "small")
	if err := os.WriteFile(small, []byte("node"), 0o600); err != nil {
		t.Fatal(err)
	}
	if got, err := digestBoundedRegularFile(small, freshCommandExecutableLimit); err != nil || got != sha256Hex([]byte("node")) {
		t.Fatalf("small file digest %q %v", got, err)
	}
	link := filepath.Join(dir, "link")
	if err := os.Symlink(small, link); err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{dir, link, filepath.Join(dir, "missing")} {
		if _, err := digestBoundedRegularFile(path, freshCommandExecutableLimit); err == nil {
			t.Fatalf("%s digested as a regular file", path)
		}
	}
	entry := filepath.Join(dir, "entry.cjs")
	if err := os.WriteFile(entry, []byte("entry"), 0o600); err != nil {
		t.Fatal(err)
	}
	got := freshCommandObserve(FreshCommandIdentity{Argv: []string{big, entry}})
	if got == nil || got.ExecutableDigest != want || got.EntrypointDigest != sha256Hex([]byte("entry")) {
		t.Fatalf("command observation over 64 MiB: %+v", got)
	}
}

// PWP-V0-052: the freshness reporter receives its (runner, Node) rows from
// the shared table, and its pre-filter admits exactly those rows.
func TestPWPV0052ReporterFreshnessTuplesFromTable(t *testing.T) {
	want := [][2]string{{"1.63.0", freshnessNodeVersion}, {"1.64.0", qualifiedNodeVersion}}
	got := freshnessReporterTuples()
	if len(got) != len(want) || got[0] != want[0] || got[1] != want[1] {
		t.Fatalf("freshness reporter tuples %v", got)
	}
	scratch := t.TempDir()
	config, _, _, err := externalCommand(E2EConfig{Config: Config{ConfigFile: "/repo/playwright.config.cjs"}, Freshness: &FreshnessConfig{}}, scratch)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(config, `, freshnessProfile:true, freshnessTuples:[["1.63.0","v22.23.3"],["1.64.0","v22.23.2"]]}`) {
		t.Fatalf("freshness config lacks the table tuples:\n%s", config)
	}
	if strings.Contains(string(qualifiedReporter), `'v22.23.3'`) {
		t.Fatal("reporter repeats a freshness Node literal")
	}
	node, err := exec.LookPath("node")
	if err != nil {
		t.Skip("node unavailable for the reporter pre-filter check")
	}
	reporter := filepath.Join(scratch, "reporter.cjs")
	if err := os.WriteFile(reporter, qualifiedReporter, 0o600); err != nil {
		t.Fatal(err)
	}
	tuples, _ := json.Marshal(got)
	script := `const fs = require('node:fs'); const m = {exports: {}};
new Function('module', 'exports', 'require', fs.readFileSync(process.argv[1], 'utf8') + '\nmodule.exports.q = qualifiedBundledIdentity;')(m, m.exports, Object.assign(id => id === 'playwright' ? {} : require(id), {resolve: id => id}));
const tuples = JSON.parse(process.argv[2]);
const browser = (rev, version, sha, node) => ({platform: 'darwin', arch: 'arm64', nodeVersion: node, browserType: 'chromium', browserVersion: 'Google Chrome for Testing ' + version, channel: '', executableSource: 'playwright-bundled', executableName: 'chromium-headless-shell', executablePath: '/cache/chromium_headless_shell-' + rev + '/chrome-headless-shell-mac-arm64/chrome-headless-shell', executableSha256: sha, browserRevision: rev, manifestBrowserVersion: version, headlessShellAvailable: true});
const b164 = node => browser('1248', '156.0.8078.4', '77d8278b1ac9d0687fad4e817ea83023ede09b1deef36e1864b49ba194ac9634', node);
const b163 = node => browser('1243', '153.0.8010.12', 'a0bfe7b4da4787b66058477d696cd1d09065d25f06a548947722b9af77ee8282', node);
const q = m.exports.q;
process.stdout.write(JSON.stringify([
  !!q(b164('v22.23.2'), '1.64.0', true, tuples), !!q(b163('v22.23.3'), '1.63.0', true, tuples),
  !!q(b164('v24.11.1'), '1.64.0', true, tuples), !!q(b164('v22.23.3'), '1.64.0', true, tuples),
  !!q(b163('v22.23.2'), '1.63.0', true, tuples), !!q(b164('v22.23.2'), '1.64.0', true, []),
  !!q(b164('v24.11.1'), '1.64.0', false, [])]));`
	out, err := exec.Command(node, "-e", script, reporter, string(tuples)).CombinedOutput()
	if err != nil {
		t.Fatalf("reporter evaluation: %v\n%s", err, out)
	}
	if string(out) != "[true,true,false,false,false,false,true]" {
		t.Fatalf("reporter freshness pre-filter %s", out)
	}
}
