package capcatalog

import (
	"fmt"
	"slices"
	"strings"
)

// WorkflowSource is one code-declared multi-command chain (CAP-V0-030): the ordered steps an
// adopter runs, each with the help text the binary prints for it. Steps whose Command is empty
// are repository states (a file to write or commit), not commands.
type WorkflowSource struct {
	Name     string
	Summary  string
	Maturity string
	Specs    []string
	Guide    string
	Steps    []WorkflowStepSource
}

// WorkflowStepSource is one step of a WorkflowSource. Help holds a summary line and the
// "Usage:", "Inputs:", "Commit:", "Produces:", "Options:" and "Example:" sections, each a header
// line followed by two-space indented lines.
type WorkflowStepSource struct {
	ID       string
	Command  string
	Optional bool
	Requires []string
	Help     string
}

// Workflow is a catalog workflow entry, derived from a WorkflowSource (CAP-V0-030, CAP-V0-031).
type Workflow struct {
	Name     string         `json:"name"`
	Summary  string         `json:"summary"`
	Maturity string         `json:"maturity"`
	Specs    []string       `json:"specs"`
	Guide    string         `json:"guide"`
	Chain    []string       `json:"chain"`
	Steps    []WorkflowStep `json:"steps"`
}

// WorkflowStep is one derived step. Command and HelpTopic are null for a repository state.
type WorkflowStep struct {
	Order         int      `json:"order"`
	ID            string   `json:"id"`
	Optional      bool     `json:"optional"`
	Command       *string  `json:"command"`
	HelpTopic     *string  `json:"helpTopic"`
	Summary       string   `json:"summary"`
	Requires      []string `json:"requires"`
	Usage         []string `json:"usage"`
	RequiredFlags []string `json:"requiredFlags"`
	Flags         []string `json:"flags"`
	SchemaIDs     []string `json:"schemaIds"`
	Inputs        []string `json:"inputs"`
	Commit        []string `json:"commit"`
	Produces      []string `json:"produces"`
	Options       []string `json:"options"`
	Example       []string `json:"example"`
}

// workflowSections are the step help sections, in the order StepHelp prints them.
var workflowSections = []string{"Usage:", "Inputs:", "Commit:", "Produces:", "Options:", "Example:"}

// BuildWorkflow derives a catalog workflow from its source. Every field is read from the source
// declaration or the step help text (CAP-V0-016).
func BuildWorkflow(source WorkflowSource) Workflow {
	workflow := Workflow{
		Name: source.Name, Summary: source.Summary, Maturity: source.Maturity, Specs: slices.Clone(source.Specs),
		Guide: source.Guide, Chain: []string{}, Steps: make([]WorkflowStep, 0, len(source.Steps)),
	}
	for index, step := range source.Steps {
		summary, sections := stepSections(step.Help)
		derived := WorkflowStep{
			Order: index + 1, ID: step.ID, Optional: step.Optional, Summary: summary,
			Requires: append([]string{}, step.Requires...), Usage: sections["Usage:"],
			Inputs: sections["Inputs:"], Commit: sections["Commit:"], Produces: sections["Produces:"],
			Options: sections["Options:"], Example: sections["Example:"],
			RequiredFlags: []string{}, SchemaIDs: schemaIDs(step.Help),
		}
		if step.Command != "" {
			derived.Command, derived.HelpTopic = stringPointer(step.Command), stringPointer(step.Command+" --help")
			// --root is the companion's own repository flag but corvint's global option.
			companion := !strings.HasPrefix(step.Command, "corvint ")
			derived.Flags = flags(strings.Join(append(slices.Clone(derived.Usage), derived.Options...), "\n"), companion)
			derived.RequiredFlags = stepRequiredFlags(step.Command, derived.Usage)
		} else {
			derived.Flags = []string{}
		}
		if !step.Optional {
			workflow.Chain = append(workflow.Chain, step.ID)
		}
		workflow.Steps = append(workflow.Steps, derived)
	}
	return workflow
}

// stepSections splits step help into its summary line and its sections' lines, with the
// two-space indent removed. A section the help does not carry is an empty list.
func stepSections(help string) (string, map[string][]string) {
	sections := map[string][]string{}
	for _, header := range workflowSections {
		sections[header] = []string{}
	}
	lines := strings.Split(strings.TrimRight(help, "\n"), "\n")
	current := ""
	for _, line := range lines[1:] {
		switch {
		case slices.Contains(workflowSections, line):
			current = line
		case line == "":
			current = ""
		case current != "":
			sections[current] = append(sections[current], strings.TrimPrefix(line, "  "))
		}
	}
	return lines[0], sections
}

// stepRequiredFlags lists the options every usage form of the step names outside brackets, with
// the same rule the command catalog uses (CAP-V0-023): a lower bound, never read from prose.
func stepRequiredFlags(command string, usage []string) []string {
	var forms []string
	for _, line := range usage {
		if strings.HasPrefix(line, " ") && len(forms) > 0 {
			forms[len(forms)-1] += " " + strings.TrimSpace(line)
			continue
		}
		forms = append(forms, line)
	}
	var required []string
	for index, form := range forms {
		words := topLevelFlags(form)
		if index == 0 {
			required = words
			continue
		}
		required = slices.DeleteFunc(required, func(flag string) bool { return !slices.Contains(words, flag) })
	}
	if required == nil {
		return []string{}
	}
	return required
}

// StepHelp renders one workflow step's help: its summary and sections, options (the step's own,
// else the caller's preformatted block, such as a flag set's defaults), and its place in the
// chain with the guide path. It returns "" for an unknown step.
func StepHelp(workflow Workflow, id, options string) string {
	position := slices.IndexFunc(workflow.Steps, func(step WorkflowStep) bool { return step.ID == id })
	if position < 0 {
		return ""
	}
	step := workflow.Steps[position]
	var text strings.Builder
	text.WriteString(step.Summary + "\n")
	for _, header := range workflowSections {
		lines := map[string][]string{"Usage:": step.Usage, "Inputs:": step.Inputs, "Commit:": step.Commit,
			"Produces:": step.Produces, "Options:": step.Options, "Example:": step.Example}[header]
		if header == "Options:" && len(lines) == 0 && options != "" {
			text.WriteString("\nOptions:\n" + strings.TrimRight(options, "\n") + "\n")
			continue
		}
		if len(lines) == 0 {
			continue
		}
		text.WriteString("\n" + header + "\n")
		for _, line := range lines {
			text.WriteString("  " + line + "\n")
		}
	}
	requires := "nothing"
	if len(step.Requires) > 0 {
		requires = strings.Join(step.Requires, ", ")
	}
	kind := "required"
	if step.Optional {
		kind = "optional"
	}
	fmt.Fprintf(&text, "\nStep %d of %d (%s; after %s) in the %s chain:\n  %s\n", step.Order, len(workflow.Steps), kind,
		requires, workflow.Name, strings.Join(workflow.Chain, " -> "))
	text.WriteString("Every step: corvint help --json (workflows). Guide: " + workflow.Guide + "\n")
	return text.String()
}
