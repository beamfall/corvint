package postmergehost

import (
	"fmt"
	"path"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

// Finding is one audit violation. An empty finding list is a PASS for the
// declared workflow text only; it observes no host, runner or repository setting.
type Finding struct {
	Code   string `json:"code"`
	Where  string `json:"where"`
	Detail string `json:"detail"`
}

func (f Finding) String() string { return f.Code + " " + f.Where + ": " + f.Detail }

// Workflow roles declared by the top-level CORVINT_PM_WORKFLOW env value.
const (
	RoleSourceTrigger = "source-trigger"
	RolePipeline      = "pipeline"
	RoleReconcile     = "reconcile"
	roleMarker        = "CORVINT_PM_WORKFLOW"
	installScript     = "install-pinned.sh"
	maxTriggerMinutes = 10
)

var (
	pinnedAction   = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_.-]*/[A-Za-z0-9_.-]+(/[A-Za-z0-9_./-]+)?@[0-9a-f]{40}$`)
	expression     = regexp.MustCompile(`\$\{\{(.*?)\}\}`)
	secretRef      = regexp.MustCompile(`\bsecrets\s*\.\s*([A-Za-z_][A-Za-z0-9_]*)`)
	secretAny      = regexp.MustCompile(`\bsecrets\b`)
	githubToken    = regexp.MustCompile(`\bgithub\s*\.\s*token\b`)
	shellSeparator = regexp.MustCompile(`&&|\|\||[;|()]`)
	shellName      = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)
	// lineValidation finds a value checked by a line-oriented grep: it passes
	// when any one line matches, so a newline can smuggle a second value.
	lineValidation = regexp.MustCompile(`\b(printf|echo)\b[^\n|]*\|\s*e?grep\b|\be?grep\b[^\n]*<<<`)
	branchLiteral  = regexp.MustCompile(`^[A-Za-z0-9._/-]+$`)
	// runnerEnvFile finds a script that touches the runner's env or path
	// file, which sets variables (such as BASH_ENV) for every later step.
	runnerEnvFile = regexp.MustCompile(`\bGITHUB_(ENV|PATH)\b`)
	// unsecureCommand finds both forms of the deprecated workflow commands that set
	// an environment variable or PATH entry for later steps when a runner
	// has ACTIONS_ALLOW_UNSECURE_COMMANDS set outside the workflow text.
	// The runner matches command names without regard to case.
	unsecureCommand = regexp.MustCompile(`(?i)(::|##\[)(set-env|add-path)\b`)
	inputName       = regexp.MustCompile(`^[a-z0-9_-]+$`)
	// startupEnv names variables that make a shell, the dynamic loader or an
	// interpreter run code the audit does not see, or that make the runner
	// re-enable the ::set-env and ::add-path commands or choose the Node.js
	// runtime that loads action code. It is a denylist.
	startupEnv = map[string]bool{
		"BASH_ENV": true, "ENV": true, "BASHOPTS": true, "SHELLOPTS": true, "PS4": true,
		"PROMPT_COMMAND": true, "IFS": true, "CDPATH": true, "PATH": true, "HOME": true,
		"CC": true, "GOFLAGS": true, "GOTOOLCHAIN": true, "JAVA_TOOL_OPTIONS": true,
		"NODE_OPTIONS": true, "PYTHONSTARTUP": true, "PYTHONPATH": true, "PERL5OPT": true,
		"PERL5LIB": true, "RUBYOPT": true,
		"ACTIONS_ALLOW_UNSECURE_COMMANDS": true, "ACTIONS_ALLOW_USE_UNSECURE_NODE_VERSION": true,
	}
	startupEnvPrefixes = []string{"LD_", "DYLD_", "BASH_FUNC_", "GIT_CONFIG", "FORCE_JAVASCRIPT_ACTIONS_TO_NODE"}
	changeRequest      = map[string]bool{
		"pull_request": true, "pull_request_target": true, "pull_request_review": true,
		"pull_request_review_comment": true, "merge_group": true, "workflow_run": true,
	}
	allowedTriggers = map[string]map[string]bool{
		RoleSourceTrigger: {"push": true, "workflow_dispatch": true},
		RolePipeline:      {"workflow_dispatch": true, "repository_dispatch": true},
		RoleReconcile:     {"schedule": true, "workflow_dispatch": true},
	}
	topLevelKeys = map[string]bool{"name": true, "run-name": true, "on": true, "permissions": true, "concurrency": true, "env": true, "jobs": true, "defaults": true}
	stepKeys     = map[string]bool{"id": true, "name": true, "if": true, "uses": true, "with": true, "run": true, "env": true, "timeout-minutes": true, "continue-on-error": true, "working-directory": true}
	jobKeys      = map[string]bool{"name": true, "needs": true, "if": true, "runs-on": true, "timeout-minutes": true, "continue-on-error": true, "permissions": true, "env": true, "outputs": true, "steps": true, "concurrency": true, "strategy": true, "defaults": true}
	modes        = map[string]bool{"dry-run": true, "recording": true}
)

type auditor struct {
	g        *Graph
	findings []Finding
}

func (a *auditor) add(code, where, format string, args ...any) {
	a.findings = append(a.findings, Finding{Code: code, Where: where, Detail: fmt.Sprintf(format, args...)})
}

// Audit checks one workflow template against the graph (PCH-V0-007). It
// fails closed: unparseable or unmodelled constructs are findings.
func Audit(g *Graph, name string, data []byte) []Finding {
	a := &auditor{g: g}
	root, err := ParseYAML(data)
	if err != nil {
		a.add("UNSUPPORTED_YAML", name, "%v", err)
		return a.findings
	}
	if root.Kind != Mapping {
		a.add("UNSUPPORTED_YAML", name, "root is not a mapping")
		return a.findings
	}
	for _, k := range root.Keys {
		if !topLevelKeys[k] {
			a.add("UNMODELLED_KEY", name, "top-level key %q", k)
		}
	}
	env := root.Get("env")
	role := scalar(env.Get(roleMarker))
	if _, ok := allowedTriggers[role]; !ok {
		a.add("MISSING_WORKFLOW_ROLE", name, "env.%s must be source-trigger, pipeline or reconcile", roleMarker)
	}
	a.triggers(name, role, root.Get("on"))
	a.permissions(name, root.Get("permissions"), true)
	a.defaults(name, root.Get("defaults"))
	a.env(name, env)
	for _, s := range scalars(env) {
		if refs := a.credentialRefs(s); len(refs) > 0 {
			a.add("WORKFLOW_LEVEL_SECRET", name, "workflow env exposes %s to every job", strings.Join(refs, ","))
		}
	}
	if role == RolePipeline {
		a.concurrency(name, root.Get("concurrency"))
	}
	jobs := root.Get("jobs")
	if jobs == nil || jobs.Kind != Mapping || len(jobs.Keys) == 0 {
		a.add("NO_JOBS", name, "jobs mapping is required")
		return a.findings
	}
	stepsOf := map[string][]string{}
	for _, id := range jobs.Keys {
		stepsOf[id] = a.job(name+"/"+id, role, jobs.Map[id])
	}
	a.order(name, jobs, stepsOf)
	return a.findings
}

func scalar(n *Node) string {
	if n == nil || n.Kind != Scalar {
		return ""
	}
	return n.Text
}

func scalars(n *Node) []string {
	if n == nil {
		return nil
	}
	switch n.Kind {
	case Scalar:
		return []string{n.Text}
	case Mapping:
		var out []string
		for _, k := range n.Keys {
			out = append(out, scalars(n.Map[k])...)
		}
		return out
	}
	var out []string
	for _, item := range n.Items {
		out = append(out, scalars(item)...)
	}
	return out
}

func (a *auditor) triggers(where, role string, on *Node) {
	var events []string
	switch {
	case on == nil:
	case on.Kind == Scalar:
		events = []string{on.Text}
	case on.Kind == Sequence:
		events = scalars(on)
	default:
		events = on.Keys
	}
	if len(events) == 0 {
		a.add("NO_TRIGGER", where, "on: is required")
	}
	for _, e := range events {
		if changeRequest[e] {
			a.add("CHANGE_REQUEST_TRIGGER", where, "%q runs against an open or untrusted change request", e)
		} else if allowed := allowedTriggers[role]; allowed != nil && !allowed[e] {
			a.add("TRIGGER_NOT_ALLOWED", where, "%q is not allowed for role %s", e, role)
		}
	}
	has := func(e string) bool {
		for _, x := range events {
			if x == e {
				return true
			}
		}
		return false
	}
	switch role {
	case RoleSourceTrigger:
		if !has("push") {
			a.add("SOURCE_TRIGGER_NOT_POST_MERGE", where, "source trigger must run on push to the merged branch")
		} else {
			a.pushBranches(where, on.Get("push"))
		}
	case RolePipeline:
		inputs := on.Get("workflow_dispatch").Get("inputs")
		if scalar(inputs.Get("change").Get("required")) != "true" {
			a.add("NO_REPLAY_BY_CHANGE", where, "workflow_dispatch input change must be required")
		}
		a.modeInput(where, inputs.Get("mode"), true)
	case RoleReconcile:
		if !has("schedule") {
			a.add("NO_RECONCILIATION_SCHEDULE", where, "reconciliation must be scheduled")
		}
		a.modeInput(where, on.Get("workflow_dispatch").Get("inputs").Get("mode"), false)
	}
}

// pushBranches requires push to name only literal branches. The audit cannot
// see which branch is the repository default; the operator checks that.
func (a *auditor) pushBranches(where string, push *Node) {
	branches := push.Get("branches")
	ok := push != nil && push.Kind == Mapping && len(push.Keys) == 1 && branches != nil && branches.Kind == Sequence && len(branches.Items) > 0
	if ok {
		for _, b := range branches.Items {
			ok = ok && b.Kind == Scalar && branchLiteral.MatchString(b.Text)
		}
	}
	if !ok {
		a.add("SOURCE_TRIGGER_BRANCHES", where, "push must declare only branches: a non-empty list of literal branch names")
	}
}

// defaults admits only defaults.run.working-directory. A defaults.run.shell
// would run every step's script under an unaudited interpreter.
func (a *auditor) defaults(where string, d *Node) {
	if d == nil {
		return
	}
	if d.Kind != Mapping {
		a.add("UNMODELLED_KEY", where, "defaults must be a mapping")
		return
	}
	for _, k := range d.Keys {
		run := d.Map[k]
		if k != "run" || run == nil || run.Kind != Mapping {
			a.add("UNMODELLED_KEY", where, "defaults key %q", k)
			continue
		}
		for _, rk := range run.Keys {
			switch rk {
			case "working-directory":
				a.workingDirectory(where, run.Map[rk])
			case "shell":
				a.add("CUSTOM_SHELL", where, "defaults.run.shell %q (custom shells are not audited)", scalar(run.Map[rk]))
			default:
				a.add("UNMODELLED_KEY", where, "defaults.run key %q", rk)
			}
		}
	}
}

// workingDirectory admits only a literal scalar: an expression or a
// non-scalar would choose the script's directory at run time.
func (a *auditor) workingDirectory(where string, n *Node) {
	if n != nil && (n.Kind != Scalar || strings.Contains(n.Text, "${{")) {
		a.add("WORKING_DIRECTORY", where, "working-directory must be a literal path")
	}
}

// env refuses a non-mapping env, a merge key or non-scalar value (either
// could hide a variable from the name screen), and any startup or loader
// variable.
func (a *auditor) env(where string, env *Node) {
	if env == nil {
		return
	}
	if env.Kind != Mapping {
		a.add("UNMODELLED_KEY", where, "env must be a mapping")
		return
	}
	for _, k := range env.Keys {
		if k == "<<" {
			a.add("UNMODELLED_KEY", where, "env merge key <<")
		}
		if v := env.Map[k]; v != nil && v.Kind != Scalar {
			a.add("UNMODELLED_KEY", where, "env.%s must be a scalar", k)
		}
		// Windows runners read environment names without regard to case.
		name := strings.ToUpper(k)
		bad := startupEnv[name]
		for _, p := range startupEnvPrefixes {
			bad = bad || strings.HasPrefix(name, p)
		}
		if bad {
			a.add("STARTUP_ENV", where, "env.%s makes a shell, loader, interpreter or the runner run unaudited code", k)
		}
	}
}

func (a *auditor) modeInput(where string, mode *Node, required bool) {
	if mode == nil {
		if required {
			a.add("MODE_INPUT", where, "workflow_dispatch input mode is required")
		}
		return
	}
	options := scalars(mode.Get("options"))
	if scalar(mode.Get("default")) != "dry-run" || len(options) == 0 {
		a.add("MODE_INPUT", where, "mode must default to dry-run with explicit options")
	}
	for _, o := range options {
		if !modes[o] {
			a.add("MODE_INPUT", where, "mode option %q is not dry-run or recording", o)
		}
	}
}

func (a *auditor) concurrency(where string, c *Node) {
	group := scalar(c.Get("group"))
	if !strings.Contains(group, "inputs.change") || scalar(c.Get("cancel-in-progress")) != "false" {
		a.add("NO_CHANGE_CONCURRENCY", where, "pipeline concurrency must group by inputs.change without cancelling")
	}
}

// permissions returns the outward-write credential classes the token holds.
func (a *auditor) permissions(where string, p *Node, workflow bool) []string {
	if p == nil {
		a.add("MISSING_PERMISSIONS", where, "explicit permissions are required")
		return nil
	}
	if p.Kind == Scalar {
		if p.Text != "read-all" {
			a.add("WRITE_PERMISSION", where, "permissions %q", p.Text)
		}
		return nil
	}
	if p.Kind != Mapping {
		a.add("MISSING_PERMISSIONS", where, "permissions must be a mapping")
		return nil
	}
	held := map[string]bool{}
	for _, scope := range p.Keys {
		switch v := scalar(p.Map[scope]); v {
		case "read", "none":
		case "write":
			if workflow {
				a.add("WORKFLOW_WRITE_PERMISSION", where, "%s: write is inherited by every job", scope)
				continue
			}
			switch scope {
			case "actions":
				held["dispatch"] = true
			case "issues":
				held["tracker-write"] = true
			default:
				held["forge-write"] = true
			}
		default:
			a.add("WRITE_PERMISSION", where, "%s: %q", scope, v)
		}
	}
	return sortedKeys(held)
}

// credentialRefs lists secret and token references in one scalar.
func (a *auditor) credentialRefs(text string) []string {
	var refs []string
	for _, m := range expression.FindAllStringSubmatch(text, -1) {
		body := m[1]
		named := secretRef.FindAllStringSubmatch(body, -1)
		if len(secretAny.FindAllString(body, -1)) != len(named) {
			refs = append(refs, "secrets[dynamic]")
		}
		for _, s := range named {
			refs = append(refs, "secrets."+s[1])
		}
		if githubToken.MatchString(body) {
			refs = append(refs, "github.token")
		}
	}
	return refs
}

func (a *auditor) job(where, role string, j *Node) []string {
	if j == nil || j.Kind != Mapping {
		a.add("UNSUPPORTED_JOB", where, "job must be a mapping")
		return nil
	}
	for _, k := range j.Keys {
		if !jobKeys[k] {
			a.add("UNMODELLED_KEY", where, "job key %q (reusable workflows, containers and services are not audited)", k)
		}
	}
	marker := scalar(j.Get("env").Get(a.g.StepMarker))
	var steps []string
	allowed := map[string]bool{}
	commands := map[string]bool{}
	for _, id := range strings.Split(marker, ",") {
		id = strings.TrimSpace(id)
		s := a.g.Step(id)
		if s == nil {
			a.add("MISSING_STEP_MARKER", where, "env.%s %q names no graph step", a.g.StepMarker, marker)
			continue
		}
		steps = append(steps, id)
		for _, c := range s.Credentials {
			allowed[c] = true
		}
		for _, c := range s.Commands {
			commands[c] = true
		}
	}
	authoring := false
	for _, s := range steps {
		authoring = authoring || s == "authoring"
	}
	if authoring && len(steps) != 1 {
		a.add("AUTHORING_NOT_ISOLATED", where, "an authoring job may not perform %v", steps)
	}
	if role == RoleSourceTrigger {
		if marker != "trigger" {
			a.add("SOURCE_TRIGGER_STEP", where, "a source trigger job performs only the trigger step")
		}
		if scalar(j.Get("continue-on-error")) != "true" {
			a.add("SOURCE_TRIGGER_CAN_FAIL", where, "continue-on-error: true is required")
		}
		if m, err := strconv.Atoi(scalar(j.Get("timeout-minutes"))); err != nil || m < 1 || m > maxTriggerMinutes {
			a.add("SOURCE_TRIGGER_UNBOUNDED", where, "timeout-minutes must be 1..%d", maxTriggerMinutes)
		}
	}
	a.defaults(where, j.Get("defaults"))
	a.env(where, j.Get("env"))
	held := map[string]bool{}
	for _, c := range a.permissions(where, j.Get("permissions"), false) {
		held[c] = true
	}
	for _, s := range scalars(j) {
		for _, ref := range a.credentialRefs(s) {
			switch {
			case ref == "github.token" || ref == "secrets.GITHUB_TOKEN":
				if authoring {
					a.add("AUTHORING_WRITE_CREDENTIAL", where, "%s is reachable by the author", ref)
				}
			case ref == "secrets[dynamic]":
				a.add("UNCLASSIFIED_SECRET", where, "dynamic secrets access")
			default:
				class, ok := a.g.SecretClass(strings.TrimPrefix(ref, "secrets."))
				if !ok {
					a.add("UNCLASSIFIED_SECRET", where, "%s has no credential-class prefix", ref)
					continue
				}
				held[class] = true
			}
		}
	}
	for _, c := range sortedKeys(held) {
		switch {
		case authoring && a.g.Class(c).OutwardWrite:
			a.add("AUTHORING_WRITE_CREDENTIAL", where, "authoring environment holds %s", c)
		case !allowed[c]:
			a.add("CREDENTIAL_NOT_ALLOWED", where, "%s is not allowed for %v", c, steps)
		}
	}
	a.steps(where, j.Get("steps"), commands)
	return steps
}

func (a *auditor) steps(where string, steps *Node, commands map[string]bool) {
	if steps == nil || steps.Kind != Sequence || len(steps.Items) == 0 {
		a.add("NO_STEPS", where, "steps sequence is required")
		return
	}
	installed := false
	for i, s := range steps.Items {
		at := fmt.Sprintf("%s/steps[%d]", where, i)
		if s.Kind != Mapping {
			a.add("UNSUPPORTED_STEP", at, "step must be a mapping")
			continue
		}
		for _, k := range s.Keys {
			if !stepKeys[k] {
				a.add("UNMODELLED_KEY", at, "step key %q (custom shells are not audited)", k)
			}
		}
		// Action input names are case-insensitive: Script, and script spelt with a
		// dotless i (U+0131), both become INPUT_SCRIPT. The screen folds case and
		// admits only lowercase ASCII input names. A non-mapping with is unmodelled.
		var inputs []string
		with := s.Get("with")
		if with != nil && with.Kind != Mapping {
			a.add("UNMODELLED_KEY", at, "with must be a mapping")
		} else if with != nil {
			inputs = with.Keys
		}
		for _, k := range inputs {
			if !inputName.MatchString(k) {
				a.add("NON_LOWERCASE_INPUT", at, "with key %q must match [a-z0-9_-]", k)
			}
			if v := with.Map[k]; v != nil && v.Kind != Scalar {
				a.add("UNMODELLED_KEY", at, "with.%s must be a scalar", k)
			}
			if strings.EqualFold(k, "script") && strings.Contains(scalar(with.Map[k]), "${{") {
				a.add("RUN_EXPRESSION_INTERPOLATION", at, "pass expressions through env, never into with.script")
			}
		}
		a.env(at, s.Get("env"))
		a.workingDirectory(at, s.Get("working-directory"))
		if uses := scalar(s.Get("uses")); s.Get("uses") != nil {
			if !pinnedAction.MatchString(uses) {
				a.add("UNPINNED_ACTION", at, "%q must name owner/repo@<40-hex commit>", uses)
			}
			if isCheckout(uses) && scalar(s.Get("with").Get("persist-credentials")) != "false" {
				a.add("CHECKOUT_PERSISTS_CREDENTIALS", at, "persist-credentials: false is required")
			}
		}
		run := s.Get("run")
		if run == nil {
			continue
		}
		if run.Kind != Scalar {
			a.add("UNMODELLED_KEY", at, "run must be a scalar script")
			continue
		}
		if strings.Contains(run.Text, "${{") {
			a.add("RUN_EXPRESSION_INTERPOLATION", at, "pass expressions through env, never into the script text")
		}
		if runnerEnvFile.MatchString(run.Text) {
			a.add("RUNNER_ENV_FILE", at, "a script may not write $GITHUB_ENV or $GITHUB_PATH, which change later steps' environment")
		}
		if unsecureCommand.MatchString(run.Text) {
			a.add("RUNNER_ENV_FILE", at, "a script may not print set-env or add-path workflow commands, which change later steps' environment")
		}
		if lineValidation.MatchString(run.Text) {
			a.add("LINE_ORIENTED_VALIDATION", at, "validate a value with whole-string case and length checks, not grep")
		}
		for _, cmd := range shellCommands(run.Text) {
			base := path.Base(cmd[0])
			if base == installScript {
				installed = true
				continue
			}
			if !strings.HasPrefix(base, "corvint") {
				continue
			}
			if !installed {
				a.add("UNPINNED_BINARY", at, "%s runs before %s verified the pinned binaries", base, installScript)
			}
			if !documented(base, cmd[1:], commands) {
				a.add("UNDOCUMENTED_COMMAND", at, "%s is not a documented command of this job's steps", strings.Join(cmd, " "))
			}
		}
	}
}

// isCheckout reports whether uses names actions/checkout. Owner and
// repository names are case-insensitive, and a sub-path such as /. can still
// resolve to the repository's root action.
func isCheckout(uses string) bool {
	ref, _, _ := strings.Cut(uses, "@")
	parts := strings.SplitN(strings.ToLower(ref), "/", 3)
	return len(parts) >= 2 && parts[0] == "actions" && parts[1] == "checkout"
}

// documented accepts `bin sub` or `bin sub1 sub2` after leading global flags.
func documented(bin string, args []string, commands map[string]bool) bool {
	var words []string
	for i := 0; i < len(args) && len(words) < 2; i++ {
		if args[i] == "--root" {
			i++
			continue
		}
		if strings.HasPrefix(args[i], "-") {
			break
		}
		words = append(words, args[i])
	}
	for n := len(words); n >= 1; n-- {
		if commands[bin+" "+strings.Join(words[:n], " ")] {
			return true
		}
	}
	return false
}

var shellKeywords = map[string]bool{"then": true, "do": true, "else": true, "!": true, "exec": true, "time": true, "if": true, "while": true, "until": true, "command": true}

// shellCommands returns the simple commands of a POSIX script approximately:
// lines (with continuations joined) split on ; && || | and leading keywords
// and NAME=value assignments removed. It is a lint, not a shell parser.
func shellCommands(script string) [][]string {
	script = strings.ReplaceAll(script, "\\\n", " ")
	var out [][]string
	for _, line := range strings.Split(script, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		for _, seg := range shellSeparator.Split(line, -1) {
			words := strings.Fields(seg)
			for len(words) > 0 && (shellKeywords[words[0]] || isAssignment(words[0])) {
				words = words[1:]
			}
			if len(words) > 0 {
				for i := range words {
					words[i] = strings.Trim(words[i], `"'`)
				}
				out = append(out, words)
			}
		}
	}
	return out
}

func isAssignment(word string) bool {
	eq := strings.IndexByte(word, '=')
	return eq > 0 && shellName.MatchString(word[:eq])
}

func (a *auditor) order(where string, jobs *Node, stepsOf map[string][]string) {
	needs := map[string][]string{}
	for _, id := range jobs.Keys {
		n := jobs.Map[id].Get("needs")
		needs[id] = scalars(n)
		for _, dep := range needs[id] {
			if _, ok := stepsOf[dep]; !ok {
				a.add("UNKNOWN_NEED", where+"/"+id, "needs unknown job %q", dep)
			}
		}
	}
	provider := map[string][]string{}
	for _, id := range jobs.Keys {
		for _, s := range stepsOf[id] {
			provider[s] = append(provider[s], id)
		}
	}
	for _, id := range jobs.Keys {
		ancestors := map[string]bool{id: true}
		var walk func(string)
		walk = func(j string) {
			for _, dep := range needs[j] {
				if !ancestors[dep] {
					ancestors[dep] = true
					walk(dep)
				}
			}
		}
		walk(id)
		for _, s := range stepsOf[id] {
			for _, before := range a.g.Step(s).After {
				for _, p := range provider[before] {
					if !ancestors[p] {
						a.add("ORDER_VIOLATION", where+"/"+id, "%s must run after %s in job %s", s, before, p)
					}
				}
			}
		}
	}
}

func sortedKeys(m map[string]bool) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}
