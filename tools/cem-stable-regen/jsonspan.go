package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
)

// jnode is one JSON value with its byte span in the source document. Objects
// keep their member order and the span of every key, so an edit can splice new
// bytes into the original text and leave every other byte untouched.
type jnode struct {
	kind  byte // '{', '[', '"', '0' (number) or 'l' (true, false, null)
	start int
	end   int
	keys  []string
	kspan [][2]int
	elems []*jnode
}

func parseJSON(src []byte) (*jnode, error) {
	if !json.Valid(src) {
		return nil, errors.New("not valid JSON")
	}
	p := &spanParser{src: src}
	p.space()
	n := p.value()
	p.space()
	if p.pos != len(src) {
		return nil, fmt.Errorf("trailing data at byte %d", p.pos)
	}
	if key := duplicateKey(n); key != "" {
		return nil, fmt.Errorf("duplicate key %q", key)
	}
	return n, nil
}

// duplicateKey returns the first object key repeated anywhere under n, since
// encoding/json would silently keep the last value while get keeps the first.
func duplicateKey(n *jnode) string {
	if n.kind == '{' {
		seen := map[string]bool{}
		for _, k := range n.keys {
			if seen[k] {
				return k
			}
			seen[k] = true
		}
	}
	for _, e := range n.elems {
		if k := duplicateKey(e); k != "" {
			return k
		}
	}
	return ""
}

// spanParser walks text that json.Valid has already accepted, so it only
// records spans and never reports syntax errors itself.
type spanParser struct {
	src []byte
	pos int
}

func (p *spanParser) space() {
	for p.pos < len(p.src) && strings.IndexByte(" \t\r\n", p.src[p.pos]) >= 0 {
		p.pos++
	}
}

func (p *spanParser) value() *jnode {
	n := &jnode{start: p.pos}
	switch c := p.src[p.pos]; {
	case c == '{':
		n.kind = '{'
		p.pos++
		for p.space(); p.src[p.pos] != '}'; p.space() {
			kstart := p.pos
			p.str()
			var key string
			_ = json.Unmarshal(p.src[kstart:p.pos], &key)
			n.keys = append(n.keys, key)
			n.kspan = append(n.kspan, [2]int{kstart, p.pos})
			p.space()
			p.pos++ // ':'
			p.space()
			n.elems = append(n.elems, p.value())
			p.space()
			if p.src[p.pos] == ',' {
				p.pos++
			}
		}
		p.pos++
	case c == '[':
		n.kind = '['
		p.pos++
		for p.space(); p.src[p.pos] != ']'; p.space() {
			n.elems = append(n.elems, p.value())
			p.space()
			if p.src[p.pos] == ',' {
				p.pos++
			}
		}
		p.pos++
	case c == '"':
		n.kind = '"'
		p.str()
	case c == 't' || c == 'f' || c == 'n':
		n.kind = 'l'
		for p.pos < len(p.src) && p.src[p.pos] >= 'a' && p.src[p.pos] <= 'z' {
			p.pos++
		}
	default:
		n.kind = '0'
		for p.pos < len(p.src) && strings.IndexByte("+-.0123456789eE", p.src[p.pos]) >= 0 {
			p.pos++
		}
	}
	n.end = p.pos
	return n
}

func (p *spanParser) str() {
	p.pos++
	for p.src[p.pos] != '"' {
		if p.src[p.pos] == '\\' {
			p.pos++
		}
		p.pos++
	}
	p.pos++
}

func (n *jnode) index(key string) int {
	for i, k := range n.keys {
		if k == key {
			return i
		}
	}
	return -1
}

func (n *jnode) get(key string) *jnode {
	if n == nil || n.kind != '{' {
		return nil
	}
	if i := n.index(key); i >= 0 {
		return n.elems[i]
	}
	return nil
}

func (n *jnode) raw(src []byte) []byte { return src[n.start:n.end] }

// text decodes a string node; it returns "" for any other node.
func (n *jnode) text(src []byte) string {
	if n == nil || n.kind != '"' {
		return ""
	}
	var s string
	_ = json.Unmarshal(n.raw(src), &s)
	return s
}

// decode returns the generic Go value of a node, for semantic comparison.
func (n *jnode) decode(src []byte) (any, error) {
	var v any
	err := json.Unmarshal(n.raw(src), &v)
	return v, err
}

// lineIndent returns the run of spaces between the last newline before off and
// off, which is the indentation convention of the member starting at off. Text
// that shares its line with other tokens (compact JSON) has no indentation.
func lineIndent(src []byte, off int) string {
	line := bytes.LastIndexByte(src[:off], '\n') + 1
	prefix := src[line:off]
	if len(bytes.Trim(prefix, " \t")) != 0 {
		return ""
	}
	return string(prefix)
}

// render re-emits n at the given indentation in the packet's two-space style,
// keeping member order and every scalar's original bytes.
func render(src []byte, n *jnode, indent string) string {
	var b strings.Builder
	var walk func(n *jnode, indent string)
	walk = func(n *jnode, indent string) {
		switch n.kind {
		case '{', '[':
			open, closing := "{", "}"
			if n.kind == '[' {
				open, closing = "[", "]"
			}
			if len(n.elems) == 0 {
				b.WriteString(open + closing)
				return
			}
			b.WriteString(open + "\n")
			inner := indent + "  "
			for i, e := range n.elems {
				b.WriteString(inner)
				if n.kind == '{' {
					b.Write(src[n.kspan[i][0]:n.kspan[i][1]])
					b.WriteString(": ")
				}
				walk(e, inner)
				if i < len(n.elems)-1 {
					b.WriteByte(',')
				}
				b.WriteByte('\n')
			}
			b.WriteString(indent + closing)
		default:
			b.Write(n.raw(src))
		}
	}
	walk(n, indent)
	return b.String()
}

func quote(s string) string {
	b, _ := json.Marshal(s)
	return string(b)
}

// splice replaces src[at:end] with text.
type splice struct {
	at, end int
	text    string
}

func applySplices(src []byte, edits []splice) ([]byte, error) {
	sort.SliceStable(edits, func(i, j int) bool { return edits[i].at < edits[j].at })
	var out bytes.Buffer
	last := 0
	for _, e := range edits {
		if e.at < last || e.end < e.at {
			return nil, errors.New("overlapping edits")
		}
		out.Write(src[last:e.at])
		out.WriteString(e.text)
		last = e.end
	}
	out.Write(src[last:])
	if !json.Valid(out.Bytes()) {
		return nil, errors.New("edit produced invalid JSON")
	}
	return out.Bytes(), nil
}

// replaceValue replaces the whole value of n.
func replaceValue(n *jnode, text string) splice { return splice{at: n.start, end: n.end, text: text} }

// insertMember adds member: value after the member named after in obj, in the
// object's own indentation.
func insertMember(src []byte, obj *jnode, after, member string, value func(indent string) string) (splice, error) {
	if obj == nil || obj.kind != '{' {
		return splice{}, errors.New("not an object")
	}
	if obj.index(member) >= 0 {
		return splice{}, fmt.Errorf("member %q already exists", member)
	}
	i := obj.index(after)
	if i < 0 {
		return splice{}, fmt.Errorf("no member %q to insert after", after)
	}
	indent := lineIndent(src, obj.kspan[i][0])
	end := obj.elems[i].end
	return splice{at: end, end: end, text: ",\n" + indent + quote(member) + ": " + value(indent)}, nil
}

// insertString adds the string member after the string element after in arr.
func insertString(src []byte, arr *jnode, after, member string) (splice, error) {
	if arr == nil || arr.kind != '[' {
		return splice{}, errors.New("not an array")
	}
	at := -1
	for i, e := range arr.elems {
		switch e.text(src) {
		case member:
			return splice{}, fmt.Errorf("%q is already listed", member)
		case after:
			at = i
		}
	}
	if at < 0 {
		return splice{}, fmt.Errorf("%q is not listed", after)
	}
	e := arr.elems[at]
	return splice{at: e.end, end: e.end, text: ",\n" + lineIndent(src, e.start) + quote(member)}, nil
}
