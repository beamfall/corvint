//go:build darwin || linux

package taskman

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"

	"github.com/Beamfall/corvint/internal/cem/verify"
	"github.com/Beamfall/corvint/internal/cem/wire"
)

// TestMain lets this test binary answer the portable consumer's one route, so
// the gate's cross-check runs against a real subprocess without these unit
// tests reading the interop module; cmd/corvint's TCB-V0-016 test and the
// interop module's own tests run the real interop/cem01-go consumer.
func TestMain(m *testing.M) {
	if len(os.Args) > 1 && os.Args[1] == portableRoute {
		os.Exit(standInConsumer(os.Args[2:]))
	}
	os.Exit(m.Run())
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
	unsupported := `s/"outcome":"ACCEPT"/"outcome":"UNSUPPORTED"/;s/"stage":"complete"/"stage":"repository"/;s/"code":null/"code":"git-read-failed"/`
	for name, status := range map[string]string{"exit 2": "2", "exit 1": "1"} {
		refusing := in
		refusing.PortableConsumer = patchedConsumer(t, f.portable, unsupported, status)
		view := requireVerdictRefusal(t, refusing, CEMVerdictUnverifiable, "portable-route-unsupported")
		if view.Verdict == nil || view.Verdict.Outcome != "ACCEPT" || view.Verdict.VerdictSha256 != native || view.Portable == nil || view.Portable.Route != portableRoute || view.Portable.Outcome != "UNSUPPORTED" || view.Portable.Stage != "repository" || view.Portable.Code == nil || *view.Portable.Code != "git-read-failed" || view.Portable.VerdictSha256 != native || string(view.Citation) != "null" || len(view.Governed) != 0 || len(view.Paths) != 0 {
			t.Fatalf("%s: an unsupported route must pin the native verdict and the consumer's stage and code, and evaluate no predicate: %+v %+v", name, view.Verdict, view.Portable)
		}
	}
	// UNSUPPORTED with the exit status of an ACCEPT is a result its process
	// does not explain.
	mismatched := in
	mismatched.PortableConsumer = patchedConsumer(t, f.portable, unsupported, "0")
	if view := requireVerdictRefusal(t, mismatched, CEMVerdictUnverifiable, "portable-consumer-failed"); view.Portable != nil {
		t.Fatalf("exit 0: %+v", view.Portable)
	}
	for name, expression := range map[string]string{
		"open code":     `s/"code":null/"code":"Git read failed: \/usr\/bin\/git"/`,
		"empty code":    `s/"code":null/"code":""/`,
		"numeric code":  `s/"code":null/"code":7/`,
		"open stage":    `s/"stage":"complete"/"stage":"Repository Read"/`,
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

func TestTCBV0018PathsCarryTheMinimumRecomputedTier(t *testing.T) {
	g := &verdictGate{doc: cemVerdictDocument{Paths: []cemVerdictPath{}}}
	hunks := []wire.Hunk{{ID: "h1", Path: "src/a.go"}, {ID: "h2", Path: "src/a.go"}, {ID: "h3", Path: "lib/b.go"}, {ID: "h4", Path: "docs/c.md"}}
	citation := &verify.Citation{Hunks: []verify.CitationHunk{{HunkID: "h1", Tier: verify.TierHeld}, {HunkID: "h2", Tier: verify.TierCited}, {HunkID: "h3", Tier: verify.TierAnchored}, {HunkID: "h4", Tier: verify.TierUnknown}}}
	g.predicate(PredicateTouchedAnchored, []string{"lib/", "src/"}, hunks, citation)
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
