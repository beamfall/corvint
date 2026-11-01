//go:build darwin || linux

package taskman

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"syscall"
	"testing"

	"github.com/Beamfall/corvint/internal/cem/verify"
	"github.com/Beamfall/corvint/internal/cem/wire"
	"github.com/Beamfall/corvint/internal/testsupport"
)

// TestMain lets this test binary answer the portable consumer's one route, so
// the gate's cross-check runs against a real subprocess without these unit
// tests reading the interop module; cmd/corvint's TCB-V0-016 test and the
// interop module's own tests run the real interop/cem01-go consumer. Capture
// tests' work source scratch stays under the test temporary root (V1-2768).
// Every git the tests spawn runs detached from the account's global and system
// configuration (testsupport.IsolateGit, V1-2635).
func TestMain(m *testing.M) {
	if len(os.Args) > 1 && os.Args[1] == portableRoute {
		os.Exit(standInConsumer(os.Args[2:]))
	}
	testsupport.IsolateGit()
	removeRunParent, err := testsupport.PrivateWorkRunParent()
	if err != nil {
		fmt.Fprintln(os.Stderr, "work run parent:", err)
		os.Exit(1)
	}
	code := m.Run()
	if err := removeRunParent(); err != nil {
		fmt.Fprintln(os.Stderr, "remove work run parent:", err)
		code = 1
	}
	os.Exit(code)
}

// standInConsumer answers the closed verify-stable-canonical argv with the
// in-process stable verifier's result document: exit 0 on ACCEPT, 1 on any
// other verdict, 2 on a malformed invocation.
func standInConsumer(args []string) int {
	flags := map[string]string{}
	for i := 0; i+1 < len(args); i += 2 {
		flags[args[i]] = args[i+1]
	}
	for _, name := range []string{"--repository", "--map", "--expected-base", "--target", "--artifacts"} {
		if flags[name] == "" {
			return 2
		}
	}
	if len(args) != 10 || len(flags) != 5 {
		return 2
	}
	raw, err := os.ReadFile(flags["--map"])
	if err != nil {
		return 2
	}
	r, _ := verify.Stable(context.Background(), raw, verify.StableOptions{Repository: flags["--repository"], ExpectedBase: flags["--expected-base"], Target: flags["--target"], ArtifactRoot: flags["--artifacts"]})
	out, err := json.Marshal(r)
	if err != nil {
		return 2
	}
	if _, err := os.Stdout.Write(append(out, '\n')); err != nil {
		return 2
	}
	if r.Outcome == "ACCEPT" {
		return 0
	}
	return 1
}

func portableConsumer(t *testing.T) string {
	t.Helper()
	exe, err := os.Executable()
	if err != nil || !filepath.IsAbs(exe) {
		t.Fatalf("test executable: %q %v", exe, err)
	}
	return filepath.Clean(exe)
}

// patchedConsumer wraps consumer so its result passes through one sed
// expression and the wrapper exits with status; an empty status keeps the
// consumer's own.
func patchedConsumer(t *testing.T, consumer, expression, status string) string {
	t.Helper()
	if status == "" {
		status = `"$status"`
	}
	path := filepath.Join(t.TempDir(), "patched-consumer")
	script := "#!/bin/sh\nout=$('" + consumer + "' \"$@\")\nstatus=$?\nprintf '%s\\n' \"$out\" | sed -e '" + expression + "'\nexit " + status + "\n"
	if err := os.WriteFile(path, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestTCBV0017PortableConsumerMustRecomputeTheSameVerdict(t *testing.T) {
	f := newVerdictFixture(t, nil)
	in := f.input(PredicateTouchedAnchored, "src/")
	result, view := evaluateVerdict(t, in)
	if !result.Granted || view.Portable == nil {
		t.Fatalf("agreeing consumers must grant and pin the portable verdict: %s", result.Document)
	}
	citationSha256 := sum(canonical(mustParse(t, view.Citation)))
	if view.Portable.Route != portableRoute || view.Portable.Outcome != "ACCEPT" || view.Portable.Stage != "complete" || view.Portable.Code != nil || view.Portable.VerdictSha256 != view.Verdict.VerdictSha256 || view.Portable.CitationSha256 == nil || *view.Portable.CitationSha256 != citationSha256 {
		t.Fatalf("portable pin: %+v want verdict %s citation %s", view.Portable, view.Verdict.VerdictSha256, citationSha256)
	}
	native := view.Verdict.VerdictSha256
	for name, patch := range map[string]struct {
		expression, claimed, status string
	}{
		// The consumer's own verdictSha256 claim differs.
		"claimed verdict": {`s/"verdictSha256":"[0-9a-f]*"/"verdictSha256":"` + strings.Repeat("0", 64) + `"/`, strings.Repeat("0", 64), ""},
		// A raised tier under an unchanged claim: the recomputed digest and
		// the citation block both differ.
		"raised tier": {`s/"tier":"cited"/"tier":"anchored"/`, native, ""},
		// The outcome differs under an unchanged claim, with the exit status
		// a REJECT carries.
		"outcome": {`s/"outcome":"ACCEPT"/"outcome":"REJECT"/`, native, "1"},
	} {
		disagreeing := in
		disagreeing.PortableConsumer = patchedConsumer(t, f.portable, patch.expression, patch.status)
		view := requireVerdictRefusal(t, disagreeing, CEMVerdictVerifierDisagreement, "verifier-disagreement")
		if view.Verdict == nil || view.Verdict.VerdictSha256 != native || view.Portable == nil || view.Portable.VerdictSha256 != patch.claimed || string(view.Citation) != "null" || len(view.Governed) != 0 || len(view.Paths) != 0 {
			t.Fatalf("%s: a disagreement must pin both verdicts and evaluate no predicate: %+v %+v", name, view.Verdict, view.Portable)
		}
	}
	garbage := filepath.Join(t.TempDir(), "garbage-consumer")
	if err := os.WriteFile(garbage, []byte("#!/bin/sh\necho not-a-result\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	killed := filepath.Join(t.TempDir(), "killed-consumer")
	if err := os.WriteFile(killed, []byte("#!/bin/sh\nout=$('"+f.portable+"' \"$@\")\nprintf '%s\\n' \"$out\"\nkill -KILL $$\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	for name, consumer := range map[string]string{
		"missing executable": filepath.Join(t.TempDir(), "absent"),
		"unreadable result":  garbage,
		"other profile":      patchedConsumer(t, f.portable, `s/"profile":"cem-stable-verification\/1"/"profile":"other\/1"/`, ""),
		"no verdict digest":  patchedConsumer(t, f.portable, `s/"verdictSha256":"[0-9a-f]*"/"verdictSha256":"x"/`, ""),
		// An agreeing ACCEPT document whose process then fails is no result.
		"accept, exit 2": patchedConsumer(t, f.portable, `s/^//`, "2"),
		"accept, exit 1": patchedConsumer(t, f.portable, `s/^//`, "1"),
		"accept, killed": killed,
	} {
		failing := in
		failing.PortableConsumer = consumer
		if view := requireVerdictRefusal(t, failing, CEMVerdictUnverifiable, "portable-consumer-failed"); view.Verdict == nil || view.Portable != nil {
			t.Fatalf("%s: an unrunnable consumer pins the native verdict and no portable one: %+v %+v", name, view.Verdict, view.Portable)
		}
	}
}

// A consumer that cannot run its route on the host answers UNSUPPORTED with
// its own closed code. The gate pins that stage and code (TCB-V0-040) and
// refuses under the route's name, not as a disagreement (TCB-V0-041); a stage
// or code outside the closed token shape is no readable result.
func TestTCBV0040PortableUnsupportedRefusesNamingTheRoute(t *testing.T) {
	f := newVerdictFixture(t, nil)
	in := f.input(PredicateTouchedAnchored, "src/")
	_, agreeing := evaluateVerdict(t, in)
	native := agreeing.Verdict.VerdictSha256
	// The hosted macos-15 condition: the git the consumer's PATH resolves
	// rejects the stable diff's --attr-source option, so the consumer's own
	// repository read fails and it answers UNSUPPORTED with its own code and
	// a verdictSha256 that its result recomputes to (TCB-V0-041).
	refusing := in
	refusing.PortableConsumer = attrSourceRejectingConsumer(t, f.portable, "")
	view := requireVerdictRefusal(t, refusing, CEMVerdictUnverifiable, "portable-route-unsupported")
	if view.Verdict == nil || view.Verdict.Outcome != "ACCEPT" || view.Verdict.VerdictSha256 != native || view.Portable == nil || view.Portable.Route != portableRoute || view.Portable.Outcome != "UNSUPPORTED" || view.Portable.Stage != "repository" || view.Portable.Code == nil || *view.Portable.Code != "git-diff-failed" || view.Portable.VerdictSha256 == native || !digest(view.Portable.VerdictSha256) || view.Portable.CitationSha256 != nil || string(view.Citation) != "null" || len(view.Governed) != 0 || len(view.Paths) != 0 {
		t.Fatalf("an unsupported route must pin the native verdict and the consumer's stage and code, and evaluate no predicate: %+v %+v", view.Verdict, view.Portable)
	}
	// UNSUPPORTED with the exit status of an ACCEPT is a result its process
	// does not explain.
	mismatched := in
	mismatched.PortableConsumer = attrSourceRejectingConsumer(t, f.portable, "0")
	if view := requireVerdictRefusal(t, mismatched, CEMVerdictUnverifiable, "portable-consumer-failed"); view.Portable != nil {
		t.Fatalf("exit 0: %+v", view.Portable)
	}
	// An UNSUPPORTED whose verdictSha256 claim is not the digest of its own
	// result is not that consumer's answer: it is a disagreement, so the
	// named refusal never pins a false claim.
	claimed := in
	claimed.PortableConsumer = patchedConsumer(t, f.portable, `s/"outcome":"ACCEPT"/"outcome":"UNSUPPORTED"/;s/"stage":"complete"/"stage":"repository"/;s/"code":null/"code":"git-diff-failed"/`, "2")
	if view := requireVerdictRefusal(t, claimed, CEMVerdictVerifierDisagreement, "verifier-disagreement"); view.Portable == nil || view.Portable.VerdictSha256 != native {
		t.Fatalf("false claim: %+v", view.Portable)
	}
	for name, expression := range map[string]string{
		"open code":     `s/"code":null/"code":"Git read failed: \/usr\/bin\/git"/`,
		"unknown code":  `s/"code":null/"code":"private-hostname"/`,
		"empty code":    `s/"code":null/"code":""/`,
		"numeric code":  `s/"code":null/"code":7/`,
		"open stage":    `s/"stage":"complete"/"stage":"Repository Read"/`,
		"unknown stage": `s/"stage":"complete"/"stage":"host"/`,
		"missing stage": `s/"stage":"complete",//`,
		"missing code":  `s/"code":null,//`,
	} {
		// The consumer's own ACCEPT exit status, so only the pin is unreadable.
		unreadable := in
		unreadable.PortableConsumer = patchedConsumer(t, f.portable, expression, "")
		if view := requireVerdictRefusal(t, unreadable, CEMVerdictUnverifiable, "portable-consumer-failed"); view.Verdict == nil || view.Portable != nil {
			t.Fatalf("%s: an unreadable stage or code pins no portable result: %+v %+v", name, view.Verdict, view.Portable)
		}
	}
}

// The stage and code sets the gate admits into the document are exactly the
// portable consumer's result contract (TCB-V0-040).
func TestTCBV0040PortablePinSetsMatchTheResultSchema(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join("..", "..", "protocol", "cem-1.0", "stable", "RESULT.schema.json"))
	if err != nil {
		t.Fatal(err)
	}
	var schema struct {
		Properties struct {
			Stage struct{ Enum []string }  `json:"stage"`
			Code  struct{ Enum []*string } `json:"code"`
		} `json:"properties"`
	}
	if err := json.Unmarshal(raw, &schema); err != nil {
		t.Fatal(err)
	}
	stages := closedSet(schema.Properties.Stage.Enum...)
	codes := map[string]bool{}
	for _, code := range schema.Properties.Code.Enum {
		if code != nil {
			codes[*code] = true
		}
	}
	if len(stages) == 0 || len(codes) == 0 || !reflect.DeepEqual(stages, portableStages) || !reflect.DeepEqual(codes, portableCodes) {
		t.Fatalf("schema stages %v codes %v; gate stages %v codes %v", stages, codes, portableStages, portableCodes)
	}
}

// attrSourceRejectingConsumer wraps consumer so that the first git on its
// PATH is a shim that rejects --attr-source as Git before 2.41 does and
// otherwise runs the gate's own git; a non-empty status replaces the
// consumer's exit status.
func attrSourceRejectingConsumer(t *testing.T, consumer, status string) string {
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
	if err := os.WriteFile(filepath.Join(shim, "git"), []byte(git), 0o755); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "old-git-consumer")
	script := "#!/bin/sh\nPATH='" + shim + ":/usr/bin:/bin'\nexport PATH\n"
	if status == "" {
		script += "exec '" + consumer + "' \"$@\"\n"
	} else {
		script += "'" + consumer + "' \"$@\"\nexit " + status + "\n"
	}
	if err := os.WriteFile(path, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	return path
}

// The portable consumer runs in the gate's own process group, so a supervisor
// that kills that group, as the Tasks runner does on timeout, stops it too.
func TestTCBV0017PortableConsumerSharesTheGateProcessGroup(t *testing.T) {
	f := newVerdictFixture(t, nil)
	in := f.input(PredicateTouchedAnchored, "src/")
	group := filepath.Join(t.TempDir(), "group")
	in.PortableConsumer = filepath.Join(t.TempDir(), "group-consumer")
	if err := os.WriteFile(in.PortableConsumer, []byte("#!/bin/sh\nps -o pgid= -p $$ > '"+group+"'\nexec '"+f.portable+"' \"$@\"\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	if result, _ := evaluateVerdict(t, in); !result.Granted {
		t.Fatalf("the wrapped consumer must still grant: %s", result.Document)
	}
	raw, err := os.ReadFile(group)
	if err != nil {
		t.Fatal(err)
	}
	if pgid, err := strconv.Atoi(strings.TrimSpace(string(raw))); err != nil || pgid != syscall.Getpgrp() {
		t.Fatalf("portable consumer group %q, gate group %d", raw, syscall.Getpgrp())
	}
}

// The portable consumer resolves git from the gate's own git first: the
// directory exec.LookPath resolves in the gate's process, ahead of the fixed
// /usr/bin:/bin, so both legs of the cross-check run one Git (TCB-V0-043).
func TestTCBV0043PortableConsumerResolvesTheGateGit(t *testing.T) {
	real, err := exec.LookPath("git")
	if err != nil || !filepath.IsAbs(real) {
		t.Fatalf("git: %q %v", real, err)
	}
	shimDir := filepath.Join(t.TempDir(), "bin")
	if err := os.Mkdir(shimDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(shimDir, "git"), []byte("#!/bin/sh\nexec '"+real+"' \"$@\"\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", shimDir+":"+os.Getenv("PATH"))
	if got := portablePath(); got != shimDir+":"+portableFixedPath {
		t.Fatalf("portablePath %q want %q", got, shimDir+":"+portableFixedPath)
	}
	f := newVerdictFixture(t, nil)
	in := f.input(PredicateTouchedAnchored, "src/")
	seen := filepath.Join(t.TempDir(), "seen")
	in.PortableConsumer = filepath.Join(t.TempDir(), "recording-consumer")
	if err := os.WriteFile(in.PortableConsumer, []byte("#!/bin/sh\nprintf '%s\\n%s\\n' \"$PATH\" \"$(command -v git)\" > '"+seen+"'\nexec '"+f.portable+"' \"$@\"\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	if result, _ := evaluateVerdict(t, in); !result.Granted {
		t.Fatalf("the consumer under the gate's git must grant: %s", result.Document)
	}
	raw, err := os.ReadFile(seen)
	if err != nil {
		t.Fatal(err)
	}
	if want := shimDir + ":" + portableFixedPath + "\n" + filepath.Join(shimDir, "git") + "\n"; string(raw) != want {
		t.Fatalf("consumer saw %q want %q", raw, want)
	}
	// A git already under the fixed path, no git, or a relative lookup keeps
	// the fixed path alone.
	for name, path := range map[string]string{"fixed": portableFixedPath, "empty": "", "relative": "bin:" + portableFixedPath} {
		t.Setenv("PATH", path)
		if name == "relative" {
			t.Chdir(filepath.Dir(shimDir))
		}
		if got := portablePath(); got != portableFixedPath {
			t.Fatalf("%s: portablePath %q want %q", name, got, portableFixedPath)
		}
	}
}

func TestTCBV0018PathsCarryTheMinimumRecomputedTier(t *testing.T) {
	g := &verdictGate{doc: cemVerdictDocument{Paths: []cemVerdictPath{}}}
	hunks := []wire.Hunk{{ID: "h1", Path: "src/a.go"}, {ID: "h2", Path: "src/a.go"}, {ID: "h3", Path: "lib/b.go"}, {ID: "h4", Path: "docs/c.md"}}
	citation := &verify.Citation{Hunks: []verify.CitationHunk{{HunkID: "h1", Tier: verify.TierHeld}, {HunkID: "h2", Tier: verify.TierCited}, {HunkID: "h3", Tier: verify.TierAnchored}, {HunkID: "h4", Tier: verify.TierUnknown}}}
	g.predicate(PredicateTouchedAnchored, []string{"lib/", "src/"}, hunks, citation, nil)
	want := []cemVerdictPath{{Path: "lib/b.go", Tier: verify.TierAnchored, Satisfied: true}, {Path: "src/a.go", Tier: verify.TierCited, Satisfied: false}}
	if len(g.doc.Paths) != len(want) || g.doc.Paths[0] != want[0] || g.doc.Paths[1] != want[1] || g.doc.Outcome == "GRANT" {
		t.Fatalf("paths: %+v want %+v (%s)", g.doc.Paths, want, g.doc.Outcome)
	}

	f := newVerdictFixture(t, nil)
	all := requireVerdictRefusal(t, f.input(PredicateAllAnchored), CEMVerdictPredicateFailed, "tier-below-anchored")
	if len(all.Paths) != 2 || all.Paths[0] != (cemVerdictPath{Path: "lib/util.txt", Tier: verify.TierCited}) || all.Paths[1] != (cemVerdictPath{Path: "src/app.txt", Tier: verify.TierAnchored, Satisfied: true}) {
		t.Fatalf("all-anchored paths: %+v", all.Paths)
	}
	result, granted := evaluateVerdict(t, f.input(PredicateTouchedAnchored, "src/"))
	if !result.Granted || len(granted.Paths) != 1 || granted.Paths[0] != (cemVerdictPath{Path: "src/app.txt", Tier: verify.TierAnchored, Satisfied: true}) {
		t.Fatalf("touched paths: %+v\n%s", granted.Paths, result.Document)
	}
}

func TestTCBV0018ProducerEditedTierIsNeverAdmitted(t *testing.T) {
	// The producer claims a held tier on every hunk of the map it binds and
	// seals; the consumer recomputes tiers and admits no producer member.
	f := newVerdictFixture(t, func(raw []byte) []byte {
		edited := bytes.ReplaceAll(raw, []byte(`"reason":`), []byte(`"tier":"held","reason":`))
		if bytes.Equal(edited, raw) {
			t.Fatal("the tier edit changed nothing")
		}
		return edited
	})
	view := requireVerdictRefusal(t, f.input(PredicateAllAnchored), CEMVerdictUnverifiable, "stable-not-accepted")
	if view.Verdict == nil || view.Verdict.Outcome == "ACCEPT" || string(view.Citation) != "null" || len(view.Governed) != 0 || len(view.Paths) != 0 {
		t.Fatalf("a producer tier must not reach the predicate: %+v %s", view.Verdict, view.Citation)
	}
}
