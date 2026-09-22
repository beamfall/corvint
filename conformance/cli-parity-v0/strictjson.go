package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"unicode/utf8"
)

func decodeStrictJSON(raw []byte, dst any) error {
	if !utf8.Valid(raw) {
		return fmt.Errorf("invalid UTF-8 in JSON")
	}
	if err := validateJSONValue(raw); err != nil {
		return err
	}

	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(dst); err != nil {
		return err
	}
	if err := requireJSONEOF(decoder); err != nil {
		return err
	}
	return nil
}

func validateJSONValue(raw []byte) error {
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	if err := walkJSONValue(decoder); err != nil {
		return err
	}
	return requireJSONEOF(decoder)
}

func walkJSONValue(decoder *json.Decoder) error {
	token, err := decoder.Token()
	if err != nil {
		return err
	}
	delimiter, ok := token.(json.Delim)
	if !ok {
		return nil
	}

	switch delimiter {
	case '{':
		return walkJSONObject(decoder)
	case '[':
		return walkJSONArray(decoder)
	default:
		return fmt.Errorf("unexpected JSON delimiter %q", delimiter)
	}
}

func walkJSONObject(decoder *json.Decoder) error {
	seen := make(map[string]struct{})
	for decoder.More() {
		keyToken, err := decoder.Token()
		if err != nil {
			return err
		}
		key, ok := keyToken.(string)
		if !ok {
			return fmt.Errorf("non-string JSON object key")
		}
		if _, exists := seen[key]; exists {
			return fmt.Errorf("duplicate JSON object key %q", key)
		}
		seen[key] = struct{}{}
		if err := walkJSONValue(decoder); err != nil {
			return err
		}
	}
	return requireJSONDelimiter(decoder, '}')
}

func walkJSONArray(decoder *json.Decoder) error {
	for decoder.More() {
		if err := walkJSONValue(decoder); err != nil {
			return err
		}
	}
	return requireJSONDelimiter(decoder, ']')
}

func requireJSONDelimiter(decoder *json.Decoder, want json.Delim) error {
	token, err := decoder.Token()
	if err != nil {
		return err
	}
	if token != want {
		return fmt.Errorf("unexpected JSON delimiter %q", token)
	}
	return nil
}

func requireJSONEOF(decoder *json.Decoder) error {
	_, err := decoder.Token()
	if errors.Is(err, io.EOF) {
		return nil
	}
	if err != nil {
		return err
	}
	return fmt.Errorf("trailing JSON value")
}
