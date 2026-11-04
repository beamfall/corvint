// SPDX-License-Identifier: Apache-2.0

package cli_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/tasks/fixture"
	"github.com/Beamfall/corvint/internal/tasks/intent"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// TEA-V0-7100: a malformed evidence claim names the retention route before
// attempting a write, and both mutation helps describe the digest preimage.
func TestTEAV07100_EvidenceRetentionGuidance(t *testing.T) {
	r, local, revision := papercutRepo(t)
	for _, verb := range []string{"complete-manual", "attach-evidence"} {
		t.Run(verb, func(t *testing.T) {
			help := atm(t, r.Root, nil, "ticket", verb, "--help")
			for _, want := range []string{"exact file bytes", "receipt put FILE", "receipt path SHA256", "witness-receipts"} {
				if !strings.Contains(string(help.stdout), want) {
					t.Errorf("help lacks %q: %s", want, help.stdout)
				}
			}
			x := atm(t, r.Root, nil, "ticket", verb, "--target", local, "--expected-revision", revision, "--request-id", "bad-evidence-"+verb, "--payload", `{"evidence":["text"],"reason":"retained proof"}`)
			if !hasCode(x.res, wire.CodeMalformed) || !strings.Contains(strings.Join(x.res.Warnings, " "), "Retain exact file bytes with corvint-tasks receipt put FILE, then pass the returned sha256 in the evidence array; corvint-tasks receipt path SHA256 verifies the local retained artifact.") {
				t.Fatalf("missing retention remedy: %s", x.stdout)
			}
		})
	}
}

// TCL-V0-7100: receipt put is a file-argument reader too. It must resolve
// relative paths against Env.Cwd without changing the process directory.
func TestTCLV07100_ReceiptPutResolvesEnvCwd(t *testing.T) {
	r, _, _ := papercutRepo(t)
	raw := []byte("retained evidence\n")
	if err := os.WriteFile(filepath.Join(r.Root, "evidence.txt"), raw, 0600); err != nil {
		t.Fatal(err)
	}
	x := atm(t, r.Root, nil, "receipt", "put", "evidence.txt")
	if x.code != 0 || len(x.res.Items) != 1 || field(x.res.Items[0], "sha256").Str != string(wire.Sum(raw)) {
		t.Fatalf("put relative to Env.Cwd: %s", x.stdout)
	}
}

// TCL-V0-7101: a long gate argument keeps the wire bound, naming the
// offending argv element and its byte limit rather than reporting a spawn failure.
func TestTCLV07101_GateArgumentNamesLimit(t *testing.T) {
	p := fixture.PolicyValue()
	p.Obj.Set("gates", wire.Array(commandGate("verify", "/bin/sh", "-c", strings.Repeat("x", 129))))
	_, err := intent.DecodePolicy(wire.EncodeFile(p))
	if err == nil || wire.CodeOf(err) != wire.CodeLimitExceeded || !strings.Contains(err.Error(), "/gates/0/argv/2") || !strings.Contains(err.Error(), "128 bytes") {
		t.Fatalf("limit not named: %v", err)
	}
}
