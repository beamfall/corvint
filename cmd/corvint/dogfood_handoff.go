package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
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
)

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
	handoff := map[string]any{"state": "reresolved", "drift": drift, "packetSha256": current.Packet.SHA256}
	payload := map[string]any{"ok": true, "profile": "corvint-local-completion/0", "tool": "dogfood-handoff", "mode": "consume", "mutates": false, "claim": "caller-owned-selected-workflow-only", "handoff": handoff}
	if len(drift) != 0 {
		// A drifted receipt reports the difference; the recompiled packet is withheld.
		handoff["state"] = "drifted"
	} else {
		payload["packet"] = packet
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
// the anchors, inside a repository stability bracket. It writes nothing.
func resolveHandoff(ctx context.Context, root, key string, anchors []string) (handoffReceipt, map[string]any, error) {
	before, err := gokernel.ProbeRepositoryContext(ctx, root)
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
	after, err := gokernel.ProbeRepositoryContext(ctx, root)
	if err != nil {
		return handoffReceipt{}, nil, err
	}
	if before != after {
		return handoffReceipt{}, nil, errors.New("dogfood-handoff-repository-drift")
	}
	receipt := handoffReceipt{
		Anchors: digests, Authority: "none", Profile: handoffProfile, Root: root, SessionKey: key,
		Degradations: handoffDegradations(evaluation.Lifecycle, before.DirtyPathCount, packet),
		Enrollment:   handoffEnrollment{Base: evaluation.Base, Lifecycle: evaluation.Lifecycle, PlanDigest: evaluation.PlanDigest},
		Packet:       handoffPacket{BudgetBytes: handoffBudget, Bytes: len(encoded), Limit: handoffLimit, Profile: "corvint-dogfood-prompt/0", SHA256: dogfoodSHA(encoded)},
		Revision:     handoffRevision{Commit: before.CommitRevision, DirtyPathsSHA256: before.DirtyPathsSHA, Tree: before.TreeRevision, WorktreeState: before.WorktreeState},
	}
	return receipt, packet, nil
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
	if reason, _ := packet["resolution"].(map[string]any)["reason"].(string); reason != "none" {
		degradations = append(degradations, reason)
	}
	slices.Sort(degradations)
	return degradations
}

// handoffDrift lists every receipt member the receiver could not reproduce, in
// a fixed order: root, revision, enrollment, each anchor, then the packet.
func handoffDrift(received, current handoffReceipt) []map[string]any {
	drift := []map[string]any{}
	if received.Root != current.Root {
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

// decodeHandoff accepts exactly one emitted handoff document with no unknown
// members, then validates the receipt shape. Digests are identity, not trust.
func decodeHandoff(raw []byte) (handoffReceipt, error) {
	invalid := errors.New("invalid-handoff-receipt")
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	var document handoffDocument
	if decoder.Decode(&document) != nil || decoder.Decode(&struct{}{}) != io.EOF {
		return handoffReceipt{}, invalid
	}
	receipt := document.Receipt
	if document.Tool != "dogfood-handoff" || document.Mode != "emit" || !document.OK || document.Mutates {
		return handoffReceipt{}, invalid
	}
	if receipt.Profile != handoffProfile || receipt.Authority != "none" || !handoffDigestPattern.MatchString(receipt.SessionKey) {
		return handoffReceipt{}, invalid
	}
	if receipt.Packet.BudgetBytes != handoffBudget || receipt.Packet.Limit != handoffLimit || !handoffDigestPattern.MatchString(receipt.Packet.SHA256) {
		return handoffReceipt{}, invalid
	}
	anchors := make([]string, 0, len(receipt.Anchors))
	for _, anchor := range receipt.Anchors {
		if !handoffDigestPattern.MatchString(anchor.SHA256) {
			return handoffReceipt{}, invalid
		}
		anchors = append(anchors, anchor.Anchor)
	}
	if !validHandoffAnchors(anchors) {
		return handoffReceipt{}, invalid
	}
	return receipt, nil
}

var handoffDigestPattern = regexp.MustCompile(`^[0-9a-f]{64}$`)

// handoffErrorCode keeps only fixed codes on stderr; the failure writer maps
// anything else to its generic code.
func handoffErrorCode(err error) string {
	var indexErr *contextindex.Error
	if errors.As(err, &indexErr) {
		return indexErr.Code
	}
	return err.Error()
}
