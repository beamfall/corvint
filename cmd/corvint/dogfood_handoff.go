package main

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/Beamfall/corvint/internal/contextindex"
	"github.com/Beamfall/corvint/internal/gokernel"
	"github.com/Beamfall/corvint/internal/localcompletion"
	"github.com/Beamfall/corvint/internal/repoenvelope"
)

// The handoff receipt (SESSION-V0-017..019) names what a receiving session
// must re-resolve. It is caller-owned data, never authority: the receiver
// recompiles the same packet and compares digests, it never trusts them.
const (
	handoffProfile    = "corvint-dogfood-handoff/0"
	handoffBudget     = 8000
	handoffLimit      = 10
	handoffMaxAnchors = 32
	handoffAnchorLen  = 512
	handoffRootLen    = 4096
	handoffPacketKind = "corvint-dogfood-prompt/0"
)

var (
	handoffDigestPattern = regexp.MustCompile(`^[0-9a-f]{64}$`)
	handoffObjectPattern = regexp.MustCompile(`^([0-9a-f]{40}|[0-9a-f]{64})?$`)
	handoffPlanPattern   = regexp.MustCompile(`^([0-9a-f]{64})?$`)
	handoffWorktrees     = map[string]bool{"clean": true, "mixed": true}
	handoffLifecycles    = map[string]bool{"inactive": true, "active": true, "satisfied": true, "cancelled": true}
)

// handoffProbe is the repository stability probe; tests replace it to reach
// the drift refusals, which otherwise need a concurrent writer.
var handoffProbe = gokernel.ProbeRepositoryContext

// Field order is the JSON name order, so emit's encoder output stays canonical.
type handoffReceipt struct {
	Anchors      []handoffAnchor   `json:"anchors"`
	Authority    string            `json:"authority"`
	Degradations []string          `json:"degradations"`
	Enrollment   handoffEnrollment `json:"enrollment"`
	Packet       handoffPacket     `json:"packet"`
	Profile      string            `json:"profile"`
	Revision     handoffRevision   `json:"revision"`
	Root         string            `json:"root"`
	SessionKey   string            `json:"sessionKey"`
}

type handoffAnchor struct {
	Anchor string `json:"anchor"`
	SHA256 string `json:"sha256"`
}

type handoffEnrollment struct {
	Base       string `json:"base"`
	Lifecycle  string `json:"lifecycle"`
	PlanDigest string `json:"planDigest"`
}

type handoffPacket struct {
	BudgetBytes int    `json:"budgetBytes"`
	Bytes       int    `json:"bytes"`
	Limit       int    `json:"limit"`
	Profile     string `json:"profile"`
	SHA256      string `json:"sha256"`
}

type handoffRevision struct {
	Commit           string `json:"commit"`
	DirtyPathsSHA256 string `json:"dirtyPathsSha256"`
	Tree             string `json:"tree"`
	WorktreeState    string `json:"worktreeState"`
}

// handoffDocument is the complete emitted stdout document a receiver reads back.
type handoffDocument struct {
	Claim    string         `json:"claim"`
	Envelope string         `json:"envelope"`
	Mode     string         `json:"mode"`
	Mutates  bool           `json:"mutates"`
	OK       bool           `json:"ok"`
	Profile  string         `json:"profile"`
	Receipt  handoffReceipt `json:"receipt"`
	Tool     string         `json:"tool"`
}

func runDogfoodHandoff(ctx context.Context, root, key string, flags map[string]string, stdout, stderr io.Writer) int {
	if flags["--receipt"] == "" {
		return emitDogfoodHandoff(ctx, root, key, flags["--anchors"], stdout, stderr)
	}
	return consumeDogfoodHandoff(ctx, root, key, flags["--receipt"], stdout, stderr)
}

func emitDogfoodHandoff(ctx context.Context, root, key, raw string, stdout, stderr io.Writer) int {
	anchors, err := handoffAnchors(raw)
	if err != nil {
		return emitLocalCompletionFailure(stderr, err.Error())
	}
	receipt, _, err := resolveHandoff(ctx, root, key, anchors)
	if err != nil {
		return emitLocalCompletionFailure(stderr, handoffErrorCode(err))
	}
	encoded, err := gokernel.CanonicalJSON(receipt)
	if err != nil {
		return emitLocalCompletionFailure(stderr, "output-failed")
	}
	framed, err := repoenvelope.Frame(string(encoded))
	if err != nil {
		return emitLocalCompletionFailure(stderr, repoenvelope.CollisionCode)
	}
	document := handoffDocument{Claim: "caller-owned-selected-workflow-only", Envelope: framed, Mode: "emit", OK: true, Profile: "corvint-local-completion/0", Receipt: receipt, Tool: "dogfood-handoff"}
	if err = emit(stdout, document); err != nil {
		return emitLocalCompletionFailure(stderr, "output-failed")
	}
	return 0
}

func consumeDogfoodHandoff(ctx context.Context, root, key, name string, stdout, stderr io.Writer) int {
	raw, err := localcompletion.ReadPlan(name)
	if err != nil {
		return emitLocalCompletionFailure(stderr, "handoff-receipt-unavailable")
	}
	received, err := decodeHandoff(raw)
	if err != nil {
		return emitLocalCompletionFailure(stderr, err.Error())
	}
	if received.SessionKey != key {
		return emitLocalCompletionFailure(stderr, "handoff-session-key-mismatch")
	}
	anchors := make([]string, 0, len(received.Anchors))
	for _, anchor := range received.Anchors {
		anchors = append(anchors, anchor.Anchor)
	}
	current, packet, err := resolveHandoff(ctx, root, key, anchors)
	if err != nil {
		return emitLocalCompletionFailure(stderr, handoffErrorCode(err))
	}
	drift := handoffDrift(received, current)
	handoff := map[string]any{"state": "reresolved", "drift": drift, "degradations": current.Degradations, "packetSha256": current.Packet.SHA256}
	payload := map[string]any{"ok": true, "profile": "corvint-local-completion/0", "tool": "dogfood-handoff", "mode": "consume", "mutates": false, "claim": "caller-owned-selected-workflow-only", "handoff": handoff}
	if len(drift) != 0 {
		// A drifted receipt reports the difference; the recompiled packet is withheld.
		handoff["state"] = "drifted"
	} else {
		// emit escapes non-ASCII, so the exact digested bytes travel as base64.
		payload["packetBase64"] = base64.StdEncoding.EncodeToString(packet)
	}
	if err = emit(stdout, payload); err != nil {
		return emitLocalCompletionFailure(stderr, "output-failed")
	}
	if len(drift) != 0 {
		return 1
	}
	return 0
}

// handoffAnchors keeps only whitespace-delimited anchor tokens, sorted and
// unique, so both sessions compile the packet from identical task text.
func handoffAnchors(raw string) ([]string, error) {
	fields := strings.Fields(raw)
	slices.Sort(fields)
	anchors := slices.Compact(fields)
	if !validHandoffAnchors(anchors) {
		return nil, errors.New("invalid-handoff-anchors")
	}
	return anchors, nil
}

func validHandoffAnchors(anchors []string) bool {
	if len(anchors) > handoffMaxAnchors || !slices.IsSorted(anchors) || len(slices.Compact(slices.Clone(anchors))) != len(anchors) {
		return false
	}
	for _, anchor := range anchors {
		invalid := func(r rune) bool { return unicode.IsSpace(r) || unicode.IsControl(r) }
		if anchor == "" || len(anchor) > handoffAnchorLen || !utf8.ValidString(anchor) || strings.ContainsFunc(anchor, invalid) {
			return false
		}
	}
	return true
}

// resolveHandoff compiles the dogfood prompt packet for the enrolled scope and
// the anchors, inside a repository stability bracket, and returns the exact
// digested packet bytes. It writes nothing.
func resolveHandoff(ctx context.Context, root, key string, anchors []string) (handoffReceipt, []byte, error) {
	before, err := handoffProbe(ctx, root)
	if err != nil {
		return handoffReceipt{}, nil, err
	}
	evaluation, err := localcompletion.Evaluate(ctx, root, key)
	if err != nil {
		return handoffReceipt{}, nil, err
	}
	index, hit, err := loadSnapshot(ctx, root)
	if err != nil || !hit {
		index, err = contextindex.BuildContext(ctx, root, "")
	}
	if err != nil {
		return handoffReceipt{}, nil, err
	}
	dirtyBytes, err := gokernel.CanonicalJSON(append([]string{}, index.DirtyPaths...))
	if err != nil {
		return handoffReceipt{}, nil, err
	}
	if index.CommitRevision != before.CommitRevision || index.Revision != before.TreeRevision || dogfoodSHA(dirtyBytes) != before.DirtyPathsSHA {
		return handoffReceipt{}, nil, errors.New("dogfood-handoff-context-drift")
	}
	pointers := make([]contextindex.PinnedIntentPointer, 0, len(evaluation.IntentPointers))
	for _, pointer := range evaluation.IntentPointers {
		pointers = append(pointers, contextindex.PinnedIntentPointer{Path: pointer.Path, Revision: pointer.Revision, BlobHash: pointer.BlobHash})
	}
	packet, err := contextindex.DogfoodPromptContext(ctx, index, strings.Join(anchors, " "), pointers, handoffLimit, handoffBudget)
	if err != nil {
		return handoffReceipt{}, nil, err
	}
	encoded, err := gokernel.CanonicalJSON(packet)
	if err != nil {
		return handoffReceipt{}, nil, err
	}
	encoded = append(encoded, '\n')
	digests, err := handoffAnchorDigests(ctx, index, anchors)
	if err != nil {
		return handoffReceipt{}, nil, err
	}
	after, err := handoffProbe(ctx, root)
	if err != nil {
		return handoffReceipt{}, nil, err
	}
	if before != after {
		return handoffReceipt{}, nil, errors.New("dogfood-handoff-repository-drift")
	}
	receipt := handoffReceipt{
		Anchors: digests, Authority: "none", Profile: handoffProfile, Root: handoffRootIdentity(root), SessionKey: key,
		Degradations: handoffDegradations(evaluation.Lifecycle, before.DirtyPathCount, packet),
		Enrollment:   handoffEnrollment{Base: evaluation.Base, Lifecycle: evaluation.Lifecycle, PlanDigest: evaluation.PlanDigest},
		Packet:       handoffPacket{BudgetBytes: handoffBudget, Bytes: len(encoded), Limit: handoffLimit, Profile: handoffPacketKind, SHA256: dogfoodSHA(encoded)},
		Revision:     handoffRevision{Commit: before.CommitRevision, DirtyPathsSHA256: before.DirtyPathsSHA, Tree: before.TreeRevision, WorktreeState: before.WorktreeState},
	}
	return receipt, encoded, nil
}

// handoffRootIdentity names a root by its symlink-resolved Git toplevel, so an
// alias such as /tmp for /private/tmp or a subdirectory is not root drift.
func handoffRootIdentity(root string) string {
	resolved, err := filepath.EvalSymlinks(root)
	if err != nil {
		return root
	}
	for directory := resolved; directory != filepath.Dir(directory); directory = filepath.Dir(directory) {
		if _, err := os.Lstat(filepath.Join(directory, ".git")); err == nil {
			return directory
		}
	}
	return resolved
}

// handoffAnchorDigests binds each anchor to its own resolution and task
// evidence, so revision drift names exactly the anchors whose evidence moved.
func handoffAnchorDigests(ctx context.Context, index *contextindex.Index, anchors []string) ([]handoffAnchor, error) {
	digests := make([]handoffAnchor, 0, len(anchors))
	for _, anchor := range anchors {
		packet, err := contextindex.DogfoodPromptContext(ctx, index, anchor, nil, handoffLimit, handoffBudget)
		if err != nil {
			return nil, err
		}
		encoded, err := gokernel.CanonicalJSON(map[string]any{"resolution": packet["resolution"], "task_evidence": packet["task_evidence"]})
		if err != nil {
			return nil, err
		}
		digests = append(digests, handoffAnchor{Anchor: anchor, SHA256: dogfoodSHA(encoded)})
	}
	return digests, nil
}

func handoffDegradations(lifecycle string, dirty int, packet map[string]any) []string {
	degradations := []string{"frontier-authority-unavailable"}
	if lifecycle != "active" && lifecycle != "satisfied" {
		degradations = append(degradations, "enrollment-"+lifecycle)
	}
	if dirty > 0 {
		degradations = append(degradations, "uncommitted-work")
	}
	resolution, _ := packet["resolution"].(map[string]any)
	reason, ok := resolution["reason"].(string)
	if !ok {
		reason = "resolution-unavailable"
	}
	if reason != "none" {
		degradations = append(degradations, reason)
	}
	slices.Sort(degradations)
	return degradations
}

// handoffDrift lists every receipt member the receiver could not reproduce, in
// a fixed order: root, revision, enrollment, each anchor, then the packet.
// Every receipt value it echoes has passed validHandoffDocument.
func handoffDrift(received, current handoffReceipt) []map[string]any {
	drift := []map[string]any{}
	if handoffRootIdentity(received.Root) != current.Root {
		drift = append(drift, map[string]any{"field": "root", "receipt": received.Root, "current": current.Root})
	}
	if received.Revision != current.Revision {
		drift = append(drift, map[string]any{"field": "revision", "receipt": received.Revision, "current": current.Revision})
	}
	if received.Enrollment != current.Enrollment {
		drift = append(drift, map[string]any{"field": "enrollment", "receipt": received.Enrollment, "current": current.Enrollment})
	}
	for index, anchor := range received.Anchors {
		if anchor.SHA256 != current.Anchors[index].SHA256 {
			drift = append(drift, map[string]any{"field": "anchor", "anchor": anchor.Anchor, "receipt": anchor.SHA256, "current": current.Anchors[index].SHA256})
		}
	}
	if received.Packet != current.Packet {
		drift = append(drift, map[string]any{"field": "packet", "receipt": received.Packet, "current": current.Packet})
	}
	return drift
}

// decodeHandoff accepts exactly the bytes emit produced for one handoff
// document. Re-encoding never yields a duplicate, case-folded, unknown or
// reordered member or trailing data, so the byte comparison is stricter than
// the plan parser; every field must then have its emitted shape.
func decodeHandoff(raw []byte) (handoffReceipt, error) {
	invalid := errors.New("invalid-handoff-receipt")
	var document handoffDocument
	if json.Unmarshal(raw, &document) != nil {
		return handoffReceipt{}, invalid
	}
	var canonical bytes.Buffer
	if emit(&canonical, document) != nil || !bytes.Equal(canonical.Bytes(), raw) {
		return handoffReceipt{}, invalid
	}
	if !validHandoffDocument(document) {
		return handoffReceipt{}, invalid
	}
	return document.Receipt, nil
}

func validHandoffDocument(document handoffDocument) bool {
	receipt := document.Receipt
	checks := []bool{
		document.Tool == "dogfood-handoff", document.Mode == "emit", document.OK, !document.Mutates,
		receipt.Profile == handoffProfile, receipt.Authority == "none",
		handoffDigestPattern.MatchString(receipt.SessionKey), validHandoffRoot(receipt.Root),
		handoffObjectPattern.MatchString(receipt.Revision.Commit), handoffObjectPattern.MatchString(receipt.Revision.Tree),
		handoffDigestPattern.MatchString(receipt.Revision.DirtyPathsSHA256), handoffWorktrees[receipt.Revision.WorktreeState],
		handoffObjectPattern.MatchString(receipt.Enrollment.Base), handoffPlanPattern.MatchString(receipt.Enrollment.PlanDigest),
		handoffLifecycles[receipt.Enrollment.Lifecycle],
		receipt.Packet.Profile == handoffPacketKind, receipt.Packet.BudgetBytes == handoffBudget, receipt.Packet.Limit == handoffLimit,
		receipt.Packet.Bytes >= 1, receipt.Packet.Bytes <= handoffBudget, handoffDigestPattern.MatchString(receipt.Packet.SHA256),
		validHandoffAnchorDigests(receipt.Anchors),
	}
	return !slices.Contains(checks, false)
}

func validHandoffRoot(root string) bool {
	return len(root) <= handoffRootLen && filepath.IsAbs(root) && filepath.Clean(root) == root && utf8.ValidString(root) && !strings.ContainsFunc(root, unicode.IsControl)
}

func validHandoffAnchorDigests(anchors []handoffAnchor) bool {
	names := make([]string, 0, len(anchors))
	for _, anchor := range anchors {
		if !handoffDigestPattern.MatchString(anchor.SHA256) {
			return false
		}
		names = append(names, anchor.Anchor)
	}
	return validHandoffAnchors(names)
}

// handoffErrorCode keeps only fixed codes on stderr; the failure writer maps
// anything else to its generic code.
func handoffErrorCode(err error) string {
	var indexErr *contextindex.Error
	if errors.As(err, &indexErr) {
		return indexErr.Code
	}
	return err.Error()
}
