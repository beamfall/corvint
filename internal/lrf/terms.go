package lrf

import (
	"bytes"
	"regexp"
	"unicode/utf8"

	"github.com/Beamfall/corvint/internal/cem/wire"
)

// obligationIDPattern accepts every OCM obligation ID (OCM-V0-952: three or
// more digits). The term-scan ID grammar of rule 4 stays three digits (decision 0265).
var obligationIDPattern = regexp.MustCompile(`^[A-Z][A-Z0-9-]{2,31}-[0-9]{3,}$`)

func requirementBody(statement []byte, obligationID string) ([]byte, error) {
	prefix := []byte("- `" + obligationID + "`: ")
	if !bytes.HasPrefix(statement, prefix) {
		return nil, &Error{Code: "invalid-requirement-prefix", Message: "requirement line prefix is invalid"}
	}
	body := statement[len(prefix):]
	if bytes.HasSuffix(body, []byte("\r\n")) {
		body = body[:len(body)-2]
	} else if bytes.HasSuffix(body, []byte("\n")) {
		body = body[:len(body)-1]
	}
	if bytes.ContainsAny(body, "\r\n") {
		return nil, &Error{Code: "invalid-requirement-prefix", Message: "requirement must occupy one logical line"}
	}
	return body, nil
}

func validPath(path string) bool { return utf8.ValidString(path) && wire.ValidatePath(path) == nil }
