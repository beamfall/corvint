package testsource

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"unicode"

	"github.com/Beamfall/corvint/internal/appmap"
	"github.com/Beamfall/corvint/internal/testvalidity"
)

// Cross-file readings of the advisory source rules (TSA-V0-300..312): a stub, cleanup registry or
// fixture declared in another file is not followed; the rule withholds its reading as a
// RuleAbstention naming the import instead (TSA-V0-300..304, TSA-V0-307). A fixture or hook the
// file declares that seeds data suppresses VACUOUS-EMPTY-EQUALITY (TSA-V0-305, TSA-V0-306). A JSON
// behaviour contract carries the EITHER-OR-OUTCOME justification as a sibling member
// (TSA-V0-308..310). Following the import is NOT_PRODUCED (TSA-V0-312).

// xfImport is one module a file imports, re-exports or requires, with the line naming it and, for
// an `import ... from` form, the names it binds.
type xfImport struct {
	spec  string
	line  int
	names []string
}

// xfImports returns every module specifier the file imports, re-exports or requires, in source
// order.
func (s *source) xfImports() []xfImport {
	out := []xfImport{}
	literal := func(k int) bool {
		return k < len(s.tokens) && s.tokens[k].Kind == appmap.JSString && s.tokens[k].Literal
	}
	for k := range s.tokens {
		switch {
		case s.ident(k, "from") && literal(k+1):
			out = append(out, xfImport{s.tokens[k+1].Text, s.tokens[k+1].Line, s.xfBound(k)})
		case s.ident(k, "import") && !s.memberAccess(k) && literal(k+1):
			out = append(out, xfImport{s.tokens[k+1].Text, s.tokens[k+1].Line, nil})
		case (s.ident(k, "require") || s.ident(k, "import")) && !s.memberAccess(k) && s.punct(k+1, "(") && literal(k+2) && (s.punct(k+3, ")") || s.punct(k+3, ",")):
			out = append(out, xfImport{s.tokens[k+2].Text, s.tokens[k+2].Line, nil})
		}
	}
	return out
}

// xfBound returns the names an `import ... from` or `export ... from` clause ending at the `from`
// token k names, imported and local alike, or nil when no such clause starts the statement.
func (s *source) xfBound(k int) []string {
	names := []string{}
	for j := k - 1; j >= 0 && !s.punct(j, ";"); j-- {
		if s.ident(j, "import") || s.ident(j, "export") {
			return names
		}
		if s.ident(j, "") {
			names = append(names, s.tokens[j].Text)
		}
	}
	return nil
}

// xfSibling reports a relative (sibling) module specifier: `.`, `..`, or one starting `./` or
// `../`.
func xfSibling(spec string) bool {
	return spec == "." || spec == ".." || strings.HasPrefix(spec, "./") || strings.HasPrefix(spec, "../")
}

// xfSiblingImport returns the file's first sibling module import.
func (s *source) xfSiblingImport() (xfImport, bool) {
	for _, im := range s.xfImports() {
		if xfSibling(im.spec) {
			return im, true
		}
	}
	return xfImport{}, false
}

// xfStubWords mark a module that may declare a stub table: a sibling import whose specifier or a
// name it binds contains one of them, ignoring case (TSA-V0-301).
var xfStubWords = []string{"stub", "mock", "msw", "handler", "route", "fake", "intercept"}

// xfStubImport returns the file's first sibling import that may declare a stub table (TSA-V0-301,
// TSA-V0-302).
func (s *source) xfStubImport() (xfImport, bool) {
	for _, im := range s.xfImports() {
		if !xfSibling(im.spec) {
			continue
		}
		for _, text := range append([]string{im.spec}, im.names...) {
			lower := strings.ToLower(text)
			for _, w := range xfStubWords {
				if strings.Contains(lower, w) {
					return im, true
				}
			}
		}
	}
	return xfImport{}, false
}

// xfFixtureImport returns the import most likely to declare a fixture the file does not: its first
// sibling module import, else its first import of a module other than `@playwright/test`
// (TSA-V0-303, TSA-V0-307).
func (s *source) xfFixtureImport() (xfImport, bool) {
	if im, ok := s.xfSiblingImport(); ok {
		return im, true
	}
	for _, im := range s.xfImports() {
		if im.spec != "@playwright/test" {
			return im, true
		}
	}
	return xfImport{}, false
}

// xfWhere describes an import for a message and returns its `path:line`, or the location and
// "another file" when the file names none.
func (s *source) xfWhere(im xfImport, ok bool, location string) (string, string) {
	if !ok {
		return "another file", location
	}
	cause := fmt.Sprintf("%s:%d", s.path, im.line)
	return fmt.Sprintf("%q (imported at %s)", im.spec, cause), cause
}

// xfAbstain records a withheld advisory reading (TSA-V0-300).
func (s *source) xfAbstain(rule, reason, test, location, cause, message string) {
	s.abstentions = append(s.abstentions, testvalidity.RuleAbstention{
		Rule: rule, Reason: reason, Test: test, Location: location, Cause: cause, Message: message,
	})
}

// xfSeedPrefixes name a seeding call (TSA-V0-305): a callee whose name is one of these words, or
// starts with one followed by an upper-case letter, digit or `_` (`seedTodos`, `createUser`). A
// bare `add(` (a set or a cleanup registry) is not one; `addTodo(` is.
var xfSeedPrefixes = []string{"seed", "create", "insert", "populate", "add"}

// xfSeedExact are request methods that write data when called on a member (`request.post(...)`).
var xfSeedExact = map[string]bool{"post": true, "put": true}

// xfNotSeeding are Playwright page and context methods that match a seeding prefix but write no
// data.
var xfNotSeeding = map[string]bool{"addInitScript": true, "addCookies": true, "addScriptTag": true, "addStyleTag": true, "addLocatorHandler": true}

// xfSeedingCall reports a seeding call inside r: a call `name(` or `x.name(` whose name is a
// seeding name (TSA-V0-305).
func (s *source) xfSeedingCall(r [2]int) bool {
	for j := r[0]; j < r[1]; j++ {
		if !s.ident(j, "") || !s.punct(j+1, "(") || s.ident(j-1, "function") || s.match[j+1] < 0 || s.punct(s.match[j+1]+1, "{") {
			continue // not a call: a function or method declaration `name(...) {`
		}
		name := s.tokens[j].Text
		if xfNotSeeding[name] {
			continue
		}
		if xfSeedExact[name] && s.memberAccess(j) {
			return true
		}
		lower := strings.ToLower(name)
		for _, p := range xfSeedPrefixes {
			if !strings.HasPrefix(lower, p) {
				continue
			}
			rest := name[len(p):]
			if rest == "" && p == "add" {
				continue
			}
			if rest == "" || rest[0] == '_' || unicode.IsUpper(rune(rest[0])) || unicode.IsDigit(rune(rest[0])) {
				return true
			}
		}
	}
	return false
}

// xfSeeded reports a test whose setup seeds data (TSA-V0-305, TSA-V0-306): a fixture it requests
// that the file declares and whose function, or that of a declared fixture it requests in turn,
// makes a seeding call; or a `beforeEach` or `beforeAll` hook in the test's scope whose callback
// makes one.
func (s *source) xfSeeded(d decl, fixtures map[string]chFixture, bindings map[string]string) bool {
	seen := map[string]bool{}
	var seeds func(name string) bool
	seeds = func(name string) bool {
		f, ok := fixtures[name]
		if !ok || f.line == 0 || seen[name] {
			return false
		}
		seen[name] = true
		params, body, ok := s.chFunction(f.fn)
		if !ok {
			return false
		}
		if s.xfSeedingCall(body) {
			return true
		}
		for _, dep := range s.chBindings(params) {
			if seeds(dep) {
				return true
			}
		}
		return false
	}
	for _, name := range bindings {
		if seeds(name) {
			return true
		}
	}
	for _, hook := range []string{"beforeEach", "beforeAll"} {
		for _, at := range s.chHooks(hook) {
			if s.xfInScope(at, d) && s.xfSeedingCall(s.chHookCallback(at)) {
				return true
			}
		}
	}
	return false
}

// xfInScope reports a hook call at token i that applies to test d: every describe whose callback
// holds the hook also holds the test.
func (s *source) xfInScope(i int, d decl) bool {
	for _, g := range s.decls {
		if g.kind != "describe" || i < g.open || i > g.close {
			continue
		}
		if d.at < g.open || d.at > g.close {
			return false
		}
	}
	return true
}

// xfPlaywrightOptions are Playwright's option fixtures: they configure the browser or context and
// seed no data, so requesting one is not a cross-file fixture (TSA-V0-307).
var xfPlaywrightOptions = map[string]bool{
	"acceptDownloads": true, "actionTimeout": true, "baseURL": true, "bypassCSP": true, "channel": true,
	"clientCertificates": true, "colorScheme": true, "connectOptions": true, "contextOptions": true,
	"defaultBrowserType": true, "deviceScaleFactor": true, "extraHTTPHeaders": true, "geolocation": true,
	"hasTouch": true, "headless": true, "httpCredentials": true, "ignoreHTTPSErrors": true, "isMobile": true,
	"javaScriptEnabled": true, "launchOptions": true, "locale": true, "navigationTimeout": true,
	"offline": true, "permissions": true, "proxy": true, "screenshot": true, "serviceWorkers": true,
	"storageState": true, "testIdAttribute": true, "timezoneId": true, "trace": true, "userAgent": true,
	"video": true, "viewport": true,
}

// xfUndeclaredFixture returns the first fixture, in parameter order, that the test requests and
// the file neither declares nor knows as a Playwright built-in or option (TSA-V0-307).
func (s *source) xfUndeclaredFixture(params [2]int, fixtures map[string]chFixture) string {
	if !s.punct(params[0], "{") || s.match[params[0]] < 0 {
		return ""
	}
	for _, e := range s.splitArgs(params[0], s.match[params[0]]) {
		if !s.ident(e[0], "") {
			continue
		}
		name := s.tokens[e[0]].Text
		if _, ok := fixtures[name]; !ok && !xfPlaywrightOptions[name] {
			return name
		}
	}
	return ""
}

// eoJSONAllowMember is the key of the JSON annotation member (TSA-V0-308).
const eoJSONAllowMember = testvalidity.ValidityAllowMarker

// eoJSONAllowed returns the lines of a JSON behaviour contract exempted by the sibling-member
// annotation (TSA-V0-308, TSA-V0-309): in the object holding the flagged member, a member
// `"corvint-validity-allow": {"<key>": "EITHER-OR-OUTCOME: <reason>"}` exempts every line holding
// a `<key>` member of that object. A value without a reason, naming another rule, or not a string,
// and a key it does not name, exempt nothing. A file that is not one valid JSON value exempts
// nothing. Each exempted line maps to the annotation's reason (TSA-V0-870).
func eoJSONAllowed(text string) map[int]string {
	type frame struct {
		object, wantKey bool
		key             string
		keyLine         int
		members         map[string][]int
		allow           map[string]string
		allowFor        *frame // set on the annotation object: the object it annotates
	}
	exempt := map[int]string{}
	if !json.Valid([]byte(text)) {
		return exempt
	}
	dec := json.NewDecoder(strings.NewReader(text))
	dec.UseNumber()
	lineAt := func(offset int64) int { return strings.Count(text[:offset], "\n") + 1 }
	var stack []*frame
	value := func() { // a value ended in the top frame
		if n := len(stack); n > 0 && stack[n-1].object {
			stack[n-1].wantKey = true
		}
	}
	for {
		tok, err := dec.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return map[int]string{}
		}
		var top *frame
		if len(stack) > 0 {
			top = stack[len(stack)-1]
		}
		switch v := tok.(type) {
		case json.Delim:
			switch v {
			case '{', '[':
				f := &frame{object: v == '{', wantKey: v == '{', members: map[string][]int{}, allow: map[string]string{}}
				if v == '{' && top != nil && top.object && top.key == eoJSONAllowMember {
					f.allowFor = top
				}
				stack = append(stack, f)
			default:
				for key, lines := range top.members {
					if reason, ok := top.allow[key]; ok {
						for _, line := range lines {
							exempt[line] = reason
						}
					}
				}
				stack = stack[:len(stack)-1]
				value()
			}
		case string:
			if top != nil && top.object && top.wantKey {
				top.key, top.keyLine, top.wantKey = v, lineAt(dec.InputOffset()), false
				top.members[v] = append(top.members[v], top.keyLine)
				continue
			}
			if reason, ok := eoJSONAllowValue(v); ok && top != nil && top.allowFor != nil {
				top.allowFor.allow[top.key] = reason
			}
			value()
		default:
			value()
		}
	}
	return exempt
}

// eoJSONAllowValue returns the reason of an annotation value `EITHER-OR-OUTCOME: <reason>`, and
// whether it carries one.
func eoJSONAllowValue(v string) (string, bool) {
	rule, reason, ok := strings.Cut(v, ":")
	reason = strings.TrimSpace(reason)
	if !ok || strings.TrimSpace(rule) != testvalidity.RuleEitherOrOutcome || reason == "" {
		return "", false
	}
	return reason, true
}
