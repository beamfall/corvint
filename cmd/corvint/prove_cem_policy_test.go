package main

import (
	"strings"
	"testing"
)

// TestCEMV1638ProveAttestCEMV2PolicyCeilings checks CEM-V1-638: prove
// --attest-cem-v2 admits --max-unknown and --max-mechanical. A ceiling the
// accepted map meets leaves the attestation byte-identical; an exceeded one
// refuses attest-cem-v2-policy-exceeded with an empty stdout, so nothing is
// attested; and a ceiling without --attest-cem-v2 or with a malformed value is
// an argument error.
func TestCEMV1638ProveAttestCEMV2PolicyCeilings(t *testing.T) {
	t.Parallel()
	f := materializeCEMV2Case(t)
	_, plain := proveStableCEM(t, f)
	if _, within := proveStableCEM(t, f, "--max-unknown", "0", "--max-mechanical", "0"); string(within) != string(plain) {
		t.Fatalf("met ceilings changed the attestation:\n%s\n%s", within, plain)
	}

	root, base := proveCEMRepository(t)
	if code, out, stderr := runCLI(t, "--root", root, "cem", "mark", "--map", ".corvint/change.cem.json", "--hunk", "1",
		"--disposition", "unknown", "--reason", "insufficient-evidence"); code != 0 {
		t.Fatalf("mark: %s %s", out, stderr)
	}
	cemGit(t, root, "add", ".corvint/change.cem.json")
	cemGit(t, root, "commit", "-qm", "unknown")
	head := cemGit(t, root, "rev-parse", "HEAD")
	common := []string{"--root", root, "prove", "--cem", ".corvint/change.cem.json", "--expected-base", base,
		"--target", head, "--artifacts", root, "--attest-cem-v2"}
	if stdout, stderr, code := runProveArguments(t, common...); code != 0 || len(stdout) == 0 {
		t.Fatalf("no ceiling: exit %d stderr %s", code, stderr)
	}
	stdout, stderr, code := runProveArguments(t, append(common, "--max-unknown", "0")...)
	if code != 2 || len(stdout) != 0 || !strings.Contains(stderr, `"attest-cem-v2-policy-exceeded"`) || !strings.Contains(stderr, "unknown 1 > 0") {
		t.Fatalf("exceeded ceiling: exit %d stdout %q stderr %s", code, stdout, stderr)
	}
	for _, arguments := range [][]string{
		{"--root", root, "prove", "--cem", ".corvint/change.cem.json", "--max-unknown", "0"},
		append(append([]string{}, common...), "--max-mechanical", "x"),
		append(append([]string{}, common...), "--max-unknown", "0", "--max-unknown", "1"),
	} {
		stdout, stderr, code := runProveArguments(t, arguments...)
		if code != 2 || len(stdout) != 0 || !strings.Contains(stderr, `"invalid-arguments"`) {
			t.Fatalf("%v: exit %d stdout %q stderr %s", arguments, code, stdout, stderr)
		}
	}
}
