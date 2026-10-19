package appmap

import (
	"strings"

	"github.com/Beamfall/corvint/internal/contextindex"
)

// This file reads the lexical class, module and method structure of one Ruby file after the
// AMAP-V0-105 masking (comments, strings, heredoc bodies and literals blanked), for the route-action
// check (AMAP-V0-102..105) and the branch query (AMAP-V0-111..114). It is a nesting reader, not a
// Ruby grammar: a file whose openers and `end`s do not balance is marked broken and callers abstain.

type rubyDef struct {
	name       string
	line, end  int
	visibility string // public, private or protected
	instance   bool   // an instance method of its scope (not singleton, not nested in a body)
	scope      *rubyScope
}

type rubyRef struct {
	name string
	line int
}

type rubyScope struct {
	kind      string // class or module
	name      string // full constant path
	nesting   []string
	super     string
	superLine int
	line, end int
	defs      []*rubyDef
	includes  []rubyRef // an empty name is a non-constant include
	dynamic   []rubyRef // a construct that can define methods at run time
	hidden    map[string]string
	parts     []*rubyScope // the reopenings a merged scope joins (rubyFile.scope)
}

type rubyFile struct {
	path   string
	code   []string // masked lines
	raw    []string // original lines
	scopes []*rubyScope
	defs   []*rubyDef
	broken int // first line where the nesting failed; 0 when it balanced
}

type rubyFrame struct {
	kind       string // class, module, def, block, other, singleton, included, classmethods
	scope      *rubyScope
	def        *rubyDef
	visibility string
}

var rubyDynamicAnywhere = map[string]bool{"define_method": true, "class_eval": true, "instance_eval": true, "module_eval": true, "class_exec": true, "instance_exec": true}
var rubyDynamicBody = map[string]bool{"alias_method": true, "alias": true, "send": true, "public_send": true, "attr_reader": true, "attr_writer": true, "attr_accessor": true, "attr": true, "delegate": true}

func rubyIdentByte(c byte) bool {
	return c == '_' || c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9'
}

// rubyWords returns the identifier-like words of masked code with their offsets, a trailing `?`
// or `!` included. Words after `.`, `@`, `$` or a single `:` and words followed by a single `:`
// (keyword arguments, symbols) are marked as non-keyword positions.
type rubyWord struct {
	text    string
	at      int
	keyword bool
}

func rubyWordsOf(code string) []rubyWord {
	out := []rubyWord{}
	for i := 0; i < len(code); {
		c := code[i]
		if !rubyIdentByte(c) || c >= '0' && c <= '9' {
			i++
			continue
		}
		start := i
		for i < len(code) && rubyIdentByte(code[i]) {
			i++
		}
		if i < len(code) && (code[i] == '?' || code[i] == '!') && (i+1 >= len(code) || code[i+1] != '=') {
			i++
		}
		keyword := true
		if start > 0 {
			p := code[start-1]
			if p == '.' || p == '@' || p == '$' || p == ':' && (start < 2 || code[start-2] != ':') {
				keyword = false
			}
		}
		if i < len(code) && code[i] == ':' && (i+1 >= len(code) || code[i+1] != ':') {
			keyword = false
		}
		out = append(out, rubyWord{text: code[start:i], at: start, keyword: keyword})
	}
	return out
}

// rubyConstantAt reads a constant path (`A::B`, optionally `::A`) at code[i:]. A leading `::`
// is kept: it makes the reference top-level only (AMAP-V0-103).
func rubyConstantAt(code string, i int) (string, int) {
	for i < len(code) && code[i] == ' ' {
		i++
	}
	start := i
	if strings.HasPrefix(code[i:], "::") {
		i += 2
	}
	for {
		if i >= len(code) || code[i] < 'A' || code[i] > 'Z' {
			return "", start
		}
		for i < len(code) && rubyIdentByte(code[i]) {
			i++
		}
		if !strings.HasPrefix(code[i:], "::") {
			return code[start:i], i
		}
		i += 2
	}
}

// rubyDefName reads `def [self.]name` at code[i:] (i after `def`): the name, whether it is a
// singleton definition and whether it is endless (`def x = expr`).
func rubyDefName(code string, i int) (name string, singleton, endless bool) {
	for i < len(code) && code[i] == ' ' {
		i++
	}
	if strings.HasPrefix(code[i:], "self.") {
		singleton = true
		i += 5
	}
	start := i
	for i < len(code) && (rubyIdentByte(code[i]) || strings.IndexByte("?!=<>+-*/%[]~^&|", code[i]) >= 0) {
		if code[i] == '=' && (i+1 >= len(code) || code[i+1] != '(') && i > start && rubyIdentByte(code[i-1]) {
			break
		}
		i++
	}
	name = code[start:i]
	if i < len(code) && code[i] == '.' {
		// def obj.method: a singleton definition on another object.
		return name, true, false
	}
	j := i
	for j < len(code) && code[j] == ' ' {
		j++
	}
	if j < len(code) && code[j] == '(' {
		depth := 0
		for ; j < len(code); j++ {
			if code[j] == '(' {
				depth++
			} else if code[j] == ')' {
				depth--
				if depth == 0 {
					j++
					break
				}
			}
		}
		for j < len(code) && code[j] == ' ' {
			j++
		}
	}
	endless = j < len(code) && code[j] == '=' && (j+1 >= len(code) || !strings.ContainsRune("=~>", rune(code[j+1])))
	return name, singleton, endless
}

func parseRubyStructure(p string, data []byte) *rubyFile {
	f := &rubyFile{path: p, code: contextindex.FlowCodeLines("ruby", string(data)), raw: strings.Split(string(data), "\n")}
	stack := []*rubyFrame{}
	body := func() *rubyFrame {
		if len(stack) == 0 {
			return nil
		}
		return stack[len(stack)-1]
	}
	nesting := func() []string {
		out := []string{}
		for i := len(stack) - 1; i >= 0; i-- {
			if k := stack[i].kind; k == "class" || k == "module" {
				out = append(out, stack[i].scope.name)
			}
		}
		return out
	}
	innerScope := func() *rubyScope {
		for i := len(stack) - 1; i >= 0; i-- {
			if stack[i].scope != nil {
				return stack[i].scope
			}
		}
		return nil
	}
	fail := func(line int) {
		if f.broken == 0 {
			f.broken = line
		}
	}
	raw := strings.Split(string(data), "\n")
	for n, full := range f.code {
		line := n + 1
		if full == "__END__" {
			break
		}
		rawLine, offset := "", 0
		if n < len(raw) {
			rawLine = raw[n]
		}
		for _, piece := range strings.Split(full, ";") {
			lead := offset + len(piece) - len(strings.TrimLeft(piece, " \t"))
			offset += len(piece) + 1
			stmt := strings.TrimSpace(piece)
			if stmt == "" {
				continue
			}
			words := rubyWordsOf(stmt)
			if len(words) == 0 {
				continue
			}
			top := body()
			bodyLevel := top != nil && (top.kind == "class" || top.kind == "module" || top.kind == "included")
			first := words[0]
			visibility := ""
			if bodyLevel && first.at == 0 && first.keyword {
				switch first.text {
				case "private", "protected", "public":
					rest := strings.TrimSpace(stmt[len(first.text):])
					switch {
					case rest == "":
						top.visibility = first.text
					case strings.HasPrefix(rest, "def ") || strings.HasPrefix(rest, "def("):
						visibility = first.text
					case strings.HasPrefix(rest, ":") || strings.HasPrefix(rest, "("):
						for _, w := range rubyWordsOf(rest) {
							if !w.keyword && w.at > 0 && rest[w.at-1] == ':' {
								top.scope.hidden[w.text] = first.text
							}
						}
					}
				case "include":
					rest := strings.TrimSpace(stmt[len(first.text):])
					rest = strings.TrimSuffix(strings.TrimPrefix(rest, "("), ")")
					// `include A, B` puts A before B in the ancestors, as `include B` then
					// `include A` would; includes are stored in that equivalent statement order.
					parts := strings.Split(rest, ",")
					for k := len(parts) - 1; k >= 0; k-- {
						part := strings.TrimSpace(parts[k])
						name, end := rubyConstantAt(part, 0)
						if name == "" || strings.TrimSpace(part[end:]) != "" {
							name = ""
						}
						top.scope.includes = append(top.scope.includes, rubyRef{name: name, line: line})
					}
				default:
					if rubyDynamicBody[first.text] {
						top.scope.dynamic = append(top.scope.dynamic, rubyRef{name: first.text, line: line})
					}
				}
			}
			loopDo := first.keyword && (first.text == "while" || first.text == "until" || first.text == "for")
			for wi, w := range words {
				if !w.keyword {
					continue
				}
				if rubyDynamicAnywhere[w.text] {
					if s := innerScope(); s != nil {
						s.dynamic = append(s.dynamic, rubyRef{name: w.text, line: line})
					}
				}
				switch w.text {
				case "class", "module":
					if w.text == "class" && strings.HasPrefix(strings.TrimSpace(stmt[w.at+5:]), "<<") {
						stack = append(stack, &rubyFrame{kind: "singleton"})
						continue
					}
					name, end := rubyConstantAt(stmt, w.at+len(w.text))
					if name == "" {
						stack = append(stack, &rubyFrame{kind: "other"})
						continue
					}
					name = strings.TrimPrefix(name, "::")
					outer := nesting()
					full := name
					if len(outer) > 0 && !strings.HasPrefix(strings.TrimSpace(stmt[w.at+len(w.text):]), "::") {
						full = outer[0] + "::" + name
					}
					s := &rubyScope{kind: w.text, name: full, nesting: append([]string{full}, outer...), line: line, hidden: map[string]string{}}
					if w.text == "class" {
						rest := strings.TrimSpace(stmt[end:])
						if strings.HasPrefix(rest, "<") && !strings.HasPrefix(rest, "<<") {
							super, after := rubyConstantAt(rest, 1)
							if super == "" || strings.TrimSpace(rest[after:]) != "" {
								super = "?"
							}
							s.super, s.superLine = super, line
						}
					}
					f.scopes = append(f.scopes, s)
					stack = append(stack, &rubyFrame{kind: w.text, scope: s, visibility: "public"})
				case "def":
					name, singleton, endless := rubyDefName(stmt, w.at+3)
					d := &rubyDef{name: name, line: line, end: line, visibility: "public"}
					if t := body(); t != nil {
						switch t.kind {
						case "class", "module", "included":
							d.scope, d.instance = t.scope, !singleton
							d.visibility = t.visibility
							if visibility != "" && wi == 1 {
								d.visibility = visibility
							}
						case "singleton", "classmethods":
							d.scope = innerScope()
						default:
							d.scope = innerScope()
							// A def under a body-level conditional, loop, begin or block (not
							// inside a method) may define an instance method at run time.
							if s := conditionalBody(stack); s != nil {
								s.dynamic = append(s.dynamic, rubyRef{name: "def", line: line})
							}
						}
					}
					if d.scope != nil {
						d.scope.defs = append(d.scope.defs, d)
						if name == "method_missing" || name == "action_missing" || name == "respond_to_missing?" {
							d.scope.dynamic = append(d.scope.dynamic, rubyRef{name: name, line: line})
						}
					}
					f.defs = append(f.defs, d)
					if !endless {
						stack = append(stack, &rubyFrame{kind: "def", def: d})
					}
				case "end":
					if len(stack) == 0 {
						fail(line)
						continue
					}
					t := stack[len(stack)-1]
					stack = stack[:len(stack)-1]
					if t.scope != nil && (t.kind == "class" || t.kind == "module") {
						t.scope.end = line
					}
					if t.def != nil {
						t.def.end = line
					}
				case "do":
					if loopDo {
						loopDo = false
						continue
					}
					fr := &rubyFrame{kind: "block"}
					if t := body(); t != nil && t.kind == "module" && wi == 1 && first.keyword {
						switch first.text {
						case "included":
							fr = &rubyFrame{kind: "included", scope: t.scope, visibility: "public"}
						case "class_methods":
							fr = &rubyFrame{kind: "classmethods", scope: t.scope}
						}
					}
					stack = append(stack, fr)
				case "if", "unless", "while", "until":
					if wi == 0 || strings.ContainsRune("=(,[{|&!", rune(codeBefore(full, rawLine, lead+w.at))) {
						stack = append(stack, &rubyFrame{kind: "other"})
					}
				case "case", "begin":
					stack = append(stack, &rubyFrame{kind: "other"})
				case "for":
					if wi == 0 {
						stack = append(stack, &rubyFrame{kind: "other"})
					}
				}
			}
		}
	}
	if len(stack) != 0 {
		fail(len(f.code))
	}
	return f
}

// conditionalBody returns the class or module whose body directly holds the open conditional,
// loop, begin or block frames at the top of the stack, or nil when a method, singleton or
// class_methods frame comes first.
func conditionalBody(stack []*rubyFrame) *rubyScope {
	for i := len(stack) - 1; i >= 0; i-- {
		switch stack[i].kind {
		case "class", "module", "included":
			return stack[i].scope
		case "other", "block":
			continue
		default:
			return nil
		}
	}
	return nil
}

// codeBefore returns the last code byte before column at of a masked line, or '"' when a masked
// literal ends there (an expression value): masking blanks a whole literal, quotes included.
func codeBefore(code, raw string, at int) byte {
	for i := at - 1; i >= 0 && i < len(code); i-- {
		if c := code[i]; c != ' ' && c != '\t' {
			return c
		}
		if i < len(raw) && raw[i] != ' ' && raw[i] != '\t' {
			return '"'
		}
	}
	return 0
}

// scope returns the scope with this full constant name, or nil. A class or module reopened in
// the same file is one scope, as in Ruby: its parts' definitions, includes, dynamic constructs
// and visibility changes join in file order, and parts lists each reopening.
func (f *rubyFile) scope(name string) *rubyScope {
	var merged *rubyScope
	for _, s := range f.scopes {
		if s.name != name {
			continue
		}
		if merged == nil {
			merged = &rubyScope{kind: s.kind, name: s.name, nesting: s.nesting, line: s.line, hidden: map[string]string{}}
		}
		if merged.super == "" {
			merged.super, merged.superLine = s.super, s.superLine
		}
		merged.end = s.end
		merged.defs = append(merged.defs, s.defs...)
		merged.includes = append(merged.includes, s.includes...)
		merged.dynamic = append(merged.dynamic, s.dynamic...)
		for k, v := range s.hidden {
			merged.hidden[k] = v
		}
		merged.parts = append(merged.parts, s)
	}
	return merged
}

// enclosing returns the innermost definition and scope spanning a line.
func (f *rubyFile) enclosing(line int) (*rubyDef, *rubyScope) {
	var def *rubyDef
	for _, d := range f.defs {
		if d.line <= line && line <= d.end && (def == nil || d.line >= def.line) {
			def = d
		}
	}
	var scope *rubyScope
	for _, s := range f.scopes {
		if s.line <= line && line <= s.end && (scope == nil || s.line >= scope.line) {
			scope = s
		}
	}
	return def, scope
}

// rubyUnderscore converts a constant path to its Zeitwerk file path (`Admin::UserPhotos` ->
// `admin/user_photos`); acronyms split per capital, which the convention also expects.
func rubyUnderscore(constant string) string {
	var b strings.Builder
	for i, part := range strings.Split(constant, "::") {
		if i > 0 {
			b.WriteByte('/')
		}
		for j := 0; j < len(part); j++ {
			c := part[j]
			if c >= 'A' && c <= 'Z' {
				if j > 0 && (part[j-1] < 'A' || part[j-1] > 'Z' || j+1 < len(part) && part[j+1] >= 'a' && part[j+1] <= 'z') && part[j-1] != '_' {
					b.WriteByte('_')
				}
				c += 'a' - 'A'
			}
			b.WriteByte(c)
		}
	}
	return b.String()
}

// rubyCamelize converts a file path segment list to its constant (`admin/user_photos` ->
// `Admin::UserPhotos`).
func rubyCamelize(p string) string {
	parts := strings.Split(p, "/")
	for i, part := range parts {
		var b strings.Builder
		up := true
		for j := 0; j < len(part); j++ {
			c := part[j]
			if c == '_' {
				up = true
				continue
			}
			if up && c >= 'a' && c <= 'z' {
				c -= 'a' - 'A'
			}
			up = false
			b.WriteByte(c)
		}
		parts[i] = b.String()
	}
	return strings.Join(parts, "::")
}
