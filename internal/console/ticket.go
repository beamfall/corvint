package console

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"strings"
	"time"
)

// Detail is one ticket as the detail pane shows it: the tool's own view, its
// blocker closure, and the attribution of both reads.
type Detail struct {
	TicketID      string
	Card          Card
	Record        map[string]any
	Blockers      *Envelope
	Show          Source
	BlockerSource Source
	Envelope      *Envelope
	Err           string
	Untrusted     []string
	Requirements  []string
}

// ReadDetail reads one ticket and its blockers. Both reads are attributed
// separately, because a value derived from both can be no stronger than the
// weaker of them (LAC-V0-010).
func (t *Taskman) ReadDetail(ctx context.Context, id string) *Detail {
	envelope, source := t.Run(ctx, "ticket", "show", id)
	detail := &Detail{TicketID: id, Show: source, Envelope: envelope}
	if envelope == nil {
		detail.Err = source.Err
		return detail
	}
	detail.Untrusted = envelope.Untrusted
	if envelope.Refused() {
		return detail
	}
	if len(envelope.Items) > 0 {
		item := envelope.Items[0]
		detail.Card = cardOf(item)
		if record, ok := item["record"].(map[string]any); ok {
			detail.Record = record
			detail.Requirements = stringsOf(record["requirementRefs"])
		}
	}
	detail.Blockers, detail.BlockerSource = t.Run(ctx, "ticket", "blockers", id)
	return detail
}

// Control is one mutation the board offers for a ticket. Enabled is decided
// by the owning tool's own capability report, never by the console; a
// disabled control still renders, carrying the tool's reason (LAC-V0-013).
type Control struct {
	Verb            string
	Label           string
	Enabled         bool
	Reason          string
	PayloadTemplate string
	NeedsPayload    bool
}

// controls are the §3.3 ticket mutations this surface offers, with the
// payload shape each one takes. The list is the console's menu; whether any
// entry is usable is the tool's answer, asked at request time.
var controls = []Control{
	{Verb: "hold", Label: "Hold", PayloadTemplate: `{"holdId":"blocked","reason":"REASON"}`, NeedsPayload: true},
	{Verb: "release-hold", Label: "Release hold", PayloadTemplate: `{"holdId":"blocked"}`, NeedsPayload: true},
	{Verb: "prioritize", Label: "Prioritize", PayloadTemplate: `{"order":"0","priority":"P1"}`, NeedsPayload: true},
	{Verb: "refine", Label: "Refine", PayloadTemplate: `{"title":"NEW TITLE"}`, NeedsPayload: true},
	{Verb: "archive", Label: "Archive", PayloadTemplate: `{"reason":"REASON"}`, NeedsPayload: true},
	{Verb: "restore", Label: "Restore", PayloadTemplate: `{"reason":"REASON"}`, NeedsPayload: true},
	{Verb: "reopen", Label: "Reopen", PayloadTemplate: `{"reason":"REASON"}`, NeedsPayload: true},
	{Verb: "set-dependencies", Label: "Set dependencies (raw payload)", PayloadTemplate: `{"dependencies":[]}`, NeedsPayload: true},
}

// Controls answers, for one ticket, which mutations this tool can perform.
func (c *Capabilities) Controls() []Control {
	out := make([]Control, 0, len(controls))
	for _, control := range controls {
		control.Enabled = c.Implements("ticket " + control.Verb)
		if !control.Enabled {
			control.Reason = c.Reason("ticket " + control.Verb)
		}
		out = append(out, control)
	}
	return out
}

// MutationRequest is one delegated change. RequestID and IssuedAt are minted
// when the form is rendered and travel with it, so resubmitting the same form
// replays the original request instead of committing a second one.
type MutationRequest struct {
	Verb      string
	TicketID  string
	Expected  string
	Payload   string
	RequestID string
	IssuedAt  string
}

// MutationResult is what the owning tool answered.
type MutationResult struct {
	Request  MutationRequest
	Envelope *Envelope
	Source   Source
	Conflict bool
	Err      string
}

// Mutate performs one change by invoking the owning tool's own verb. The
// console writes nothing itself (LAC-V0-012), sends the expectedRevision the
// operator was shown, and never re-reads and retries on their behalf: a
// conflict is returned for the operator to re-read (LAC-V0-014).
func (t *Taskman) Mutate(ctx context.Context, request MutationRequest) *MutationResult {
	if !reviewedMutation(request.Verb) {
		return &MutationResult{Request: request, Err: "mutation is not offered by this console"}
	}
	args := []string{"ticket", request.Verb, "--request-id", request.RequestID}
	if request.IssuedAt != "" {
		args = append(args, "--issued-at", request.IssuedAt)
	}
	if request.Verb != "create" {
		args = append(args, "--target", request.TicketID, "--expected-revision", request.Expected)
	}
	args = append(args, "--payload", request.Payload)

	envelope, source := t.Run(ctx, args...)
	result := &MutationResult{Request: request, Envelope: envelope, Source: source}
	if envelope == nil {
		result.Err = source.Err
		return result
	}
	for _, code := range envelope.Codes {
		if code == "STALE_TICKET" || code == "STALE_TREE" || code == "REQUEST_ID_CONFLICT" {
			result.Conflict = true
		}
	}
	// A refusal whose warning names the revision is the stale-read case even
	// when the tool did not attach a code for it.
	if envelope.Refused() && !result.Conflict {
		for _, warning := range envelope.Warnings {
			if strings.Contains(warning, "revision") {
				result.Conflict = true
			}
		}
	}
	return result
}

// NewRequestID mints an idempotency key for one form. It is stable for that
// rendered form, so a double submit is a replay rather than a second commit.
func NewRequestID() string {
	var raw [16]byte
	if _, err := rand.Read(raw[:]); err != nil {
		return "console-" + time.Now().UTC().Format("20060102150405.000000000")
	}
	return "console-" + hex.EncodeToString(raw[:])
}

// IssuedNow is the timestamp form `atm` admits.
func IssuedNow() string { return time.Now().UTC().Format("2006-01-02T15:04:05Z") }
