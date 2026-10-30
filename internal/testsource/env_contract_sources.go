package testsource

// Environment sources beyond the bootstrap scripts (docs/specs/test-source-analysis-v0.md,
// TSA-V0-350..354): the CI workflow `env` blocks of the steps that run a profile script, and the
// `environment` and `env_file` entries of the compose files a profile script launches. Dotenv files
// a script sources are read by the shell parser itself (TSA-V0-351).

import (
	"path"
	"slices"
	"sort"
	"strconv"
	"strings"
)

// readEnvTexts reads the committed environment sources at commit, then up to two further rounds
// of the files the profile names that the first read left out: a compose file given with `-f` or
// an `env_file` under any name (TSA-V0-352).
func readEnvTexts(repo Repo, commit, profile string, budget *readBudget) (map[string]string, error) {
	texts, _, err := repo.readFilesWithin(commit, envSourceFile, budget)
	if err != nil {
		return nil, err
	}
	tried := map[string]bool{}
	for round := 0; round < 2; round++ {
		want := map[string]bool{}
		for name := range readProfile(texts, profile).missing {
			if !tried[name] {
				want[name], tried[name] = true, true
			}
		}
		if len(want) == 0 {
			break
		}
		more, _, err := repo.readFilesWithin(commit, func(name string) bool { return want[name] }, budget)
		if err != nil {
			return nil, err
		}
		for name, text := range more {
			texts[name] = text
		}
	}
	return texts, nil
}

// workflowEntry is one `env` entry of a workflow, job or step.
type workflowEntry struct {
	raw  string
	line int
}

// workflowCallers reads the caller environment of the profile scripts from the committed
// `.github/workflows/*.yml` files: for each step whose `run` names a profile script, the
// workflow's, the job's and the step's `env` in that precedence. A variable the invoking steps set
// to different values, or leave unset in some, is caller-dependent; a `${{ }}` expression is
// unknown (TSA-V0-350, TSA-V0-354). A workflow that names a profile script but does not parse is
// noted, not guessed.
func workflowCallers(texts map[string]string, scripts []string) (map[string]shellAssignment, []EnvSourceNote) {
	files := []string{}
	for name := range texts {
		if path.Dir(name) == ".github/workflows" && (path.Ext(name) == ".yml" || path.Ext(name) == ".yaml") {
			files = append(files, name)
		}
	}
	sort.Strings(files)
	type use struct {
		file  string
		entry workflowEntry
	}
	values, steps := map[string][]use{}, 0
	notes := []EnvSourceNote{}
	for _, file := range files {
		if !namesScript(texts[file], scripts) {
			continue
		}
		doc, ok := readYAML(texts[file])
		if !ok || doc.kind != yamlMap {
			notes = append(notes, EnvSourceNote{Path: file, Reason: envWorkflowUnparsed})
			continue
		}
		unparsed := false
		workflowEnv := yamlEnv(doc.get("env"), &unparsed)
		fileValues, fileSteps := map[string][]use{}, 0
		jobs := doc.get("jobs")
		if jobs == nil || jobs.kind != yamlMap {
			continue
		}
		for _, job := range jobs.keys {
			jobNode := jobs.m[job]
			if jobNode == nil || jobNode.kind != yamlMap {
				continue
			}
			jobEnv := yamlEnv(jobNode.get("env"), &unparsed)
			list := jobNode.get("steps")
			if list == nil || list.kind != yamlList {
				continue
			}
			for _, step := range list.items {
				run := step.get("run")
				if step.kind != yamlMap || run == nil || run.kind != yamlScalar || !namesScript(run.text, scripts) {
					continue
				}
				fileSteps++
				merged := map[string]workflowEntry{}
				for _, env := range []map[string]workflowEntry{workflowEnv, jobEnv, yamlEnv(step.get("env"), &unparsed)} {
					for name, entry := range env {
						merged[name] = entry
					}
				}
				for name, entry := range merged {
					fileValues[name] = append(fileValues[name], use{file: file, entry: entry})
				}
			}
		}
		if unparsed {
			notes = append(notes, EnvSourceNote{Path: file, Reason: envWorkflowUnparsed})
			continue
		}
		steps += fileSteps
		for name, uses := range fileValues {
			values[name] = append(values[name], uses...)
		}
	}
	out := map[string]shellAssignment{}
	for name, uses := range values {
		first := uses[0]
		a := shellAssignment{name: name, raw: first.entry.raw, source: first.file, kind: SourceWorkflow, line: first.entry.line, exported: true}
		distinct := []string{}
		for _, u := range uses {
			if !slices.Contains(distinct, u.entry.raw) {
				distinct = append(distinct, u.entry.raw)
			}
		}
		switch {
		case len(distinct) > 1 || len(uses) < steps:
			a.unknown, a.alternatives = envCallerValuesDiffer, distinct
		case strings.Contains(a.raw, "${{"):
			a.unknown = envWorkflowExpression
		}
		out[name] = a
	}
	return out, notes
}

func namesScript(text string, scripts []string) bool {
	for _, script := range scripts {
		if strings.Contains(text, script) {
			return true
		}
	}
	return false
}

// yamlEnv reads a workflow `env` map. A workflow value is not shell-expanded, so a value with a
// `$` other than a `${{ }}` expression is kept literal.
func yamlEnv(node *yamlNode, unparsed *bool) map[string]workflowEntry {
	out := map[string]workflowEntry{}
	if node == nil || node.kind != yamlMap {
		return out
	}
	for _, name := range node.keys {
		value := node.m[name]
		if !nameWord.MatchString(name) || value == nil || value.kind != yamlScalar {
			continue
		}
		if value.block {
			*unparsed = true // a block scalar's exact value is not read, so it is never guessed
		}
		raw := value.text
		if strings.Contains(raw, "$") && !strings.Contains(raw, "${{") {
			raw = "'" + raw + "'"
		}
		out[name] = workflowEntry{raw: raw, line: value.line}
	}
	return out
}

// composeVariables reads the `environment` and `env_file` entries of each service of the compose
// files the profile launches. The profile's own compose.NAME file stays unread (TSA-V0-210,
// TSA-V0-353). A service's variables reach the tests only when the service runs the test runner
// (TSA-V0-352).
func composeVariables(texts map[string]string, launches []composeLaunch, profile string, resolver envResolver, missing map[string]bool) ([]EnvVariable, []EnvSourceNote) {
	out, notes, seen := []EnvVariable{}, []EnvSourceNote{}, map[string]bool{}
	for _, launch := range launches {
		file := launch.file
		if seen[file] || composeProfile(file) == profile {
			continue
		}
		seen[file] = true
		text, ok := texts[file]
		if !ok {
			notes = append(notes, EnvSourceNote{Path: file, Reason: envNotCommitted})
			continue
		}
		doc, ok := readYAML(text)
		if !ok || doc.kind != yamlMap {
			notes = append(notes, EnvSourceNote{Path: file, Reason: envComposeUnparsed})
			continue
		}
		services := doc.get("services")
		if services == nil || services.kind != yamlMap {
			continue
		}
		fileOut, blockValue := []EnvVariable{}, false
		for _, service := range services.keys {
			node := services.m[service]
			if node == nil || node.kind != yamlMap {
				continue
			}
			runner := false
			for _, key := range []string{"command", "entrypoint"} {
				if c := node.get(key); c != nil {
					runner = runner || testRunnerWord.MatchString(" "+c.joined()+" ")
				}
			}
			vars := map[string]EnvVariable{}
			add := func(name, raw, source, kind string, line int) {
				r := resolver.evaluate(raw, map[string]bool{})
				vars[name] = EnvVariable{Name: name, Source: source, SourceKind: kind, Service: service, Line: line, Value: r.value,
					UnknownReason: r.reason, Derivation: derive(raw), PassedToTests: runner, Reliable: runner,
					Endpoint: endpointName(name) || r.value != nil && (strings.HasPrefix(*r.value, "http://") || strings.HasPrefix(*r.value, "https://"))}
			}
			for _, envFile := range composeEnvFiles(node.get("env_file")) {
				name := path.Clean(path.Join(path.Dir(file), envFile))
				if path.Base(name) == ".env."+profile {
					continue // the profile's own dotenv stays unread (TSA-V0-353)
				}
				envText, ok := texts[name]
				if !ok {
					missing[name] = true
					notes = append(notes, EnvSourceNote{Path: name, Reason: envNotCommitted})
					continue
				}
				for i, line := range strings.Split(envText, "\n") {
					line = strings.TrimPrefix(strings.TrimSpace(line), "export ")
					if m := assignmentWord.FindStringSubmatch(line); m != nil && !strings.HasPrefix(line, "#") {
						add(m[1], m[2], name, SourceComposeEnvFile, i+1)
					}
				}
			}
			environment := node.get("environment")
			switch {
			case environment == nil:
			case environment.kind == yamlMap:
				for _, name := range environment.keys {
					value := environment.m[name]
					if value.kind != yamlScalar || !nameWord.MatchString(name) {
						continue
					}
					if value.block {
						blockValue = true
					}
					raw := value.text
					if value.null {
						raw = "${" + name + "}"
					}
					add(name, raw, file, SourceComposeEnvironment, value.line)
				}
			case environment.kind == yamlList:
				for _, item := range environment.items {
					if item.kind != yamlScalar {
						continue
					}
					name, raw, assigned := strings.Cut(item.text, "=")
					if !nameWord.MatchString(name) {
						continue
					}
					if !assigned {
						raw = "${" + name + "}"
					}
					add(name, raw, file, SourceComposeEnvironment, item.line)
				}
			}
			names := make([]string, 0, len(vars))
			for name := range vars {
				names = append(names, name)
			}
			sort.Strings(names)
			for _, name := range names {
				fileOut = append(fileOut, vars[name])
			}
		}
		if blockValue {
			notes = append(notes, EnvSourceNote{Path: file, Reason: envComposeUnparsed})
			continue
		}
		out = append(out, fileOut...)
	}
	return out, notes
}

// composeEnvFiles reads `env_file` as a path, a list of paths or a list of `path:` entries.
func composeEnvFiles(node *yamlNode) []string {
	out := []string{}
	switch {
	case node == nil:
	case node.kind == yamlScalar && !node.null:
		out = append(out, node.text)
	case node.kind == yamlList:
		for _, item := range node.items {
			if item.kind == yamlScalar && !item.null {
				out = append(out, item.text)
			} else if p := item.get("path"); p != nil && p.kind == yamlScalar {
				out = append(out, p.text)
			}
		}
	}
	return out
}

// A small, tolerant reader for the block-style YAML of workflow and compose files. It reads maps,
// lists, plain and quoted scalars, block scalars and one-level flow collections, and fails closed
// on anchors, aliases, tags, merge keys, several documents and any shape it does not read, so a
// file it cannot read is reported unparsed instead of guessed.

const (
	yamlScalar = 's'
	yamlMap    = 'm'
	yamlList   = 'l'
)

type yamlNode struct {
	kind  byte
	line  int
	text  string
	null  bool
	block bool // a `|` or `>` block scalar, whose bytes the reader does not keep exactly
	keys  []string
	m     map[string]*yamlNode
	items []*yamlNode
}

func (n *yamlNode) get(key string) *yamlNode {
	if n == nil || n.kind != yamlMap {
		return nil
	}
	return n.m[key]
}

// joined is a scalar's text or a list of scalars joined by spaces.
func (n *yamlNode) joined() string {
	if n.kind == yamlScalar {
		return n.text
	}
	parts := []string{}
	for _, item := range n.items {
		if item.kind == yamlScalar {
			parts = append(parts, item.text)
		}
	}
	return strings.Join(parts, " ")
}

type yamlLine struct {
	indent, number int
	text           string
}

type yamlReader struct {
	lines []yamlLine
	i     int
	bad   bool
}

func readYAML(text string) (*yamlNode, bool) {
	r := &yamlReader{}
	documents := 0
	for i, raw := range strings.Split(text, "\n") {
		trimmed := strings.TrimRight(raw, " \t\r")
		content := strings.TrimLeft(trimmed, " ")
		if content == "" || strings.HasPrefix(content, "#") {
			continue
		}
		if strings.HasPrefix(content, "\t") {
			return nil, false
		}
		if content == "---" || strings.HasPrefix(content, "--- ") || content == "..." {
			if documents++; documents > 1 || len(r.lines) > 0 {
				return nil, false
			}
			continue
		}
		r.lines = append(r.lines, yamlLine{indent: len(trimmed) - len(content), number: i + 1, text: content})
	}
	if len(r.lines) == 0 {
		return &yamlNode{kind: yamlMap, m: map[string]*yamlNode{}}, true
	}
	node := r.node(r.lines[0].indent)
	if r.bad || r.i != len(r.lines) {
		return nil, false
	}
	return node, true
}

func yamlItem(text string) bool { return text == "-" || strings.HasPrefix(text, "- ") }

func (r *yamlReader) node(indent int) *yamlNode {
	if yamlItem(r.lines[r.i].text) {
		return r.list(indent)
	}
	return r.mapping(indent)
}

func (r *yamlReader) list(indent int) *yamlNode {
	n := &yamlNode{kind: yamlList, line: r.lines[r.i].number}
	for !r.bad && r.i < len(r.lines) && r.lines[r.i].indent == indent && yamlItem(r.lines[r.i].text) {
		line := r.lines[r.i]
		content := strings.TrimLeft(line.text[1:], " ")
		switch {
		case content == "":
			r.i++
			if r.i < len(r.lines) && r.lines[r.i].indent > indent {
				n.items = append(n.items, r.node(r.lines[r.i].indent))
			} else {
				n.items = append(n.items, &yamlNode{kind: yamlScalar, line: line.number, null: true})
			}
		case yamlItem(content) || mapEntry(content):
			at := indent + len(line.text) - len(content)
			r.lines[r.i] = yamlLine{indent: at, number: line.number, text: content}
			n.items = append(n.items, r.node(at))
		default:
			r.i++
			n.items = append(n.items, r.scalar(content, line.number))
		}
	}
	if r.i < len(r.lines) && r.lines[r.i].indent > indent {
		r.bad = true
	}
	return n
}

// mapEntry reports a `key:` or `key: value` line.
func mapEntry(text string) bool {
	_, _, ok := splitKey(text)
	return ok
}

func splitKey(text string) (string, string, bool) {
	if text[0] == '"' || text[0] == '\'' {
		end := strings.IndexByte(text[1:], text[0])
		if end < 0 {
			return "", "", false
		}
		key, rest := text[1:end+1], text[end+2:]
		if rest != ":" && !strings.HasPrefix(rest, ": ") {
			return "", "", false
		}
		return key, strings.TrimPrefix(rest, ":"), true
	}
	for i := 0; i < len(text); i++ {
		if text[i] == ' ' && i+1 < len(text) && text[i+1] == '#' {
			return "", "", false
		}
		if text[i] == ':' && (i+1 == len(text) || text[i+1] == ' ') {
			return strings.TrimSpace(text[:i]), text[i+1:], i > 0
		}
	}
	return "", "", false
}

func (r *yamlReader) mapping(indent int) *yamlNode {
	n := &yamlNode{kind: yamlMap, line: r.lines[r.i].number, m: map[string]*yamlNode{}}
	for !r.bad && r.i < len(r.lines) && r.lines[r.i].indent == indent && !yamlItem(r.lines[r.i].text) {
		line := r.lines[r.i]
		key, rest, ok := splitKey(line.text)
		if !ok || key == "" || key == "<<" || strings.ContainsAny(key[:1], "?&*!") {
			r.bad = true
			return n
		}
		r.i++
		rest = strings.TrimSpace(stripYAMLComment(rest))
		var child *yamlNode
		switch {
		case rest == "":
			if r.i < len(r.lines) && (r.lines[r.i].indent > indent || r.lines[r.i].indent == indent && yamlItem(r.lines[r.i].text)) {
				child = r.node(r.lines[r.i].indent)
			} else {
				child = &yamlNode{kind: yamlScalar, line: line.number, null: true}
			}
		case rest[0] == '|' || rest[0] == '>':
			parts := []string{}
			for r.i < len(r.lines) && r.lines[r.i].indent > indent {
				parts = append(parts, r.lines[r.i].text)
				r.i++
			}
			child = &yamlNode{kind: yamlScalar, line: line.number, text: strings.Join(parts, "\n"), block: true}
		default:
			child = r.scalar(rest, line.number)
		}
		if _, dup := n.m[key]; dup {
			r.bad = true
			return n
		}
		n.keys = append(n.keys, key)
		n.m[key] = child
	}
	if r.i < len(r.lines) && r.lines[r.i].indent > indent {
		r.bad = true
	}
	return n
}

func (r *yamlReader) scalar(text string, line int) *yamlNode {
	text = strings.TrimSpace(stripYAMLComment(text))
	n := &yamlNode{kind: yamlScalar, line: line, text: text}
	switch {
	case text == "" || text == "~" || text == "null":
		n.text, n.null = "", true
	case strings.ContainsAny(text[:1], "&*!%@`"):
		r.bad = true
	case text[0] == '"':
		value, err := strconv.Unquote(text)
		if err != nil {
			r.bad = true
		}
		n.text = value
	case text[0] == '\'':
		if len(text) < 2 || text[len(text)-1] != '\'' {
			r.bad = true
			return n
		}
		n.text = strings.ReplaceAll(text[1:len(text)-1], "''", "'")
	case text[0] == '[':
		inner, ok := strings.CutSuffix(text[1:], "]")
		if !ok || strings.ContainsAny(inner, "[]{}") {
			r.bad = true
			return n
		}
		n = &yamlNode{kind: yamlList, line: line}
		for _, part := range strings.Split(inner, ",") {
			if part = strings.TrimSpace(part); part != "" {
				n.items = append(n.items, r.scalar(part, line))
			}
		}
	case text[0] == '{':
		inner, ok := strings.CutSuffix(text[1:], "}")
		if !ok || strings.ContainsAny(inner, "[]{}") {
			r.bad = true
			return n
		}
		n = &yamlNode{kind: yamlMap, line: line, m: map[string]*yamlNode{}}
		for _, part := range strings.Split(inner, ",") {
			if part = strings.TrimSpace(part); part == "" {
				continue
			}
			key, value, ok := splitKey(part)
			if _, dup := n.m[key]; !ok || key == "" || dup {
				r.bad = true
				return n
			}
			n.keys = append(n.keys, key)
			n.m[key] = r.scalar(value, line)
		}
	}
	return n
}

// stripYAMLComment removes a ` #` comment outside quotes.
func stripYAMLComment(text string) string {
	quote := byte(0)
	for i := 0; i < len(text); i++ {
		c := text[i]
		switch {
		case quote != 0:
			if c == quote {
				quote = 0
			}
		case c == '"' || c == '\'':
			if i == 0 || text[i-1] == ' ' {
				quote = c
			}
		case c == '#' && (i == 0 || text[i-1] == ' '):
			return text[:i]
		}
	}
	return text
}
