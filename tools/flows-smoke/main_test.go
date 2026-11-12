// SPDX-License-Identifier: Apache-2.0

package main

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/appflows"
	"github.com/Beamfall/corvint/internal/execfile"
)

const (
	boundedPlan     = `{"plan":{"e2e":{"scope":"BOUNDED","selected":[{"path":"e2e/checkout.spec.ts"}]}}}`
	unsupportedPlan = `{"plan":{"e2e":{"scope":"FULL","selected":[],"unknown":[{"reason":"FLOWS_SCHEMA_UNSUPPORTED","detail":".corvint/flows.json version 1; supported 0"}]}}}`
	refusal         = `{"code": "flows-schema-unsupported", "error": "FLOWS_SCHEMA_UNSUPPORTED: .corvint/flows.json: profile corvint-flows/1 is schema version 1", "ok": false}`
)

// candidate is a stand-in corvint: the outputs it gives for the supported and
// the unsupported fixture.
type candidate struct {
	supportedPlan, unsupportedPlan string
	supportedReport                result
	unsupportedReport              result
}

func (c candidate) run(root string, args ...string) result {
	unsupported := strings.HasSuffix(root, "unsupported")
	switch {
	case args[0] == "affected" && unsupported:
		return result{stdout: c.unsupportedPlan}
	case args[0] == "affected":
		return result{stdout: c.supportedPlan}
	case unsupported:
		return c.unsupportedReport
	default:
		return c.supportedReport
	}
}

var (
	supportedFixture   = fixture{root: "/w/supported", base: "b", e2e: fixtureE2E}
	unsupportedFixture = fixture{root: "/w/unsupported", base: "b", e2e: fixtureE2E}
	passing            = candidate{supportedPlan: boundedPlan, unsupportedPlan: unsupportedPlan,
		supportedReport: result{stdout: `{"schema":"report"}`}, unsupportedReport: result{code: 2, stderr: refusal}}
)

// FSV-V1-007 (V1-5410): a candidate that plans and reports as expected
// passes every check.
func TestFSVV1007PassingCandidate_V15410(t *testing.T) {
	if failures := checks(passing.run, supportedFixture, unsupportedFixture, 1); len(failures) != 0 {
		t.Fatalf("a passing candidate failed: %v", failures)
	}
}

// FSV-V1-007 (V1-5410): the smoke fails when the unsupported-version fixture
// plans fewer than all E2E tests, whether the reader treated the file as
// version 0, listed only some tests under FULL, or dropped the member; and
// when the reason is missing or a report prints partial output.
func TestFSVV1007FailsWhenUnsupportedPlansFewerThanAll_V15410(t *testing.T) {
	for name, tc := range map[string]struct {
		mutate func(*candidate)
		want   string
	}{
		"read as version 0": {func(c *candidate) { c.unsupportedPlan = boundedPlan }, "plans fewer than all E2E tests: plan.e2e is BOUNDED"},
		"partial FULL": {func(c *candidate) {
			c.unsupportedPlan = strings.Replace(unsupportedPlan, `"selected":[]`, `"selected":[{"path":"e2e/checkout.spec.ts"}]`, 1)
		}, "plans fewer than all E2E tests: plan.e2e is FULL, missing [e2e/search.spec.ts]"},
		"no e2e member": {func(c *candidate) { c.unsupportedPlan = `{"plan":{}}` }, "plans fewer than all E2E tests"},
		"invalid instead": {func(c *candidate) {
			c.unsupportedPlan = strings.Replace(unsupportedPlan, "FLOWS_SCHEMA_UNSUPPORTED", "FLOWS_DECLARATION_INVALID", 1)
		}, "does not name FLOWS_SCHEMA_UNSUPPORTED"},
		"partial report": {func(c *candidate) { c.unsupportedReport = result{code: 2, stdout: `{"gaps":[]}`, stderr: refusal} },
			"want a non-zero exit naming FLOWS_SCHEMA_UNSUPPORTED and no report"},
		"report not installed": {func(c *candidate) {
			c.supportedReport = result{code: 2, stderr: `{"code": "not-installed", "error": "corvint-docs is not installed"}`}
		}, "supported: corvint flows status --flows flows exited 2 without a report"},
		"supported widened": {func(c *candidate) { c.supportedPlan = unsupportedPlan }, "supported: plan.e2e is FULL"},
	} {
		c := passing
		tc.mutate(&c)
		failures := checks(c.run, supportedFixture, unsupportedFixture, 1)
		if !strings.Contains(strings.Join(failures, "\n"), tc.want) {
			t.Errorf("%s: failures %q lack %q", name, failures, tc.want)
		}
	}
}

// FSV-V1-007 (V1-5410): another corvint executable on the PATH fails the
// "installed alone" check.
func TestFSVV1007InstalledAlone_V15410(t *testing.T) {
	bin, other := t.TempDir(), t.TempDir()
	installed := filepath.Join(bin, "corvint")
	for _, name := range []string{installed, filepath.Join(other, "git")} {
		if err := execfile.WriteFile(name, nil, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	path := bin + string(os.PathListSeparator) + other
	if failure := aloneOnPath(path, installed); failure != "" {
		t.Fatalf("alone: %s", failure)
	}
	docs := filepath.Join(other, "corvint-docs")
	if err := execfile.WriteFile(docs, nil, 0o755); err != nil {
		t.Fatal(err)
	}
	if failure := aloneOnPath(path, installed); !strings.Contains(failure, docs) {
		t.Fatalf("a second binary was not named: %q", failure)
	}
}

// FSV-V1-007 (V1-5410): the fixture is pinned: two builds give the same
// commits, the two copies differ only in the profile, and the unsupported
// copy is one above the highest supported version.
func TestFSVV1007FixtureIsPinned_V15410(t *testing.T) {
	git, err := exec.LookPath("git")
	if err != nil {
		t.Skip("git is not installed")
	}
	work := t.TempDir()
	first, err := buildFixture(context.Background(), git, filepath.Join(work, "a"), appflows.FlowsWriteVersion())
	if err != nil {
		t.Fatal(err)
	}
	second, err := buildFixture(context.Background(), git, filepath.Join(work, "b"), appflows.FlowsWriteVersion())
	if err != nil {
		t.Fatal(err)
	}
	if first.base != second.base || first.head != second.head || len(first.base) != 40 {
		t.Fatalf("the fixture is not pinned: %+v %+v", first, second)
	}
	unsupported, err := buildFixture(context.Background(), git, filepath.Join(work, "unsupported"), appflows.FlowsWriteVersion()+1)
	if err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(filepath.Join(unsupported.root, filepath.FromSlash(appflows.FlowsDeclarationPath)))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := appflows.ReadFlowsDeclaration(unsupported.root); appflows.AsFlowsRefusal(err) == nil || !appflows.AsFlowsRefusal(err).Unsupported() {
		t.Fatalf("the unsupported copy %s is not refused as unsupported: %v", raw, err)
	}
	if declared, err := appflows.ReadFlowsDeclaration(first.root); err != nil || declared.Version != appflows.FlowsWriteVersion() {
		t.Fatalf("the supported copy: %+v %v", declared, err)
	}
}

// FSV-V1-007 (V1-5410): only the corvint member leaves the archive.
func TestFSVV1007ExtractsOnlyCorvint_V15410(t *testing.T) {
	var buffer bytes.Buffer
	zipped := gzip.NewWriter(&buffer)
	archive := tar.NewWriter(zipped)
	for name, body := range map[string]string{"corvint_linux_amd64/corvint": "binary", "corvint_linux_amd64/corvint-docs": "other", "corvint_linux_amd64/LICENSE": "license"} {
		if err := archive.WriteHeader(&tar.Header{Name: name, Mode: 0o755, Size: int64(len(body)), Typeflag: tar.TypeReg}); err != nil {
			t.Fatal(err)
		}
		if _, err := archive.Write([]byte(body)); err != nil {
			t.Fatal(err)
		}
	}
	if err := archive.Close(); err != nil {
		t.Fatal(err)
	}
	if err := zipped.Close(); err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	file := filepath.Join(dir, "corvint_linux_amd64.tar.gz")
	if err := os.WriteFile(file, buffer.Bytes(), 0o600); err != nil {
		t.Fatal(err)
	}
	bin := filepath.Join(dir, "bin")
	if err := os.Mkdir(bin, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := extractCorvint(file, filepath.Join(bin, "corvint")); err != nil {
		t.Fatal(err)
	}
	entries, err := os.ReadDir(bin)
	if err != nil || len(entries) != 1 || entries[0].Name() != "corvint" {
		t.Fatalf("extracted %v %v", entries, err)
	}
	if raw, _ := os.ReadFile(filepath.Join(bin, "corvint")); string(raw) != "binary" {
		t.Fatalf("extracted %q", raw)
	}
}
