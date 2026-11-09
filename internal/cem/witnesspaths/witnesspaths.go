// Package witnesspaths decodes the declared witness path policy
// .corvint/witness-paths.json. Package datareaders finds a data path's readers.
// corvint witness (AGW-V0-041..045) and the cem-path-policy/1 profile of cem
// prepare and cem status (CAX-V0-060..065) share it, so both read one policy
// the same way. It runs no Git: each caller reads the base-tree blob through
// its own bounded runner and hands the bytes here. It lives under internal/cem
// and imports only the standard library, so the CEM seams keep their
// stdlib-and-Git dependency boundary (docs/specs/native-cem-adapter.md).
package witnesspaths

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"path"
	"slices"
	"strings"
)

// PolicyPath is where the declared witness path policy is committed. It is
// read from the range's base tree only, so a range cannot exempt its own
// paths (AGW-V0-041).
const PolicyPath = ".corvint/witness-paths.json"

// Profile is the closed profile of the policy document.
const Profile = "corvint-witness-paths/0"

// MaxBytes bounds the policy blob a caller reads.
const MaxBytes = 65536

const (
	maxRules       = 256
	maxRuleReaders = 16
)

// Path classes a policy rule may declare (AGW-V0-041).
const (
	ClassInert = "inert"
	ClassData  = "data"
)

// Policy is a report's account of the base tree's policy (AGW-V0-045).
type Policy struct {
	Path   string `json:"path"`
	Status string `json:"status"`
	Detail string `json:"detail"`
	Rules  int    `json:"rules"`
}

// Declaration is the policy rule that classified one path, with the file and
// line that declare it (AGW-V0-042).
type Declaration struct {
	File    string   `json:"file"`
	Line    int      `json:"line"`
	Glob    string   `json:"glob"`
	Class   string   `json:"class"`
	Readers []Reader `json:"readers,omitempty"`
	// rule is the 1-based position of the declaring rule, 0 when unknown:
	// several rules may start on one line.
	rule int
}

// Reader is one reader of a data path and where it came from: declared in the
// rule, found by its quoted path, or declared but untracked (AGW-V0-044).
type Reader struct {
	Path   string `json:"path"`
	Origin string `json:"origin"`
	// Reason is why an unusable reader does not count: the index's exclusion
	// or extraction refusal, or its out-of-profile file mode (AGW-V0-044).
	Reason string `json:"reason,omitempty"`
}

type rule struct {
	Glob    string   `json:"glob"`
	Class   string   `json:"class"`
	Readers []string `json:"readers,omitempty"`
	line    int
}

type document struct {
	Profile string `json:"profile"`
	Rules   []rule `json:"rules"`
}

// Rules is a decoded policy. Its zero value declares nothing.
type Rules struct{ rules []rule }

// Invalid is the account of a defective policy: it names the defect and
// contributes no rule (AGW-V0-041).
func Invalid(detail string) *Policy {
	return &Policy{Path: PolicyPath, Status: "INVALID", Detail: detail}
}

// FromBlob decodes the base tree's policy blob into its account and rules. A
// defective blob is INVALID with no rule.
func FromBlob(raw []byte) (*Policy, Rules) {
	rules, err := decode(raw)
	if err != nil {
		return Invalid(err.Error()), Rules{}
	}
	return &Policy{
		Path: PolicyPath, Status: "BOUND", Rules: len(rules),
		Detail: "read from the base tree; a policy this range adds or edits applies only to a later range",
	}, Rules{rules: rules}
}

// Len is the number of rules.
func (r Rules) Len() int { return len(r.rules) }

// Rule returns the declaration of the first rule in file order whose glob
// matches name, or nil, without judging the path itself.
func (r Rules) Rule(name string) *Declaration {
	for position, candidate := range r.rules {
		if match(candidate.Glob, name) {
			return &Declaration{File: PolicyPath, Line: candidate.line, Glob: candidate.Glob, Class: candidate.Class, rule: position + 1}
		}
	}
	return nil
}

// Declare is Rule for a changed regular file, except that it never
// classifies the policy file itself or a path an extractor reads, so a code
// path is never declared inert or data (AGW-V0-042, CAX-V0-062).
func (r Rules) Declare(name string) *Declaration {
	if name == PolicyPath {
		return nil
	}
	if tier, _ := LanguageCoverage(name); tier != "NONE" {
		return nil
	}
	return r.Rule(name)
}

// ReaderNames is the readers list of the rule that declaration, from Rule or
// Declare on the same rules, names (AGW-V0-044), nil for none.
func (r Rules) ReaderNames(declaration *Declaration) []string {
	if declaration.rule < 1 || declaration.rule > len(r.rules) {
		return nil
	}
	return r.rules[declaration.rule-1].Readers
}

// LanguageCoverage names how deeply Corvint can analyse a regular file at
// name by its suffix alone: the tier and its reason, NONE with
// LANGUAGE_COVERAGE_NONE when no extractor reads it. Language coverage is
// uneven, and a weakly covered language widens NOT_RUN rather than shrinking
// it.
func LanguageCoverage(name string) (string, string) {
	switch strings.ToLower(path.Ext(name)) {
	case ".go":
		if path.Dir(name) == "." {
			return "DEEP", "ROOT_PACKAGE_OUT_OF_PROFILE"
		}
		return "DEEP", ""
	case ".py":
		return "SHALLOW", "LANGUAGE_COVERAGE_SHALLOW_PYTHON"
	// The lexically-extracted languages report GENERIC rather than SHALLOW on
	// purpose. SHALLOW is Python's tier, and Python is analysed by a validated
	// grammar that yields both definitions and imports. These languages are
	// read by a line scanner that yields definitions only: no import graph, no
	// syntax validation, and a declaration whose keyword the scanner does not
	// know is simply missed. That is more than the GENERIC languages get and
	// less than Python gets, and given the choice between the two existing
	// tiers the honest direction to round is down. The reason code carries the
	// detail the tier cannot.
	case ".rs":
		return "GENERIC", "LANGUAGE_COVERAGE_LEXICAL_SYMBOLS_RUST"
	case ".cs":
		return "GENERIC", "LANGUAGE_COVERAGE_LEXICAL_SYMBOLS_CSHARP"
	case ".swift":
		return "GENERIC", "LANGUAGE_COVERAGE_LEXICAL_SYMBOLS_SWIFT"
	case ".kt", ".kts":
		return "GENERIC", "LANGUAGE_COVERAGE_LEXICAL_SYMBOLS_KOTLIN"
	case ".rb":
		return "GENERIC", "LANGUAGE_COVERAGE_LEXICAL_SYMBOLS_RUBY"
	// Admitted as searchable text with no symbol extraction at all. `.m` is
	// shared by Objective-C, MATLAB and Mathematica, so it is deliberately left
	// text-only rather than analysed as whichever language was guessed.
	case ".sql", ".m":
		return "GENERIC", "LANGUAGE_COVERAGE_TEXT_ONLY"
	case ".ts", ".tsx", ".js", ".jsx", ".mjs", ".cjs", ".sh":
		return "GENERIC", "LANGUAGE_COVERAGE_GENERIC_EXTRACTOR"
	default:
		return "NONE", "LANGUAGE_COVERAGE_NONE"
	}
}

// decode strictly decodes the policy and records the 1-based line
// where each rule object starts.
func decode(raw []byte) ([]rule, error) {
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	var document document
	if err := decoder.Decode(&document); err != nil {
		return nil, fmt.Errorf("the policy is not a %s document: %v", Profile, err)
	}
	if _, err := decoder.Token(); err != io.EOF {
		return nil, fmt.Errorf("the policy holds data after its object")
	}
	if document.Profile != Profile {
		return nil, fmt.Errorf("the policy profile is %q, not %s", document.Profile, Profile)
	}
	if document.Rules == nil {
		return nil, fmt.Errorf("the policy has no rules array")
	}
	if len(document.Rules) > maxRules {
		return nil, fmt.Errorf("the policy has %d rules, over the %d bound", len(document.Rules), maxRules)
	}
	lines, err := ruleLines(raw)
	if err != nil {
		return nil, fmt.Errorf("the policy is not strict %s: %v", Profile, err)
	}
	if len(lines) != len(document.Rules) {
		return nil, fmt.Errorf("the policy rules cannot be located by line")
	}
	for position := range document.Rules {
		rule := &document.Rules[position]
		rule.line = lines[position]
		if err := checkRule(*rule); err != nil {
			return nil, fmt.Errorf("rule at line %d: %v", rule.line, err)
		}
	}
	return document.Rules, nil
}

func checkRule(rule rule) error {
	if !validGlob(rule.Glob) {
		return fmt.Errorf("glob %q is not a clean relative slash-separated pattern", rule.Glob)
	}
	switch rule.Class {
	case ClassInert:
		if rule.Readers != nil {
			return fmt.Errorf("an inert rule declares no readers")
		}
	case ClassData:
		if len(rule.Readers) > maxRuleReaders {
			return fmt.Errorf("%d readers, over the %d bound", len(rule.Readers), maxRuleReaders)
		}
		for _, reader := range rule.Readers {
			if !validRelative(reader) || strings.ContainsAny(reader, "*?[") {
				return fmt.Errorf("reader %q is not a clean repository-relative path", reader)
			}
		}
	default:
		return fmt.Errorf("class %q is neither %s nor %s", rule.Class, ClassInert, ClassData)
	}
	return nil
}

func validRelative(name string) bool {
	if name == "" || strings.HasPrefix(name, "/") || strings.ContainsAny(name, "\\\x00\n") || path.Clean(name) != name {
		return false
	}
	for _, segment := range strings.Split(name, "/") {
		if segment == "." || segment == ".." {
			return false
		}
	}
	return true
}

func validGlob(glob string) bool {
	if !validRelative(glob) {
		return false
	}
	for _, segment := range strings.Split(glob, "/") {
		if segment == "**" {
			continue
		}
		if _, err := path.Match(segment, ""); err != nil {
			return false
		}
	}
	return true
}

// exactKeys refuses an object member that is not spelled exactly as one of
// allowed, or that repeats, which encoding/json would otherwise accept by
// case-folding or by keeping the last value (AGW-V0-041).
func exactKeys(raw []byte, allowed ...string) error {
	decoder := json.NewDecoder(bytes.NewReader(raw))
	if token, err := decoder.Token(); err != nil || token != json.Delim('{') {
		return fmt.Errorf("not an object")
	}
	seen := map[string]bool{}
	for decoder.More() {
		token, err := decoder.Token()
		if err != nil {
			return err
		}
		key, _ := token.(string)
		if seen[key] || !slices.Contains(allowed, key) {
			return fmt.Errorf("member %q is repeated or not one of %s", key, strings.Join(allowed, ", "))
		}
		seen[key] = true
		var skip json.RawMessage
		if err := decoder.Decode(&skip); err != nil {
			return err
		}
	}
	return nil
}

// ruleLines walks the document's tokens to the rules array and returns the
// line where each element starts, refusing a member not spelled exactly.
func ruleLines(raw []byte) ([]int, error) {
	if err := exactKeys(raw, "profile", "rules"); err != nil {
		return nil, err
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	if token, err := decoder.Token(); err != nil || token != json.Delim('{') {
		return nil, fmt.Errorf("not an object")
	}
	for decoder.More() {
		key, err := decoder.Token()
		if err != nil {
			return nil, err
		}
		if key != "rules" {
			var skip json.RawMessage
			if err := decoder.Decode(&skip); err != nil {
				return nil, err
			}
			continue
		}
		if token, err := decoder.Token(); err != nil || token != json.Delim('[') {
			return nil, fmt.Errorf("rules is not an array")
		}
		lines := []int{}
		for decoder.More() {
			offset := int(decoder.InputOffset())
			for offset < len(raw) && strings.IndexByte(" \t\r\n,", raw[offset]) >= 0 {
				offset++
			}
			lines = append(lines, 1+bytes.Count(raw[:offset], []byte("\n")))
			var rule json.RawMessage
			if err := decoder.Decode(&rule); err != nil {
				return nil, err
			}
			if err := exactKeys(rule, "glob", "class", "readers"); err != nil {
				return nil, fmt.Errorf("rule at line %d: %v", lines[len(lines)-1], err)
			}
		}
		return lines, nil
	}
	return nil, fmt.Errorf("no rules")
}

// match reports whether name matches glob segment by segment, where a "**"
// segment matches zero or more whole segments and any other segment is a
// path.Match pattern. It is a local copy of sharedpath.Match, kept here so
// this package stays stdlib-only; TestMatchAgreesWithSharedpath pins parity.
func match(glob, name string) bool {
	return matchSegments(strings.Split(glob, "/"), strings.Split(name, "/"))
}

func matchSegments(glob, name []string) bool {
	for len(glob) > 0 {
		if glob[0] == "**" {
			for skip := 0; skip <= len(name); skip++ {
				if matchSegments(glob[1:], name[skip:]) {
					return true
				}
			}
			return false
		}
		if len(name) == 0 {
			return false
		}
		if ok, _ := path.Match(glob[0], name[0]); !ok {
			return false
		}
		glob, name = glob[1:], name[1:]
	}
	return len(name) == 0
}
