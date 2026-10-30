package model

import (
	"bytes"
	json "encoding/json/v2"
	"fmt"
	"sort"
	"unicode/utf8"
)

// canonicalJSON emits the V0 JSON subset: printable ASCII strings, null,
// arrays, and objects with member names sorted by their raw UTF-8 bytes.
// JSON numbers are deliberately absent from the wire; decimal quantities are
// strings so independent decoders cannot disagree about numeric spelling.
func canonicalJSON(value any) ([]byte, error) {
	raw, err := json.Marshal(value)
	if err != nil {
		return nil, fmt.Errorf("marshal intermediate: %w", err)
	}
	var generic any
	if err := json.Unmarshal(raw, &generic); err != nil {
		return nil, fmt.Errorf("decode intermediate: %w", err)
	}
	var output bytes.Buffer
	if err := appendCanonical(&output, generic); err != nil {
		return nil, err
	}
	return output.Bytes(), nil
}

func appendCanonical(output *bytes.Buffer, value any) error {
	switch typed := value.(type) {
	case nil:
		output.WriteString("null")
	case string:
		if err := validateWireString(typed); err != nil {
			return err
		}
		encoded, err := json.Marshal(typed)
		if err != nil {
			return fmt.Errorf("marshal string: %w", err)
		}
		output.Write(encoded)
	case []any:
		output.WriteByte('[')
		for index, element := range typed {
			if index != 0 {
				output.WriteByte(',')
			}
			if err := appendCanonical(output, element); err != nil {
				return err
			}
		}
		output.WriteByte(']')
	case map[string]any:
		keys := make([]string, 0, len(typed))
		for key := range typed {
			if err := validateWireString(key); err != nil {
				return err
			}
			keys = append(keys, key)
		}
		sort.Strings(keys)
		output.WriteByte('{')
		for index, key := range keys {
			if index != 0 {
				output.WriteByte(',')
			}
			encoded, err := json.Marshal(key)
			if err != nil {
				return fmt.Errorf("marshal key: %w", err)
			}
			output.Write(encoded)
			output.WriteByte(':')
			if err := appendCanonical(output, typed[key]); err != nil {
				return err
			}
		}
		output.WriteByte('}')
	default:
		return fmt.Errorf("dashboard wire contains forbidden JSON scalar %T", value)
	}
	return nil
}

// V0 has no free-form text. Every emitted identifier, hash, timestamp, fixed
// label, and closed enum is printable ASCII, which is already NFC. Rejecting
// all other strings keeps normalization deterministic and dependency-free.
func validateWireString(value string) error {
	if !utf8.ValidString(value) {
		return fmt.Errorf("dashboard wire string is not UTF-8")
	}
	for _, character := range value {
		if character < 0x20 || character > 0x7e {
			return fmt.Errorf("dashboard V0 wire string is not printable ASCII")
		}
	}
	return nil
}
