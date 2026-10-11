package typescript

import (
	"regexp"
	"strings"
)

// playwrightGrepOmissions holds the selected (project, file) pairs a listing omits that the
// project's grep or grepInvert accounts for (TJAA-V0-021): explained pairs, which the static pattern
// excludes for every listed case of the file, and unverified pairs, whose pattern is not statically
// evaluable.
type playwrightGrepOmissions struct {
	explained, unverified []PlaywrightDiscoveryUnit
}

type playwrightOmission int

const (
	playwrightOmissionUnexplained playwrightOmission = iota
	playwrightOmissionExplained
	playwrightOmissionUnverified
)

// playwrightTitleMatcher is one project grep or grepInvert regular expression that matches a
// Playwright grep title exactly when it matches one of the title's space-separated parts.
type playwrightTitleMatcher struct {
	re   *regexp.Regexp
	fold bool
}

// classifyPlaywrightGrepOmission decides why a project that declares grep or grepInvert omits a
// file whose listed cases (from any project) are cases. Playwright keeps a case in a project only
// when its grep title, `project file describe... title tags...` joined by single spaces, does not
// match grepInvert and matches grep (any element of an array). The omission is explained when
// every listed case is excluded for this project; it is unverified when a declared pattern is not
// evaluable here or a case title is outside the matcher model; otherwise, including a file with no
// listed case, it is unexplained. A case filtered out of every project is not listed; under the
// caller-declared listing it is excluded for this project too.
func classifyPlaywrightGrepOmission(project PlaywrightProject, cases [][]string) playwrightOmission {
	grep, grepOK := playwrightTitleMatchers(project.hasGrep, project.Grep)
	invert, invertOK := playwrightTitleMatchers(project.hasGrepInvert, project.GrepInvert)
	if !grepOK || !invertOK {
		return playwrightOmissionUnverified
	}
	if len(cases) == 0 {
		return playwrightOmissionUnexplained
	}
	for _, tokens := range cases {
		tokens = append([]string{project.Name}, tokens...)
		for _, token := range tokens {
			if playwrightOutsideBMP(token) || strings.ContainsAny(token, "ſK") && playwrightTitleMatchersFold(grep, invert) {
				return playwrightOmissionUnverified
			}
		}
		excluded := project.hasGrepInvert && playwrightTitleMatch(invert, tokens) || project.hasGrep && !playwrightTitleMatch(grep, tokens)
		if !excluded {
			return playwrightOmissionUnexplained
		}
	}
	return playwrightOmissionExplained
}

// playwrightTitleMatchers compiles a declared grep or grepInvert value, one regular-expression
// literal or an array of them. It reports false when the value is dynamic or any element is not a
// regular-expression literal whose match against the joined grep title is decided part by part:
// the body must be static (playwrightRegexBodyStatic), carry only the i, m and s flags (an ASCII
// body under i), and contain no construct that can match or assert across the separating space:
// no `.`, `^`, `$`, literal space, `\D`, `\W`, negated class, class holding a space, or class
// range starting at or below U+0020.
func playwrightTitleMatchers(declared bool, raw string) ([]playwrightTitleMatcher, bool) {
	if !declared {
		return nil, true
	}
	if raw == "" {
		return nil, false
	}
	items, array := playwrightArrayItems(raw)
	if !array {
		items = []string{raw}
	}
	matchers := make([]playwrightTitleMatcher, 0, len(items))
	for _, item := range items {
		body, flags, ok := playwrightRegexLiteral(strings.TrimSpace(item))
		if !ok || !playwrightRegexBodyStatic(body) || !playwrightRegexBodyPartwise(body) {
			return nil, false
		}
		prefix, fold := "", false
		for _, flag := range flags {
			switch flag {
			case 'i':
				prefix, fold = prefix+"(?i)", true
			case 'm':
				prefix += "(?m)"
			case 's':
				prefix += "(?s)"
			default:
				return nil, false
			}
		}
		if fold && strings.IndexFunc(body, func(r rune) bool { return r > 0x7f }) >= 0 {
			return nil, false // JavaScript and Go case folding agree only on ASCII
		}
		re, err := regexp.Compile(prefix + body)
		if err != nil {
			return nil, false
		}
		matchers = append(matchers, playwrightTitleMatcher{re: re, fold: fold})
	}
	return matchers, true
}

// playwrightRegexBodyPartwise reports whether a static regular-expression body can neither match
// a space nor anchor to the start or end of the whole title, so it matches the space-joined grep
// title exactly when it matches one of its parts: a match never spans a separator, and `\b`/`\B`
// see a separator as they see a string edge, as a non-word character.
func playwrightRegexBodyPartwise(body string) bool {
	for index := 0; index < len(body); index++ {
		switch body[index] {
		case '.', '^', '$', ' ':
			return false
		case '\\':
			index++
			if body[index] == 'D' || body[index] == 'W' {
				return false
			}
		case '{':
			index = strings.IndexByte(body[index:], '}') + index
		case '[':
			if body[index+1] == '^' {
				return false
			}
			for index++; body[index] != ']'; index++ {
				character := body[index]
				if character <= ' ' {
					return false
				}
				if character == '\\' {
					index++
					escape := body[index]
					if escape == 'D' || escape == 'W' || strings.IndexByte("tnrfv", escape) >= 0 && body[index+1] == '-' {
						return false
					}
				}
			}
		}
	}
	return true
}

func playwrightTitleMatch(matchers []playwrightTitleMatcher, tokens []string) bool {
	for _, matcher := range matchers {
		for _, token := range tokens {
			if matcher.re.MatchString(token) {
				return true
			}
		}
	}
	return false
}

// playwrightTitleMatchersFold reports whether any matcher folds case; Go folds k and s to U+212A
// and U+017F, which JavaScript without the u flag does not.
func playwrightTitleMatchersFold(groups ...[]playwrightTitleMatcher) bool {
	for _, group := range groups {
		for _, matcher := range group {
			if matcher.fold {
				return true
			}
		}
	}
	return false
}
