package testsource

// Test-side uses of an environment profile (docs/specs/test-source-analysis-v0.md,
// TSA-V0-214..215): which tests read a declared variable, which re-derive a service address from a
// hard-coded offset or literal, and which of those a bootstrap change leaves pointing at the wrong
// address.

import (
	"path"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

// Environment warning rules and dependency kinds (TSA-V0-214, TSA-V0-215).
const (
	RuleEnvHardCodedOffset  = "ENV-HARD-CODED-OFFSET"
	RuleEnvHardCodedAddress = "ENV-HARD-CODED-ADDRESS"
	DependencyDeclared      = "DECLARED_VARIABLE"
	DependencyOffset        = "HARD_CODED_OFFSET"
	DependencyLiteral       = "HARD_CODED_LITERAL"
)

var (
	envRead        = regexp.MustCompile(`process\.env\.([A-Za-z_][A-Za-z0-9_]*)|process\.env\[\s*['"]([A-Za-z_][A-Za-z0-9_]*)['"]\s*\]`)
	envOffsetTail  = regexp.MustCompile(`^(?:\s*(?:\?\?|\|\|)\s*(?:'[^']*'|"[^"]*"|[0-9]+))?\s*(?:,\s*[0-9]+\s*)?(\))?\s*([+-])\s*([0-9]+)\b`)
	literalAddress = regexp.MustCompile(`\b(?:localhost|127\.0\.0\.1|0\.0\.0\.0|\[::1\]):([0-9]{2,5})\b`)
)

// testEnvRef is one test-side use of an environment value.
type testEnvRef struct {
	path, kind, variable string
	line                 int
	offset               int
	numeric              bool
	literal              string
	warning              *EnvWarning
}

// envTestFile keeps test files and the runner configurations that hand addresses to tests.
func envTestFile(name string) bool {
	base := path.Base(name)
	return IsTestFile(name) || webExtensions[path.Ext(name)] && (strings.HasPrefix(base, "playwright.config.") || strings.HasPrefix(base, "cypress.config."))
}

// testEnvRefs reads every test-side use in path and line order and warns on each hard-coded
// derivation (TSA-V0-214).
func testEnvRefs(texts map[string]string, model profileModel) []testEnvRef {
	names := []string{}
	for name := range texts {
		if envTestFile(name) {
			names = append(names, name)
		}
	}
	sort.Strings(names)
	out := []testEnvRef{}
	for _, name := range names {
		for i, line := range strings.Split(texts[name], "\n") {
			out = append(out, lineEnvRefs(name, i+1, line, model)...)
		}
	}
	return out
}

func lineEnvRefs(name string, number int, line string, model profileModel) []testEnvRef {
	out := []testEnvRef{}
	trimmed := strings.TrimSpace(line)
	if strings.HasPrefix(trimmed, "//") || strings.HasPrefix(trimmed, "*") {
		return out
	}
	for _, m := range envRead.FindAllStringSubmatchIndex(line, -1) {
		variable := submatch(line, m, 1) + submatch(line, m, 2)
		tail := envOffsetTail.FindStringSubmatch(line[m[1]:])
		if tail == nil {
			if _, declared := model.byName[variable]; declared {
				out = append(out, testEnvRef{path: name, line: number, kind: DependencyDeclared, variable: variable})
			}
			continue
		}
		n, _ := strconv.Atoi(tail[3])
		if tail[2] == "-" {
			n = -n
		}
		// Without a closing conversion call, process.env.X + N concatenates strings: the value is
		// left unknown rather than computed as arithmetic.
		ref := testEnvRef{path: name, line: number, kind: DependencyOffset, variable: variable, offset: n, numeric: tail[1] != ""}
		computed := ref.computed(model)
		ref.warning = &EnvWarning{Rule: RuleEnvHardCodedOffset, Path: name, Line: number, Text: clip(trimmed), ComputedValue: computed,
			DeclaredAlternatives: declaredAlternatives(model, computed),
			Message:              "test derives a service address as " + variable + " " + tail[2] + " " + tail[3] + " instead of reading the variable the bootstrap declares for it"}
		out = append(out, ref)
	}
	for _, m := range literalAddress.FindAllStringSubmatchIndex(line, -1) {
		port := line[m[2]:m[3]]
		ref := testEnvRef{path: name, line: number, kind: DependencyLiteral, literal: port}
		ref.warning = &EnvWarning{Rule: RuleEnvHardCodedAddress, Path: name, Line: number, Text: clip(trimmed), ComputedValue: &port,
			DeclaredAlternatives: declaredAlternatives(model, &port),
			Message:              "test hard-codes the service address " + line[m[0]:m[1]] + " instead of reading the variable the bootstrap declares for it"}
		out = append(out, ref)
	}
	return out
}

func submatch(text string, m []int, group int) string {
	if m[2*group] < 0 {
		return ""
	}
	return text[m[2*group]:m[2*group+1]]
}

func clip(text string) string {
	if len(text) > 200 {
		return text[:200]
	}
	return text
}

// computed is the value a hard-coded derivation yields in the profile, or nil when unknown.
func (r testEnvRef) computed(model profileModel) *string {
	switch r.kind {
	case DependencyLiteral:
		return &r.literal
	case DependencyOffset:
		v, ok := model.byName[r.variable]
		if !ok || v.Value == nil || !r.numeric {
			return nil
		}
		n, err := strconv.Atoi(*v.Value)
		if err != nil {
			return nil
		}
		out := strconv.Itoa(n + r.offset)
		return &out
	}
	return nil
}

// carries reports a value that is, or is an address ending in, the port.
func carries(value *string, port *string) bool {
	return value != nil && port != nil && (*value == *port || strings.HasSuffix(*value, ":"+*port) || strings.Contains(*value, ":"+*port+"/"))
}

// declaredAlternatives are the variables passed to tests that carry the computed value.
func declaredAlternatives(model profileModel, computed *string) []string {
	out := []string{}
	for _, v := range model.report.Variables {
		if v.PassedToTests && carries(v.Value, computed) {
			out = append(out, v.Name)
		}
	}
	return out
}

// variables are the profile variables the use depends on: the variable read, or the variables
// whose value a hard-coded derivation reproduces.
func (r testEnvRef) variables(model profileModel) []string {
	if r.kind == DependencyDeclared {
		return []string{r.variable}
	}
	computed := r.computed(model)
	out := []string{}
	for _, v := range model.report.Variables {
		if v.Name != r.variable && carries(v.Value, computed) {
			out = append(out, v.Name)
		}
	}
	return out
}

func sameValue(a, b *string) bool {
	return a == nil && b == nil || a != nil && b != nil && *a == *b
}

// EnvChanges lists the variables whose value or derivation differs between the base and the
// revision, with the tests that depend on each (TSA-V0-215). A use reading the declared variable
// stays valid; a hard-coded derivation that reproduced the old value stays valid only when it
// still reproduces the new one; an unresolved value leaves the hard-coded uses unknown.
func EnvChanges(baseTexts, texts map[string]string, profile string) []EnvChange {
	before, after := readProfile(baseTexts, profile), readProfile(texts, profile)
	refs := testEnvRefs(texts, after)
	names := map[string]bool{}
	for name := range before.byName {
		names[name] = true
	}
	for name := range after.byName {
		names[name] = true
	}
	sorted := make([]string, 0, len(names))
	for name := range names {
		sorted = append(sorted, name)
	}
	sort.Strings(sorted)
	out := []EnvChange{}
	for _, name := range sorted {
		old, now := before.byName[name], after.byName[name]
		change := EnvChange{Variable: name, Dependents: []EnvDependency{}}
		if old != nil {
			change.Before, change.BeforeExpression = old.Value, old.Derivation.Expression
		}
		if now != nil {
			change.After, change.AfterExpression = now.Value, now.Derivation.Expression
		}
		if sameValue(change.Before, change.After) && change.BeforeExpression == change.AfterExpression && (old == nil) == (now == nil) {
			continue
		}
		for _, ref := range refs {
			dep := EnvDependency{Path: ref.path, Line: ref.line, Variable: name, Kind: ref.kind}
			switch {
			case ref.kind == DependencyDeclared && ref.variable == name:
				valid := true
				dep.StaysValid = &valid
			case ref.kind == DependencyDeclared:
				continue
			case change.Before == nil:
				// Membership needs the base value; an offset from this variable is kept unknown.
				if ref.kind != DependencyOffset || ref.variable != name {
					continue
				}
			default:
				if !carries(change.Before, ref.computed(before)) {
					continue
				}
				if now := ref.computed(after); now != nil && change.After != nil {
					valid := carries(change.After, now)
					dep.StaysValid = &valid
				}
			}
			change.Dependents = append(change.Dependents, dep)
		}
		out = append(out, change)
	}
	return out
}
