package main

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// ignoredSourceFixtureMain answers the loose gate's native smoke contract and
// otherwise prints the marks that Git-ignored init files append.
const ignoredSourceFixtureMain = `package main

import "os"

var marks string

var build = "0"

func main() {
	switch {
	case len(os.Args) > 1 && os.Args[1] == "--version":
		os.Stdout.WriteString("Fixture 1 (build " + build + ")\n")
	case len(os.Args) > 3 && os.Args[3] == "query":
		os.Stdout.WriteString("{\"context\":{\"intent\":{\"id\":\"fixture\"}}}\n")
	default:
		os.Stdout.WriteString("marks=" + marks + "\n")
	}
}
`

// ARTIFACT-GO-V0-003, decision 0158: a .go file hidden by a committed .gitignore
// or by .git/info/exclude leaves Git status empty, so the loose gate builds it
// from the live root and reports PASS with vcs.modified=false. The archive's
// raw-commit builds omit it, and the loose-binary binding refuses the bytes.
func TestIgnoredLiveSourcePassesLooseGateButArchiveBindingRefuses(t *testing.T) {
	t.Setenv("GIT_CONFIG_GLOBAL", os.DevNull)
	t.Setenv("GIT_CONFIG_NOSYSTEM", "1")
	ctx := context.Background()
	root := filepath.Join(t.TempDir(), "repository")
	command := filepath.Join(root, "cmd", "fixture")
	if err := os.MkdirAll(command, 0o700); err != nil {
		t.Fatal(err)
	}
	mustWrite(t, filepath.Join(root, "go.mod"), "module example.com/ignoredsource\n\ngo 1.27.1\n")
	mustWrite(t, filepath.Join(command, "main.go"), ignoredSourceFixtureMain)
	mustWrite(t, filepath.Join(root, ".gitignore"), "committed_hidden.go\n")
	runGitTest(t, root, "init", "-q")
	runGitTest(t, root, "add", "go.mod", "cmd/fixture/main.go", ".gitignore")
	runGitTest(t, root, "-c", "user.name=Corvint Test", "-c", "user.email=corvint@example.invalid", "commit", "-qm", "fixture")
	mustWrite(t, filepath.Join(command, "committed_hidden.go"), "package main\n\nfunc init() { marks += \"C\" }\n")
	mustWrite(t, filepath.Join(root, ".git", "info", "exclude"), "local_hidden.go\n")
	mustWrite(t, filepath.Join(command, "local_hidden.go"), "package main\n\nfunc init() { marks += \"L\" }\n")

	// The pin is the host's selected toolchain so the observation isolates source provenance.
	selected, err := goEnv(ctx, root, "GOVERSION")
	if err != nil {
		t.Fatal(err)
	}
	manifest := Manifest{
		Toolchain: Toolchain{GoVersion: selected, GoModDirective: "1.27.1"},
		Profile: Profile{Package: "./cmd/fixture", ModulePath: "example.com/ignoredsource", BinaryName: "fixture", BuildFlags: []string{"-trimpath"},
			Environment: map[string]string{"CGO_ENABLED": "0", "GOENV": "off", "GOFLAGS": "-mod=readonly", "GOPROXY": "off", "GOSUMDB": "off", "GOTOOLCHAIN": "local"}},
		Smoke: Smoke{VersionArgument: "--version", ExpectedVersion: "Fixture 1", QueryTask: "task", QueryLimit: "1", ExpectedIntent: "fixture", FixtureInstruction: "# Agents\n"},
	}
	target := Target{GOOS: runtime.GOOS, GOARCH: runtime.GOARCH}
	options := Options{Root: root, Output: filepath.Join(t.TempDir(), "loose")}
	loose := Report{}
	if err := checkOutputLocation(options, &loose); err != nil {
		t.Fatal(err)
	}
	if err := preflight(ctx, options, manifest, &loose); err != nil {
		t.Fatal(err)
	}
	looseTarget, err := buildTarget(ctx, options, manifest, target, &loose)
	if err != nil {
		t.Fatal(err)
	}
	if verdict(loose) != statusPass || !looseTarget.ByteIdentical || looseTarget.BuildInfo["vcs.modified"] != "false" || looseTarget.Smoke.Status != statusPass {
		t.Fatalf("loose gate observation changed: verdict=%s reasons=%+v modified=%q smoke=%+v", verdict(loose), loose.Reasons, looseTarget.BuildInfo["vcs.modified"], looseTarget.Smoke)
	}
	if marks := runFixtureMarks(t, looseTarget.Artifact); marks != "marks=CL" {
		t.Fatalf("loose binary did not link both ignored sources: %q", marks)
	}

	scratch := t.TempDir()
	state, err := resolveCleanState(ctx, ArchiveOptions{Root: root, Revision: "HEAD"}, scratch)
	if err != nil || state.commit != loose.Commit || state.tree != loose.Tree {
		t.Fatalf("archive clean state %+v err=%v; loose commit=%s tree=%s", state, err, loose.Commit, loose.Tree)
	}
	sourceA, sourceB := filepath.Join(scratch, "source-a"), filepath.Join(scratch, "source-b")
	for _, source := range []string{sourceA, sourceB} {
		if err := exportRawCommit(ctx, root, scratch, state.commit, source); err != nil {
			t.Fatal(err)
		}
	}
	gitDirectoryRaw, err := closedGit(ctx, root, scratch, "rev-parse", "--absolute-git-dir")
	if err != nil {
		t.Fatal(err)
	}
	verifier := writeVerifierFixture(t, []byte(`{}`), 1)
	archiveTarget, _, _, err := buildArchiveTarget(ctx, ArchiveOptions{Root: root, Revision: "HEAD"}, scratch, sourceA, sourceB,
		strings.TrimSpace(string(gitDirectoryRaw)), verifier, state, nil, manifest, target, nil, looseTarget)
	if err == nil || err.Error() != "loose-gate-report-mismatch" {
		t.Fatalf("archive accepted or misclassified ignored-source divergence: %v", err)
	}
	if marks := runFixtureMarks(t, filepath.Join(scratch, "build-a", target.GOOS+"-"+target.GOARCH, "fixture")); marks != "marks=" {
		t.Fatalf("raw commit build linked ignored live source: %q", marks)
	}
	if archiveTarget.RetainedBinary.SHA256 != "" {
		t.Fatal("refused target recorded a retained binary")
	}
}

func runFixtureMarks(t *testing.T, binary string) string {
	t.Helper()
	output, err := exec.Command(binary).Output()
	if err != nil {
		t.Fatalf("%s: %v", binary, err)
	}
	return strings.TrimSpace(string(output))
}
