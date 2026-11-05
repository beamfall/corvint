package affected

import (
	"context"
	"path"
	"slices"
	"sort"
	"strings"
)

// The driver claimant (AZU-V0-001) owns every build or CI driver input: make
// files at any depth and GitHub Actions workflow and action files. Its units
// are make targets and workflow jobs; its edges are the literal paths, Go
// package patterns and make targets their recipes and steps run.
const (
	DriverNamespace = "driver"
	// FrontierDriverComputedInvocation marks a recipe or step whose command,
	// interpreter or sourced file is computed, so what it runs is unknown.
	FrontierDriverComputedInvocation = "driver:computed-invocation"
	// FrontierDriverUnparsableFile marks the single unit of a driver file
	// outside the subset the claimant reads.
	FrontierDriverUnparsableFile = "driver:unparsable-file"
	// SelectionKindDriverTarget selects one make target, run with Argv.
	SelectionKindDriverTarget = "DRIVER_TARGET"
	// SelectionKindCIJob selects one workflow job, left to CI.
	SelectionKindCIJob = "CI_JOB"
	// WitnessFrontierSite names a make target or workflow job selected
	// because it invokes a computed name (AZU-V0-001, AZU-V0-003).
	WitnessFrontierSite = "FRONTIER_SITE"
)

// driverSites selects, on every plan with a dirty path, each make target or
// workflow job whose recipe or step invokes a computed name, and the driver
// units that depend on it. The hidden edge of a computed invocation starts at
// that target or job, so whatever path it binds, only the site and its
// dependents can observe the change; selecting them discharges the
// driver:computed-invocation entry, which frontierUnknowns then leaves out
// for a reached unit. A plan of declared data alone still selects them: a
// computed name can run a check that reads such a path.
func (graph *Graph) driverSites(reached map[string]Witness, followed map[string]bool, dirty []string) {
	if len(dirty) == 0 {
		return
	}
	var queue []string
	for _, id := range graph.order {
		if slices.Contains(graph.units[id].Frontier, FrontierDriverComputedInvocation) {
			if _, hit := reached[id]; !hit {
				reached[id] = Witness{Kind: WitnessFrontierSite, DirtyPath: dirty[0], Via: []string{id}}
			}
			queue = append(queue, id)
		}
	}
	visited := make(map[string]bool, len(queue))
	for len(queue) != 0 {
		id := queue[0]
		queue = queue[1:]
		if visited[id] {
			continue
		}
		visited[id] = true
		followed[id] = true
		for _, dependent := range graph.dependents[id] {
			if _, hit := reached[dependent]; !hit {
				reached[dependent] = Witness{Kind: WitnessDependency, DirtyPath: reached[id].DirtyPath, Via: append(slices.Clone(reached[id].Via), dependent)}
			}
			queue = append(queue, dependent)
		}
	}
}

// DriverInput reports a path a build or CI executor outside every language
// plugin runs as its own configuration (AFP-V0-601): a make file at any
// depth, or a GitHub Actions workflow or action definition. The driver
// claimant owns exactly these paths (AZU-V0-001).
func DriverInput(path string) bool {
	if driverMakeName(path[strings.LastIndex(path, "/")+1:]) {
		return true
	}
	return strings.HasPrefix(path, ".github/workflows/") || strings.HasPrefix(path, ".github/actions/")
}

// Driver is the built-in driver claimant (AZU-V0-001).
type Driver struct{}

// Name is the claimant's namespace.
func (Driver) Name() string { return DriverNamespace }

// Owns claims every driver input.
func (Driver) Owns(path string) bool { return DriverInput(path) }

// Units observes the repository rooted at root.
func (driver Driver) Units(ctx context.Context, root string) (Result, error) {
	return driver.UnitsSource(DiskSourceContext(ctx, root))
}

// driverRegistry holds every parsed driver file while units are built.
type driverRegistry struct {
	module    string
	makes     map[string]*driverMakeFile
	makeOrder []string
	workflows map[string]*driverWorkflow
	actions   map[string]string
	fileUnits map[string][]string
	allMake   []string
}

// UnitsSource observes the driver files through source.
func (Driver) UnitsSource(source *Source) (Result, error) {
	makeFiles, err := source.Files(ByName(driverMakeName))
	if err != nil {
		return Result{}, err
	}
	workflowFiles, err := source.FilesUnder(".github/workflows", func(name string) bool {
		return strings.HasSuffix(name, ".yml") || strings.HasSuffix(name, ".yaml")
	})
	if err != nil {
		return Result{}, err
	}
	actionFiles, err := source.FilesUnder(".github/actions", func(name string) bool {
		return name == "action.yml" || name == "action.yaml"
	})
	if err != nil {
		return Result{}, err
	}
	registry := &driverRegistry{
		makes:     map[string]*driverMakeFile{},
		workflows: map[string]*driverWorkflow{},
		actions:   map[string]string{},
		fileUnits: map[string][]string{},
	}
	if body, readErr := source.Read("go.mod"); readErr == nil {
		registry.module = driverModulePath(body)
	} else if fatal := source.Err(); fatal != nil {
		return Result{}, fatal
	}
	bodies := map[string][]byte{}
	for _, file := range slices.Concat(makeFiles, workflowFiles, actionFiles) {
		body, err := source.Read(file)
		if err != nil {
			return Result{}, err
		}
		bodies[file] = body
	}
	sort.Strings(makeFiles)
	for _, file := range makeFiles {
		parsed := parseDriverMake(file, bodies[file])
		registry.makes[file] = parsed
		registry.makeOrder = append(registry.makeOrder, file)
		if parsed.unparsable {
			registry.fileUnits[file] = []string{driverMakeUnitID(file, "*")}
		} else {
			ids := make([]string, 0, len(parsed.order))
			for _, target := range parsed.order {
				ids = append(ids, driverMakeUnitID(file, target))
			}
			sort.Strings(ids)
			registry.fileUnits[file] = ids
		}
		registry.allMake = append(registry.allMake, registry.fileUnits[file]...)
	}
	for _, file := range workflowFiles {
		workflow := parseDriverWorkflow(file, bodies[file])
		registry.workflows[file] = workflow
		if workflow.unparsable {
			registry.fileUnits[file] = []string{driverWorkflowUnitID(file, "*")}
			continue
		}
		ids := make([]string, 0, len(workflow.jobs))
		for _, job := range workflow.jobs {
			ids = append(ids, driverWorkflowUnitID(file, job))
		}
		sort.Strings(ids)
		registry.fileUnits[file] = ids
	}
	for _, file := range actionFiles {
		registry.actions[path.Dir(file)] = file
	}
	units := make([]Unit, 0, len(registry.allMake)+len(workflowFiles)*4)
	for _, file := range registry.makeOrder {
		units = append(units, registry.makeUnits(registry.makes[file], bodies[file])...)
	}
	for _, file := range workflowFiles {
		units = append(units, registry.workflowUnits(registry.workflows[file], bodies[file])...)
	}
	for _, file := range actionFiles {
		units = append(units, registry.actionUnit(file, bodies[file]))
	}
	sort.Slice(units, func(i, j int) bool { return units[i].ID < units[j].ID })
	return Result{Units: units}, nil
}

func driverModulePath(body []byte) string {
	for _, line := range strings.Split(string(body), "\n") {
		if fields := strings.Fields(line); len(fields) >= 2 && fields[0] == "module" {
			return strings.Trim(fields[1], `"`)
		}
	}
	return ""
}

// driverUnitBuilder collects one unit's edges.
type driverUnitBuilder struct {
	id       string
	imports  map[string]bool
	paths    map[string]bool
	tokens   map[string]bool
	computed bool
}

func newDriverUnitBuilder(id string) *driverUnitBuilder {
	return &driverUnitBuilder{id: id, imports: map[string]bool{}, paths: map[string]bool{}, tokens: map[string]bool{}}
}

func (b *driverUnitBuilder) importAll(ids []string) {
	for _, id := range ids {
		b.imports[id] = true
	}
}

func (b *driverUnitBuilder) addTokens(text string) {
	for _, token := range driverPathTokens(text) {
		b.tokens[token] = true
	}
}

// merge folds a shell scan into the unit, its make calls resolved.
func (b *driverUnitBuilder) merge(registry *driverRegistry, scan *driverScan) {
	if scan.computed {
		b.computed = true
	}
	for token := range scan.tokens {
		b.tokens[token] = true
	}
	for value := range scan.paths {
		b.paths[value] = true
	}
	for _, call := range scan.makes {
		b.importAll(registry.resolveMake(call))
	}
}

func (b *driverUnitBuilder) unit() Unit {
	delete(b.imports, b.id)
	unit := Unit{
		ID:          b.id,
		Sources:     []string{},
		Tests:       []string{},
		Imports:     driverSorted(b.imports),
		PathImports: driverSorted(b.paths),
		PathTokens:  driverSorted(b.tokens),
	}
	if len(unit.PathImports) == 0 {
		unit.PathImports = nil
	}
	if len(unit.PathTokens) == 0 {
		unit.PathTokens = nil
	}
	if b.computed {
		unit.Frontier = []string{FrontierDriverComputedInvocation}
	}
	return unit
}

func driverSorted(set map[string]bool) []string {
	values := make([]string, 0, len(set))
	for value := range set {
		if value != "" {
			values = append(values, value)
		}
	}
	sort.Strings(values)
	return values
}

// scanBoth scans text with the working directory cwd and, when that is not
// the root, with the root as well: the directory an executor runs it from is
// not always the file's own.
func (registry *driverRegistry) scanBoth(text, cwd string) *driverScan {
	scan := newDriverScan()
	scan.scanShell(text, cwd, registry.module)
	if cwd != "." {
		scan.scanShell(text, ".", registry.module)
	}
	return scan
}

// resolveMake names the units one make invocation can run. A computed
// directory or file reaches every make unit; a computed or unknown goal every
// unit of the file it names.
func (registry *driverRegistry) resolveMake(call driverMakeCall) []string {
	if call.Unbounded {
		return registry.allMake
	}
	file := ""
	if call.File != "" {
		file = driverJoin(call.Dir, call.File)
	} else {
		for _, name := range []string{"GNUmakefile", "makefile", "Makefile"} {
			candidate := driverJoin(call.Dir, name)
			if _, present := registry.makes[candidate]; present {
				file = candidate
				break
			}
		}
	}
	parsed, present := registry.makes[file]
	if !present {
		return nil
	}
	all := registry.fileUnits[file]
	if parsed.unparsable || call.Computed {
		return all
	}
	goals := call.Targets
	if len(goals) == 0 {
		if parsed.defaultGoal == "" || driverDynamic(parsed.defaultGoal) {
			return all
		}
		goals = strings.Fields(parsed.defaultGoal)
	}
	ids := make([]string, 0, len(goals))
	for _, goal := range goals {
		if _, known := parsed.rules[goal]; !known {
			return all
		}
		ids = append(ids, driverMakeUnitID(file, goal))
		for _, pattern := range parsed.order {
			if parsed.rules[pattern].pattern && driverPatternMatches(pattern, goal) {
				ids = append(ids, driverMakeUnitID(file, pattern))
			}
		}
	}
	return ids
}

// makeUnits builds one unit per rule of a parsed make file, or the single
// unparsable-file unit.
func (registry *driverRegistry) makeUnits(parsed *driverMakeFile, body []byte) []Unit {
	if parsed.unparsable {
		builder := newDriverUnitBuilder(driverMakeUnitID(parsed.path, "*"))
		builder.addTokens(string(body))
		unit := builder.unit()
		unit.Frontier = []string{FrontierDriverUnparsableFile}
		return []Unit{unit}
	}
	// What every target shares: the remainder's text, the recipes of rules
	// whose targets are special or computed, every `$(shell ...)` the
	// variables run while make reads the file, and the included files.
	shared := newDriverUnitBuilder("")
	shared.addTokens(strings.Join(parsed.remainder, "\n"))
	fileExpander := parsed.expander(map[string]string{})
	for _, name := range driverSortedKeys(parsed.vars) {
		for _, value := range parsed.vars[name] {
			fileExpander.expand(value)
		}
	}
	for _, line := range parsed.remainder {
		fileExpander.expand(line)
	}
	for _, script := range fileExpander.shells {
		shared.merge(registry, registry.scanBoth(script, parsed.dir))
	}
	for _, recipe := range parsed.remainderRecipes {
		line := fileExpander.expand(driverRecipeLine(recipe))
		shared.addTokens(line)
		shared.merge(registry, registry.scanBoth(line, parsed.dir))
	}
	for _, include := range parsed.includes {
		expanded := fileExpander.expand(include)
		shared.addTokens(expanded)
		for _, word := range strings.Fields(expanded) {
			if driverDynamic(word) || strings.ContainsAny(word, "*?[") {
				shared.importAll(registry.allMake)
				continue
			}
			for _, candidate := range driverVariants(parsed.dir, word) {
				if _, present := registry.makes[candidate]; present {
					shared.importAll(registry.fileUnits[candidate])
				}
			}
		}
	}
	shared.computed = shared.computed || fileExpander.ambiguous
	all := registry.fileUnits[parsed.path]
	units := make([]Unit, 0, len(parsed.order))
	for _, target := range parsed.order {
		rule := parsed.rules[target]
		builder := newDriverUnitBuilder(driverMakeUnitID(parsed.path, target))
		builder.computed = shared.computed
		builder.importAll(driverSorted(shared.imports))
		for value := range shared.paths {
			builder.paths[value] = true
		}
		for value := range shared.tokens {
			builder.tokens[value] = true
		}
		builder.addTokens(strings.Join(rule.block, "\n"))
		auto := map[string]string{"@": target, "@D": path.Dir(target), "@F": path.Base(target)}
		if rule.pattern {
			auto = map[string]string{}
		}
		expander := parsed.expander(auto)
		prerequisites := strings.Fields(expander.expand(strings.Join(rule.prereqs, " ")))
		prerequisites = slices.DeleteFunc(prerequisites, func(word string) bool { return word == "|" })
		if !rule.pattern {
			if len(prerequisites) > 0 {
				auto["<"] = prerequisites[0]
			}
			joined := strings.Join(prerequisites, " ")
			auto["^"], auto["+"], auto["?"] = joined, joined, joined
		}
		builder.addTokens(strings.Join(prerequisites, " "))
		for _, word := range prerequisites {
			registry.prerequisite(builder, parsed, word, all)
		}
		for _, pattern := range parsed.order {
			if parsed.rules[pattern].pattern && pattern != target && driverPatternMatches(pattern, target) {
				builder.imports[driverMakeUnitID(parsed.path, pattern)] = true
			}
		}
		for _, recipe := range rule.recipe {
			line := expander.expand(driverRecipeLine(recipe))
			builder.addTokens(line)
			builder.merge(registry, registry.scanBoth(line, parsed.dir))
		}
		for _, script := range expander.shells {
			builder.merge(registry, registry.scanBoth(script, parsed.dir))
		}
		builder.computed = builder.computed || expander.ambiguous
		units = append(units, builder.unit())
	}
	return units
}

// prerequisite adds the edge one prerequisite word names: a computed word
// every unit of the file, a target or a pattern rule matching it that unit,
// and any other word the owner of the file it names.
func (registry *driverRegistry) prerequisite(builder *driverUnitBuilder, parsed *driverMakeFile, word string, all []string) {
	if driverDynamic(word) {
		builder.importAll(all)
		return
	}
	matched := false
	if _, known := parsed.rules[word]; known {
		builder.imports[driverMakeUnitID(parsed.path, word)] = true
		matched = true
	}
	for _, pattern := range parsed.order {
		if parsed.rules[pattern].pattern && driverPatternMatches(pattern, word) {
			builder.imports[driverMakeUnitID(parsed.path, pattern)] = true
			matched = true
		}
	}
	if matched || strings.ContainsAny(word, "%*?[") {
		return
	}
	for _, candidate := range driverVariants(parsed.dir, word) {
		if ValidRelativePath(candidate) {
			builder.paths[candidate] = true
		}
	}
}

// driverRecipeLine drops a recipe line's leading `@`, `-` and `+` prefixes.
func driverRecipeLine(recipe string) string {
	return strings.TrimLeft(recipe, " \t@-+")
}

func driverSortedKeys(values map[string][]string) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

// workflowUnits builds one unit per job of a parsed workflow, or the single
// unparsable-file unit.
func (registry *driverRegistry) workflowUnits(workflow *driverWorkflow, body []byte) []Unit {
	if workflow.unparsable {
		builder := newDriverUnitBuilder(driverWorkflowUnitID(workflow.path, "*"))
		builder.addTokens(driverExpressions(string(body)))
		unit := builder.unit()
		unit.Frontier = []string{FrontierDriverUnparsableFile}
		return []Unit{unit}
	}
	remainderTokens := driverPathTokens(driverExpressions(workflow.remainder))
	topDirectory := workflow.root.get("defaults").get("run").get("working-directory").text()
	topShell := workflow.root.get("defaults").get("run").get("shell").text()
	jobs := workflow.root.get("jobs")
	units := make([]Unit, 0, len(workflow.jobs))
	for _, job := range workflow.jobs {
		node := jobs.get(job)
		builder := newDriverUnitBuilder(driverWorkflowUnitID(workflow.path, job))
		for _, token := range remainderTokens {
			builder.tokens[token] = true
		}
		builder.addTokens(driverExpressions(workflow.blocks[job]))
		for _, needed := range node.get("needs").strings() {
			builder.imports[driverWorkflowUnitID(workflow.path, needed)] = true
		}
		registry.uses(builder, node.get("uses").text())
		directory := node.get("defaults").get("run").get("working-directory").text()
		if directory == "" {
			directory = topDirectory
		}
		shell := node.get("defaults").get("run").get("shell").text()
		if shell == "" {
			shell = topShell
		}
		prefixes := map[string]bool{}
		for _, value := range []string{directory, topDirectory} {
			if value != "" {
				prefixes[value] = true
			}
		}
		steps := node.get("steps")
		if steps != nil && !steps.isSeq {
			builder.computed = true
		}
		if steps != nil && steps.isSeq {
			for _, step := range steps.items {
				uses := step.get("uses").text()
				if strings.HasPrefix(uses, "actions/checkout") {
					if value := step.get("with").get("path").text(); value != "" {
						prefixes[value] = true
					}
				}
				if value := step.get("working-directory").text(); value != "" {
					prefixes[value] = true
				}
			}
			for _, step := range steps.items {
				// Flow maps remain scalars in the YAML subset (AZU-V0-9703).
				// Preserve the job as a computed site instead of skipping it.
				if !step.isMap {
					builder.computed = true
				}
				registry.uses(builder, step.get("uses").text())
				script := step.get("run").text()
				if script == "" {
					continue
				}
				cwd := step.get("working-directory").text()
				if cwd == "" {
					cwd = directory
				}
				cwd = driverWorkflowDirectory(cwd)
				stepShell := step.get("shell").text()
				if stepShell == "" {
					stepShell = shell
				}
				registry.runStep(builder, driverExpressions(script), cwd, stepShell)
			}
		}
		registry.stripPrefixes(builder, prefixes)
		units = append(units, builder.unit())
	}
	return units
}

// driverWorkflowDirectory turns a step's working directory into a scan cwd:
// a computed or absolute one is unknown ("").
func driverWorkflowDirectory(value string) string {
	value = driverExpressions(value)
	if value == "" {
		return "."
	}
	if driverDynamic(value) || strings.HasPrefix(value, "/") {
		return ""
	}
	joined := driverJoin(".", value)
	return joined
}

// runStep scans one run script. A shell other than sh or bash is read for
// its literals only and marks the step computed.
func (registry *driverRegistry) runStep(builder *driverUnitBuilder, script, cwd, shell string) {
	builder.addTokens(script)
	interpreter := strings.Fields(shell)
	if len(interpreter) > 0 && interpreter[0] != "bash" && interpreter[0] != "sh" {
		builder.computed = true
	}
	scan := newDriverScan()
	scan.scanShell(script, cwd, registry.module)
	if cwd != "." {
		scan.scanShell(script, ".", registry.module)
	}
	builder.merge(registry, scan)
}

// uses adds the edge a job- or step-level `uses:` names: a local reusable
// workflow's jobs or a local action's unit.
func (registry *driverRegistry) uses(builder *driverUnitBuilder, value string) {
	if !strings.HasPrefix(value, "./") {
		return
	}
	target := path.Clean(strings.TrimPrefix(value, "./"))
	builder.tokens[target] = ValidRelativePath(target)
	if !ValidRelativePath(target) {
		delete(builder.tokens, target)
		return
	}
	if ids, present := registry.fileUnits[target]; present {
		builder.importAll(ids)
		return
	}
	if _, present := registry.actions[target]; present {
		builder.imports[driverActionUnitID(target)] = true
	}
}

// stripPrefixes adds, for each path import under a directory the job checks
// out into or runs from, the same path relative to the repository root:
// the job's workspace may hold the repository at that directory.
func (registry *driverRegistry) stripPrefixes(builder *driverUnitBuilder, prefixes map[string]bool) {
	for prefix := range prefixes {
		clean := driverJoin(".", driverExpressions(prefix))
		if clean == "" || clean == "." || driverDynamic(clean) {
			continue
		}
		for value := range builder.paths {
			switch {
			case value == clean+"/...":
				builder.paths["..."] = true
			case strings.HasPrefix(value, clean+"/"):
				builder.paths[strings.TrimPrefix(value, clean+"/")] = true
			}
		}
	}
}

func driverActionUnitID(directory string) string {
	return DriverNamespace + ":action:" + directory
}

// actionUnit builds a local action's unit: its run steps scanned and its
// whole text read for literals.
func (registry *driverRegistry) actionUnit(file string, body []byte) Unit {
	directory := path.Dir(file)
	builder := newDriverUnitBuilder(driverActionUnitID(directory))
	builder.addTokens(driverExpressions(string(body)))
	root, _, err := parseDriverYAML(string(body))
	if err != nil {
		unit := builder.unit()
		unit.Frontier = []string{FrontierDriverUnparsableFile}
		return unit
	}
	steps := root.get("runs").get("steps")
	if steps != nil && steps.isSeq {
		for _, step := range steps.items {
			registry.uses(builder, step.get("uses").text())
			if script := step.get("run").text(); script != "" {
				cwd := driverWorkflowDirectory(step.get("working-directory").text())
				registry.runStep(builder, driverExpressions(script), cwd, step.get("shell").text())
			}
		}
	}
	if using := root.get("runs").get("using").text(); using != "" && using != "composite" {
		// A JavaScript or container action runs files of its own directory.
		builder.paths[directory+"/..."] = true
	}
	return builder.unit()
}

// DriverRevision is one driver file's content at the plan's base and in the
// working tree, which attributes its diff to targets and jobs (AZU-V0-001).
type DriverRevision struct {
	Base       []byte
	Head       []byte
	BaseExists bool
	HeadExists bool
}

// WithDriverContent returns a copy of the graph that attributes a dirty
// driver file's diff with the given revisions. A dirty driver file without
// one selects every unit of the file.
func (graph *Graph) WithDriverContent(revisions map[string]DriverRevision) *Graph {
	copied := *graph
	copied.driverContent = revisions
	return &copied
}

// driverIndex maps each driver file to its units, and each local action
// directory to its unit.
func (graph *Graph) driverIndex() map[string][]string {
	index := map[string][]string{}
	for _, id := range graph.order {
		file, ok := driverUnitFile(id)
		if ok {
			index[file] = append(index[file], id)
		}
	}
	return index
}

// driverUnitFile is the file (or, for an action, the directory) a driver
// unit comes from.
func driverUnitFile(id string) (string, bool) {
	rest, ok := strings.CutPrefix(id, DriverNamespace+":")
	if !ok {
		return "", false
	}
	switch {
	case strings.HasPrefix(rest, "make:"):
		file, _, _ := strings.Cut(strings.TrimPrefix(rest, "make:"), "#")
		return file, true
	case strings.HasPrefix(rest, "ci:"):
		file, _, _ := strings.Cut(strings.TrimPrefix(rest, "ci:"), "#")
		return file, true
	case strings.HasPrefix(rest, "action:"):
		return strings.TrimPrefix(rest, "action:"), true
	}
	return "", false
}

// driverSeed seeds the driver units a dirty driver input changes and reports
// whether the claimant accounted for the path (AZU-V0-001). A hunk inside a
// target's or job's text seeds that unit; any other change, a file either
// side of which is outside the parsed subset, or a file without revisions
// seeds every unit of the file. A deleted file or a removed target or job
// seeds every driver unit. A path with no unit that is not a deletion is not
// accounted for, so it stays an unindexed source path.
func (graph *Graph) driverSeed(path string, seeds map[string]Witness) bool {
	if _, present := graph.claimants[DriverNamespace]; !present || !DriverInput(path) {
		return false
	}
	revision, known := graph.driverContent[path]
	ids := graph.driverFiles[path]
	if len(ids) == 0 {
		if action := graph.driverAction(path); action != "" {
			ids = []string{action}
		}
	}
	add := func(id string) {
		if _, already := seeds[id]; !already {
			seeds[id] = Witness{Kind: WitnessDirectSource, DirtyPath: path, Via: []string{id}}
		}
	}
	all := func(values []string) bool {
		for _, id := range values {
			add(id)
		}
		return true
	}
	// A deleted file or a removed target or job leaves whatever invoked it
	// unresolved in the head graph, so every driver unit is selected.
	every := func() bool {
		for _, id := range graph.order {
			if strings.HasPrefix(id, DriverNamespace+":") {
				add(id)
			}
		}
		return true
	}
	if known && !revision.HeadExists {
		if !revision.BaseExists {
			return len(ids) != 0 && all(ids)
		}
		return every()
	}
	if len(ids) == 0 {
		return false
	}
	if !known || !revision.BaseExists || strings.HasPrefix(path, ".github/actions/") {
		return all(ids)
	}
	var base, head map[string]string
	var baseRest, headRest string
	var baseOK, headOK bool
	if strings.HasPrefix(path, ".github/workflows/") {
		base, baseRest, baseOK = driverWorkflowBlocks(path, revision.Base)
		head, headRest, headOK = driverWorkflowBlocks(path, revision.Head)
	} else {
		base, baseRest, baseOK = driverMakeBlocks(path, revision.Base)
		head, headRest, headOK = driverMakeBlocks(path, revision.Head)
	}
	if !baseOK || !headOK || baseRest != headRest {
		return all(ids)
	}
	for id := range base {
		if _, kept := head[id]; !kept {
			return every()
		}
	}
	for _, id := range ids {
		if text, present := head[id]; !present || base[id] != text {
			add(id)
		}
	}
	return true
}

// driverAction is the unit of the local action whose directory holds path.
func (graph *Graph) driverAction(path string) string {
	for directory := relativeDirectory(path); directory != "." && directory != ".github/actions"; directory = relativeDirectory(directory) {
		id := driverActionUnitID(directory)
		if _, present := graph.units[id]; present {
			return id
		}
	}
	return ""
}

func driverWorkflowBlocks(file string, body []byte) (map[string]string, string, bool) {
	workflow := parseDriverWorkflow(file, body)
	if workflow.unparsable {
		return nil, "", false
	}
	blocks := make(map[string]string, len(workflow.blocks))
	for job, text := range workflow.blocks {
		blocks[driverWorkflowUnitID(file, job)] = text
	}
	return blocks, workflow.remainder, true
}

// driverSelectable reports a driver unit a plan can select: an explicit make
// target or a parsed workflow job.
func driverSelectable(id string) bool {
	rest, ok := strings.CutPrefix(id, DriverNamespace+":")
	if !ok {
		return false
	}
	_, target, found := strings.Cut(rest, "#")
	if !found || target == "*" {
		return false
	}
	return strings.HasPrefix(rest, "ci:") || strings.HasPrefix(rest, "make:") && !strings.Contains(target, "%")
}

// driverSelection is the selection of a reached driver unit: a make target
// with the argv that runs it, or a workflow job left to CI.
func (graph *Graph) driverSelection(id string, witness Witness) (Selection, bool) {
	if !driverSelectable(id) {
		return Selection{}, false
	}
	rest := strings.TrimPrefix(id, DriverNamespace+":")
	selection := Selection{UnitID: id, Tests: []string{}, Witness: witness}
	if file, ok := strings.CutPrefix(rest, "ci:"); ok {
		workflow, job, _ := strings.Cut(file, "#")
		selection.Kind, selection.Workflow, selection.Job = SelectionKindCIJob, workflow, job
		return selection, true
	}
	file, target, _ := strings.Cut(strings.TrimPrefix(rest, "make:"), "#")
	directory, base := path.Dir(file), path.Base(file)
	argv := []string{"make"}
	if directory != "." {
		argv = append(argv, "-C", directory)
	}
	if graph.driverDefaultFile(directory) != file {
		argv = append(argv, "-f", base)
	}
	selection.Kind, selection.Argv = SelectionKindDriverTarget, append(argv, target)
	return selection, true
}

// driverDefaultFile is the make file GNU make reads in directory when none is
// named.
func (graph *Graph) driverDefaultFile(directory string) string {
	for _, name := range []string{"GNUmakefile", "makefile", "Makefile"} {
		candidate := name
		if directory != "." {
			candidate = directory + "/" + name
		}
		if len(graph.driverFiles[candidate]) != 0 {
			return candidate
		}
	}
	return ""
}
