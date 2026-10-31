package main

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/gokernel"
)

// ASD-V0-072 (V1-3230): `affected --base` in a shallow clone refuses with shallow-repository and
// the `git fetch --unshallow` recovery, before an abbreviated base is resolved; the same checkout
// with its full history plans as before.
func TestASDV0072AffectedRefusesAShallowClone(t *testing.T) {
	root := materializationFixture(t)
	full := strings.TrimSpace(materializationGit(t, root, "rev-parse", "HEAD"))
	if _, _, err := parseAffectedInvocation([]string{"--root", root, "affected", "--base", full}); err != nil {
		t.Fatalf("full history: %v", err)
	}
	gitDir := strings.TrimSpace(materializationGit(t, root, "rev-parse", "--absolute-git-dir"))
	if err := os.WriteFile(filepath.Join(gitDir, "shallow"), []byte(full+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	_, handled, err := parseAffectedInvocation([]string{"--root", root, "affected", "--base", full})
	var coded *gokernel.Error
	if !handled || !errors.As(err, &coded) || coded.Code != affectedShallowCode || !strings.Contains(coded.Message, "git fetch --unshallow") {
		t.Fatalf("shallow clone: handled=%v err=%v", handled, err)
	}
	// An abbreviated base the shallow history does not hold still gets the shallow recovery.
	_, _, err = parseAffectedInvocation([]string{"--root", root, "affected", "--base", "0000000"})
	if !errors.As(err, &coded) || coded.Code != affectedShallowCode {
		t.Fatalf("shallow clone, unavailable abbreviated base: err=%v", err)
	}
	if _, _, err := parseAffectedInvocation([]string{"--root", root, "affected"}); err != nil {
		t.Fatalf("a shallow clone without --base was refused: %v", err)
	}
}
