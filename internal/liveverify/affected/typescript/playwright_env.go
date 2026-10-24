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
	// Resolved is the value the config reads after its modelled dotenv loads, present only when
	// the config makes one (AFP-V0-407).
	Resolved *PlaywrightEnvironmentResolution `json:"resolved,omitzero"`
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
// workers that evaluate the config again (AFP-V0-386). A "dotenv" load is returned as modelled
// loads instead when playwrightDotenvLoads models every use of it (AFP-V0-405, AFP-V0-408).
func playwrightEnvironmentWriter(configPath, source string) (string, []playwrightDotenvLoad) {
	scan := scanImportSites(configPath, source)
	if scan.err != nil || scan.dynamic >= 0 || scan.unresolved >= 0 {
		return "the config's module loads are not all static", nil
	}
	for _, ref := range scan.refs {
		if ref == "@playwright/test" || ref == "dotenv" || playwrightEnvNeutralBuiltins[strings.TrimPrefix(ref, "node:")] {
			continue
		}
		return "the config loads " + strconv.Quote(ref) + ", which can set the variable before the config reads it", nil
	}
	clean, err := stripComments(source, false)
	if err != nil {
		return "the config's module loads are not all static", nil
	}
	loads, residual, frontier := playwrightDotenvLoads(configPath, clean)
	if frontier != "" {
		return frontier, nil
	}
	if name, at := playwrightEnvLoaderReference(residual); at >= 0 {
		site := configPath + ":" + strconv.Itoa(1+strings.Count(residual[:at], "\n"))
		return "the config references " + name + " at " + site + ", which can load a module or file that sets the variable before the config reads it", nil
	}
	if strings.Contains(clean, "globalSetup") {
		return "the config declares globalSetup, which can set the variable before workers evaluate the config", nil
	}
	return "", loads
}

// playwrightEnvNeutralBuiltins are Node built-ins, with or without the node: prefix, whose import
// does not write process.env.
var playwrightEnvNeutralBuiltins = map[string]bool{"path": true, "os": true, "url": true, "fs": true}

// playwrightEnvLoaderNames are the globals through which a config can load a module or an env
// file or evaluate code that does (AFP-V0-386).
var playwrightEnvLoaderNames = []string{"require", "createRequire", "module", "loadEnvFile", "eval", "Function", "globalThis", "global"}

// playwrightEnvLoaderReference returns the first loader name, in playwrightEnvLoaderNames order,
// that the comment-stripped config references and the offset of its first reference, or -1. A
// name counts in code and as a whole string literal that is a property key
// (playwrightEnvKeyString); inside other string, template or regular-expression text, an array
// literal element included, it is data, such as a project named 'global-setup' (AFP-V0-417). That exemption holds only while
// the module tokenizes and no text can become code or reach a property the scan cannot see
// (playwrightReflectiveChannel); otherwise every occurrence counts, as before.
func playwrightEnvLoaderReference(clean string) (string, int) {
	lexed, ok := playwrightLex(clean)
	textIsData := ok
	if ok {
		at, _ := playwrightReflectiveChannel(clean, lexed)
		textIsData = at < 0
	}
	for _, candidate := range playwrightEnvLoaderNames {
		for _, at := range playwrightTokenOffsets(clean, candidate) {
			if !textIsData || lexed.code[at] || playwrightEnvKeyString(clean, lexed, at, len(candidate)) {
				return candidate, at
			}
		}
	}
	return "", -1
}

// playwrightEnvKeyString reports whether raw[at:at+size] is the whole content of a quoted string
// literal through which the config can read a property of that name: a computed member key
// (`box['global']`), a computed property key (`{ ['loadEnvFile']: load }`) or a quoted property
// key (`{ 'loadEnvFile': load }`), since a destructuring pattern reads through either. A whole
// string that is an array literal element (`['loadEnvFile']`) or any other operand is data
// (AFP-V0-417).
func playwrightEnvKeyString(raw string, lexed playwrightLexed, at, size int) bool {
	if at == 0 || at+size >= len(raw) || lexed.code[at-1] {
		return false
	}
	quote := raw[at-1]
	if strings.IndexByte("'\"`", quote) < 0 || raw[at+size] != quote {
		return false
	}
	previous := playwrightPreviousCode(raw, at-1)
	if previous >= 0 && raw[previous] == '[' && lexed.code[previous] {
		if playwrightMemberBracket(raw, lexed, previous) {
			return true
		}
		closeAt, closed := lexed.closes[previous]
		return !closed || playwrightPropertyKey(raw, previous, closeAt+1)
	}
	return playwrightPropertyKey(raw, at-1, at+size+1)
}

// playwrightPropertyKey reports whether raw[start:end] sits where an object literal or pattern
// key does: after a code `{` or `,` and before `:`.
func playwrightPropertyKey(raw string, start, end int) bool {
	before, after := playwrightPreviousCode(raw, start), playwrightNextCode(raw, end)
	return before >= 0 && (raw[before] == '{' || raw[before] == ',') && after < len(raw) && raw[after] == ':'
}

// resolvePlaywrightExecutables replaces a project's computed-executablePath unknown with the
// recorded resolution when the expression is a modelled process.env read of a variable the caller
// named and no other writer can set it. Every other case keeps the EXECUTION unknown, and its
// detail names the variable and what is missing (AFP-V0-384 to AFP-V0-387). Modelled dotenv
// loads read their files under root; the returned inputs carry the resolved values and the
// returned files record each read (AFP-V0-405 to AFP-V0-408).
func resolvePlaywrightExecutables(root string, projects []PlaywrightProject, inputs []PlaywrightEnvironmentInput, configPath, source string) ([]PlaywrightEnvironmentInput, []PlaywrightEnvironmentFile) {
	writer, channelled := "", len(playwrightTokenOffsets(source, "channel")) != 0
	var files []PlaywrightEnvironmentFile
	if len(inputs) != 0 {
		var loads []playwrightDotenvLoad
		writer, loads = playwrightEnvironmentWriter(configPath, source)
		if writer == "" && len(loads) != 0 {
			var values []map[string]string
			if files, values, writer = observePlaywrightDotenv(root, loads); writer == "" {
				inputs = resolvePlaywrightDotenv(inputs, files, values)
			}
		}
	}
	byName := make(map[string]PlaywrightEnvironmentInput, len(inputs))
	for _, input := range inputs {
		if input.Resolved != nil {
			input.State, input.Value = input.Resolved.State, input.Resolved.Value
		}
		byName[input.Name] = input
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
	return inputs, files
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
