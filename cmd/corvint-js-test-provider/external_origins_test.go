package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/testvaliditydoc/externalorigins"
)

const externalOriginsTestPolicy = `{"profile":"corvint-external-origins-policy/0","environment":["http://127.0.0.1:3002"],"deterministic":[{"origin":"https://assets.example-cdn.test","reason":"pinned content-hashed assets"}]}`

const externalOriginsTestRun = `{"profile":"corvint-network-evidence/0","run":"ci-1","revision":"0123456789abcdef0123456789abcdef01234567","tests":[
{"file":"tests/catalog.spec.ts","title":"catalog > renders","outcome":"failed","requests":[
{"url":"http://127.0.0.1:3002/catalog","resourceType":"document","status":200,"durationMs":5,"followUps":2},
{"url":"https://cdn.jsdelivr.example/npm/catalog-mfe@1/+esm","resourceType":"module","status":200,"durationMs":9000,"followUps":0},
{"url":"https://assets.example-cdn.test/app.3f9a1c.js","resourceType":"script","status":200,"durationMs":4,"followUps":0}]}]}`

func writeExternalOriginsInput(t *testing.T, dir, name, content string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

// PWP-V0-027: external-origins prints the corvint-external-origins/0 report, listing each external
// origin, and refuses a missing policy or an invalid evidence document with exit status 2.
func TestPWPV0027ExternalOriginsCommandPrintsReport(t *testing.T) {
	dir := t.TempDir()
	policy := writeExternalOriginsInput(t, dir, "policy.json", externalOriginsTestPolicy)
	network := writeExternalOriginsInput(t, dir, "run.json", externalOriginsTestRun)
	var stdout, stderr bytes.Buffer
	if err := runExternalOrigins([]string{"--policy", policy, "--network", network}, &stdout, &stderr); err != nil {
		t.Fatalf("external-origins: %v %s", err, stderr.String())
	}
	var report externalorigins.ExternalOriginsReport
	if err := json.Unmarshal(stdout.Bytes(), &report); err != nil {
		t.Fatal(err)
	}
	if report.Profile != externalorigins.ExternalOriginsReportProfile || report.Verdict != externalorigins.ExternalOriginsDependent ||
		len(report.Origins) != 2 || report.Origins[0].Origin != "https://assets.example-cdn.test" || report.Origins[0].Class != externalorigins.OriginAllowed ||
		report.Origins[1].Origin != "https://cdn.jsdelivr.example" || len(report.Findings) != 1 || report.Findings[0].FirstTest.Title != "catalog > renders" {
		t.Fatalf("report = %s", stdout.String())
	}

	invalid := writeExternalOriginsInput(t, dir, "invalid.json", strings.Replace(externalOriginsTestRun, `"module"`, `"wasm"`, 1))
	for name, args := range map[string][]string{
		"no policy":        {"--network", network},
		"no network":       {"--policy", policy},
		"positional":       {"--policy", policy, "--network", network, "extra"},
		"invalid evidence": {"--policy", policy, "--network", invalid},
		"missing file":     {"--policy", policy, "--network", filepath.Join(dir, "absent.json")},
		"invalid policy":   {"--policy", network, "--network", network},
	} {
		stdout.Reset()
		stderr.Reset()
		if err := runExternalOrigins(args, &stdout, &stderr); !errors.Is(err, errExternalOriginsRefused) || stdout.Len() != 0 || stderr.Len() == 0 {
			t.Errorf("%s: err %v stdout %q stderr %q", name, err, stdout.String(), stderr.String())
		}
	}
	stdout.Reset()
	stderr.Reset()
	_ = runExternalOrigins([]string{"--policy", policy, "--network", invalid}, &stdout, &stderr)
	if !strings.Contains(stderr.String(), externalorigins.NetworkEvidenceInvalid) {
		t.Errorf("invalid evidence refusal does not name its code: %s", stderr.String())
	}

	var usage bytes.Buffer
	printUsage(&usage)
	for _, want := range []string{"external-origins ", "Usage of external-origins:", "-policy", "-network", "PWP-V0-027"} {
		if !strings.Contains(usage.String(), want) {
			t.Errorf("usage omits %q", want)
		}
	}
}
