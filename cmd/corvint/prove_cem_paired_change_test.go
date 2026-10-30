package main

import (
	"encoding/json"
	"strings"
	"testing"

	cemcli "github.com/Beamfall/corvint/internal/cem/cli"
)

// TestCEMV1648ProveCEMRefusesAPairedChange: `prove --cem` and
// `prove --verify-cem-attestation ... --cem` refuse the paired-change flags
// with the same CEM-V1-648 reason `cem prepare` gives, in the spaced and the =
// form, never with the bare unrecognized-arguments line; a flag that only
// shares the prefix stays unrecognized.
func TestCEMV1648ProveCEMRefusesAPairedChange(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	for _, flag := range []string{"--range", "--range-root", "--paired-from"} {
		want, ok := cemcli.PairedChangeRefusal([]string{flag})
		if !ok {
			t.Fatalf("%s is not a paired-change flag", flag)
		}
		for _, form := range [][]string{{flag, "app=a..b"}, {flag + "=app=a..b"}} {
			for _, arguments := range [][]string{
				append([]string{"--root", root, "cem", "prepare"}, form...),
				append([]string{"--root", root, "prove", "--cem", "m.json"}, form...),
				append(append([]string{"--root", root, "prove", "--cem", "m.json", "--attest-cem-v2"}, form...), "--target", "HEAD"),
				append([]string{"--root", root, "prove", "--verify-cem-attestation", "e.json", "--attest-public-key", "k.pem", "--cem", "m.json"}, form...),
			} {
				stdout, stderr, code := runProveArguments(t, arguments...)
				if code != 2 || len(stdout) != 0 || !strings.Contains(stderr, "invalid-arguments") || !strings.Contains(stderr, want) || strings.Contains(stderr, "unrecognized arguments") {
					t.Fatalf("%v: exit=%d stdout=%q stderr=%q, want the CEM-V1-648 refusal %q", arguments, code, stdout, stderr, want)
				}
			}
		}
	}
	stdout, stderr, code := runProveArguments(t, "--root", root, "prove", "--cem", "m.json", "--ranges", "x")
	if code != 2 || len(stdout) != 0 || !strings.Contains(stderr, "unrecognized arguments: --ranges") {
		t.Fatalf("--ranges: exit=%d stdout=%q stderr=%q", code, stdout, stderr)
	}
}

// TestCEMV1648MapReadingSurfacesRefuseAPairedChange: the closed parsers of
// `cem verify-stable`, `cem bundle` and `cem gate-verdict` refuse each
// paired-change flag, spaced or in = form, with the CEM-V1-648 reason, each in
// its own envelope (V1-2943).
func TestCEMV1648MapReadingSurfacesRefuseAPairedChange(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	for _, flag := range []string{"--range", "--range-root", "--paired-from"} {
		want, _ := cemcli.PairedChangeRefusal([]string{flag})
		for _, form := range [][]string{{flag, "app=a..b"}, {flag + "=app=a..b"}} {
			arguments := append([]string{"cem", "verify-stable", "--repository", root, "--map", root + "/m.json"}, form...)
			stdout, stderr, code := runProveArguments(t, arguments...)
			if code != 2 || !strings.Contains(string(stdout), `"invalid-arguments"`) || stderr != "corvint cem verify-stable: "+want+"\n" {
				t.Fatalf("%v: exit=%d stdout=%q stderr=%q", arguments, code, stdout, stderr)
			}
			arguments = append([]string{"cem", "bundle"}, form...)
			stdout, stderr, code = runProveArguments(t, arguments...)
			var refusal struct{ Code, Error string }
			if code != 2 || len(stdout) != 0 || json.Unmarshal([]byte(stderr), &refusal) != nil || refusal.Code != "invalid-arguments" || refusal.Error != want {
				t.Fatalf("%v: exit=%d stdout=%q stderr=%q", arguments, code, stdout, stderr)
			}
			arguments = append([]string{"--root", root, "cem", "gate-verdict", "--base", "a"}, form...)
			stdout, stderr, code = runProveArguments(t, arguments...)
			if code != 2 || len(stdout) != 0 || stderr != "corvint: cem gate-verdict: "+cemGateVerdictInvalidArguments+": "+want+"\n" {
				t.Fatalf("%v: exit=%d stdout=%q stderr=%q", arguments, code, stdout, stderr)
			}
		}
	}
}
