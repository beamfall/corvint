package wire

// Obligation-ledger bounds and grammars (TOL-V0-001..004). The native codec
// and Core's read-only planner both use them, so the two readers of the
// optional `obligations` record member cannot drift.
const (
	// ObligationsMaxEntries bounds the folded ledger (TOL-V0-003).
	ObligationsMaxEntries = 256
	// ObligationsMaxEvents bounds the ledger event chain (TOL-V0-001).
	ObligationsMaxEvents = 1024
	// ObligationsMaxPrefixBytes bounds the ledger id prefix (TOL-V0-001).
	ObligationsMaxPrefixBytes = 16
	// ObligationsMaxIDDigits bounds the decimal suffix of an id (TOL-V0-003).
	ObligationsMaxIDDigits = 6
	// ObligationsMaxAliasBytes bounds an adopter alias (TOL-V0-034, raised
	// from 64 to 128 bytes by TOL-V0-055).
	ObligationsMaxAliasBytes = 128
)

// ObligationAliasByte reports whether c is in the alias alphabet
// [A-Za-z0-9._:-] (TOL-V0-034).
func ObligationAliasByte(c byte) bool {
	return c >= 'A' && c <= 'Z' || c >= 'a' && c <= 'z' || c >= '0' && c <= '9' || c == '.' || c == '_' || c == ':' || c == '-'
}

// ParseObligationAlias validates an adopter alias: 1..128 bytes of
// [A-Za-z0-9._:-] that start and end with a letter or digit (TOL-V0-034,
// TOL-V0-055). A length refusal names the limit and the alias length.
func ParseObligationAlias(where, s string) (string, error) {
	if s == "" || len(s) > ObligationsMaxAliasBytes {
		return "", Errorf(CodeMalformed, where, "obligation alias is %d bytes; the alias limit is 1..%d bytes", len(s), ObligationsMaxAliasBytes)
	}
	alnum := func(c byte) bool { return c >= 'A' && c <= 'Z' || c >= 'a' && c <= 'z' || c >= '0' && c <= '9' }
	for i := 0; i < len(s); i++ {
		if !ObligationAliasByte(s[i]) {
			return "", Errorf(CodeMalformed, where, "obligation alias must match [A-Za-z0-9][A-Za-z0-9._:-]*")
		}
	}
	if !alnum(s[0]) || !alnum(s[len(s)-1]) {
		return "", Errorf(CodeMalformed, where, "obligation alias must start and end with a letter or digit")
	}
	return s, nil
}

// ParseObligationPrefix validates a ledger prefix: 1..16 bytes of [A-Z0-9]
// starting with a letter (TOL-V0-001).
func ParseObligationPrefix(where, s string) (string, error) {
	if s == "" || len(s) > ObligationsMaxPrefixBytes {
		return "", Errorf(CodeMalformed, where, "obligation prefix must be 1..%d bytes", ObligationsMaxPrefixBytes)
	}
	for i := 0; i < len(s); i++ {
		c := s[i]
		upper := c >= 'A' && c <= 'Z'
		if !upper && (i == 0 || c < '0' || c > '9') {
			return "", Errorf(CodeMalformed, where, "obligation prefix must match [A-Z][A-Z0-9]*")
		}
	}
	return s, nil
}

// ParseObligationID validates `<prefix>-<1..6 decimal digits without a
// leading zero>` (TOL-V0-003) and returns the prefix.
func ParseObligationID(where, s string) (prefix string, err error) {
	dash := -1
	for i := len(s) - 1; i >= 0; i-- {
		if s[i] == '-' {
			dash = i
			break
		}
	}
	if dash <= 0 {
		return "", Errorf(CodeMalformed, where, "obligation id must be <prefix>-<number>")
	}
	if _, err := ParseObligationPrefix(where, s[:dash]); err != nil {
		return "", err
	}
	n := s[dash+1:]
	if n == "" || len(n) > ObligationsMaxIDDigits || n[0] == '0' {
		return "", Errorf(CodeMalformed, where, "obligation id number must be 1..%d digits without a leading zero", ObligationsMaxIDDigits)
	}
	for i := 0; i < len(n); i++ {
		if n[i] < '0' || n[i] > '9' {
			return "", Errorf(CodeMalformed, where, "obligation id number must be decimal")
		}
	}
	return s[:dash], nil
}
