package testsource

import (
	"fmt"
	"regexp"
	"sort"
	"strings"

	"github.com/Beamfall/corvint/internal/testvalidity"
)

// Denial examples without a no-effect assertion (TSA-V0-564, TSA-V0-565): an example that asserts
// only a 401/403 refusal status of a mutating request proves the caller saw a refusal, not that the
// refused operation had no effect. The reading is line-based, advisory only, and abstains with a
// named rule whenever it cannot name the request verb or a matcher it depends on.

var (
	// rspecDenialStatus is a built-in refusal status matcher over stripped text.
	rspecDenialStatus = regexp.MustCompile(`\bhave_http_status\s*\(?\s*(?::forbidden|:unauthorized|401|403)\b|\bbe_(?:forbidden|unauthorized)\b|\.\s*(?:status|code)\s*\)\s*\.\s*to\s*\(?\s*(?:eq|eql|be)\s*\(?\s*(?:401|403)\b`)
	// rspecDenialStatusQuoted is the same status given as a string, read from the raw text.
	rspecDenialStatusQuoted = regexp.MustCompile(`\bhave_http_status\s*\(?\s*["'](?:401|403)["']|\.\s*(?:status|code)\s*\)\s*\.\s*to\s*\(?\s*(?:eq|eql)\s*\(?\s*["'](?:401|403)["']`)
	// rspecNegation is a negated expectation; a denial matcher after it is not a denial.
	rspecNegation = regexp.MustCompile(`\b(?:not_to|to_not|should_not)\b`)
	// rspecPositiveMatcher names the matcher of a positive expectation (group 1).
	rspecPositiveMatcher = regexp.MustCompile(`(?:\.\s*to\b|\bshould\b)\s*\(?\s*([a-z_]\w*[?!]?)`)
	// rspecDenialWord is a matcher name that reads as a refusal.
	rspecDenialWord = regexp.MustCompile(`forbidden|unauthori[sz]ed|denied|not_authori[sz]ed|not_permitted`)
	// rspecRequestVerb is a bare Rails request helper call; group 1 is the verb. A receiver, a
	// symbol, a hash key, an assignment or a bare reference is not a call.
	rspecRequestVerb = regexp.MustCompile(`(?:^|[^.\w:@$])(get|head|post|put|patch|delete)(?:\s*\(|[ \t]+[^\s=,)\]}.|&?])`)
	// rspecNoEffect is an assertion over persisted state or outbound calls: a change or count
	// matcher, a reload or record lookup inside an expectation, or a negated message, request,
	// job, mail or broadcast expectation.
	rspecNoEffect = regexp.MustCompile(`\bchange\s*(?:\(|\{|do\b)|\bexpect\s*\(.*(?:\.(?:count|size|length|reload|exists\?|find_by!?|where|pluck|first|last)\b|\bfind_by\b)|(?:\b(?:not_to|to_not)\s*\(?\s*)(?:have_received|receive|receive_messages|have_been_made|have_requested|have_been_requested|have_enqueued_\w+|have_been_enqueued|have_been_performed|have_performed_\w+|enqueue_\w+|send_email|have_broadcasted_to|broadcast_to|exist)\b|\bassert_no_(?:difference|changes|enqueued_\w+|performed_\w+|emails)\b`)
	// rspecNegatedBlockMatcher names the matcher of a negated block expectation (group 1).
	rspecNegatedBlockMatcher = regexp.MustCompile(`\}\s*\.\s*(?:not_to|to_not)\s*\(?\s*([a-z_]\w*[?!]?)|^end\s*\.\s*(?:not_to|to_not)\s*\(?\s*([a-z_]\w*[?!]?)`)
	// rspecBuiltinMatchers are RSpec, rspec-rails and common library matchers whose meaning the
	// reader knows; any other negated block matcher may be a project's own no-effect assertion.
	rspecBuiltinMatchers = map[string]bool{
		"raise_error": true, "raise_exception": true, "output": true, "throw_symbol": true,
		"yield_control": true, "yield_with_args": true, "yield_with_no_args": true, "yield_successive_args": true,
		"change": true, "have_enqueued_job": true, "have_enqueued_mail": true, "have_been_enqueued": true,
		"have_broadcasted_to": true, "broadcast_to": true, "send_email": true, "make_database_queries": true,
	}
)

// denialSite is a line of the example or a block it runs: its `path:line` and stripped text.
type denialSite struct {
	at, text, raw string
}

// denialFindings returns the TSA-V0-564/565 finding of one example, or none.
func (r *rspecReader) denialFindings(test string, e rspecExample) []testvalidity.SourceFinding {
	own := sites(e.body)
	denial, customDenial, customOther := "", "", ""
	for _, s := range own {
		if rspecNoEffect.MatchString(s.text) {
			return nil
		}
		if at, ok := denialAt(s); ok && denial == "" {
			denial = at
		}
		if d, o := customMatcherAt(s); customDenial == "" && d {
			customDenial = s.at
		} else if customOther == "" && o {
			customOther = s.at
		}
	}
	location := denial
	if location == "" {
		location = customDenial
	}
	if location == "" {
		return nil
	}
	finding := func(rule, cause, message string) []testvalidity.SourceFinding {
		return []testvalidity.SourceFinding{{Rule: rule, Severity: testvalidity.SeverityAdvisory, Test: test, Title: e.title, Location: location, Cause: cause, Message: message}}
	}
	mutating, safe, shadowed := "", false, false
	for _, s := range r.requestSites(e, own) {
		for _, m := range rspecRequestVerb.FindAllStringSubmatch(s.text, -1) {
			switch verb := m[1]; {
			case r.declaredOnChain(e.group, verb):
				shadowed = true
			case verb == "get" || verb == "head":
				safe = true
			case mutating == "":
				mutating = s.at
			}
		}
	}
	custom := customDenial
	if custom == "" || denial != "" {
		custom = customOther
	}
	switch {
	case mutating == "" && safe:
		return nil // a refused read has no effect to prove
	case custom != "":
		return finding(testvalidity.RuleRBDenialMatcherCustom, custom,
			"the example asserts a refusal and holds a matcher the reader does not know, which may itself assert the refusal or that nothing changed, so it abstains from judging whether the denial proves no effect")
	case mutating != "":
		return finding(testvalidity.RuleRBDenialWithoutNoEffect, mutating, fmt.Sprintf(
			"the example asserts only that the mutating request at %s was refused, not that it had no effect, so a handler that writes before it denies stays green; add a no-effect assertion (expect { request }.not_to change(...), a reload comparison, or not_to have_received on the outbound client)", mutating))
	case shadowed:
		return finding(testvalidity.RuleRBDenialVerbUnresolved, fmt.Sprintf("%s:%d", e.path, e.line),
			"the example asserts a refusal, but a let or subject on its group chain shadows a request helper name, so the reader cannot tell which request it observes and abstains")
	}
	return finding(testvalidity.RuleRBDenialVerbUnresolved, fmt.Sprintf("%s:%d", e.path, e.line),
		"the example asserts a refusal, but no literal get, head, post, put, patch or delete call is read in it, its hooks or the subject and lets it calls, so the reader cannot tell whether the refused request mutates and abstains")
}

// sites returns the lines as denial sites.
func sites(lines []rubyLine) []denialSite {
	out := make([]denialSite, 0, len(lines))
	for _, l := range lines {
		out = append(out, denialSite{at: fmt.Sprintf("%s:%d", l.path, l.line), text: stripRubyStrings(l.text), raw: l.text})
	}
	return out
}

// denialAt returns the site's location when it asserts a built-in refusal status positively.
func denialAt(s denialSite) (string, bool) {
	for _, re := range []*regexp.Regexp{rspecDenialStatus, rspecDenialStatusQuoted} {
		text := s.text
		if re == rspecDenialStatusQuoted {
			text = s.raw
		}
		loc := re.FindStringIndex(text)
		if loc == nil {
			continue
		}
		if neg := rspecNegation.FindStringIndex(text); neg != nil && neg[0] < loc[0] {
			continue
		}
		return s.at, true
	}
	return "", false
}

// customMatcherAt reports whether the site holds a positive matcher that reads as a refusal but is
// not a built-in one (denial), or a negated block matcher the reader does not know (other).
func customMatcherAt(s denialSite) (denial, other bool) {
	for _, m := range rspecPositiveMatcher.FindAllStringSubmatch(s.text, -1) {
		if name := m[1]; rspecDenialWord.MatchString(name) && name != "be_forbidden" && name != "be_unauthorized" {
			return true, false
		}
	}
	for _, m := range rspecNegatedBlockMatcher.FindAllStringSubmatch(s.text, -1) {
		if !rspecBuiltinMatchers[m[1]+m[2]] {
			return false, true
		}
	}
	return false, false
}

// requestSites returns the lines a request of example e can sit in: the example body, then per
// group on its chain, innermost first, the bodies of the subject and lets the example or a hook
// calls by name, in name order, and the group's before, let! and subject! hooks.
func (r *rspecReader) requestSites(e rspecExample, own []denialSite) []denialSite {
	out := append([]denialSite{}, own...)
	names := invokedNames(e.body)
	if strings.Contains(stripRubyStrings(joinLines(e.body)), "is_expected") {
		names["subject"] = true
	}
	for g := e.group; g >= 0; g = r.groups[g].parent {
		for _, h := range r.groups[g].hooks {
			for name := range h.names {
				names[name] = true
			}
		}
	}
	seen := map[string]bool{}
	for g := e.group; g >= 0; g = r.groups[g].parent {
		declared := make([]string, 0, len(r.groups[g].bodies))
		for name := range r.groups[g].bodies {
			declared = append(declared, name)
		}
		sort.Strings(declared)
		for _, name := range declared {
			if names[name] && !seen[name] {
				seen[name] = true
				out = append(out, sites(r.groups[g].bodies[name])...)
			}
		}
		for _, h := range r.groups[g].hooks {
			out = append(out, sites(h.body)...)
		}
	}
	return out
}

// declaredOnChain reports whether a let or subject named name is declared on group g's chain.
func (r *rspecReader) declaredOnChain(g int, name string) bool {
	for ; g >= 0; g = r.groups[g].parent {
		if _, ok := r.groups[g].decls[name]; ok {
			return true
		}
	}
	return false
}

func joinLines(lines []rubyLine) string {
	texts := make([]string, len(lines))
	for k, l := range lines {
		texts[k] = l.text
	}
	return strings.Join(texts, "\n")
}
