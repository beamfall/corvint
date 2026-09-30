package main

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"path"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/Beamfall/corvint/internal/cem/wire"
	"github.com/Beamfall/corvint/internal/gokernel"
	"github.com/Beamfall/corvint/internal/localcompletion"
	"github.com/Beamfall/corvint/internal/repoenvelope"
)

const taskHandoffProfile = "corvint-task-handoff/0"

// These are caller reports, never accepted intent, commands or lease authority.
type handoffTaskState struct {
	Decisions   []string `json:"decisions"`
	Feedback    []string `json:"feedback"`
	NextActions []string `json:"nextActions"`
	Scope       []string `json:"scope"`
	TaskID      string   `json:"taskId"`
	Unknowns    []string `json:"unknowns"`
}
type handoffBundle struct {
	Authority string           `json:"authority"`
	Receipt   handoffReceipt   `json:"receipt"`
	State     handoffTaskState `json:"state"`
}
type handoffBundleDocument struct {
	Bundle   handoffBundle `json:"bundle"`
	Envelope string        `json:"envelope"`
	Mutates  bool          `json:"mutates"`
	Profile  string        `json:"profile"`
	SHA256   string        `json:"sha256"`
}

func validHandoffTaskState(s handoffTaskState) bool {
	validText := func(s string) bool {
		return len(s) > 0 && len(s) <= 2048 && utf8.ValidString(s) && !strings.ContainsFunc(s, func(r rune) bool { return unicode.IsControl(r) && r != '\n' && r != '\t' })
	}
	if !validText(s.TaskID) || len(s.TaskID) > 256 || len(s.Scope) == 0 || len(s.NextActions) == 0 {
		return false
	}
	for _, list := range [][]string{s.Decisions, s.Feedback, s.NextActions, s.Scope, s.Unknowns} {
		if list == nil || len(list) > 32 {
			return false
		}
		for _, item := range list {
			if !validText(item) {
				return false
			}
		}
	}
	for _, p := range s.Scope {
		if path.IsAbs(p) || path.Clean(p) != p || p == ".." || strings.HasPrefix(p, "../") || strings.ContainsAny(p, "\\\n\t") {
			return false
		}
	}
	return true
}

func decodeHandoffTaskState(raw []byte) (handoffTaskState, error) {
	var state handoffTaskState
	invalid := errors.New("invalid-handoff-task-state")
	if _, err := wire.Parse(raw); err != nil {
		return state, invalid
	}
	var fields map[string]json.RawMessage
	if json.Unmarshal(raw, &fields) != nil || len(fields) != 6 {
		return state, invalid
	}
	for _, key := range []string{"decisions", "feedback", "nextActions", "scope", "taskId", "unknowns"} {
		if _, ok := fields[key]; !ok {
			return state, invalid
		}
	}
	if json.Unmarshal(raw, &state) != nil || !validHandoffTaskState(state) {
		return state, invalid
	}
	return state, nil
}

func frameHandoffBundle(b handoffBundle) (handoffBundleDocument, error) {
	raw, err := gokernel.CanonicalJSON(b)
	if err != nil {
		return handoffBundleDocument{}, err
	}
	envelope, err := repoenvelope.Frame(string(raw))
	return handoffBundleDocument{Bundle: b, Envelope: envelope, Profile: taskHandoffProfile, SHA256: dogfoodSHA(raw)}, err
}

func runDogfoodHandoffBundle(ctx context.Context, root, key string, flags map[string]string, stdout, stderr io.Writer) int {
	fail := func(code string) int { return emitLocalCompletionFailure(stderr, code) }
	if flags["--task-state"] != "" {
		raw, err := localcompletion.ReadPlan(flags["--task-state"])
		if err != nil {
			return fail("handoff-task-state-unavailable")
		}
		state, err := decodeHandoffTaskState(raw)
		if err != nil {
			return fail(err.Error())
		}
		anchors, err := handoffAnchors(flags["--anchors"])
		if err != nil {
			return fail(err.Error())
		}
		receipt, _, err := resolveHandoff(ctx, root, key, anchors)
		if err != nil {
			return fail(handoffErrorCode(err))
		}
		if receipt.Revision.WorktreeState != "clean" || receipt.Revision.Commit == "" {
			return fail("handoff-bundle-clean-candidate-required")
		}
		document, err := frameHandoffBundle(handoffBundle{Authority: "none", Receipt: receipt, State: state})
		if err != nil {
			return fail(repoenvelope.CollisionCode)
		}
		var out bytes.Buffer
		if emit(&out, document) != nil {
			return fail("output-failed")
		}
		// The doubled framing and ASCII escaping count toward the consumer's byte cap.
		if out.Len() > localcompletion.MaxPlanBytes {
			return fail("handoff-bundle-too-large")
		}
		if _, err = stdout.Write(out.Bytes()); err != nil {
			return fail("output-failed")
		}
		return 0
	}
	raw, err := localcompletion.ReadPlan(flags["--bundle"])
	if err != nil {
		return fail("handoff-bundle-unavailable")
	}
	var document handoffBundleDocument
	if json.Unmarshal(raw, &document) != nil {
		return fail("invalid-handoff-bundle")
	}
	var canonical bytes.Buffer
	if emit(&canonical, document) != nil || !bytes.Equal(raw, canonical.Bytes()) {
		return fail("invalid-handoff-bundle")
	}
	expected, err := frameHandoffBundle(document.Bundle)
	b := document.Bundle
	receiptDoc := handoffDocument{Claim: "caller-owned-selected-workflow-only", Mode: "emit", OK: true, Profile: "corvint-local-completion/0", Receipt: b.Receipt, Tool: "dogfood-handoff"}
	if err != nil || document.Profile != taskHandoffProfile || document.Mutates || document.SHA256 != expected.SHA256 || document.Envelope != expected.Envelope || b.Authority != "none" || !validHandoffDocument(receiptDoc) || !validHandoffTaskState(b.State) || b.Receipt.Revision.WorktreeState != "clean" || b.Receipt.Revision.Commit == "" {
		return fail("invalid-handoff-bundle")
	}
	if key != b.Receipt.SessionKey {
		return fail("handoff-session-key-mismatch")
	}
	anchors := make([]string, 0, len(b.Receipt.Anchors))
	for _, a := range b.Receipt.Anchors {
		anchors = append(anchors, a.Anchor)
	}
	current, packet, err := resolveHandoff(ctx, root, key, anchors)
	if err != nil {
		return fail(handoffErrorCode(err))
	}
	drift := handoffDrift(b.Receipt, current)
	result := map[string]any{"profile": taskHandoffProfile, "authority": "none", "mutates": false, "state": "drifted", "drift": drift, "degradations": current.Degradations}
	if len(drift) == 0 {
		result["state"] = "reresolved"
		result["envelope"] = document.Envelope
		result["taskState"] = b.State
		result["taskStateAuthority"] = "caller-reported-unverified"
		result["packetBase64"] = base64.StdEncoding.EncodeToString(packet)
	}
	if emit(stdout, result) != nil {
		return fail("output-failed")
	}
	if len(drift) > 0 {
		return 1
	}
	return 0
}
