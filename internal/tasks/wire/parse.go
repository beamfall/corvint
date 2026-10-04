package wire

import (
	"fmt"
	"unicode/utf8"
)

// Parse decodes one canonical taskman document: a single JSON value with no
// insignificant whitespace, sorted keys, canonical escapes and exactly one
// trailing LF (SPEC §2, WQO §4.1). It enforces the §1 decode bounds (depth,
// array elements, aggregate nodes), refuses duplicate keys, JSON numbers,
// invalid UTF-8, lone surrogates and hostile code points, and then proves
// canonical framing by re-encoding the value and comparing bytes.
func Parse(data []byte) (Value, error) {
	return ParseWith(data, ParseOptions{})
}

// ParseOptions is the opt-in widening a single profile may request. Only
// the array that is the direct value of the top-level key WideArrayKey is
// bounded by WideArrayMax instead of MaxJSONArrayElements; every other
// array, at any depth, keeps the ordinary bound. MaxNodes replaces
// MaxJSONNodes for the whole document. Zero values select the defaults, so
// ParseOptions{} is Parse. Depth is never widened.
type ParseOptions struct {
	WideArrayKey string
	WideArrayMax int
	MaxNodes     int
}

// ParseWith is Parse under explicit decode bounds (§3.5: the
// taskman-archive/0 manifest is the only profile that uses it). The bounds
// are enforced incrementally while parsing, before the document is
// materialized; a document over a bound fails LIMIT_EXCEEDED at the first
// element or node beyond it.
func ParseWith(data []byte, opts ParseOptions) (Value, error) {
	if opts.WideArrayMax <= 0 {
		opts.WideArrayKey = ""
		opts.WideArrayMax = MaxJSONArrayElements
	}
	if opts.MaxNodes <= 0 {
		opts.MaxNodes = MaxJSONNodes
	}
	if len(data) == 0 {
		return Value{}, Errorf(CodeMalformed, "byte 0", "empty document")
	}
	if !utf8.Valid(data) {
		return Value{}, Errorf(CodeMalformed, "", "invalid UTF-8")
	}
	if len(data) >= 3 && data[0] == 0xEF && data[1] == 0xBB && data[2] == 0xBF {
		return Value{}, Errorf(CodeMalformed, "byte 0", "byte order mark is a hostile code point")
	}
	if data[len(data)-1] != '\n' {
		return Value{}, Errorf(CodeMalformed, fmt.Sprintf("byte %d", len(data)), "missing the single trailing LF")
	}
	body := data[:len(data)-1]
	p := &parser{data: body, opts: opts}
	v, err := p.value()
	if err != nil {
		return Value{}, err
	}
	if p.pos != len(body) {
		return Value{}, Errorf(CodeMalformed, p.where(), "trailing bytes after the document")
	}
	enc := Encode(v)
	if string(enc) != string(body) {
		off := 0
		for off < len(enc) && off < len(body) && enc[off] == body[off] {
			off++
		}
		// Whitespace is refused while parsing, so only key order or an escape
		// form can differ here; name the one that does (V1-0750).
		if path, ok := unsortedKeys(v, ""); ok {
			return Value{}, Errorf(CodeMalformed, fmt.Sprintf("byte %d", off), "non-canonical key order: the keys of the object at %q must be sorted in UTF-8 byte order", path)
		}
		return Value{}, Errorf(CodeMalformed, fmt.Sprintf("byte %d", off), "non-canonical string escape form: write literal UTF-8, use only the \\t \\n \\r \\\" \\\\ short escapes, and lowercase \\u00xx for any other control")
	}
	return v, nil
}

// unsortedKeys returns the JSON pointer of the first object, in document
// order, whose keys were parsed out of UTF-8 byte order.
func unsortedKeys(v Value, path string) (string, bool) {
	switch v.Kind {
	case KindObject:
		if v.Obj == nil {
			return "", false
		}
		for i := 1; i < len(v.Obj.Keys); i++ {
			if v.Obj.Keys[i] < v.Obj.Keys[i-1] {
				if path == "" {
					return "/", true
				}
				return path, true
			}
		}
		for _, k := range v.Obj.Keys {
			if p, ok := unsortedKeys(v.Obj.Vals[k], path+"/"+k); ok {
				return p, true
			}
		}
	case KindArray:
		for i, e := range v.Arr {
			if p, ok := unsortedKeys(e, path+"/"+itoa(i)); ok {
				return p, true
			}
		}
	}
	return "", false
}

// ParseInput decodes one caller-supplied JSON value under the same value
// rules as Parse (decode bounds, duplicate keys, no JSON numbers, valid UTF-8,
// no lone surrogates or hostile code points) but without the canonical
// framing rules: insignificant JSON whitespace (space, TAB, LF, CR) may
// surround any token, object keys may appear in any order, any valid JSON
// escape form is decoded, and the trailing LF is optional. It is the CLI input
// boundary only (V1-0750): the caller re-encodes the value with Encode, so
// stored records, envelopes and digests stay canonical. Array order is
// preserved exactly; sorting a set is the schema's decision, never the parser's.
func ParseInput(data []byte) (Value, error) {
	if len(data) == 0 {
		return Value{}, Errorf(CodeMalformed, "byte 0", "empty document")
	}
	if !utf8.Valid(data) {
		return Value{}, Errorf(CodeMalformed, "", "invalid UTF-8")
	}
	if len(data) >= 3 && data[0] == 0xEF && data[1] == 0xBB && data[2] == 0xBF {
		return Value{}, Errorf(CodeMalformed, "byte 0", "byte order mark is a hostile code point")
	}
	p := &parser{data: data, opts: ParseOptions{WideArrayMax: MaxJSONArrayElements, MaxNodes: MaxJSONNodes}, lenient: true}
	v, err := p.value()
	if err != nil {
		return Value{}, err
	}
	p.ws()
	if p.pos != len(data) {
		return Value{}, Errorf(CodeMalformed, p.where(), "trailing bytes after the document")
	}
	return v, nil
}

type parser struct {
	data    []byte
	pos     int
	depth   int
	nodes   int
	opts    ParseOptions
	lenient bool
	// wide is set while the value of the top-level WideArrayKey member is
	// being parsed; only an array opened directly at that point (depth 2)
	// takes the wide bound.
	wide bool
}

func (p *parser) where() string { return fmt.Sprintf("byte %d", p.pos) }

func isJSONSpace(c byte) bool { return c == ' ' || c == '\t' || c == '\n' || c == '\r' }

// ws skips insignificant whitespace in lenient (ParseInput) mode only.
func (p *parser) ws() {
	for p.lenient && p.pos < len(p.data) && isJSONSpace(p.data[p.pos]) {
		p.pos++
	}
}

// expected reports a structural mismatch. Where the offending byte is
// whitespace it names whitespace, the only defect a pretty-printed but
// otherwise valid document has (V1-0750), instead of the token it expected.
func (p *parser) expected(what string) error {
	if p.pos < len(p.data) && isJSONSpace(p.data[p.pos]) {
		return p.fail("insignificant whitespace is not canonical (expected %s)", what)
	}
	return p.fail("expected %s", what)
}

func (p *parser) fail(format string, args ...interface{}) error {
	return Errorf(CodeMalformed, p.where(), format, args...)
}

func (p *parser) node() error {
	p.nodes++
	if p.nodes > p.opts.MaxNodes {
		return Errorf(CodeLimitExceeded, p.where(), "more than %d decoded nodes", p.opts.MaxNodes)
	}
	return nil
}

func (p *parser) value() (Value, error) {
	p.ws()
	if err := p.node(); err != nil {
		return Value{}, err
	}
	if p.pos >= len(p.data) {
		return Value{}, p.fail("unexpected end of document")
	}
	switch c := p.data[p.pos]; {
	case c == '{':
		return p.object()
	case c == '[':
		return p.array()
	case c == '"':
		s, err := p.str()
		if err != nil {
			return Value{}, err
		}
		return String(s), nil
	case c == 't':
		return p.literal("true", Bool(true))
	case c == 'f':
		return p.literal("false", Bool(false))
	case c == 'n':
		return p.literal("null", Null())
	case c == '-' || (c >= '0' && c <= '9'):
		return Value{}, p.fail("JSON numbers are not used by any taskman profile; Count and Size are decimal strings")
	case isJSONSpace(c):
		return Value{}, p.fail("insignificant whitespace is not canonical")
	default:
		return Value{}, p.fail("unexpected byte 0x%02x", c)
	}
}

func (p *parser) literal(lit string, v Value) (Value, error) {
	if len(p.data)-p.pos < len(lit) || string(p.data[p.pos:p.pos+len(lit)]) != lit {
		return Value{}, p.fail("invalid literal")
	}
	p.pos += len(lit)
	return v, nil
}

func (p *parser) enter() error {
	p.depth++
	if p.depth > MaxJSONDepth {
		return Errorf(CodeLimitExceeded, p.where(), "nesting deeper than %d", MaxJSONDepth)
	}
	return nil
}

func (p *parser) object() (Value, error) {
	if err := p.enter(); err != nil {
		return Value{}, err
	}
	p.pos++ // '{'
	obj := NewObject()
	p.ws()
	if p.pos < len(p.data) && p.data[p.pos] == '}' {
		p.pos++
		p.depth--
		return ObjectValue(obj), nil
	}
	for {
		p.ws()
		if p.pos >= len(p.data) || p.data[p.pos] != '"' {
			return Value{}, p.expected("object key")
		}
		key, err := p.str()
		if err != nil {
			return Value{}, err
		}
		if _, dup := obj.Vals[key]; dup {
			return Value{}, p.fail("duplicate key %q", key)
		}
		p.ws()
		if p.pos >= len(p.data) || p.data[p.pos] != ':' {
			return Value{}, p.expected(fmt.Sprintf("':' after key %q", key))
		}
		p.pos++
		p.wide = p.depth == 1 && p.opts.WideArrayKey != "" && key == p.opts.WideArrayKey
		v, err := p.value()
		p.wide = false
		if err != nil {
			return Value{}, err
		}
		obj.Set(key, v)
		p.ws()
		if p.pos >= len(p.data) {
			return Value{}, p.fail("unterminated object")
		}
		switch p.data[p.pos] {
		case ',':
			p.pos++
		case '}':
			p.pos++
			p.depth--
			return ObjectValue(obj), nil
		default:
			return Value{}, p.expected("',' or '}'")
		}
	}
}

func (p *parser) array() (Value, error) {
	if err := p.enter(); err != nil {
		return Value{}, err
	}
	p.pos++ // '['
	// The wide bound applies only to the array that is itself the value of
	// the top-level wide key (opened at depth 2 while wide is set); the flag
	// is cleared here so nested arrays inside it keep the ordinary bound.
	bound := MaxJSONArrayElements
	if p.wide && p.depth == 2 {
		bound = p.opts.WideArrayMax
	}
	p.wide = false
	arr := []Value{}
	p.ws()
	if p.pos < len(p.data) && p.data[p.pos] == ']' {
		p.pos++
		p.depth--
		return Array(arr...), nil
	}
	for {
		v, err := p.value()
		if err != nil {
			return Value{}, err
		}
		arr = append(arr, v)
		if len(arr) > bound {
			return Value{}, Errorf(CodeLimitExceeded, p.where(), "array longer than %d elements", bound)
		}
		p.ws()
		if p.pos >= len(p.data) {
			return Value{}, p.fail("unterminated array")
		}
		switch p.data[p.pos] {
		case ',':
			p.pos++
		case ']':
			p.pos++
			p.depth--
			return Array(arr...), nil
		default:
			return Value{}, p.expected("',' or ']'")
		}
	}
}

func hexVal(b byte) (int, bool) {
	switch {
	case b >= '0' && b <= '9':
		return int(b - '0'), true
	case b >= 'a' && b <= 'f':
		return int(b-'a') + 10, true
	case b >= 'A' && b <= 'F':
		return int(b-'A') + 10, true
	}
	return 0, false
}

func (p *parser) hex4() (rune, bool) {
	if len(p.data)-p.pos < 4 {
		return 0, false
	}
	var r rune
	for i := 0; i < 4; i++ {
		h, ok := hexVal(p.data[p.pos+i])
		if !ok {
			return 0, false
		}
		r = r<<4 | rune(h)
	}
	p.pos += 4
	return r, true
}

// str parses a JSON string starting at the opening quote and returns the
// decoded text. Every decoded rune is checked against the hostile code point
// list; TAB, LF and CR survive here and are refused later by the field kinds
// that forbid them (identifiers, labels, paths).
func (p *parser) str() (string, error) {
	start := p.pos
	p.pos++ // opening quote
	out := make([]byte, 0, 32)
	for {
		if p.pos >= len(p.data) {
			return "", Errorf(CodeMalformed, fmt.Sprintf("byte %d", start), "unterminated string")
		}
		c := p.data[p.pos]
		switch {
		case c == '"':
			p.pos++
			s := string(out)
			if err := checkHostile(s, fmt.Sprintf("byte %d", start), true); err != nil {
				return "", err
			}
			return s, nil
		case c == '\\':
			p.pos++
			if p.pos >= len(p.data) {
				return "", p.fail("unterminated escape")
			}
			e := p.data[p.pos]
			p.pos++
			switch e {
			case '"':
				out = append(out, '"')
			case '\\':
				out = append(out, '\\')
			case '/':
				out = append(out, '/')
			case 'b':
				out = append(out, '\b')
			case 'f':
				out = append(out, '\f')
			case 'n':
				out = append(out, '\n')
			case 'r':
				out = append(out, '\r')
			case 't':
				out = append(out, '\t')
			case 'u':
				r, ok := p.hex4()
				if !ok {
					return "", p.fail("invalid \\u escape")
				}
				if r >= 0xD800 && r <= 0xDBFF {
					// high surrogate: a low surrogate escape must follow
					if len(p.data)-p.pos < 6 || p.data[p.pos] != '\\' || p.data[p.pos+1] != 'u' {
						return "", p.fail("invalid Unicode: lone high surrogate")
					}
					p.pos += 2
					lo, ok := p.hex4()
					if !ok || lo < 0xDC00 || lo > 0xDFFF {
						return "", p.fail("invalid Unicode: lone high surrogate")
					}
					r = 0x10000 + (r-0xD800)<<10 + (lo - 0xDC00)
				} else if r >= 0xDC00 && r <= 0xDFFF {
					return "", p.fail("invalid Unicode: lone low surrogate")
				}
				out = utf8.AppendRune(out, r)
			default:
				return "", p.fail("invalid escape \\%c", e)
			}
		case c < 0x20:
			return "", p.fail("raw control byte 0x%02x inside string", c)
		default:
			out = append(out, c)
			p.pos++
		}
	}
}
