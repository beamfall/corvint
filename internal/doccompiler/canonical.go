package doccompiler

import (
	"bytes"
	"encoding/json"
	"sort"
	"strconv"
	"unicode/utf8"
)

// MaxCanonicalJSONBytes is the HDCV0 limit on one canonical plan or receipt.
const MaxCanonicalJSONBytes = 8 << 20

// VerifyCanonicalJSON accepts raw only when it is exactly the HDCV0-041
// canonical encoding of its own parsed value, including the trailing LF.
func VerifyCanonicalJSON(raw []byte) error {
	if len(raw) > MaxCanonicalJSONBytes {
		return failure("canonical-json-too-large", "canonical JSON exceeds %d bytes", MaxCanonicalJSONBytes)
	}
	if !utf8.Valid(raw) {
		return failure("canonical-json-invalid-utf8", "canonical JSON is not valid UTF-8")
	}
	if len(raw) == 0 || raw[len(raw)-1] != '\n' {
		return failure("canonical-json-missing-final-lf", "canonical JSON does not end with one LF")
	}
	body := raw[:len(raw)-1]
	// The token walk inherits the standard scanner's 10,000-level nesting bound.
	decoder := json.NewDecoder(bytes.NewReader(body))
	decoder.UseNumber()
	value, err := decodeCanonicalValue(decoder)
	if err != nil {
		return err
	}
	var encoded bytes.Buffer
	encodeCanonicalValue(&encoded, value)
	if !bytes.Equal(encoded.Bytes(), body) {
		return failure("canonical-json-not-canonical", "bytes differ from the canonical encoding of their value")
	}
	return nil
}

func decodeCanonicalValue(decoder *json.Decoder) (any, error) {
	token, err := decoder.Token()
	if err != nil {
		return nil, failure("canonical-json-syntax", "canonical JSON is not one well-formed value")
	}
	switch typed := token.(type) {
	case json.Delim:
		if typed == '{' {
			return decodeCanonicalObject(decoder)
		}
		return decodeCanonicalArray(decoder)
	case json.Number:
		integer, err := strconv.ParseInt(typed.String(), 10, 64)
		if err != nil {
			return nil, failure("canonical-json-non-integer", "number %s is not a signed 64-bit integer", typed)
		}
		return integer, nil
	default:
		return typed, nil
	}
}

func decodeCanonicalObject(decoder *json.Decoder) (any, error) {
	object := map[string]any{}
	for decoder.More() {
		token, err := decoder.Token()
		if err != nil {
			return nil, failure("canonical-json-syntax", "canonical JSON is not one well-formed value")
		}
		key := token.(string)
		if _, duplicate := object[key]; duplicate {
			return nil, failure("canonical-json-duplicate-key", "object key %q is duplicated", key)
		}
		value, err := decodeCanonicalValue(decoder)
		if err != nil {
			return nil, err
		}
		object[key] = value
	}
	_, err := decoder.Token()
	return object, syntaxFailure(err)
}

func decodeCanonicalArray(decoder *json.Decoder) (any, error) {
	array := []any{}
	for decoder.More() {
		value, err := decodeCanonicalValue(decoder)
		if err != nil {
			return nil, err
		}
		array = append(array, value)
	}
	_, err := decoder.Token()
	return array, syntaxFailure(err)
}

func syntaxFailure(err error) error {
	if err == nil {
		return nil
	}
	return failure("canonical-json-syntax", "canonical JSON is not one well-formed value")
}

func encodeCanonicalValue(output *bytes.Buffer, value any) {
	switch typed := value.(type) {
	case map[string]any:
		encodeCanonicalObject(output, typed)
	case []any:
		encodeCanonicalArray(output, typed)
	case string:
		encodeCanonicalString(output, typed)
	case int64:
		output.WriteString(strconv.FormatInt(typed, 10))
	case bool:
		output.WriteString(strconv.FormatBool(typed))
	default:
		output.WriteString("null")
	}
}

func encodeCanonicalObject(output *bytes.Buffer, object map[string]any) {
	keys := make([]string, 0, len(object))
	for key := range object {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	output.WriteByte('{')
	for index, key := range keys {
		if index > 0 {
			output.WriteByte(',')
		}
		encodeCanonicalString(output, key)
		output.WriteByte(':')
		encodeCanonicalValue(output, object[key])
	}
	output.WriteByte('}')
}

func encodeCanonicalArray(output *bytes.Buffer, array []any) {
	output.WriteByte('[')
	for index, value := range array {
		if index > 0 {
			output.WriteByte(',')
		}
		encodeCanonicalValue(output, value)
	}
	output.WriteByte(']')
}

var canonicalShortEscapes = map[rune]string{'"': `\"`, '\\': `\\`, '\b': `\b`, '\t': `\t`, '\n': `\n`, '\f': `\f`, '\r': `\r`}

const lowerHex = "0123456789abcdef"

func encodeCanonicalString(output *bytes.Buffer, value string) {
	output.WriteByte('"')
	for _, character := range value {
		output.WriteString(canonicalCharacter(character))
	}
	output.WriteByte('"')
}

func canonicalCharacter(character rune) string {
	if escape, ok := canonicalShortEscapes[character]; ok {
		return escape
	}
	if character < 0x20 {
		return `\u00` + string([]byte{lowerHex[character>>4], lowerHex[character&0xf]})
	}
	return string(character)
}

// CanonicalJSON emits the HDCV0-041 canonical encoding of value, including the
// trailing LF. VerifyCanonicalJSON accepts every byte string it returns; a
// value with a non-integer number or a canonical form over 8 MiB is refused.
func CanonicalJSON(value any) ([]byte, error) {
	encoded, err := CanonicalJSONUnbounded(value)
	if err != nil {
		return nil, err
	}
	if len(encoded) > MaxCanonicalJSONBytes {
		return nil, failure("canonical-json-too-large", "canonical JSON exceeds %d bytes", MaxCanonicalJSONBytes)
	}
	return encoded, nil
}

// CanonicalJSONUnbounded emits the same HDCV0-041 bytes as CanonicalJSON
// without the 8 MiB plan/receipt bound. It serves a downstream profile that
// owns its own output limit (CATN-V0-012); that caller MUST enforce the limit.
func CanonicalJSONUnbounded(value any) ([]byte, error) {
	raw, err := json.Marshal(value)
	if err != nil {
		return nil, failure("canonical-json-unencodable", "value cannot be encoded as JSON")
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	parsed, err := decodeCanonicalValue(decoder)
	if err != nil {
		return nil, err
	}
	var encoded bytes.Buffer
	encodeCanonicalValue(&encoded, parsed)
	encoded.WriteByte('\n')
	return encoded.Bytes(), nil
}
