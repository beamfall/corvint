// SPDX-License-Identifier: Apache-2.0
package mergedrivers

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"reflect"
)

// JSON unions map keys using three-way equality. The registry containers scripts,
// packages and tests union their entries; each entry (including arrays and nested
// objects) is atomic, so competing edits to one entry conflict (DWC-V0-7702).
// Conflicts and malformed inputs return no output, leaving the driver's ours intact.
func JSON(base, ours, theirs []byte) ([]byte, error) {
	var maps [3]map[string]any
	for i, data := range [][]byte{base, ours, theirs} {
		decoder := json.NewDecoder(bytes.NewReader(data))
		decoder.UseNumber()
		value, err := jsonValue(decoder)
		if err != nil {
			return nil, err
		}
		var ok bool
		maps[i], ok = value.(map[string]any)
		if !ok {
			return nil, fmt.Errorf("registry must be an object")
		}
		if _, err := decoder.Token(); err != io.EOF {
			return nil, fmt.Errorf("trailing registry input")
		}
	}
	merged, err := unionMap(maps[0], maps[1], maps[2], true)
	if err != nil {
		return nil, err
	}
	var out bytes.Buffer
	encoder := json.NewEncoder(&out)
	encoder.SetEscapeHTML(false)
	encoder.SetIndent("", "  ")
	err = encoder.Encode(merged)
	return out.Bytes(), err
}

// jsonValue rejects duplicate keys rather than silently dropping authority-bearing entries.
func jsonValue(d *json.Decoder) (any, error) {
	token, err := d.Token()
	if err != nil {
		return nil, err
	}
	switch token {
	case json.Delim('{'):
		m := map[string]any{}
		for d.More() {
			key, err := d.Token()
			if err != nil {
				return nil, err
			}
			name, ok := key.(string)
			if !ok {
				return nil, fmt.Errorf("invalid registry key")
			}
			if _, exists := m[name]; exists {
				return nil, fmt.Errorf("duplicate registry key %q", name)
			}
			m[name], err = jsonValue(d)
			if err != nil {
				return nil, err
			}
		}
		_, err = d.Token()
		return m, err
	case json.Delim('['):
		a := []any{}
		for d.More() {
			v, err := jsonValue(d)
			if err != nil {
				return nil, err
			}
			a = append(a, v)
		}
		_, err = d.Token()
		return a, err
	default:
		if _, delimiter := token.(json.Delim); delimiter {
			return nil, fmt.Errorf("invalid registry delimiter")
		}
		return token, nil
	}
}

func unionMap(base, ours, theirs map[string]any, containers bool) (map[string]any, error) {
	keys := map[string]bool{}
	for _, m := range []map[string]any{base, ours, theirs} {
		for key := range m {
			keys[key] = true
		}
	}
	out := map[string]any{}
	for key := range keys {
		b, bp := base[key]
		o, op := ours[key]
		t, tp := theirs[key]
		v, present := o, op
		switch {
		case op == tp && reflect.DeepEqual(o, t):
		case op == bp && reflect.DeepEqual(o, b):
			v, present = t, tp
		case tp == bp && reflect.DeepEqual(t, b):
		default:
			bm, bok := b.(map[string]any)
			om, ook := o.(map[string]any)
			tm, tok := t.(map[string]any)
			if !containers || (key != "scripts" && key != "packages" && key != "tests") || (!bok && bp) || !ook || !tok {
				return nil, fmt.Errorf("competing registry key %q", key)
			}
			var err error
			v, err = unionMap(bm, om, tm, false)
			if err != nil {
				return nil, err
			}
		}
		if present {
			out[key] = v
		}
	}
	return out, nil
}
