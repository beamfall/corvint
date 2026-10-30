package doccorpus

import (
	"os"
	"regexp"
	"strconv"
	"strings"
	"testing"
)

// DCX-V1-564 (V1-2545): the release notes name the merge of every derivation version from 1 to the
// running DerivationVersion, with the rebuild command, so a merge that raises the derivation fails
// here until its entry is written.
func TestDCXV1564ReleaseNotesListEveryDerivation(t *testing.T) {
	t.Parallel()
	data, err := os.ReadFile("../../docs/RELEASE-NOTES.md")
	if err != nil {
		t.Fatal(err)
	}
	notes := string(data)
	start := strings.Index(notes, "### Corpus derivation versions")
	if start < 0 {
		t.Fatal("docs/RELEASE-NOTES.md has no Corpus derivation versions section")
	}
	section := notes[start+3:]
	if end := strings.Index(section, "\n## "); end >= 0 {
		section = section[:end]
	}
	if end := strings.Index(section, "\n### "); end >= 0 {
		section = section[:end]
	}
	if !strings.Contains(section, "corvint docs corpus build --artifact ") {
		t.Fatal("the derivation section does not give the rebuild command")
	}
	current, err := strconv.Atoi(DerivationVersion)
	if err != nil {
		t.Fatal(err)
	}
	for n := 1; n <= current+1; n++ {
		listed := regexp.MustCompile(`(?m)^- Derivation ` + strconv.Itoa(n) + `: #[0-9]+ `).MatchString(section)
		if listed != (n <= current) {
			t.Fatalf("derivation %d listed=%v with a merge number; the running derivation is %s", n, listed, DerivationVersion)
		}
	}
}
