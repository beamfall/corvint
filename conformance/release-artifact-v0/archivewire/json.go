package archivewire

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"unicode/utf8"
)

func DecodeStrict(raw []byte, destination any) error {
	if !utf8.Valid(raw) {
		return errors.New("invalid UTF-8 in JSON")
	}
	if err := validateJSON(raw); err != nil {
		return err
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(destination); err != nil {
		return err
	}
	return jsonEOF(decoder)
}

func validateJSON(raw []byte) error {
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	if err := walkValue(decoder); err != nil {
		return err
	}
	return jsonEOF(decoder)
}

func walkValue(decoder *json.Decoder) error {
	token, err := decoder.Token()
	if err != nil {
		return err
	}
	delimiter, ok := token.(json.Delim)
	if !ok {
		return nil
	}
	if delimiter == '[' {
		for decoder.More() {
			if err := walkValue(decoder); err != nil {
				return err
			}
		}
		return delimiterEnd(decoder, ']')
	}
	if delimiter != '{' {
		return fmt.Errorf("unexpected JSON delimiter %q", delimiter)
	}
	seen := map[string]struct{}{}
	for decoder.More() {
		keyToken, err := decoder.Token()
		if err != nil {
			return err
		}
		key, ok := keyToken.(string)
		if !ok {
			return errors.New("non-string JSON object key")
		}
		if _, exists := seen[key]; exists {
			return fmt.Errorf("duplicate JSON object key %q", key)
		}
		seen[key] = struct{}{}
		if err := walkValue(decoder); err != nil {
			return err
		}
	}
	return delimiterEnd(decoder, '}')
}

func delimiterEnd(decoder *json.Decoder, want json.Delim) error {
	token, err := decoder.Token()
	if err != nil {
		return err
	}
	if token != want {
		return fmt.Errorf("unexpected JSON delimiter %q", token)
	}
	return nil
}

func jsonEOF(decoder *json.Decoder) error {
	_, err := decoder.Token()
	if errors.Is(err, io.EOF) {
		return nil
	}
	if err != nil {
		return err
	}
	return errors.New("trailing JSON value")
}
