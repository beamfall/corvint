package contextindex

import (
	"bytes"
	"strings"
	"unicode"
	"unicode/utf8"
)

const (
	maxHistoryCommits = 200
	maxHistoryBytes   = 64 * 1024 * 1024
)

type historyEntry struct {
	commit, tree, subject string
	paths                 []string
}

// parseHistory refuses a path that is not UTF-8 unless skipNonUTF8 is set, in
// which case the path is left out of its commit. Query learning (GPK-V0-072)
// and the co-change slot both skip, because the index already excludes and
// discloses such a path (nonUTF8PathReason); no existing digest moves, since
// every repository the skip changes was refused before.
func parseHistory(raw []byte, objectFormat string, skipNonUTF8 bool) ([]historyEntry, []any, error) {
	entries := make([]historyEntry, 0, maxHistoryCommits)
	canonical := make([]any, 0, maxHistoryCommits)
	for _, record := range bytes.Split(raw, []byte{0x1e}) {
		if len(record) == 0 {
			continue
		}
		fields := bytes.Split(record, []byte{0})
		header := bytes.Trim(fields[0], "\n")
		parts := bytes.SplitN(header, []byte{0x1f}, 3)
		if len(parts) != 3 || !validObjectID(string(parts[0]), objectFormat) || !validObjectID(string(parts[1]), objectFormat) {
			return nil, nil, &Error{Message: "invalid Git history record"}
		}
		paths := make([]string, 0, len(fields)-1)
		pathValues := make([]any, 0, len(fields)-1)
		for _, rawPath := range fields[1:] {
			rawPath = bytes.Trim(rawPath, "\n")
			if len(rawPath) == 0 {
				continue
			}
			if !utf8.Valid(rawPath) {
				if skipNonUTF8 {
					continue
				}
				return nil, nil, &Error{Code: "unsupported-query-history", Message: "native Go authority-start query requires UTF-8 Git history paths"}
			}
			value := string(rawPath)
			paths = append(paths, value)
			pathValues = append(pathValues, value)
		}
		subject := decodePythonUTF8(parts[2])
		entry := historyEntry{commit: string(parts[0]), tree: string(parts[1]), subject: subject, paths: paths}
		entries = append(entries, entry)
		canonical = append(canonical, []any{entry.commit, entry.tree, entry.subject, pathValues})
		if len(entries) > maxHistoryCommits {
			return nil, nil, &Error{Message: "Git history exceeds its commit limit"}
		}
	}
	return entries, canonical, nil
}

// decodePythonUTF8 matches bytes.decode("utf-8", "replace"): malformed
// prefixes are replaced separately, while a valid prefix of a truncated or
// interrupted multibyte sequence is replaced once as a unit.
func decodePythonUTF8(value []byte) string {
	if utf8.Valid(value) {
		return string(value)
	}
	var output strings.Builder
	output.Grow(len(value))
	for offset := 0; offset < len(value); {
		first := value[offset]
		if first < utf8.RuneSelf {
			output.WriteByte(first)
			offset++
			continue
		}
		width := 0
		switch {
		case first >= 0xc2 && first <= 0xdf:
			width = 2
		case first >= 0xe0 && first <= 0xef:
			width = 3
		case first >= 0xf0 && first <= 0xf4:
			width = 4
		default:
			output.WriteRune(utf8.RuneError)
			offset++
			continue
		}
		consumed := 1
		if offset+1 < len(value) && validPythonSecondByte(first, value[offset+1]) {
			consumed = 2
			if width >= 3 && offset+2 < len(value) && continuationByte(value[offset+2]) {
				consumed = 3
				if width == 4 && offset+3 < len(value) && continuationByte(value[offset+3]) {
					consumed = 4
				}
			}
		}
		if consumed == width {
			output.Write(value[offset : offset+width])
		} else {
			output.WriteRune(utf8.RuneError)
		}
		offset += consumed
	}
	return output.String()
}

// TrimPythonSpace matches str.strip() for Python's Unicode whitespace set.
func TrimPythonSpace(value string) string {
	return strings.TrimFunc(value, func(character rune) bool {
		return unicode.IsSpace(character) || character >= 0x1c && character <= 0x1f
	})
}

func validPythonSecondByte(first, second byte) bool {
	if !continuationByte(second) {
		return false
	}
	switch first {
	case 0xe0:
		return second >= 0xa0
	case 0xed:
		return second <= 0x9f
	case 0xf0:
		return second >= 0x90
	case 0xf4:
		return second <= 0x8f
	default:
		return true
	}
}

func continuationByte(value byte) bool { return value >= 0x80 && value <= 0xbf }
