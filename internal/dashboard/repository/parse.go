package repository

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"sort"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/Beamfall/corvint/internal/contextindex"
)

var errMalformed = errors.New("malformed repository authority output")

func strictLFFields(raw []byte, count int) ([]string, error) {
	if count <= 0 || len(raw) == 0 || raw[len(raw)-1] != '\n' ||
		bytes.Count(raw, []byte{'\n'}) != count || bytes.ContainsAny(raw, "\r\x00") {
		return nil, errMalformed
	}
	fields := bytes.Split(raw[:len(raw)-1], []byte{'\n'})
	if len(fields) != count {
		return nil, errMalformed
	}
	result := make([]string, count)
	for index, field := range fields {
		if len(field) == 0 || !asciiField(field) {
			return nil, errMalformed
		}
		result[index] = string(field)
	}
	return result, nil
}

func strictUTF8LFFields(raw []byte, count int) ([]string, error) {
	if count <= 0 || len(raw) == 0 || raw[len(raw)-1] != '\n' ||
		bytes.Count(raw, []byte{'\n'}) != count || bytes.ContainsAny(raw, "\r\x00") || !utf8.Valid(raw) {
		return nil, errMalformed
	}
	fields := bytes.Split(raw[:len(raw)-1], []byte{'\n'})
	if len(fields) != count {
		return nil, errMalformed
	}
	result := make([]string, count)
	for index, field := range fields {
		if len(field) == 0 {
			return nil, errMalformed
		}
		for _, character := range string(field) {
			if character == 0 || character == '\r' || character == '\n' {
				return nil, errMalformed
			}
		}
		result[index] = string(field)
	}
	return result, nil
}

func asciiField(value []byte) bool {
	for _, character := range value {
		if character < 0x20 || character > 0x7e {
			return false
		}
	}
	return true
}

func validObjectID(value, format string) bool {
	length := 40
	if format == "sha256" {
		length = 64
	} else if format != "sha1" {
		return false
	}
	if len(value) != length {
		return false
	}
	for index := range value {
		if (value[index] < '0' || value[index] > '9') && (value[index] < 'a' || value[index] > 'f') {
			return false
		}
	}
	return true
}

func parseIdentity(raw []byte) (format, head, tree string, err error) {
	fields, parseErr := strictLFFields(raw, 3)
	if parseErr != nil || (fields[0] != "sha1" && fields[0] != "sha256") ||
		!validObjectID(fields[1], fields[0]) || !validObjectID(fields[2], fields[0]) {
		return "", "", "", errMalformed
	}
	return fields[0], fields[1], fields[2], nil
}

func parseSingleObject(raw []byte, format string) (string, error) {
	fields, err := strictLFFields(raw, 1)
	if err != nil || !validObjectID(fields[0], format) {
		return "", errMalformed
	}
	return fields[0], nil
}

func parseCount(raw []byte) (uint64, error) {
	fields, err := strictLFFields(raw, 1)
	if err != nil || fields[0] == "" || (len(fields[0]) > 1 && fields[0][0] == '0') {
		return 0, errMalformed
	}
	value, parseErr := strconv.ParseUint(fields[0], 10, 64)
	if parseErr != nil {
		return 0, errMalformed
	}
	return value, nil
}

func parseDiscovery(raw []byte) ([3]string, error) {
	var result [3]string
	fields, err := strictUTF8LFFields(raw, 3)
	if err != nil {
		return result, err
	}
	for index, field := range fields {
		if !validNativeAbsolutePath(field) {
			return result, errMalformed
		}
		result[index] = field
	}
	return result, nil
}

func parseDirtyPaths(raw []byte) ([]string, error) {
	paths := make(map[string]struct{})
	for offset := 0; offset < len(raw); {
		field, next, ok := nextNUL(raw, offset)
		if !ok || len(field) < 4 || field[2] != ' ' || !validStatusPair(field[:2]) {
			return nil, errMalformed
		}
		offset = next
		pathValue := string(field[3:])
		if !validTracePath(pathValue) {
			return nil, errMalformed
		}
		paths[pathValue] = struct{}{}
		renameOrCopy := field[0] == 'R' || field[0] == 'C' || field[1] == 'R' || field[1] == 'C'
		if renameOrCopy {
			source, sourceNext, sourceOK := nextNUL(raw, offset)
			if !sourceOK || !validTracePath(string(source)) {
				return nil, errMalformed
			}
			offset = sourceNext
			paths[string(source)] = struct{}{}
		}
		if len(paths) > maxDirtyPaths {
			return nil, errMalformed
		}
	}
	result := make([]string, 0, len(paths))
	for value := range paths {
		result = append(result, value)
	}
	sort.Slice(result, func(left, right int) bool {
		return bytes.Compare([]byte(result[left]), []byte(result[right])) < 0
	})
	return result, nil
}

func validStatusPair(value []byte) bool {
	if len(value) != 2 {
		return false
	}
	if value[0] == ' ' && value[1] == ' ' {
		return false
	}
	if value[0] == '?' || value[1] == '?' {
		return value[0] == '?' && value[1] == '?'
	}
	for _, character := range value {
		if !strings.ContainsRune(" MTADRCU", rune(character)) {
			return false
		}
	}
	return true
}

func nextNUL(raw []byte, offset int) ([]byte, int, bool) {
	if offset >= len(raw) {
		return nil, offset, false
	}
	relative := bytes.IndexByte(raw[offset:], 0)
	if relative < 0 {
		return nil, offset, false
	}
	end := offset + relative
	return raw[offset:end], end + 1, true
}

func dirtyDigest(paths []string) (string, error) {
	canonical, err := contextindex.CanonicalJSON(paths)
	if err != nil {
		return "", errMalformed
	}
	hash := sha256.New()
	_, _ = hash.Write([]byte("corvint-dashboard-dirty-paths/0"))
	_, _ = hash.Write([]byte{0})
	_, _ = hash.Write(canonical)
	return "sha256:" + hex.EncodeToString(hash.Sum(nil)), nil
}

type treeObject struct {
	objectID string
	path     string
}

func parseTreeObjects(raw []byte, requested []string, format string) ([]treeObject, bool, error) {
	requestedSet := make(map[string]struct{}, len(requested))
	for _, value := range requested {
		requestedSet[value] = struct{}{}
	}
	objects := make([]treeObject, 0, len(requested))
	seen := make(map[string]struct{}, len(requested))
	for offset := 0; offset < len(raw); {
		row, next, ok := nextNUL(raw, offset)
		if !ok || len(row) == 0 || !utf8.Valid(row) {
			return nil, false, errMalformed
		}
		offset = next
		tab := bytes.IndexByte(row, '\t')
		if tab <= 0 || bytes.IndexByte(row[tab+1:], '\t') >= 0 {
			return nil, false, errMalformed
		}
		header := bytes.Split(row[:tab], []byte{' '})
		if len(header) != 3 || len(header[0]) == 0 || len(header[1]) == 0 || len(header[2]) == 0 {
			return nil, false, errMalformed
		}
		pathValue := string(row[tab+1:])
		if _, wanted := requestedSet[pathValue]; !wanted {
			return nil, false, errMalformed
		}
		if _, duplicate := seen[pathValue]; duplicate {
			return nil, false, errMalformed
		}
		seen[pathValue] = struct{}{}
		mode, objectType, objectID := string(header[0]), string(header[1]), string(header[2])
		if !validObjectID(objectID, format) {
			return nil, false, errMalformed
		}
		if (mode != "100644" && mode != "100755") || objectType != "blob" {
			return nil, false, nil
		}
		objects = append(objects, treeObject{objectID: objectID, path: pathValue})
	}
	if len(seen) != len(requestedSet) {
		return nil, false, nil
	}
	return objects, true, nil
}
