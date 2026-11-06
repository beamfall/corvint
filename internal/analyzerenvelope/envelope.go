// Package analyzerenvelope holds the request-envelope helpers that the Ruby
// and shell analyzer candidates of docs/specs/analyzer-candidate-profiles.md
// carried as byte-identical copies: the exact decoded length of padded base64,
// the bounded duplicate-member scan, and safe canonical extension stripping
// (decision 0222). It imports only the standard library, so a candidate that
// uses it gains no Core or provider dependency. Each candidate keeps its own
// request types, bounds, refusal reasons and language grammar.
package analyzerenvelope

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"encoding/json/jsontext"
	"strings"
)

// DecodedLength returns the exact decoded size of padded RFC 4648 base64, and
// false when the encoding length is not a multiple of four.
func DecodedLength(encoded string) (int, bool) {
	if len(encoded)%4 != 0 {
		return 0, false
	}
	size := base64.StdEncoding.DecodedLen(len(encoded))
	if len(encoded) > 0 && encoded[len(encoded)-1] == '=' {
		size--
	}
	if len(encoded) > 1 && encoded[len(encoded)-2] == '=' {
		size--
	}
	return size, size >= 0
}

// DuplicateJSONName reports whether raw repeats a member name in any object,
// is not exactly one JSON value, or exceeds maxDepth nesting or maxTokens
// values. Every such case is a refusal for the caller.
func DuplicateJSONName(raw []byte, maxDepth, maxTokens int) bool {
	d := json.NewDecoder(bytes.NewReader(raw))
	tokens := 0
	var walk func(int) bool
	walk = func(depth int) bool {
		tokens++
		if depth > maxDepth || tokens > maxTokens {
			return true
		}
		token, err := d.Token()
		if err != nil {
			return true
		}
		delim, object := token.(json.Delim)
		if !object {
			return false
		}
		switch delim {
		case '{':
			seen := map[string]bool{}
			for d.More() {
				key, err := d.Token()
				if err != nil {
					return true
				}
				value, ok := key.(string)
				if !ok || seen[value] {
					return true
				}
				seen[value] = true
				if walk(depth + 1) {
					return true
				}
			}
			_, err := d.Token()
			return err != nil
		case '[':
			for d.More() {
				if walk(depth + 1) {
					return true
				}
			}
			_, err := d.Token()
			return err != nil
		}
		return true
	}
	if walk(0) {
		return true
	}
	_, err := d.Token()
	return err == nil
}

var extensionFields = map[string]string{"": " profile family request_id scope_id compilation_unit_id target inputs ", "target": " os architecture abi features ", "inputs": " handle family path sha256 content_base64 "}

// WithoutExtensions returns body with every extension member of the request,
// target and input objects removed, and true only when body is canonical JSON
// whose extensions follow their object's schema fields in increasing name
// order and carry only integer numbers.
func WithoutExtensions(body []byte) ([]byte, bool) {
	decoder, encoded, known, at := jsontext.NewDecoder(bytes.NewReader(body)), &bytes.Buffer{}, []byte{}, int64(0)
	encoder := jsontext.NewEncoder(encoded)
	var value func(kind string) bool
	value = func(kind string) bool {
		token, err := decoder.ReadToken()
		delimiter, text := token.Kind(), token.String()
		ok, fields, extended, last := err == nil && encoder.WriteToken(token) == nil, extensionFields[kind], false, ""
		for ok && delimiter == '[' && decoder.PeekKind() != ']' {
			ok = value(kind)
		}
		for ok && delimiter == '{' && decoder.PeekKind() == '"' {
			start := decoder.InputOffset()
			token, _ = decoder.ReadToken()
			name := token.String()
			ok = encoder.WriteToken(token) == nil
			if _, nested := extensionFields[name]; !strings.Contains(name, " ") && strings.Contains(fields, " "+name+" ") {
				if !nested {
					name = "-"
				}
				ok = ok && !extended && value(name)
				continue
			}
			ok = ok && (!extended || name > last) && value("-")
			extended, last = true, name
			if fields != "" {
				known, at = append(known, body[at:start]...), decoder.InputOffset()
			}
		}
		if delimiter == '[' || delimiter == '{' {
			token, err = decoder.ReadToken()
			ok = ok && err == nil && encoder.WriteToken(token) == nil
		}
		return ok && (delimiter != '0' || text != "-0" && !strings.ContainsAny(text, ".eE"))
	}
	ok := value("") && encoded.String() == string(body)+"\n"
	return append(known, body[at:]...), ok
}
