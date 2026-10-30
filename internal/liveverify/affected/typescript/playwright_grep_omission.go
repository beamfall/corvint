package typescript

import (
	"regexp"
	"slices"
	"strings"
)

// playwrightGrepOmissions holds the selected (project, file) pairs a listing omits that the
// project's grep or grepInvert accounts for (TJAA-V0-021): explained pairs, where a static
// grepInvert matches a title part of every listed case of the file, and unverified pairs, whose
// exclusion rests on a pattern that is not statically evaluable, on grep matching no title, or on
// tags the listing reports only in part.
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

// playwrightListCase is one listed case of a file: titles holds the file suite title, each
// describe title and the test title; tags holds the reported tags with their `@` restored.
type playwrightListCase struct {
	titles, tags []string
}

// classifyPlaywrightGrepOmission decides why a project that declares grep or grepInvert omits a
// file whose listed cases (from any project) are cases. Playwright keeps a case in a project only
// when its grep title does not match grepInvert and matches grep (any element of an array). That
// title joins, with single spaces, the project name, the file suite title, the config tags, each
// describe title and its explicit tags, the test title and the test's explicit tags. The listing
// shows the titles exactly but not the explicit tags: it reports each case's tags as the `@\S+`
// runs of the config tags, describe titles and tags and test title, then the test's own tags, so a
// config or describe tag containing whitespace is reported only in part and one that is `@` alone
// or has whitespace right after `@` not at all. A case is therefore excluded for certain only when
// grepInvert matches a title part, which no hidden text can undo. Otherwise the reported tags, less
// the runs of the titles, stand in for the explicit tags: a case they keep is unexplained, and a
// case they exclude is unverified, because hidden tag text could still match grep or fail to
// match grepInvert. The omission is explained when every case is excluded for certain; unverified
// when a declared pattern is not evaluable here, a title part is outside the matcher model, or a
// case's exclusion is unverified; otherwise, including a file with no listed case, it is
// unexplained. A case filtered out of every project is not listed; under the caller-declared
// listing it is excluded for this project too. A non-nil sourceTags means the sources prove no tag
// text is hidden and hold exactly those literal tags (playwrightTagSources); a case whose explicit
// tags are all among them has tokens equal to its grep title's parts, so a case they exclude is
// excluded for certain (AFP-V0-409, AFP-V0-410).
func classifyPlaywrightGrepOmission(project PlaywrightProject, cases []playwrightListCase, sourceTags map[string]bool) playwrightOmission {
	grep, grepOK := playwrightTitleMatchers(project.hasGrep, project.Grep)
	invert, invertOK := playwrightTitleMatchers(project.hasGrepInvert, project.GrepInvert)
	if !grepOK || !invertOK || len(cases) == 0 {
		if len(cases) == 0 {
			return playwrightOmissionUnexplained
		}
		return playwrightOmissionUnverified
	}
	fold := playwrightTitleMatchersFold(grep, invert)
	verdict := playwrightOmissionExplained
	for _, listCase := range cases {
		parts := append([]string{project.Name}, listCase.titles...)
		if len(listCase.titles) == 0 || slices.ContainsFunc(append(slices.Clone(parts), listCase.tags...), func(part string) bool {
			return playwrightOutsideBMP(part) || fold && strings.ContainsAny(part, "\u017f\u212a")
		}) {
			verdict = playwrightOmissionUnverified // outside the matcher model; a later case may still be kept
			continue
		}
		if project.hasGrepInvert && playwrightTitleMatch(invert, parts) {
			continue
		}
		explicit, ok := playwrightExplicitTags(listCase.tags, playwrightTitleTags(strings.Join(listCase.titles[1:], " ")))
		if !ok {
			verdict = playwrightOmissionUnverified
			continue
		}
		tokens := append(parts, explicit...)
		if !(project.hasGrepInvert && playwrightTitleMatch(invert, tokens) || project.hasGrep && !playwrightTitleMatch(grep, tokens)) {
			return playwrightOmissionUnexplained
		}
		if sourceTags == nil || slices.ContainsFunc(explicit, func(tag string) bool { return !sourceTags[tag] }) {
			verdict = playwrightOmissionUnverified
		}
	}
	return verdict
}

// playwrightTitleTags returns the matches of JavaScript's /@[\S]+/g over title, as Playwright's
// test.tags derives title tags: an `@` followed by the longest run of non-whitespace characters,
// whitespace being JavaScript's \s set.
func playwrightTitleTags(title string) []string {
	tags := []string{}
	runes := []rune(title)
	for index := 0; index < len(runes); index++ {
		if runes[index] != '@' || index+1 == len(runes) || playwrightJSSpace(runes[index+1]) {
			continue
		}
		end := index + 1
		for end < len(runes) && !playwrightJSSpace(runes[end]) {
			end++
		}
		tags = append(tags, string(runes[index:end]))
		index = end - 1
	}
	return tags
}

// playwrightJSSpace reports whether r is in JavaScript's \s set.
func playwrightJSSpace(r rune) bool {
	switch r {
	case '\t', '\n', '\v', '\f', '\r', ' ', 0xa0, 0x1680, 0x2028, 0x2029, 0x202f, 0x205f, 0x3000, 0xfeff:
		return true
	}
	return r >= 0x2000 && r <= 0x200a
}

// playwrightExplicitTags removes each title tag once from the reported tags, keeping the rest in
// order; it reports false when a title tag is not among them, which a real listing never does.
func playwrightExplicitTags(reported, titleTags []string) ([]string, bool) {
	remaining := slices.Clone(reported)
	for _, tag := range titleTags {
		index := slices.Index(remaining, tag)
		if index < 0 {
			return nil, false
		}
		remaining = slices.Delete(remaining, index, index+1)
	}
	return remaining, true
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
