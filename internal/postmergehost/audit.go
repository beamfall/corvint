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
	pinnedAction   = regexp.MustCompile(`^[A-Za-z0-9_.-]+/[A-Za-z0-9_.-]+(/[A-Za-z0-9_./-]+)?@[0-9a-f]{40}$`)
	expression     = regexp.MustCompile(`\$\{\{(.*?)\}\}`)
	secretRef      = regexp.MustCompile(`\bsecrets\s*\.\s*([A-Za-z_][A-Za-z0-9_]*)`)
	secretAny      = regexp.MustCompile(`\bsecrets\b`)
	githubToken    = regexp.MustCompile(`\bgithub\s*\.\s*token\b`)
	shellSeparator = regexp.MustCompile(`&&|\|\||[;|()]`)
	shellName      = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)
	changeRequest  = map[string]bool{
		"pull_request": true, "pull_request_target": true, "pull_request_review": true,
		"pull_request_review_comment": true, "merge_group": true, "workflow_run": true,
	}
	allowedTriggers = map[string]map[string]bool{
		RoleSourceTrigger: {"push": true, "workflow_dispatch": true},
		RolePipeline:      {"workflow_dispatch": true, "repository_dispatch": true},
		RoleReconcile:     {"schedule": true, "workflow_dispatch": true},
	}
	topLevelKeys = map[string]bool{"name": true, "run-name": true, "on": true, "permissions": true, "concurrency": true, "env": true, "jobs": true, "defaults": true}
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
		if uses := scalar(s.Get("uses")); s.Get("uses") != nil {
			if !pinnedAction.MatchString(uses) {
				a.add("UNPINNED_ACTION", at, "%q must name owner/repo@<40-hex commit>", uses)
			}
			if strings.HasPrefix(uses, "actions/checkout@") && scalar(s.Get("with").Get("persist-credentials")) != "false" {
				a.add("CHECKOUT_PERSISTS_CREDENTIALS", at, "persist-credentials: false is required")
			}
		}
		run := s.Get("run")
		if run == nil {
			continue
		}
		if strings.Contains(run.Text, "${{") {
			a.add("RUN_EXPRESSION_INTERPOLATION", at, "pass expressions through env, never into the script text")
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
