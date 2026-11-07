package main

import (
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"
)

// TestAffectedAuditTempUnavailableNamesTMPDIR_AFPV0920 forces the scratch
// directory creation to fail and checks the refusal names the temporary
// directory and suggests TMPDIR (AFP-V0-920, V1-3912). Not parallel: it sets
// TMPDIR for the process.
func TestAffectedAuditTempUnavailableNamesTMPDIR_AFPV0920(t *testing.T) {
	root, base, head, outcomes := auditFixtureChange(t)
	manifest := writeAuditManifest(t, outcomes, `{"id":"temp","base":"`+base+`","head":"`+head+`"}`)
	missing := filepath.Join(t.TempDir(), "absent", "tmp")
	t.Setenv("TMPDIR", missing)
	_, code, stderr := runAuditCLI(t, root, manifest)
	if code != 2 {
		t.Fatalf("exit %d, want 2: %s", code, stderr)
	}
	var refusal struct {
		Code  string `json:"code"`
		Error string `json:"error"`
	}
	if err := json.Unmarshal([]byte(strings.TrimSpace(stderr)), &refusal); err != nil {
		t.Fatalf("stderr is not one JSON refusal: %v\n%s", err, stderr)
	}
	if refusal.Code != "temp-unavailable" || !strings.Contains(refusal.Error, missing) || !strings.Contains(refusal.Error, "set TMPDIR") {
		t.Fatalf("refusal = %+v", refusal)
	}
}
