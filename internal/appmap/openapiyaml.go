package appmap

import (
	"encoding/json"
	"strconv"
	"strings"
)

// YAML OpenAPI subset (AMAP-V0-219). An OpenAPI or Swagger document written in YAML is read when
// it uses only block mappings, block sequences and plain or quoted one-line scalars. Every other
// YAML form (flow collections, anchors, aliases, tags, block scalars, merge keys, multi-line plain
// scalars, several documents, tabs in indentation, duplicate keys) is refused with the line, so the
// document yields no endpoint and an `openapi-yaml-unsupported` unknown instead of a partial read.

type yamlLine struct {
	indent, n int
	text      string
}

type yamlRefusal struct {
	line   int
	reason string
}

// yamlToJSON converts a block-style YAML document to JSON with every scalar a string. It returns
// the refusal detail when the document is outside the subset.
func yamlToJSON(raw []byte) ([]byte, string) {
	var lines []yamlLine
	started := false
	for i, l := range strings.Split(string(raw), "\n") {
		l = strings.TrimRight(l, "\r")
		body := strings.TrimLeft(l, " ")
		if strings.HasPrefix(body, "\t") {
			return nil, "line " + strconv.Itoa(i+1) + ": tab indentation"
		}
		text, ok := yamlStripComment(body)
		if !ok {
			return nil, "line " + strconv.Itoa(i+1) + ": unterminated quoted scalar"
		}
		text = strings.TrimRight(text, " \t")
		if text == "" {
			continue
		}
		if text == "---" && !started {
			started = true
			continue
		}
		if text == "---" || text == "..." || strings.HasPrefix(text, "%") {
			return nil, "line " + strconv.Itoa(i+1) + ": several documents or a directive"
		}
		started = true
		lines = append(lines, yamlLine{indent: len(l) - len(body), n: i + 1, text: text})
	}
	if len(lines) == 0 || lines[0].indent != 0 {
		return nil, "no top-level mapping"
	}
	v, next, ref := yamlNode(lines, 0, 0)
	if ref == nil && next < len(lines) {
		ref = &yamlRefusal{lines[next].n, "unexpected indentation"}
	}
	if ref != nil {
		return nil, "line " + strconv.Itoa(ref.line) + ": " + ref.reason
	}
	if _, ok := v.(map[string]any); !ok {
		return nil, "no top-level mapping"
	}
	out, err := json.Marshal(v)
	if err != nil {
		return nil, err.Error()
	}
	return out, ""
}

// yamlStripComment drops a `#` comment that starts the line or follows a space outside quotes.
func yamlStripComment(s string) (string, bool) {
	var quote byte
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch {
		case quote == '"' && c == '\\':
			i++
		case quote != 0:
			if c == quote {
				if quote == '\'' && i+1 < len(s) && s[i+1] == '\'' {
					i++
				} else {
					quote = 0
				}
			}
		case (c == '"' || c == '\'') && (i == 0 || strings.ContainsRune(" :-[{,", rune(s[i-1]))):
			quote = c
		case c == '#' && (i == 0 || s[i-1] == ' ' || s[i-1] == '\t'):
			return s[:i], true
		}
	}
	return s, quote == 0
}

func yamlDash(text string) bool { return text == "-" || strings.HasPrefix(text, "- ") }

// yamlNode reads the block collection whose first line is lines[i] at indent ind.
func yamlNode(lines []yamlLine, i, ind int) (any, int, *yamlRefusal) {
	if yamlDash(lines[i].text) {
		return yamlSequence(lines, i, ind)
	}
	return yamlMapping(lines, i, ind)
}

func yamlSequence(lines []yamlLine, i, ind int) (any, int, *yamlRefusal) {
	out := []any{}
	for i < len(lines) && lines[i].indent == ind && yamlDash(lines[i].text) {
		l := lines[i]
		rest := strings.TrimLeft(strings.TrimPrefix(l.text, "-"), " ")
		if rest == "" {
			v, next, ref := yamlChild(lines, i, ind, false)
			if ref != nil {
				return nil, 0, ref
			}
			out, i = append(out, v), next
			continue
		}
		if _, _, isKey := yamlKey(rest); isKey || yamlDash(rest) {
			// A compact collection: the item's content starts a block at its own column.
			lines[i] = yamlLine{indent: l.indent + len(l.text) - len(rest), n: l.n, text: rest}
			v, next, ref := yamlNode(lines, i, lines[i].indent)
			if ref != nil {
				return nil, 0, ref
			}
			out, i = append(out, v), next
			continue
		}
		v, ref := yamlScalar(rest, l.n)
		if ref != nil {
			return nil, 0, ref
		}
		out, i = append(out, v), i+1
		if i < len(lines) && lines[i].indent > ind {
			return nil, 0, &yamlRefusal{lines[i].n, "multi-line scalar"}
		}
	}
	return out, i, nil
}

func yamlMapping(lines []yamlLine, i, ind int) (any, int, *yamlRefusal) {
	out := map[string]any{}
	for i < len(lines) && lines[i].indent >= ind {
		l := lines[i]
		if l.indent > ind {
			return nil, 0, &yamlRefusal{l.n, "unexpected indentation or multi-line scalar"}
		}
		if yamlDash(l.text) {
			return nil, 0, &yamlRefusal{l.n, "sequence item in a mapping"}
		}
		key, rest, ok := yamlKey(l.text)
		if !ok {
			return nil, 0, &yamlRefusal{l.n, "not a `key: value` line"}
		}
		k, ref := yamlScalar(key, l.n)
		if ref != nil {
			return nil, 0, ref
		}
		if k == "<<" {
			return nil, 0, &yamlRefusal{l.n, "merge key"}
		}
		if _, dup := out[k]; dup {
			return nil, 0, &yamlRefusal{l.n, "duplicate key " + strconv.Quote(k)}
		}
		if rest == "" {
			v, next, ref := yamlChild(lines, i, ind, true)
			if ref != nil {
				return nil, 0, ref
			}
			out[k], i = v, next
			continue
		}
		v, ref := yamlScalar(rest, l.n)
		if ref != nil {
			return nil, 0, ref
		}
		out[k], i = v, i+1
	}
	return out, i, nil
}

// yamlChild reads the collection nested under lines[i]: more indented, or for a mapping key a
// sequence at the key's own indent. An entry with no nested lines is null.
func yamlChild(lines []yamlLine, i, ind int, sameIndentSeq bool) (any, int, *yamlRefusal) {
	if i+1 >= len(lines) {
		return nil, i + 1, nil
	}
	n := lines[i+1]
	switch {
	case n.indent > ind:
		return yamlNode(lines, i+1, n.indent)
	case sameIndentSeq && n.indent == ind && yamlDash(n.text):
		return yamlSequence(lines, i+1, ind)
	}
	return nil, i + 1, nil
}

// yamlKey splits `key: value` or `key:` at the first `: ` or trailing `:` outside quotes.
func yamlKey(s string) (string, string, bool) {
	if s == "" || s[0] == '?' {
		return "", "", false
	}
	if s[0] == '"' || s[0] == '\'' {
		end := yamlQuoteEnd(s)
		if end < 0 || end+1 >= len(s) || s[end+1] != ':' {
			return "", "", false
		}
		rest := s[end+2:]
		if rest != "" && rest[0] != ' ' {
			return "", "", false
		}
		return s[:end+1], strings.TrimSpace(rest), true
	}
	if k := strings.Index(s, ": "); k >= 0 {
		return s[:k], strings.TrimSpace(s[k+2:]), true
	}
	if strings.HasSuffix(s, ":") {
		return s[:len(s)-1], "", true
	}
	return "", "", false
}

// yamlQuoteEnd returns the index of the quote closing the scalar s opens, or -1.
func yamlQuoteEnd(s string) int {
	q := s[0]
	for i := 1; i < len(s); i++ {
		switch {
		case q == '"' && s[i] == '\\':
			i++
		case s[i] == q && q == '\'' && i+1 < len(s) && s[i+1] == '\'':
			i++
		case s[i] == q:
			return i
		}
	}
	return -1
}

// yamlScalar reads a plain, single-quoted or double-quoted one-line scalar as a string.
func yamlScalar(s string, n int) (string, *yamlRefusal) {
	switch s[0] {
	case '\'':
		if yamlQuoteEnd(s) != len(s)-1 {
			return "", &yamlRefusal{n, "text after a quoted scalar"}
		}
		return strings.ReplaceAll(s[1:len(s)-1], "''", "'"), nil
	case '"':
		if yamlQuoteEnd(s) != len(s)-1 {
			return "", &yamlRefusal{n, "text after a quoted scalar"}
		}
		v, err := strconv.Unquote(s)
		if err != nil {
			return "", &yamlRefusal{n, "double-quoted escape not read"}
		}
		return v, nil
	case '{', '[':
		return "", &yamlRefusal{n, "flow collection"}
	case '&', '*':
		return "", &yamlRefusal{n, "anchor or alias"}
	case '!':
		return "", &yamlRefusal{n, "tag"}
	case '|', '>':
		return "", &yamlRefusal{n, "block scalar"}
	case '@', '`', '%':
		return "", &yamlRefusal{n, "reserved indicator"}
	}
	return s, nil
}
