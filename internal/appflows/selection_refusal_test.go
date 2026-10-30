package appflows

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// tieredProvider is a minimal valid provider whose flow_tiers member is the given JSON object text.
func tieredProvider(tiers string) string {
	return `{"schema":"` + SelectionProviderSchema + `","flows":"flows","runner_config":"playwright.config.ts","global_paths":[],` +
		`"inventory":[{"test_key":"auth > recovers","project":"chromium","path":"e2e/admin/auth/password-recovery.spec.ts"}],` +
		`"flow_tiers":` + tiers + `}`
}

// AFU-V1-126 (GH #1888): a flow_tiers member name that is a flow ID is not read as a credential
// name, while secret-shaped values, non-identifier names and every other position stay refused.
func TestAFUV1126FlowTierIdentifierKeyIsNotSecret(t *testing.T) {
	const key = "cov-tests-e2e-admin-auth-password-recovery-spec-ts"
	accepted := tieredProvider(`{"` + key + `":["server"],"token":["client","server"]}`)
	if err := Decode([]byte(accepted), new(SelectionProvider)); err == nil || err.Error() != "flow input contains secret-shaped data" {
		t.Fatalf("generic Decode must keep screening member names: %v", err)
	}
	var p SelectionProvider
	if err := decode([]byte(accepted), &p, maskFlowTierKeys); err != nil {
		t.Fatalf("flow ID key refused: %v", err)
	}
	if got := p.FlowTiers[key]; len(got) != 1 || got[0] != "server" {
		t.Fatalf("flow_tiers decoded as %v", p.FlowTiers)
	}
	for name, data := range map[string]string{
		"secret tier value":       tieredProvider(`{"` + key + `":["ghp_` + strings.Repeat("a", 36) + `"]}`),
		"non-identifier key":      tieredProvider(`{"1-password":["server"]}`),
		"over-long key":           tieredProvider(`{"` + strings.Repeat("a", 64) + `-password":["server"]}`),
		"assignment in key":       tieredProvider(`{"password=hunter2":["server"]}`),
		"secret elsewhere":        strings.Replace(tieredProvider(`{"`+key+`":["server"]}`), `"global_paths":[]`, `"global_paths":["password: hunter2"]`, 1),
		"credential in test key":  strings.Replace(tieredProvider(`{"`+key+`":["server"]}`), `auth > recovers`, `auth > password: hunter2`, 1),
		"nested flow_tiers name":  strings.Replace(tieredProvider(`{}`), `"project":"chromium"`, `"project":"chromium","flow_tiers":{"password-x":["s"]}`, 1),
		"top-level password name": strings.Replace(tieredProvider(`{}`), `"flow_tiers":{}`, `"flow_tiers":{},"password":"hunter2"`, 1),
	} {
		if err := decode([]byte(data), new(SelectionProvider), maskFlowTierKeys); err == nil || err.Error() != "flow input contains secret-shaped data" {
			t.Errorf("%s: decode = %v, want the secret-shaped refusal", name, err)
		}
	}
}

// AFU-V1-127 (GH #1888): a provider-unavailable selection names the refused document and a closed
// refusal class, never the refused text.
func TestAFUV1127ProviderUnavailableNamesRefusal(t *testing.T) {
	const secret = "hunter2-do-not-echo"
	valid := tieredProvider(`{"checkout":["server"]}`)
	withCoverage := strings.Replace(valid, `"flow_tiers"`, `"coverage":"coverage.json","flow_tiers"`, 1)
	for name, tc := range map[string]struct {
		files    map[string]string
		provider string
		want     string
	}{
		"absent provider":       {nil, "e2e-provider.json", "provider:unreadable"},
		"absolute provider":     {nil, "/abs/e2e-provider.json", "provider:invalid-path"},
		"oversized provider":    {map[string]string{"e2e-provider.json": strings.Repeat(" ", MaxBytes+1)}, "e2e-provider.json", "provider:byte-limit"},
		"secret in provider":    {map[string]string{"e2e-provider.json": strings.Replace(valid, `"global_paths":[]`, `"global_paths":["password: `+secret+`"]`, 1)}, "e2e-provider.json", "provider:secret-shaped"},
		"provider JSON":         {map[string]string{"e2e-provider.json": `{"schema":`}, "e2e-provider.json", "provider:invalid-json"},
		"provider unknown":      {map[string]string{"e2e-provider.json": strings.Replace(valid, `"flows"`, `"extra":1,"flows"`, 1)}, "e2e-provider.json", "provider:invalid-schema"},
		"provider content":      {map[string]string{"e2e-provider.json": strings.Replace(valid, SelectionProviderSchema, "foreign/9", 1)}, "e2e-provider.json", "provider:invalid-content"},
		"absent coverage":       {map[string]string{"e2e-provider.json": withCoverage}, "e2e-provider.json", "coverage:unreadable"},
		"secret in coverage":    {map[string]string{"e2e-provider.json": withCoverage, "coverage.json": `{"schema":"` + CoverageSchema + `","records":[],"note":"token=` + secret + `"}`}, "e2e-provider.json", "coverage:secret-shaped"},
		"coverage schema":       {map[string]string{"e2e-provider.json": withCoverage, "coverage.json": `{"schema":"` + CoverageSchema + `","records":[],"extra":1}`}, "e2e-provider.json", "coverage:invalid-schema"},
		"coverage content":      {map[string]string{"e2e-provider.json": withCoverage, "coverage.json": `{"schema":"foreign/9","records":[]}`}, "e2e-provider.json", "coverage:invalid-content"},
		"flow ID with password": {map[string]string{"e2e-provider.json": tieredProvider(`{"cov-password-recovery-spec-ts":["server"]}`)}, "e2e-provider.json", ""},
	} {
		root := t.TempDir()
		for file, body := range tc.files {
			if err := os.WriteFile(filepath.Join(root, file), []byte(body), 0o600); err != nil {
				t.Fatal(err)
			}
		}
		_, _, err := readSelectionProvider(root, tc.provider)
		if tc.want == "" {
			if err != nil {
				t.Errorf("%s: refused: %v", name, err)
			}
			continue
		}
		if err == nil {
			t.Errorf("%s: accepted, want %s", name, tc.want)
			continue
		}
		if got := selectionRefusalSubject(err); got != tc.want || strings.Contains(got, secret) || strings.Contains(err.Error(), secret) {
			t.Errorf("%s: subject %q (%v), want %q", name, got, err, tc.want)
		}
	}
	out := SelectE2E(t.Context(), t.TempDir(), E2EInput{Provider: "absent.json"})
	if out.State != "blocked" || out.StateReason != "provider-unavailable" || out.Refusal == nil ||
		*out.Refusal != (E2EFallback{Code: "provider-unavailable", Subject: "provider:unreadable"}) || len(out.Fallback) != 0 {
		t.Fatalf("blocked selection %+v refusal %+v", out, out.Refusal)
	}
}
