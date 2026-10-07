package appmap

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"regexp"
	"sort"
	"strings"
	"unicode"

	"github.com/Beamfall/corvint/internal/gokernel"
	"github.com/Beamfall/corvint/internal/jstestprovider"
	"github.com/Beamfall/corvint/internal/testvaliditydoc"
)

// Run-verified navigation (docs/specs/run-verified-navigation-v0.md, RVN-V0): passing and failing
// Playwright receipt outcomes bound to map steps, evaluated per projection call. Nothing is stored.

// Step verification statuses (RVN-V0-005). A step printed without a verification reads Unverified.
const (
	Verified         = "VERIFIED"
	UnverifiedAtHead = "UNVERIFIED_AT_HEAD"
	Contradicted     = "CONTRADICTED"
	Unverified       = "unverified"
)

// Verification input bounds (RVN-V0-001, RVN-V0-008).
const (
	MaxReceipts     = 16
	MaxBinds        = 64
	maxTestKeyBytes = 256
)

var commitOID = regexp.MustCompile(`^(?:[0-9a-f]{40}|[0-9a-f]{64})$`)

// ReceiptRef names the receipt outcome a verification cites: the receipt file's SHA-256, the
// outcome's test ID and project, and the receipt profile.
type ReceiptRef struct {
	SHA256  string `json:"sha256"`
	TestKey string `json:"test_key"`
	Project string `json:"project"`
	Profile string `json:"profile"`
}

// StepVerification is one step's run-verification status (RVN-V0-005, RVN-V0-006). Revision is
// the application revision the cited receipt ran against; Authority is always AuthorityLearned.
type StepVerification struct {
	Status   string      `json:"status"`
	Revision string      `json:"revision,omitempty"`
	Receipt  *ReceiptRef `json:"receipt,omitempty"`
	// AppIdentity is `attested` (PWP /1, or /2 with attestation) or `declared`.
	AppIdentity string `json:"app_identity,omitempty"`
	// SelectorEvidence is `run-verified` or `run-contradicted` for a step with a selector; the
	// selector's static strength never changes (RVN-V0-007).
	SelectorEvidence string `json:"selector_evidence,omitempty"`
	Reason           string `json:"reason,omitempty"`
	Outcomes         int    `json:"outcomes"`
	Authority        string `json:"authority"`
}

type receiptInput struct {
	sha string
	r   jstestprovider.Receipt
}

// Verification is the decoded run evidence for one projection call: PWP receipts and agent-declared
// step bindings. Build it with LoadVerification; read it through Overlay or VerifySteps.
type Verification struct {
	receipts []receiptInput
	binds    map[string][]string
}

func verifyError(code, format string, args ...any) error {
	return &gokernel.Error{Code: code, Message: fmt.Sprintf(format, args...)}
}

// readReceipt reads one receipt file: a regular file, opened without following a symlink or
// blocking on a FIFO, unchanged in identity, size and modification time while read, within the
// PWP bound.
func readReceipt(filename string) ([]byte, error) {
	bad := verifyError("appmap-verify-invalid-receipt", "--receipt must name a regular file of at most %d bytes, unchanged while read", testvaliditydoc.MaxInputBytes)
	before, err := os.Lstat(filename)
	if err != nil || !before.Mode().IsRegular() {
		return nil, bad
	}
	f, err := openInput(filename)
	if err != nil {
		return nil, bad
	}
	defer f.Close()
	opened, err := f.Stat()
	if err != nil || !opened.Mode().IsRegular() || !os.SameFile(before, opened) || !unchanged(before, opened) {
		return nil, bad
	}
	raw, err := io.ReadAll(io.LimitReader(f, testvaliditydoc.MaxInputBytes+1))
	if err != nil || len(raw) > testvaliditydoc.MaxInputBytes {
		return nil, bad
	}
	after, err := f.Stat()
	if err != nil || !unchanged(opened, after) || int64(len(raw)) != after.Size() {
		return nil, bad
	}
	return raw, nil
}

// unchanged reports whether two observations of one file agree in size and modification time.
func unchanged(a, b os.FileInfo) bool {
	return a.Size() == b.Size() && a.ModTime().Equal(b.ModTime())
}

func validTestKey(k string) bool {
	if k == "" || len(k) > maxTestKeyBytes {
		return false
	}
	for _, r := range k {
		if unicode.IsControl(r) || unicode.IsSpace(r) {
			return false
		}
	}
	return true
}

// LoadVerification decodes receipt files through the PWP provider document contract and checks
// each `STEP_ID=TEST_KEY` binding against the map and the receipts (RVN-V0-001, RVN-V0-002).
func LoadVerification(m *Map, receiptFiles, binds []string) (*Verification, error) {
	return LoadPlanVerification([]*Map{m}, receiptFiles, binds)
}

// LoadPlanVerification is LoadVerification over the maps of one plan (AMSP-V0-007): a binding
// must name a step of at least one map. Build one Overlay per map from the result; a step ID
// that two maps share is unattributable and the planner reads it unverified.
func LoadPlanVerification(maps []*Map, receiptFiles, binds []string) (*Verification, error) {
	if len(receiptFiles) == 0 {
		if len(binds) > 0 {
			return nil, verifyError("appmap-invalid-query", "--bind requires at least one --receipt")
		}
		return nil, nil
	}
	if len(receiptFiles) > MaxReceipts || len(binds) > MaxBinds {
		return nil, verifyError("appmap-invalid-query", "at most %d --receipt and %d --bind", MaxReceipts, MaxBinds)
	}
	v := &Verification{binds: map[string][]string{}}
	seen := map[string]bool{}
	ids := map[string]bool{}
	for _, name := range receiptFiles {
		raw, err := readReceipt(name)
		if err != nil {
			return nil, err
		}
		in, err := testvaliditydoc.Decode(raw)
		if err != nil {
			return nil, verifyError("appmap-verify-invalid-receipt", "--receipt is not a canonical Playwright external provider document")
		}
		doc := testvaliditydoc.Project(in)
		if doc.Playwright == nil {
			return nil, verifyError("appmap-verify-invalid-receipt", "--receipt is not a Playwright external (PWP) receipt")
		}
		sum := digest(raw)
		if seen[sum] {
			continue
		}
		seen[sum] = true
		v.receipts = append(v.receipts, receiptInput{sha: sum, r: *doc.Playwright})
		for _, t := range doc.Playwright.Tests {
			ids[t.ID] = true
		}
	}
	sort.Slice(v.receipts, func(i, j int) bool { return v.receipts[i].sha < v.receipts[j].sha })
	for _, b := range binds {
		step, key, ok := strings.Cut(b, "=")
		if !ok || !strings.HasPrefix(step, "step:") || !validTestKey(key) {
			return nil, verifyError("appmap-invalid-query", "--bind must be STEP_ID=TEST_KEY")
		}
		found := false
		for _, m := range maps {
			if _, st := m.step(step); st != nil {
				found = true
				break
			}
		}
		if !found {
			return nil, verifyError("appmap-verify-unknown-step", "--bind names no step of any --map: %s", step)
		}
		if !ids[key] {
			return nil, verifyError("appmap-verify-test-absent", "--bind test key is absent from every --receipt: %s", key)
		}
		v.binds[step] = append(v.binds[step], key)
	}
	return v, nil
}

// stepAnchors are the application source anchors a step's verification depends on: the router
// states of its screen and every ancestor (RVN-V0-004). The flow intent and the test source are
// test-side: a changed test file changes the receipt's test ID, so its old outcome stops binding.
func (m *Map) stepAnchors(st Step) []Anchor {
	return m.lineage(m.screen(st.Screen))
}

// revState is the freshness of one step's app-source anchors at one revision.
type revState struct{ evaluated, state string }

// verifier evaluates step verifications for one call. head is the evaluated revision (Options
// Revision, default HEAD) resolved once, or FreshUnknown.
type verifier struct {
	ctx  context.Context
	m    *Map
	o    Options
	v    *Verification
	head string
	revs map[string]revState
}

func newVerifier(ctx context.Context, m *Map, v *Verification, o Options) *verifier {
	return &verifier{ctx: ctx, m: m, o: o, v: v, head: newFreshness(ctx, o, nil).evaluated, revs: map[string]revState{}}
}

// anchorsAt returns the resolved revision and folded freshness of one step's app-source anchors
// at rev. It reads only those anchors, so an unrelated anchor of the projection cannot change a
// step's status (RVN-V0-006), and caches per revision and anchor set: steps of one screen share a
// lineage, so a call reads each distinct (revision, lineage) once (RVN-V0-008).
func (e *verifier) anchorsAt(rev string, anchors []Anchor) revState {
	key := rev
	for _, a := range anchors {
		key += fmt.Sprintf("\x00%s\x00%d\x00%d\x00%s\x00%s", a.Path, a.Start, a.End, a.Blob, a.SpanSHA256)
	}
	if r, ok := e.revs[key]; ok {
		return r
	}
	f := newFreshness(e.ctx, Options{Root: e.o.Root, Revision: rev}, anchors)
	r := revState{evaluated: f.evaluated, state: Fresh}
	for _, a := range anchors {
		switch f.of(a) {
		case Stale:
			r.state = Stale
		case FreshUnknown:
			if r.state != Stale {
				r.state = FreshUnknown
			}
		}
	}
	e.revs[key] = r
	return r
}

type evidence struct {
	ref      ReceiptRef
	revision string
	identity string
}

func (e evidence) less(o evidence) bool {
	if e.revision != o.revision {
		return e.revision < o.revision
	}
	if e.ref.SHA256 != o.ref.SHA256 {
		return e.ref.SHA256 < o.ref.SHA256
	}
	if e.ref.TestKey != o.ref.TestKey {
		return e.ref.TestKey < o.ref.TestKey
	}
	return e.ref.Project < o.ref.Project
}

// Unverified reasons, in the order the first applicable one is reported (RVN-V0-005).
var unverifiedOrder = []string{"freshness-unknown", "unplaced-failure", "app-revision-unresolved", "anchor-differs-at-app-revision", "inconclusive-outcome"}

// verify evaluates one step against the call's receipts (RVN-V0-003..RVN-V0-005).
func (e *verifier) verify(st Step) *StepVerification {
	v := e.v
	out := &StepVerification{Status: Unverified, Authority: AuthorityLearned}
	if st.Status != StatusResolved || e.m.screen(st.Screen) == nil {
		out.Reason = "step-unresolved"
		return out
	}
	keys := map[string]bool{}
	for _, k := range st.Tests {
		keys[k] = true
	}
	for _, k := range v.binds[st.ID] {
		keys[k] = true
	}
	if len(keys) == 0 {
		out.Reason = "no-binding"
		return out
	}
	anchors := e.m.stepAnchors(st)
	// The evaluated revision is resolved once per call; the step's anchors are then read at that
	// exact commit.
	head := FreshUnknown
	if e.head != FreshUnknown {
		head = e.anchorsAt(e.head, anchors).state
	}
	reasons := map[string]bool{}
	var pass, fail, placed *evidence
	pick := func(cur *evidence, ev evidence) *evidence {
		if cur == nil || ev.less(*cur) {
			return &ev
		}
		return cur
	}
	for _, in := range v.receipts {
		for _, t := range in.r.Tests {
			if !keys[t.ID] {
				continue
			}
			out.Outcomes++
			passed, failed := false, false
			if jstestprovider.QualifiedReceiptBindingReady(in.r, t) {
				passed = t.State == jstestprovider.StatePassed
				failed = t.State == jstestprovider.StateFailed || t.State == jstestprovider.StateTimedOut
			}
			if !passed && !failed {
				reasons["inconclusive-outcome"] = true
				continue
			}
			rev, _ := jstestprovider.QualifiedApplicationRevision(in.r, t)
			project := ""
			if t.Project != nil {
				project = t.Project.Name
			}
			identity := "declared"
			if in.r.Profile == jstestprovider.AttestedExternalProfile || (in.r.Profile == jstestprovider.SensitiveExternalProfile && in.r.ApplicationAttestation != nil) {
				identity = "attested"
			}
			ev := evidence{ref: ReceiptRef{SHA256: in.sha, TestKey: t.ID, Project: project, Profile: in.r.Profile}, revision: rev, identity: identity}
			resolved := false
			at := FreshUnknown
			if commitOID.MatchString(rev) {
				r := e.anchorsAt(rev, anchors)
				resolved = r.evaluated == rev
				if resolved {
					at = r.state
				}
			}
			if !resolved {
				reasons["app-revision-unresolved"] = true
			}
			switch at {
			case Fresh:
				placed = pick(placed, ev)
				if passed {
					pass = pick(pass, ev)
				} else {
					fail = pick(fail, ev)
				}
			case Stale:
				reasons["anchor-differs-at-app-revision"] = true
			default:
				if resolved {
					reasons["freshness-unknown"] = true
				}
			}
			if failed && at == FreshUnknown {
				reasons["unplaced-failure"] = true
			}
		}
	}
	cite := func(status string, ev *evidence) *StepVerification {
		out.Status, out.Revision, out.AppIdentity = status, ev.revision, ev.identity
		ref := ev.ref
		out.Receipt = &ref
		return out
	}
	switch {
	case out.Outcomes == 0:
		out.Reason = "no-receipt-outcome"
		return out
	case head == FreshUnknown && placed != nil:
		out.Reason = "freshness-unknown"
		return out
	case head == Stale && placed != nil:
		return cite(UnverifiedAtHead, placed)
	case head == Fresh && fail != nil:
		cite(Contradicted, fail)
		if st.Selector != nil {
			out.SelectorEvidence = "run-contradicted"
		}
		return out
	case head == Fresh && pass != nil && !reasons["unplaced-failure"]:
		cite(Verified, pass)
		if st.Selector != nil {
			out.SelectorEvidence = "run-verified"
		}
		return out
	}
	for _, r := range unverifiedOrder {
		if reasons[r] {
			out.Reason = r
			return out
		}
	}
	out.Reason = "inconclusive-outcome"
	return out
}

// VerifySteps returns the verification of every step of one flow, keyed by step ID, for a caller
// such as the V1-0959 planner that needs statuses without a rendered projection. It returns nil
// when v is nil; a step absent from the result reads Unverified.
func VerifySteps(ctx context.Context, m *Map, flow string, v *Verification, o Options) map[string]StepVerification {
	fl := m.flow(strings.TrimSpace(flow))
	if fl == nil || v == nil {
		return nil
	}
	e := newVerifier(ctx, m, v, o)
	out := map[string]StepVerification{}
	for _, st := range fl.Steps {
		out[st.ID], _ = boundedVerification(*e.verify(st))
	}
	return out
}

// VerificationSource is the overlay source of run-verification facts (RVN-V0-006).
const VerificationSource = "run-verification"

// Overlay returns v as an AMAP-V0-014 overlay: one fact per selected `step:` element that has a
// binding, Kind the status, Revision the cited application revision and Text the compact
// StepVerification JSON. o must carry the projection's Root and Revision. A nil v yields no facts.
func (v *Verification) Overlay(m *Map, o Options) Overlay {
	return verificationOverlay{v: v, m: m, o: o}
}

type verificationOverlay struct {
	v *Verification
	m *Map
	o Options
}

func (vo verificationOverlay) Facts(ctx context.Context, ids []string) ([]Fact, error) {
	if vo.v == nil {
		return nil, nil
	}
	var e *verifier
	facts := []Fact{}
	for _, id := range ids {
		if !strings.HasPrefix(id, "step:") {
			continue
		}
		_, st := vo.m.step(id)
		if st == nil {
			continue
		}
		if e == nil {
			e = newVerifier(ctx, vo.m, vo.v, vo.o)
		}
		// A step with no binding has no run evidence; it reads Unverified by absence, which keeps
		// the learned section within the projection budget for the steps that do.
		if sv := e.verify(*st); sv.Reason != "no-binding" {
			facts = append(facts, verificationFact(id, *sv))
		}
	}
	return facts, nil
}

// boundedVerification returns sv and its compact JSON, or, when that JSON exceeds the overlay
// fact bound, an unverified `evidence-too-large` status (fail closed), so VerifySteps and the
// overlay always agree (RVN-V0-006).
func boundedVerification(sv StepVerification) (StepVerification, []byte) {
	raw, err := json.Marshal(sv)
	if err != nil || len(raw) > maxFactBytes {
		sv = StepVerification{Status: Unverified, Reason: "evidence-too-large", Outcomes: sv.Outcomes, Authority: AuthorityLearned}
		raw, _ = json.Marshal(sv)
	}
	return sv, raw
}

// verificationFact renders one status as a fact. A text over the overlay fact bound degrades to
// an unverified fact rather than being dropped silently (fail closed).
func verificationFact(id string, sv StepVerification) Fact {
	sv, raw := boundedVerification(sv)
	return Fact{ElementID: id, Source: VerificationSource, Kind: sv.Status, Revision: sv.Revision, Text: string(raw), Authority: AuthorityLearned}
}
