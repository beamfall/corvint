package gokernel

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strconv"
	"unicode/utf8"
)

// CanonicalJSON matches the compact, sorted, UTF-8 JSON used by the Python
// harness protocol. It intentionally returns no terminal newline so the bytes
// are suitable for receipt hashing.
func CanonicalJSON(value any) ([]byte, error) {
	var output bytes.Buffer
	encoder := json.NewEncoder(&output)
	encoder.SetEscapeHTML(false)
	if err := encoder.Encode(value); err != nil {
		return nil, fmt.Errorf("canonical JSON: %w", err)
	}
	encoded := bytes.TrimSuffix(output.Bytes(), []byte{'\n'})
	// encoding/json always quotes the two JavaScript separators. Python's
	// ensure_ascii=False does not, so restore their canonical UTF-8 spelling.
	encoded = restoreJSONSeparators(encoded)
	return encoded, nil
}

// WriteCanonicalJSON writes the CanonicalJSON bytes of value and one newline
// to output without first holding them in a second buffer, so a large receipt
// costs one encoder buffer instead of two copies (AFP-V0-398). The bytes are
// identical to CanonicalJSON(value) followed by "\n". IsCanonicalOutputFailure
// reports a failure of output, after which a prefix of the document may
// already have been written; any other error is an encoding failure, which
// writes nothing.
func WriteCanonicalJSON(output io.Writer, value any) error {
	restorer := &separatorWriter{output: output}
	encoder := json.NewEncoder(restorer)
	encoder.SetEscapeHTML(false)
	err := encoder.Encode(value)
	if err == nil {
		err = restorer.flush()
	}
	switch {
	case err == nil:
		return nil
	case restorer.failed:
		return &canonicalOutputError{cause: err}
	default:
		return fmt.Errorf("canonical JSON: %w", err)
	}
}

// canonicalOutputError is unexported so that no marshaler's error can pass
// for an output failure.
type canonicalOutputError struct{ cause error }

func (failure *canonicalOutputError) Error() string {
	return "canonical JSON output failed: " + failure.cause.Error()
}

func (failure *canonicalOutputError) Unwrap() error { return failure.cause }

// IsCanonicalOutputFailure reports whether a WriteCanonicalJSON error came
// from its output writer rather than from encoding.
func IsCanonicalOutputFailure(err error) bool {
	var failure *canonicalOutputError
	return errors.As(err, &failure)
}

// separatorWriter restores the two JavaScript separators in a stream, as
// restoreJSONSeparators does, writing the bytes between escapes straight
// through. A trailing backslash run that is not yet followed by five bytes is
// held back, so an escape split across writes is decided only once complete.
type separatorWriter struct {
	output  io.Writer
	pending []byte
	failed  bool
}

func (writer *separatorWriter) Write(chunk []byte) (int, error) {
	data := chunk
	if len(writer.pending) > 0 {
		data = append(writer.pending, chunk...)
		writer.pending = nil
	}
	cut := undecidedEscapeStart(data)
	if err := writer.emit(data[:cut]); err != nil {
		return 0, err
	}
	writer.pending = append([]byte(nil), data[cut:]...)
	return len(chunk), nil
}

func (writer *separatorWriter) flush() error {
	pending := writer.pending
	writer.pending = nil
	return writer.emit(pending)
}

// emit writes data with every escape that an odd backslash run starts, and
// that names U+2028 or U+2029, replaced by the separator's UTF-8 bytes. data
// never begins inside a backslash run: Write cuts only before a run or after
// a decided one.
func (writer *separatorWriter) emit(data []byte) error {
	written := 0
	for from := 0; ; {
		found := bytes.Index(data[from:], []byte(`\u202`))
		if found < 0 {
			break
		}
		at := from + found
		from = at + 1
		if at+6 > len(data) {
			break
		}
		var separator string
		switch data[at+5] {
		case '8':
			separator = "\u2028"
		case '9':
			separator = "\u2029"
		default:
			continue
		}
		run := 0
		for index := at; index >= 0 && data[index] == '\\'; index-- {
			run++
		}
		if run%2 == 0 {
			continue
		}
		if err := writer.write(data[written:at]); err != nil {
			return err
		}
		if err := writer.write([]byte(separator)); err != nil {
			return err
		}
		written = at + 6
		from = written
	}
	return writer.write(data[written:])
}

func (writer *separatorWriter) write(data []byte) error {
	if len(data) == 0 {
		return nil
	}
	if _, err := writer.output.Write(data); err != nil {
		writer.failed = true
		return err
	}
	return nil
}

// undecidedEscapeStart returns where the final backslash run starts when fewer
// than five bytes follow it, and len(data) otherwise.
func undecidedEscapeStart(data []byte) int {
	last := bytes.LastIndexByte(data, '\\')
	if last < 0 || last+6 <= len(data) {
		return len(data)
	}
	start := last
	for start > 0 && data[start-1] == '\\' {
		start--
	}
	return start
}

func restoreJSONSeparators(encoded []byte) []byte {
	// Most documents carry neither escape; returning them unchanged avoids a
	// second copy of a large receipt. The capacity is clipped so a caller's
	// append still copies instead of writing into the encoder's spare buffer.
	if !bytes.Contains(encoded, []byte(`\u202`)) {
		return encoded[:len(encoded):len(encoded)]
	}
	output := make([]byte, 0, len(encoded))
	for index := 0; index < len(encoded); {
		if encoded[index] != '\\' {
			output = append(output, encoded[index])
			index++
			continue
		}
		start := index
		for index < len(encoded) && encoded[index] == '\\' {
			index++
		}
		run := index - start
		separator := []byte(nil)
		if run%2 == 1 && index+5 <= len(encoded) {
			switch string(encoded[index : index+5]) {
			case "u2028":
				separator = []byte("\u2028")
			case "u2029":
				separator = []byte("\u2029")
			}
		}
		if separator == nil {
			output = append(output, encoded[start:index]...)
			continue
		}
		output = append(output, encoded[start:index-1]...)
		output = append(output, separator...)
		index += 5
	}
	return output
}

func decodeStrictJSON(raw []byte) (map[string]any, error) {
	if !utf8.Valid(raw) {
		return nil, newError("invalid-harness-input", "harness input must be one JSON object")
	}
	if err := rejectUnpairedEscapedSurrogates(raw); err != nil {
		return nil, err
	}
	if containsInvalidJSONConstant(raw) {
		return nil, newError("invalid-harness-input", "harness input contains a non-JSON value")
	}
	if exceedsJSONNesting(raw) {
		return nil, newError("invalid-harness-input", "harness input exceeds its nesting limit")
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	value, err := decodeValue(decoder, 0)
	if err != nil {
		return nil, err
	}
	if _, err := decoder.Token(); err != io.EOF {
		if err == nil {
			return nil, newError("invalid-harness-input", "harness input must be one JSON object")
		}
		return nil, newError("invalid-harness-input", "harness input must be one JSON object")
	}
	object, ok := value.(map[string]any)
	if !ok {
		return nil, newError("invalid-harness-input", "harness input must be one JSON object")
	}
	return object, nil
}

func exceedsJSONNesting(raw []byte) bool {
	inString := false
	escaped := false
	depth := 0
	for _, value := range raw {
		if inString {
			if escaped {
				escaped = false
			} else if value == '\\' {
				escaped = true
			} else if value == '"' {
				inString = false
			}
			continue
		}
		switch value {
		case '"':
			inString = true
		case '{', '[':
			depth++
			if depth > maxJSONDepth+1 {
				return true
			}
		case '}', ']':
			if depth > 0 {
				depth--
			}
		}
	}
	return false
}

func containsInvalidJSONConstant(raw []byte) bool {
	inString := false
	escaped := false
	for index := 0; index < len(raw); index++ {
		if inString {
			if escaped {
				escaped = false
				continue
			}
			if raw[index] == '\\' {
				escaped = true
			} else if raw[index] == '"' {
				inString = false
			}
			continue
		}
		if raw[index] == '"' {
			inString = true
			continue
		}
		for _, constant := range []string{"NaN", "Infinity", "-Infinity"} {
			end := index + len(constant)
			if end <= len(raw) && string(raw[index:end]) == constant &&
				(index == 0 || isJSONBoundary(raw[index-1])) &&
				(end == len(raw) || isJSONBoundary(raw[end])) {
				return true
			}
		}
	}
	return false
}

func isJSONBoundary(value byte) bool {
	switch value {
	case ' ', '\t', '\r', '\n', ':', ',', '[', ']', '{', '}':
		return true
	default:
		return false
	}
}

func rejectUnpairedEscapedSurrogates(raw []byte) error {
	inString := false
	for index := 0; index < len(raw); index++ {
		switch raw[index] {
		case '"':
			inString = !inString
		case '\\':
			if !inString || index+1 >= len(raw) {
				continue
			}
			if raw[index+1] != 'u' {
				index++
				continue
			}
			if index+6 > len(raw) {
				continue // the JSON decoder reports the malformed escape
			}
			value, err := strconv.ParseUint(string(raw[index+2:index+6]), 16, 16)
			if err != nil {
				continue // the JSON decoder reports the malformed escape
			}
			switch {
			case value >= 0xd800 && value <= 0xdbff:
				if index+12 > len(raw) || raw[index+6] != '\\' || raw[index+7] != 'u' {
					return newError("invalid-harness-input", "harness JSON is not canonicalizable")
				}
				low, lowErr := strconv.ParseUint(string(raw[index+8:index+12]), 16, 16)
				if lowErr != nil || low < 0xdc00 || low > 0xdfff {
					return newError("invalid-harness-input", "harness JSON is not canonicalizable")
				}
				index += 11
			case value >= 0xdc00 && value <= 0xdfff:
				return newError("invalid-harness-input", "harness JSON is not canonicalizable")
			default:
				index += 5
			}
		}
	}
	return nil
}

const maxJSONDepth = 256

func decodeValue(decoder *json.Decoder, depth int) (any, error) {
	if depth > maxJSONDepth {
		return nil, newError("invalid-harness-input", "harness input exceeds its nesting limit")
	}
	token, err := decoder.Token()
	if err != nil {
		return nil, newError("invalid-harness-input", "harness input must be one JSON object")
	}
	delimiter, composite := token.(json.Delim)
	if !composite {
		return token, nil
	}
	switch delimiter {
	case '{':
		object := make(map[string]any)
		for decoder.More() {
			keyToken, err := decoder.Token()
			if err != nil {
				return nil, newError("invalid-harness-input", "harness input must be one JSON object")
			}
			key, ok := keyToken.(string)
			if !ok {
				return nil, newError("invalid-harness-input", "harness object key must be a string")
			}
			if _, exists := object[key]; exists {
				return nil, newError("invalid-harness-input", "harness input has duplicate keys")
			}
			value, err := decodeValue(decoder, depth+1)
			if err != nil {
				return nil, err
			}
			object[key] = value
		}
		end, err := decoder.Token()
		if err != nil || end != json.Delim('}') {
			return nil, newError("invalid-harness-input", "harness input must be one JSON object")
		}
		return object, nil
	case '[':
		array := make([]any, 0)
		for decoder.More() {
			value, err := decodeValue(decoder, depth+1)
			if err != nil {
				return nil, err
			}
			array = append(array, value)
		}
		end, err := decoder.Token()
		if err != nil || end != json.Delim(']') {
			return nil, newError("invalid-harness-input", "harness input must be one JSON object")
		}
		return array, nil
	default:
		return nil, newError("invalid-harness-input", "harness input must be one JSON object")
	}
}
