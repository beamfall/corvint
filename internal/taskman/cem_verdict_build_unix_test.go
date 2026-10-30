//go:build darwin || linux

package taskman

import (
	"runtime/debug"
	"strings"
	"testing"
)

// TCB-V0-045: a disagreement whose two legs recorded different builds names
// the build mismatch; equal or unrecorded builds keep verifier-disagreement.
func TestTCBV0045DisagreementNamesAVerifierBuildMismatch(t *testing.T) {
	f := newVerdictFixture(t, nil)
	in := f.input(PredicateTouchedAnchored, "src/")
	in.PortableConsumer = patchedConsumer(t, f.portable, `s/"verdictSha256":"[0-9a-f]*"/"verdictSha256":"`+strings.Repeat("0", 64)+`"/`, "")
	restore := verifierBuilds
	t.Cleanup(func() { verifierBuilds = restore })
	for name, c := range map[string]struct {
		native, portable verifierBuild
		reason           string
	}{
		"stale consumer":      {verifierBuild{Revision: "b"}, verifierBuild{Revision: "a"}, "verifier-build-mismatch"},
		"modified consumer":   {verifierBuild{Revision: "a"}, verifierBuild{Revision: "a", Modified: true}, "verifier-build-mismatch"},
		"modified verifier":   {verifierBuild{Revision: "a", Modified: true}, verifierBuild{Revision: "a"}, "verifier-build-mismatch"},
		"same build":          {verifierBuild{Revision: "a"}, verifierBuild{Revision: "a"}, "verifier-disagreement"},
		"unrecorded consumer": {verifierBuild{Revision: "a"}, verifierBuild{}, "verifier-disagreement"},
		"unrecorded verifier": {verifierBuild{}, verifierBuild{Revision: "a"}, "verifier-disagreement"},
	} {
		c := c
		verifierBuilds = func(string) (verifierBuild, verifierBuild) { return c.native, c.portable }
		if view := requireVerdictRefusal(t, in, CEMVerdictVerifierDisagreement, c.reason); view.Portable == nil {
			t.Fatalf("%s: the disagreeing portable pin is missing", name)
		}
	}
	// An agreeing pair never reads the builds: skew alone does not refuse.
	verifierBuilds = func(string) (verifierBuild, verifierBuild) {
		t.Fatal("builds read without a disagreement")
		return verifierBuild{}, verifierBuild{}
	}
	in.PortableConsumer = f.portable
	if result, _ := evaluateVerdict(t, in); !result.Granted {
		t.Fatalf("agreeing legs refused: %s", result.Code)
	}
}

func TestTCBV0045RecordedBuildReadsVCSSettings(t *testing.T) {
	got := recordedBuild(&debug.BuildInfo{Settings: []debug.BuildSetting{{Key: "vcs", Value: "git"}, {Key: "vcs.revision", Value: "ef2a"}, {Key: "vcs.modified", Value: "true"}}})
	if got != (verifierBuild{Revision: "ef2a", Modified: true}) {
		t.Fatalf("%+v", got)
	}
	if got := recordedBuild(&debug.BuildInfo{}); got != (verifierBuild{}) {
		t.Fatalf("%+v", got)
	}
	// The test binary records no VCS settings, and a shell wrapper is no Go
	// executable: both read as unrecorded.
	native, portable := verifierBuilds("/bin/sh")
	if native.Revision != "" || portable.Revision != "" {
		t.Fatalf("native %+v portable %+v", native, portable)
	}
}
