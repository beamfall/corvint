package testsource

// Test environment contract (docs/specs/test-source-analysis-v0.md, TSA-V0-210..216). For a named
// environment profile, the committed shell bootstrap scripts are read for the variables and
// service endpoints a test process receives, how each value is derived, and which a test can rely
// on; the stand-in services the bootstrap launches are indexed with their control routes. The
// reading never runs a script: a value it cannot derive is UNKNOWN with a reason.

import (
	"path"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

// EnvContractSchema names the `corvint tests env` report (TSA-V0-210).
const EnvContractSchema = "corvint-tests-env/0"

// Derivation kinds (TSA-V0-211). The set is closed.
const (
	DerivationLiteral      = "LITERAL"
	DerivationVariable     = "VARIABLE"
	DerivationInterpolated = "INTERPOLATED"
	DerivationOffset       = "OFFSET"
	DerivationDefaulted    = "DEFAULTED"
	DerivationUnknown      = "UNKNOWN"
)

// Unknown reasons and source notes (TSA-V0-211, TSA-V0-216).
const (
	envCommandSubstitution = "command-substitution"
	envArithmetic          = "arithmetic-not-offset"
	envParameterExpansion  = "parameter-expansion"
	envNotAssigned         = "not-assigned-in-profile"
	envCycle               = "reference-cycle"
	envSeveralScripts      = "assigned-in-several-scripts"
	envProfileNotFound     = "profile-not-found"
	envComposeNotRead      = "compose-not-read"
	envDotenvNotRead       = "dotenv-not-read"
)

// Stand-in control route kinds and scopes (TSA-V0-213).
const (
	RouteFaultInjection = "FAULT_INJECTION"
	RouteReset          = "RESET"
	RouteRequestLog     = "REQUEST_LOG"
	RouteControl        = "CONTROL"
	RouteService        = "SERVICE"
	ScopeServiceWide    = "SERVICE_WIDE"
	scopeKeyedPrefix    = "KEYED:"
)

// EnvDerivation says how one value is derived from the bootstrap source.
type EnvDerivation struct {
	Kind       string   `json:"kind"`
	Expression string   `json:"expression"`
	References []string `json:"references,omitempty"`
	Offset     *int     `json:"offset,omitempty"`
	Reason     string   `json:"reason,omitempty"`
}

// EnvVariable is one variable a profile's bootstrap assigns. Value is nil when it cannot be
// resolved from the profile alone; UnknownReason says why. Reliable marks a variable passed to the
// test process, which a test may read instead of re-deriving it (TSA-V0-212).
type EnvVariable struct {
	Name          string        `json:"name"`
	Source        string        `json:"source"`
	Line          int           `json:"line"`
	Value         *string       `json:"value"`
	UnknownReason string        `json:"unknownReason,omitempty"`
	Derivation    EnvDerivation `json:"derivation"`
	PassedToTests bool          `json:"passedToTests"`
	Reliable      bool          `json:"reliable"`
	Endpoint      bool          `json:"endpoint"`
}

// StandInRoute is one route a stand-in service registers (TSA-V0-213). Teardown lists the
// stand-in's RESET routes for a control route; empty means none was found.
type StandInRoute struct {
	Method   string   `json:"method"`
	Path     string   `json:"path"`
	Source   string   `json:"source"`
	Line     int      `json:"line"`
	Kind     string   `json:"kind"`
	Payload  []string `json:"payload"`
	Query    []string `json:"query"`
	Scope    string   `json:"scope,omitempty"`
	Teardown []string `json:"teardown"`
}

// StandIn is one stand-in service the profile's bootstrap launches (TSA-V0-213).
type StandIn struct {
	Source            string         `json:"source"`
	LaunchedAt        string         `json:"launchedAt"`
	PortVariable      string         `json:"portVariable,omitempty"`
	URL               *string        `json:"url"`
	DeclaredVariables []string       `json:"declaredVariables"`
	Routes            []StandInRoute `json:"routes"`
}

// EnvSourceNote is one profile source the reader names but does not read.
type EnvSourceNote struct {
	Path   string `json:"path"`
	Reason string `json:"reason"`
}

// EnvWarning is one test-validity warning: a test derives a service address from a hard-coded
// offset or literal instead of a declared variable (TSA-V0-214).
type EnvWarning struct {
	Rule                 string   `json:"rule"`
	Path                 string   `json:"path"`
	Line                 int      `json:"line"`
	Text                 string   `json:"text"`
	ComputedValue        *string  `json:"computedValue"`
	DeclaredAlternatives []string `json:"declaredAlternatives"`
	Message              string   `json:"message"`
}

// EnvDependency is one test-side use of a profile value (TSA-V0-215). StaysValid is set only in
// a change: whether the use still yields the variable's value after the change; nil is unknown.
type EnvDependency struct {
	Path       string `json:"path"`
	Line       int    `json:"line"`
	Variable   string `json:"variable"`
	Kind       string `json:"kind"`
	StaysValid *bool  `json:"staysValid,omitempty"`
}

// EnvChange is one variable whose value or derivation differs between --base and the revision,
// with the test uses that depend on it (TSA-V0-215).
type EnvChange struct {
	Variable         string          `json:"variable"`
	Before           *string         `json:"before"`
	After            *string         `json:"after"`
	BeforeExpression string          `json:"beforeExpression"`
	AfterExpression  string          `json:"afterExpression"`
	Dependents       []EnvDependency `json:"dependents"`
}

// EnvContractReport is the deterministic environment-contract reading of one profile.
type EnvContractReport struct {
	Schema        string          `json:"schema"`
	Profile       string          `json:"profile"`
	Revision      string          `json:"revision"`
	Base          string          `json:"base,omitempty"`
	Reason        string          `json:"reason,omitempty"`
	Sources       []string        `json:"sources"`
	UnreadSources []EnvSourceNote `json:"unreadSources"`
	Variables     []EnvVariable   `json:"variables"`
	StandIns      []StandIn       `json:"standIns"`
	Warnings      []EnvWarning    `json:"warnings"`
	Dependencies  []EnvDependency `json:"dependencies"`
	Changes       []EnvChange     `json:"changes,omitempty"`
}

// EnvContractOptions selects the profile, the revision and an optional base for impact.
type EnvContractOptions struct{ Profile, Revision, Base string }

var profileNamePattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_.-]{0,63}$`)

// ValidProfileName reports a profile name the reader accepts.
func ValidProfileName(name string) bool { return profileNamePattern.MatchString(name) }

// EnvContract reads the profile at the revision and, with a base, the variables whose value or
// derivation changed and the test uses that depend on them.
func EnvContract(repo Repo, options EnvContractOptions) (EnvContractReport, error) {
	commit, err := repo.Resolve(options.Revision)
	if err != nil {
		return EnvContractReport{}, err
	}
	budget := newReadBudget()
	texts, _, err := repo.readFilesWithin(commit, envSourceFile, budget)
	if err != nil {
		return EnvContractReport{}, err
	}
	report := ReadEnvContract(texts, options.Profile)
	report.Revision = commit
	if options.Base == "" {
		return report, nil
	}
	base, err := repo.Resolve(options.Base)
	if err != nil {
		return EnvContractReport{}, err
	}
	baseTexts, _, err := repo.readFilesWithin(base, envSourceFile, budget)
	if err != nil {
		return EnvContractReport{}, err
	}
	report.Base = base
	report.Changes = EnvChanges(baseTexts, texts, options.Profile)
	return report, nil
}

func envSourceFile(name string) bool {
	return strings.HasSuffix(name, ".sh") || webExtensions[path.Ext(name)] || composeProfile(name) != "" || strings.HasPrefix(path.Base(name), ".env.")
}

// profileModel is one profile's resolved bootstrap reading.
type profileModel struct {
	report EnvContractReport
	byName map[string]*EnvVariable
}

// ReadEnvContract applies TSA-V0-210..214 to committed sources keyed by path.
func ReadEnvContract(texts map[string]string, profile string) EnvContractReport {
	model := readProfile(texts, profile)
	refs := testEnvRefs(texts, model)
	report := model.report
	for _, ref := range refs {
		if ref.warning != nil {
			report.Warnings = append(report.Warnings, *ref.warning)
		}
		for _, name := range ref.variables(model) {
			report.Dependencies = append(report.Dependencies, EnvDependency{Path: ref.path, Line: ref.line, Variable: name, Kind: ref.kind})
		}
	}
	return report
}

// bootstrapScripts are the committed shell scripts of the profile: a `.sh` file whose stem is the
// profile name or ends in it after a `-`, `_` or `.` separator (TSA-V0-210).
func bootstrapScripts(texts map[string]string, profile string) []string {
	out := []string{}
	for name := range texts {
		if !strings.HasSuffix(name, ".sh") {
			continue
		}
		stem := strings.TrimSuffix(path.Base(name), ".sh")
		if stem == profile || strings.HasSuffix(stem, "-"+profile) || strings.HasSuffix(stem, "_"+profile) || strings.HasSuffix(stem, "."+profile) {
			out = append(out, name)
		}
	}
	sort.Strings(out)
	return out
}

// composeProfile is the profile a compose file names (compose.NAME.yml, docker-compose.NAME.yaml).
func composeProfile(name string) string {
	base := path.Base(name)
	for _, prefix := range []string{"compose.", "docker-compose."} {
		for _, ext := range []string{".yml", ".yaml"} {
			if strings.HasPrefix(base, prefix) && strings.HasSuffix(base, ext) && len(base) > len(prefix)+len(ext) {
				return base[len(prefix) : len(base)-len(ext)]
			}
		}
	}
	return ""
}

type shellAssignment struct {
	name, raw, source string
	line              int
	exported          bool
	toTests           bool
}

type shellLaunch struct {
	source, file string
	line         int
	env          map[string]string
}

var (
	assignmentWord = regexp.MustCompile(`^([A-Za-z_][A-Za-z0-9_]*)=(.*)$`)
	nameWord       = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)
	offsetExpr     = regexp.MustCompile(`^\$\(\(\s*\$?\{?([A-Za-z_][A-Za-z0-9_]*)\}?\s*([+-])\s*([0-9]+)\s*\)\)$`)
	offsetExprLeft = regexp.MustCompile(`^\$\(\(\s*([0-9]+)\s*\+\s*\$?\{?([A-Za-z_][A-Za-z0-9_]*)\}?\s*\)\)$`)
	defaultedExpr  = regexp.MustCompile(`^\$\{([A-Za-z_][A-Za-z0-9_]*):?[-=]([^}$]*)\}$`)
	plainReference = regexp.MustCompile(`\$\{([A-Za-z_][A-Za-z0-9_]*)\}|\$([A-Za-z_][A-Za-z0-9_]*)`)
	testRunnerWord = regexp.MustCompile(`(?:^|\s)(?:playwright\s+test|cypress\s+run|jest|vitest|(?:npm|yarn|pnpm)\s+(?:run\s+)?test(?::[A-Za-z0-9:_-]+)?|(?:npm|yarn|pnpm)\s+run\s+e2e)(?:\s|$)`)
	standInSegment = map[string]bool{"stub": true, "stubs": true, "fake": true, "fakes": true, "mock": true, "mocks": true,
		"stand-in": true, "stand-ins": true, "standin": true, "standins": true, "doubles": true, "test-doubles": true}
)

func readProfile(texts map[string]string, profile string) profileModel {
	report := EnvContractReport{Schema: EnvContractSchema, Profile: profile, Sources: bootstrapScripts(texts, profile),
		UnreadSources: []EnvSourceNote{}, Variables: []EnvVariable{}, StandIns: []StandIn{}, Warnings: []EnvWarning{}, Dependencies: []EnvDependency{}}
	for name := range texts {
		switch {
		case composeProfile(name) == profile:
			report.UnreadSources = append(report.UnreadSources, EnvSourceNote{Path: name, Reason: envComposeNotRead})
		case path.Base(name) == ".env."+profile:
			report.UnreadSources = append(report.UnreadSources, EnvSourceNote{Path: name, Reason: envDotenvNotRead})
		}
	}
	sort.Slice(report.UnreadSources, func(i, j int) bool { return report.UnreadSources[i].Path < report.UnreadSources[j].Path })
	model := profileModel{report: report, byName: map[string]*EnvVariable{}}
	if len(report.Sources) == 0 {
		model.report.Reason = envProfileNotFound
		return model
	}
	assignments, launches := map[string]shellAssignment{}, []shellLaunch{}
	several := map[string]bool{}
	for _, script := range report.Sources {
		own, ownLaunches := parseBootstrap(script, texts[script], texts)
		for name, a := range own {
			if prior, ok := assignments[name]; ok && prior.source != a.source {
				several[name] = true
			}
			assignments[name] = a
		}
		launches = append(launches, ownLaunches...)
	}
	names := make([]string, 0, len(assignments))
	for name := range assignments {
		names = append(names, name)
	}
	sort.Strings(names)
	resolver := envResolver{assignments: assignments, several: several, memo: map[string]resolved{}}
	for _, name := range names {
		a := assignments[name]
		r := resolver.resolve(name, map[string]bool{})
		v := EnvVariable{Name: name, Source: a.source, Line: a.line, Value: r.value, UnknownReason: r.reason,
			Derivation: derive(a.raw), PassedToTests: a.exported || a.toTests}
		if several[name] {
			v.Derivation = EnvDerivation{Kind: DerivationUnknown, Expression: a.raw, Reason: envSeveralScripts}
		}
		v.Reliable = v.PassedToTests
		v.Endpoint = endpointName(name) || (r.value != nil && (strings.HasPrefix(*r.value, "http://") || strings.HasPrefix(*r.value, "https://")))
		model.report.Variables = append(model.report.Variables, v)
	}
	for i := range model.report.Variables {
		model.byName[model.report.Variables[i].Name] = &model.report.Variables[i]
	}
	for _, launch := range launches {
		model.report.StandIns = append(model.report.StandIns, standIn(texts, launch, resolver, model))
	}
	return model
}

func endpointName(name string) bool {
	for _, suffix := range []string{"_URL", "_PORT", "_HOST", "_ADDR", "_ADDRESS", "_ENDPOINT", "_ORIGIN"} {
		if strings.HasSuffix(name, suffix) {
			return true
		}
	}
	return false
}

// parseBootstrap reads one script's assignments, exports and launches. Lines are joined at a
// trailing backslash; a comment line, a function body line and any other command are ignored.
func parseBootstrap(script, text string, texts map[string]string) (map[string]shellAssignment, []shellLaunch) {
	assignments, exported, launches := map[string]shellAssignment{}, map[string]bool{}, []shellLaunch{}
	allExport := false
	lines := strings.Split(text, "\n")
	for i := 0; i < len(lines); i++ {
		number, line := i+1, strings.TrimSpace(lines[i])
		for strings.HasSuffix(line, "\\") && i+1 < len(lines) {
			i++
			line = strings.TrimSuffix(line, "\\") + " " + strings.TrimSpace(lines[i])
		}
		words := shellWords(line)
		if len(words) == 0 {
			continue
		}
		switch {
		case words[0] == "set" && len(words) > 1 && (words[1] == "-a" || words[1] == "-o" && len(words) > 2 && words[2] == "allexport"):
			allExport = true
			continue
		case words[0] == "set" && len(words) > 1 && (words[1] == "+a" || words[1] == "+o" && len(words) > 2 && words[2] == "allexport"):
			allExport = false
			continue
		case words[0] == "export" || words[0] == "declare" && len(words) > 1 && words[1] == "-x":
			start := 1
			if words[0] == "declare" {
				start = 2
			}
			for _, w := range words[start:] {
				if m := assignmentWord.FindStringSubmatch(w); m != nil {
					assignments[m[1]] = shellAssignment{name: m[1], raw: m[2], source: script, line: number, exported: true}
					exported[m[1]] = true
				} else if nameWord.MatchString(w) {
					exported[w] = true
				}
			}
			continue
		}
		prefix := 0
		for prefix < len(words) && assignmentWord.MatchString(words[prefix]) {
			prefix++
		}
		if prefix == len(words) {
			for _, w := range words {
				m := assignmentWord.FindStringSubmatch(w)
				assignments[m[1]] = shellAssignment{name: m[1], raw: m[2], source: script, line: number, exported: allExport}
			}
			continue
		}
		command := strings.Join(words[prefix:], " ")
		env := map[string]string{}
		for _, w := range words[:prefix] {
			m := assignmentWord.FindStringSubmatch(w)
			env[m[1]] = m[2]
		}
		if testRunnerWord.MatchString(" " + command + " ") {
			for _, w := range words[:prefix] {
				m := assignmentWord.FindStringSubmatch(w)
				assignments[m[1]] = shellAssignment{name: m[1], raw: m[2], source: script, line: number, toTests: true}
			}
			continue
		}
		for _, w := range words[prefix:] {
			file := strings.TrimPrefix(strings.Trim(w, `"'`), "./")
			if _, ok := texts[file]; ok && isStandInPath(file) {
				launches = append(launches, shellLaunch{source: script, file: file, line: number, env: env})
				break
			}
		}
	}
	for name, a := range assignments {
		if exported[name] {
			a.exported = true
			assignments[name] = a
		}
	}
	return assignments, launches
}

func isStandInPath(name string) bool {
	for _, segment := range strings.Split(path.Dir(name), "/") {
		if standInSegment[strings.ToLower(segment)] {
			return true
		}
	}
	base := strings.ToLower(path.Base(name))
	return strings.Contains(base, "stub") || strings.Contains(base, "fake") || strings.Contains(base, "mock")
}

// shellWords splits a line into words outside quotes, `$(...)` and `${...}`; an unquoted `#`
// starting a word begins a comment, and `;`, `&`, `|` end the first command.
func shellWords(line string) []string {
	words, current := []string{}, strings.Builder{}
	quote, depth, inWord := byte(0), 0, false
	flush := func() {
		if inWord {
			words = append(words, current.String())
		}
		current.Reset()
		inWord = false
	}
	for i := 0; i < len(line); i++ {
		c := line[i]
		switch {
		case quote != 0:
			current.WriteByte(c)
			if c == quote {
				quote = 0
			}
			continue
		case c == '"' || c == '\'':
			quote = c
		case c == '$' && i+1 < len(line) && (line[i+1] == '(' || line[i+1] == '{'):
			depth++
			current.WriteByte(c)
			current.WriteByte(line[i+1])
			i++
			inWord = true
			continue
		case depth > 0 && (c == '(' || c == '{'):
			depth++
		case depth > 0 && (c == ')' || c == '}'):
			depth--
		case depth == 0 && (c == ' ' || c == '\t'):
			flush()
			continue
		case depth == 0 && c == '#' && !inWord:
			flush()
			return words
		case depth == 0 && (c == ';' || c == '&' || c == '|'):
			flush()
			return words
		}
		current.WriteByte(c)
		inWord = true
	}
	flush()
	return words
}

// unquote removes one level of surrounding double quotes; a single-quoted value is literal.
func unquote(raw string) (string, bool) {
	if len(raw) >= 2 && raw[0] == '\'' && raw[len(raw)-1] == '\'' {
		return raw[1 : len(raw)-1], true
	}
	if len(raw) >= 2 && raw[0] == '"' && raw[len(raw)-1] == '"' {
		return raw[1 : len(raw)-1], false
	}
	return raw, false
}

// derive classifies one assigned value (TSA-V0-211).
func derive(raw string) EnvDerivation {
	value, literal := unquote(raw)
	d := EnvDerivation{Expression: raw}
	if literal || !strings.Contains(value, "$") && !strings.Contains(value, "`") {
		d.Kind = DerivationLiteral
		return d
	}
	if m := offsetExpr.FindStringSubmatch(value); m != nil {
		n, _ := strconv.Atoi(m[3])
		if m[2] == "-" {
			n = -n
		}
		d.Kind, d.References, d.Offset = DerivationOffset, []string{m[1]}, &n
		return d
	}
	if m := offsetExprLeft.FindStringSubmatch(value); m != nil {
		n, _ := strconv.Atoi(m[1])
		d.Kind, d.References, d.Offset = DerivationOffset, []string{m[2]}, &n
		return d
	}
	switch {
	case strings.Contains(value, "$(("):
		d.Kind, d.Reason = DerivationUnknown, envArithmetic
		return d
	case strings.Contains(value, "$(") || strings.Contains(value, "`"):
		d.Kind, d.Reason = DerivationUnknown, envCommandSubstitution
		return d
	}
	if m := defaultedExpr.FindStringSubmatch(value); m != nil {
		d.Kind, d.References = DerivationDefaulted, []string{m[1]}
		return d
	}
	if strings.Contains(plainReference.ReplaceAllString(value, ""), "${") {
		d.Kind, d.Reason = DerivationUnknown, envParameterExpansion
		return d
	}
	for _, m := range plainReference.FindAllStringSubmatch(value, -1) {
		d.References = append(d.References, m[1]+m[2])
	}
	d.Kind = DerivationInterpolated
	if len(d.References) == 1 && plainReference.FindString(value) == value {
		d.Kind = DerivationVariable
	}
	return d
}

type resolved struct {
	value  *string
	reason string
}

type envResolver struct {
	assignments map[string]shellAssignment
	several     map[string]bool
	memo        map[string]resolved
}

// resolve computes a variable's value from the profile alone (TSA-V0-211).
func (r envResolver) resolve(name string, visiting map[string]bool) resolved {
	if done, ok := r.memo[name]; ok {
		return done
	}
	a, ok := r.assignments[name]
	if !ok {
		return resolved{reason: envNotAssigned + ":" + name}
	}
	if r.several[name] {
		return resolved{reason: envSeveralScripts}
	}
	if visiting[name] {
		return resolved{reason: envCycle}
	}
	visiting[name] = true
	defer delete(visiting, name)
	out := r.evaluate(a.raw, visiting)
	r.memo[name] = out
	return out
}

// evaluate computes one raw value in the profile's context.
func (r envResolver) evaluate(raw string, visiting map[string]bool) resolved {
	d := derive(raw)
	value, _ := unquote(raw)
	switch d.Kind {
	case DerivationLiteral:
		return resolved{value: &value}
	case DerivationUnknown:
		return resolved{reason: d.Reason}
	case DerivationOffset:
		base := r.resolve(d.References[0], visiting)
		if base.value == nil {
			return base
		}
		n, err := strconv.Atoi(*base.value)
		if err != nil {
			return resolved{reason: envArithmetic}
		}
		out := strconv.Itoa(n + *d.Offset)
		return resolved{value: &out}
	case DerivationDefaulted:
		if _, assigned := r.assignments[d.References[0]]; assigned {
			return r.resolve(d.References[0], visiting)
		}
		m := defaultedExpr.FindStringSubmatch(value)
		return resolved{value: &m[2]}
	}
	var failed *resolved
	out := plainReference.ReplaceAllStringFunc(value, func(ref string) string {
		m := plainReference.FindStringSubmatch(ref)
		got := r.resolve(m[1]+m[2], visiting)
		if got.value == nil {
			if failed == nil {
				failed = &got
			}
			return ""
		}
		return *got.value
	})
	if failed != nil {
		return *failed
	}
	return resolved{value: &out}
}

var (
	routeRegistration = regexp.MustCompile(`\b(?:app|router|server|api|fastify|stub)\.(get|post|put|patch|delete|all)\(\s*['"\x60]([^'"\x60]+)['"\x60]`)
	bodyField         = regexp.MustCompile(`\b(?:req|request)\.body\.([A-Za-z_][A-Za-z0-9_]*)|\b(?:req|request)\.body\[['"]([^'"]+)['"]\]`)
	bodyDestructure   = regexp.MustCompile(`\{([^{}]*)\}\s*=\s*(?:req|request)\.body\b`)
	queryField        = regexp.MustCompile(`\b(?:req|request)\.query\.([A-Za-z_][A-Za-z0-9_]*)`)
	scopeKey          = regexp.MustCompile(`(?i)^(?:test|session|scenario|tenant|run)_?id$|^(?:scope|key)$`)
)

// standIn indexes one launched stand-in: its URL from the launch's port variable and its routes
// from the launched file and the other non-test JS/TS files of its directory (TSA-V0-213).
func standIn(texts map[string]string, launch shellLaunch, resolver envResolver, model profileModel) StandIn {
	s := StandIn{Source: launch.file, LaunchedAt: launch.source + ":" + strconv.Itoa(launch.line), DeclaredVariables: []string{}, Routes: []StandInRoute{}}
	keys := make([]string, 0, len(launch.env))
	for name := range launch.env {
		keys = append(keys, name)
	}
	sort.Strings(keys)
	for _, name := range keys {
		if name != "PORT" && !strings.HasSuffix(name, "_PORT") {
			continue
		}
		raw := launch.env[name]
		if d := derive(raw); d.Kind == DerivationVariable {
			s.PortVariable = d.References[0]
		}
		if port := resolver.evaluate(raw, map[string]bool{}); port.value != nil {
			url := "http://localhost:" + *port.value
			s.URL = &url
			for _, v := range model.report.Variables {
				if v.PassedToTests && v.Value != nil && (*v.Value == *port.value || strings.Contains(*v.Value, ":"+*port.value)) {
					s.DeclaredVariables = append(s.DeclaredVariables, v.Name)
				}
			}
		}
		break
	}
	dir := path.Dir(launch.file)
	files := []string{}
	for name := range texts {
		if path.Dir(name) == dir && webExtensions[path.Ext(name)] && !IsTestFile(name) {
			files = append(files, name)
		}
	}
	sort.Strings(files)
	for _, name := range files {
		s.Routes = append(s.Routes, routes(name, texts[name])...)
	}
	resets := []string{}
	for _, r := range s.Routes {
		if r.Kind == RouteReset {
			resets = append(resets, r.Method+" "+r.Path)
		}
	}
	for i := range s.Routes {
		if s.Routes[i].Kind != RouteReset && s.Routes[i].Kind != RouteService {
			s.Routes[i].Teardown = append([]string{}, resets...)
		}
	}
	return s
}

func routes(name, text string) []StandInRoute {
	out := []StandInRoute{}
	matches := routeRegistration.FindAllStringSubmatchIndex(text, -1)
	for i, m := range matches {
		end := len(text)
		if i+1 < len(matches) {
			end = matches[i+1][0]
		}
		body := text[m[0]:end]
		r := StandInRoute{Method: strings.ToUpper(text[m[2]:m[3]]), Path: text[m[4]:m[5]], Source: name,
			Line: 1 + strings.Count(text[:m[0]], "\n"), Payload: fieldNames(body, true), Query: fieldNames(body, false), Teardown: []string{}}
		r.Kind = routeKind(r.Path)
		if r.Kind != RouteService {
			r.Scope = ScopeServiceWide
			for _, key := range append(pathParams(r.Path), r.Payload...) {
				if scopeKey.MatchString(key) {
					r.Scope = scopeKeyedPrefix + key
					break
				}
			}
		}
		out = append(out, r)
	}
	return out
}

func fieldNames(body string, payload bool) []string {
	seen := map[string]bool{}
	if payload {
		for _, m := range bodyField.FindAllStringSubmatch(body, -1) {
			seen[m[1]+m[2]] = true
		}
		for _, m := range bodyDestructure.FindAllStringSubmatch(body, -1) {
			for _, part := range strings.Split(m[1], ",") {
				field := strings.TrimSpace(strings.SplitN(strings.SplitN(part, "=", 2)[0], ":", 2)[0])
				if nameWord.MatchString(field) {
					seen[field] = true
				}
			}
		}
	} else {
		for _, m := range queryField.FindAllStringSubmatch(body, -1) {
			seen[m[1]] = true
		}
	}
	out := make([]string, 0, len(seen))
	for field := range seen {
		out = append(out, field)
	}
	sort.Strings(out)
	return out
}

func pathParams(route string) []string {
	out := []string{}
	for _, segment := range strings.Split(route, "/") {
		if strings.HasPrefix(segment, ":") {
			out = append(out, segment[1:])
		}
	}
	return out
}

// routeKind classifies a route by its path tokens (TSA-V0-213).
func routeKind(route string) string {
	tokens := strings.FieldsFunc(strings.ToLower(route), func(r rune) bool { return r == '/' || r == '-' || r == '_' || r == '.' || r == ':' })
	has := func(words ...string) bool {
		for _, t := range tokens {
			for _, w := range words {
				if t == w {
					return true
				}
			}
		}
		return false
	}
	switch {
	case has("fault", "faults", "inject", "fail", "failure", "failures", "error", "errors", "delay", "latency", "chaos"):
		return RouteFaultInjection
	case has("reset", "clear", "teardown", "restore"):
		return RouteReset
	case has("requests", "calls", "log", "logs", "history", "received", "recorded"):
		return RouteRequestLog
	case has("control", "admin", "config", "scenario", "scenarios") || strings.Contains(route, "/_"):
		return RouteControl
	}
	return RouteService
}
