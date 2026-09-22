package analyzerdotnet

import (
	"bytes"
	"encoding/xml"
	"io"
	"strings"
	"unicode/utf8"
)

const (
	msbuildNamespace = "http://schemas.microsoft.com/developer/msbuild/2003"

	maxXMLDepth      = 8
	maxXMLElements   = 8192
	maxXMLAttributes = 8
)

// element is one node of the closed XML subset. text is the element's verbatim
// character data with no trimming, so a scalar whose value is padded with
// whitespace fails its atom check rather than being silently normalized.
type element struct {
	name       string
	attributes []attribute
	text       string
	children   []element
}

type attribute struct {
	name  string
	value string
}

func (e element) attribute(name string) (string, bool) {
	for _, candidate := range e.attributes {
		if candidate.name == name {
			return candidate.value, true
		}
	}
	return "", false
}

func (e element) blank() bool { return strings.TrimLeft(e.text, " \t\n") == "" }

// parseClosedXML decodes one document of the closed XML subset. namespaces is
// the exact set of element namespaces this input family admits; every other
// namespace, and every prefixed name, rejects.
//
// Rejected outright, before any shape check: byte-order marks, carriage
// returns, C0 controls, invalid UTF-8, and the ampersand. Excluding `&` closes
// the entity and character-reference channel completely, so no atom can be
// smuggled past its grammar as `&#x6e;et8.0`.
func parseClosedXML(body []byte, namespaces ...string) (element, string) {
	if len(body) == 0 || !utf8.Valid(body) {
		return element{}, "MALFORMED_INPUT"
	}
	if bytes.HasPrefix(body, []byte("\xef\xbb\xbf")) || bytes.IndexByte(body, '&') >= 0 {
		return element{}, "MALFORMED_INPUT"
	}
	for _, character := range body {
		if character < 0x20 && character != '\n' && character != '\t' {
			return element{}, "MALFORMED_INPUT"
		}
		if character == 0x7f {
			return element{}, "MALFORMED_INPUT"
		}
	}
	decoder := xml.NewDecoder(bytes.NewReader(body))
	reader := &xmlReader{decoder: decoder, namespaces: namespaces}
	start, why := reader.rootStart()
	if why != "" {
		return element{}, why
	}
	root, why := reader.element(start, 1)
	if why != "" {
		return element{}, why
	}
	if why := reader.trailing(); why != "" {
		return element{}, why
	}
	return root, ""
}

type xmlReader struct {
	decoder    *xml.Decoder
	namespaces []string
	elements   int
}

// rootStart consumes leading whitespace and requires the first structural token
// to be a start element. A processing instruction, comment, or doctype -- the
// `<?xml ... ?>` prologue included -- is not part of this subset.
func (r *xmlReader) rootStart() (xml.StartElement, string) {
	for {
		token, err := r.decoder.Token()
		if err != nil {
			return xml.StartElement{}, "MALFORMED_INPUT"
		}
		switch typed := token.(type) {
		case xml.CharData:
			if strings.TrimLeft(string(typed), " \t\n") != "" {
				return xml.StartElement{}, "MALFORMED_INPUT"
			}
		case xml.StartElement:
			return typed, ""
		default:
			return xml.StartElement{}, "MALFORMED_INPUT"
		}
	}
}

// trailing requires end-of-document after the single root element.
func (r *xmlReader) trailing() string {
	for {
		token, err := r.decoder.Token()
		if err == io.EOF {
			return ""
		}
		if err != nil {
			return "MALFORMED_INPUT"
		}
		data, ok := token.(xml.CharData)
		if !ok || strings.TrimLeft(string(data), " \t\n") != "" {
			return "MALFORMED_INPUT"
		}
	}
}

func (r *xmlReader) element(start xml.StartElement, depth int) (element, string) {
	if depth > maxXMLDepth {
		return element{}, "LIMIT_EXCEEDED"
	}
	r.elements++
	if r.elements > maxXMLElements {
		return element{}, "LIMIT_EXCEEDED"
	}
	if !r.knownNamespace(start.Name.Space) {
		return element{}, "UNSUPPORTED_SCHEMA"
	}
	current := element{name: start.Name.Local}
	attributes, why := r.attributes(start)
	if why != "" {
		return element{}, why
	}
	current.attributes = attributes
	var text strings.Builder
	for {
		token, err := r.decoder.Token()
		if err != nil {
			return element{}, "MALFORMED_INPUT"
		}
		switch typed := token.(type) {
		case xml.CharData:
			text.Write(typed)
		case xml.StartElement:
			child, why := r.element(typed, depth+1)
			if why != "" {
				return element{}, why
			}
			current.children = append(current.children, child)
		case xml.EndElement:
			current.text = text.String()
			// Mixed content is not in this subset: an element either holds
			// character data or holds child elements, never both.
			if len(current.children) != 0 && !current.blank() {
				return element{}, "MALFORMED_INPUT"
			}
			return current, ""
		default:
			return element{}, "MALFORMED_INPUT"
		}
	}
}

// attributes validates one element's attribute list. A default xmlns
// declaration is a namespace binding rather than an attribute and is consumed
// here; a prefixed declaration or a prefixed attribute name rejects.
func (r *xmlReader) attributes(start xml.StartElement) ([]attribute, string) {
	if len(start.Attr) > maxXMLAttributes {
		return nil, "LIMIT_EXCEEDED"
	}
	collected := make([]attribute, 0, len(start.Attr))
	for _, raw := range start.Attr {
		if raw.Name.Space == "xmlns" {
			return nil, "UNSUPPORTED_SCHEMA"
		}
		if raw.Name.Space == "" && raw.Name.Local == "xmlns" {
			if !r.knownNamespace(raw.Value) {
				return nil, "UNSUPPORTED_SCHEMA"
			}
			continue
		}
		if raw.Name.Space != "" {
			return nil, "UNSUPPORTED_SCHEMA"
		}
		for _, prior := range collected {
			if prior.name == raw.Name.Local {
				return nil, "DUPLICATE_VALUE"
			}
		}
		collected = append(collected, attribute{name: raw.Name.Local, value: raw.Value})
	}
	return collected, ""
}

func (r *xmlReader) knownNamespace(value string) bool {
	for _, permitted := range r.namespaces {
		if value == permitted {
			return true
		}
	}
	return false
}

// requireNoAttributes and requireEmpty express the two shape rules the closed
// schemas apply over and over, one condition per guard.
func requireNoAttributes(node element) string {
	if len(node.attributes) != 0 {
		return "UNKNOWN_FIELD"
	}
	return ""
}

// requirePlainGroup is the shape MSBuild's two container elements share: no
// attributes -- which is what makes every Condition= reject -- and no text.
func requirePlainGroup(node element) string {
	if why := requireNoAttributes(node); why != "" {
		return why
	}
	if !node.blank() {
		return "MALFORMED_INPUT"
	}
	return ""
}

func requireEmpty(node element) string {
	if len(node.children) != 0 {
		return "UNSUPPORTED_SCHEMA"
	}
	if !node.blank() {
		return "MALFORMED_INPUT"
	}
	return ""
}
