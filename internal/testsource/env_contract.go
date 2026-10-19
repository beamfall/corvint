package testsource

// Test environment contract (docs/specs/test-source-analysis-v0.md, TSA-V0-210..216). For a named
// environment profile, the committed shell bootstrap scripts are read for the variables and
// service endpoints a test process receives, how each value is derived, and which a test can rely
// on; the stand-in services the bootstrap launches are indexed with their control routes. The
// reading never runs a script: a value it cannot derive is UNKNOWN with a reason.

import (
	"path"
	"regexp"
	"slices"
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
	envInherited           = "inherited-from-caller"
	envValueTooLarge       = "value-too-large"
	envBranchDependent     = "branch-dependent"
	envWorkflowExpression  = "workflow-expression"
	envCallerValuesDiffer  = "caller-values-differ"
	envComposeUnparsed     = "compose-unparsed"
	envWorkflowUnparsed    = "workflow-unparsed"
	envNotCommitted        = "not-committed"
	envLanguageNotRead     = "stand-in-language-not-read"
	maxEnvValueBytes       = 4096
)

// Source kinds of a variable not written in a bootstrap script itself (TSA-V0-350..352). The set
// is closed; a script's own variable has none.
const (
	SourceDotenv             = "DOTENV"
	SourceComposeEnvironment = "COMPOSE_ENVIRONMENT"
	SourceComposeEnvFile     = "COMPOSE_ENV_FILE"
	SourceWorkflow           = "WORKFLOW"
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
	// Alternatives are the candidate values of a branch-dependent or caller-dependent variable.
	Alternatives []string `json:"alternatives,omitempty"`
}

// EnvVariable is one variable a profile's bootstrap assigns. Value is nil when it cannot be
// resolved from the profile alone; UnknownReason says why. Reliable marks a variable passed to the
// test process, which a test may read instead of re-deriving it (TSA-V0-212).
type EnvVariable struct {
	Name          string        `json:"name"`
	Source        string        `json:"source"`
	SourceKind    string        `json:"sourceKind,omitempty"`
	Service       string        `json:"service,omitempty"`
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
	// Abstentions name the stand-in's files in a language whose routes are not read (TSA-V0-359).
	Abstentions []EnvSourceNote `json:"abstentions,omitempty"`
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
	texts, err := readEnvTexts(repo, commit, options.Profile, budget)
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
	baseTexts, err := readEnvTexts(repo, base, options.Profile, budget)
	if err != nil {
		return EnvContractReport{}, err
	}
	report.Base = base
	report.Changes = EnvChanges(baseTexts, texts, options.Profile)
	return report, nil
}

// envSourceFile keeps the committed files the reader may need: shell scripts, JS/TS sources,
// compose and dotenv files, CI workflows and the other files of stand-in directories
// (TSA-V0-210, TSA-V0-350..359).
func envSourceFile(name string) bool {
	base, ext := path.Base(name), path.Ext(name)
	yaml := ext == ".yml" || ext == ".yaml"
	switch {
	case strings.HasSuffix(name, ".sh") || webExtensions[ext]:
	case yaml && (strings.HasPrefix(base, "compose") || strings.HasPrefix(base, "docker-compose")):
	case yaml && path.Dir(name) == ".github/workflows":
	case base == ".env" || strings.HasPrefix(base, ".env.") || strings.HasSuffix(base, ".env"):
	case isStandInPath(name) && (standInRouteFile(name) || standInOtherLanguage[ext]):
	default:
		return false
	}
	return true
}

// profileModel is one profile's resolved bootstrap reading.
type profileModel struct {
	report EnvContractReport
	byName map[string]*EnvVariable
	// missing are files the profile names (a compose file, an env_file) that the read left out.
	missing map[string]bool
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

// shellAssignment is one assignment. script is the bootstrap script that read it; source is the
// file it is written in (the script, a dotenv file it sources, or a CI workflow for a caller
// value); kind is that file's source kind, empty for the script. unknown, with alternatives, marks
// a value the reading cannot pin down (TSA-V0-354, TSA-V0-361).
type shellAssignment struct {
	name, raw, source, script, kind string
	line                            int
	exported                        bool
	toTests                         bool
	exportOnly                      bool
	unknown                         string
	alternatives                    []string
}

type shellLaunch struct {
	source, file string
	line         int
	dir          bool
	env          map[string]string
}

var (
	assignmentWord = regexp.MustCompile(`^([A-Za-z_][A-Za-z0-9_]*)=(.*)$`)
	nameWord       = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)
	offsetExpr     = regexp.MustCompile(`^\$\(\(\s*\$?\{?([A-Za-z_][A-Za-z0-9_]*)\}?\s*([+-])\s*([0-9]+)\s*\)\)$`)
	offsetExprLeft = regexp.MustCompile(`^\$\(\(\s*([0-9]+)\s*\+\s*\$?\{?([A-Za-z_][A-Za-z0-9_]*)\}?\s*\)\)$`)
	defaultedExpr  = regexp.MustCompile(`^\$\{([A-Za-z_][A-Za-z0-9_]*)(:?)[-=]([^}$]*)\}$`)
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
	model := profileModel{report: report, byName: map[string]*EnvVariable{}, missing: map[string]bool{}}
	if len(report.Sources) == 0 {
		model.report.Reason = envProfileNotFound
		sortNotes(model.report.UnreadSources)
		return model
	}
	assignments, launches, composes := map[string]shellAssignment{}, []shellLaunch{}, []composeLaunch{}
	several := map[string]bool{}
	for _, script := range report.Sources {
		p := newShellParser(script, texts, profile)
		p.read(script, texts[script], "", false, 0)
		own, ownLaunches := p.finish()
		for name, a := range own {
			prior, ok := assignments[name]
			switch {
			case !ok || prior.exportOnly && a.exportOnly:
			case a.exportOnly:
				prior.exported = true
				a = prior
			case prior.exportOnly:
				a.exported = true
			case prior.script != a.script:
				several[name] = true
			}
			assignments[name] = a
		}
		launches = append(launches, ownLaunches...)
		composes = append(composes, p.composes...)
		for name := range p.missing {
			model.missing[name] = true
		}
	}
	caller, notes := workflowCallers(texts, report.Sources)
	model.report.UnreadSources = append(model.report.UnreadSources, notes...)
	for name, c := range caller {
		if a, ok := assignments[name]; !ok || a.exportOnly {
			assignments[name] = c
		}
	}
	names := make([]string, 0, len(assignments))
	for name := range assignments {
		names = append(names, name)
	}
	sort.Strings(names)
	resolver := envResolver{assignments: assignments, caller: caller, several: several, memo: map[string]resolved{}}
	for _, name := range names {
		a := assignments[name]
		r := resolver.resolve(name, map[string]bool{})
		// A variable every calling step exports stays exported when the script assigns it again.
		c, called := caller[name]
		inherited := called && c.unknown != envCallerValuesDiffer
		v := EnvVariable{Name: name, Source: a.source, SourceKind: a.kind, Line: a.line, Value: r.value, UnknownReason: r.reason,
			Derivation: derive(a.raw), PassedToTests: a.exported || a.toTests || inherited}
		switch {
		case several[name]:
			v.Derivation = EnvDerivation{Kind: DerivationUnknown, Expression: a.raw, Reason: envSeveralScripts}
		case a.exportOnly:
			v.Derivation = EnvDerivation{Kind: DerivationUnknown, Reason: envInherited}
		case a.unknown != "":
			v.Derivation = EnvDerivation{Kind: DerivationUnknown, Expression: a.raw, Reason: a.unknown, Alternatives: a.alternatives}
		}
		v.Reliable = v.PassedToTests
		v.Endpoint = endpointName(name) || (r.value != nil && (strings.HasPrefix(*r.value, "http://") || strings.HasPrefix(*r.value, "https://")))
		model.report.Variables = append(model.report.Variables, v)
	}
	services, notes := composeVariables(texts, composes, profile, resolver, model.missing)
	model.report.UnreadSources = append(model.report.UnreadSources, notes...)
	model.report.Variables = append(model.report.Variables, services...)
	sort.SliceStable(model.report.Variables, func(i, j int) bool {
		a, b := model.report.Variables[i], model.report.Variables[j]
		return a.Name < b.Name || a.Name == b.Name && a.Service < b.Service
	})
	for i := range model.report.Variables {
		v := &model.report.Variables[i]
		if _, taken := model.byName[v.Name]; !taken && (v.Service == "" || v.PassedToTests) {
			model.byName[v.Name] = v
		}
	}
	sortNotes(model.report.UnreadSources)
	for _, launch := range launches {
		model.report.StandIns = append(model.report.StandIns, standIn(texts, launch, resolver, model))
	}
	return model
}

func sortNotes(notes []EnvSourceNote) {
	sort.Slice(notes, func(i, j int) bool {
		return notes[i].Path < notes[j].Path || notes[i].Path == notes[j].Path && notes[i].Reason < notes[j].Reason
	})
}

func endpointName(name string) bool {
	for _, suffix := range []string{"_URL", "_PORT", "_HOST", "_ADDR", "_ADDRESS", "_ENDPOINT", "_ORIGIN"} {
		if strings.HasSuffix(name, suffix) {
			return true
		}
	}
	return false
}

// shellParser reads one script's assignments, exports and launches, and the dotenv files it
// sources (TSA-V0-351). Lines are joined at a trailing backslash and split into commands at `;`,
// `&&`, `||` and `&`; a comment and any other command are ignored. An assignment inside an
// if/case/loop/function body or after `&&`/`||` is conditional and keeps every candidate value
// (TSA-V0-361).
type shellParser struct {
	script, profile string
	texts           map[string]string
	dirs            map[string]bool
	assignments     map[string]shellAssignment
	exported        map[string]shellAssignment
	launches        []shellLaunch
	composes        []composeLaunch
	missing         map[string]bool
	allExport       bool
}

// composeLaunch is one `docker compose` invocation of a compose file (TSA-V0-352).
type composeLaunch struct {
	script, file string
	line         int
}

func newShellParser(script string, texts map[string]string, profile string) *shellParser {
	return &shellParser{script: script, profile: profile, texts: texts, assignments: map[string]shellAssignment{},
		exported: map[string]shellAssignment{}, missing: map[string]bool{}}
}

// read reads one shell or dotenv text; kind is "" for the script itself and SourceDotenv for a
// sourced dotenv file, which inherits the caller's conditional state.
func (p *shellParser) read(source, text, kind string, inherited bool, depth int) {
	stack, pendingFunction := []string{}, false
	lines := strings.Split(text, "\n")
	for i := 0; i < len(lines); i++ {
		number, line := i+1, strings.TrimSpace(lines[i])
		// A trailing `\`, `&&` or `||` continues the command on the next line, so a list split
		// across lines keeps its conditional part (TSA-V0-361).
		for i+1 < len(lines) && (strings.HasSuffix(line, "\\") || strings.HasSuffix(line, "&&") || strings.HasSuffix(line, "||")) {
			i++
			line = strings.TrimSuffix(line, "\\") + " " + strings.TrimSpace(lines[i])
		}
		for _, segment := range shellSegments(line) {
			words := shellWords(segment.text)
			if len(words) == 0 {
				continue
			}
			top := ""
			if len(stack) > 0 {
				top = stack[len(stack)-1]
			}
			switch words[0] {
			case "if", "while", "until", "for", "select":
				stack = append(stack, "branch")
				continue
			case "case":
				stack = append(stack, "case")
				continue
			case "elif":
				continue
			case "fi", "done", "esac", "}":
				if len(stack) > 0 {
					stack = stack[:len(stack)-1]
				}
				continue
			case "then", "else", "do":
				words = words[1:]
			case "{":
				if !pendingFunction {
					stack = append(stack, "group")
				}
				pendingFunction = false
				words = words[1:]
			case "function":
				stack, pendingFunction = append(stack, "function"), !strings.HasSuffix(segment.text, "{")
				continue
			default:
				if strings.HasSuffix(words[0], "()") || len(words) > 1 && words[1] == "()" {
					stack, pendingFunction = append(stack, "function"), !strings.HasSuffix(segment.text, "{")
					continue
				}
				if top == "case" {
					if at := strings.Index(segment.text, ")"); at >= 0 && !strings.ContainsAny(segment.text[:at], "=$(") {
						words = shellWords(segment.text[at+1:])
					}
				}
			}
			if len(words) == 0 {
				continue
			}
			conditional := inherited || segment.conditional
			for _, kind := range stack {
				conditional = conditional || kind != "group"
			}
			p.command(source, kind, number, words, conditional, depth)
		}
	}
}

// command reads one simple command.
func (p *shellParser) command(source, kind string, number int, words []string, conditional bool, depth int) {
	set := func(name, raw string, exported, toTests bool) {
		p.assign(shellAssignment{name: name, raw: raw, source: source, script: p.script, kind: kind, line: number, exported: exported, toTests: toTests}, conditional)
	}
	switch {
	case words[0] == "set" && len(words) > 1 && (words[1] == "-a" || words[1] == "-o" && len(words) > 2 && words[2] == "allexport"):
		p.allExport = true
		return
	case words[0] == "set" && len(words) > 1 && (words[1] == "+a" || words[1] == "+o" && len(words) > 2 && words[2] == "allexport"):
		p.allExport = false
		return
	case words[0] == "export" || words[0] == "declare" && len(words) > 1 && words[1] == "-x":
		start := 1
		if words[0] == "declare" {
			start = 2
		}
		for _, w := range words[start:] {
			if m := assignmentWord.FindStringSubmatch(w); m != nil {
				set(m[1], m[2], true, false)
				p.exported[m[1]] = shellAssignment{source: source, line: number, kind: kind}
			} else if nameWord.MatchString(w) {
				p.exported[w] = shellAssignment{source: source, line: number, kind: kind}
			}
		}
		return
	case (words[0] == "." || words[0] == "source") && len(words) > 1:
		file := p.scriptPath(source, words[1])
		if text, ok := p.texts[file]; ok && dotenvFile(file, p.profile) && depth < 2 {
			p.read(file, text, SourceDotenv, conditional, depth+1)
		}
		return
	}
	prefix := 0
	for prefix < len(words) && assignmentWord.MatchString(words[prefix]) {
		prefix++
	}
	if prefix == len(words) {
		for _, w := range words {
			m := assignmentWord.FindStringSubmatch(w)
			set(m[1], m[2], p.allExport, false)
		}
		return
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
			set(m[1], m[2], false, true)
		}
		return
	}
	if files := p.composeFiles(source, words[prefix:]); files != nil {
		for _, file := range files {
			p.composes = append(p.composes, composeLaunch{script: source, file: file, line: number})
		}
		return
	}
	for i, w := range words[prefix:] {
		if launch, ok := p.launch(source, w, words[prefix+i+1:]); ok {
			launch.line, launch.env = number, env
			p.launches = append(p.launches, launch)
			return
		}
	}
}

// assign records one assignment; a conditional one keeps the prior value as an alternative and
// makes the variable branch-dependent until an unconditional assignment (TSA-V0-361).
func (p *shellParser) assign(a shellAssignment, conditional bool) {
	if conditional {
		alternatives := []string{}
		if prior, ok := p.assignments[a.name]; ok && !prior.exportOnly {
			alternatives = append(alternatives, prior.alternatives...)
			if len(prior.alternatives) == 0 {
				alternatives = append(alternatives, prior.raw)
			}
		}
		if !slices.Contains(alternatives, a.raw) {
			alternatives = append(alternatives, a.raw)
		}
		a.alternatives, a.unknown = alternatives, envBranchDependent
	}
	p.assignments[a.name] = a
}

func (p *shellParser) finish() (map[string]shellAssignment, []shellLaunch) {
	for name, at := range p.exported {
		a, ok := p.assignments[name]
		if !ok {
			a = shellAssignment{name: name, source: at.source, script: p.script, kind: at.kind, line: at.line, exportOnly: true}
		}
		a.exported = true
		p.assignments[name] = a
	}
	return p.assignments, p.launches
}

// scriptPath resolves a path word of a script relative to the repository root, the directory the
// profile is run from; `$(dirname "$0")/` names the script's directory.
func (p *shellParser) scriptPath(source, word string) string {
	word = strings.ReplaceAll(strings.ReplaceAll(word, `"`, ""), `'`, "")
	for _, dir := range []string{"$(dirname $0)/", "$(dirname ${BASH_SOURCE[0]})/", "${BASH_SOURCE%/*}/"} {
		if strings.HasPrefix(word, dir) {
			return path.Clean(path.Join(path.Dir(source), strings.TrimPrefix(word, dir)))
		}
	}
	return path.Clean(strings.TrimPrefix(word, "./"))
}

func (p *shellParser) dir(name string) bool {
	if p.dirs == nil {
		p.dirs = map[string]bool{}
		for file := range p.texts {
			for d := path.Dir(file); d != "." && d != "/" && !p.dirs[d]; d = path.Dir(d) {
				p.dirs[d] = true
			}
		}
	}
	return p.dirs[name]
}

// composeFiles are the compose files a `docker compose` or `docker-compose` command reads: each
// `-f`/`--file` argument, or the first default file committed at the repository root. A named file
// that is not read is recorded as missing for a further read. Nil means the words are no compose
// command (TSA-V0-352).
func (p *shellParser) composeFiles(source string, words []string) []string {
	at := -1
	for i, w := range words {
		if w == "docker-compose" || w == "docker" && i+1 < len(words) && words[i+1] == "compose" {
			at = i
			break
		}
	}
	if at < 0 {
		return nil
	}
	files := []string{}
	for i := at + 1; i < len(words); i++ {
		w, value := words[i], ""
		switch {
		case (w == "-f" || w == "--file") && i+1 < len(words):
			value, i = words[i+1], i+1
		case strings.HasPrefix(w, "--file="):
			value = strings.TrimPrefix(w, "--file=")
		case strings.HasPrefix(w, "-f") && len(w) > 2:
			value = strings.TrimPrefix(strings.TrimPrefix(w, "-f"), "=")
		default:
			continue
		}
		file := p.scriptPath(source, value)
		if _, ok := p.texts[file]; !ok {
			p.missing[file] = true
		}
		files = append(files, file)
	}
	if len(files) == 0 {
		for _, name := range []string{"compose.yaml", "compose.yml", "docker-compose.yaml", "docker-compose.yml"} {
			if _, ok := p.texts[name]; ok {
				return []string{name}
			}
		}
	}
	return files
}

// launch reads one command word as a stand-in launch: a committed stand-in file, a committed
// stand-in directory (`go run ./stubs/x`, WireMock's `--root-dir`), or a Flask module named
// without its `.py` (TSA-V0-213, TSA-V0-355).
func (p *shellParser) launch(source, word string, rest []string) (shellLaunch, bool) {
	if word == "--root-dir" && len(rest) > 0 {
		word = rest[0]
	}
	word = strings.TrimPrefix(word, "--root-dir=")
	file := strings.TrimPrefix(strings.Trim(word, `"'`), "./")
	if file == "" || strings.HasPrefix(file, "-") {
		return shellLaunch{}, false
	}
	file = path.Clean(file)
	if _, ok := p.texts[file]; ok && isStandInPath(file) {
		return shellLaunch{source: source, file: file}, true
	}
	if _, ok := p.texts[file+".py"]; ok && isStandInPath(file+".py") {
		return shellLaunch{source: source, file: file + ".py"}, true
	}
	if p.dir(file) && isStandInPath(file+"/") {
		return shellLaunch{source: source, file: file, dir: true}, true
	}
	return shellLaunch{}, false
}

// dotenvFile is a dotenv name a script may source: `.env`, `.env.SUFFIX` other than the profile's
// own `.env.NAME` (TSA-V0-210, TSA-V0-353) and `NAME.env`.
func dotenvFile(name, profile string) bool {
	base := path.Base(name)
	return base == ".env" || strings.HasPrefix(base, ".env.") && base != ".env."+profile || strings.HasSuffix(base, ".env") && len(base) > len(".env")
}

type shellSegment struct {
	text        string
	conditional bool
}

// shellSegments splits a line into commands at `;`, `;;`, `&&`, `||` and a background `&`
// outside quotes, `$(...)` and `${...}`; a command after `&&` or `||` is conditional until the
// next `;` or `&`. An unquoted `#` starting a word begins a comment.
func shellSegments(line string) []shellSegment {
	out, start, conditional := []shellSegment{}, 0, false
	quote, depth := byte(0), 0
	emit := func(end int) {
		if text := strings.TrimSpace(line[start:end]); text != "" {
			out = append(out, shellSegment{text: text, conditional: conditional})
		}
	}
	for i := 0; i < len(line); i++ {
		c := line[i]
		switch {
		case quote != 0:
			if c == quote {
				quote = 0
			}
		case c == '"' || c == '\'' || c == '`':
			quote = c
		case c == '$' && i+1 < len(line) && (line[i+1] == '(' || line[i+1] == '{'):
			depth++
			i++
		case depth > 0 && (c == '(' || c == '{'):
			depth++
		case depth > 0 && (c == ')' || c == '}'):
			depth--
		case depth > 0:
		case c == '#' && (i == 0 || line[i-1] == ' ' || line[i-1] == '\t'):
			emit(i)
			return out
		case c == ';':
			emit(i)
			if i+1 < len(line) && line[i+1] == ';' {
				i++
			}
			start, conditional = i+1, false
		case (c == '&' || c == '|') && i+1 < len(line) && line[i+1] == c:
			emit(i)
			i++
			start, conditional = i+1, true
		case c == '&' && (i == 0 || line[i-1] != '>') && (i+1 >= len(line) || line[i+1] != '>'):
			emit(i)
			start, conditional = i+1, false
		}
	}
	emit(len(line))
	return out
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

// callerMark prefixes a visiting key that records a caller value already being evaluated.
const callerMark = "\x00caller:"

type resolved struct {
	value  *string
	reason string
}

type envResolver struct {
	assignments map[string]shellAssignment
	caller      map[string]shellAssignment
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
	if a.unknown != "" {
		return resolved{reason: a.unknown}
	}
	if a.exportOnly {
		return resolved{reason: envNotAssigned + ":" + name}
	}
	if visiting[name] {
		// A script value naming itself (`PORT=${PORT:-3000}`) reads the caller's value (TSA-V0-354).
		// The caller's value is evaluated once: a caller value naming the variable again is a cycle.
		if c, ok := r.caller[name]; ok && a.kind != SourceWorkflow && !visiting[callerMark+name] {
			if c.unknown != "" {
				return resolved{reason: c.unknown}
			}
			return r.evaluate(c.raw, map[string]bool{callerMark + name: true})
		}
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
		m := defaultedExpr.FindStringSubmatch(value)
		if a, assigned := r.assignments[d.References[0]]; assigned && !a.exportOnly {
			got := r.resolve(d.References[0], visiting)
			if got.value == nil || *got.value != "" || m[2] != ":" {
				return got
			}
		}
		return resolved{value: &m[3]}
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
	if len(out) > maxEnvValueBytes {
		return resolved{reason: envValueTooLarge}
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
	files := []string{}
	for name := range texts {
		if launch.dir && strings.HasPrefix(name, launch.file+"/") || !launch.dir && path.Dir(name) == path.Dir(launch.file) {
			files = append(files, name)
		}
	}
	sort.Strings(files)
	for _, name := range files {
		switch {
		case standInRouteFile(name):
			s.Routes = append(s.Routes, routes(name, texts[name], texts)...)
		case standInOtherLanguage[path.Ext(name)]:
			s.Abstentions = append(s.Abstentions, EnvSourceNote{Path: name, Reason: envLanguageNotRead})
		}
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

// pathParams are a route's `:name`, `{name}` and `<name>` parameters.
func pathParams(route string) []string {
	out := []string{}
	for _, segment := range strings.Split(route, "/") {
		if strings.HasPrefix(segment, ":") {
			out = append(out, segment[1:])
		}
	}
	for _, m := range bracePathParam.FindAllStringSubmatch(route, -1) {
		out = append(out, m[1]+m[2])
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
