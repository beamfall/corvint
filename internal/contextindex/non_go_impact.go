package contextindex

import (
	"regexp"
	"strings"

	"github.com/Beamfall/corvint/internal/langplugin/decl"
	"github.com/Beamfall/corvint/internal/langplugin/dialect"
)

// NonGoImpactLanguage names the bounded syntax profiles, never runtime
// closure: the impact dialect a language plugin declares for the path's
// suffix (LGP-V0-015).
func NonGoImpactLanguage(name string) string {
	return decl.DialectFor(decl.ImpactDialect, name)
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

// webImpact is the bounded syntax profile of every impact dialect whose rule
// reads ECMAScript module imports. Core registers it because the web lexer it
// masks with is shared with core's registry, setup and evaluation readings
// (LGP-V0-027); moving it to the TypeScript plugin is a recorded follow-up.
var webImpact = dialect.Impact{
	CodeLines: func(text string) []string { return strings.Split(flowWebMask(text), "\n") },
	Probes: []dialect.Probe{
		{Code: "observed-reflective-dispatch", Pattern: dialect.Reflection},
		{Code: "observed-computed-dispatch", Pattern: regexp.MustCompile(`\]\s*(?:\?\.)?\s*\(`)},
	},
	Loads:      regexp.MustCompile(`\b(?:import|require)\s*\(`),
	StaticLoad: staticWebLoadArgument,
}

// staticWebLoadArgument recognizes only a complete quoted argument inside the
// call's parentheses. A quote-first expression can still concatenate a dynamic
// name.
func staticWebLoadArgument(tail string) bool {
	tail = strings.TrimSpace(tail)
	if len(tail) < 2 || (tail[0] != '\'' && tail[0] != '"') {
		return false
	}
	quote := tail[0]
	for i := 1; i < len(tail); i++ {
		if tail[i] == '\\' {
			i++
			continue
		}
		if tail[i] != quote {
			continue
		}
		return strings.HasPrefix(strings.TrimSpace(tail[i+1:]), ")")
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
//
// extra holds each path's declared-registry rows (NGI-V0-010), appended after
// the path's lexical rows so they stay independent of the result limit.
//
// ruby, when non-nil, is the autoload map the impact used: a Ruby path it
// resolves to a constant has a named reverse-reference rule, so only an
// unmapped Ruby path reports reverse-import-rule-unavailable (NGI-V0-036).
func nonGoImpactUnknowns(index *Index, paths []string, unresolvedImports int, extra map[string][]any, ruby *rubyAutoload) []any {
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
		rule := decl.RuleOf(decl.ImpactDialect, language)
		if rule == decl.ConstantReferences && !ruby.mapped(name) {
			add("reverse-import-rule-unavailable", 0)
		}
		if rule == decl.ModuleImports && unresolvedImports != 0 && !isTestPath(name) {
			add("bare-import-unresolved", 0)
			seen["bare-import-unresolved"]["count"] = unresolvedImports
		}
		// A declared dialect whose plugin registered no profile cannot be
		// read, so it fails closed exactly as unreadable bytes do (LGP-V0-026).
		profile, registered := dialect.LookupImpact(language)
		source, exists := index.Sources[name]
		text, valid, loaded := source.Text()
		if !registered || !exists || !valid || !loaded {
			add("source-analysis-unavailable", 0)
			rows = append(rows, extra[name]...)
			continue
		}
		masked := profile.CodeLines(text)
		raw := strings.Split(text, "\n")
		for i, code := range masked {
			for _, probe := range profile.Probes {
				if probe.Pattern.MatchString(code) {
					add(probe.Code, i+1)
				}
			}
			for _, loc := range profile.Loads.FindAllStringIndex(code, -1) {
				// The shared mask preserves columns, so quoted literal loads
				// can be distinguished without returning any literal body.
				tail := strings.TrimSpace(raw[i][loc[1]:])
				if !profile.StaticLoad(tail) {
					add("dynamic-load-unresolved", i+1)
					break
				}
			}
		}
		rows = append(rows, extra[name]...)
	}
	return rows
}

// ruby, when non-nil, is the autoload map the named-constant reverse
// references applied; the receipt names its roots and inflections
// (NGI-V0-021).
func attachNonGoImpactUnknowns(result map[string]any, index *Index, paths []string, unresolvedImports int, extra map[string][]any, ruby *rubyAutoload) error {
	unknowns := nonGoImpactUnknowns(index, paths, unresolvedImports, extra, ruby)
	if len(unknowns) == 0 {
		return nil
	}
	result["unknowns"] = unknowns
	result["language_profile"] = "non-go-syntax-v0"
	if ruby != nil {
		result["ruby_autoload"] = ruby.report()
	}
	coverage := result["coverage"].(map[string]any)
	coverage["uncertainty"] = append(anySlice(coverage["uncertainty"]), "non-Go syntax impact does not establish dynamic dispatch or test closure")
	return stabilizePacketBytes(result)
}

// constantReferencePath reports whether name's impact dialect runs the
// constant-reference rule (LGP-V0-024).
func constantReferencePath(name string) bool {
	return decl.RuleFor(decl.ImpactDialect, name) == decl.ConstantReferences
}

// moduleImportPath reports whether name's impact dialect runs the ECMAScript
// module-import rule (LGP-V0-024).
func moduleImportPath(name string) bool {
	return decl.RuleFor(decl.ImpactDialect, name) == decl.ModuleImports
}
