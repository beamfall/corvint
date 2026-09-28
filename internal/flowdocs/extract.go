// Package flowdocs produces bounded source observations, never accepted intent
// or witnessed runtime behavior, from immutable native-index source bytes.
package flowdocs

import (
	"fmt"
	"path"
	"regexp"
	"sort"
	"strings"

	"github.com/Beamfall/corvint/internal/contextindex"
	"github.com/Beamfall/corvint/internal/doccorpus"
)

type observation struct {
	key, kind, section, text string
	line, end                int
}
type declaration struct {
	name         string
	line, end    int
	observations []observation
}

var rubyDef = regexp.MustCompile(`^\s*def\s+([A-Za-z_][A-Za-z0-9_!?]*(?:\.[A-Za-z_][A-Za-z0-9_!?]*)?)`)
var rubyClass = regexp.MustCompile(`^\s*(?:class|module)\s+([A-Za-z_][A-Za-z0-9_:]*)`)
var webDef = regexp.MustCompile(`^\s*(?:export\s+(?:default\s+)?)?(?:async\s+)?function\s+([A-Za-z_$][A-Za-z0-9_$]*)\s*\(`)
var webArrow = regexp.MustCompile(`^\s*(?:export\s+)?(?:const|let)\s+([A-Za-z_$][A-Za-z0-9_$]*)\s*=.*=>`)
var call = regexp.MustCompile(`\b([A-Za-z_$][A-Za-z0-9_$]*)(?:[.!?])?\s*\(`)
var rubyCall = regexp.MustCompile(`(?:^|[.;\s])([A-Za-z_][A-Za-z0-9_]*[!?]?)(?:\s|\(|$)`)
var rubyBlock = regexp.MustCompile(`^\s*(?:class|module|def|if|unless|case|begin|while|until|for)\b|\bdo\s*(?:\|[^|]*\|)?\s*$`)
var route = regexp.MustCompile(`^\s*(get|post|put|patch|delete|resources|resource)\s+`)
var registration = regexp.MustCompile(`\.(directive|controller|component|config|get|post|put|patch|delete)\s*\(`)
var safeLiteral = regexp.MustCompile(`^[A-Za-z0-9_/:.# -]{1,160}$`)

// literalAt is called only after an executable token in masked code. It cannot
// discover declarations in strings/comments and never returns arbitrary bodies.
func literalAt(raw string, start int) string {
	if start > len(raw) {
		return ""
	}
	s := strings.TrimSpace(raw[start:])
	if len(s) < 2 || (s[0] != '\'' && s[0] != '"') {
		return ""
	}
	end := strings.IndexByte(s[1:], s[0])
	if end < 0 {
		return ""
	}
	value := s[1 : end+1]
	if !safeLiteral.MatchString(value) {
		return ""
	}
	return value
}
func extract(source contextindex.Source) ([]declaration, []string) {
	ext := path.Ext(source.Path)
	if ext == ".html" {
		return templateDeclarations(source)
	}
	language := "web"
	if ext == ".rb" {
		language = "ruby"
	}
	raw := strings.Split(string(source.Data), "\n")
	code := contextindex.FlowCodeLines(language, string(source.Data))
	ds := []declaration{}
	gaps := []string{}
	// Native declarations establish named functions/methods; lexical extents are
	// deliberately bounded to balanced blocks, never a runtime execution order.
	stack := []string{}
	active := -1
	depth := 0
	startDepth := 0
	for i, line := range code {
		trimmed := strings.TrimSpace(line)
		if language == "ruby" {
			if m := rubyClass.FindStringSubmatch(line); m != nil {
				stack = append(stack, m[1])
			}
			if m := rubyDef.FindStringSubmatch(line); m != nil {
				name := strings.Join(stack, "::")
				if name != "" {
					name += "#"
				}
				name += m[1]
				ds = append(ds, declaration{name: name, line: i + 1, end: i + 1})
				active = len(ds) - 1
				startDepth = depth
			}
			if rubyBlock.MatchString(line) {
				depth++
			}
			if trimmed == "end" || strings.HasPrefix(trimmed, "end;") {
				depth--
				if depth < 0 {
					depth = 0
				}
				if active >= 0 && depth == startDepth {
					ds[active].end = i + 1
					active = -1
				} else if active < 0 && len(stack) > 0 {
					stack = stack[:len(stack)-1]
				}
			}
		} else {
			m := webDef.FindStringSubmatch(line)
			if m == nil {
				m = webArrow.FindStringSubmatch(line)
			}
			if m != nil {
				ds = append(ds, declaration{name: m[1], line: i + 1, end: i + 1})
				active = len(ds) - 1
				startDepth = depth
			}
			depth += strings.Count(line, "{") - strings.Count(line, "}")
			if active >= 0 {
				ds[active].end = i + 1
				if depth <= startDepth {
					active = -1
				}
			}
		}
		if active >= 0 {
			ds[active].end = i + 1
		}
		// Literal registrations are their own entry-point inventory. Computed
		// dispatch remains unresolved rather than guessed from the route string.
		if language == "ruby" {
			if loc := route.FindStringSubmatchIndex(line); loc != nil {
				value := literalAt(raw[i], loc[3])
				if value != "" {
					ds = append(ds, declaration{name: "route:" + line[loc[2]:loc[3]] + ":" + value, line: i + 1, end: i + 1})
				} else {
					gaps = append(gaps, "dynamic Ruby route unresolved")
				}
			}
		} else {
			for _, loc := range registration.FindAllStringSubmatchIndex(line, -1) {
				value := literalAt(raw[i], loc[1])
				if value != "" {
					ds = append(ds, declaration{name: "binding:" + line[loc[2]:loc[3]] + ":" + value, line: i + 1, end: webExtent(code, i)})
				} else {
					gaps = append(gaps, "computed web registration unresolved")
				}
			}
		}
	}
	for n := range ds {
		d := &ds[n]
		if d.end-d.line > 512 {
			gaps = append(gaps, "method extent exceeds 512 lines")
			d.end = d.line
		}
		for i := d.line - 1; i < d.end; i++ {
			line := code[i]
			line = withoutDeclarationName(language, line)
			matches := call.FindAllStringSubmatch(line, -1)
			if language == "ruby" {
				matches = rubyCall.FindAllStringSubmatch(line, -1)
			}
			seen := map[string]bool{}
			for _, m := range matches {
				name := strings.TrimRight(m[1], "!?")
				kind, section := "", "technical-deep-dive"
				switch name {
				case "validate", "validates", "validates_presence_of", "validate_presence", "assert", "ensure":
					kind = "constraint"
				case "authorize", "authorize_user", "authenticate", "authenticate_user", "can", "permit", "policy":
					kind = "permission"
				case "raise", "throw", "fail":
					kind = "constraint"
					section = "exceptions"
				case "save", "saveAll", "create", "update", "update_all", "destroy", "delete", "insert", "persist", "perform_later", "enqueue":
					kind = "side_effect"
				case "fetch", "request", "publish", "send", "perform_async":
					kind = "integration"
				}
				if kind == "" || seen[name] {
					continue
				}
				seen[name] = true
				d.observations = append(d.observations, observation{key: kind + ":" + name, kind: kind, section: section, text: "Source contains a lexical call to " + name + "; dispatch and effects are unresolved.", line: i + 1})
			}
			if strings.Contains(line, "throw ") && !seen["throw"] {
				d.observations = append(d.observations, observation{key: "constraint:throw", kind: "constraint", section: "exceptions", text: "Source contains a throw statement; runtime reachability is unresolved.", line: i + 1})
			}
			if language != "ruby" && (strings.Contains(line, "&&") || strings.Contains(line, "?")) && strings.Contains(line, "<") {
				d.observations = append(d.observations, observation{key: "behavior:jsx-conditional", kind: "behavior", section: "functional-overview", text: "Source contains a JSX conditional candidate; accepted variations and runtime branches are unknown.", line: i + 1})
			}
		}
	}
	// Rails model validations and permission filters often live in class scope.
	if language == "ruby" {
		scope := declaration{name: "module-initialization"}
		for i, line := range code {
			covered := false
			for _, d := range ds {
				covered = covered || i+1 >= d.line && i+1 <= d.end
			}
			if covered {
				continue
			}
			trim := strings.TrimSpace(line)
			for _, name := range []string{"validates", "validate", "validates_presence_of", "before_action"} {
				if strings.HasPrefix(trim, name+" ") || strings.HasPrefix(trim, name+"(") {
					kind := "constraint"
					if name == "before_action" {
						kind = "integration"
					}
					if scope.line == 0 {
						scope.line = i + 1
					}
					scope.end = i + 1
					scope.observations = append(scope.observations, observation{key: kind + ":" + name, kind: kind, section: "technical-deep-dive", text: "Source declares " + name + " hook; validation or authorization execution is unresolved.", line: i + 1})
				}
			}
		}
		if scope.line > 0 {
			ds = append(ds, scope)
		}
	}
	sort.Slice(ds, func(i, j int) bool { return ds[i].name < ds[j].name })
	return ds, gaps
}

func anchor(repo doccorpus.Repository, s contextindex.Source, line int) doccorpus.Anchor {
	lines := strings.SplitAfter(string(s.Data), "\n")
	return doccorpus.Anchor{Repository: repo.ID, Revision: repo.Revision, Path: s.Path, Blob: s.BlobHash, SHA256: doccorpus.Digest(s.Data), Start: line, End: line, SpanSHA256: doccorpus.Digest([]byte(lines[line-1])), Authority: "external-provider", Kind: "source", Reason: "bounded lexical source observation; no runtime or intent authority"}
}
func evidence(a doccorpus.Anchor) doccorpus.Evidence {
	return doccorpus.Evidence{Derivation: "source-derived", Trust: "generated", State: "supported", Freshness: "fresh", Anchors: []doccorpus.Anchor{a}, Limitations: []string{"lexical candidate; dynamic dispatch, execution order and accepted intent unresolved"}}
}
func identity(kind, key string) string {
	return "flowdocs:" + kind + ":" + doccorpus.Digest([]byte(key))[:32]
}
func fail(s string) error { return fmt.Errorf("flowdocs-refused: %s", s) }

func webExtent(code []string, start int) int {
	depth := strings.Count(code[start], "{") - strings.Count(code[start], "}")
	if depth <= 0 {
		return start + 1
	}
	for i := start + 1; i < len(code) && i-start <= 512; i++ {
		depth += strings.Count(code[i], "{") - strings.Count(code[i], "}")
		if depth <= 0 {
			return i + 1
		}
	}
	return start + 1
}

// A declaration name is syntax, even when named save, authorize or fetch.
func withoutDeclarationName(language, line string) string {
	pattern := webDef
	if language == "ruby" {
		pattern = rubyDef
	}
	if loc := pattern.FindStringSubmatchIndex(line); loc != nil {
		return line[:loc[2]] + strings.Repeat(" ", loc[3]-loc[2]) + line[loc[3]:]
	}
	return line
}
