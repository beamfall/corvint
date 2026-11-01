package transaction

import (
	"github.com/Beamfall/corvint/internal/tasks/snapshot"
	"github.com/Beamfall/corvint/internal/tasks/ticket"
)

// TicketPosts memoizes, for one receipt at a time, the bytes and decoded
// record of each intent/tickets/ and attempts/ post, so the ERG-V0-009
// review, ESC-V0-010 escalation, obligation and gate reuse binding steps of
// one fold read and decode a post once (V1-3186). It returns exactly what
// each step would compute itself, errors included; a different receipt
// resets it. The steps only read the records.
type TicketPosts struct {
	rc       *snapshot.Receipt
	posts    map[int]ticketPost
	attempts map[int]attemptPost
}

type ticketPost struct {
	raw            []byte
	rec            *ticket.Record
	rawErr, decErr error
}

type attemptPost struct {
	raw            []byte
	at             *snapshot.Attempt
	rawErr, decErr error
}

func (t *TicketPosts) reset(rc *snapshot.Receipt) {
	if t.rc != rc {
		t.rc, t.posts, t.attempts = rc, map[int]ticketPost{}, map[int]attemptPost{}
	}
}

// post returns post i of rc as externalPostBytes and ticket.Decode would; a
// nil memo computes it afresh.
func (t *TicketPosts) post(rc *snapshot.Receipt, i int, blob ExternalReviewBlob) ticketPost {
	if t != nil {
		t.reset(rc)
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

// attempt returns post i of rc as externalPostBytes and snapshot.DecodeAttempt
// would; a nil memo computes it afresh.
func (t *TicketPosts) attempt(rc *snapshot.Receipt, i int, blob ExternalReviewBlob) attemptPost {
	if t != nil {
		t.reset(rc)
		if p, ok := t.attempts[i]; ok {
			return p
		}
	}
	var p attemptPost
	if p.raw, p.rawErr = externalPostBytes(rc.Post[i], blob); p.rawErr == nil {
		p.at, p.decErr = snapshot.DecodeAttempt(p.raw)
	}
	if t != nil {
		t.attempts[i] = p
	}
	return p
}
