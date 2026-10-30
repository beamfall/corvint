package transaction

import (
	"github.com/Beamfall/corvint/internal/tasks/snapshot"
	"github.com/Beamfall/corvint/internal/tasks/ticket"
)

// TicketPosts memoizes, for one receipt at a time, the bytes and decoded
// record of each intent/tickets/ post, so the ERG-V0-009 review and
// ESC-V0-010 escalation binding steps of one fold read and decode a post
// once. It returns exactly what each step would compute itself, errors
// included; a different receipt resets it. The steps only read the record.
type TicketPosts struct {
	rc    *snapshot.Receipt
	posts map[int]ticketPost
}

type ticketPost struct {
	raw            []byte
	rec            *ticket.Record
	rawErr, decErr error
}

// post returns post i of rc as externalPostBytes and ticket.Decode would; a
// nil memo computes it afresh.
func (t *TicketPosts) post(rc *snapshot.Receipt, i int, blob ExternalReviewBlob) ticketPost {
	if t != nil {
		if t.rc != rc {
			t.rc, t.posts = rc, map[int]ticketPost{}
		}
		if p, ok := t.posts[i]; ok {
			return p
		}
	}
	var p ticketPost
	if p.raw, p.rawErr = externalPostBytes(rc.Post[i], blob); p.rawErr == nil {
		p.rec, p.decErr = ticket.Decode(p.raw)
	}
	if t != nil {
		t.posts[i] = p
	}
	return p
}
