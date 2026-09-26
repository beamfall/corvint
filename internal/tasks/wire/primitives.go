package wire

import (
	"strconv"
	"strings"
	"time"
	"unicode/utf8"
)

// Count is a decimal string `0` or a non-zero integer without leading zero,
// at most 2147483647 (§2). Used only for cardinalities and small counters.
type Count string

// Size is a decimal string without leading zeros in 0..18446744073709551615
// (§2). Used for byte sizes, sequence numbers, generations, pids and so on.
type Size string

// Digest is 64 lowercase hexadecimal SHA-256 characters.
type Digest string

// Timestamp is `YYYY-MM-DDTHH:MM:SSZ` (UTC, seconds; advisory only).
type Timestamp string

func decimalShape(s string) bool {
	if s == "" {
		return false
	}
	if s == "0" {
		return true
	}
	if s[0] < '1' || s[0] > '9' {
		return false
	}
	for i := 1; i < len(s); i++ {
		if s[i] < '0' || s[i] > '9' {
			return false
		}
	}
	return true
}

// ParseCount validates a Count.
func ParseCount(where, s string) (Count, error) {
	if !decimalShape(s) {
		return "", Errorf(CodeMalformed, where, "Count must be 0 or a decimal integer without sign or leading zero, got %q", s)
	}
	if len(s) > 10 {
		return "", Errorf(CodeMalformed, where, "Count above 2147483647: %q", s)
	}
	n, err := strconv.ParseUint(s, 10, 64)
	if err != nil || n > MaxCountValue {
		return "", Errorf(CodeMalformed, where, "Count above 2147483647: %q", s)
	}
	return Count(s), nil
}

// Int returns the numeric value of a validated Count.
func (c Count) Int() int64 {
	n, _ := strconv.ParseInt(string(c), 10, 64)
	return n
}

// CountOf renders a non-negative integer as a Count; values above the Count
// maximum are a programming error and are clamped so they can never encode
// as a valid Count silently: the caller must validate.
func CountOf(n int64) Count {
	if n < 0 {
		n = 0
	}
	return Count(strconv.FormatInt(n, 10))
}

// ParseSize validates a Size.
func ParseSize(where, s string) (Size, error) {
	if !decimalShape(s) {
		return "", Errorf(CodeMalformed, where, "Size must be 0 or a decimal integer without sign or leading zero, got %q", s)
	}
	if len(s) > 20 {
		return "", Errorf(CodeMalformed, where, "Size above uint64: %q", s)
	}
	if _, err := strconv.ParseUint(s, 10, 64); err != nil {
		return "", Errorf(CodeMalformed, where, "Size above uint64: %q", s)
	}
	return Size(s), nil
}

// Uint64 returns the numeric value of a validated Size.
func (s Size) Uint64() uint64 {
	n, _ := strconv.ParseUint(string(s), 10, 64)
	return n
}

// SizeOf renders an unsigned integer as a Size.
func SizeOf(n uint64) Size { return Size(strconv.FormatUint(n, 10)) }

func isLowerHex(s string) bool {
	for i := 0; i < len(s); i++ {
		c := s[i]
		if !(c >= '0' && c <= '9') && !(c >= 'a' && c <= 'f') {
			return false
		}
	}
	return true
}

// ParseDigest validates a Digest.
func ParseDigest(where, s string) (Digest, error) {
	if len(s) != 64 || !isLowerHex(s) {
		return "", Errorf(CodeMalformed, where, "Digest must be 64 lowercase hex characters")
	}
	return Digest(s), nil
}

// ParseOID validates a Git object id (40 hex for sha1, 64 for sha256).
func ParseOID(where, s string) (string, error) {
	if (len(s) != 40 && len(s) != 64) || !isLowerHex(s) {
		return "", Errorf(CodeMalformed, where, "OID must be 40 or 64 lowercase hex characters")
	}
	return s, nil
}

// Hostile code points (WQO §4.1): NUL, BOM, C0 and C1 controls, bidi format
// controls, invalid Unicode. Prose additionally permits TAB, LF and CR.
func hostileRune(r rune, allowTabLfCr bool) bool {
	switch {
	case r == 0:
		return true
	case r == '\t' || r == '\n' || r == '\r':
		return !allowTabLfCr
	case r < 0x20:
		return true
	case r >= 0x80 && r <= 0x9F:
		return true
	case r == 0xFEFF:
		return true
	case r == 0x061C, r == 0x200E, r == 0x200F:
		return true
	case r >= 0x202A && r <= 0x202E:
		return true
	case r >= 0x2066 && r <= 0x2069:
		return true
	case r >= 0xD800 && r <= 0xDFFF:
		return true
	case r > 0x10FFFF:
		return true
	}
	return false
}

func checkHostile(s, where string, allowTabLfCr bool) error {
	for i := 0; i < len(s); {
		r, size := utf8.DecodeRuneInString(s[i:])
		if r == utf8.RuneError && size <= 1 {
			return Errorf(CodeMalformed, where, "invalid UTF-8")
		}
		if hostileRune(r, allowTabLfCr) {
			return Errorf(CodeMalformed, where, "hostile code point U+%04X", r)
		}
		i += size
	}
	return nil
}

// ParseIdentifier validates an Identifier: 1..128 UTF-8 bytes, no hostile
// code points, no TAB/LF/CR.
func ParseIdentifier(where, s string) (string, error) {
	return parseText(where, s, 1, MaxIdentifierBytes, false, "Identifier")
}

// ParseLabel validates a label: 1..64 bytes under the identifier rules.
func ParseLabel(where, s string) (string, error) {
	return parseText(where, s, 1, MaxLabelBytes, false, "label")
}

// ParseProse validates a prose field of at most max bytes; TAB, LF and CR are
// permitted. min is 0 or 1.
func ParseProse(where, s string, min, max int) (string, error) {
	return parseText(where, s, min, max, true, "prose")
}

func parseText(where, s string, min, max int, allowTabLfCr bool, kind string) (string, error) {
	if len(s) < min {
		return "", Errorf(CodeMalformed, where, "%s must be at least %d byte(s)", kind, min)
	}
	if len(s) > max {
		return "", Errorf(CodeLimitExceeded, where, "%s longer than %d bytes (%d)", kind, max, len(s))
	}
	if err := checkHostile(s, where, allowTabLfCr); err != nil {
		return "", err
	}
	return s, nil
}

// ParsePathText validates a PathText (§2): an absolute filesystem path of
// 1..4096 UTF-8 bytes with no hostile code point and no TAB, LF or CR. It
// is the type of `head.primaryWorktree` and `manifest.primaryWorktree`; it
// is not an Identifier and shares no bound with one. Nothing is normalized:
// the bytes are compared exactly against the resolved primary worktree.
func ParsePathText(where, s string) (string, error) {
	if len(s) < 1 {
		return "", Errorf(CodeMalformed, where, "PathText must not be empty")
	}
	if len(s) > MaxPathTextBytes {
		return "", Errorf(CodeLimitExceeded, where, "PathText longer than %d bytes (%d)", MaxPathTextBytes, len(s))
	}
	if err := checkHostile(s, where, false); err != nil {
		return "", err
	}
	if s[0] != '/' {
		return "", Errorf(CodeMalformed, where, "PathText must be an absolute path")
	}
	return s, nil
}

func isTokenByte(c byte, first bool) bool {
	if (c >= 'A' && c <= 'Z') || (c >= 'a' && c <= 'z') || (c >= '0' && c <= '9') {
		return true
	}
	if first {
		return false
	}
	return c == '.' || c == '_' || c == '-'
}

// ParseToken validates the `[A-Za-z0-9][A-Za-z0-9._-]*` grammar of §2 with
// a byte bound.
func ParseToken(where, s string, max int) (string, error) {
	if s == "" {
		return "", Errorf(CodeMalformed, where, "token must not be empty")
	}
	if len(s) > max {
		return "", Errorf(CodeLimitExceeded, where, "token longer than %d bytes", max)
	}
	for i := 0; i < len(s); i++ {
		if !isTokenByte(s[i], i == 0) {
			return "", Errorf(CodeMalformed, where, "token %q violates [A-Za-z0-9][A-Za-z0-9._-]*", s)
		}
	}
	return s, nil
}

// FoldToken is the ASCII case folding under which local tokens are unique.
func FoldToken(s string) string {
	return strings.ToLower(s)
}

// ParsePath validates a WQO Path: 1..512 bytes, `/` separators, no leading
// `/`, no empty, `.` or `..` segment, no backslash, NUL or control byte. A
// trailing `/` denotes a directory prefix.
func ParsePath(where, s string) (string, error) {
	if s == "" {
		return "", Errorf(CodeMalformed, where, "Path must not be empty")
	}
	if len(s) > 512 {
		return "", Errorf(CodeLimitExceeded, where, "Path longer than 512 bytes")
	}
	if err := checkHostile(s, where, false); err != nil {
		return "", err
	}
	for i := 0; i < len(s); i++ {
		if s[i] == '\\' || s[i] == 0x7F {
			return "", Errorf(CodeMalformed, where, "Path contains a backslash or control byte")
		}
	}
	if s[0] == '/' {
		return "", Errorf(CodeMalformed, where, "Path must be repository-relative (no leading '/')")
	}
	body := s
	if strings.HasSuffix(body, "/") {
		body = body[:len(body)-1]
	}
	for _, seg := range strings.Split(body, "/") {
		if seg == "" || seg == "." || seg == ".." {
			return "", Errorf(CodeMalformed, where, "Path has an empty, '.' or '..' segment")
		}
	}
	return s, nil
}

// ParseTimestamp validates `YYYY-MM-DDTHH:MM:SSZ`.
func ParseTimestamp(where, s string) (Timestamp, error) {
	const layout = "2006-01-02T15:04:05Z"
	if len(s) != len(layout) {
		return "", Errorf(CodeMalformed, where, "timestamp must be YYYY-MM-DDTHH:MM:SSZ")
	}
	t, err := time.Parse(layout, s)
	if err != nil || t.UTC().Format(layout) != s {
		return "", Errorf(CodeMalformed, where, "timestamp must be a valid YYYY-MM-DDTHH:MM:SSZ")
	}
	return Timestamp(s), nil
}

// ParseDate validates `YYYY-MM-DD`.
func ParseDate(where, s string) (string, error) {
	const layout = "2006-01-02"
	if len(s) != len(layout) {
		return "", Errorf(CodeMalformed, where, "date must be YYYY-MM-DD")
	}
	t, err := time.Parse(layout, s)
	if err != nil || t.Format(layout) != s {
		return "", Errorf(CodeMalformed, where, "date must be a valid YYYY-MM-DD")
	}
	return s, nil
}

// CheckProfile checks a profile string against the expected `<name>/0`. A
// different version of the same profile is UNSUPPORTED_VERSION; a different
// profile entirely is MALFORMED.
func CheckProfile(where, got, want string) error {
	if got == want {
		return nil
	}
	slash := strings.LastIndex(want, "/")
	name := want[:slash]
	if strings.HasPrefix(got, name+"/") {
		return Errorf(CodeUnsupportedVersion, where, "profile %q is not supported; want %q", got, want)
	}
	return Errorf(CodeMalformed, where, "profile %q; want %q", got, want)
}
