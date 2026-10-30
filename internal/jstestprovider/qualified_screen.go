package jstestprovider

import (
	"bytes"
	"encoding/json"
	"reflect"
	"regexp"
	"strconv"
	"strings"

	"github.com/Beamfall/corvint/internal/secretscreen"
)

// qualifiedSecretShaped is the wire refusal code of both qualified encoders.
const qualifiedSecretShaped = "qualified-document-secret-shaped"

// QualifiedSecretError is the qualified-document-secret-shaped refusal. Error returns exactly the
// wire code, so every reason-code consumer is unchanged; Pointer names the member whose text
// matched as an RFC 6901 JSON pointer into the refused document, built only from member names and
// array indices of the encoded schema, so it never echoes the matched text or a dynamic member
// name such as an environment key (PWP-V3-012).
type QualifiedSecretError struct {
	Pointer string
}

func (e *QualifiedSecretError) Error() string { return qualifiedSecretShaped }

// identifierMembers names the members of one encoded document that the provider writes as
// names, not values (PWP-V3-011). keys lists objects whose member names are file paths (their
// values stay screened as written); values lists string members that are a file path or a
// Playwright test title; anchors lists string members that are identifiers only when their text is
// exactly one of anchorTexts, the `file:line` anchors the receipt's own tests carry. In a pattern
// `#` matches one array index and `*` any one member name. Attachment names and paths, failure
// messages, step metadata, project and reporter names, the environment, argv, `use` and URLs are
// not identifiers and stay screened in place.
type identifierMembers struct {
	keys, values, anchors []string
	anchorTexts           map[string]bool
	// schema is the set of member names the encoded Go type declares; only these and array
	// indices are printed in a refusal pointer.
	schema map[string]bool
}

func (m identifierMembers) key(object []string) bool { return matchesPattern(m.keys, object) }

func (m identifierMembers) value(path []string, text string) bool {
	return matchesPattern(m.values, path) || (m.anchorTexts[text] && matchesPattern(m.anchors, path))
}

// qualifiedIdentifiers is the identifier set of an EncodeQualified document for receipt r.
func qualifiedIdentifiers(r Receipt) identifierMembers {
	anchors := map[string]bool{}
	add := func(a *Anchor) {
		if a != nil && a.File != "" {
			anchors[anchorString(*a)] = true
		}
	}
	for _, test := range r.Tests {
		add(test.Anchor)
		for _, attempt := range test.AttemptDetails {
			add(attempt.Anchor)
			add(attempt.FailureLocation)
		}
	}
	return identifierMembers{
		keys: []string{
			"receipt/identity/testFileDigests",
			"receipt/identity/configInputDigests",
			"receipt/external/projectReporters/qualification/configInputs",
		},
		values: []string{
			"receipt/identity/configFile",
			"receipt/tests/#/name",
			"receipt/tests/#/fullName",
			"receipt/tests/#/anchor/file",
			"receipt/tests/#/attemptDetails/#/anchor/file",
			"receipt/tests/#/attemptDetails/#/failureLocation/file",
			"receipt/schedule/starts/#/fullName",
			"receipt/schedule/starts/#/file",
			"receipt/infrastructure/location/file",
			"testProjections/#/name",
		},
		anchors:     []string{"testProjections/#/projection/*/anchors/#", "runProjection/*/anchors/#"},
		anchorTexts: anchors,
	}
}

// keepReportersIdentifiers is the identifier set of an EncodeKeepReportersQualification record.
var keepReportersIdentifiers = identifierMembers{keys: []string{"configInputs"}, schema: schemaNames(reflect.TypeOf(KeepReportersQualification{}))}

// schemaNames collects the JSON member names a Go type declares, through pointers, slices, arrays,
// map values and embedded structs. Map keys and the contents of untyped values are not declared
// names, so a pointer never prints them.
func schemaNames(t reflect.Type) map[string]bool {
	names := map[string]bool{}
	seen := map[reflect.Type]bool{}
	var walk func(reflect.Type)
	walk = func(t reflect.Type) {
		for t.Kind() == reflect.Pointer || t.Kind() == reflect.Slice || t.Kind() == reflect.Array || t.Kind() == reflect.Map {
			t = t.Elem()
		}
		if t.Kind() != reflect.Struct || seen[t] {
			return
		}
		seen[t] = true
		for i := 0; i < t.NumField(); i++ {
			f := t.Field(i)
			if !f.IsExported() && !f.Anonymous {
				continue
			}
			name, _, _ := strings.Cut(f.Tag.Get("json"), ",")
			if name == "-" {
				continue
			}
			if name == "" && f.Anonymous {
				walk(f.Type)
				continue
			}
			if name == "" {
				name = f.Name
			}
			names[name] = true
			walk(f.Type)
		}
	}
	walk(t)
	return names
}

// credentialNames is the writer screen's credential-name vocabulary (secretscreen
// writerAssignmentNames plus the bare `pass` field). Masking it in an identifier disables only the
// name-driven assignment, authorization and credential-flag rules; every value-shape rule (vendor
// tokens, credentialed URLs, private keys, JWTs, bearer credentials) still reads the identifier.
// A name the shared vocabulary gains later is simply not masked here, so the drift refuses more,
// never less. A name followed by an assignment (see assignmentAfter) is never masked, so an
// identifier that carries a credential assignment still refuses.
var credentialNames = regexp.MustCompile(`(?i)api[_-]?key|access[_-]?key|account[_-]?key|authorization|credentials?|passphrase|passwd|password|private[_-]?key|secret|token|pass`)

// screenQualifiedDocument refuses secret-shaped canonical bytes. A document the shared writer
// screen accepts as a whole is accepted unchanged. Otherwise the identifier members are blanked in
// place and the rest of the document is screened again byte for byte, then each identifier is
// screened on its own with its credential names masked (PWP-V3-011). A refusal names the member
// whose text matched (PWP-V3-012).
func screenQualifiedDocument(data []byte, identifiers identifierMembers) error {
	if !secretscreen.MatchString(string(data)) {
		return nil
	}
	members := documentMembers(data, identifiers)
	masked := bytes.Clone(data)
	for _, m := range members {
		if m.identifier && m.end-m.start >= 2 && data[m.start] == '"' && data[m.end-1] == '"' {
			for i := m.start + 1; i < m.end-1; i++ {
				masked[i] = 'x'
			}
		}
	}
	if spans := secretscreen.MatchIndexes(string(masked)); len(spans) != 0 {
		return &QualifiedSecretError{Pointer: pointerAt(members, spans[0][0], identifiers.schema)}
	}
	for _, m := range members {
		if m.identifier && secretscreen.MatchString(maskCredentialNames(m.text)) {
			return &QualifiedSecretError{Pointer: safePointer(m, identifiers.schema)}
		}
	}
	return nil
}

// assignmentAfter matches the text after a credential name that makes it an assignment: an `=`,
// or a `:` whose value is more than a short plain word. A title such as `password: too short is
// rejected` names a concept; `password: hunter2` assigns a value. A plain word of up to seven
// letters after a colon is the documented residual gap (PWP-V3-011).
var (
	equalsAfter = regexp.MustCompile(`^\s*["']?\s*=`)
	colonAfter  = regexp.MustCompile(`^\s*["']?\s*:\s*["']?([^\s"']*)`)
	plainWord   = regexp.MustCompile(`^[A-Za-z][a-z]{0,6}$`)
)

func assignmentAfter(text string) bool {
	if equalsAfter.MatchString(text) {
		return true
	}
	value := colonAfter.FindStringSubmatch(text)
	return value != nil && value[1] != "" && !plainWord.MatchString(value[1])
}

func maskCredentialNames(text string) string {
	masked := []byte(text)
	for _, span := range credentialNames.FindAllStringIndex(text, -1) {
		if assignmentAfter(text[span[1]:]) {
			continue
		}
		for i := span[0]; i < span[1]; i++ {
			masked[i] = 'x'
		}
	}
	return string(masked)
}

// documentMember is one member name or scalar value of the document: its byte span, its path
// (object member names and decimal array indices), and its decoded text when it is a string.
type documentMember struct {
	start, end int
	path       []string
	index      []bool // index[i]: path[i] is an array index, not a member name
	text       string
	identifier bool
}

// documentMembers walks the canonical bytes once. Malformed JSON stops the walk; anything after
// the stop is not an identifier and stays screened as written.
func documentMembers(data []byte, identifiers identifierMembers) []documentMember {
	type frame struct {
		object, awaitingKey bool
		key                 string
		index               int
	}
	var stack []frame
	path := func() ([]string, []bool) {
		p, index := make([]string, len(stack)), make([]bool, len(stack))
		for i, f := range stack {
			if f.object {
				p[i] = f.key
			} else {
				p[i], index[i] = strconv.Itoa(f.index), true
			}
		}
		return p, index
	}
	beginValue := func() {
		if n := len(stack); n > 0 && !stack[n-1].object {
			stack[n-1].index++
		}
	}
	endValue := func() {
		if n := len(stack); n > 0 && stack[n-1].object {
			stack[n-1].awaitingKey = true
		}
	}
	var members []documentMember
	d := json.NewDecoder(bytes.NewReader(data))
	d.UseNumber()
	for {
		start := int(d.InputOffset())
		tok, err := d.Token()
		if err != nil {
			break
		}
		end := int(d.InputOffset())
		for start < end && strings.IndexByte(" \t\r\n,:", data[start]) >= 0 {
			start++
		}
		if n := len(stack); n > 0 && stack[n-1].object && stack[n-1].awaitingKey {
			if name, ok := tok.(string); ok {
				stack[n-1].key, stack[n-1].awaitingKey = name, false
				p, index := path()
				members = append(members, documentMember{start: start, end: end, path: p, index: index, text: name, identifier: identifiers.key(p[:len(p)-1])})
				continue
			}
		}
		switch tok {
		case json.Delim('{'):
			beginValue()
			stack = append(stack, frame{object: true, awaitingKey: true})
			continue
		case json.Delim('['):
			beginValue()
			stack = append(stack, frame{index: -1})
			continue
		case json.Delim('}'), json.Delim(']'):
			if len(stack) > 0 {
				stack = stack[:len(stack)-1]
			}
			endValue()
			continue
		}
		beginValue()
		p, index := path()
		text, isString := tok.(string)
		members = append(members, documentMember{start: start, end: end, path: p, index: index, text: text, identifier: isString && identifiers.value(p, text)})
		endValue()
	}
	return members
}

func matchesPattern(patterns []string, path []string) bool {
	for _, pattern := range patterns {
		parts := strings.Split(pattern, "/")
		if len(parts) != len(path) {
			continue
		}
		matched := true
		for i, part := range parts {
			if part == "#" {
				if _, err := strconv.Atoi(path[i]); err != nil {
					matched = false
				}
			} else if part != "*" && part != path[i] {
				matched = false
			}
		}
		if matched {
			return true
		}
	}
	return false
}

// pointerAt names the member whose span starts at or most recently before offset.
func pointerAt(members []documentMember, offset int, schema map[string]bool) string {
	var member documentMember
	for _, m := range members {
		if m.start > offset {
			break
		}
		member = m
	}
	return safePointer(member, schema)
}

// safePointer prints the member's path up to its first reference token that is neither an array
// index nor a member name the encoded schema declares. A dynamic member name (an environment key,
// a file path, a `use` option) ends the pointer at its parent, so no text the document author or
// the run supplied is ever printed.
func safePointer(m documentMember, schema map[string]bool) string {
	var pointer strings.Builder
	for i, token := range m.path {
		if !m.index[i] && !schema[token] {
			break
		}
		pointer.WriteString("/" + token)
	}
	return pointer.String()
}
