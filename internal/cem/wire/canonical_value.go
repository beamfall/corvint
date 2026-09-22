package wire

import (
	"sort"
	"strconv"
	"strings"
)

// CanonicalValue encodes one strict JSON value in the frozen canonical form:
// UTF-8, lexicographically sorted object keys, minimal separators, shortest
// signed decimal integers, no terminal LF. It is the single implementation of
// the `canonical-json-value` primitive every content address in the workspace
// hashes over, so a second approximation would silently fork identity.
func CanonicalValue(value Value) []byte {
	var output strings.Builder
	WriteCanonicalValue(&output, value)
	return []byte(output.String())
}

// WriteCanonicalValue appends the canonical encoding of value to output.
func WriteCanonicalValue(output *strings.Builder, value Value) {
	switch value.Kind {
	case KindNull:
		output.WriteString("null")
	case KindBool:
		if value.Bool {
			output.WriteString("true")
		} else {
			output.WriteString("false")
		}
	case KindInt:
		output.WriteString(strconv.FormatInt(value.Int, 10))
	case KindString:
		output.WriteString(CanonicalString(value.Str))
	case KindArray:
		writeCanonicalArray(output, value.Arr)
	case KindObject:
		writeCanonicalObject(output, value.Obj)
	}
}

func writeCanonicalArray(output *strings.Builder, items []Value) {
	output.WriteByte('[')
	for index, item := range items {
		if index > 0 {
			output.WriteByte(',')
		}
		WriteCanonicalValue(output, item)
	}
	output.WriteByte(']')
}

func writeCanonicalObject(output *strings.Builder, object *Object) {
	keys := append([]string(nil), object.Keys...)
	sort.Strings(keys)
	output.WriteByte('{')
	for index, key := range keys {
		if index > 0 {
			output.WriteByte(',')
		}
		output.WriteString(CanonicalString(key))
		output.WriteByte(':')
		WriteCanonicalValue(output, object.Values[key])
	}
	output.WriteByte('}')
}
