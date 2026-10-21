package main

import (
	"bytes"
	"encoding/json"
	"reflect"
	"runtime/debug"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/contextindex"
)

func buildInfoWith(settings ...string) *debug.BuildInfo {
	info := &debug.BuildInfo{}
	for index := 0; index+1 < len(settings); index += 2 {
		info.Settings = append(info.Settings, debug.BuildSetting{Key: settings[index], Value: settings[index+1]})
	}
	return info
}

// TestPUBV0028ProducerDevBuildMarkers covers a source dev build: an
// unstamped build and absent VCS settings yield explicit markers, never a
// guessed value.
func TestPUBV0028ProducerDevBuildMarkers(t *testing.T) {
	for name, info := range map[string]*debug.BuildInfo{
		"no build info":       nil,
		"no vcs settings":     buildInfoWith("-trimpath", "true"),
		"non-git vcs":         buildInfoWith("vcs", "hg", "vcs.revision", strings.Repeat("a", 40)),
		"malformed revision":  buildInfoWith("vcs", "git", "vcs.revision", "abc"),
		"uppercase revision":  buildInfoWith("vcs", "git", "vcs.revision", strings.Repeat("A", 40)),
		"empty build setting": buildInfoWith(),
	} {
		for _, stamp := range []string{"0", ""} {
			got := producerFrom("1.0.0-rc.3", stamp, info)
			want := producerIdentity{Version: "1.0.0-rc.3", Build: "unstamped", Commit: "unknown", CommitState: "unknown"}
			if got != want {
				t.Fatalf("%s build %q: got %+v, want %+v", name, stamp, got, want)
			}
		}
	}
	revision := strings.Repeat("1", 40)
	for state, want := range map[string]string{"false": "clean", "true": "modified", "": "unknown", "maybe": "unknown"} {
		got := producerFrom("1.0.0-rc.3", "0", buildInfoWith("vcs", "git", "vcs.revision", revision, "vcs.modified", state))
		if got.Commit != revision || got.CommitState != want || got.Build != "unstamped" {
			t.Fatalf("vcs.modified %q: got %+v", state, got)
		}
	}
	sha256 := strings.Repeat("b", 64)
	if got := producerFrom("v", "0", buildInfoWith("vcs", "git", "vcs.revision", sha256)); got.Commit != sha256 {
		t.Fatalf("SHA-256 revision not stated: %+v", got)
	}
}

// TestPUBV0029ProducerReleaseStampedBuild covers a release-stamped build:
// release archives pass -X main.build=N and -buildvcs=false, so the build is
// stated and the commit stays an explicit unknown for the manifest to name.
func TestPUBV0029ProducerReleaseStampedBuild(t *testing.T) {
	release := buildInfoWith("-buildvcs", "false", "-trimpath", "true", "-ldflags", "-X main.build=676")
	got := producerFrom("1.0.0-rc.3", "676", release)
	want := producerIdentity{Version: "1.0.0-rc.3", Build: "676", Commit: "unknown", CommitState: "unknown"}
	if got != want {
		t.Fatalf("got %+v, want %+v", got, want)
	}
	revision := strings.Repeat("c", 40)
	stamped := producerFrom("1.0.0-rc.3", "676", buildInfoWith("vcs", "git", "vcs.revision", revision, "vcs.modified", "false"))
	if stamped != (producerIdentity{Version: "1.0.0-rc.3", Build: "676", Commit: revision, CommitState: "clean"}) {
		t.Fatalf("stamped source build: %+v", stamped)
	}
	if !reflect.DeepEqual(got.members(), map[string]any{"version": "1.0.0-rc.3", "build": "676", "commit": "unknown", "commitState": "unknown"}) {
		t.Fatalf("map form disagrees: %v", got.members())
	}
}

// TestPUBV0027ImpactProducerIsAdditive proves --producer adds exactly one
// member to the impact receipt and leaves every other byte unchanged; and
// (PUB-V0-030) that the default receipt carries no producer member.
func TestPUBV0027ImpactProducerIsAdditive(t *testing.T) {
	root := impactCLIRepository(t)
	invoke := func(arguments ...string) string {
		var stdout, stderr bytes.Buffer
		if code := run(append([]string{"--root", root, "impact"}, arguments...), strings.NewReader(""), &stdout, &stderr); code != 0 {
			t.Fatalf("impact %v exit %d: %s", arguments, code, stderr.String())
		}
		return stdout.String()
	}
	plain := invoke("pkg/main.go")
	with := invoke("--producer", "pkg/main.go")
	member, err := contextindex.CanonicalJSON(currentProducer().members())
	if err != nil {
		t.Fatal(err)
	}
	assertProducerAdditive(t, plain, with, strings.Replace(with, `"producer":`+string(member)+`,`, "", 1))
}

// TestPUBV0027TestValidityProducerIsAdditive covers the test-validity
// receipt: the producer member follows the unchanged document members.
func TestPUBV0027TestValidityProducerIsAdditive(t *testing.T) {
	invoke := func(arguments ...string) string {
		var stdout, stderr bytes.Buffer
		if code := run(append([]string{"test-validity"}, arguments...), strings.NewReader(""), &stdout, &stderr); code != 0 {
			t.Fatalf("test-validity %v exit %d: %s", arguments, code, stderr.String())
		}
		return stdout.String()
	}
	plain := invoke()
	with := invoke("--producer")
	member, err := json.Marshal(currentProducer())
	if err != nil {
		t.Fatal(err)
	}
	assertProducerAdditive(t, plain, with, strings.Replace(with, `,"producer":`+string(member)+"}\n", "}\n", 1))
	for _, arguments := range [][]string{{"--producer", "--producer"}, {"--producer=yes"}} {
		var stdout, stderr bytes.Buffer
		if code := run(append([]string{"test-validity"}, arguments...), strings.NewReader(""), &stdout, &stderr); code != 2 || stdout.Len() != 0 {
			t.Fatalf("test-validity %v: exit %d stdout %q", arguments, code, stdout.String())
		}
	}
}

// TestPUBV0030DefaultReceiptsCarryNoProducer pins the frozen default bytes
// (CCF-V1-006): without --producer neither receipt names a producer.
func TestPUBV0030DefaultReceiptsCarryNoProducer(t *testing.T) {
	root := impactCLIRepository(t)
	for _, arguments := range [][]string{{"--root", root, "impact", "pkg/main.go"}, {"test-validity"}} {
		var stdout, stderr bytes.Buffer
		if code := run(arguments, strings.NewReader(""), &stdout, &stderr); code != 0 {
			t.Fatalf("%v exit %d: %s", arguments, code, stderr.String())
		}
		if strings.Contains(stdout.String(), `"producer"`) {
			t.Fatalf("%v default output names a producer: %s", arguments, stdout.String())
		}
	}
}

func assertProducerAdditive(t *testing.T, plain, with, stripped string) {
	t.Helper()
	var decoded map[string]any
	if err := json.Unmarshal([]byte(with), &decoded); err != nil {
		t.Fatal(err)
	}
	producer, ok := decoded["producer"].(map[string]any)
	if !ok {
		t.Fatalf("no producer member: %s", with)
	}
	want := currentProducer().members()
	if !reflect.DeepEqual(producer, want) {
		t.Fatalf("producer %v, want %v", producer, want)
	}
	for _, key := range []string{"version", "build", "commit", "commitState"} {
		if value, _ := producer[key].(string); value == "" {
			t.Fatalf("producer.%s is empty", key)
		}
	}
	if stripped != plain {
		t.Fatalf("--producer changed more than one member:\nplain %s\nwith  %s", plain, with)
	}
}

// TestGPKV0084ImpactCapabilitiesListing checks the CLI listing envelope and
// reads no repository content beyond the impact root check (GPK-V0-084).
func TestGPKV0084ImpactCapabilitiesListing(t *testing.T) {
	var stdout, stderr bytes.Buffer
	if code := run([]string{"--root", cliRepository(t), "impact", "--capabilities"}, strings.NewReader(""), &stdout, &stderr); code != 0 {
		t.Fatalf("exit %d: %s", code, stderr.String())
	}
	var decoded map[string]any
	if err := json.Unmarshal(stdout.Bytes(), &decoded); err != nil {
		t.Fatal(err)
	}
	if decoded["ok"] != true || decoded["mutates"] != false || decoded["tool"] != "impact-capabilities" || decoded["profile"] != "corvint-impact-capabilities/0" {
		t.Fatalf("envelope: %v", decoded)
	}
	suffixes := decoded["suffixes"].(map[string]any)
	if !containsString(suffixes["withReverseImportRule"], ".go") || !containsString(suffixes["admittedWithoutRule"], ".rb") || containsString(suffixes["admittedWithoutRule"], ".csv") {
		t.Fatalf("suffixes: %v", suffixes)
	}
	if suffixes["unadmitted"] != "unsupported-impact-path-suffix" {
		t.Fatalf("unadmitted code: %v", suffixes["unadmitted"])
	}
	limits := decoded["limits"].(map[string]any)
	if limits["pathsMax"] != float64(100) || limits["limitMax"] != float64(50) || limits["limitDefault"] != float64(defaultImpactLimit) || limits["pathCharsMax"] != float64(1024) {
		t.Fatalf("limits: %v", limits)
	}
}

// TestGPKV0084ImpactCapabilitiesOutsideGitRoot is V1-2067: the listing reads
// no repository, so it prints outside any Git checkout, while path impact
// there is still refused for the missing root.
func TestGPKV0084ImpactCapabilitiesOutsideGitRoot(t *testing.T) {
	t.Chdir(t.TempDir())
	var stdout, stderr bytes.Buffer
	if code := run([]string{"impact", "--capabilities"}, strings.NewReader(""), &stdout, &stderr); code != 0 {
		t.Fatalf("exit %d: %s", code, stderr.String())
	}
	var decoded map[string]any
	if err := json.Unmarshal(stdout.Bytes(), &decoded); err != nil || decoded["tool"] != "impact-capabilities" {
		t.Fatalf("listing: %v %s", err, stdout.String())
	}
	for _, arguments := range [][]string{{"impact", "README.md"}, {"impact", "--capabilities", "--limit", "3"}} {
		stdout.Reset()
		stderr.Reset()
		if code := run(arguments, strings.NewReader(""), &stdout, &stderr); code != 2 || stdout.Len() != 0 || !strings.Contains(stderr.String(), "not a Git repository") {
			t.Fatalf("%v: exit %d stdout %q stderr %s, want the root refusal", arguments, code, stdout.String(), stderr.String())
		}
	}
}

// TestGPKV0086ImpactCapabilitiesArgumentsAndRefusalUnchanged keeps
// --capabilities a sole, valueless argument and the suffix refusal intact.
func TestGPKV0086ImpactCapabilitiesArgumentsAndRefusalUnchanged(t *testing.T) {
	for _, arguments := range [][]string{{"--capabilities", "--limit", "3"}, {"--capabilities=yes"}, {"pkg/main.go", "--capabilities"}, {"--capabilities", "--capabilities"}} {
		var stdout, stderr bytes.Buffer
		if code := run(append([]string{"impact"}, arguments...), strings.NewReader(""), &stdout, &stderr); code != 2 || stdout.Len() != 0 {
			t.Fatalf("impact %v: exit %d stdout %q", arguments, code, stdout.String())
		}
	}
	root := impactCLIRepository(t)
	var stdout, stderr bytes.Buffer
	if code := run([]string{"--root", root, "impact", "--producer", "README.csv"}, strings.NewReader(""), &stdout, &stderr); code != 2 || !strings.Contains(stderr.String(), "unsupported-impact-path-suffix") {
		t.Fatalf("suffix refusal changed: exit %d stderr %s", code, stderr.String())
	}
}

func containsString(list any, want string) bool {
	values, _ := list.([]any)
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}
