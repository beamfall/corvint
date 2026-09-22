package workqueue

import (
	"regexp"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/Beamfall/corvint/internal/cem/wire"
)

const maxCount = uint64(2147483647)

var tokenPattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]*$`)

func ParseCount(value string) (Count, error) {
	parsed, err := parseDecimal(value)
	if err != nil {
		return 0, err
	}
	return Count(parsed), nil
}

func ParseRank(value string) (Rank, error) {
	parsed, err := parseDecimal(value)
	if err != nil {
		return 0, err
	}
	return Rank(parsed), nil
}

func parseDecimal(value string) (uint32, error) {
	if value == "" {
		return 0, fail(CodeMalformedInput, "count is empty")
	}
	if len(value) > 1 && value[0] == '0' {
		return 0, fail(CodeMalformedInput, "count has a leading zero")
	}
	for _, current := range value {
		if current < '0' || current > '9' {
			return 0, fail(CodeMalformedInput, "count is not decimal")
		}
	}
	parsed, err := strconv.ParseUint(value, 10, 31)
	if err != nil || parsed > maxCount {
		return 0, fail(CodeMalformedInput, "count exceeds its range")
	}
	return uint32(parsed), nil
}

func ValidateIdentifier(value string) error {
	if len(value) == 0 || len([]byte(value)) > 128 || !utf8.ValidString(value) {
		return fail(CodeMalformedInput, "identifier length or encoding is invalid")
	}
	for _, current := range value {
		if forbiddenIdentifierRune(current) {
			return fail(CodeHostileInput, "identifier contains a forbidden code point")
		}
	}
	return nil
}

func forbiddenIdentifierRune(value rune) bool {
	if value <= 0x1f || value == 0x7f || (value >= 0x80 && value <= 0x9f) {
		return true
	}
	switch value {
	case '\ufeff', '\u061c', '\u200e', '\u200f', '\u202a', '\u202b', '\u202c', '\u202d', '\u202e', '\u2066', '\u2067', '\u2068', '\u2069':
		return true
	}
	return false
}

func ValidatePath(value string) error {
	if len(value) == 0 || len([]byte(value)) > 512 || !utf8.ValidString(value) {
		return fail(CodeMalformedInput, "path length or encoding is invalid")
	}
	if strings.HasPrefix(value, "/") || strings.Contains(value, `\`) {
		return fail(CodeMalformedInput, "path is not repository-relative POSIX")
	}
	trimmed := strings.TrimSuffix(value, "/")
	if trimmed == "" {
		return fail(CodeMalformedInput, "path is empty")
	}
	for _, segment := range strings.Split(trimmed, "/") {
		if segment == "" || segment == "." || segment == ".." {
			return fail(CodeMalformedInput, "path contains an invalid segment")
		}
	}
	for _, current := range value {
		if current <= 0x1f || current == 0x7f || (current >= 0x80 && current <= 0x9f) {
			return fail(CodeHostileInput, "path contains a forbidden code point")
		}
	}
	return nil
}

func validDigest(value string) bool { return wire.IsSha256(value) }

func validContentID(value, kind string) bool {
	prefix := kind + ":sha256:"
	return strings.HasPrefix(value, prefix) && len(value) == len(prefix)+64 && validDigest(value[len(prefix):])
}

func splitRepositoryID(value string) (string, bool) {
	parts := strings.Split(value, ":")
	if len(parts) != 2 || parts[0] != "repo" || !validToken(parts[1]) || len(value) > 128 {
		return "", false
	}
	return parts[1], true
}

func splitQueueID(value string) (string, string, bool) {
	parts := strings.Split(value, ":")
	if len(parts) != 3 || parts[0] != "queue" || !validToken(parts[1]) || !validToken(parts[2]) || len(value) > 128 {
		return "", "", false
	}
	return parts[1], parts[2], true
}

func splitQualifiedID(value, kind string) (string, string, string, bool) {
	parts := strings.Split(value, ":")
	if len(value) > 128 || parts[0] != kind {
		return "", "", "", false
	}
	if shortQualifiedKind(kind) {
		if len(parts) != 3 || !validToken(parts[1]) || !validToken(parts[2]) {
			return "", "", "", false
		}
		return parts[1], parts[2], "", true
	}
	if len(parts) != 4 {
		return "", "", "", false
	}
	if !validToken(parts[1]) || !validToken(parts[2]) || !validToken(parts[3]) {
		return "", "", "", false
	}
	return parts[1], parts[2], parts[3], true
}

func shortQualifiedKind(kind string) bool {
	return kind == "access" || kind == "checkpoint" || kind == "scope"
}

func validToken(value string) bool { return tokenPattern.MatchString(value) }

func sameAuthority(value, kind, authority, queue string) (bool, bool) {
	actualAuthority, actualQueue, _, valid := splitQualifiedID(value, kind)
	if !valid {
		return false, false
	}
	return actualAuthority == authority && actualQueue == queue, true
}
