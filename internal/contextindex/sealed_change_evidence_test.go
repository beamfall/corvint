package contextindex

import (
	"context"
	"strings"
	"testing"
)

// DIRTY-CACHE-019: only a `<name>.cem.json` file directly under
// `.corvint/changes/` is a sealed CEM; the sidecar and any other layout are not.
func TestSealedChangeEvidencePathLayout(t *testing.T) {
	for value, want := range map[string]bool{
		".corvint/changes/0123456789abcdef0123456789abcdef01234567.cem.json": true,
		".corvint/changes/batch.cem.json":                                    true,
		".corvint/changes/.cem.json":                                         false,
		".corvint/changes/notes.json":                                        false,
		".corvint/changes/nested/x.cem.json":                                 false,
		".corvint/change.cem.json":                                           false,
		"docs/.corvint/changes/x.cem.json":                                   false,
		"x/.corvint/changes/x.cem.json":                                      false,
	} {
		if got := sealedChangeEvidencePath(value); got != want {
			t.Errorf("sealedChangeEvidencePath(%q) = %v, want %v", value, got, want)
		}
		wantReason := ""
		if want {
			wantReason = sealedChangeEvidenceReason
		}
		if got := ForbiddenPathReason(value); got != wantReason {
			t.Errorf("ForbiddenPathReason(%q) = %q, want %q", value, got, wantReason)
		}
	}
}

// DIRTY-CACHE-019: a committed sealed CEM is excluded from index sources with
// its named reason, so it adds no admitted bytes, never ranks as evidence,
// samples behind a real gap, and an impact request naming it is answered
// OUT_OF_SCOPE rather than as confident evidence.
func TestBuildExcludesSealedChangeEvidence(t *testing.T) {
	root := t.TempDir()
	testGit(t, root, "init", "-q")
	testGit(t, root, "config", "user.email", "corvint@example.test")
	testGit(t, root, "config", "user.name", "Corvint Test")
	sealed := ".corvint/changes/0123456789abcdef0123456789abcdef01234567.cem.json"
	evidence := `{"note":"RevokeDeviceSession revoke expired device session"}` + "\n"
	files := map[string]string{
		"go.mod":                             "module example.test/sealed\n\ngo 1.27.0\n",
		"internal/auth/revoke.go":            "package auth\n\n// RevokeDeviceSession revokes an expired device session.\nfunc RevokeDeviceSession() bool { return true }\n",
		"zz/huge.go":                         "package zz\n\n// " + strings.Repeat("x", 2<<20) + "\n",
		sealed:                               evidence,
		".corvint/changes/notes.json":        evidence,
		".corvint/changes/nested/x.cem.json": evidence,
		".corvint/change.cem.json":           evidence,
	}
	for path, content := range files {
		writeTestFile(t, root, path, content)
	}
	testGit(t, root, "add", ".")
	testGit(t, root, "commit", "-qm", "seal change evidence")
	index, err := Build(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := index.Sources[sealed]; ok {
		t.Fatalf("sealed CEM admitted as index source")
	}
	excluded := false
	for _, exclusion := range index.Exclusions {
		excluded = excluded || (exclusion.Path == sealed && exclusion.Reason == sealedChangeEvidenceReason)
	}
	if !excluded {
		t.Fatalf("sealed CEM not excluded with reason %q: %v", sealedChangeEvidenceReason, index.Exclusions)
	}
	for _, kept := range []string{".corvint/changes/notes.json", ".corvint/changes/nested/x.cem.json", ".corvint/change.cem.json"} {
		if _, ok := index.Sources[kept]; !ok {
			t.Fatalf("unrecognised layout %s hidden from the index", kept)
		}
	}

	packet, err := TaskContext(context.Background(), index, "Fix RevokeDeviceSession for an expired device session", "", 20)
	if err != nil {
		t.Fatal(err)
	}
	for _, result := range mapsFromAny(packet["results"]) {
		if result["id"] == sealed {
			t.Fatalf("sealed CEM ranked as evidence: %v", packet["results"])
		}
	}
	samples := mapsFromAny(packet["exclusions"].(map[string]any)["samples"])
	if len(samples) != 2 || samples[0]["path"] != "zz/huge.go" || samples[1]["path"] != sealed || samples[1]["reason"] != sealedChangeEvidenceReason {
		t.Fatalf("TCP-V0-121: the real gap must sample ahead of the sealed CEM: %v", samples)
	}

	result, err := Impact(index, []string{sealed}, 10)
	if err != nil {
		t.Fatal(err)
	}
	if result["state"] != "OUT_OF_SCOPE" {
		t.Fatalf("impact on a sealed CEM = %v, want OUT_OF_SCOPE", result["state"])
	}
}
