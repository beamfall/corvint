package affected

import (
	"errors"
	"regexp"
	"strings"
)

// errDriverYAML marks workflow text outside the YAML subset the driver
// claimant reads (AZU-V0-001); the file then selects all of itself.
var errDriverYAML = errors.New("affected: workflow text outside the supported YAML subset")

// driverYAML is one node of the YAML subset: a scalar, a mapping with its
// keys in order, or a sequence.
type driverYAML struct {
	scalar  string
	isMap   bool
	isSeq   bool
	keys    []string
	values  map[string]*driverYAML
	items   []*driverYAML
	keyLine map[string]int
}

func (node *driverYAML) get(key string) *driverYAML {
	if node == nil || !node.isMap {
		return nil
	}
	return node.values[key]
}

func (node *driverYAML) text() string {
	if node == nil || node.isMap || node.isSeq {
		return ""
	}
	return node.scalar
}

// strings is a scalar or a sequence of scalars as a list.
func (node *driverYAML) strings() []string {
	switch {
	case node == nil || node.isMap:
		return nil
	case node.isSeq:
		values := make([]string, 0, len(node.items))
		for _, item := range node.items {
			values = append(values, item.text())
		}
		return values
	case node.scalar == "":
		return nil
	}
	return []string{node.scalar}
}

type driverYAMLLine struct {
	indent int
	text   string // without indentation or trailing comment
	raw    string
	block  bool // content of a block scalar
}

type driverYAMLParser struct {
	lines []driverYAMLLine
	at    int
	// anchors holds every `&name` node defined so far, for `*name` aliases
	// (AZU-V0-214). An alias to a name not yet defined is outside the subset.
	anchors map[string]*driverYAML
}

// driverYAMLAnchor splits a leading `&name` or `*name` property from the
// text that follows it: the name is one or more characters other than
// whitespace, flow indicators and the YAML indicator characters that cannot
// start an anchor name.
func driverYAMLAnchor(text string) (indicator byte, name, rest string, ok bool) {
	if text == "" || text[0] != '&' && text[0] != '*' {
		return 0, "", text, false
	}
	end := 1
	for end < len(text) && !strings.ContainsRune(" \t,[]{}", rune(text[end])) {
		end++
	}
	if end == 1 {
		return 0, "", text, false
	}
	return text[0], text[1:end], strings.TrimSpace(text[end:]), true
}

// alias resolves `*name` to a copy of its anchored node; the copy shares the
// node's children, which the parser never mutates after it returns them. An
// unknown name is outside the subset, so the file stays unparsable rather
// than silently dropping a step (AZU-V0-214).
func (p *driverYAMLParser) alias(name string) (*driverYAML, error) {
	node, defined := p.anchors[name]
	if !defined {
		return nil, errDriverYAML
	}
	copied := *node
	return &copied, nil
}

// anchor records node under name; redefining a name is outside the subset.
func (p *driverYAMLParser) anchor(name string, node *driverYAML) error {
	if p.anchors == nil {
		p.anchors = make(map[string]*driverYAML)
	}
	if _, defined := p.anchors[name]; defined {
		return errDriverYAML
	}
	p.anchors[name] = node
	return nil
}

// parseDriverYAML parses body, marking each line that is block-scalar content.
func parseDriverYAML(body string) (*driverYAML, []driverYAMLLine, error) {
	body = strings.ReplaceAll(body, "\r\n", "\n")
	physical := strings.Split(body, "\n")
	parser := &driverYAMLParser{lines: make([]driverYAMLLine, 0, len(physical))}
	for index, raw := range physical {
		indent := 0
		for indent < len(raw) && raw[indent] == ' ' {
			indent++
		}
		if indent < len(raw) && raw[indent] == '\t' {
			return nil, nil, errDriverYAML
		}
		text := strings.TrimRight(driverYAMLStrip(raw[indent:]), " \t")
		if index == 0 && text == "---" {
			text = ""
		}
		if indent == 0 && (text == "---" || text == "..." || strings.HasPrefix(text, "%")) {
			return nil, nil, errDriverYAML
		}
		parser.lines = append(parser.lines, driverYAMLLine{indent: indent, text: text, raw: raw})
	}
	parser.skip()
	if parser.at >= len(parser.lines) {
		return &driverYAML{isMap: true, values: map[string]*driverYAML{}, keyLine: map[string]int{}}, parser.lines, nil
	}
	root, err := parser.block(parser.lines[parser.at].indent)
	if err != nil {
		return nil, nil, err
	}
	parser.skip()
	if parser.at < len(parser.lines) {
		return nil, nil, errDriverYAML
	}
	return root, parser.lines, nil
}

// driverYAMLStrip removes a comment outside quotes: `#` at the start or after
// whitespace.
func driverYAMLStrip(text string) string {
	quote := byte(0)
	for index := 0; index < len(text); index++ {
		char := text[index]
		switch {
		case quote != 0:
			if char == quote {
				if quote == '\'' && index+1 < len(text) && text[index+1] == '\'' {
					index++
					continue
				}
				quote = 0
			} else if char == '\\' && quote == '"' {
				index++
			}
		case char == '#' && (index == 0 || text[index-1] == ' ' || text[index-1] == '\t'):
			return text[:index]
		case (char == '\'' || char == '"') && (index == 0 || strings.IndexByte(" :[{,-", text[index-1]) >= 0):
			quote = char
		}
	}
	return text
}

func (p *driverYAMLParser) skip() {
	for p.at < len(p.lines) && p.lines[p.at].text == "" {
		p.at++
	}
}

func (p *driverYAMLParser) block(indent int) (*driverYAML, error) {
	p.skip()
	if p.at >= len(p.lines) || p.lines[p.at].indent != indent {
		return nil, errDriverYAML
	}
	if driverYAMLItem(p.lines[p.at].text) {
		return p.sequence(indent)
	}
	return p.mapping(indent)
}

func driverYAMLItem(text string) bool {
	return text == "-" || strings.HasPrefix(text, "- ")
}

// driverYAMLKey splits `key: value` at the first `:` outside quotes that is
// followed by a space or ends the text.
func driverYAMLKey(text string) (key, value string, ok bool) {
	quote := byte(0)
	depth := 0
	for index := 0; index < len(text); index++ {
		char := text[index]
		switch {
		case quote != 0:
			if char == quote {
				quote = 0
			}
		case char == '\'' || char == '"':
			if index == 0 {
				quote = char
			}
		case char == '[' || char == '{':
			depth++
		case char == ']' || char == '}':
			depth--
		case char == ':' && depth == 0 && (index+1 == len(text) || text[index+1] == ' '):
			key = strings.TrimSpace(text[:index])
			return driverYAMLUnquote(key), strings.TrimSpace(text[index+1:]), true
		}
	}
	return "", "", false
}

func driverYAMLUnquote(value string) string {
	if len(value) >= 2 && (value[0] == '"' && value[len(value)-1] == '"' || value[0] == '\'' && value[len(value)-1] == '\'') {
		inner := value[1 : len(value)-1]
		if value[0] == '\'' {
			return strings.ReplaceAll(inner, "''", "'")
		}
		return strings.NewReplacer(`\"`, `"`, `\\`, `\`, `\n`, "\n", `\t`, "\t").Replace(inner)
	}
	return value
}

func (p *driverYAMLParser) mapping(indent int) (*driverYAML, error) {
	node := &driverYAML{isMap: true, values: map[string]*driverYAML{}, keyLine: map[string]int{}}
	for {
		p.skip()
		if p.at >= len(p.lines) || p.lines[p.at].indent < indent {
			return node, nil
		}
		line := p.lines[p.at]
		if line.indent > indent || driverYAMLItem(line.text) {
			return nil, errDriverYAML
		}
		key, value, ok := driverYAMLKey(line.text)
		if !ok || key == "" || key == "<<" || strings.HasPrefix(key, "?") || strings.HasPrefix(key, "&") || strings.HasPrefix(key, "*") || strings.HasPrefix(key, "!") {
			return nil, errDriverYAML
		}
		if _, duplicate := node.values[key]; duplicate {
			return nil, errDriverYAML
		}
		node.keyLine[key] = p.at
		p.at++
		child, err := p.value(value, indent, true)
		if err != nil {
			return nil, err
		}
		node.keys = append(node.keys, key)
		node.values[key] = child
	}
}

func (p *driverYAMLParser) sequence(indent int) (*driverYAML, error) {
	node := &driverYAML{isSeq: true}
	for {
		p.skip()
		if p.at >= len(p.lines) || p.lines[p.at].indent < indent {
			return node, nil
		}
		line := p.lines[p.at]
		if line.indent > indent || !driverYAMLItem(line.text) {
			if line.indent == indent {
				return node, nil
			}
			return nil, errDriverYAML
		}
		rest := strings.TrimSpace(strings.TrimPrefix(line.text, "-"))
		// `- &name` anchors the item that follows; `- *name` repeats an
		// anchored item (AZU-V0-214). An anchor on a nested item (`- &name
		// key: value`) anchors that mapping.
		anchorName := ""
		if indicator, name, after, isAnchor := driverYAMLAnchor(rest); isAnchor {
			if indicator == '*' {
				if after != "" {
					return nil, errDriverYAML
				}
				p.at++
				child, err := p.alias(name)
				if err != nil {
					return nil, err
				}
				node.items = append(node.items, child)
				continue
			}
			anchorName = name
			rest = after
			p.lines[p.at].text = strings.TrimSpace(line.text[:len(line.text)-len(strings.TrimLeft(strings.TrimPrefix(line.text, "-"), " "))]) + " " + rest
			if rest == "" {
				p.lines[p.at].text = "-"
			}
			line = p.lines[p.at]
		}
		if rest == "" {
			p.at++
			p.skip()
			if p.at < len(p.lines) && p.lines[p.at].indent > indent {
				child, err := p.block(p.lines[p.at].indent)
				if err != nil {
					return nil, err
				}
				if anchorName != "" {
					if err := p.anchor(anchorName, child); err != nil {
						return nil, err
					}
				}
				node.items = append(node.items, child)
				continue
			}
			child := &driverYAML{}
			if anchorName != "" {
				if err := p.anchor(anchorName, child); err != nil {
					return nil, err
				}
			}
			node.items = append(node.items, child)
			continue
		}
		if _, _, isKey := driverYAMLKey(rest); isKey || driverYAMLItem(rest) {
			// `- key: value` opens a mapping (or `- - x` a sequence) at the
			// column of its first key.
			offset := len(line.text) - len(strings.TrimLeft(strings.TrimPrefix(line.text, "-"), " "))
			p.lines[p.at].indent = indent + offset
			p.lines[p.at].text = rest
			child, err := p.block(indent + offset)
			if err != nil {
				return nil, err
			}
			if anchorName != "" {
				if err := p.anchor(anchorName, child); err != nil {
					return nil, err
				}
			}
			node.items = append(node.items, child)
			continue
		}
		p.at++
		child, err := p.value(rest, indent, false)
		if err != nil {
			return nil, err
		}
		if anchorName != "" {
			if err := p.anchor(anchorName, child); err != nil {
				return nil, err
			}
		}
		node.items = append(node.items, child)
	}
}

// value parses what follows a key's colon or a sequence dash. parent is the
// indentation of the key or dash; a nested block must be deeper, except that
// a mapping value may be a sequence at the key's own column.
func (p *driverYAMLParser) value(text string, parent int, mapValue bool) (*driverYAML, error) {
	switch {
	case text == "":
		p.skip()
		if p.at < len(p.lines) {
			next := p.lines[p.at]
			if next.indent > parent {
				return p.block(next.indent)
			}
			if mapValue && next.indent == parent && driverYAMLItem(next.text) {
				return p.sequence(parent)
			}
		}
		return &driverYAML{}, nil
	case strings.HasPrefix(text, "&") || strings.HasPrefix(text, "*"):
		// `key: &name value` anchors the value that follows, on this line
		// or as the nested block; `key: *name` repeats one (AZU-V0-214).
		indicator, name, rest, isAnchor := driverYAMLAnchor(text)
		if !isAnchor {
			return nil, errDriverYAML
		}
		if indicator == '*' {
			if rest != "" {
				return nil, errDriverYAML
			}
			return p.alias(name)
		}
		if _, _, _, isAlias := driverYAMLAnchor(rest); isAlias {
			return nil, errDriverYAML
		}
		child, err := p.value(rest, parent, mapValue)
		if err != nil {
			return nil, err
		}
		if err := p.anchor(name, child); err != nil {
			return nil, err
		}
		return child, nil
	case strings.HasPrefix(text, "!"):
		return nil, errDriverYAML
	case strings.HasPrefix(text, "|") || strings.HasPrefix(text, ">"):
		return p.blockScalar(text, parent)
	case strings.HasPrefix(text, "["):
		flow, err := p.flow(text, parent, '[', ']')
		if err != nil {
			return nil, err
		}
		return driverYAMLFlowSequence(flow)
	case strings.HasPrefix(text, "{"):
		flow, err := p.flow(text, parent, '{', '}')
		if err != nil {
			return nil, err
		}
		return &driverYAML{scalar: flow}, nil
	}
	// A plain or quoted scalar continues on deeper lines.
	parts := []string{text}
	for p.at < len(p.lines) {
		next := p.lines[p.at]
		if next.text != "" && next.indent <= parent {
			break
		}
		if next.text == "" {
			lookahead := p.at
			for lookahead < len(p.lines) && p.lines[lookahead].text == "" {
				lookahead++
			}
			if lookahead >= len(p.lines) || p.lines[lookahead].indent <= parent {
				break
			}
		}
		parts = append(parts, strings.TrimSpace(next.raw))
		p.lines[p.at].block = true
		p.at++
	}
	joined := strings.Join(parts, " ")
	if !mapValue && strings.HasPrefix(joined, "-") {
		return &driverYAML{scalar: joined}, nil
	}
	return &driverYAML{scalar: driverYAMLUnquote(joined)}, nil
}

// flow reads a flow collection that may span lines until its brackets close.
func (p *driverYAMLParser) flow(text string, parent int, open, close byte) (string, error) {
	collected := text
	for {
		if depth := driverYAMLDepth(collected, open, close); depth == 0 {
			return collected, nil
		} else if depth < 0 {
			return "", errDriverYAML
		}
		if p.at >= len(p.lines) || p.lines[p.at].text != "" && p.lines[p.at].indent <= parent {
			return "", errDriverYAML
		}
		collected += " " + p.lines[p.at].text
		p.at++
	}
}

func driverYAMLDepth(text string, open, close byte) int {
	depth := 0
	quote := byte(0)
	for index := 0; index < len(text); index++ {
		char := text[index]
		switch {
		case quote != 0:
			if char == quote {
				quote = 0
			}
		case char == '\'' || char == '"':
			quote = char
		case char == open || char == '[' || char == '{':
			depth++
		case char == close || char == ']' || char == '}':
			depth--
		}
	}
	return depth
}

func driverYAMLFlowSequence(flow string) (*driverYAML, error) {
	inner := strings.TrimSpace(flow[1 : len(flow)-1])
	node := &driverYAML{isSeq: true}
	if inner == "" {
		return node, nil
	}
	for _, item := range driverMakeSplitTop(inner, ',', 0) {
		item = strings.TrimSpace(item)
		if strings.HasPrefix(item, "&") || strings.HasPrefix(item, "*") || strings.HasPrefix(item, "!") {
			return nil, errDriverYAML
		}
		if item != "" {
			node.items = append(node.items, &driverYAML{scalar: driverYAMLUnquote(item)})
		}
	}
	return node, nil
}

func (p *driverYAMLParser) blockScalar(header string, parent int) (*driverYAML, error) {
	folded := header[0] == '>'
	for _, char := range header[1:] {
		if !strings.ContainsRune("+-123456789", char) {
			return nil, errDriverYAML
		}
	}
	content := -1
	var lines []string
	for p.at < len(p.lines) {
		line := p.lines[p.at]
		blank := strings.TrimSpace(line.raw) == ""
		if !blank && line.indent <= parent {
			break
		}
		if !blank && content < 0 {
			content = line.indent
		}
		if !blank && line.indent < content {
			break
		}
		p.lines[p.at].block = true
		if blank {
			lines = append(lines, "")
		} else {
			lines = append(lines, line.raw[content:])
		}
		p.at++
	}
	for len(lines) > 0 && lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}
	separator := "\n"
	if folded {
		separator = " "
	}
	return &driverYAML{scalar: strings.Join(lines, separator)}, nil
}

// driverWorkflow is one parsed workflow: its jobs in order with each job's
// text, and the remainder text outside every job.
type driverWorkflow struct {
	path       string
	root       *driverYAML
	jobs       []string
	blocks     map[string]string
	remainder  string
	unparsable bool
}

// parseDriverWorkflow reads a workflow file. A file outside the subset, or
// one without a jobs mapping, is unparsable.
func parseDriverWorkflow(file string, body []byte) *driverWorkflow {
	workflow := &driverWorkflow{path: file, blocks: map[string]string{}}
	root, lines, err := parseDriverYAML(string(body))
	jobs := root.get("jobs")
	if err != nil || jobs == nil || !jobs.isMap || len(jobs.keys) == 0 {
		workflow.unparsable = true
		return workflow
	}
	workflow.root = root
	workflow.jobs = jobs.keys
	starts := make(map[int]string, len(jobs.keys))
	for _, job := range jobs.keys {
		starts[jobs.keyLine[job]] = job
	}
	jobsLine := root.keyLine["jobs"]
	jobIndent := lines[jobs.keyLine[jobs.keys[0]]].indent
	current := ""
	collected := map[string][]string{}
	var remainder []string
	for index, line := range lines {
		if job, start := starts[index]; start {
			current = job
		} else if current != "" && !line.block && line.text != "" && line.indent < jobIndent {
			current = ""
		}
		if index <= jobsLine {
			current = ""
		}
		if !line.block && (line.text == "") {
			continue
		}
		if current == "" {
			remainder = append(remainder, line.raw)
			continue
		}
		collected[current] = append(collected[current], line.raw)
	}
	for _, job := range jobs.keys {
		workflow.blocks[job] = strings.Join(collected[job], "\n")
	}
	workflow.remainder = strings.Join(remainder, "\n")
	return workflow
}

func driverWorkflowUnitID(file, job string) string {
	return DriverNamespace + ":ci:" + file + "#" + job
}

// driverStepOutputRef matches one `steps.ID.outputs.NAME` reference.
var driverStepOutputRef = regexp.MustCompile(`steps\.([A-Za-z0-9_-]+)\.outputs\.`)

// driverLiteralOutputLine matches a run line that writes one literal output:
// `echo "NAME=VALUE" >> "$GITHUB_OUTPUT"`, with or without quotes, where
// VALUE holds no `$`, backtick, backslash, quote or brace.
var driverLiteralOutputLine = regexp.MustCompile(`^echo\s+(?:"[A-Za-z_][A-Za-z0-9_-]*=[^"$\x60\\{}']*"|'[A-Za-z_][A-Za-z0-9_-]*=[^'$\x60\\{}"]*'|[A-Za-z_][A-Za-z0-9_-]*=[^\s"'$\x60\\{}]*)\s*>>?\s*(?:"\$GITHUB_OUTPUT"|"\$\{GITHUB_OUTPUT\}"|\$GITHUB_OUTPUT|\$\{GITHUB_OUTPUT\})$`)

// driverLiteralOutputSteps returns the ids of the steps whose every output is
// a literal (AFP-V0-9132): a step with an id, no `uses`, and a `run` script
// whose every line is blank, a comment or one driverLiteralOutputLine. Any
// other step, including one that runs a program, may write a value chosen
// at run time, so its outputs are not literal.
func driverLiteralOutputSteps(steps *driverYAML) map[string]bool {
	literal := map[string]bool{}
	if steps == nil || !steps.isSeq {
		return literal
	}
	for _, step := range steps.items {
		id := step.get("id").text()
		if id == "" || !step.isMap || step.get("uses").text() != "" {
			continue
		}
		script := step.get("run").text()
		if script == "" || strings.Contains(script, "${{") {
			continue
		}
		plain := true
		for _, line := range strings.Split(script, "\n") {
			line = strings.TrimSpace(line)
			if line == "" || strings.HasPrefix(line, "#") || driverLiteralOutputLine.MatchString(line) {
				continue
			}
			plain = false
			break
		}
		if plain {
			literal[id] = true
		}
	}
	return literal
}

// driverOutputExpressions replaces each `${{ ... }}` expression of text that
// reads a step output no step in literal wrote with driverRuntimeComputed,
// before driverExpressions sees it (AFP-V0-9132). A step output written from
// a command's output chooses what a later command word names at run time, so
// such a word is never root-like.
func driverOutputExpressions(text string, literal map[string]bool) string {
	var out strings.Builder
	for {
		start := strings.Index(text, "${{")
		if start < 0 {
			out.WriteString(text)
			return out.String()
		}
		end := strings.Index(text[start:], "}}")
		if end < 0 {
			out.WriteString(text)
			return out.String()
		}
		inner := text[start+3 : start+end]
		runtime := false
		for _, match := range driverStepOutputRef.FindAllStringSubmatch(inner, -1) {
			if !literal[match[1]] {
				runtime = true
			}
		}
		out.WriteString(text[:start])
		if runtime {
			out.WriteString(driverRuntimeComputed)
		} else {
			out.WriteString(text[start : start+end+2])
		}
		text = text[start+end+2:]
	}
}

// driverExpressions replaces each workflow `${{ ... }}` expression with
// driverComputed, so a scanner never reads one as a literal.
func driverExpressions(text string) string {
	for {
		start := strings.Index(text, "${{")
		if start < 0 {
			return text
		}
		end := strings.Index(text[start:], "}}")
		if end < 0 {
			return text[:start] + driverComputed
		}
		text = text[:start] + driverComputed + text[start+end+2:]
	}
}
