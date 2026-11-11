package companioncli

import (
	"bytes"
	"encoding/json"
	"testing"

	"github.com/Beamfall/corvint/internal/versionline"
)

// CAP-V0-700: Answer handles exactly the five contract forms; any other
// argument list is left to the binary. The built binaries are checked end to
// end by the binaries' own help tests.
func TestCAPV0700AnswerHandlesOnlyTheContractForms(t *testing.T) {
	spec := Spec{Binary: "corvint-x", Version: ExperimentalVersion, Usage: "usage: corvint-x\nExample: corvint-x run\n"}
	for _, arguments := range [][]string{{"--help"}, {"-h"}, {"help"}, {"help", "--json"}, {"--version"}, {"--version", "--json"}} {
		var stdout bytes.Buffer
		if code, handled := Answer(spec, arguments, &stdout); !handled || code != 0 || stdout.Len() == 0 {
			t.Fatalf("%q: handled=%v code=%d", arguments, handled, code)
		}
	}
	var help bytes.Buffer
	Answer(spec, []string{"help", "--json"}, &help)
	var decoded struct {
		Profile, Binary string
		Usage           []string
	}
	if json.Unmarshal(help.Bytes(), &decoded) != nil || decoded.Profile != HelpProfile || len(decoded.Usage) != 2 {
		t.Fatalf("help --json %s", help.Bytes())
	}
	for _, arguments := range [][]string{nil, {"run"}, {"--help", "extra"}, {"help", "run"}, {"--version", "--yaml"}} {
		var stdout bytes.Buffer
		if _, handled := Answer(spec, arguments, &stdout); handled || stdout.Len() != 0 {
			t.Fatalf("%q handled", arguments)
		}
	}
}

// ARTIFACT-BIN-V0-508 / CMS-V0-002: an unstamped companion keeps the plain
// `BINARY VERSION` line; a binary a release builder stamped with main.build and
// main.sourceCommit reports the release version, the build and the commit on
// the plain line and in --version --json, so an installed set can be checked
// for one version.
func TestARTIFACTBINV0508StampedCompanionReportsBuildAndCommit(t *testing.T) {
	commit := "0123456789abcdef0123456789abcdef01234567"
	unstamped := Spec{Binary: "corvint-x", Version: ExperimentalVersion}
	if got := unstamped.Text(); got != "corvint-x "+ExperimentalVersion || unstamped.Stamped() {
		t.Fatalf("unstamped line %q stamped=%v", got, unstamped.Stamped())
	}
	stamped := Spec{Binary: "corvint-x", Version: ExperimentalVersion, Build: "183", Commit: commit}
	want := "corvint-x " + versionline.Release + " (build 183, commit " + commit + ")"
	if got := stamped.Text(); got != want || !stamped.Stamped() {
		t.Fatalf("stamped line %q want %q", got, want)
	}
	line, ok := versionline.Parse(stamped.Text())
	if !ok || line != (versionline.Line{Name: "corvint-x", Version: versionline.Release, Build: "183", Commit: commit}) {
		t.Fatalf("stamped line does not parse back: %#v %v", line, ok)
	}
	// Build alone is not a release: the version stays the development form.
	buildOnly := Spec{Binary: "corvint-x", Version: ExperimentalVersion, Build: "183"}
	if got := buildOnly.Text(); got != "corvint-x "+ExperimentalVersion+" (build 183, commit unknown)" || buildOnly.Stamped() {
		t.Fatalf("build-only line %q", got)
	}
	var stdout bytes.Buffer
	Answer(stamped, []string{"--version", "--json"}, &stdout)
	var decoded struct {
		Profile, Binary, Version, Build, Commit, CommitState string
		Release                                              bool
	}
	if json.Unmarshal(stdout.Bytes(), &decoded) != nil || decoded.Profile != VersionProfile || decoded.Version != versionline.Release || decoded.Build != "183" || decoded.Commit != commit || decoded.CommitState != "clean" || !decoded.Release {
		t.Fatalf("stamped --version --json %s", stdout.Bytes())
	}
	stdout.Reset()
	Answer(unstamped, []string{"--version", "--json"}, &stdout)
	var plain map[string]any
	if json.Unmarshal(stdout.Bytes(), &plain) != nil || plain["version"] != ExperimentalVersion || plain["release"] != false {
		t.Fatalf("unstamped --version --json %s", stdout.Bytes())
	}
	if _, present := plain["build"]; present {
		t.Fatalf("unstamped --version --json reports a build: %s", stdout.Bytes())
	}
}
