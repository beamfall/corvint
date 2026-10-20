//go:build darwin || linux

package cli_test

import (
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/tasks/obligation"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// TestTOLV0127_LiveFixtureQualifies (TOL-V0-127, 128) runs the embedded
// TOL-V0-060 fixture unchanged with an installed Playwright and the json
// reporter, checks the live report with qualify-version and then witnesses a
// report of that version, refused before unless the version is shipped
// (TOL-V0-126) and admitted after. It
// needs CORVINT_PLAYWRIGHT_MODULES (node_modules holding @playwright/test) and
// CORVINT_TOL_QUALIFY_NODE (an absolute node binary with a sibling npx, placed
// first on PATH); without them it is NOT_RUN, never qualification success.
func TestTOLV0127_LiveFixtureQualifies(t *testing.T) {
	modules, node := os.Getenv("CORVINT_PLAYWRIGHT_MODULES"), os.Getenv("CORVINT_TOL_QUALIFY_NODE")
	if modules == "" || node == "" {
		t.Skip("NOT_RUN: set CORVINT_PLAYWRIGHT_MODULES and CORVINT_TOL_QUALIFY_NODE for the live qualify-version run")
	}
	if !filepath.IsAbs(node) || filepath.Base(node) != "node" {
		t.Fatalf("CORVINT_TOL_QUALIFY_NODE must be an absolute path to a node binary, got %q", node)
	}
	npx := filepath.Join(filepath.Dir(node), "npx")
	t.Setenv("PATH", filepath.Dir(node)+string(os.PathListSeparator)+os.Getenv("PATH"))
	if resolved, err := exec.LookPath("node"); err != nil || resolved != node {
		t.Fatalf("PATH resolves node to %q, want %q: %v", resolved, node, err)
	}
	raw, err := os.ReadFile(filepath.Join(modules, "@playwright", "test", "package.json"))
	if err != nil {
		t.Fatal(err)
	}
	var pkg struct {
		Version string `json:"version"`
	}
	if err = json.Unmarshal(raw, &pkg); err != nil || pkg.Version == "" {
		t.Fatalf("@playwright/test version: %q %v", pkg.Version, err)
	}
	nodeVersion, err := exec.Command(node, "--version").Output()
	if err != nil {
		t.Fatal(err)
	}

	project, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(filepath.Join(project, obligation.FixtureName), obligation.Fixture(), 0o600); err != nil {
		t.Fatal(err)
	}
	if err = os.Symlink(modules, filepath.Join(project, "node_modules")); err != nil {
		t.Fatal(err)
	}
	report := filepath.Join(t.TempDir(), "report.json")
	cmd := exec.Command(npx, "playwright", "test", obligation.FixtureName, "--reporter=json")
	cmd.Dir = project
	cmd.Env = append(os.Environ(), "PLAYWRIGHT_JSON_OUTPUT_NAME="+report, "CI=1")
	out, err := cmd.CombinedOutput()
	// The fixture holds soft failures and a retry-0 failure, so Playwright
	// exits 1; any other status is a broken run, not a qualification.
	var exit *exec.ExitError
	if !errors.As(err, &exit) || exit.ExitCode() != 1 {
		t.Fatalf("playwright run: %v\n%s", err, out)
	}

	r, id, head := obligationRepo(t)
	seedObligations(t, r.Root, id)
	spec := pwSpec{Title: "AC-1 login", ID: "s1", File: "e2e/login.spec.ts", Tests: []pwTest{passed("chromium")}}
	witness := writeReport(t, t.TempDir(), r.Root, pkg.Version, spec)
	if !slices.Contains(obligation.QualifiedVersions(), pkg.Version) {
		before := atm(t, r.Root, nil, witnessArgs(id, "w-before", obligationRevision(t, r.Root, id), head, "--from-playwright-report", witness)...)
		if before.res.Outcome == wire.OutcomeOK || !hasCode(before.res, wire.CodeUnsupportedVersion) {
			t.Fatalf("version %s admitted before qualification: %s", pkg.Version, before.stdout)
		}
	}

	x := qualifyVersion(t, r.Root, report)
	if x.res.Outcome != wire.OutcomeOK || len(x.res.Items) != 1 {
		t.Fatalf("live qualify-version refused: %s", x.stdout)
	}
	it := x.res.Items[0]
	if field(it, "version").Str != pkg.Version || field(it, "fixtureSha256").Str != string(obligation.FixtureSha256()) ||
		strList(it, "credited") != "TOLQ-1,TOLQ-3,TOLQ-6,TOLQ-8" || strList(it, "failed") != "TOLQ-2,TOLQ-4,TOLQ-5,TOLQ-7" ||
		strList(it, "defectConfirmed") != "TOLQ-9" {
		t.Fatalf("live qualification item: %s", x.stdout)
	}
	t.Logf("TOL live qualification: node %s playwright %s reportSha256 %s fixtureSha256 %s", strings.TrimSpace(string(nodeVersion)),
		pkg.Version, field(it, "reportSha256").Str, field(it, "fixtureSha256").Str)

	after := atm(t, r.Root, nil, witnessArgs(id, "w-after", obligationRevision(t, r.Root, id), head, "--from-playwright-report", witness)...)
	if after.res.Outcome != wire.OutcomeOK || strList(after.res.Items[0], "credited") != "AC-1" || !auditConsistent(t, r.Root) {
		t.Fatalf("witness after live qualification: %s", after.stdout)
	}
}
