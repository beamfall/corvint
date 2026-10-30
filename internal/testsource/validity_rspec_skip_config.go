package testsource

import (
	"fmt"
	"regexp"
	"sort"
	"strings"

	"github.com/Beamfall/corvint/internal/testvalidity"
)

// Conditional and configuration-driven skips (TSA-V0-450, V1-1837). A skip the static reader
// cannot decide does not leave the example clean: the example abstains with a named rule.

var (
	// rspecFilterRunExcluding is a `filter_run_excluding` call; the rest of the line is its
	// arguments.
	rspecFilterRunExcluding = regexp.MustCompile(`\bfilter_run_excluding\b(.*)$`)
	// rspecMetadataPair is a `key: VALUE` or `:key => VALUE` metadata pair; group 1 or 2 is the key
	// and group 3 the raw VALUE up to the next separator.
	rspecMetadataPair = regexp.MustCompile(`(?:^|[,({\s])(?:(\w+):|:(\w+)\s*=>)\s*([^,)}\s]*)`)
	// rspecMetadataValue is a VALUE the reader resolves: a symbol, true, false, nil or a number.
	rspecMetadataValue = regexp.MustCompile(`^(?::\w+|true|false|nil|\d+)$`)
	// rspecMetadataTag is a `:tag` symbol, which RSpec reads as `tag: true`.
	rspecMetadataTag = regexp.MustCompile(`(?:^|[,({\s]):(\w+)\b`)
)

// rspecExclusion is one metadata filter a committed support file excludes from every run.
type rspecExclusion struct {
	key, value string // value "" matches any value of key
	cause      string // path:line of the filter_run_excluding call
}

// rspecMetadataPairs returns a declaring line's metadata as key to value, outside string
// literals and before its block opens. A `:tag` symbol is `tag` = "true"; a value that is not a
// symbol, true, false, nil or a number (a constant, lambda or call) is unknown, "".
func rspecMetadataPairs(text string) map[string]string {
	text = stripRubyStrings(text)
	text = text[:rspecMetadataEnd(text)]
	out := map[string]string{}
	for _, m := range rspecMetadataPair.FindAllStringSubmatch(text, -1) {
		key := m[1] + m[2]
		out[key] = ""
		if rspecMetadataValue.MatchString(m[3]) {
			out[key] = m[3]
		}
	}
	for _, m := range rspecMetadataTag.FindAllStringSubmatch(rspecMetadataPair.ReplaceAllString(text, " "), -1) {
		out[m[1]] = "true"
	}
	return out
}

// rspecMetadataEnd is where a declaring line's block opens: a `do`, or a `{` that does not open a
// hash argument (one after `(` or `,`), else the line's length.
func rspecMetadataEnd(text string) int {
	for _, at := range rspecBlockStart.FindAllStringIndex(text, -1) {
		before := strings.TrimRight(text[:at[0]], " \t")
		if text[at[0]] == '{' && (strings.HasSuffix(before, "(") || strings.HasSuffix(before, ",")) {
			continue
		}
		return at[0]
	}
	return len(text)
}

// rspecExclusions reads every `filter_run_excluding` line of the bound support files, in path then
// line order. A filter whose value is false or nil excludes nothing.
func rspecExclusions(files map[string]string) []rspecExclusion {
	paths := make([]string, 0, len(files))
	for path := range files {
		paths = append(paths, path)
	}
	sort.Strings(paths)
	out := []rspecExclusion{}
	for _, path := range paths {
		for i, line := range strings.Split(files[path], "\n") {
			m := rspecFilterRunExcluding.FindStringSubmatch(stripRubyStrings(stripRubyComment(line)))
			if m == nil {
				continue
			}
			pairs := rspecMetadataPairs(m[1])
			keys := make([]string, 0, len(pairs))
			for key := range pairs {
				keys = append(keys, key)
			}
			sort.Strings(keys)
			for _, key := range keys {
				if value := pairs[key]; value != "false" && value != "nil" {
					out = append(out, rspecExclusion{key: key, value: value, cause: fmt.Sprintf("%s:%d", path, i+1)})
				}
			}
		}
	}
	return out
}

// configExcluded returns the cause of the first exclusion whose key the example or one of its
// groups carries with a matching value, innermost declaration first, or "".
func (r *rspecReader) configExcluded(e rspecExample) string {
	if len(r.excluded) == 0 {
		return ""
	}
	heads := []string{e.body[0].text}
	for g := e.group; g > 0; g = r.groups[g].parent {
		heads = append(heads, r.groups[g].head)
	}
	for _, head := range heads {
		pairs := rspecMetadataPairs(head)
		for _, x := range r.excluded {
			if value, ok := pairs[x.key]; ok && value != "false" && value != "nil" && (x.value == "" || value == "" || value == x.value) {
				return x.cause
			}
		}
	}
	return ""
}

// conditionalSkip returns the line of the first skip or pending statement in an example body that
// is not an unconditional TSA-V0-063 statement: one with a trailing if or unless, or one inside a
// nested block, or 0.
func conditionalSkip(body []rubyLine) int {
	for k := 1; k < len(body); k++ {
		if rspecSkipStatement.MatchString(body[k].text) {
			return body[k].line
		}
	}
	return 0
}

// skipConfigFindings returns the abstained RSPEC-CONDITIONAL-SKIP and RSPEC-CONFIG-EXCLUDED
// findings of an example no RSPEC-SKIPPED finding covers (TSA-V0-450).
func (r *rspecReader) skipConfigFindings(test string, e rspecExample) []testvalidity.SourceFinding {
	out := []testvalidity.SourceFinding{}
	at := func(line int) string { return fmt.Sprintf("%s:%d", e.path, line) }
	if line := conditionalSkip(e.body); line != 0 {
		out = append(out, testvalidity.SourceFinding{Rule: testvalidity.RuleRSpecConditionalSkip, Severity: testvalidity.SeverityAbstained,
			Test: test, Title: e.title, Location: at(line), Cause: at(e.line),
			Message: "the example skips or is pending under a condition the static reader cannot decide, so a run may not execute it"})
	}
	if cause := r.configExcluded(e); cause != "" {
		out = append(out, testvalidity.SourceFinding{Rule: testvalidity.RuleRSpecConfigExcluded, Severity: testvalidity.SeverityAbstained,
			Test: test, Title: e.title, Location: at(e.line), Cause: cause,
			Message: "the example carries metadata a committed filter_run_excluding removes from a run, so a run may not execute it"})
	}
	return out
}
