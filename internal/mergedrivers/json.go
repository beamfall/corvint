// SPDX-License-Identifier: Apache-2.0
package mergedrivers

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"reflect"
	"slices"
	"sort"
	"strings"
)

// JSON unions map keys using three-way equality. The registry containers scripts,
// packages and tests union their entries; each entry (including arrays and nested
// objects) is atomic, so competing edits to one entry conflict (DWC-V0-7702).
// Conflicts and malformed inputs return no output and an error naming every
// refusal; JSONConflict renders the file the driver leaves for them (ASD-V0-3792).
func JSON(base, ours, theirs []byte) ([]byte, error) {
	merged, conflicts, err := mergeJSON(base, ours, theirs)
	if err != nil {
		return nil, err
	}
	if len(conflicts) > 0 {
		return nil, conflictError(conflicts)
	}
	return encodeJSON(merged, "")
}

// JSONConflict renders the file a refused JSON merge leaves (ASD-V0-3792): the
// merge with every independent key resolved and a marker block, of the given size,
// around each competing key; or, when an input cannot be merged at all, the three
// inputs whole between markers sized above every marker run that starts a line.
// Either result is invalid JSON, so no registry reader accepts it unresolved.
func JSONConflict(base, ours, theirs []byte, size int) []byte {
	if size < 7 {
		size = 7
	}
	merged, conflicts, err := mergeJSON(base, ours, theirs)
	if err != nil || len(conflicts) == 0 {
		return wholeConflict(base, ours, theirs, size)
	}
	var out bytes.Buffer
	if err := renderMarked(&out, merged, "", size); err != nil {
		return wholeConflict(base, ours, theirs, size)
	}
	out.WriteByte('\n')
	return out.Bytes()
}

// conflict is a competing key left in the merged map at its own position.
type conflict struct {
	container, key     string
	base, ours, theirs any
	bp, op, tp         bool
}

func mergeJSON(base, ours, theirs []byte) (map[string]any, []*conflict, error) {
	// Git supplies an empty %O for add/add; it is the absent object map,
	// while empty or malformed current sides still refuse.
	if len(base) == 0 {
		base = []byte("{}")
	}
	var maps [3]map[string]any
	for i, data := range [][]byte{base, ours, theirs} {
		side := [...]string{"base", "ours", "theirs"}[i]
		decoder := json.NewDecoder(bytes.NewReader(data))
		decoder.UseNumber()
		value, err := jsonValue(decoder)
		if err != nil {
			return nil, nil, fmt.Errorf("%s registry: %w", side, err)
		}
		var ok bool
		maps[i], ok = value.(map[string]any)
		if !ok {
			return nil, nil, fmt.Errorf("%s registry must be an object", side)
		}
		if _, err := decoder.Token(); err != io.EOF {
			return nil, nil, fmt.Errorf("trailing %s registry input", side)
		}
	}
	var conflicts []*conflict
	merged := unionMap(maps[0], maps[1], maps[2], "", &conflicts)
	// Name refusals in the order the file renders them: by key path.
	path := func(c *conflict) []string {
		if c.container == "" {
			return []string{c.key}
		}
		return []string{c.container, c.key}
	}
	sort.Slice(conflicts, func(i, j int) bool {
		return slices.Compare(path(conflicts[i]), path(conflicts[j])) < 0
	})
	return merged, conflicts, nil
}

// conflictError names each competing key with its container and all three values.
func conflictError(conflicts []*conflict) error {
	lines := make([]string, 0, len(conflicts))
	for _, c := range conflicts {
		where := ""
		if c.container != "" {
			where = fmt.Sprintf(" in %q", c.container)
		}
		lines = append(lines, fmt.Sprintf("competing registry key %q%s: base %s, ours %s, theirs %s",
			c.key, where, compact(c.base, c.bp), compact(c.ours, c.op), compact(c.theirs, c.tp)))
	}
	return errors.New(strings.Join(lines, "\n"))
}

func compact(value any, present bool) string {
	if !present {
		return "absent"
	}
	var out bytes.Buffer
	encoder := json.NewEncoder(&out)
	encoder.SetEscapeHTML(false)
	if err := encoder.Encode(value); err != nil {
		return "unencodable"
	}
	return strings.TrimSuffix(out.String(), "\n")
}

// encodeJSON is the DWC-V0-7702 encoding: sorted keys, two-space indentation,
// unescaped HTML; prefix indents every line after the first, and only a
// top-level value (empty prefix) keeps the final LF.
func encodeJSON(value any, prefix string) ([]byte, error) {
	var out bytes.Buffer
	encoder := json.NewEncoder(&out)
	encoder.SetEscapeHTML(false)
	encoder.SetIndent(prefix, "  ")
	if err := encoder.Encode(value); err != nil {
		return nil, err
	}
	if prefix != "" {
		return bytes.TrimSuffix(out.Bytes(), []byte("\n")), nil
	}
	return out.Bytes(), nil
}

// renderMarked writes m as encodeJSON would, except that each competing key
// becomes a marker block holding that key's entry as each side renders it.
func renderMarked(out *bytes.Buffer, m map[string]any, indent string, size int) error {
	keys := make([]string, 0, len(m))
	for key := range m {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	if len(keys) == 0 {
		out.WriteString("{}")
		return nil
	}
	out.WriteString("{\n")
	inner := indent + "  "
	for i, key := range keys {
		comma := ""
		if i < len(keys)-1 {
			comma = ","
		}
		name, err := encodeJSON(key, inner)
		if err != nil {
			return err
		}
		entry := func(value any) error {
			encoded, err := encodeJSON(value, inner)
			if err != nil {
				return err
			}
			fmt.Fprintf(out, "%s%s: %s%s\n", inner, name, encoded, comma)
			return nil
		}
		switch value := m[key].(type) {
		case *conflict:
			for _, side := range []struct {
				marker  string
				value   any
				present bool
			}{
				{strings.Repeat("<", size) + " ours", value.ours, value.op},
				{strings.Repeat("|", size) + " base", value.base, value.bp},
				{strings.Repeat("=", size), value.theirs, value.tp},
			} {
				out.WriteString(side.marker + "\n")
				if side.present {
					if err := entry(side.value); err != nil {
						return err
					}
				}
			}
			out.WriteString(strings.Repeat(">", size) + " theirs\n")
		case map[string]any:
			if !hasConflict(value) {
				if err := entry(value); err != nil {
					return err
				}
				continue
			}
			fmt.Fprintf(out, "%s%s: ", inner, name)
			if err := renderMarked(out, value, inner, size); err != nil {
				return err
			}
			out.WriteString(comma + "\n")
		default:
			if err := entry(value); err != nil {
				return err
			}
		}
	}
	out.WriteString(indent + "}")
	return nil
}

func hasConflict(m map[string]any) bool {
	for _, value := range m {
		if _, ok := value.(*conflict); ok {
			return true
		}
	}
	return false
}

// wholeConflict writes the three inputs between markers longer than any marker
// run that starts one of their lines, so the markers are unambiguous.
func wholeConflict(base, ours, theirs []byte, size int) []byte {
	size = appendMarkerSize(size, base, ours, theirs)
	var out bytes.Buffer
	for _, side := range []struct {
		marker string
		data   []byte
	}{
		{strings.Repeat("<", size) + " ours", ours},
		{strings.Repeat("|", size) + " base", base},
		{strings.Repeat("=", size), theirs},
	} {
		out.WriteString(side.marker + "\n")
		out.Write(side.data)
		if len(side.data) > 0 && side.data[len(side.data)-1] != '\n' {
			out.WriteByte('\n')
		}
	}
	out.WriteString(strings.Repeat(">", size) + " theirs\n")
	return out.Bytes()
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

// unionMap merges one object level. Only the top level unions the registry
// containers; a competing key is recorded and left as its *conflict.
func unionMap(base, ours, theirs map[string]any, container string, conflicts *[]*conflict) map[string]any {
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
			if container != "" || (key != "scripts" && key != "packages" && key != "tests") || (!bok && bp) || !ook || !tok {
				c := &conflict{container: container, key: key, base: b, ours: o, theirs: t, bp: bp, op: op, tp: tp}
				*conflicts = append(*conflicts, c)
				v, present = c, true
				break
			}
			v = unionMap(bm, om, tm, key, conflicts)
		}
		if present {
			out[key] = v
		}
	}
	return out
}
