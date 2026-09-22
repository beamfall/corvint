package main

import (
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"
)

// normalizeProse collapses whitespace runs (including line wraps) to single spaces, so a
// substring check does not depend on exactly where hand-wrapped markdown prose breaks lines.
func normalizeProse(s string) string {
	return strings.Join(strings.Fields(s), " ")
}

func readReleaseNotesProse(t *testing.T, relative string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("..", "..", relative))
	if err != nil {
		t.Fatal(err)
	}
	return normalizeProse(string(data))
}

// TestReleaseNotesExcludeRustAndWindowsFromReleaseSet binds PUB-V0-007's second sentence:
// "Rust and unqualified Windows artifacts MUST NOT enter the release set." Nothing previously
// checked that the exclusion statements in the release notes survive an edit; this would have
// let a rewrite silently drop the Windows/Rust exclusion while every other gate still passed.
func TestReleaseNotesExcludeRustAndWindowsFromReleaseSet(t *testing.T) {
	notes := readReleaseNotesProse(t, "docs/RELEASE-NOTES.md")
	alpha := readReleaseNotesProse(t, "docs/RELEASE-NOTES-alpha.md")

	for _, want := range []string{
		"No Rust runtime is included.",
		"Other workflow-bundle platforms and Windows CLI shipment remain outside this release set.",
	} {
		if !strings.Contains(notes, normalizeProse(want)) {
			t.Fatalf("docs/RELEASE-NOTES.md no longer states %q", want)
		}
	}
	for _, want := range []string{
		"The release set is the `darwin_amd64`, `darwin_arm64`, `linux_amd64` and `linux_arm64` archives",
		"that file also names `corvint_windows_amd64.zip`, which is deliberately not attached.",
		"**Windows artifacts are not qualified and do not enter this release set.**",
		"**No Rust artifact exists or ships.**",
	} {
		if !strings.Contains(alpha, normalizeProse(want)) {
			t.Fatalf("docs/RELEASE-NOTES-alpha.md no longer states %q", want)
		}
	}
}

// TestReleaseNotesPlatformAndBrowserClaimsNameTestedVersionsOrDeferQualification binds
// PUB-V0-004 and PUB-V0-007: concrete platform scope stays visible, while actual tested
// versions and installed-path status come from the exact release's qualification evidence.
func TestReleaseNotesPlatformAndBrowserClaimsNameTestedVersionsOrDeferQualification(t *testing.T) {
	alpha := readReleaseNotesProse(t, "docs/RELEASE-NOTES-alpha.md")

	for _, want := range []string{
		"**Native `corvint` smoke qualification runs only on the build host's own `GOOS`/`GOARCH`**",
		"**The optional workflow bundle** (`corvint-companion-bundle/2`) contains nine Go tools",
		"It targets **macOS arm64 only**",
		"Availability, artifact identity and tested platform status come only from versioned release assets and their attached qualification evidence.",
		"The exact release evidence records installed-path browser/UI qualification for the console (PUB-V0-004/005).",
	} {
		if !strings.Contains(alpha, normalizeProse(want)) {
			t.Fatalf("docs/RELEASE-NOTES-alpha.md no longer states %q", want)
		}
	}
}

// TestReleaseNotesPublicationReasonMatchesReleaseChecklist requires owner publication evidence
// in both the checklist and enduring notes, without hard-coding one run's publication status.
func TestReleaseNotesPublicationReasonMatchesReleaseChecklist(t *testing.T) {
	alpha := readReleaseNotesProse(t, "docs/RELEASE-NOTES-alpha.md")
	checklist := readReleaseNotesProse(t, "script/release-checklist")

	for _, want := range []string{"publication-status", "no repository-owner publication receipt is present"} {
		if !strings.Contains(checklist, want) {
			t.Fatalf("script/release-checklist no longer contains %q", want)
		}
	}
	if !strings.Contains(alpha, "Publication status is established only by the versioned release and its owner publication receipt, not by this tracked document.") {
		t.Fatal("docs/RELEASE-NOTES-alpha.md no longer requires release-specific owner publication evidence")
	}
	if strings.Contains(alpha, "no receipt reader") {
		t.Fatal("docs/RELEASE-NOTES-alpha.md still claims the publication receipt reader is absent")
	}
}

// TestReleaseNotesCEMTrustCitationLandsOnTrustRoots binds PUB-V0-007's "default CEM trust in the
// local Git executable/object database" disclosure to its source. The release notes sit outside
// the line-citation gate's document set, and their citation had drifted onto the verifier budget
// bullet; the cited range must still carry the roots-of-trust statement.
func TestReleaseNotesCEMTrustCitationLandsOnTrustRoots(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("..", "..", "docs", "RELEASE-NOTES-alpha.md"))
	if err != nil {
		t.Fatal(err)
	}
	match := regexp.MustCompile("`docs/CHANGE-EVIDENCE-MAP.md:([0-9]+)-([0-9]+)(@[0-9a-f]+)?`").FindSubmatch(data)
	if match == nil {
		t.Fatal("docs/RELEASE-NOTES-alpha.md no longer cites the CEM trust boundary by line range")
	}
	first, _ := strconv.Atoi(string(match[1]))
	last, _ := strconv.Atoi(string(match[2]))
	cem, err := os.ReadFile(filepath.Join("..", "..", "docs", "CHANGE-EVIDENCE-MAP.md"))
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(string(cem), "\n")
	if first < 1 || last < first || last > len(lines) {
		t.Fatalf("cited range %d-%d is outside docs/CHANGE-EVIDENCE-MAP.md", first, last)
	}
	cited := normalizeProse(strings.Join(lines[first-1:last], "\n"))
	for _, want := range []string{"local Git executable, object database", "roots of trust"} {
		if !strings.Contains(cited, want) {
			t.Fatalf("docs/CHANGE-EVIDENCE-MAP.md:%d-%d does not state %q", first, last, want)
		}
	}
}
