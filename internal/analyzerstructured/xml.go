package analyzerstructured

import (
	"bytes"
	"encoding/xml"
	"io"
	"strings"
)

const svgNamespace = "http://www.w3.org/2000/svg"

type xmlProfile uint8

const (
	xmlGeneric xmlProfile = iota
	xmlPlist
	xmlSVG
)

// parseXML accepts one strict XML document. The token walk owns declaration,
// root, namespace, text, and duplicate-attribute boundaries; encoding/xml owns
// XML grammar and matching-tag validation.
func parseXML(in decodedInput, plist bool) string {
	profile := xmlGeneric
	if plist {
		profile = xmlPlist
	}
	return parseXMLDocument(in.bytes, profile)
}

func parsePlist(in decodedInput) string {
	const declaration = "<?xml version=\"1.0\" encoding=\"UTF-8\"?>\n"
	if !bytes.HasPrefix(in.bytes, []byte(declaration)) {
		return "UNSUPPORTED_SCHEMA"
	}
	return parseXMLDocument(in.bytes, xmlPlist)
}

func parseSVG(in decodedInput) string {
	return parseXMLDocument(in.bytes, xmlSVG)
}

func parseXMLDocument(raw []byte, profile xmlProfile) string {
	if len(raw) == 0 || bytes.Contains(raw, []byte("<!")) || bytes.Contains(raw, []byte("&")) {
		return "UNSUPPORTED_SCHEMA"
	}
	decoder := xml.NewDecoder(bytes.NewReader(raw))
	decoder.Strict = true
	declaration, outside, closed := false, false, false
	depth, tokens, roots := 0, 0, 0
	for {
		token, err := decoder.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return "MALFORMED_INPUT"
		}
		tokens++
		if tokens > maxTokenCount {
			return "LIMIT_EXCEEDED"
		}
		switch value := token.(type) {
		case xml.ProcInst:
			if declaration || outside || roots != 0 || value.Target != "xml" || !exactXMLDeclaration(value.Inst) || !bytes.HasPrefix(raw, append([]byte("<?xml "), append(value.Inst, '?', '>')...)) {
				return "UNSUPPORTED_SCHEMA"
			}
			declaration = true
		case xml.Directive, xml.Comment:
			return "UNSUPPORTED_SCHEMA"
		case xml.StartElement:
			if closed {
				return "MALFORMED_INPUT"
			}
			depth++
			if depth > maxDepth {
				return "LIMIT_EXCEEDED"
			}
			if depth == 1 {
				roots++
				if roots != 1 {
					return "MALFORMED_INPUT"
				}
			}
			if reason := validateXMLStart(value, depth, profile); reason != "" {
				return reason
			}
		case xml.EndElement:
			depth--
			if depth < 0 {
				return "MALFORMED_INPUT"
			}
			if depth == 0 {
				closed = true
			}
		case xml.CharData:
			if depth == 0 {
				outside = true
				if !xmlWhitespace(value) {
					return "MALFORMED_INPUT"
				}
				continue
			}
			if len(value) > maxStringBytes {
				return "LIMIT_EXCEEDED"
			}
		}
	}
	if profile == xmlPlist && !declaration {
		return "UNSUPPORTED_SCHEMA"
	}
	if roots != 1 || depth != 0 || !closed {
		return "MALFORMED_INPUT"
	}
	return ""
}

func exactXMLDeclaration(value []byte) bool {
	return string(value) == `version="1.0"` || string(value) == `version="1.0" encoding="UTF-8"` || string(value) == `version="1.0" encoding="utf-8"`
}

func validateXMLStart(element xml.StartElement, depth int, profile xmlProfile) string {
	if !canonicalXMLName(element.Name.Local) {
		return "UNSUPPORTED_SCHEMA"
	}
	seen := make(map[string]struct{}, len(element.Attr))
	for _, attribute := range element.Attr {
		if !canonicalXMLName(attribute.Name.Local) {
			return "UNSUPPORTED_SCHEMA"
		}
		key := attribute.Name.Space + "\x00" + attribute.Name.Local
		if _, duplicate := seen[key]; duplicate {
			return "DUPLICATE_VALUE"
		}
		seen[key] = struct{}{}
		if len(attribute.Value) > maxStringBytes {
			return "LIMIT_EXCEEDED"
		}
	}
	switch profile {
	case xmlSVG:
		return validateSVGElement(element, depth)
	default:
		if element.Name.Space != "" {
			return "UNSUPPORTED_SCHEMA"
		}
		for _, attribute := range element.Attr {
			if attribute.Name.Space != "" || attribute.Name.Local == "xmlns" {
				return "UNSUPPORTED_SCHEMA"
			}
		}
		if profile == xmlPlist && depth == 1 && (element.Name.Local != "plist" || !exactPlistRoot(element)) {
			return "UNSUPPORTED_SCHEMA"
		}
	}
	return ""
}

func validateSVGElement(element xml.StartElement, depth int) string {
	if element.Name.Space != svgNamespace {
		return "UNSUPPORTED_SCHEMA"
	}
	if depth == 1 && element.Name.Local != "svg" {
		return "UNSUPPORTED_SCHEMA"
	}
	if activeSVGElement(element.Name.Local) {
		return "UNSUPPORTED_SCHEMA"
	}
	for _, attribute := range element.Attr {
		if attribute.Name.Space != "" {
			return "UNSUPPORTED_SCHEMA"
		}
		if attribute.Name.Local == "xmlns" && (depth != 1 || attribute.Value != svgNamespace) {
			return "UNSUPPORTED_SCHEMA"
		}
		if activeSVGAttribute(attribute.Name.Local) {
			return "UNSUPPORTED_SCHEMA"
		}
		if containsURLReference(attribute.Value) {
			return "UNSUPPORTED_SCHEMA"
		}
	}
	return ""
}

func canonicalXMLName(value string) bool {
	if len(value) == 0 || len(value) > 128 {
		return false
	}
	for index := range value {
		character := value[index]
		if (character >= 'A' && character <= 'Z') || (character >= 'a' && character <= 'z') || character == '_' {
			continue
		}
		if index != 0 && ((character >= '0' && character <= '9') || character == '-' || character == '.') {
			continue
		}
		return false
	}
	return true
}

func activeSVGElement(name string) bool {
	switch name {
	case "animate", "animateColor", "animateMotion", "animateTransform", "discard", "foreignObject", "script", "set", "style":
		return true
	}
	return false
}

func activeSVGAttribute(name string) bool {
	return name == "href" || name == "style" || strings.HasPrefix(strings.ToLower(name), "on")
}

// containsURLReference reports whether value contains a case-insensitive
// "url(" substring, without allocating (unlike strings.ToLower).
func containsURLReference(value string) bool {
	const needle = "url("
	for index := 0; index+len(needle) <= len(value); index++ {
		if strings.EqualFold(value[index:index+len(needle)], needle) {
			return true
		}
	}
	return false
}

func xmlWhitespace(value []byte) bool {
	return strings.Trim(string(value), " \t\r\n") == ""
}

func exactPlistRoot(element xml.StartElement) bool {
	return len(element.Attr) == 1 && element.Attr[0].Name.Local == "version" && element.Attr[0].Value == "1.0"
}
