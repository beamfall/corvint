package trace

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"
)

var requiredFields = map[string]struct{}{
	"schema_version": {}, "revision": {}, "trace_id": {}, "task": {},
	"opened_paths": {}, "changed_paths": {}, "verification": {}, "outcome": {},
}

type recordJSONError struct{ cause error }

func (err *recordJSONError) Error() string { return err.cause.Error() }
func (err *recordJSONError) Unwrap() error { return err.cause }

// Encode returns the exact Python-compatible canonical JSONL bytes for record.
func Encode(record Record) ([]byte, error) {
	if record.SchemaVersion != SchemaVersion {
		return nil, fmt.Errorf("unsupported local trace schema")
	}
	expected, err := traceID(record)
	if err != nil {
		return nil, err
	}
	if record.TraceID != expected {
		return nil, fmt.Errorf("local trace digest mismatch")
	}
	encoded, err := canonicalRecord(record, true)
	if err != nil {
		return nil, err
	}
	encoded = append(encoded, '\n')
	if len(encoded) > MaxTraceRowBytes {
		return nil, fmt.Errorf("local trace row exceeds %d bytes", MaxTraceRowBytes)
	}
	return encoded, nil
}

// DecodeStore validates one revision file and preserves its row order.
func DecodeStore(data []byte, revision string, trackedPaths []string) ([]Record, error) {
	lines, err := splitRows(data, MaxTraces, MaxTraceStoreBytes, revision+".jsonl")
	if err != nil {
		return nil, err
	}
	tracked := stringSet(trackedPaths)
	records := make([]Record, 0, len(lines))
	ids := make(map[string]struct{}, len(lines))
	for number, line := range lines {
		record, err := decodeRecord(line)
		if err != nil {
			var syntaxError *recordJSONError
			if errors.As(err, &syntaxError) {
				return nil, fmt.Errorf("invalid local trace JSON in %s.jsonl at line %d", revision, number+1)
			}
			return nil, err
		}
		record, err = normalizeStoredRecord(record, revision, tracked)
		if err != nil {
			return nil, err
		}
		if _, duplicate := ids[record.TraceID]; duplicate {
			return nil, fmt.Errorf("local trace store contains duplicate trace ids: %s.jsonl", revision)
		}
		ids[record.TraceID] = struct{}{}
		records = append(records, record)
	}
	return records, nil
}

// TransformLegacy rewrites validated tree-named rows to a commit revision.
func TransformLegacy(data []byte, treeRevision, commitRevision string, trackedPaths []string) ([]byte, error) {
	records, err := DecodeStore(data, treeRevision, trackedPaths)
	if err != nil {
		return nil, err
	}
	var output bytes.Buffer
	for _, record := range records {
		record.Revision = commitRevision
		record.TraceID, err = traceID(record)
		if err != nil {
			return nil, err
		}
		row, err := Encode(record)
		if err != nil {
			return nil, err
		}
		if output.Len()+len(row) > MaxTraceStoreBytes {
			return nil, fmt.Errorf("local trace store exceeds %d bytes", MaxTraceStoreBytes)
		}
		output.Write(row)
	}
	return output.Bytes(), nil
}

func decodeRecord(data []byte) (Record, error) {
	if !utf8.Valid(data) {
		return Record{}, fmt.Errorf("invalid local trace encoding")
	}
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return Record{}, &recordJSONError{cause: err}
	}
	if raw == nil || len(raw) != len(requiredFields) {
		return Record{}, fmt.Errorf("invalid local trace fields")
	}
	for name := range raw {
		if _, ok := requiredFields[name]; !ok {
			return Record{}, fmt.Errorf("invalid local trace fields")
		}
	}
	schema, err := decodePythonSchema(raw["schema_version"])
	if err != nil {
		return Record{}, fmt.Errorf("unsupported local trace schema")
	}
	revision, err := decodePythonString(raw["revision"])
	if err != nil {
		return Record{}, err
	}
	traceID, err := decodePythonString(raw["trace_id"])
	if err != nil {
		return Record{}, err
	}
	task, err := decodePythonString(raw["task"])
	if err != nil {
		return Record{}, err
	}
	opened, err := decodeStringArray(raw["opened_paths"])
	if err != nil {
		return Record{}, err
	}
	changed, err := decodeStringArray(raw["changed_paths"])
	if err != nil {
		return Record{}, err
	}
	verification, err := decodeStringArray(raw["verification"])
	if err != nil {
		return Record{}, err
	}
	outcome, err := decodePythonString(raw["outcome"])
	if err != nil {
		return Record{}, err
	}
	return Record{schema, revision, traceID, task, opened, changed, verification, outcome}, nil
}

func decodePythonSchema(raw []byte) (int, error) {
	value := strings.TrimSpace(string(raw))
	if value == "true" {
		return SchemaVersion, nil
	}
	if value == "" || value == "false" || value == "null" {
		return 0, fmt.Errorf("not schema 1")
	}
	if !strings.ContainsAny(value, ".eE") {
		integer, err := strconv.ParseInt(value, 10, 64)
		if err == nil && integer == 1 {
			return SchemaVersion, nil
		}
		return 0, fmt.Errorf("not schema 1")
	}
	number, err := strconv.ParseFloat(value, 64)
	if err != nil || number != 1 {
		return 0, fmt.Errorf("not schema 1")
	}
	return SchemaVersion, nil
}

func decodeStringArray(raw []byte) ([]string, error) {
	var values []json.RawMessage
	if err := json.Unmarshal(raw, &values); err != nil || values == nil {
		return nil, fmt.Errorf("trace field must be a list of strings")
	}
	result := make([]string, len(values))
	for index, value := range values {
		decoded, err := decodePythonString(value)
		if err != nil {
			return nil, fmt.Errorf("trace field must be a list of strings")
		}
		result[index] = decoded
	}
	return result, nil
}

func decodePythonString(raw []byte) (string, error) {
	if len(raw) < 2 || raw[0] != '"' || raw[len(raw)-1] != '"' {
		return "", fmt.Errorf("trace field must be a string")
	}
	var output bytes.Buffer
	for index := 1; index < len(raw)-1; {
		if raw[index] != '\\' {
			start := index
			for index < len(raw)-1 && raw[index] != '\\' {
				index++
			}
			output.Write(raw[start:index])
			continue
		}
		index++
		if index >= len(raw)-1 {
			return "", fmt.Errorf("invalid JSON string")
		}
		switch raw[index] {
		case '"', '\\', '/':
			output.WriteByte(raw[index])
			index++
		case 'b':
			output.WriteByte('\b')
			index++
		case 'f':
			output.WriteByte('\f')
			index++
		case 'n':
			output.WriteByte('\n')
			index++
		case 'r':
			output.WriteByte('\r')
			index++
		case 't':
			output.WriteByte('\t')
			index++
		case 'u':
			value, next, err := decodeHexEscape(raw, index)
			if err != nil {
				return "", err
			}
			index = next
			if value >= 0xd800 && value <= 0xdbff && index+6 <= len(raw)-1 && raw[index] == '\\' && raw[index+1] == 'u' {
				low, after, lowErr := decodeHexEscape(raw, index+1)
				if lowErr == nil && low >= 0xdc00 && low <= 0xdfff {
					combined := rune(0x10000 + (value-0xd800)*0x400 + (low - 0xdc00))
					output.WriteRune(combined)
					index = after
					continue
				}
			}
			if value >= 0xd800 && value <= 0xdfff {
				writeWTF8(&output, value)
				continue
			}
			output.WriteRune(rune(value))
		default:
			return "", fmt.Errorf("invalid JSON string escape")
		}
	}
	return output.String(), nil
}

func decodeHexEscape(raw []byte, uIndex int) (int, int, error) {
	if uIndex+5 > len(raw) {
		return 0, 0, fmt.Errorf("invalid JSON unicode escape")
	}
	value := 0
	for _, digit := range raw[uIndex+1 : uIndex+5] {
		value <<= 4
		switch {
		case digit >= '0' && digit <= '9':
			value += int(digit - '0')
		case digit >= 'a' && digit <= 'f':
			value += int(digit-'a') + 10
		case digit >= 'A' && digit <= 'F':
			value += int(digit-'A') + 10
		default:
			return 0, 0, fmt.Errorf("invalid JSON unicode escape")
		}
	}
	return value, uIndex + 5, nil
}

func writeWTF8(output *bytes.Buffer, value int) {
	output.WriteByte(byte(0xe0 | value>>12))
	output.WriteByte(byte(0x80 | value>>6&0x3f))
	output.WriteByte(byte(0x80 | value&0x3f))
}

func splitRows(data []byte, remainingRows, remainingBytes int, label string) ([][]byte, error) {
	rows := make([][]byte, 0)
	consumed := 0
	for len(data) != 0 {
		end := len(data)
		if newline := bytes.IndexByte(data, '\n'); newline >= 0 {
			end = newline + 1
		}
		line := data[:end]
		if len(line) > MaxTraceRowBytes {
			return nil, fmt.Errorf("local trace row in %s exceeds %d bytes", label, MaxTraceRowBytes)
		}
		consumed += len(line)
		if consumed > remainingBytes {
			return nil, fmt.Errorf("local trace store exceeds %d bytes", MaxTraceStoreBytes)
		}
		if len(rows) >= remainingRows {
			return nil, fmt.Errorf("local trace store exceeds %d rows", MaxTraces)
		}
		if !utf8.Valid(line) {
			return nil, fmt.Errorf("invalid local trace encoding in %s", label)
		}
		value := line
		for len(value) != 0 && (value[len(value)-1] == '\r' || value[len(value)-1] == '\n') {
			value = value[:len(value)-1]
		}
		rows = append(rows, value)
		data = data[end:]
	}
	return rows, nil
}

func canonicalRecord(record Record, includeID bool) ([]byte, error) {
	var output bytes.Buffer
	output.WriteByte('{')
	writeMemberName(&output, "changed_paths", false)
	if err := writeStringArray(&output, record.ChangedPaths); err != nil {
		return nil, err
	}
	writeMemberName(&output, "opened_paths", true)
	if err := writeStringArray(&output, record.OpenedPaths); err != nil {
		return nil, err
	}
	writeMemberName(&output, "outcome", true)
	if err := writePythonString(&output, record.Outcome); err != nil {
		return nil, err
	}
	writeMemberName(&output, "revision", true)
	if err := writePythonString(&output, record.Revision); err != nil {
		return nil, err
	}
	writeMemberName(&output, "schema_version", true)
	output.WriteByte('1')
	writeMemberName(&output, "task", true)
	if err := writePythonString(&output, record.Task); err != nil {
		return nil, err
	}
	if includeID {
		writeMemberName(&output, "trace_id", true)
		if err := writePythonString(&output, record.TraceID); err != nil {
			return nil, err
		}
	}
	writeMemberName(&output, "verification", true)
	if err := writeStringArray(&output, record.Verification); err != nil {
		return nil, err
	}
	output.WriteByte('}')
	return output.Bytes(), nil
}

func canonicalSemantic(record Record) ([]byte, error) {
	var output bytes.Buffer
	output.WriteByte('{')
	writeMemberName(&output, "changed_paths", false)
	if err := writeStringArray(&output, record.ChangedPaths); err != nil {
		return nil, err
	}
	writeMemberName(&output, "opened_paths", true)
	if err := writeStringArray(&output, record.OpenedPaths); err != nil {
		return nil, err
	}
	writeMemberName(&output, "outcome", true)
	if err := writePythonString(&output, record.Outcome); err != nil {
		return nil, err
	}
	writeMemberName(&output, "schema_version", true)
	output.WriteByte('1')
	writeMemberName(&output, "task", true)
	if err := writePythonString(&output, record.Task); err != nil {
		return nil, err
	}
	writeMemberName(&output, "verification", true)
	if err := writeStringArray(&output, record.Verification); err != nil {
		return nil, err
	}
	output.WriteByte('}')
	return output.Bytes(), nil
}

func writeMemberName(output *bytes.Buffer, name string, comma bool) {
	if comma {
		output.WriteByte(',')
	}
	output.WriteByte('"')
	output.WriteString(name)
	output.WriteString("\":")
}

func writeStringArray(output *bytes.Buffer, values []string) error {
	output.WriteByte('[')
	for index, value := range values {
		if index != 0 {
			output.WriteByte(',')
		}
		if err := writePythonString(output, value); err != nil {
			return err
		}
	}
	output.WriteByte(']')
	return nil
}

func writePythonString(output *bytes.Buffer, value string) error {
	units, err := pythonUnits(value)
	if err != nil {
		return fmt.Errorf("trace string is not valid Python text: %w", err)
	}
	output.WriteByte('"')
	for _, unit := range units {
		character := unit.value
		switch character {
		case '"', '\\':
			output.WriteByte('\\')
			output.WriteByte(byte(character))
		case '\b':
			output.WriteString(`\b`)
		case '\t':
			output.WriteString(`\t`)
		case '\n':
			output.WriteString(`\n`)
		case '\f':
			output.WriteString(`\f`)
		case '\r':
			output.WriteString(`\r`)
		default:
			switch {
			case character < 0x20 || character == 0x7f:
				writeUnicodeEscape(output, character)
			case character < 0x80:
				output.WriteByte(byte(character))
			case character <= 0xffff:
				writeUnicodeEscape(output, character)
			default:
				value := character - 0x10000
				writeUnicodeEscape(output, 0xd800+value>>10)
				writeUnicodeEscape(output, 0xdc00+value&0x3ff)
			}
		}
	}
	output.WriteByte('"')
	return nil
}

func writeUnicodeEscape(output *bytes.Buffer, value int) {
	const hexadecimal = "0123456789abcdef"
	output.WriteString(`\u`)
	output.WriteByte(hexadecimal[value>>12&0x0f])
	output.WriteByte(hexadecimal[value>>8&0x0f])
	output.WriteByte(hexadecimal[value>>4&0x0f])
	output.WriteByte(hexadecimal[value&0x0f])
}

type pythonUnit struct {
	value int
	start int
	end   int
}

func pythonUnits(value string) ([]pythonUnit, error) {
	units := make([]pythonUnit, 0, len(value))
	for index := 0; index < len(value); {
		if index+3 <= len(value) && value[index] == 0xed && value[index+1] >= 0xa0 && value[index+1] <= 0xbf && value[index+2] >= 0x80 && value[index+2] <= 0xbf {
			codepoint := int(value[index]&0x0f)<<12 | int(value[index+1]&0x3f)<<6 | int(value[index+2]&0x3f)
			units = append(units, pythonUnit{codepoint, index, index + 3})
			index += 3
			continue
		}
		character, size := utf8.DecodeRuneInString(value[index:])
		if character == utf8.RuneError && size == 1 {
			return nil, fmt.Errorf("invalid UTF-8 at byte %d", index)
		}
		units = append(units, pythonUnit{int(character), index, index + size})
		index += size
	}
	return units, nil
}

func pythonString(value string) bool {
	_, err := pythonUnits(value)
	return err == nil
}

func trimPythonSpace(value string) (string, int, error) {
	units, err := pythonUnits(value)
	if err != nil {
		return "", 0, err
	}
	first, last := 0, len(units)
	for first < last && pythonSpace(units[first].value) {
		first++
	}
	for last > first && pythonSpace(units[last-1].value) {
		last--
	}
	if first == last {
		return "", 0, nil
	}
	return value[units[first].start:units[last-1].end], last - first, nil
}

func pythonSpace(value int) bool {
	return value >= 0x09 && value <= 0x0d || value >= 0x1c && value <= 0x20 ||
		value == 0x85 || value == 0xa0 || value == 0x1680 || value >= 0x2000 && value <= 0x200a ||
		value == 0x2028 || value == 0x2029 || value == 0x202f || value == 0x205f || value == 0x3000
}

const maxPathBytes = 4_096

// witnessPath applies the lexical limits of corvint-dashboard-trace-path-witness/0
// that pythonString and the normalization check do not already cover: at most
// 4,096 bytes, valid UTF-8 (no encoded surrogate), no Unicode control, no
// backslash, and no drive-letter prefix (decision 0235).
func witnessPath(value string) bool {
	if len(value) > maxPathBytes || !utf8.ValidString(value) || strings.Contains(value, "\\") || strings.IndexFunc(value, unicode.IsControl) >= 0 {
		return false
	}
	return !hasVolumePrefix(value)
}

func hasVolumePrefix(value string) bool {
	if len(value) < 2 || value[1] != ':' {
		return false
	}
	letter := value[0] | 0x20
	return letter >= 'a' && letter <= 'z'
}

// validTask refuses a screened trace task that is not valid UTF-8: the Python
// string decoding admits an encoded surrogate, which the dashboard trace
// adapter refuses, so the recorder and the store reader refuse it too
// (decision 0246).
func validTask(task string, err error) (string, error) {
	if err != nil {
		return "", err
	}
	if !utf8.ValidString(task) {
		return "", fmt.Errorf("trace task must be valid UTF-8")
	}
	return task, nil
}
