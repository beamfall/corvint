package typescript

import (
	"regexp"
	"regexp/syntax"
	"strings"
)

// PlaywrightRegexMatcher compiles a Playwright testMatch or testIgnore regular-expression literal,
// its source with slashes and flags (`/global-setup\.ts/i`), to the Go matcher the path-mode
// resolver uses: a static body (playwrightRegexBodyStatic) with only the i, m and s flags. Any
// other literal is not static and is refused (TJAA-V0-127, TSA-V0-654).
func PlaywrightRegexMatcher(source string) (*regexp.Regexp, bool) {
	source = strings.TrimSpace(source)
	if !strings.HasPrefix(source, "/") {
		return nil, false
	}
	matcher, ok := compilePlaywrightMatcher(source)
	return matcher.re, ok
}

// PlaywrightRegexReachesRoot reports whether Playwright's verdict for a static regex testMatch on
// the absolute path of the repository path name can depend on the directories above the
// repository (TJAA-V0-127). Playwright searches the regex in `<checkout>/<name>`. A regex with no
// start anchor and no word boundary decides the same on every checkout when it matches `/<name>`,
// since that is a suffix of the absolute path. When it does not, any match would have to start
// above the repository, which is impossible only for a regex whose every match ends at the end of
// the path and can contain no `/`. Every other regex, a literal that is not static, and a path
// outside the matcher model (playwrightPathOutsideMatcherModel) reach the root.
func PlaywrightRegexReachesRoot(source, name string) bool {
	re, ok := PlaywrightRegexMatcher(source)
	if !ok || playwrightPathOutsideMatcherModel(name) {
		return true
	}
	parsed, err := syntax.Parse(re.String(), syntax.Perl)
	if err != nil || regexContextual(parsed) {
		return true
	}
	if re.MatchString("/" + name) {
		return false
	}
	return !regexEndAnchored(parsed) || regexMatchesSlash(parsed)
}

// PlaywrightRegexContextFree reports a static regex (PlaywrightRegexMatcher) with no start anchor
// and no word boundary, so a match on `/<name>` is a match on `<checkout>/<name>` for every
// checkout (TJAA-V0-127).
func PlaywrightRegexContextFree(source string) bool {
	re, ok := PlaywrightRegexMatcher(source)
	if !ok {
		return false
	}
	parsed, err := syntax.Parse(re.String(), syntax.Perl)
	return err == nil && !regexContextual(parsed)
}

// PlaywrightRegexMatches reports Playwright's verdict for a static regex on the repository path
// name when PlaywrightRegexReachesRoot reports it independent of the checkout.
func PlaywrightRegexMatches(source, name string) bool {
	re, ok := PlaywrightRegexMatcher(source)
	return ok && re.MatchString("/"+name)
}

// regexContextual reports a construct whose match depends on the text before the match: a start
// anchor (also a line start, the path holding no line break) or a word boundary.
func regexContextual(node *syntax.Regexp) bool {
	switch node.Op {
	case syntax.OpBeginLine, syntax.OpBeginText, syntax.OpWordBoundary, syntax.OpNoWordBoundary:
		return true
	}
	for _, sub := range node.Sub {
		if regexContextual(sub) {
			return true
		}
	}
	return false
}

// regexEndAnchored reports that every match of node ends at the end of the subject.
func regexEndAnchored(node *syntax.Regexp) bool {
	switch node.Op {
	case syntax.OpEndText, syntax.OpEndLine:
		return true
	case syntax.OpConcat:
		return len(node.Sub) != 0 && regexEndAnchored(node.Sub[len(node.Sub)-1])
	case syntax.OpCapture:
		return regexEndAnchored(node.Sub[0])
	case syntax.OpAlternate:
		for _, sub := range node.Sub {
			if !regexEndAnchored(sub) {
				return false
			}
		}
		return len(node.Sub) != 0
	}
	return false
}

// regexMatchesSlash reports whether some match of node can contain `/`.
func regexMatchesSlash(node *syntax.Regexp) bool {
	switch node.Op {
	case syntax.OpAnyChar, syntax.OpAnyCharNotNL:
		return true
	case syntax.OpLiteral:
		return strings.ContainsRune(string(node.Rune), '/')
	case syntax.OpCharClass:
		for index := 0; index+1 < len(node.Rune); index += 2 {
			if node.Rune[index] <= '/' && '/' <= node.Rune[index+1] {
				return true
			}
		}
		return false
	}
	for _, sub := range node.Sub {
		if regexMatchesSlash(sub) {
			return true
		}
	}
	return false
}
