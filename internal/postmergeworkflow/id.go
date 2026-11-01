package postmergeworkflow

import (
	"errors"
	"fmt"
	"strings"
)

// MaxIDBytes bounds one identifier, the grammar's 128 bytes.
const MaxIDBytes = 128

// EncodeID is the one documented encoding of free text, such as a branch name
// claude/lane-a, into the identifier grammar ^[A-Za-z0-9][A-Za-z0-9._:-]{0,127}$
// (PMR-V0-960): every byte outside [A-Za-z0-9._-], ':' included, becomes ':'
// and two uppercase hex digits, so claude/lane-a is claude:2Flane-a. The
// first byte must be a letter or digit, as Git branch names that the grammar
// can carry are; a value that would not fit 128 bytes is refused.
func EncodeID(value string) (string, error) {
	if value == "" || !alphanumeric(value[0]) {
		return "", fmt.Errorf("identifier source %q must begin with a letter or digit", value)
	}
	var b strings.Builder
	for i := 0; i < len(value); i++ {
		c := value[i]
		if alphanumeric(c) || c == '.' || c == '_' || c == '-' {
			b.WriteByte(c)
			continue
		}
		fmt.Fprintf(&b, ":%02X", c)
	}
	encoded := b.String()
	if len(encoded) > MaxIDBytes {
		return "", fmt.Errorf("encoded identifier is %d bytes, over the %d-byte bound", len(encoded), MaxIDBytes)
	}
	return encoded, nil
}

// DecodeID reverses EncodeID. It refuses an identifier outside the grammar or
// one that EncodeID would not have produced, so decode then encode is the
// identity.
func DecodeID(id string) (string, error) {
	if !identifier.MatchString(id) {
		return "", errors.New("not an identifier")
	}
	var b strings.Builder
	for i := 0; i < len(id); i++ {
		if id[i] != ':' {
			b.WriteByte(id[i])
			continue
		}
		if i+2 >= len(id) || !upperHex(id[i+1]) || !upperHex(id[i+2]) {
			return "", fmt.Errorf("identifier %q has a ':' not followed by two uppercase hex digits", id)
		}
		c := hexValue(id[i+1])<<4 | hexValue(id[i+2])
		if alphanumeric(c) || c == '.' || c == '_' || c == '-' {
			return "", fmt.Errorf("identifier %q escapes %q, which the encoding keeps literal", id, string(rune(c)))
		}
		b.WriteByte(c)
		i += 2
	}
	return b.String(), nil
}

func alphanumeric(c byte) bool {
	return c >= 'A' && c <= 'Z' || c >= 'a' && c <= 'z' || c >= '0' && c <= '9'
}

func upperHex(c byte) bool { return c >= '0' && c <= '9' || c >= 'A' && c <= 'F' }

func hexValue(c byte) byte {
	if c <= '9' {
		return c - '0'
	}
	return c - 'A' + 10
}
