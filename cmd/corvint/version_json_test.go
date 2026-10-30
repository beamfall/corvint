package main

import (
	"bytes"
	"encoding/json"
	"runtime/debug"
	"slices"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/gokernel"
	"github.com/Beamfall/corvint/internal/versionline"
)

// TestPRSV1020VersionJSONNamesVerbsAndHosts (V1-0300): `corvint --version --json`
// emits the corvint-version/1 identity a plugin
// reads to detect binary skew: version, build, every dispatched verb and every
// admitted harness host. Plain --version keeps its text line, and any other
// argument after --json stays an argument error.
func TestPRSV1020VersionJSONNamesVerbsAndHosts(t *testing.T) {
	for _, args := range [][]string{{"--version", "--json"}} {
		var stdout, stderr bytes.Buffer
		if code := run(args, strings.NewReader(""), &stdout, &stderr); code != 0 {
			t.Fatalf("%v: exit %d, stderr %s", args, code, stderr.String())
		}
		var got struct {
			Profile, Tool, Version, Build string
			Commands, HarnessHosts        []string
		}
		if err := json.Unmarshal(stdout.Bytes(), &got); err != nil {
			t.Fatalf("%v: %v in %s", args, err, stdout.String())
		}
		// PUB-V0-051: a test binary is unstamped, so build is omitted rather than "0".
		if got.Profile != "corvint-version/1" || got.Tool != "corvint" || got.Version != version || got.Build != "" || strings.Contains(stdout.String(), `"build"`) {
			t.Fatalf("%v: identity %+v", args, got)
		}
		if !slices.IsSorted(got.Commands) || len(got.Commands) != len(topLevelCommands) {
			t.Fatalf("%v: commands %v", args, got.Commands)
		}
		for _, verb := range topLevelCommands {
			if !slices.Contains(got.Commands, verb) {
				t.Fatalf("%v: commands lack %q", args, verb)
			}
		}
		if !slices.Equal(got.HarnessHosts, gokernel.HarnessHosts()) || len(got.HarnessHosts) == 0 {
			t.Fatalf("%v: harnessHosts %v", args, got.HarnessHosts)
		}
	}
	var stdout, stderr bytes.Buffer
	info, _ := debug.ReadBuildInfo()
	if code := run([]string{"--version"}, strings.NewReader(""), &stdout, &stderr); code != 0 || stdout.String() != versionText(versionIdentityFrom(version, build, sourceCommit, info))+"\n" {
		t.Fatalf("plain --version: exit %d, %q", code, stdout.String())
	}
	stdout.Reset()
	if code := run([]string{"--version", "--json", "extra"}, strings.NewReader(""), &stdout, &stderr); code != 2 || stdout.Len() != 0 {
		t.Fatalf("--version --json extra: exit %d, stdout %q", code, stdout.String())
	}
}

// TestPUBV0050VersionJSONStatesSourceCommit (V1-2839): --version --json
// states the source commit and its dirty state from the binary's own build
// information, with explicit unknown markers when it carries none.
func TestPUBV0050VersionJSONStatesSourceCommit(t *testing.T) {
	var stdout, stderr bytes.Buffer
	if code := run([]string{"--version", "--json"}, strings.NewReader(""), &stdout, &stderr); code != 0 {
		t.Fatalf("exit %d, stderr %s", code, stderr.String())
	}
	var got map[string]any
	if err := json.Unmarshal(stdout.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	info, _ := debug.ReadBuildInfo()
	want := versionIdentityFrom(version, build, sourceCommit, info)
	if got["commit"] != want.Commit || got["commitState"] != want.CommitState || got["release"] != want.Release || got["displayVersion"] != want.DisplayVersion {
		t.Fatalf("payload %v, want %+v", got, want)
	}
	revision := strings.Repeat("d", 40)
	for name, test := range map[string]struct {
		info          *debug.BuildInfo
		commit, state string
	}{
		"clean checkout":    {buildInfoWith("vcs", "git", "vcs.revision", revision, "vcs.modified", "false"), revision, "clean"},
		"modified checkout": {buildInfoWith("vcs", "git", "vcs.revision", revision, "vcs.modified", "true"), revision, "modified"},
		"no vcs":            {buildInfoWith("-buildvcs", "false"), "unknown", "unknown"},
		"no build info":     {nil, "unknown", "unknown"},
	} {
		identity := versionIdentityFrom("1.0.0-rc.3", "774", "", test.info)
		if identity.Commit != test.commit || identity.CommitState != test.state || identity.Build != "774" {
			t.Fatalf("%s: %+v", name, identity)
		}
	}
}

// TestPUBV0051UnstampedBuildOmitsBuild (V1-2839): an unstamped binary never
// reports a constant build 0 in --version --json; the member is omitted.
func TestPUBV0051UnstampedBuildOmitsBuild(t *testing.T) {
	for _, stamp := range []string{"0", ""} {
		if identity := versionIdentityFrom("1.0.0-rc.3", stamp, "", nil); identity.Build != producerUnstamped {
			t.Fatalf("build %q: %+v", stamp, identity)
		}
	}
	if identity := versionIdentityFrom("1.0.0-rc.3", "774", "", nil); identity.Build != "774" {
		t.Fatalf("stamped build lost: %+v", identity)
	}
}

// TestPUBV0052UnreleasedBuildIsPreRelease (V1-2839): only a binary a release
// builder stamped with both its build number and source commit reports the
// compiled VERSION; every other build reports a pre-release of it.
func TestPUBV0052UnreleasedBuildIsPreRelease(t *testing.T) {
	commit := strings.Repeat("e", 40)
	for name, test := range map[string]struct {
		version, build, stamp, display string
		release                        bool
	}{
		"make build":                 {"1.0.0-rc.3", "774", "", "1.0.0-rc.3.dev", false},
		"go build":                   {"1.0.0-rc.3", "0", "", "1.0.0-rc.3.dev", false},
		"final version dev":          {"1.0.0", "800", "", "1.0.0-dev", false},
		"alpha version dev":          {"0.5.0a3", "9", "", "0.5.0a3-dev", false},
		"release archive":            {"1.0.0-rc.3", "774", commit, "1.0.0-rc.3", true},
		"commit stamp without build": {"1.0.0-rc.3", "0", commit, "1.0.0-rc.3.dev", false},
		"malformed commit stamp":     {"1.0.0-rc.3", "774", "abc", "1.0.0-rc.3.dev", false},
	} {
		identity := versionIdentityFrom(test.version, test.build, test.stamp, nil)
		if identity.DisplayVersion != test.display || identity.Release != test.release {
			t.Fatalf("%s: %+v", name, identity)
		}
		if test.release && (identity.Commit != commit || identity.CommitState != "clean") {
			t.Fatalf("%s: release commit not stated: %+v", name, identity)
		}
	}
}

// TestPUBV0053PlainVersionLineFormat (V1-2839, #1902): the plain --version line
// is `Corvint DISPLAY (build N, commit SHA[, modified])`, drops the build member
// when unstamped, names `commit unknown` without a commit, and parses back.
func TestPUBV0053PlainVersionLineFormat(t *testing.T) {
	commit := strings.Repeat("a", 40)
	for name, test := range map[string]struct {
		build, stamp string
		info         *debug.BuildInfo
		want         string
	}{
		"make build, dirty": {"776", "", buildInfoWith("vcs", "git", "vcs.revision", commit, "vcs.modified", "true"), "Corvint 1.0.0-rc.3.dev (build 776, commit " + commit + ", modified)"},
		"make build, clean": {"776", "", buildInfoWith("vcs", "git", "vcs.revision", commit, "vcs.modified", "false"), "Corvint 1.0.0-rc.3.dev (build 776, commit " + commit + ")"},
		"go build":          {"0", "", buildInfoWith("vcs", "git", "vcs.revision", commit, "vcs.modified", "false"), "Corvint 1.0.0-rc.3.dev (commit " + commit + ")"},
		"no vcs, unstamped": {"0", "", nil, "Corvint 1.0.0-rc.3.dev (commit unknown)"},
		"release archive":   {"780", commit, buildInfoWith("-buildvcs", "false"), "Corvint 1.0.0-rc.3 (build 780, commit " + commit + ")"},
		"stamped, no vcs":   {"780", "", buildInfoWith("-buildvcs", "false"), "Corvint 1.0.0-rc.3.dev (build 780, commit unknown)"},
	} {
		got := versionText(versionIdentityFrom("1.0.0-rc.3", test.build, test.stamp, test.info))
		if got != test.want {
			t.Fatalf("%s: %q, want %q", name, got, test.want)
		}
		line, ok := versionline.Parse(got)
		if !ok || line.Name != "Corvint" || !versionline.SameRelease(line.Version, "1.0.0-rc.3") {
			t.Fatalf("%s: %q does not parse back: %+v", name, got, line)
		}
	}
	var stdout, stderr bytes.Buffer
	if code := run([]string{"--version"}, strings.NewReader(""), &stdout, &stderr); code != 0 || strings.Count(stdout.String(), "\n") != 1 {
		t.Fatalf("plain --version is not one line: %q", stdout.String())
	}
}
