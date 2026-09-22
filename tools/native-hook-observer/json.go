package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strconv"
	"unicode/utf8"
)

func decodeClosedJSON(raw []byte) (any, error) {
	if !utf8.Valid(raw) || !validSurrogateEscapes(raw) {
		return nil, errors.New("invalid-json-unicode")
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	value, err := decodeJSONValue(decoder, 0)
	if err != nil {
		return nil, err
	}
	if _, err := decoder.Token(); err != io.EOF {
		return nil, errors.New("trailing-json")
	}
	return value, nil
}

func validSurrogateEscapes(raw []byte) bool {
	inString := false
	for index := 0; index < len(raw); index++ {
		if raw[index] == '"' {
			inString = !inString
			continue
		}
		if raw[index] != '\\' || !inString || index+1 >= len(raw) {
			continue
		}
		if raw[index+1] != 'u' {
			index++
			continue
		}
		if index+6 > len(raw) {
			return false
		}
		value, err := strconv.ParseUint(string(raw[index+2:index+6]), 16, 16)
		if err != nil || value >= 0xdc00 && value <= 0xdfff {
			return false
		}
		if value >= 0xd800 && value <= 0xdbff {
			if index+12 > len(raw) || string(raw[index+6:index+8]) != `\u` {
				return false
			}
			low, err := strconv.ParseUint(string(raw[index+8:index+12]), 16, 16)
			if err != nil || low < 0xdc00 || low > 0xdfff {
				return false
			}
			index += 11
		} else {
			index += 5
		}
	}
	return true
}

func decodeJSONValue(decoder *json.Decoder, depth int) (any, error) {
	if depth > 256 {
		return nil, errors.New("json-nesting")
	}
	token, err := decoder.Token()
	if err != nil {
		return nil, err
	}
	delim, compound := token.(json.Delim)
	if !compound {
		return token, nil
	}
	switch delim {
	case '{':
		object := map[string]any{}
		for decoder.More() {
			keyToken, err := decoder.Token()
			key, ok := keyToken.(string)
			if err != nil || !ok {
				return nil, errors.New("invalid-key")
			}
			if _, duplicate := object[key]; duplicate {
				return nil, errors.New("duplicate-field")
			}
			value, err := decodeJSONValue(decoder, depth+1)
			if err != nil {
				return nil, err
			}
			object[key] = value
		}
		closing, err := decoder.Token()
		if err != nil || closing != json.Delim('}') {
			return nil, errors.New("unterminated-object")
		}
		return object, nil
	case '[':
		array := []any{}
		for decoder.More() {
			value, err := decodeJSONValue(decoder, depth+1)
			if err != nil {
				return nil, err
			}
			array = append(array, value)
		}
		closing, err := decoder.Token()
		if err != nil || closing != json.Delim(']') {
			return nil, errors.New("unterminated-array")
		}
		return array, nil
	default:
		return nil, errors.New("unexpected-delimiter")
	}
}

func canonical(value any) ([]byte, error) {
	var buffer bytes.Buffer
	encoder := json.NewEncoder(&buffer)
	encoder.SetEscapeHTML(false)
	if err := encoder.Encode(value); err != nil {
		return nil, err
	}
	return bytes.TrimSuffix(buffer.Bytes(), []byte{'\n'}), nil
}

func projectionBytes(value any) ([]byte, error) {
	raw, err := canonical(value)
	if err != nil {
		return nil, err
	}
	raw = append(raw, '\n')
	if len(raw) > maxProjectionBytes {
		return nil, errors.New("projection-too-large")
	}
	return raw, nil
}

func digestBytes(raw []byte) string {
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:])
}

func digestValue(value any) (string, error) {
	raw, err := canonical(value)
	if err != nil {
		return "", err
	}
	return digestBytes(raw), nil
}

func exactFields(value map[string]any, names ...string) error {
	if len(value) != len(names) {
		return errors.New("fields")
	}
	for _, name := range names {
		if _, ok := value[name]; !ok {
			return fmt.Errorf("missing-field:%s", name)
		}
	}
	return nil
}

func integer(value any, nullable bool) (int64, bool, error) {
	if value == nil && nullable {
		return 0, false, nil
	}
	number, ok := value.(json.Number)
	if !ok {
		return 0, false, errors.New("invalid-integer")
	}
	parsed, err := number.Int64()
	if err != nil {
		return 0, false, errors.New("invalid-integer")
	}
	return parsed, true, nil
}

func unsignedInteger(value any, nullable bool) (uint64, bool, error) {
	if value == nil && nullable {
		return 0, false, nil
	}
	number, ok := value.(json.Number)
	if !ok {
		return 0, false, errors.New("invalid-integer")
	}
	parsed, err := strconv.ParseUint(string(number), 10, 64)
	if err != nil {
		return 0, false, errors.New("invalid-integer")
	}
	return parsed, true, nil
}

func asObject(value any, code string) (map[string]any, error) {
	object, ok := value.(map[string]any)
	if !ok {
		return nil, errors.New(code)
	}
	return object, nil
}

func asArray(value any, code string) ([]any, error) {
	array, ok := value.([]any)
	if !ok {
		return nil, errors.New(code)
	}
	return array, nil
}
