package tcq

import (
	"strings"
	"unicode/utf8"
)

// normalPath applies the TCQ-V0-028 path normalization: replace `\` with `/`,
// remove at most one leading `./`, then enforce the CEM path grammar. Absolute,
// drive-prefixed, empty, dot, and dot-dot paths are unkeyed rather than
// rejected, because a hostile reporter path must not fail the whole invocation.
func normalPath(value string, maxBytes int) (string, bool) {
	if value == "" || len(value) > maxBytes || !utf8.ValidString(value) || hasControl(value) {
		return "", false
	}
	value = strings.ReplaceAll(value, "\\", "/")
	value = strings.TrimPrefix(value, "./")
	if value == "" || value == "." || strings.HasPrefix(value, "/") || hasDrivePrefix(value) {
		return "", false
	}
	for _, part := range strings.Split(value, "/") {
		if part == "" || part == "." || part == ".." {
			return "", false
		}
	}
	return value, true
}

func hasControl(value string) bool {
	for _, character := range value {
		if character < 0x20 || character == 0x7f {
			return true
		}
	}
	return false
}

func hasDrivePrefix(value string) bool {
	if len(value) < 2 || value[1] != ':' {
		return false
	}
	letter := value[0]
	return (letter >= 'A' && letter <= 'Z') || (letter >= 'a' && letter <= 'z')
}
