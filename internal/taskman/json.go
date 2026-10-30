// Package taskman implements the explicitly trusted-local native fixture planner.
package taskman

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"path"
	"sort"
	"strconv"
	"strings"
	"unicode"

	"github.com/Beamfall/corvint/internal/cem/wire"
)

func sum(raw []byte) string { h := sha256.Sum256(raw); return hex.EncodeToString(h[:]) }
func digest(s string) bool {
	if len(s) != 64 {
		return false
	}
	for _, c := range s {
		if !strings.ContainsRune("0123456789abcdef", c) {
			return false
		}
	}
	return true
}
func identifier(s string) bool {
	return boundedText(s, 128)
}
func boundedText(s string, max int) bool {
	if s == "" || len(s) > max {
		return false
	}
	for _, c := range s {
		if unicode.IsControl(c) || unicode.In(c, unicode.Cf) {
			return false
		}
	}
	return true
}
func validPath(s string) bool {
	if !boundedText(s, 512) || strings.Contains(s, "\\") {
		return false
	}
	p := strings.TrimSuffix(s, "/")
	return p != "" && p != "." && p != ".." && !strings.HasPrefix(p, "../") && !path.IsAbs(p) && path.Clean(p) == p
}
func token(s string, max int) bool {
	if s == "" || len(s) > max {
		return false
	}
	for i, c := range s {
		if c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9' {
			continue
		}
		if i == 0 || !strings.ContainsRune("._-", c) {
			return false
		}
	}
	return true
}
func queueID(s string) bool {
	p := strings.Split(s, ":")
	return identifier(s) && len(p) == 3 && p[0] == "queue" && token(p[1], 128) && token(p[2], 128)
}
func nativeID(s, kind, queue string) bool {
	p := strings.Split(s, ":")
	if !identifier(s) || len(p) != 4 || p[0] != kind || "queue:"+p[1]+":"+p[2] != queue || !queueID(queue) {
		return false
	}
	if kind == "attempt" {
		return len(p[3]) == 32 && digest(p[3]+p[3])
	}
	return kind == "ticket" && token(p[3], 64)
}
func number(v wire.Value, max uint64) (uint64, error) {
	if v.Kind != wire.KindString || v.Str == "" {
		return 0, errors.New("decimal string required")
	}
	n, e := strconv.ParseUint(v.Str, 10, 64)
	if e != nil || n > max || strconv.FormatUint(n, 10) != v.Str {
		return 0, errors.New("invalid decimal string")
	}
	return n, nil
}
func text(v wire.Value) (string, error) {
	if v.Kind != wire.KindString || !boundedText(v.Str, 512) {
		return "", errors.New("invalid identifier")
	}
	return v.Str, nil
}
func object(v wire.Value, keys string) error {
	if v.Kind != wire.KindObject {
		return errors.New("object required")
	}
	allowed := strings.Fields(keys)
	if len(v.Obj.Keys) != len(allowed) {
		return errors.New("closed object member count")
	}
	for _, k := range allowed {
		if _, ok := v.Obj.Values[k]; !ok {
			return fmt.Errorf("missing member %s", k)
		}
	}
	return nil
}
func value(v wire.Value, k string) wire.Value {
	if v.Kind != wire.KindObject {
		return wire.Value{}
	}
	return v.Obj.Values[k]
}
func stringAt(v wire.Value, k string) string { return value(v, k).Str }
func boolAt(v wire.Value, k string) bool {
	return value(v, k).Kind == wire.KindBool && value(v, k).Bool
}
func array(v wire.Value, max int) ([]wire.Value, error) {
	if v.Kind != wire.KindArray || len(v.Arr) > max {
		return nil, errors.New("array bound or type")
	}
	return v.Arr, nil
}
func canonical(v wire.Value) []byte { var b bytes.Buffer; writeValue(&b, v); return b.Bytes() }
func writeString(b *bytes.Buffer, s string) {
	b.WriteByte('"')
	for _, c := range s {
		switch c {
		case '"', '\\':
			b.WriteByte('\\')
			b.WriteRune(c)
		case '\n':
			b.WriteString(`\n`)
		case '\r':
			b.WriteString(`\r`)
		case '\t':
			b.WriteString(`\t`)
		default:
			if c < 32 {
				fmt.Fprintf(b, `\u%04x`, c)
			} else {
				b.WriteRune(c)
			}
		}
	}
	b.WriteByte('"')
}
func writeValue(b *bytes.Buffer, v wire.Value) {
	switch v.Kind {
	case wire.KindNull:
		b.WriteString("null")
	case wire.KindString:
		writeString(b, v.Str)
	case wire.KindBool:
		fmt.Fprint(b, v.Bool)
	case wire.KindInt:
		fmt.Fprint(b, v.Int)
	case wire.KindArray:
		b.WriteByte('[')
		for i, x := range v.Arr {
			if i > 0 {
				b.WriteByte(',')
			}
			writeValue(b, x)
		}
		b.WriteByte(']')
	case wire.KindObject:
		b.WriteByte('{')
		keys := append([]string{}, v.Obj.Keys...)
		sort.Strings(keys)
		for i, k := range keys {
			if i > 0 {
				b.WriteByte(',')
			}
			writeString(b, k)
			b.WriteByte(':')
			writeValue(b, v.Obj.Values[k])
		}
		b.WriteByte('}')
	}
}
func document(raw []byte, max int) (wire.Value, error) {
	if len(raw) > max {
		return wire.Value{}, errors.New("document bound")
	}
	v, e := wire.Parse(raw)
	if e != nil {
		return v, e
	}
	if !bytes.Equal(raw, append(canonical(v), '\n')) {
		return v, errors.New("noncanonical native document")
	}
	return v, nil
}
func encode(v any) ([]byte, error) {
	raw, e := json.Marshal(v)
	if e != nil {
		return nil, e
	}
	parsed, e := wire.Parse(raw)
	if e != nil {
		return nil, e
	}
	return append(canonical(parsed), '\n'), nil
}
func count(n uint64) string { return strconv.FormatUint(n, 10) }
func stringsAt(v wire.Value, k string, max int) ([]string, error) {
	a, e := array(value(v, k), max)
	if e != nil {
		return nil, e
	}
	r := make([]string, 0, len(a))
	for _, x := range a {
		s, e := text(x)
		if e != nil {
			return nil, e
		}
		r = append(r, s)
	}
	return r, nil
}
