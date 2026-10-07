package contextindex

import (
	"path"
	"regexp"
	"strings"
)

// NonGoImpactLanguage names the bounded syntax profiles, never runtime closure.
func NonGoImpactLanguage(name string) string {
	switch path.Ext(name) {
	case ".rb":
		return "ruby"
	case ".js", ".jsx", ".mjs", ".cjs":
		return "javascript"
	case ".ts", ".tsx":
		return "typescript"
	}
	return ""
}

// MCPImpactPathAdmitted is the closed language set shared by MCP validation and
// range admission. Positional CLI impact retains its wider indexed suffix set.
func MCPImpactPathAdmitted(name string) bool {
	return strings.HasSuffix(name, ".go") || NonGoImpactLanguage(name) != ""
}

func hasNonGoImpactSource(index *Index) bool {
	for name := range index.Sources {
		if NonGoImpactLanguage(name) != "" {
			return true
		}
	}
	return false
}

var impactReflection = regexp.MustCompile(`\b(?:send|public_send|__send__|define_method|define_singleton_method|method_missing|class_eval|module_eval|eval|constantize|const_get)\b`)
var impactComputedCall = regexp.MustCompile(`\]\s*(?:\?\.)?\s*\(`)
var impactDynamicLoad = regexp.MustCompile(`\b(?:import|require)\s*\(`)
var impactRubyLoad = regexp.MustCompile(`\b(?:require_relative|require|load)\b`)
var impactAutoload = regexp.MustCompile(`\bautoload\b`)

// staticImpactLoadArgument recognizes only a complete quoted argument. A
// quote-first expression can still concatenate or interpolate a dynamic name.
func staticImpactLoadArgument(tail, language string) bool {
	tail = strings.TrimSpace(tail)
	parenthesized := language != "ruby" || strings.HasPrefix(tail, "(")
	if language == "ruby" && parenthesized {
		tail = strings.TrimSpace(tail[1:])
	}
	if len(tail) < 2 || (tail[0] != '\'' && tail[0] != '"') {
		return false
	}
	quote := tail[0]
	for i := 1; i < len(tail); i++ {
		if language == "ruby" && quote == '"' && strings.HasPrefix(tail[i:], "#{") {
			return false
		}
		if tail[i] == '\\' {
			i++
			continue
		}
		if tail[i] != quote {
			continue
		}
		rest := strings.TrimSpace(tail[i+1:])
		if parenthesized {
			return strings.HasPrefix(rest, ")")
		}
		return rest == "" || strings.HasPrefix(rest, "#") || strings.HasPrefix(rest, ";")
	}
	return false
}

// nonGoImpactUnknowns intentionally emits a baseline even when lexical probes
// find no dynamic token. Absence of a syntax candidate cannot prove closure.
//
// unresolvedImports is the repository's count of bare web import specifiers
// that resolve to no repository file or declared package (GPK-V0-080,
// proposed). It is reported once per web path, because any of them may name
// that path, so the path's reverse importers are not closed.
func nonGoImpactUnknowns(index *Index, paths []string, unresolvedImports int) []any {
	rows := []any{}
	for _, name := range paths {
		language := NonGoImpactLanguage(name)
		if language == "" {
			continue
		}
		seen := map[string]map[string]any{}
		add := func(code string, line int) {
			if row, ok := seen[code]; ok {
				row["count"] = row["count"].(int) + 1
				return
			}
			row := map[string]any{"path": name, "language": language, "code": code, "line": line, "count": 1}
			seen[code] = row
			rows = append(rows, row)
		}
		add("dynamic-dispatch-unresolved", 0)
		if language == "ruby" {
			add("reverse-import-rule-unavailable", 0)
		}
		if language != "ruby" && unresolvedImports != 0 && !isTestPath(name) {
			add("bare-import-unresolved", 0)
			seen["bare-import-unresolved"]["count"] = unresolvedImports
		}
		source, exists := index.Sources[name]
		text, valid, loaded := source.Text()
		if !exists || !valid || !loaded {
			add("source-analysis-unavailable", 0)
			continue
		}
		masked := FlowCodeLines(language, text)
		raw := strings.Split(text, "\n")
		for i, code := range masked {
			if impactReflection.MatchString(code) {
				add("observed-reflective-dispatch", i+1)
			}
			if language != "ruby" && impactComputedCall.MatchString(code) {
				add("observed-computed-dispatch", i+1)
			}
			if language == "ruby" && impactAutoload.MatchString(code) {
				add("autoload-unresolved", i+1)
			}
			loads := impactDynamicLoad
			if language == "ruby" {
				loads = impactRubyLoad
			}
			for _, loc := range loads.FindAllStringIndex(code, -1) {
				// The shared mask preserves columns, so quoted literal loads
				// can be distinguished without returning any literal body.
				tail := strings.TrimSpace(raw[i][loc[1]:])
				if !staticImpactLoadArgument(tail, language) {
					add("dynamic-load-unresolved", i+1)
					break
				}
			}
		}
	}
	return rows
}

func attachNonGoImpactUnknowns(result map[string]any, index *Index, paths []string, unresolvedImports int) error {
	unknowns := nonGoImpactUnknowns(index, paths, unresolvedImports)
	if len(unknowns) == 0 {
		return nil
	}
	result["unknowns"] = unknowns
	result["language_profile"] = "non-go-syntax-v0"
	coverage := result["coverage"].(map[string]any)
	coverage["uncertainty"] = append(anySlice(coverage["uncertainty"]), "non-Go syntax impact does not establish dynamic dispatch or test closure")
	return stabilizePacketBytes(result)
}
