package testsource

// Role locators against component markup (docs/specs/test-source-analysis-v0.md,
// TSA-V0-206..209). A component test that mounts or renders a component and locates its error or
// notification output by ARIA role can only match when the component's markup renders that role.
// The check reads committed sources only, before any run; a component it cannot resolve or
// whose markup it cannot close abstains instead of reporting.

import (
	"path"
	"regexp"
	"sort"
	"strings"
)

// RoleLocatorSchema names the `corvint tests role-locators` report (TSA-V0-206).
const RoleLocatorSchema = "corvint-tests-role-locators/0"

// Role-locator outcomes and abstention reasons (TSA-V0-207, TSA-V0-209).
const (
	RuleRoleLocatorUnrendered  = "ROLE-LOCATOR-UNRENDERED"
	roleComponentUnresolved    = "component-unresolved"
	roleComponentMarkupOpen    = "component-markup-open"
	roleComponentNotMounted    = "component-not-mounted"
	roleMarkupChildComponent   = "renders-child-component"
	roleMarkupDynamicRole      = "role-not-literal"
	roleMarkupSpreadAttributes = "spreads-attributes"
	roleMarkupRawHTML          = "sets-inner-html"
	roleMarkupReexport         = "re-exports"
	roleMarkupChildren         = "renders-children"
)

// notificationRoles are the roles of error and notification output the check covers, with the
// HTML elements that carry each implicitly (TSA-V0-207).
var notificationRoles = map[string][]string{"alert": nil, "alertdialog": nil, "status": {"output"}, "log": nil}

var (
	roleLocatorPattern = regexp.MustCompile(`(?:getByRole|findByRole|queryByRole|getAllByRole|findAllByRole|queryAllByRole)\(\s*['"]([a-z]+)['"]` +
		`|['"\x60]role=([a-z]+)|\[role=["']?([a-z]+)["']?\]`)
	mountPattern     = regexp.MustCompile(`\b(?:mount|render)\(\s*<([A-Z][A-Za-z0-9_]*)`)
	defaultImport    = regexp.MustCompile(`import\s+([A-Z][A-Za-z0-9_]*)\s*(?:,\s*\{[^}]*\})?\s*from\s*['"]([^'"]+)['"]`)
	namedImport      = regexp.MustCompile(`import\s*(?:[A-Za-z0-9_]+\s*,\s*)?\{([^}]*)\}\s*from\s*['"]([^'"]+)['"]`)
	literalRoleAttr  = regexp.MustCompile(`\brole\s*=\s*(?:"([a-z]+)"|'([a-z]+)'|\{\s*["'\x60]([a-z]+)["'\x60]\s*\})`)
	anyRoleAttr      = regexp.MustCompile(`\brole\s*=\s*\{`)
	bracedRoleAttr   = regexp.MustCompile(`\brole\s*=\s*\{\s*["'\x60][a-z]+["'\x60]\s*\}`)
	childComponent   = regexp.MustCompile(`<([A-Z][A-Za-z0-9_.]*)[\s/>]`)
	spreadAttributes = regexp.MustCompile(`<[a-zA-Z][^<>]*\{\s*\.\.\.`)
	reexport         = regexp.MustCompile(`export\s+(?:\*|\{[^}]*\})\s*from\s*['"]`)
	childrenSlot     = regexp.MustCompile(`\{\s*(?:props\.)?children\s*\}`)
)

// RoleLocatorFinding is one role locator no mounted component renders (TSA-V0-207).
type RoleLocatorFinding struct {
	Rule       string   `json:"rule"`
	Path       string   `json:"path"`
	Line       int      `json:"line"`
	Role       string   `json:"role"`
	Components []string `json:"components"`
	Message    string   `json:"message"`
}

// RoleLocatorAbstention is one role locator the check could not decide (TSA-V0-209).
type RoleLocatorAbstention struct {
	Path      string `json:"path"`
	Line      int    `json:"line"`
	Role      string `json:"role"`
	Reason    string `json:"reason"`
	Component string `json:"component,omitempty"`
	Detail    string `json:"detail,omitempty"`
}

// RoleLocatorReport is the deterministic role-locator reading of one revision (TSA-V0-206).
type RoleLocatorReport struct {
	Schema      string                  `json:"schema"`
	Revision    string                  `json:"revision"`
	Checked     int                     `json:"checked"`
	Findings    []RoleLocatorFinding    `json:"findings"`
	Abstentions []RoleLocatorAbstention `json:"abstentions"`
}

// RoleLocatorOptions selects the revision to read.
type RoleLocatorOptions struct{ Revision string }

// RoleLocators reads the committed JS/TS sources at the revision and checks every notification
// role locator of a component test against the mounted components' markup.
func RoleLocators(repo Repo, options RoleLocatorOptions) (RoleLocatorReport, error) {
	commit, err := repo.Resolve(options.Revision)
	if err != nil {
		return RoleLocatorReport{}, err
	}
	texts, _, err := repo.readFiles(commit, func(name string) bool { return webExtensions[path.Ext(name)] })
	if err != nil {
		return RoleLocatorReport{}, err
	}
	report := CheckRoleLocators(texts)
	report.Revision = commit
	return report, nil
}

// componentMarkup is a component's literal roles, or why its markup cannot be closed.
type componentMarkup struct {
	roles map[string]bool
	open  string
}

// CheckRoleLocators applies TSA-V0-207..209 to committed sources keyed by path.
func CheckRoleLocators(texts map[string]string) RoleLocatorReport {
	report := RoleLocatorReport{Schema: RoleLocatorSchema, Findings: []RoleLocatorFinding{}, Abstentions: []RoleLocatorAbstention{}}
	names := make([]string, 0, len(texts))
	for name := range texts {
		if IsTestFile(name) {
			names = append(names, name)
		}
	}
	sort.Strings(names)
	for _, name := range names {
		text := texts[name]
		locators := roleLocators(text)
		if len(locators) == 0 {
			continue
		}
		scopes := testScopes(name, text)
		imports := componentImports(text)
		for _, loc := range locators {
			report.Checked++
			mounted := mountedComponents(text, scopes, loc.line)
			if len(mounted) == 0 {
				report.Abstentions = append(report.Abstentions, RoleLocatorAbstention{Path: name, Line: loc.line, Role: loc.role, Reason: roleComponentNotMounted})
				continue
			}
			paths, abstained := []string{}, false
			rendered := false
			for _, component := range mounted {
				resolved := resolveComponent(texts, name, imports[component])
				if resolved == "" {
					report.Abstentions = append(report.Abstentions, RoleLocatorAbstention{Path: name, Line: loc.line, Role: loc.role, Reason: roleComponentUnresolved, Component: component})
					abstained = true
					break
				}
				markup := readMarkup(texts[resolved])
				if markup.roles[loc.role] {
					rendered = true
					break
				}
				if markup.open != "" {
					report.Abstentions = append(report.Abstentions, RoleLocatorAbstention{Path: name, Line: loc.line, Role: loc.role, Reason: roleComponentMarkupOpen, Component: resolved, Detail: markup.open})
					abstained = true
					break
				}
				paths = append(paths, resolved)
			}
			if rendered || abstained {
				continue
			}
			report.Findings = append(report.Findings, RoleLocatorFinding{Rule: RuleRoleLocatorUnrendered, Path: name, Line: loc.line, Role: loc.role, Components: paths,
				Message: "locator by role " + loc.role + " cannot match: " + strings.Join(paths, ", ") + " renders no element with that role"})
		}
	}
	return report
}

type roleLocator struct {
	role string
	line int
}

func roleLocators(text string) []roleLocator {
	out := []roleLocator{}
	for _, m := range roleLocatorPattern.FindAllStringSubmatchIndex(text, -1) {
		role := ""
		for g := 1; g <= 3; g++ {
			if m[2*g] >= 0 {
				role = text[m[2*g]:m[2*g+1]]
			}
		}
		lineStart := strings.LastIndexByte(text[:m[0]], '\n') + 1
		if commentedOut(text[lineStart:m[0]]) {
			continue
		}
		if _, covered := notificationRoles[role]; covered {
			out = append(out, roleLocator{role: role, line: 1 + strings.Count(text[:m[0]], "\n")})
		}
	}
	return out
}

// commentedOut reports a match whose line prefix opens a line or block comment.
func commentedOut(before string) bool {
	trimmed := strings.TrimSpace(before)
	if strings.HasPrefix(trimmed, "*") || strings.HasPrefix(trimmed, "/*") {
		return true
	}
	i := strings.Index(before, "//")
	return i >= 0 && (i == 0 || before[i-1] != ':')
}

// opensElement reports a literal <element> opening tag, followed by space, slash or >.
func opensElement(text, element string) bool {
	for rest := text; ; {
		i := strings.Index(rest, "<"+element)
		if i < 0 {
			return false
		}
		rest = rest[i+1+len(element):]
		if rest != "" && strings.ContainsRune(" \t\r\n/>", rune(rest[0])) {
			return true
		}
	}
}

// testScopes are the line spans of a file's tests (TSA-V0-362).
func testScopes(name, text string) [][2]int {
	out := [][2]int{}
	for _, d := range parseSource(name, text).decls {
		if d.kind == "test" && d.endLine >= d.line {
			out = append(out, [2]int{d.line, d.endLine})
		}
	}
	return out
}

// innermostTest is the span of the innermost test containing the line, or false outside every test.
func innermostTest(scopes [][2]int, line int) ([2]int, bool) {
	best, found := [2]int{}, false
	for _, s := range scopes {
		if s[0] <= line && line <= s[1] && (!found || s[1]-s[0] < best[1]-best[0]) {
			best, found = s, true
		}
	}
	return best, found
}

// mountedComponents are the components mounted where a locator on the line runs (TSA-V0-362): a
// mount inside a test counts only for locators in that same test; a mount outside every test (a
// hook or a shared helper) counts for every locator of the file. A locator outside every test sees
// every mount of the file.
func mountedComponents(text string, scopes [][2]int, line int) []string {
	own, inTest := innermostTest(scopes, line)
	seen, out := map[string]bool{}, []string{}
	for _, m := range mountPattern.FindAllStringSubmatchIndex(text, -1) {
		at := 1 + strings.Count(text[:m[0]], "\n")
		if span, mountInTest := innermostTest(scopes, at); inTest && mountInTest && span != own {
			continue
		}
		component := text[m[2]:m[3]]
		if !seen[component] {
			seen[component] = true
			out = append(out, component)
		}
	}
	return out
}

// componentImports maps an imported identifier to its module specifier.
func componentImports(text string) map[string]string {
	out := map[string]string{}
	for _, m := range defaultImport.FindAllStringSubmatch(text, -1) {
		out[m[1]] = m[2]
	}
	for _, m := range namedImport.FindAllStringSubmatch(text, -1) {
		for _, part := range strings.Split(m[1], ",") {
			fields := strings.Fields(part)
			switch {
			case len(fields) == 1:
				out[fields[0]] = m[2]
			case len(fields) == 3 && fields[1] == "as":
				out[fields[2]] = m[2]
			}
		}
	}
	return out
}

// resolveComponent resolves a relative module specifier to a committed source path, or "".
func resolveComponent(texts map[string]string, from, specifier string) string {
	if !strings.HasPrefix(specifier, "./") && !strings.HasPrefix(specifier, "../") {
		return ""
	}
	base := path.Join(path.Dir(from), specifier)
	candidates := []string{base}
	for _, ext := range []string{".tsx", ".jsx", ".ts", ".js"} {
		candidates = append(candidates, base+ext)
	}
	for _, ext := range []string{".tsx", ".jsx", ".ts", ".js"} {
		candidates = append(candidates, base+"/index"+ext)
	}
	for _, candidate := range candidates {
		if _, ok := texts[candidate]; ok && webExtensions[path.Ext(candidate)] {
			return candidate
		}
	}
	return ""
}

// readMarkup collects a component's literal roles and implicit-role elements, and names the first
// construct that could render a role the reader cannot see (TSA-V0-208).
func readMarkup(text string) componentMarkup {
	markup := componentMarkup{roles: map[string]bool{}}
	for _, m := range literalRoleAttr.FindAllStringSubmatch(text, -1) {
		markup.roles[m[1]+m[2]+m[3]] = true
	}
	for role, elements := range notificationRoles {
		for _, element := range elements {
			if opensElement(text, element) {
				markup.roles[role] = true
			}
		}
	}
	switch {
	case len(anyRoleAttr.FindAllString(text, -1)) > len(bracedRoleAttr.FindAllString(text, -1)):
		markup.open = roleMarkupDynamicRole
	case spreadAttributes.MatchString(text):
		markup.open = roleMarkupSpreadAttributes
	case strings.Contains(text, "dangerouslySetInnerHTML") || strings.Contains(text, "innerHTML"):
		markup.open = roleMarkupRawHTML
	case reexport.MatchString(text):
		markup.open = roleMarkupReexport
	case childrenSlot.MatchString(text):
		markup.open = roleMarkupChildren
	default:
		for _, m := range childComponent.FindAllStringSubmatch(text, -1) {
			if m[1] != "Fragment" && m[1] != "React.Fragment" {
				markup.open = roleMarkupChildComponent
				break
			}
		}
	}
	return markup
}
