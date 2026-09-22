package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"unicode/utf8"
)

type runnerError string

func (e runnerError) Error() string { return string(e) }
func fail(code string) error        { return runnerError(code) }

func jsonPreflight(data []byte, code string) error {
	depth, digits := 0, 0
	inString, escaped := false, false
	for _, b := range data {
		if inString {
			if escaped {
				escaped = false
			} else if b == '\\' {
				escaped = true
			} else if b == '"' {
				inString = false
			}
			continue
		}
		switch {
		case b == '"':
			inString = true
			digits = 0
		case b == '{' || b == '[':
			depth++
			digits = 0
			if depth > maxJSONDepth {
				return fail(code + "-nesting")
			}
		case b == '}' || b == ']':
			if depth > 0 {
				depth--
			}
			digits = 0
		case b >= '0' && b <= '9':
			digits++
			if digits > maxJSONIntegerDigits {
				return fail(code + "-integer-too-long")
			}
		default:
			digits = 0
		}
	}
	return nil
}

func strictObject(data []byte, limit int, code string) (map[string]any, error) {
	if len(data) > limit {
		return nil, fail(code + "-too-large")
	}
	if !utf8.Valid(data) {
		return nil, fail(code + "-invalid-json")
	}
	if err := jsonPreflight(data, code); err != nil {
		return nil, err
	}
	d := json.NewDecoder(bytes.NewReader(data))
	d.UseNumber()
	v, err := decodeStrict(d)
	if err != nil {
		return nil, fail(code + "-invalid-json")
	}
	if _, err = d.Token(); err != io.EOF {
		return nil, fail(code + "-invalid-json")
	}
	object, ok := v.(map[string]any)
	if !ok {
		return nil, fail(code + "-invalid-root")
	}
	return object, nil
}

func decodeStrict(d *json.Decoder) (any, error) {
	token, err := d.Token()
	if err != nil {
		return nil, err
	}
	switch t := token.(type) {
	case json.Delim:
		switch t {
		case '{':
			out := map[string]any{}
			for d.More() {
				keyToken, err := d.Token()
				if err != nil {
					return nil, err
				}
				key, ok := keyToken.(string)
				if !ok {
					return nil, errors.New("object key")
				}
				if _, exists := out[key]; exists {
					return nil, errors.New("duplicate key")
				}
				value, err := decodeStrict(d)
				if err != nil {
					return nil, err
				}
				out[key] = value
			}
			end, err := d.Token()
			if err != nil || end != json.Delim('}') {
				return nil, errors.New("object end")
			}
			return out, nil
		case '[':
			out := []any{}
			for d.More() {
				value, err := decodeStrict(d)
				if err != nil {
					return nil, err
				}
				out = append(out, value)
			}
			end, err := d.Token()
			if err != nil || end != json.Delim(']') {
				return nil, errors.New("array end")
			}
			return out, nil
		default:
			return nil, errors.New("delimiter")
		}
	case nil, bool, string, json.Number:
		return t, nil
	case float64:
		return nil, errors.New("unexpected float")
	}
	return nil, fmt.Errorf("token %T", token)
}

func canonical(v any) ([]byte, error) {
	var b bytes.Buffer
	e := json.NewEncoder(&b)
	e.SetEscapeHTML(false)
	if err := e.Encode(v); err != nil {
		return nil, err
	}
	return bytes.TrimSuffix(b.Bytes(), []byte{'\n'}), nil
}
func cloneObject(v map[string]any) map[string]any {
	raw, _ := canonical(v)
	out, _ := strictObject(raw, len(raw), "clone")
	return out
}
func exactKeys(v map[string]any, keys ...string) bool {
	if len(v) != len(keys) {
		return false
	}
	for _, k := range keys {
		if _, ok := v[k]; !ok {
			return false
		}
	}
	return true
}
func integer(v any, max *int) (int64, bool) {
	n, ok := v.(json.Number)
	if !ok {
		return 0, false
	}
	i, err := n.Int64()
	if err != nil || i < 0 {
		return 0, false
	}
	if max != nil && i > int64(*max) {
		return 0, false
	}
	return i, true
}
