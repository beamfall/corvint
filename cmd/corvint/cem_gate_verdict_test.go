package main

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/cem/stablefixture"
	"github.com/Beamfall/corvint/internal/cem/wire"
	"github.com/Beamfall/corvint/internal/cem/workflow"
	"github.com/Beamfall/corvint/internal/execfile"
	"github.com/Beamfall/corvint/internal/taskman"
)

// TCB-V0-016: the gate-verdict argv is closed; an unreadable one prints one
// stderr code and no document and exits 2, and a bare word before cem leaves
// the arguments to the command that owns them.
func TestTCBV0016GateVerdictArgvIsClosed(t *testing.T) {
	oid := strings.Repeat("a", 40)
	flags := []string{"--base", oid, "--candidate", oid, "--predicate", "touched-anchored", "--touch-paths", `["src/"]`, "--portable-consumer", "/bin/false"}
	with := func(lead []string, tail ...string) []string {
		return append(append(append([]string{}, lead...), "cem", "gate-verdict"), tail...)
	}
	replaced := func(key, value string) []string {
		out := append([]string{}, flags...)
		for i := 0; i < len(out); i += 2 {
			if out[i] == key {
				out[i+1] = value
			}
		}
		return out
	}
	for name, args := range map[string][]string{
		"no flags":           with(nil),
		"missing flag":       with(nil, flags[:8]...),
		"unknown flag":       with(nil, append(append([]string{}, flags...), "--map", "x")...),
		"repeated flag":      with(nil, append(append([]string{}, flags...), "--base", oid)...),
		"missing value":      with(nil, flags[:9]...),
		"option-like value":  with(nil, replaced("--predicate", "--all")...),
		"relative root":      with([]string{"--root", "repo"}, flags...),
		"unclean root":       with([]string{"--root", "/tmp/../repo"}, flags...),
		"other global":       with([]string{"--json"}, flags...),
		"touch not json":     with(nil, replaced("--touch-paths", "src/")...),
		"touch not strings":  with(nil, replaced("--touch-paths", `["src/",1]`)...),
		"touch not an array": with(nil, replaced("--touch-paths", `{"src/":true}`)...),
	} {
		var stdout, stderr strings.Builder
		code, handled := runCEMGateVerdictInvocation(context.Background(), args, &stdout, &stderr)
		if !handled || code != 2 || stdout.Len() != 0 || stderr.String() != "corvint: cem gate-verdict: "+cemGateVerdictInvalidArguments+"\n" {
			t.Fatalf("%s: handled=%v exit %d stdout %q stderr %q", name, handled, code, stdout.String(), stderr.String())
		}
	}
	for _, args := range [][]string{{"cem", "verify-stable"}, append([]string{"ask", "cem", "gate-verdict"}, flags...)} {
		if _, handled := runCEMGateVerdictInvocation(context.Background(), args, &strings.Builder{}, &strings.Builder{}); handled {
			t.Fatalf("%v must stay with its own command", args)
		}
	}
}

// TCB-V0-016 and TCB-V0-017: over a dogfood-shaped base, change, bind and seal,
// the command prints exactly the library's gate document with the native and
// the real interop/cem01-go verdicts pinned, exits 0 on GRANT, and exits 1 with
// verifier-disagreement when the portable result is patched.
func TestTCBV0016GateVerdictRunsBothConsumers(t *testing.T) {
	stablefixture.RequireStableHost(t)
	// interop/cem01-go qualifies its process containment on darwin and on
	// linux amd64/arm64 (CEM-V1-180); there the real consumer must answer
	// verify-stable-canonical with the native verdictSha256, never UNSUPPORTED.
	if runtime.GOOS == "linux" && runtime.GOARCH != "amd64" && runtime.GOARCH != "arm64" {
		t.Skip("NOT_RUN: interop/cem01-go process containment is not qualified on linux/" + runtime.GOARCH)
	}
	consumer := buildPortableConsumer(t)
	root, base, seal := sealedVerdictFixture(t)
	in := taskman.CEMVerdictInput{Root: root, BaseCommit: base, CandidateCommit: seal, TouchPaths: []string{"src/"}, Predicate: taskman.PredicateTouchedAnchored, PortableConsumer: consumer}
	library, err := taskman.EvaluateCEMVerdictGate(context.Background(), in)
	if err != nil {
		t.Fatal(err)
	}
	args := func(portable string) []string {
		return []string{"cem", "gate-verdict", "--base", base, "--candidate", seal, "--predicate", "touched-anchored", "--touch-paths", `["src/"]`, "--portable-consumer", portable}
	}
	// The Tasks runner starts the gate in the attempt worktree with no --root.
	t.Chdir(root)
	code, stdout, stderr := runCLI(t, args(consumer)...)
	if code != 0 || stderr != "" || stdout != string(library.Document) || !library.Granted {
		t.Fatalf("exit %d stderr %q\ncommand %s\nlibrary %s\n%s", code, stderr, stdout, library.Document, bareHostGitDiagnostic(root))
	}
	var doc struct {
		Outcome string `json:"outcome"`
		Verdict struct {
			Outcome, VerdictSha256 string
		} `json:"verdict"`
		Portable struct {
			Route, Outcome, Stage, VerdictSha256 string
			Code, CitationSha256                 *string
		} `json:"portable"`
		Paths []struct {
			Path, Tier string
			Satisfied  bool
		} `json:"paths"`
	}
	if err := json.Unmarshal([]byte(stdout), &doc); err != nil {
		t.Fatal(err)
	}
	t.Logf("%s/%s native verdictSha256 %s portable %s %s", runtime.GOOS, runtime.GOARCH, doc.Verdict.VerdictSha256, doc.Portable.Outcome, doc.Portable.VerdictSha256)
	if doc.Outcome != "GRANT" || doc.Portable.Route != "verify-stable-canonical" || doc.Portable.Outcome != "ACCEPT" || doc.Portable.Stage != "complete" || doc.Portable.Code != nil || doc.Portable.VerdictSha256 != doc.Verdict.VerdictSha256 || doc.Portable.CitationSha256 == nil {
		t.Fatalf("pins: %+v", doc)
	}
	if len(doc.Paths) != 1 || doc.Paths[0].Path != "src/app.txt" || doc.Paths[0].Tier != "anchored" || !doc.Paths[0].Satisfied {
		t.Fatalf("paths: %+v", doc.Paths)
	}
	patched := filepath.Join(t.TempDir(), "patched-cem01-go")
	script := "#!/bin/sh\n'" + consumer + "' \"$@\" | sed -e 's/\"tier\":\"anchored\"/\"tier\":\"cited\"/'\n"
	if err := execfile.WriteFile(patched, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	code, stdout, stderr = runCLI(t, append([]string{"--root", root}, args(patched)...)...)
	if code != 1 || stderr != "" || !strings.Contains(stdout, `"code":"`+taskman.CEMVerdictVerifierDisagreement+`"`) || !strings.Contains(stdout, `"reason":"verifier-disagreement"`) || !strings.Contains(stdout, `"outcome":"REFUSE"`) {
		t.Fatalf("patched consumer: exit %d stderr %q\n%s", code, stderr, stdout)
	}
	// The real consumer whose PATH resolves a git that rejects --attr-source
	// (the hosted macos-15 condition) answers UNSUPPORTED with its own code,
	// and the command refuses under the route's name with that code pinned,
	// never a grant and never a disagreement (TCB-V0-040, TCB-V0-041).
	unsupported := attrSourceRejectingConsumer(t, consumer)
	code, stdout, stderr = runCLI(t, append([]string{"--root", root}, args(unsupported)...)...)
	if code != 1 || stderr != "" || !strings.Contains(stdout, `"code":"`+taskman.CEMVerdictUnverifiable+`"`) || !strings.Contains(stdout, `"reason":"portable-route-unsupported"`) || !strings.Contains(stdout, `"outcome":"REFUSE"`) || !strings.Contains(stdout, `"citationSha256":null,"code":"git-diff-failed","outcome":"UNSUPPORTED","route":"verify-stable-canonical","stage":"repository","verdictSha256":"`) || strings.Contains(stdout, `"stage":"repository","verdictSha256":"`+doc.Verdict.VerdictSha256+`"`) || !strings.Contains(stdout, `"citation":null`) || !strings.Contains(stdout, `"paths":[]`) {
		t.Fatalf("unsupported consumer: exit %d stderr %q\n%s", code, stderr, stdout)
	}
}

// TCB-V0-042: under the fixed PATH the consumer resolved git from before
// TCB-V0-043 (/usr/bin:/bin), the real consumer's answer is the one that git
// explains. A git there that accepts the stable diff's --attr-source option
// (Git 2.41 or later) verifies the map and the gate grants; one that rejects
// it, as Apple Git 2.39 on the hosted macos-15 image does, ends the consumer
// at `repository` / `git-diff-failed`, and the gate refuses under the route's
// name with that code pinned. Either way the document names the host fact.
func TestTCBV0042PortableConsumerUnderBareHostPath(t *testing.T) {
	stablefixture.RequireStableHost(t)
	if runtime.GOOS == "linux" && runtime.GOARCH != "amd64" && runtime.GOARCH != "arm64" {
		t.Skip("NOT_RUN: interop/cem01-go process containment is not qualified on linux/" + runtime.GOARCH)
	}
	consumer := buildPortableConsumer(t)
	root, base, seal := sealedVerdictFixture(t)
	bare := filepath.Join(t.TempDir(), "bare-path-cem01-go")
	if err := execfile.WriteFile(bare, []byte("#!/bin/sh\nPATH=/usr/bin:/bin\nexport PATH\nexec '"+consumer+"' \"$@\"\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	in := taskman.CEMVerdictInput{Root: root, BaseCommit: base, CandidateCommit: seal, TouchPaths: []string{"src/"}, Predicate: taskman.PredicateTouchedAnchored, PortableConsumer: bare}
	result, err := taskman.EvaluateCEMVerdictGate(context.Background(), in)
	if err != nil {
		t.Fatal(err)
	}
	diagnostic := bareHostGitDiagnostic(root)
	t.Logf("%s/%s %s\n%s", runtime.GOOS, runtime.GOARCH, diagnostic, result.Document)
	var doc struct {
		Outcome  string  `json:"outcome"`
		Code     *string `json:"code"`
		Reason   *string `json:"reason"`
		Portable *struct {
			Outcome, Stage string
			Code           *string
		} `json:"portable"`
	}
	if err := json.Unmarshal(result.Document, &doc); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(diagnostic, "attr-source: accepted") {
		if !result.Granted || doc.Outcome != "GRANT" || doc.Portable == nil || doc.Portable.Outcome != "ACCEPT" {
			t.Fatalf("a bare-PATH git that accepts --attr-source must verify the map:\n%s\n%s", diagnostic, result.Document)
		}
		return
	}
	if result.Granted || doc.Code == nil || *doc.Code != taskman.CEMVerdictUnverifiable || doc.Reason == nil || *doc.Reason != "portable-route-unsupported" || doc.Portable == nil || doc.Portable.Outcome != "UNSUPPORTED" || doc.Portable.Stage != "repository" || doc.Portable.Code == nil || *doc.Portable.Code != "git-diff-failed" {
		t.Fatalf("a bare-PATH git that rejects --attr-source must refuse under the route's name with the consumer's code:\n%s\n%s", diagnostic, result.Document)
	}
}

// bareHostGitDiagnostic reports which git the fixed PATH /usr/bin:/bin
// resolves in root, its version and whether it accepts the stable diff's
// --attr-source option, so a hosted failure retains the host fact behind the
// consumer's own code. The output is bounded and is test output only.
func bareHostGitDiagnostic(root string) string {
	command := exec.Command("/bin/sh", "-c", `command -v git; git --version; if out=$(git --attr-source=HEAD diff --no-color HEAD HEAD 2>&1 >/dev/null); then echo "attr-source: accepted"; else echo "attr-source: rejected exit $?"; printf '%s\n' "$out" | head -c 512; fi`)
	command.Dir = root
	command.Env = []string{"PATH=/usr/bin:/bin", "LC_ALL=C", "LANG=C", "HOME=" + root}
	raw, err := command.CombinedOutput()
	if len(raw) > 2048 {
		raw = raw[:2048]
	}
	status := "exit 0"
	if err != nil {
		status = err.Error()
	}
	return "bare PATH=/usr/bin:/bin git diagnostic (" + status + "):\n" + string(raw)
}

// buildPortableConsumer builds the independent interop/cem01-go module.
// attrSourceRejectingConsumer wraps consumer so that the first git on its
// PATH is a shim that rejects --attr-source as Git before 2.41 does and
// otherwise runs the gate's own git.
func attrSourceRejectingConsumer(t *testing.T, consumer string) string {
	t.Helper()
	real, err := exec.LookPath("git")
	if err != nil || !filepath.IsAbs(real) {
		t.Fatalf("git: %q %v", real, err)
	}
	shim := filepath.Join(t.TempDir(), "old-git")
	if err := os.MkdirAll(shim, 0o755); err != nil {
		t.Fatal(err)
	}
	git := "#!/bin/sh\nfor a in \"$@\"; do case \"$a\" in --attr-source=*) printf '%s\\n' \"error: unknown option \\`attr-source'\" >&2; exit 129;; esac; done\nexec '" + real + "' \"$@\"\n"
	if err := execfile.WriteFile(filepath.Join(shim, "git"), []byte(git), 0o755); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "old-git-consumer")
	if err := execfile.WriteFile(path, []byte("#!/bin/sh\nPATH='"+shim+":/usr/bin:/bin'\nexport PATH\nexec '"+consumer+"' \"$@\"\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	return path
}

func buildPortableConsumer(t *testing.T) string {
	t.Helper()
	module, err := filepath.Abs(filepath.Join("..", "..", "interop", "cem01-go"))
	if err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(t.TempDir(), "cem01-go")
	command := exec.Command("go", "build", "-o", out, ".")
	command.Dir = module
	command.Env = append(os.Environ(), "GOTOOLCHAIN=local", "GOWORK=off", "GOFLAGS=")
	if raw, err := command.CombinedOutput(); err != nil {
		t.Fatalf("build interop/cem01-go: %v\n%s", err, raw)
	}
	resolved, err := filepath.EvalSymlinks(out)
	if err != nil {
		t.Fatal(err)
	}
	return resolved
}

// sealedVerdictFixture commits base, a change to src/app.txt, the map citing
// a base span that names src/app.txt (anchored), and the seal that renames it
// to .corvint/changes/<bind>.cem.json; it returns the root, base and seal.
func sealedVerdictFixture(t *testing.T) (string, string, string) {
	t.Helper()
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	home := t.TempDir()
	git := func(args ...string) string {
		t.Helper()
		command := exec.Command("git", append([]string{"-c", "maintenance.auto=false", "-c", "gc.auto=0"}, args...)...)
		command.Dir = root
		command.Env = append(os.Environ(), "GIT_CONFIG_NOSYSTEM=1", "HOME="+home, "XDG_CONFIG_HOME="+home,
			"GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@example.invalid", "GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@example.invalid",
			"GIT_AUTHOR_DATE=2000-01-01T00:00:00+0000", "GIT_COMMITTER_DATE=2000-01-01T00:00:00+0000")
		out, err := command.CombinedOutput()
		if err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
		return strings.TrimSpace(string(out))
	}
	put := func(path, data string) {
		t.Helper()
		full := filepath.Join(root, filepath.FromSlash(path))
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte(data), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	git("init", "-q", "-b", "main")
	put("docs/rule.txt", "src/app.txt renders the widget\n")
	put("src/app.txt", "alpha\n")
	git("add", ".")
	git("commit", "-qm", "base")
	base := git("rev-parse", "HEAD")
	put("src/app.txt", "alpha\nwidget\n")
	git("commit", "-qam", "change")
	change := git("rev-parse", "HEAD")
	ctx := context.Background()
	session, err := workflow.Open(root)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := session.Prepare(ctx, workflow.PrepareOptions{Base: base, Target: change}); err != nil {
		t.Fatal(err)
	}
	prepared, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(wire.ExcludedCEMPath)))
	if err != nil {
		t.Fatal(err)
	}
	var shape struct {
		Hunks []struct{ ID string } `json:"hunks"`
	}
	if err := json.Unmarshal(prepared, &shape); err != nil || len(shape.Hunks) != 1 {
		t.Fatalf("fixture expects one hunk: %v %s", err, prepared)
	}
	if session, err = workflow.Open(root); err != nil {
		t.Fatal(err)
	}
	if _, err := session.Cite(ctx, workflow.CiteOptions{MapPath: wire.ExcludedCEMPath, Hunk: shape.Hunks[0].ID, EvidencePath: "docs/rule.txt", Lines: "1:1", Relation: "specification"}); err != nil {
		t.Fatal(err)
	}
	git("add", "-f", wire.ExcludedCEMPath)
	git("commit", "-qm", "chore: bind change evidence")
	bind := git("rev-parse", "HEAD")
	if err := os.MkdirAll(filepath.Join(root, ".corvint", "changes"), 0o755); err != nil {
		t.Fatal(err)
	}
	git("mv", wire.ExcludedCEMPath, ".corvint/changes/"+bind+".cem.json")
	git("commit", "-qm", "chore: seal change evidence")
	return root, base, git("rev-parse", "HEAD")
}
