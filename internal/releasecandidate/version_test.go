package releasecandidate

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestPUBV0023CandidateVersionGrammar(t *testing.T) {
	t.Run("PUB-V0-023 candidate version grammar admits aN, -rc.N and stable only", testPUBV0023CandidateVersionGrammar)
}

func testPUBV0023CandidateVersionGrammar(t *testing.T) {
	cases := []struct {
		version  string
		admitted bool
	}{
		{"0.5.0a1", true},
		{"0.5.0a2", true},
		{"0.5.0a3", true},
		{"0.5.0a10", true},
		{"1.0.0-rc.1", true},
		{"1.0.0-rc.2", true},
		{"1.0.0-rc.10", true},
		{"1.0.0", true},
		{"0.0.0", true},
		{"10.20.30", true},
		{"", false},
		{"1.0.0-beta.1", false},
		{"1.0.0-rc1", false},
		{"1.0.0rc1", false},
		{"1.0.0-rc.0", false},
		{"1.0.0-rc.01", false},
		{"1.0.0-rc.", false},
		{"1.0.0-RC.1", false},
		{"01.0.0", false},
		{"1.00.0", false},
		{"1.0.01", false},
		{"v1.0.0", false},
		{"1.0.0+build", false},
		{"1.0.0-rc.1+build", false},
		{"1.0.0a0", true},
		{"0.4.0a0", true},
		{"1.0.0a01", false},
		{"1.0.0b1", false},
		{"1.0", false},
		{"1.0.0.0", false},
		{"1.0.0\n", false},
		{" 1.0.0", false},
		{"1.0.0-rc.1a1", false},
	}
	for _, tc := range cases {
		if got := versionPattern.MatchString(tc.version); got != tc.admitted {
			t.Errorf("version %q admitted=%v, want %v", tc.version, got, tc.admitted)
		}
	}
}

func TestPUBV0023ReleaseCandidateRC1AssemblesVerifiesAndInstalls(t *testing.T) {
	t.Run("PUB-V0-023 1.0.0-rc.1 Core-only candidate assembles, verifies and installs", testPUBV0023ReleaseCandidateRC1AssemblesVerifiesAndInstalls)
}

func testPUBV0023ReleaseCandidateRC1AssemblesVerifiesAndInstalls(t *testing.T) {
	if runtime.GOOS != "darwin" && runtime.GOOS != "linux" {
		t.Skip("host probe fixture requires a POSIX shell")
	}
	const version = "1.0.0-rc.1"
	source := canonicalTemp(t)
	gitFixture := func(arguments ...string) string {
		command := exec.Command("git", append([]string{"-C", source}, arguments...)...)
		command.Env = append(os.Environ(), "GIT_CONFIG_GLOBAL="+os.DevNull, "GIT_CONFIG_NOSYSTEM=1", "GIT_AUTHOR_NAME=fixture", "GIT_AUTHOR_EMAIL=fixture@example.invalid", "GIT_COMMITTER_NAME=fixture", "GIT_COMMITTER_EMAIL=fixture@example.invalid")
		out, err := command.CombinedOutput()
		if err != nil {
			t.Fatalf("git %v: %v %s", arguments, err, out)
		}
		return strings.TrimSpace(string(out))
	}
	gitFixture("init", "-q")
	if err := os.WriteFile(filepath.Join(source, "VERSION"), []byte(version+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	gitFixture("add", "VERSION")
	gitFixture("commit", "-q", "-m", "fixture")
	commit, tree := gitFixture("rev-parse", "HEAD"), gitFixture("rev-parse", "HEAD^{tree}")

	versionOutput := "Corvint " + version + " (build 1)"
	hostProgram := "#!/bin/sh\nprintf '%s\\n' '" + versionOutput + "'\n"
	isHost := func(target coreTarget) bool { return target.GOOS == runtime.GOOS && target.GOARCH == runtime.GOARCH }
	coreBinary := func(target coreTarget) []byte {
		if isHost(target) {
			return []byte(hostProgram)
		}
		return []byte("binary-" + target.GOOS + "-" + target.GOARCH)
	}
	previousCore := verifyCoreBinary
	verifyCoreBinary = func(_ []byte, target coreTarget) ([]byte, error) { return coreBinary(target), nil }
	t.Cleanup(func() { verifyCoreBinary = previousCore })

	output := filepath.Join(canonicalTemp(t), "candidates")
	result, err := Assemble(t.Context(), Options{CoreDirectory: coreGateFixture(t, commit, tree, coreBinary), SourceRoot: source, Scratch: canonicalTemp(t), OutputParent: output, Version: version})
	if err != nil {
		t.Fatalf("%s Core-only assembly refused: %v", version, err)
	}
	if result.Directory != filepath.Join(output, "corvint-v"+version+"-core") || result.Manifest.Version != version || result.Manifest.CorvintVersion != versionOutput {
		t.Fatalf("unexpected %s result: %s %#v", version, result.Directory, result.Manifest)
	}
	if _, err := VerifyContext(t.Context(), result.Directory); err != nil {
		t.Fatalf("retained %s candidate refused: %v", version, err)
	}

	hostArchive := coreScriptArchive(t, runtime.GOOS+"_"+runtime.GOARCH, hostProgram)
	coreArchive := func(target coreTarget) []byte {
		if isHost(target) {
			return hostArchive
		}
		return []byte("archive-" + target.ArchiveName)
	}
	installable := coreOnlyCandidateFixture(t, versionOutput, commit, tree, coreBinary, coreArchive, func(files map[string][]byte, _ map[string]string, manifest *Manifest, _ *Qualification) {
		manifest.Version, manifest.BuildNumber = version, "1"
		files["README.md"] = []byte(candidateReadme(version))
	})
	installed, err := InstallCore(t.Context(), installable, filepath.Join(canonicalTemp(t), "store"))
	if err != nil || !strings.HasSuffix(installed, filepath.Join("corvint", version, runtime.GOOS+"-"+runtime.GOARCH)) {
		t.Fatalf("%s candidate not installed: %q %v", version, installed, err)
	}
}
