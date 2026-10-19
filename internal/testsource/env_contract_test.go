package testsource

import (
	"context"
	"fmt"
	"strings"
	"testing"
)

const envBootstrap = `#!/usr/bin/env bash
set -euo pipefail
# Ports for the e2e profile.
BASE_PORT=4100
export APP_URL="http://localhost:${BASE_PORT}"
PAYMENTS_STUB_PORT=$((BASE_PORT + 10))
export PAYMENTS_URL="http://localhost:${PAYMENTS_STUB_PORT}"
export BUILD_SHA=$(git rev-parse HEAD)
export REGION=${DEPLOY_REGION}
PORT=$PAYMENTS_STUB_PORT node stubs/payments/server.js &
API_TOKEN=test-token npx playwright test "$@"
`

const envStub = `const express = require('express');
const app = express();
let faults = {};
app.post('/charges', (req, res) => res.json({ id: 'ch_1' }));
app.post('/__control/faults', (req, res) => {
  const { testId, status, delayMs } = req.body;
  faults[testId] = { status, delayMs };
  res.sendStatus(204);
});
app.get('/__control/requests', (req, res) => res.json(log.filter(r => r.testId === req.query.testId)));
app.post('/__control/reset', (req, res) => { faults = {}; res.sendStatus(204); });
app.listen(process.env.PORT);
`

const envSpec = `import { test, expect } from '@playwright/test';
const declared = process.env.PAYMENTS_URL;
const offset = 'http://localhost:' + (Number(process.env.BASE_PORT) + 10);
test('pays', async ({ page }) => {
  await fetch('http://localhost:4110/__control/faults', { method: 'POST' });
});
`

func envFixture() map[string]string {
	return map[string]string{
		"scripts/bootstrap-e2e.sh":   envBootstrap,
		"stubs/payments/server.js":   envStub,
		"tests/pay.spec.ts":          envSpec,
		"docker-compose.e2e.yml":     "services: {}\n",
		"scripts/bootstrap-local.sh": "export APP_URL=http://localhost:3000\n",
	}
}

func envVariable(t *testing.T, report EnvContractReport, name string) EnvVariable {
	t.Helper()
	for _, v := range report.Variables {
		if v.Name == name {
			return v
		}
	}
	t.Fatalf("variable %s missing: %+v", name, report.Variables)
	return EnvVariable{}
}

func valueOf(v EnvVariable) string {
	if v.Value == nil {
		return "<nil>"
	}
	return *v.Value
}

// TestEnvContractProfile: TSA-V0-210..212 — the named profile's bootstrap is read; each variable
// carries its derivation and value, exported and runner-prefix variables are reliable, and the
// other profile's script and the unread compose file stay out of the variables.
func TestEnvContractProfile(t *testing.T) {
	report := ReadEnvContract(envFixture(), "e2e")
	if report.Schema != EnvContractSchema || len(report.Sources) != 1 || report.Sources[0] != "scripts/bootstrap-e2e.sh" {
		t.Fatalf("sources = %+v", report.Sources)
	}
	if len(report.UnreadSources) != 1 || report.UnreadSources[0].Reason != envComposeNotRead {
		t.Fatalf("unread = %+v", report.UnreadSources)
	}
	cases := []struct {
		name, kind, value  string
		reliable, endpoint bool
	}{
		{"BASE_PORT", DerivationLiteral, "4100", false, true},
		{"APP_URL", DerivationInterpolated, "http://localhost:4100", true, true},
		{"PAYMENTS_STUB_PORT", DerivationOffset, "4110", false, true},
		{"PAYMENTS_URL", DerivationInterpolated, "http://localhost:4110", true, true},
		{"API_TOKEN", DerivationLiteral, "test-token", true, false},
	}
	for _, c := range cases {
		v := envVariable(t, report, c.name)
		if v.Derivation.Kind != c.kind || valueOf(v) != c.value || v.Reliable != c.reliable || v.Endpoint != c.endpoint {
			t.Fatalf("%s = %+v value %s", c.name, v, valueOf(v))
		}
	}
	if v := envVariable(t, report, "PAYMENTS_STUB_PORT"); v.Derivation.Offset == nil || *v.Derivation.Offset != 10 || v.Derivation.References[0] != "BASE_PORT" {
		t.Fatalf("offset = %+v", v.Derivation)
	}
	if missing := ReadEnvContract(envFixture(), "staging"); missing.Reason != envProfileNotFound || len(missing.Variables) != 0 {
		t.Fatalf("missing profile = %+v", missing)
	}
}

// TestEnvContractUnknown: TSA-V0-216 — a command substitution, an unassigned reference and a
// variable assigned by several profile scripts resolve to UNKNOWN with a reason, never a guess.
func TestEnvContractUnknown(t *testing.T) {
	report := ReadEnvContract(envFixture(), "e2e")
	sha := envVariable(t, report, "BUILD_SHA")
	if sha.Value != nil || sha.Derivation.Kind != DerivationUnknown || sha.Derivation.Reason != envCommandSubstitution || sha.UnknownReason != envCommandSubstitution {
		t.Fatalf("BUILD_SHA = %+v", sha)
	}
	region := envVariable(t, report, "REGION")
	if region.Value != nil || region.Derivation.Kind != DerivationVariable || region.UnknownReason != envNotAssigned+":DEPLOY_REGION" {
		t.Fatalf("REGION = %+v", region)
	}
	files := envFixture()
	files["env/e2e.sh"] = "export APP_URL=http://localhost:9999\n"
	twice := envVariable(t, ReadEnvContract(files, "e2e"), "APP_URL")
	if twice.Value != nil || twice.Derivation.Kind != DerivationUnknown || twice.UnknownReason != envSeveralScripts {
		t.Fatalf("several = %+v", twice)
	}
}

// TestEnvContractStandIn: TSA-V0-213 — from Corvint alone an author finds the stand-in's URL, the
// variable tests should read for it, and its fault-injection route with payload, scope and teardown.
func TestEnvContractStandIn(t *testing.T) {
	report := ReadEnvContract(envFixture(), "e2e")
	if len(report.StandIns) != 1 {
		t.Fatalf("stand-ins = %+v", report.StandIns)
	}
	s := report.StandIns[0]
	if s.Source != "stubs/payments/server.js" || s.URL == nil || *s.URL != "http://localhost:4110" || s.PortVariable != "PAYMENTS_STUB_PORT" ||
		len(s.DeclaredVariables) != 1 || s.DeclaredVariables[0] != "PAYMENTS_URL" || s.LaunchedAt != "scripts/bootstrap-e2e.sh:10" {
		t.Fatalf("stand-in = %+v", s)
	}
	kinds := map[string]StandInRoute{}
	for _, r := range s.Routes {
		kinds[r.Method+" "+r.Path] = r
	}
	fault := kinds["POST /__control/faults"]
	if fault.Kind != RouteFaultInjection || strings.Join(fault.Payload, ",") != "delayMs,status,testId" || fault.Scope != "KEYED:testId" ||
		len(fault.Teardown) != 1 || fault.Teardown[0] != "POST /__control/reset" || fault.Line != 5 {
		t.Fatalf("fault = %+v", fault)
	}
	if r := kinds["GET /__control/requests"]; r.Kind != RouteRequestLog || strings.Join(r.Query, ",") != "testId" {
		t.Fatalf("requests = %+v", r)
	}
	if r := kinds["POST /__control/reset"]; r.Kind != RouteReset || r.Scope != ScopeServiceWide {
		t.Fatalf("reset = %+v", r)
	}
	if r := kinds["POST /charges"]; r.Kind != RouteService || r.Scope != "" {
		t.Fatalf("service = %+v", r)
	}
}

// TestEnvContractWarnings: TSA-V0-214 — a hard-coded offset and a hard-coded literal address are
// warned with the computed value and the declared alternative; reading the declared variable is not.
func TestEnvContractWarnings(t *testing.T) {
	report := ReadEnvContract(envFixture(), "e2e")
	if len(report.Warnings) != 2 {
		t.Fatalf("warnings = %+v", report.Warnings)
	}
	offset, literal := report.Warnings[0], report.Warnings[1]
	if offset.Rule != RuleEnvHardCodedOffset || offset.Line != 3 || offset.ComputedValue == nil || *offset.ComputedValue != "4110" ||
		strings.Join(offset.DeclaredAlternatives, ",") != "PAYMENTS_URL" {
		t.Fatalf("offset = %+v", offset)
	}
	if literal.Rule != RuleEnvHardCodedAddress || literal.Line != 5 || *literal.ComputedValue != "4110" || strings.Join(literal.DeclaredAlternatives, ",") != "PAYMENTS_URL" {
		t.Fatalf("literal = %+v", literal)
	}
	declared := ReadEnvContract(map[string]string{"scripts/bootstrap-e2e.sh": envBootstrap, "tests/a.spec.ts": "const u = process.env.PAYMENTS_URL;\n"}, "e2e")
	if len(declared.Warnings) != 0 || len(declared.Dependencies) != 1 || declared.Dependencies[0].Kind != DependencyDeclared {
		t.Fatalf("declared = %+v", declared)
	}
}

// TestEnvContractImpact: TSA-V0-215 — a bootstrap port-offset change lists the dependent tests:
// the declared read stays valid, the hard-coded offset and literal no longer match.
func TestEnvContractImpact(t *testing.T) {
	root := t.TempDir()
	files := envFixture()
	base := commitFiles(t, root, files)
	files["scripts/bootstrap-e2e.sh"] = strings.Replace(envBootstrap, "BASE_PORT + 10", "BASE_PORT + 20", 1)
	head := commitFiles(t, root, files)
	report, err := EnvContract(OpenRepo(context.Background(), root), EnvContractOptions{Profile: "e2e", Revision: "HEAD", Base: base})
	if err != nil || report.Revision != head || report.Base != base {
		t.Fatalf("report = %+v err = %v", report, err)
	}
	changed := map[string]EnvChange{}
	for _, c := range report.Changes {
		changed[c.Variable] = c
	}
	if len(changed) != 2 || changed["PAYMENTS_STUB_PORT"].After == nil || *changed["PAYMENTS_STUB_PORT"].After != "4120" {
		t.Fatalf("changes = %+v", report.Changes)
	}
	got := map[string]bool{}
	for _, d := range changed["PAYMENTS_URL"].Dependents {
		if d.StaysValid == nil {
			t.Fatalf("unknown dependent = %+v", d)
		}
		got[fmt.Sprintf("%s:%d", d.Kind, d.Line)] = *d.StaysValid
	}
	want := map[string]bool{DependencyDeclared + ":2": true, DependencyOffset + ":3": false, DependencyLiteral + ":5": false}
	if len(got) != len(want) {
		t.Fatalf("dependents = %+v", got)
	}
	for k, v := range want {
		if got[k] != v {
			t.Fatalf("dependents = %+v", got)
		}
	}
	// With no change there is no impact.
	if same := EnvChanges(files, files, "e2e"); len(same) != 0 {
		t.Fatalf("same = %+v", same)
	}
}
