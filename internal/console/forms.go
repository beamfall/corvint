package console

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"slices"
	"strconv"
	"strings"
	"unicode/utf8"
)

type ticketForm struct {
	Verb, TicketID, Expected, RequestID, IssuedAt string
	Title, Body, Criteria                         string
	Milestone, Dependencies                       string
	Order, Priority                               string
}

// ATM's frozen §1 limits for the fields this form carries (corvint-taskman
// `wire.MaxTitleBytes`, `MaxBodyBytes`, `MaxAcceptanceCriteria`,
// `MaxCriterionBytes`). The console refuses before invoking the tool so a
// too-long entry is never lost to a tool refusal it cannot render fully.
const (
	atmTitleBytes     = 512
	atmBodyBytes      = 64 << 10
	atmMaxCriteria    = 64
	atmCriterionBytes = 4 << 10
)

// criteria splits the textarea into ATM acceptance criteria: one per
// non-blank line, trimmed.
func (f *ticketForm) criteria() []string {
	criteria := []string{}
	for _, line := range strings.Split(f.Criteria, "\n") {
		if line = strings.TrimSpace(line); line != "" {
			criteria = append(criteria, line)
		}
	}
	return criteria
}

func newTicketForm(verb string) *ticketForm {
	return &ticketForm{Verb: verb, RequestID: NewRequestID(), IssuedAt: IssuedNow()}
}
func readTicketForm(r *http.Request) *ticketForm {
	return &ticketForm{Verb: r.PostForm.Get("verb"), TicketID: r.PostForm.Get("ticket"), Expected: r.PostForm.Get("expected"), RequestID: r.PostForm.Get("requestId"), IssuedAt: r.PostForm.Get("issuedAt"), Title: r.PostForm.Get("title"), Body: r.PostForm.Get("body"), Criteria: r.PostForm.Get("criteria"), Milestone: r.PostForm.Get("milestone"), Dependencies: r.PostForm.Get("dependencies"), Order: r.PostForm.Get("order"), Priority: r.PostForm.Get("priority")}
}

// dependencies splits the dependencies textarea into one entry per non-blank
// line, its fields joined by one space: `TICKET-ID` is a COMPLETED obligation
// and `TICKET-ID GATE-ID` a GATE_PASSED one. A blank form names no dependency.
func (f *ticketForm) dependencies() []string {
	dependencies := []string{}
	for _, line := range strings.Split(f.Dependencies, "\n") {
		if fields := strings.Fields(line); len(fields) > 0 {
			dependencies = append(dependencies, strings.Join(fields, " "))
		}
	}
	return dependencies
}

func (f *ticketForm) validate() error {
	if f.Verb == "set-dependencies" {
		if !utf8.ValidString(f.Dependencies) {
			return fmt.Errorf("Dependency text must be valid UTF-8.")
		}
		for _, line := range f.dependencies() {
			if strings.Count(line, " ") > 1 {
				return fmt.Errorf("Dependency line %q has more than a ticket id and a gate id. Your entries are retained below.", line)
			}
		}
		if _, err := strconv.ParseUint(f.Expected, 10, 64); err != nil || f.TicketID == "" {
			return fmt.Errorf("The shown ticket and revision are required; reload the ticket.")
		}
		return nil
	}
	if f.Verb == "prioritize" {
		if !utf8.ValidString(f.Order + f.Priority) {
			return fmt.Errorf("Order and priority must be valid UTF-8.")
		}
		if strings.TrimSpace(f.Order) == "" || strings.TrimSpace(f.Priority) == "" {
			return fmt.Errorf("Order and priority are required. Your entries are retained below.")
		}
		if _, err := strconv.ParseUint(f.Expected, 10, 64); err != nil || f.TicketID == "" {
			return fmt.Errorf("The shown ticket and revision are required; reload the ticket.")
		}
		return nil
	}
	if !utf8.ValidString(f.Title + f.Body + f.Criteria) {
		return fmt.Errorf("Ticket text must be valid UTF-8.")
	}
	if strings.TrimSpace(f.Title) == "" {
		return fmt.Errorf("Title is required. Your entries are retained below.")
	}
	if len(f.Title) > atmTitleBytes {
		return fmt.Errorf("Title exceeds ATM's limit of %d bytes (%d). Your entries are retained below.", atmTitleBytes, len(f.Title))
	}
	if len(f.Body) > atmBodyBytes {
		return fmt.Errorf("Body exceeds ATM's limit of %d bytes (%d). Your entries are retained below.", atmBodyBytes, len(f.Body))
	}
	criteria := f.criteria()
	if len(criteria) > atmMaxCriteria {
		return fmt.Errorf("At most %d acceptance criteria are accepted by ATM (%d given). Your entries are retained below.", atmMaxCriteria, len(criteria))
	}
	for index, line := range criteria {
		if len(line) > atmCriterionBytes {
			return fmt.Errorf("Acceptance criterion %d exceeds ATM's limit of %d bytes (%d). Your entries are retained below.", index+1, atmCriterionBytes, len(line))
		}
	}
	if f.Verb == "create" && (f.Expected != "" || f.TicketID != "") {
		return fmt.Errorf("New tickets cannot name an existing ticket or revision.")
	}
	if f.Verb == "refine" {
		if _, err := strconv.ParseUint(f.Expected, 10, 64); err != nil || f.TicketID == "" {
			return fmt.Errorf("The shown ticket and revision are required; reload the ticket.")
		}
	}
	return nil
}
func reviewedMutation(verb string) bool {
	if verb == "create" {
		return true
	}
	for _, c := range controls {
		if c.Verb == verb {
			return true
		}
	}
	return false
}
func (t *Taskman) formPayload(ctx context.Context, f *ticketForm) (string, error) {
	if f.Verb == "set-dependencies" {
		return dependenciesPayload(f.dependencies())
	}
	if f.Verb == "prioritize" {
		return prioritizePayload(f.Order, f.Priority)
	}
	payload := map[string]any{"title": f.Title, "body": nil}
	if f.Body != "" {
		payload["body"] = f.Body
	}
	if f.Verb == "refine" && strings.TrimSpace(f.Milestone) != "" {
		payload["milestone"] = f.Milestone
	}
	if f.Verb == "create" {
		envelope, source := t.Run(ctx, "queue", "status")
		if envelope == nil {
			return "", fmt.Errorf("Queue could not be read: %s", source.Err)
		}
		if envelope.Refused() || len(envelope.Items) == 0 || stringOf(envelope.Items[0]["queueId"]) == "" {
			return "", fmt.Errorf("Queue identity unavailable; reload the board. %s", strings.Join(envelope.Codes, ", "))
		}
		payload["acceptanceCriteria"] = f.criteria()
		for _, key := range []string{"capabilities", "dependencies", "labels", "requiredGates", "requirementRefs"} {
			payload[key] = []string{}
		}
		for _, key := range []string{"dueDate", "estimateMinutes", "milestone", "owner", "supersededBy", "supersedes"} {
			payload[key] = nil
		}
		payload["effects"] = map[string]any{"coverage": "UNKNOWN", "externalUnbounded": false, "resources": []any{}, "touchPaths": []string{}}
		payload["executionClass"], payload["kind"], payload["priority"], payload["order"] = "MANUAL", "FEATURE", "P2", "0"
		payload["source"] = map[string]any{"kind": "NATIVE", "sourceItemId": nil, "sourceQueueId": envelope.Items[0]["queueId"], "sourceRevisionSha256": nil}
	}
	var buffer bytes.Buffer
	encoder := json.NewEncoder(&buffer)
	encoder.SetEscapeHTML(false)
	if err := encoder.Encode(payload); err != nil {
		return "", err
	}
	return atmJSON(strings.TrimSuffix(buffer.String(), "\n")), nil
}

// prioritizePayload builds the prioritize payload from the form's order and
// priority fields, both sent as ATM's wire expects: quoted strings, not
// numbers or a closed enum the console would have to invent (LAC-V0-015).
func prioritizePayload(order, priority string) (string, error) {
	payload := map[string]any{"order": order, "priority": priority}
	var buffer bytes.Buffer
	encoder := json.NewEncoder(&buffer)
	encoder.SetEscapeHTML(false)
	if err := encoder.Encode(payload); err != nil {
		return "", err
	}
	return atmJSON(strings.TrimSuffix(buffer.String(), "\n")), nil
}

// dependenciesPayload builds the set-dependencies payload from one ticket id
// per line: a COMPLETED obligation on the whole ticket, no gate named. Go's
// map marshal sorts keys alphabetically, which is the gateId/obligation/
// ticketId order ATM's canonical encoding expects.
func dependenciesPayload(ids []string) (string, error) {
	ids = slices.Compact(slices.Sorted(slices.Values(ids)))
	entries := make([]map[string]any, 0, len(ids))
	for _, id := range ids {
		ticketID, gateID, gated := strings.Cut(id, " ")
		if strings.Contains(gateID, " ") {
			return "", fmt.Errorf("dependency line %q has more than a ticket id and a gate id", id)
		}
		if !gated {
			entries = append(entries, map[string]any{"gateId": nil, "obligation": "COMPLETED", "ticketId": ticketID})
			continue
		}
		entries = append(entries, map[string]any{"gateId": gateID, "obligation": "GATE_PASSED", "ticketId": ticketID})
	}
	payload := map[string]any{"dependencies": entries}
	var buffer bytes.Buffer
	encoder := json.NewEncoder(&buffer)
	encoder.SetEscapeHTML(false)
	if err := encoder.Encode(payload); err != nil {
		return "", err
	}
	return atmJSON(strings.TrimSuffix(buffer.String(), "\n")), nil
}

// dependenciesText renders a ticket's record.dependencies (as `atm ticket
// show` reports it) back into one entry per line, the shape the
// set-dependencies form edits: a GATE_PASSED entry keeps its gate as a second
// field, because saving replaces the whole list (LAC-V0-028). An entry the
// tool did not shape as expected is skipped rather than guessed at.
func dependenciesText(value any) string {
	items, ok := value.([]any)
	if !ok {
		return ""
	}
	var lines []string
	for _, item := range items {
		entry, ok := item.(map[string]any)
		if !ok {
			continue
		}
		id := stringOf(entry["ticketId"])
		if id == "" {
			continue
		}
		if gate := stringOf(entry["gateId"]); stringOf(entry["obligation"]) == "GATE_PASSED" && gate != "" {
			id += " " + gate
		}
		lines = append(lines, id)
	}
	return strings.Join(lines, "\n")
}

// ATM canonical strings use raw Unicode separators and long escapes for
// backspace/form-feed. Consume escape pairs so literal backslashes stay literal.
func atmJSON(raw string) string {
	var out strings.Builder
	for i := 0; i < len(raw); i++ {
		if raw[i] != '\\' || i+1 == len(raw) {
			out.WriteByte(raw[i])
			continue
		}
		i++
		switch raw[i] {
		case 'b':
			out.WriteString(`\u0008`)
		case 'f':
			out.WriteString(`\u000c`)
		case 'u':
			if i+4 < len(raw) && (raw[i+1:i+5] == "2028" || raw[i+1:i+5] == "2029") {
				if raw[i+4] == '8' {
					out.WriteRune('\u2028')
				} else {
					out.WriteRune('\u2029')
				}
				i += 4
			} else {
				out.WriteString(`\u`)
			}
		default:
			out.WriteByte('\\')
			out.WriteByte(raw[i])
		}
	}
	return out.String()
}
