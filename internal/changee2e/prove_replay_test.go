package changee2e

import (
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/jstestprovider"
)

// recordingSchema is one recorded side of a scenario proof (CCE-V0-100/101): the canonical
// corvint-playwright-external/3 document one pool produced, the staged-tree root it ran in, and
// the pin of the compiled scenario it ran.
const recordingSchema = "cce-recorded-side/0"

const (
	recordingDir     = "testdata/recorded"
	syntheticTree    = "/corvint-recorded/tree"
	envRecordDir     = "CORVINT_CCE_RECORD_DIR"
	provenanceLive   = "live"
	provenanceSynth  = "synthetic"
	reasonRecMissing = "recording-missing"
	reasonRecInvalid = "recording-invalid"
	reasonRecStale   = "recording-stale"
)

var writeRecordings = flag.Bool("cce-write-recordings", false, "rewrite the synthetic CCE recorded receipts (CCE-V0-101)")

type recordedSide struct {
	Schema     string `json:"schema"`
	Provenance string `json:"provenance"`
	Toolchain  string `json:"toolchain"`
	Compiler   string `json:"compiler"`
	Scenario   string `json:"scenario_sha256"`
	Tree       string `json:"tree"`
	Receipt    string `json:"receipt"`
}

// recordedCases are the recorded ladder rungs: each case names its two recorded sides and the
// verdict a replay must read with every control killed.
var recordedCases = []struct {
	name, want, note string
}{
	{"discriminates", VerdictDiscriminates, "value mismatch"},
	{"new-reach", VerdictNewReach, "locator did not resolve"},
	{"preserves", VerdictPreserves, "regression-preservation"},
	{"regression", VerdictRegressionDetected, "the change broke a declared outcome"},
	{"unstable", VerdictUnstable, "head-pool-intermittent"},
}

// scenarioPin is the sha256 of the compiled scenario below its header comment, which names the
// fixture's commit revisions; a recording replays only against the statements it ran.
func scenarioPin(sc Scenario) string {
	lines := strings.SplitAfter(string(sc.Text), "\n")
	for len(lines) > 0 && strings.HasPrefix(lines[0], "//") {
		lines = lines[1:]
	}
	return sha256Hex([]byte(strings.Join(lines, "")))
}

func loadRecording(name string) (*recordedSide, error) {
	data, err := os.ReadFile(filepath.Join(recordingDir, name+".json"))
	if err != nil {
		return nil, err
	}
	var rec recordedSide
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&rec); err != nil {
		return nil, err
	}
	return &rec, nil
}

// replaySide is the CCE-V0-100 offline side: it decodes a recorded receipt with the provider's own
// canonical decoder, qualifies it with Prove's qualify against the recorded tree root, and moves
// only the receipt's tree locations onto the staged tree. A missing, non-canonical or stale
// recording leaves the pool unqualified with a named reason, never a synthesized outcome.
func replaySide(rec *recordedSide, sc Scenario, in runInput) (jstestprovider.Receipt, string, string) {
	if rec == nil {
		return jstestprovider.Receipt{}, "", reasonRecMissing
	}
	// The recorded tree is slash-separated and may come from another platform; it is matched
	// textually, then the receipt's tree locations move onto the host's staged tree before qualify
	// reads them, exactly as a live receipt from that tree would.
	recTree := strings.TrimSuffix(filepath.ToSlash(rec.Tree), "/")
	if rec.Schema != recordingSchema || (rec.Provenance != provenanceLive && rec.Provenance != provenanceSynth) ||
		!(path.IsAbs(recTree) || filepath.IsAbs(rec.Tree)) {
		return jstestprovider.Receipt{}, "", reasonRecInvalid
	}
	r, err := jstestprovider.DecodeAttemptReceipt([]byte(rec.Receipt))
	if err != nil {
		return jstestprovider.Receipt{}, "", reasonRecInvalid
	}
	if rec.Scenario != scenarioPin(sc) {
		return jstestprovider.Receipt{}, "", reasonRecStale
	}
	relocate := func(a *jstestprovider.Anchor) {
		if a == nil {
			return
		}
		if rel, ok := strings.CutPrefix(filepath.ToSlash(a.File), recTree+"/"); ok {
			a.File = filepath.Join(in.Tree, filepath.FromSlash(rel))
		}
	}
	for i := range r.Tests {
		for j := range r.Tests[i].AttemptDetails {
			relocate(r.Tests[i].AttemptDetails[j].FailureLocation)
		}
	}
	if r.Infrastructure != nil {
		relocate(r.Infrastructure.Location)
	}
	if _, reason := qualify(r, nil, in.Repeats, treeRoots(in.Tree)); reason != "" {
		return r, "", reason
	}
	// The side's digest is the recorded canonical document's, which DecodeAttemptReceipt accepted.
	return r, "sha256:" + sha256Hex([]byte(rec.Receipt)), ""
}

// replayEnv is a proveEnv whose sides replay recorded receipts by role; the BBF controls stay
// scripted, because a control's verdict needs its own run (CCE-V0-028).
func replayEnv(root, head string, sc Scenario, sides map[string]*recordedSide, controls string) *proveEnv {
	f := &fakeRunner{controls: controls}
	e := f.env(root)
	e.side = func(_ context.Context, in runInput) (jstestprovider.Receipt, string, string, error) {
		role := "base"
		if in.AppIdentity == head {
			role = "head"
		}
		r, digest, reason := replaySide(sides[role], sc, in)
		return r, digest, reason, nil
	}
	return e
}

// CCE-V0-100 (V1-1687): every recorded ladder rung replays offline through the provider's decoder,
// Prove's qualify and the real staging, location and verdict code, in the ordinary go test lane.
func TestCCEV0100ReplayRecordedReceipts(t *testing.T) {
	root, base := fixtureRepo(t)
	head := commitBehaviourChange(t, root)
	sc := compiledScenario(t, root, base, head)
	if *writeRecordings {
		writeSyntheticRecordings(t, sc)
	}
	for _, tc := range recordedCases {
		t.Run(tc.name, func(t *testing.T) {
			sides := map[string]*recordedSide{}
			for _, role := range []string{"head", "base"} {
				rec, err := loadRecording(tc.name + "." + role)
				if err != nil {
					t.Fatalf("recording %s.%s: %v", tc.name, role, err)
				}
				if rec.Compiler != CompilerVersion {
					t.Fatalf("recording %s.%s was made by compiler %s, not %s: re-record it (CCE-V0-101)", tc.name, role, rec.Compiler, CompilerVersion)
				}
				sides[role] = rec
			}
			p := replayEnv(root, head, sc, sides, "killed").proveScenario(bg, base, head, sc, 3)
			if p.Verdict != tc.want || !strings.Contains(p.Note, tc.note) {
				t.Fatalf("verdict %s (%s) head %q base %q, want %s containing %q", p.Verdict, p.Note, p.Head.Reason, p.Base.Reason, tc.want, tc.note)
			}
			for role, side := range map[string]sideProof{"head": p.Head, "base": p.Base} {
				want := "sha256:" + sha256Hex([]byte(sides[role].Receipt))
				if !side.Qualified || len(side.Receipts) != 1 || side.Receipts[0] != want {
					t.Fatalf("%s side receipts %v (qualified %v), want the recorded document digest %s", role, side.Receipts, side.Qualified, want)
				}
			}
		})
	}
}

// CCE-V0-100: a recording that is missing, not the canonical document, or made for other scenario
// text leaves its side unqualified and the verdict INCONCLUSIVE.
func TestCCEV0100ReplayRefusals(t *testing.T) {
	root, base := fixtureRepo(t)
	head := commitBehaviourChange(t, root)
	sc := compiledScenario(t, root, base, head)
	good, err := loadRecording("discriminates.base")
	if err != nil {
		t.Fatal(err)
	}
	headRec, err := loadRecording("discriminates.head")
	if err != nil {
		t.Fatal(err)
	}
	padded := *good
	padded.Receipt += "\n"
	schema := *good
	schema.Schema = "cce-recorded-side/9"
	relative := *good
	relative.Tree = strings.TrimPrefix(filepath.ToSlash(good.Tree), "/")
	edited := sc
	edited.Text = append(bytes.Clone(sc.Text), "// edited\n"...)
	for _, tc := range []struct {
		name   string
		base   *recordedSide
		sc     Scenario
		reason string
	}{
		{"missing", nil, sc, reasonRecMissing},
		{"non-canonical", &padded, sc, reasonRecInvalid},
		{"unknown schema", &schema, sc, reasonRecInvalid},
		{"relative tree", &relative, sc, reasonRecInvalid},
		{"other scenario text", good, edited, reasonRecStale},
	} {
		t.Run(tc.name, func(t *testing.T) {
			p := replayEnv(root, head, tc.sc, map[string]*recordedSide{"head": headRec, "base": tc.base}, "killed").proveScenario(bg, base, head, sc, 3)
			if p.Verdict != VerdictInconclusive || p.Base.Qualified || p.Base.Reason != tc.reason || len(p.Base.Receipts) != 0 {
				t.Fatalf("verdict %s (%s), base %+v; want INCONCLUSIVE with base reason %s", p.Verdict, p.Note, p.Base, tc.reason)
			}
		})
	}
	// The recorded slash tree moves onto the host's staged tree, whatever its separator.
	tree := t.TempDir()
	r, digest, reason := replaySide(good, sc, runInput{Tree: tree, Repeats: 3})
	moved := 0
	for _, o := range r.Tests {
		for _, d := range o.AttemptDetails {
			if d.FailureLocation == nil {
				continue
			}
			if rel, err := filepath.Rel(tree, d.FailureLocation.File); err != nil || strings.HasPrefix(rel, "..") {
				t.Fatalf("failure location %q not under the staged tree %q", d.FailureLocation.File, tree)
			}
			moved++
		}
	}
	if reason != "" || digest != "sha256:"+sha256Hex([]byte(good.Receipt)) || moved == 0 {
		t.Fatalf("relocated replay: reason %q digest %q, %d locations moved", reason, digest, moved)
	}
	// A recording of three repeats replayed as a pool of two reads the provider's count mismatch.
	p := replayEnv(root, head, sc, map[string]*recordedSide{"head": headRec, "base": good}, "killed").proveScenario(bg, base, head, sc, 2)
	if p.Verdict != VerdictInconclusive || p.Head.Reason != "repeat-count-mismatch" {
		t.Fatalf("two-repeat replay: verdict %s, head reason %q", p.Verdict, p.Head.Reason)
	}
}

// CCE-V0-101: with CORVINT_CCE_RECORD_DIR set and the live toolchain present, the live ladder
// writes each qualified side as a live recording that replaces the synthetic one in testdata.
func TestCCEV0101LiveRecord(t *testing.T) {
	dir := os.Getenv(envRecordDir)
	if dir == "" {
		t.Skip(envRecordDir + " is not set")
	}
	modules, node := liveToolchain(t)
	hook, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	o := ProveOptions{Repeats: DefaultRepeats, NodeModules: modules, NodeBin: node, HookExecutable: hook}
	restore := setEnv(proveEnvironment(o))
	defer restore()
	for _, c := range liveRecordingCases(t) {
		e, err := proveToolchain(c.root, o)
		if err != nil {
			t.Fatal(err)
		}
		toolchain := "playwright " + e.runnerVersion
		e.side = func(ctx context.Context, in runInput) (jstestprovider.Receipt, string, string, error) {
			r, digest, reason, runErr := runQualified(ctx, in)
			if reason != "" {
				t.Fatalf("%s: live side unqualified: %s", c.name, reason)
			}
			raw, err := jstestprovider.EncodeQualified(r)
			if err != nil {
				t.Fatal(err)
			}
			tree, _ := filepath.EvalSymlinks(in.Tree)
			role := "base"
			if in.AppIdentity == c.head {
				role = "head"
			}
			writeRecording(t, filepath.Join(dir, c.name+"."+role+".json"), recordedSide{Schema: recordingSchema, Provenance: provenanceLive,
				Toolchain: toolchain + " node " + r.Identity.NodeVersion, Compiler: CompilerVersion, Scenario: scenarioPin(c.sc), Tree: tree, Receipt: string(raw)})
			return r, digest, reason, runErr
		}
		p := e.proveScenario(bg, c.base, c.head, c.sc, DefaultRepeats)
		t.Logf("%s: %s (%s)", c.name, p.Verdict, p.Note)
	}
}

type liveRecordingCase struct {
	name             string
	root, base, head string
	sc               Scenario
}

// liveRecordingCases are the rungs a live run can produce on the static fixture: the behaviour
// change discriminates, and a class rename over it preserves.
func liveRecordingCases(t *testing.T) []liveRecordingCase {
	root, base := fixtureRepo(t)
	head := commitBehaviourChange(t, root)
	out := []liveRecordingCase{{"discriminates", root, base, head, compiledScenario(t, root, base, head)}}
	proot, _ := fixtureRepo(t)
	pbase := commitBehaviourChange(t, proot)
	booked := strings.ReplaceAll(*overlayText(t, "app/components/teesheet.head.html"), `class="slot open"`, `class="slot available"`)
	phead := commitFiles(t, proot, map[string]*string{"app/components/teesheet.html": text(booked)}, "rename slot class")
	rep, err := Compile(bg, diffOpts(proot, pbase, phead))
	if err != nil || len(rep.Compiled()) != 1 {
		t.Fatalf("compile: %v", err)
	}
	return append(out, liveRecordingCase{"preserves", proot, pbase, phead, rep.Compiled()[0]})
}

func writeRecording(t *testing.T, path string, rec recordedSide) {
	t.Helper()
	data, err := json.MarshalIndent(rec, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, append(data, '\n'), 0o644); err != nil {
		t.Fatal(err)
	}
}

// writeSyntheticRecordings writes the synthetic recordings: each is a canonical /3 document of the
// live receipt's shape for one pool of the compiled fixture scenario (CCE-V0-101). A live recording
// of the same case replaces it.
func writeSyntheticRecordings(t *testing.T, sc Scenario) {
	t.Helper()
	mismatch := assertionMessage(sc, `"Pending"`)
	unresolved := assertionMessage(sc, "<element(s) not found>")
	pass := []string{"passed", "passed", "passed"}
	fail := []string{"failed", "failed", "failed"}
	pools := map[string]struct {
		states  []string
		message string
	}{
		"discriminates.head": {pass, ""}, "discriminates.base": {fail, mismatch},
		"new-reach.head": {pass, ""}, "new-reach.base": {fail, unresolved},
		"preserves.head": {pass, ""}, "preserves.base": {pass, ""},
		"regression.head": {fail, mismatch}, "regression.base": {pass, ""},
		"unstable.head": {[]string{"passed", "failed", "passed"}, mismatch}, "unstable.base": {pass, ""},
	}
	if err := os.MkdirAll(recordingDir, 0o755); err != nil {
		t.Fatal(err)
	}
	for name, pool := range pools {
		r := syntheticReceipt(sc, strings.HasSuffix(name, ".head"), pool.states, pool.message)
		raw, err := jstestprovider.EncodeQualified(r)
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		if _, err := jstestprovider.DecodeAttemptReceipt(raw); err != nil {
			t.Fatalf("%s: the synthetic document does not decode: %v", name, err)
		}
		writeRecording(t, filepath.Join(recordingDir, name+".json"), recordedSide{Schema: recordingSchema, Provenance: provenanceSynth,
			Toolchain: "playwright 1.64.0 node v24.11.1 (synthetic)", Compiler: CompilerVersion, Scenario: scenarioPin(sc), Tree: syntheticTree, Receipt: string(raw)})
	}
}

// syntheticReceipt is a qualified /3 receipt with the fields a live Prove pool records: the
// generated config and placement under the tree, the declared application identity, and one
// outcome per repeat whose failed attempts name the scenario's assertion line.
func syntheticReceipt(sc Scenario, head bool, states []string, message string) jstestprovider.Receipt {
	sum := func(s string) string { return sha256Hex([]byte(s)) }
	config := filepath.Join(syntheticTree, proveConfigName)
	placement := filepath.Join(syntheticTree, filepath.FromSlash(sc.Placement))
	app := "base"
	if head {
		app = "head"
	}
	r := jstestprovider.Receipt{Profile: jstestprovider.AttemptExternalProfile, Kind: "e2e",
		Identity: jstestprovider.Identity{ConfigFile: config, ConfigDigest: sum(proveConfig), ConfigInputDigests: map[string]string{config: sum(proveConfig)},
			TestFileDigests: map[string]string{placement: sum(string(sc.Text))}, PackageDigest: sum("package"), NodeVersion: "v24.11.1",
			RunnerName: "playwright", RunnerVersion: "1.64.0", Environment: map[string]string{}, Argv: []string{"node", "playwright", "test"}},
		External: &jstestprovider.ExternalLifecycle{ReadyURL: "http://127.0.0.1:8000/", DeclaredAppIdentity: "synthetic-" + app, Ownership: "external",
			CleanupResponsibility: "external", ServerDescendants: "unknown", ReadyAtStart: true, ReadyAtPublish: true, RunnerDescendantsGone: true,
			InputsUnchanged: true, ConfigOverride: "corvint e2e prove"},
		AppBuildAtStart: jstestprovider.AppBuildIdentity{Digest: sum("app-" + app)}, AppBuildAtPublish: jstestprovider.AppBuildIdentity{Digest: sum("app-" + app)},
		Schedule: &jstestprovider.ExecutionSchedule{Workers: 1}}
	use := json.RawMessage(`{"browserName":"chromium","channel":"","headless":true,"launchOptions":{},"corvintBrowser":{"platform":"darwin","arch":"arm64","nodeVersion":"v24.11.1","browserType":"chromium","browserVersion":"Google Chrome for Testing 156.0.8078.4","channel":"","executableSource":"playwright-bundled","executableName":"chromium-headless-shell","executablePath":"/portable/cache/ms-playwright/chromium_headless_shell-1248/chrome-headless-shell-mac-arm64/chrome-headless-shell","executableSha256":"77d8278b1ac9d0687fad4e817ea83023ede09b1deef36e1864b49ba194ac9634","browserRevision":"1248","manifestBrowserVersion":"156.0.8078.4","headlessShellAvailable":true}}`)
	anchor := &jstestprovider.Anchor{File: placement, Line: testLine(sc)}
	for i, st := range states {
		state := jstestprovider.ExecutionState(st)
		kind, msg := "none", ""
		var at *jstestprovider.Anchor
		if state == jstestprovider.StateFailed {
			kind, msg = "assertion-or-test", message
			at = &jstestprovider.Anchor{File: placement, Line: sc.AssertionLine}
		}
		name := fmt.Sprintf("%s (repeat %d)", sc.Obligation, i)
		o := jstestprovider.TestOutcome{Name: name, FullName: name, State: state, FailureMessage: msg, Anchor: anchor,
			Project:        &jstestprovider.ProjectIdentity{Browser: "chromium", Device: "unknown", Use: use, ConfigDigest: sum(proveConfig)},
			Attempts:       []jstestprovider.Attempt{{State: state, FailureKind: kind}},
			AttemptDetails: []jstestprovider.AttemptDetail{{State: state, FailureMessage: msg, Anchor: anchor, FailureLocation: at}}}
		id, _ := json.Marshal(struct {
			Identity jstestprovider.Identity
			Project  *jstestprovider.ProjectIdentity
			Anchor   *jstestprovider.Anchor
			FullName string
		}{r.Identity, o.Project, o.Anchor, o.FullName})
		o.ID = sha256Hex(id) // the provider's qualified test identity (PWP)
		r.Tests = append(r.Tests, o)
		r.Schedule.Starts = append(r.Schedule.Starts, jstestprovider.ExecutionStart{FullName: name, File: placement, Line: anchor.Line, Project: proveProject, Retry: 0})
	}
	return r
}

// gateTB records the first way a live gate ends a test.
type gateTB struct {
	testing.TB
	ended string
}

func (g *gateTB) Helper() {}
func (g *gateTB) Fatalf(format string, args ...any) {
	if g.ended == "" {
		g.ended = "fatal: " + fmt.Sprintf(format, args...)
	}
}
func (g *gateTB) Skip(args ...any) {
	if g.ended == "" {
		g.ended = "skip: " + fmt.Sprint(args...)
	}
}

// CCE-V0-102: a missing live toolchain skips the live tests in the ordinary lane and fails them
// when CORVINT_CCE_LIVE=required.
func TestCCEV0102LiveRequiredFails(t *testing.T) {
	for _, tc := range []struct{ value, want string }{
		{"", "skip: node is not available"},
		{"1", "skip: node is not available"},
		{"required", "fatal: CORVINT_CCE_LIVE=required: node is not available"},
	} {
		t.Setenv(envLive, tc.value)
		g := &gateTB{TB: t}
		liveUnavailable(g, "node is not available")
		if g.ended != tc.want {
			t.Fatalf("%s=%q ended %q, want %q", envLive, tc.value, g.ended, tc.want)
		}
	}
}
