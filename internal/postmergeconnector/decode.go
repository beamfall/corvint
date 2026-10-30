package postmergeconnector

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
)

func Decode(data []byte, out any) error {
	if len(data) > MaxBytes {
		return fmt.Errorf("input-too-large")
	}
	// DisallowUnknownFields alone accepts duplicate keys, so walk the bounded
	// token tree before decoding the public closed object vocabulary.
	d := json.NewDecoder(bytes.NewReader(data))
	if err := uniqueValue(d, 0); err != nil {
		return err
	}
	if _, err := d.Token(); err != io.EOF {
		return fmt.Errorf("trailing-json")
	}
	d = json.NewDecoder(bytes.NewReader(data))
	d.DisallowUnknownFields()
	return d.Decode(out)
}
func uniqueValue(d *json.Decoder, depth int) error {
	if depth > 32 {
		return fmt.Errorf("json-too-deep")
	}
	t, err := d.Token()
	if err != nil {
		return err
	}
	delim, ok := t.(json.Delim)
	if !ok {
		return nil
	}
	switch delim {
	case '{':
		seen := map[string]bool{}
		for d.More() {
			key, err := d.Token()
			if err != nil {
				return err
			}
			s, ok := key.(string)
			if !ok || seen[s] {
				return fmt.Errorf("duplicate-json-key")
			}
			seen[s] = true
			if err := uniqueValue(d, depth+1); err != nil {
				return err
			}
		}
	case '[':
		for d.More() {
			if err := uniqueValue(d, depth+1); err != nil {
				return err
			}
		}
	default:
		return fmt.Errorf("invalid-json-delimiter")
	}
	_, err = d.Token()
	return err
}
func Encode(v any) ([]byte, error) {
	b, err := json.Marshal(v)
	if err != nil {
		return nil, err
	}
	return append(b, '\n'), nil
}
