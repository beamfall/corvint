package main

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/cem/stablefixture"
	"github.com/Beamfall/corvint/internal/cem/wire"
	"github.com/Beamfall/corvint/internal/cem/workflow"
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
		t.Fatalf("exit %d stderr %q\ncommand %s\nlibrary %s", code, stderr, stdout, library.Document)
	}
	var doc struct {
		Outcome string `json:"outcome"`
		Verdict struct {
			Outcome, VerdictSha256 string
		} `json:"verdict"`
		Portable struct {
			Route, Outcome, VerdictSha256 string
			CitationSha256                *string
		} `json:"portable"`
		Paths []struct {
			Path, Tier string
			Satisfied  bool
		} `json:"paths"`
	}
	if err := json.Unmarshal([]byte(stdout), &doc); err != nil {
		t.Fatal(err)
	}
	if doc.Outcome != "GRANT" || doc.Portable.Route != "verify-stable-canonical" || doc.Portable.Outcome != "ACCEPT" || doc.Portable.VerdictSha256 != doc.Verdict.VerdictSha256 || doc.Portable.CitationSha256 == nil {
		t.Fatalf("pins: %+v", doc)
	}
	if len(doc.Paths) != 1 || doc.Paths[0].Path != "src/app.txt" || doc.Paths[0].Tier != "anchored" || !doc.Paths[0].Satisfied {
		t.Fatalf("paths: %+v", doc.Paths)
	}
	patched := filepath.Join(t.TempDir(), "patched-cem01-go")
	script := "#!/bin/sh\n'" + consumer + "' \"$@\" | sed -e 's/\"tier\":\"anchored\"/\"tier\":\"cited\"/'\n"
	if err := os.WriteFile(patched, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	code, stdout, stderr = runCLI(t, append([]string{"--root", root}, args(patched)...)...)
	if code != 1 || stderr != "" || !strings.Contains(stdout, `"code":"`+taskman.CEMVerdictVerifierDisagreement+`"`) || !strings.Contains(stdout, `"reason":"verifier-disagreement"`) || !strings.Contains(stdout, `"outcome":"REFUSE"`) {
		t.Fatalf("patched consumer: exit %d stderr %q\n%s", code, stderr, stdout)
	}
}

// buildPortableConsumer builds the independent interop/cem01-go module.
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
