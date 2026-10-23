package typescript

import (
	"crypto/sha256"
	"encoding/hex"
	"sort"
	"strconv"
	"strings"
)

// Environment input sources and states (AFP-V0-382, AFP-V0-383).
const (
	// PlaywrightEnvironmentArgument is a value the caller wrote as --playwright-env NAME=VALUE.
	PlaywrightEnvironmentArgument = "ARGUMENT"
	// PlaywrightEnvironmentInvocation is a value read from the environment that invoked Corvint,
	// only for a variable the caller named with --playwright-env NAME.
	PlaywrightEnvironmentInvocation = "ENVIRONMENT"
	PlaywrightEnvironmentSet        = "SET"
	PlaywrightEnvironmentUnset      = "UNSET"
)

// PlaywrightEnvironmentInput is one caller-named environment variable the plan records. The
// value and its SHA-256 are in the plan, so a run under another value is visibly a different
// input; an UNSET variable has an empty value and digest (AFP-V0-383).
type PlaywrightEnvironmentInput struct {
	Name   string `json:"name"`
	SHA256 string `json:"sha256"`
	Source string `json:"source"`
	State  string `json:"state"`
	Value  string `json:"value"`
}

// NewPlaywrightEnvironmentInput returns the recorded form of one observed variable.
func NewPlaywrightEnvironmentInput(name, source, value string, set bool) PlaywrightEnvironmentInput {
	input := PlaywrightEnvironmentInput{Name: name, Source: source, State: PlaywrightEnvironmentUnset}
	if set {
		sum := sha256.Sum256([]byte(value))
		input.State, input.Value, input.SHA256 = PlaywrightEnvironmentSet, value, hex.EncodeToString(sum[:])
	}
	return input
}

// PlaywrightExecutable is how a recorded environment variable resolved one project's computed
// use.launchOptions.executablePath. Bundled means the expression evaluates to undefined, so
// Playwright launches its bundled browser; otherwise Path is the binary path (AFP-V0-385).
type PlaywrightExecutable struct {
	Bundled  bool   `json:"bundled"`
	Path     string `json:"path"`
	Variable string `json:"variable"`
}

// playwrightLaunchExecutable returns the executablePath value of a launchOptions object literal.
func playwrightLaunchExecutable(raw string) string {
	properties, _ := playwrightObjectProperties(raw)
	return properties["executablePath"]
}

// playwrightEnvExecutable parses expression as `process.env.NAME` or `process.env['NAME']`,
// optionally followed by `?? 'literal'` or `|| 'literal'` (AFP-V0-384). Any other form is not
// modelled.
func playwrightEnvExecutable(expression string) (name, operator, fallback string, ok bool) {
	expression = strings.TrimSpace(expression)
	// Go and JavaScript decode string escapes differently ('\U00000041' is "U00000041" and '\a'
	// is "a" in JavaScript), so a literal with any escape sequence is not modelled.
	if !strings.HasPrefix(expression, "process") || strings.ContainsRune(expression, '\\') {
		return "", "", "", false
	}
	keys, end, chained := playwrightMemberChain(expression, len("process"))
	if !chained || !playwrightEnvRead(keys) {
		return "", "", "", false
	}
	rest := strings.TrimSpace(expression[end:])
	if rest == "" {
		return keys[1], "", "", true
	}
	for _, candidate := range []string{"??", "||"} {
		if !strings.HasPrefix(rest, candidate) {
			continue
		}
		if value, literal := playwrightString(strings.TrimSpace(rest[len(candidate):])); literal {
			return keys[1], candidate, value, true
		}
	}
	return "", "", "", false
}

// playwrightEnvironmentWriter returns why something other than the caller's environment may set a
// variable before the config is evaluated, or "". The config's own writes already refuse the
// identity (TJAA-V0-042). Any module other than @playwright/test or a neutral Node built-in can
// load a dotenv file. A module loader or code evaluator reached under any name (require,
// module.require, createRequire, eval, Function, globalThis) and process.loadEnvFile can do the
// same, so the name alone keeps the unknown, however it is aliased. A globalSetup runs before the
// workers that evaluate the config again (AFP-V0-386).
func playwrightEnvironmentWriter(configPath, source string) string {
	scan := scanImportSites(configPath, source)
	if scan.err != nil || scan.dynamic >= 0 || scan.unresolved >= 0 {
		return "the config's module loads are not all static"
	}
	for _, ref := range scan.refs {
		if ref == "@playwright/test" || playwrightEnvNeutralBuiltins[strings.TrimPrefix(ref, "node:")] {
			continue
		}
		return "the config loads " + strconv.Quote(ref) + ", which can set the variable before the config reads it"
	}
	clean, err := stripComments(source, false)
	if err != nil {
		return "the config's module loads are not all static"
	}
	for _, name := range playwrightEnvLoaderNames {
		if len(playwrightTokenOffsets(clean, name)) != 0 {
			return "the config references " + name + ", which can load a module or file that sets the variable before the config reads it"
		}
	}
	if strings.Contains(clean, "globalSetup") {
		return "the config declares globalSetup, which can set the variable before workers evaluate the config"
	}
	return ""
}

// playwrightEnvNeutralBuiltins are Node built-ins, with or without the node: prefix, whose import
// does not write process.env.
var playwrightEnvNeutralBuiltins = map[string]bool{"path": true, "os": true, "url": true, "fs": true}

// playwrightEnvLoaderNames are the globals through which a config can load a module or an env
// file or evaluate code that does (AFP-V0-386).
var playwrightEnvLoaderNames = []string{"require", "createRequire", "module", "loadEnvFile", "eval", "Function", "globalThis", "global"}

// resolvePlaywrightExecutables replaces a project's computed-executablePath unknown with the
// recorded resolution when the expression is a modelled process.env read of a variable the caller
// named and no other writer can set it. Every other case keeps the EXECUTION unknown, and its
// detail names the variable and what is missing (AFP-V0-384 to AFP-V0-387).
func resolvePlaywrightExecutables(projects []PlaywrightProject, inputs []PlaywrightEnvironmentInput, configPath, source string) {
	byName := make(map[string]PlaywrightEnvironmentInput, len(inputs))
	for _, input := range inputs {
		byName[input.Name] = input
	}
	writer, channelled := "", len(playwrightTokenOffsets(source, "channel")) != 0
	if len(inputs) != 0 {
		writer = playwrightEnvironmentWriter(configPath, source)
	}
	for index := range projects {
		project := &projects[index]
		if project.executable == "" || len(project.execution) == 0 {
			continue
		}
		name, operator, fallback, modelled := playwrightEnvExecutable(project.executable)
		if !modelled {
			continue
		}
		input, observed := byName[name]
		detail := ""
		switch {
		case !observed:
			detail = "reads process.env." + name + ", which this plan did not observe; pass --playwright-env " + name + " to record it"
		case writer != "":
			detail = "reads process.env." + name + ", recorded as " + input.State + ", but " + writer
		}
		value, defined := input.Value, input.State == PlaywrightEnvironmentSet
		if detail == "" {
			switch {
			case operator == "??" && !defined, operator == "||" && value == "":
				value, defined = fallback, true
			}
			switch {
			case defined && value == "":
				detail = "reads process.env." + name + ", which evaluates to an empty string, a value the static profile does not model"
			case !defined && channelled:
				// Undefined launches the bundled browser only without a channel; a channel names an
				// installed browser whose binary the static profile does not resolve.
				detail = "reads process.env." + name + ", which evaluates to undefined, so the config's channel decides the browser binary, which the static profile does not resolve"
			}
		}
		if detail != "" {
			project.execution[0].Detail = project.Name + " use.launchOptions.executablePath " + detail
			continue
		}
		project.execution = nil
		project.Executable = &PlaywrightExecutable{Bundled: !defined, Path: value, Variable: name}
	}
}

// canonicalPlaywrightEnvironment returns the inputs sorted by name.
func canonicalPlaywrightEnvironment(inputs []PlaywrightEnvironmentInput) []PlaywrightEnvironmentInput {
	sorted := append([]PlaywrightEnvironmentInput(nil), inputs...)
	sort.Slice(sorted, func(left, right int) bool { return sorted[left].Name < sorted[right].Name })
	return sorted
}

// ValidPlaywrightEnvironmentName reports whether name is a portable environment variable name.
func ValidPlaywrightEnvironmentName(name string) bool {
	if name == "" {
		return false
	}
	for index, character := range name {
		if character != '_' && !(character >= 'A' && character <= 'Z') && !(character >= 'a' && character <= 'z') && (index == 0 || !(character >= '0' && character <= '9')) {
			return false
		}
	}
	return true
}
