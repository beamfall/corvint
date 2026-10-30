package analyzerhtmlcss

import "strings"

// Closed document grammar: lower-case ASCII, no comments/entities/raw text/optional tags/recovery.
func parseHTML(in input, body []byte, emit func(fact) string) string {
	s := string(body)
	if !printableASCII(s) || !strings.HasPrefix(s, "<html><head>") || !strings.HasSuffix(s, "</body></html>") {
		return "UNSUPPORTED_SCHEMA"
	}
	s = strings.TrimPrefix(s, "<html><head>")
	headEnd := strings.Index(s, "</head><body>")
	if headEnd < 0 || strings.Count(s, "</head><body>") != 1 {
		return "UNSUPPORTED_SCHEMA"
	}
	head, bodyPart := s[:headEnd], s[headEnd+13:len(s)-14]
	for head != "" {
		tag, rest, ok := nextTag(head)
		if !ok {
			return "UNSUPPORTED_SCHEMA"
		}
		head = rest
		name, attrs, closing, ok := parseTag(tag)
		if !ok || closing {
			return "UNSUPPORTED_SCHEMA"
		}
		switch name {
		case "link":
			href, h := attrs.value("href")
			rel, rr := attrs.value("rel")
			if !h || !rr || rel != "stylesheet" || len(attrs) != 2 || !reference(href) {
				return "UNSUPPORTED_SCHEMA"
			}
			if reason := emit(htmlFact("html.stylesheet.link", in, "imports-stylesheet", href)); reason != "" {
				return reason
			}
		case "script":
			src, h := attrs.value("src")
			typ, tt := attrs.value("type")
			if !h || !tt || typ != "module" || len(attrs) != 2 || !reference(src) || !strings.HasPrefix(head, "</script>") {
				return "UNSUPPORTED_SCHEMA"
			}
			head = strings.TrimPrefix(head, "</script>")
			if reason := emit(htmlFact("html.module.import", in, "imports-module", src)); reason != "" {
				return reason
			}
		default:
			return "UNSUPPORTED_SCHEMA"
		}
	}
	for bodyPart != "" {
		tag, rest, ok := nextTag(bodyPart)
		if !ok {
			return "UNSUPPORTED_SCHEMA"
		}
		bodyPart = rest
		name, attrs, closing, ok := parseTag(tag)
		if !ok || closing {
			return "UNSUPPORTED_SCHEMA"
		}
		switch name {
		case "a":
			v, ok := attrs.value("href")
			if !ok || len(attrs) != 1 || !reference(v) || !strings.HasPrefix(bodyPart, "</a>") {
				return "UNSUPPORTED_SCHEMA"
			}
			bodyPart = strings.TrimPrefix(bodyPart, "</a>")
			if reason := emit(htmlFact("html.link.static", in, "links", v)); reason != "" {
				return reason
			}
		case "script":
			src, h := attrs.value("src")
			typ, tt := attrs.value("type")
			if !h || !tt || typ != "module" || len(attrs) != 2 || !reference(src) || !strings.HasPrefix(bodyPart, "</script>") {
				return "UNSUPPORTED_SCHEMA"
			}
			bodyPart = strings.TrimPrefix(bodyPart, "</script>")
			if reason := emit(htmlFact("html.module.import", in, "imports-module", src)); reason != "" {
				return reason
			}
		case "img", "source":
			v, ok := attrs.value("src")
			if !ok || len(attrs) != 1 || !reference(v) {
				return "UNSUPPORTED_SCHEMA"
			}
			if reason := emit(htmlFact("html.asset.reference", in, "references-asset", v)); reason != "" {
				return reason
			}
		case "audio":
			v, ok := attrs.value("src")
			if !ok || len(attrs) != 1 || !reference(v) || !strings.HasPrefix(bodyPart, "</audio>") {
				return "UNSUPPORTED_SCHEMA"
			}
			bodyPart = strings.TrimPrefix(bodyPart, "</audio>")
			if reason := emit(htmlFact("html.asset.reference", in, "references-asset", v)); reason != "" {
				return reason
			}
		case "video":
			v, ok := attrs.value("poster")
			if !ok || len(attrs) != 1 || !reference(v) || !strings.HasPrefix(bodyPart, "</video>") {
				return "UNSUPPORTED_SCHEMA"
			}
			bodyPart = strings.TrimPrefix(bodyPart, "</video>")
			if reason := emit(htmlFact("html.asset.reference", in, "references-asset", v)); reason != "" {
				return reason
			}
		default:
			return "UNSUPPORTED_SCHEMA"
		}
	}
	return ""
}
func htmlFact(kind string, in input, predicate, value string) fact {
	return fact{Kind: kind, InputHandle: in.Handle, RelatedHandle: "-", Subject: in.Path, Predicate: predicate, Value: value, InstanceID: in.Path}
}

type attributes []attribute
type attribute struct{ name, value string }

func (a attributes) value(name string) (string, bool) {
	for _, item := range a {
		if item.name == name {
			return item.value, true
		}
	}
	return "", false
}
func nextTag(s string) (string, string, bool) {
	if !strings.HasPrefix(s, "<") {
		return "", "", false
	}
	end := strings.IndexByte(s, '>')
	if end < 2 {
		return "", "", false
	}
	return s[1:end], s[end+1:], true
}
func parseTag(raw string) (string, attributes, bool, bool) {
	if strings.HasPrefix(raw, "/") {
		return raw[1:], nil, true, htmlName(raw[1:])
	}
	parts := strings.Split(raw, " ")
	if len(parts) == 0 || !htmlName(parts[0]) {
		return "", nil, false, false
	}
	attrs := make(attributes, 0, len(parts)-1)
	for _, part := range parts[1:] {
		pair := strings.SplitN(part, "=", 2)
		if len(pair) != 2 || !htmlName(pair[0]) || len(pair[1]) < 3 || pair[1][0] != '"' || pair[1][len(pair[1])-1] != '"' {
			return "", nil, false, false
		}
		value := pair[1][1 : len(pair[1])-1]
		if value == "" || !printableASCII(value) || strings.ContainsAny(value, "<>&\\\"") {
			return "", nil, false, false
		}
		for _, old := range attrs {
			if old.name == pair[0] {
				return "", nil, false, false
			}
		}
		attrs = append(attrs, attribute{pair[0], value})
	}
	return parts[0], attrs, false, true
}
func htmlName(s string) bool {
	if len(s) == 0 || len(s) > 32 {
		return false
	}
	for _, c := range s {
		if !(c >= 'a' && c <= 'z' || c >= '0' && c <= '9' || c == '-') {
			return false
		}
	}
	return true
}

// CSS admits only exact imports and one-level simple-selector rule blocks. Imports must precede rules.
func parseCSS(in input, body []byte, emit func(fact) string) string {
	s := string(body)
	if !printableASCII(s) || s == "" {
		return "UNSUPPORTED_SCHEMA"
	}
	rules := false
	declarations := make(map[string]string)
	for s != "" {
		if strings.HasPrefix(s, "@import ") {
			if rules {
				return "UNSUPPORTED_SCHEMA"
			}
			end := strings.IndexByte(s, ';')
			if end < 0 {
				return "UNSUPPORTED_SCHEMA"
			}
			v, ok := cssImport(s[8:end])
			if !ok {
				return "UNSUPPORTED_SCHEMA"
			}
			if reason := emit(fact{Kind: "css.import.static", InputHandle: in.Handle, RelatedHandle: "-", Subject: in.Path, Predicate: "imports-stylesheet", Value: v, InstanceID: in.Path}); reason != "" {
				return reason
			}
			s = s[end+1:]
			continue
		}
		rules = true
		open := strings.IndexByte(s, '{')
		if open < 1 || !cssSelector(s[:open]) {
			return "UNSUPPORTED_SCHEMA"
		}
		close := strings.IndexByte(s[open+1:], '}')
		if close < 0 {
			return "UNSUPPORTED_SCHEMA"
		}
		decls := s[open+1 : open+1+close]
		if decls == "" || decls[len(decls)-1] != ';' || strings.ContainsAny(decls, "{}@ ") {
			return "UNSUPPORTED_SCHEMA"
		}
		for _, decl := range strings.Split(decls[:len(decls)-1], ";") {
			if decl == "" {
				return "UNSUPPORTED_SCHEMA"
			}
			pair := strings.SplitN(decl, ":", 2)
			if len(pair) != 2 || strings.Contains(pair[1], ":") {
				return "UNSUPPORTED_SCHEMA"
			}
			name, value := pair[0], pair[1]
			if previous, seen := declarations[name]; seen {
				if previous == value {
					return "DUPLICATE_VALUE"
				}
				return "CONFLICTING_VALUE"
			}
			declarations[name] = value
			if strings.HasPrefix(name, "--") {
				if !customName(name) || !cssAtom(value) {
					return "UNSUPPORTED_SCHEMA"
				}
				if reason := emit(fact{Kind: "css.custom-property.declaration", InputHandle: in.Handle, RelatedHandle: "-", Subject: name, Predicate: "declares", Value: value, InstanceID: in.Path}); reason != "" {
					return reason
				}
				continue
			}
			v, ok := cssAssetURL(value)
			if !ok || !cssProperty(name) {
				return "UNSUPPORTED_SCHEMA"
			}
			if reason := emit(fact{Kind: "css.asset.reference", InputHandle: in.Handle, RelatedHandle: "-", Subject: in.Path, Predicate: "references-asset", Value: v, InstanceID: in.Path}); reason != "" {
				return reason
			}
		}
		s = s[open+2+close:]
	}
	if !rules {
		return "UNSUPPORTED_SCHEMA"
	}
	return ""
}
func cssImport(s string) (string, bool) {
	if len(s) >= 2 && s[0] == '"' && s[len(s)-1] == '"' {
		v := s[1 : len(s)-1]
		return v, reference(v)
	}
	return cssAssetURL(s)
}
func cssAssetURL(s string) (string, bool) {
	if strings.HasPrefix(s, "url(\"") && strings.HasSuffix(s, "\")") {
		v := s[5 : len(s)-2]
		return v, reference(v)
	}
	return "", false
}
func cssSelector(s string) bool { return s == ":root" || cssAtom(s) }
func cssAtom(s string) bool {
	if len(s) == 0 || len(s) > 128 {
		return false
	}
	for _, c := range s {
		if !(c >= 'A' && c <= 'Z' || c >= 'a' && c <= 'z' || c >= '0' && c <= '9' || strings.ContainsRune("._#-", c)) {
			return false
		}
	}
	return true
}
func customName(s string) bool { return strings.HasPrefix(s, "--") && len(s) > 2 && cssAtom(s[2:]) }
func cssProperty(s string) bool {
	return s == "background" || s == "background-image" || s == "content" || s == "src"
}
func reference(s string) bool { return logicalPath(s) && !strings.HasPrefix(s, ".") }
func printableASCII(s string) bool {
	for _, c := range []byte(s) {
		if c < 0x20 || c > 0x7e {
			return false
		}
	}
	return true
}
