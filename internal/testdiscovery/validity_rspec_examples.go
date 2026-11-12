package testdiscovery

import (
	"regexp"
	"slices"
	"strings"
)

var (
	rspecDeclarator = regexp.MustCompile(`^(?:RSpec\.)?(\w+)`)
	// RSpecMetadataSymbol matches `:skip`, `:pending` or `:focus` metadata after the description.
	RSpecMetadataSymbol = regexp.MustCompile(`[,(\s]:(skip|pending|focus)\b`)
	// RSpecMetadataKey matches `skip: VALUE`, `pending: VALUE` or `focus: VALUE`; group 2 is VALUE.
	RSpecMetadataKey = regexp.MustCompile(`[,(\s](skip|pending|focus):\s*(\w*)`)
	// RSpecSkipStatement is a skip or pending call statement in an example body.
	RSpecSkipStatement = regexp.MustCompile(`^(?:skip|pending)\b`)
	RSpecConditional   = regexp.MustCompile(`\s(?:if|unless)\s`)
	RSpecBlockStart    = regexp.MustCompile(`\{|\bdo\b`)
	// RSpecExpectation counts one RSpec expectation: expect(...), expect { }, expect do,
	// is_expected, should, should_not and expect_any_instance_of.
	RSpecExpectation = regexp.MustCompile(`\bexpect\s*(?:\(|\{|do\b)|\bis_expected\b|\bshould(?:_not)?\b|\bexpect_any_instance_of\b`)
	// RSpecDescribedClassStub matches a message stub or spy on described_class.
	RSpecDescribedClassStub = regexp.MustCompile(`\b(?:allow|expect)(?:_any_instance_of)?\s*\(\s*described_class\s*\)\s*\.\s*(?:to|not_to|to_not)\b\s*\(?\s*(?:receive\w*|have_received)\b`)
)

// RSpecGroupTitle reads an example group's description from its declaring line: one string
// literal or one constant that ends the argument, followed by metadata, the block or the end of
// the call. A computed description (`"group" + suffix`, `User.name`) does not match.
var RSpecGroupTitle = regexp.MustCompile(`^(?:RSpec\.)?(?:[xf]?describe|[xf]?context|feature)\s*\(?\s*(?:'([^'\\]*)'|"([^"\\]*)"|((?:::)?[A-Z]\w*(?:::[A-Z]\w*)*))\s*(?:,\s*(\S)?|\)|do\b|\{|$)`)

// rspecLiteralTitle reads an example's description from its declaring line only when one string
// literal is the whole argument; a computed description (`"works" + suffix`) does not match.
var rspecLiteralTitle = regexp.MustCompile(`^\w+\s*\(?\s*(?:'([^'\\]*)'|"([^"\\]*)")\s*(?:,|\)|do\b|\{|$)`)

// TitlePath returns example e's literal title path, outermost group first and e's own title last
// (LPCV-V0-147), or nil when a title on it is not one literal string or constant: an interpolated
// or missing title, a constant joined with a second description, a shared group or inclusion, or
// a declaration read from another file.
func (r *RSpecReader) TitlePath(e RSpecExample) []string {
	literal := func(text string) bool { return text != "" && !strings.Contains(text, "#{") }
	if e.Path != r.Path || !literal(e.Title) || !rspecLiteralTitle.MatchString(r.Lines[e.Index].Text) {
		return nil
	}
	path := []string{e.Title}
	for g := e.Group; g > 0; g = r.Groups[g].Parent {
		m := RSpecGroupTitle.FindStringSubmatch(r.Groups[g].Head)
		if m == nil || r.Groups[g].Path != r.Path {
			return nil
		}
		title := m[1] + m[2] + m[3]
		if !literal(title) || m[3] != "" && (m[4] == "'" || m[4] == `"`) {
			return nil
		}
		path = append(path, title)
	}
	slices.Reverse(path)
	return path
}

// SharedDefinitionWords are the declarators of a shared group definition.
var SharedDefinitionWords = map[string]bool{"shared_examples": true, "shared_examples_for": true, "shared_context": true}

// Declarator returns a group or example line's first word, after any `RSpec.` receiver.
func Declarator(head string) string {
	if m := rspecDeclarator.FindStringSubmatch(head); m != nil {
		return m[1]
	}
	return ""
}
