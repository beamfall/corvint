package console

import "context"

// Card is one ticket as the board shows it. Every field is copied from the
// tool's item; nothing is computed here.
type Card struct {
	TicketID    string
	Title       string
	Status      string
	Kind        string
	Priority    string
	Owner       string
	Milestone   string
	Eligibility string
	NextAction  string
	Revision    string
	Blockers    int
	Unknowns    []Unknown
	Axes        Axes
}

// Unknown is one fact the tool reported it could not observe. It is rendered,
// never dropped: an unobservable fact is the difference between a measured
// zero and no measurement at all (LAC-V0-008).
type Unknown struct {
	Code   string
	Detail string
	Ticket string
}

// Column is one board column. Unmapped is the column for a status the tool
// did not enumerate; a card there is visible rather than dropped
// (LAC-V0-016).
type Column struct {
	Status   string
	Unmapped bool
	Cards    []Card
}

// Board is the whole board plus the attribution and refusal of the read that
// produced it.
type Board struct {
	Columns   []Column
	Source    Source
	Envelope  *Envelope
	Total     int
	Statuses  []string
	Err       string
	Untrusted []string
}

// ReadBoard lists the queue and groups it into the columns the tool
// enumerates. A refusal is carried through as a refusal: the caller renders
// it, and the board stays empty of invented rows.
func (t *Taskman) ReadBoard(ctx context.Context, caps *Capabilities) *Board {
	envelope, source := t.Run(ctx, "ticket", "list", "--limit", "500")
	board := &Board{Source: source, Envelope: envelope, Statuses: caps.Statuses}
	if envelope == nil {
		board.Err = source.Err
		return board
	}
	board.Untrusted = envelope.Untrusted
	if envelope.Refused() {
		return board
	}
	byStatus := map[string][]Card{}
	for _, item := range envelope.Items {
		card := cardOf(item)
		byStatus[card.Status] = append(byStatus[card.Status], card)
		board.Total++
	}
	// Columns come from the tool's enumeration, in its order.
	for _, status := range caps.Statuses {
		board.Columns = append(board.Columns, Column{Status: status, Cards: byStatus[status]})
		delete(byStatus, status)
	}
	// Anything the tool did not enumerate is still shown, in its own column.
	for status, cards := range byStatus {
		board.Columns = append(board.Columns, Column{Status: status, Unmapped: true, Cards: cards})
	}
	return board
}

func cardOf(item map[string]any) Card {
	card := Card{
		TicketID:    stringOf(item["ticketId"]),
		Title:       stringOf(item["title"]),
		Status:      stringOf(item["status"]),
		Kind:        stringOf(item["kind"]),
		Priority:    stringOf(item["priority"]),
		Owner:       stringOf(item["owner"]),
		Milestone:   stringOf(item["milestone"]),
		Eligibility: stringOf(item["eligibility"]),
		NextAction:  stringOf(item["nextAction"]),
		Revision:    stringOf(item["revision"]),
		Axes:        UnstatedAxes(),
	}
	if blockers, ok := item["blockers"].([]any); ok {
		card.Blockers = len(blockers)
	}
	card.Unknowns = unknownsOf(item["unknowns"])
	return card
}

func unknownsOf(value any) []Unknown {
	list, ok := value.([]any)
	if !ok {
		return nil
	}
	out := make([]Unknown, 0, len(list))
	for _, entry := range list {
		object, ok := entry.(map[string]any)
		if !ok {
			continue
		}
		out = append(out, Unknown{
			Code:   stringOf(object["code"]),
			Detail: stringOf(object["detail"]),
			Ticket: stringOf(object["ticketId"]),
		})
	}
	return out
}

// stringOf renders a JSON value as text. A JSON null becomes the empty
// string, which callers render as an absent field rather than as "null".
func stringOf(value any) string {
	s, ok := value.(string)
	if !ok {
		return ""
	}
	return s
}
