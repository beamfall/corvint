package wire

import "strings"

// QueueID is a parsed `queue:<authority>:<queue>`.
type QueueID struct {
	Raw       string
	Authority string
	Queue     string
}

// TicketID is a parsed `ticket:<authority>:<queue>:<local>`.
type TicketID struct {
	Raw       string
	Authority string
	Queue     string
	Local     string
}

// QueueID returns the queue the ticket belongs to.
func (t TicketID) QueueID() string {
	return "queue:" + t.Authority + ":" + t.Queue
}

func checkIDLength(where, s string) error {
	if len(s) > MaxIdentifierBytes {
		return Errorf(CodeLimitExceeded, where, "ID longer than %d bytes", MaxIdentifierBytes)
	}
	for i := 0; i < len(s); i++ {
		if s[i] >= 0x80 {
			return Errorf(CodeMalformed, where, "ID must be ASCII")
		}
	}
	return nil
}

// ParseRepoID validates `repo:<authority-token>` and returns the token.
func ParseRepoID(where, s string) (string, error) {
	if err := checkIDLength(where, s); err != nil {
		return "", err
	}
	if !strings.HasPrefix(s, "repo:") {
		return "", Errorf(CodeMalformed, where, "repository ID must be repo:<authority>")
	}
	tok := s[len("repo:"):]
	if _, err := ParseToken(where, tok, MaxIdentifierBytes); err != nil {
		return "", err
	}
	return tok, nil
}

// ParseQueueID validates `queue:<authority>:<queue>`.
func ParseQueueID(where, s string) (QueueID, error) {
	if err := checkIDLength(where, s); err != nil {
		return QueueID{}, err
	}
	parts := strings.Split(s, ":")
	if len(parts) != 3 || parts[0] != "queue" {
		return QueueID{}, Errorf(CodeMalformed, where, "queue ID must be queue:<authority>:<queue>")
	}
	for _, tok := range parts[1:] {
		if _, err := ParseToken(where, tok, MaxIdentifierBytes); err != nil {
			return QueueID{}, err
		}
	}
	return QueueID{Raw: s, Authority: parts[1], Queue: parts[2]}, nil
}

// ParseTicketID validates `ticket:<authority>:<queue>:<local>` with the local
// token 1..64 bytes of the token grammar.
func ParseTicketID(where, s string) (TicketID, error) {
	if err := checkIDLength(where, s); err != nil {
		return TicketID{}, err
	}
	parts := strings.Split(s, ":")
	if len(parts) != 4 || parts[0] != "ticket" {
		return TicketID{}, Errorf(CodeMalformed, where, "ticket ID must be ticket:<authority>:<queue>:<local>")
	}
	for _, tok := range parts[1:3] {
		if _, err := ParseToken(where, tok, MaxIdentifierBytes); err != nil {
			return TicketID{}, err
		}
	}
	if _, err := ParseToken(where, parts[3], MaxLocalTokenBytes); err != nil {
		return TicketID{}, err
	}
	return TicketID{Raw: s, Authority: parts[1], Queue: parts[2], Local: parts[3]}, nil
}
