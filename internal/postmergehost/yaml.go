// Package postmergehost audits reference CI host templates for the post-merge
// workflow against the host-neutral workflow graph (PCH-V0). It reads only the
// declared workflow text; it never runs, fetches or authenticates anything.
package postmergehost

import (
	"errors"
	"fmt"
	"strings"
)

// Kind is the shape of a parsed YAML node.
type Kind int

const (
	Scalar Kind = iota
	Mapping
	Sequence
)

// Node is one value of the restricted YAML subset. Mapping keys keep their
// source order; scalars keep their decoded text.
type Node struct {
	Kind  Kind
	Line  int
	Text  string
	Keys  []string
	Map   map[string]*Node
	Items []*Node
}

// Get returns a mapping value or nil.
func (n *Node) Get(key string) *Node {
	if n == nil || n.Kind != Mapping {
		return nil
	}
	return n.Map[key]
}

// ErrUnsupportedYAML reports input outside the audited subset. The audit
// fails closed: a construct it cannot model is never treated as harmless.
var ErrUnsupportedYAML = errors.New("unsupported-yaml")

type line struct {
	num    int
	indent int
	text   string // comment-stripped, right-trimmed content
	raw    string // untouched source line
}

type parser struct {
	lines []line
	all   []string
	pos   int
}

const maxTemplateBytes = 256 << 10

// ParseYAML parses the block-style subset used by the reference templates:
// mappings, sequences, plain and quoted scalars, `|`/`|-` literals, `{}`,
// `[]` and flat flow sequences. Anchors, aliases, tags, folded scalars, flow
// mappings, multiple documents, tabs and duplicate keys are refused.
func ParseYAML(data []byte) (*Node, error) {
	if len(data) > maxTemplateBytes {
		return nil, fmt.Errorf("%w: template exceeds %d bytes", ErrUnsupportedYAML, maxTemplateBytes)
	}
	p := &parser{all: strings.Split(strings.ReplaceAll(string(data), "\r\n", "\n"), "\n")}
	for i, raw := range p.all {
		if strings.ContainsRune(raw, '\r') {
			return nil, p.fail(i+1, "carriage return")
		}
		body := strings.TrimLeft(raw, " ")
		if strings.HasPrefix(body, "\t") {
			return nil, p.fail(i+1, "tab indentation")
		}
		text, err := stripComment(body)
		if err != nil {
			return nil, p.fail(i+1, err.Error())
		}
		if text == "" {
			continue
		}
		if i == 0 && text == "---" {
			continue
		}
		if text == "---" || text == "..." || strings.HasPrefix(text, "%") {
			return nil, p.fail(i+1, "multiple documents or directives")
		}
		p.lines = append(p.lines, line{num: i + 1, indent: len(raw) - len(body), text: text, raw: raw})
	}
	if len(p.lines) == 0 {
		return nil, fmt.Errorf("%w: empty document", ErrUnsupportedYAML)
	}
	if p.lines[0].indent != 0 {
		return nil, p.fail(p.lines[0].num, "indented root")
	}
	root, err := p.node(0)
	if err != nil {
		return nil, err
	}
	if p.pos != len(p.lines) {
		return nil, p.fail(p.lines[p.pos].num, "unexpected dedent")
	}
	return root, nil
}

func (p *parser) fail(num int, why string) error {
	return fmt.Errorf("%w: line %d: %s", ErrUnsupportedYAML, num, why)
}

func isItem(text string) bool { return text == "-" || strings.HasPrefix(text, "- ") }

func (p *parser) node(indent int) (*Node, error) {
	if isItem(p.lines[p.pos].text) {
		return p.sequence(indent)
	}
	return p.mapping(indent)
}

func (p *parser) mapping(indent int) (*Node, error) {
	n := &Node{Kind: Mapping, Line: p.lines[p.pos].num, Map: map[string]*Node{}}
	for p.pos < len(p.lines) {
		l := p.lines[p.pos]
		if l.indent < indent {
			break
		}
		if l.indent > indent || isItem(l.text) {
			return nil, p.fail(l.num, "misaligned mapping entry")
		}
		key, rest, ok, err := splitKey(l.text)
		if err != nil {
			return nil, p.fail(l.num, err.Error())
		}
		if !ok {
			return nil, p.fail(l.num, "expected key: value")
		}
		if _, dup := n.Map[key]; dup {
			return nil, p.fail(l.num, "duplicate key "+key)
		}
		p.pos++
		value, err := p.value(l, indent, rest)
		if err != nil {
			return nil, err
		}
		n.Keys = append(n.Keys, key)
		n.Map[key] = value
	}
	return n, nil
}

func (p *parser) sequence(indent int) (*Node, error) {
	n := &Node{Kind: Sequence, Line: p.lines[p.pos].num}
	for p.pos < len(p.lines) {
		l := p.lines[p.pos]
		if l.indent < indent {
			break
		}
		if l.indent > indent || !isItem(l.text) {
			return nil, p.fail(l.num, "misaligned sequence item")
		}
		content := strings.TrimLeft(strings.TrimPrefix(l.text, "-"), " ")
		if content == "" {
			p.pos++
			child, err := p.child(l, indent)
			if err != nil {
				return nil, err
			}
			n.Items = append(n.Items, child)
			continue
		}
		if _, _, ok, err := splitKey(content); err != nil {
			return nil, p.fail(l.num, err.Error())
		} else if ok || isItem(content) {
			// Re-read the item's content as a nested block at its own column.
			column := l.indent + (len(l.text) - len(content))
			p.lines[p.pos] = line{num: l.num, indent: column, text: content, raw: l.raw}
			child, err := p.node(column)
			if err != nil {
				return nil, err
			}
			n.Items = append(n.Items, child)
			continue
		}
		p.pos++
		value, err := p.value(l, indent, content)
		if err != nil {
			return nil, err
		}
		n.Items = append(n.Items, value)
	}
	return n, nil
}

// child parses the nested block that follows a key or item with no inline value.
func (p *parser) child(parent line, indent int) (*Node, error) {
	if p.pos < len(p.lines) {
		next := p.lines[p.pos]
		if next.indent > indent || (next.indent == indent && isItem(next.text) && !isItem(parent.text)) {
			return p.node(next.indent)
		}
	}
	return &Node{Kind: Scalar, Line: parent.num}, nil
}

func (p *parser) value(l line, indent int, rest string) (*Node, error) {
	switch {
	case rest == "":
		return p.child(l, indent)
	case rest == "|" || rest == "|-":
		return p.literal(l, indent, rest == "|-")
	}
	return inlineScalar(l.num, rest)
}

func (p *parser) literal(l line, indent int, strip bool) (*Node, error) {
	// Content lines were also scanned as structure; they belong to the literal.
	next := p.pos
	for next < len(p.lines) && p.lines[next].indent > indent {
		next++
	}
	end := len(p.all) // exclusive 0-based bound in p.all
	if next < len(p.lines) {
		end = p.lines[next].num - 1
	}
	p.pos = next
	column := -1
	var out []string
	for i := l.num; i < end; i++ { // l.num is the 0-based index of the following line
		raw := p.all[i]
		if strings.TrimSpace(raw) == "" {
			out = append(out, "")
			continue
		}
		lead := len(raw) - len(strings.TrimLeft(raw, " "))
		if column < 0 {
			column = lead
		}
		if lead < column {
			if lead <= indent && strings.HasPrefix(strings.TrimSpace(raw), "#") {
				// A comment before the next structural line ends the literal;
				// only blanks and comments may follow it inside this span.
				for _, after := range p.all[i:end] {
					if t := strings.TrimSpace(after); t != "" && !strings.HasPrefix(t, "#") {
						return nil, p.fail(i+1, "literal block resumes after comment")
					}
				}
				break
			}
			return nil, p.fail(i+1, "literal block dedent")
		}
		out = append(out, raw[column:])
	}
	if column >= 0 && column <= indent {
		return nil, p.fail(l.num, "literal block not indented")
	}
	text := strings.TrimRight(strings.Join(out, "\n"), "\n")
	if !strip && text != "" {
		text += "\n"
	}
	return &Node{Kind: Scalar, Line: l.num, Text: text}, nil
}

func inlineScalar(num int, text string) (*Node, error) {
	switch {
	case text == "{}":
		return &Node{Kind: Mapping, Line: num, Map: map[string]*Node{}}, nil
	case strings.HasPrefix(text, "["):
		if !strings.HasSuffix(text, "]") {
			return nil, fmt.Errorf("%w: line %d: unterminated flow sequence", ErrUnsupportedYAML, num)
		}
		n := &Node{Kind: Sequence, Line: num}
		body := strings.TrimSpace(text[1 : len(text)-1])
		if body == "" {
			return n, nil
		}
		for _, part := range strings.Split(body, ",") {
			item, err := quotedOrPlain(num, strings.TrimSpace(part))
			if err != nil {
				return nil, err
			}
			if strings.ContainsAny(item.Text, "[]{}") {
				return nil, fmt.Errorf("%w: line %d: nested flow collection", ErrUnsupportedYAML, num)
			}
			n.Items = append(n.Items, item)
		}
		return n, nil
	}
	return quotedOrPlain(num, text)
}

func quotedOrPlain(num int, text string) (*Node, error) {
	bad := func(why string) (*Node, error) {
		return nil, fmt.Errorf("%w: line %d: %s", ErrUnsupportedYAML, num, why)
	}
	if text == "" {
		return bad("empty flow item")
	}
	switch text[0] {
	case '"':
		if len(text) < 2 || !strings.HasSuffix(text, `"`) {
			return bad("unterminated double-quoted scalar")
		}
		inner := text[1 : len(text)-1]
		var b strings.Builder
		for i := 0; i < len(inner); i++ {
			c := inner[i]
			if c == '"' {
				return bad("unescaped quote")
			}
			if c == '\\' {
				if i+1 >= len(inner) || !strings.ContainsRune(`"\`, rune(inner[i+1])) {
					return bad("unsupported escape")
				}
				i++
				c = inner[i]
			}
			b.WriteByte(c)
		}
		return &Node{Kind: Scalar, Line: num, Text: b.String()}, nil
	case '\'':
		if len(text) < 2 || !strings.HasSuffix(text, "'") {
			return bad("unterminated single-quoted scalar")
		}
		inner := text[1 : len(text)-1]
		if strings.Contains(strings.ReplaceAll(inner, "''", ""), "'") {
			return bad("unescaped quote")
		}
		return &Node{Kind: Scalar, Line: num, Text: strings.ReplaceAll(inner, "''", "'")}, nil
	case '&', '*', '!', '{', '>', '|', '%', '@', '`', '?', ',', ']', '}':
		return bad("unsupported indicator " + text[:1])
	}
	if strings.Contains(text, ": ") || strings.HasSuffix(text, ":") {
		return bad("ambiguous plain scalar")
	}
	return &Node{Kind: Scalar, Line: num, Text: text}, nil
}

// splitKey splits `key: rest` outside quotes. ok is false for a plain scalar.
func splitKey(text string) (key, rest string, ok bool, err error) {
	if isItem(text) || text == "" {
		return "", "", false, nil
	}
	switch text[0] {
	case '"', '\'', '[', '{', '&', '*', '!', '?', '|', '>':
		if text[0] == '?' || text[0] == '&' || text[0] == '*' || text[0] == '!' {
			return "", "", false, errors.New("unsupported indicator " + text[:1])
		}
		return "", "", false, nil
	}
	for i := 0; i < len(text); i++ {
		if text[i] == ':' && (i+1 == len(text) || text[i+1] == ' ') {
			key = text[:i]
			if strings.ContainsAny(key, `"'{}[]&*!|>#`) || strings.TrimSpace(key) != key || key == "" {
				return "", "", false, errors.New("unsupported key")
			}
			if strings.Contains(key, "${{") {
				return "", "", false, nil
			}
			return key, strings.TrimSpace(text[i+1:]), true, nil
		}
	}
	return "", "", false, nil
}

// stripComment removes a `#` comment that starts the line or follows a space,
// outside quoted text, and right-trims the result.
func stripComment(body string) (string, error) {
	quote := byte(0)
	for i := 0; i < len(body); i++ {
		c := body[i]
		switch {
		case quote != 0:
			if c == quote {
				quote = 0
			}
		case c == '"' || c == '\'':
			if i == 0 || body[i-1] == ' ' || body[i-1] == '[' || body[i-1] == ',' {
				quote = c
			}
		case c == '#' && (i == 0 || body[i-1] == ' '):
			return strings.TrimRight(body[:i], " "), nil
		}
	}
	return strings.TrimRight(body, " "), nil
}
