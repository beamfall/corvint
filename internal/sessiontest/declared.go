package sessiontest

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

// Declared go-test recipe states (STR-V0-003, STR-V0-006).
const (
	// GoTestAbsent: the Makefile names no go-test target.
	GoTestAbsent = "absent"
	// GoTestParsed: one typed `[NAME=VALUE...] go test FLAGS ./...` recipe line.
	GoTestParsed = "parsed"
	// GoTestUnparsed: a go-test target this reader cannot type; only
	// `make go-test` reproduces it, so Go runs FULL through it.
	GoTestUnparsed = "unparsed"
)

const (
	maxMakefileBytes = 1 << 20
	maxExpandDepth   = 16
)

// Declared is the repository's declared full scope, read from the root
// Makefile independently of the planner (STR-V0-003). Languages names each
// non-Go language with a `<language>-test` target.
type Declared struct {
	GoEnv      []string `json:"goEnv"`
	GoFlags    []string `json:"goFlags"`
	GoTest     string   `json:"goTest"`
	GoTestNote string   `json:"goTestNote"`
	Languages  []string `json:"languages"`
	Unreadable string   `json:"unreadable"`
}

// Names reports whether anything full-suite is declared.
func (d Declared) Names() bool { return d.GoTest != GoTestAbsent || len(d.Languages) != 0 }

var (
	makeAssignment = regexp.MustCompile(`^([A-Za-z_][A-Za-z0-9_]*)[ \t]*(\?=|::=|:=|=)[ \t]*(.*)$`)
	makeRule       = regexp.MustCompile(`^([A-Za-z0-9_.-]+)[ \t]*:([^=]|$)`)
	makeReference  = regexp.MustCompile(`\$[({]([A-Za-z_][A-Za-z0-9_]*)[)}]`)
	envAssignment  = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*=`)
	// Anything a shell would interpret makes the recipe untyped.
	shellMeta = "'\"\\;|&<>`$(){}[]*?~#!\n\r"
	// The adapter adds these itself; a recipe that sets them is untyped.
	reservedGoFlags = []string{"-json", "-run", "-skip"}
)

// ReadDeclared reads root/Makefile. languages are the non-Go language names a
// `<language>-test` target may declare; lookup is the environment `?=` defers
// to, as make does.
func ReadDeclared(root string, languages []string, lookup func(string) (string, bool)) Declared {
	declared := Declared{GoEnv: []string{}, GoFlags: []string{}, GoTest: GoTestAbsent, Languages: []string{}}
	file, err := os.Open(filepath.Join(root, "Makefile"))
	if errors.Is(err, fs.ErrNotExist) {
		return declared
	}
	if err != nil {
		declared.Unreadable = "Makefile is unreadable"
		return declared
	}
	defer file.Close()
	raw, err := io.ReadAll(io.LimitReader(file, maxMakefileBytes+1))
	if err != nil || len(raw) > maxMakefileBytes {
		declared.Unreadable = fmt.Sprintf("Makefile is unreadable or exceeds %d bytes", maxMakefileBytes)
		return declared
	}
	lines := logicalLines(string(raw))
	variables := map[string]string{}
	wanted := map[string]bool{}
	for _, language := range languages {
		wanted[language+"-test"] = true
	}
	var recipe []string
	inGoTest := false
	// depth counts open conditionals; a variable assigned under one, or a
	// directive that changes what a recipe sees, cannot be evaluated here.
	depth, untyped := 0, ""
	conditional := map[string]bool{}
	for _, line := range lines {
		if strings.HasPrefix(line, "\t") {
			if inGoTest {
				if body := strings.TrimSpace(line); body != "" && !strings.HasPrefix(body, "#") {
					recipe = append(recipe, body)
				}
			}
			continue
		}
		inGoTest = false
		if strings.TrimSpace(line) == "" || strings.HasPrefix(strings.TrimSpace(line), "#") {
			continue
		}
		directive, _, _ := strings.Cut(strings.TrimSpace(line), " ")
		switch directive {
		case "ifeq", "ifneq", "ifdef", "ifndef":
			depth++
			continue
		case "endif":
			depth--
			continue
		case "else":
			continue
		case "export", "unexport", "override", "include", "-include", "sinclude", "define", ".EXPORT_ALL_VARIABLES:":
			if untyped == "" {
				untyped = "the Makefile uses " + strings.TrimSuffix(directive, ":") + ", which this reader cannot evaluate"
			}
			continue
		}
		if match := makeAssignment.FindStringSubmatch(line); match != nil {
			name, operator, value := match[1], match[2], strings.TrimSpace(stripMakeComment(match[3]))
			if depth > 0 {
				conditional[name] = true
			}
			if operator == "?=" {
				if environment, ok := lookup(name); ok {
					value = environment
				} else if _, defined := variables[name]; defined {
					continue
				}
			}
			variables[name] = value
			continue
		}
		if match := makeRule.FindStringSubmatch(line); match != nil {
			switch target := match[1]; {
			case target == "go-test":
				declared.GoTest = GoTestUnparsed
				inGoTest = true
				if depth > 0 && untyped == "" {
					untyped = "go-test is defined under a conditional"
				}
			case wanted[target]:
				declared.Languages = appendUnique(declared.Languages, strings.TrimSuffix(target, "-test"))
			}
		}
	}
	sort.Strings(declared.Languages)
	if declared.GoTest == GoTestUnparsed {
		for name := range conditional {
			delete(variables, name) // expandMake then refuses any reference to it
		}
		env, flags, note := typeGoRecipe(recipe, variables)
		if untyped != "" {
			note = untyped
		}
		if note == "" {
			declared.GoTest, declared.GoEnv, declared.GoFlags = GoTestParsed, env, flags
		}
		declared.GoTestNote = note
	}
	return declared
}

// logicalLines joins backslash continuations, as make does for every line.
func logicalLines(body string) []string {
	var lines []string
	current := ""
	for _, line := range strings.Split(strings.ReplaceAll(body, "\r\n", "\n"), "\n") {
		if strings.HasSuffix(line, "\\") {
			current += strings.TrimSuffix(line, "\\") + " "
			continue
		}
		lines = append(lines, current+line)
		current = ""
	}
	if current != "" {
		lines = append(lines, current)
	}
	return lines
}

func stripMakeComment(value string) string {
	if index := strings.Index(value, "#"); index >= 0 {
		return value[:index]
	}
	return value
}

func appendUnique(values []string, value string) []string {
	for _, existing := range values {
		if existing == value {
			return values
		}
	}
	return append(values, value)
}

// typeGoRecipe types the one go-test recipe line, or names why it cannot.
func typeGoRecipe(recipe []string, variables map[string]string) (env, flags []string, note string) {
	if len(recipe) != 1 {
		return nil, nil, fmt.Sprintf("go-test has %d recipe lines, want exactly one", len(recipe))
	}
	line := strings.TrimPrefix(recipe[0], "@")
	expanded, err := expandMake(line, variables, 0)
	if err != nil {
		return nil, nil, err.Error()
	}
	if strings.ContainsAny(expanded, shellMeta) {
		return nil, nil, "go-test recipe needs a shell"
	}
	fields := strings.Fields(expanded)
	env = []string{}
	for len(fields) > 0 && envAssignment.MatchString(fields[0]) {
		env = append(env, fields[0])
		fields = fields[1:]
	}
	if len(fields) < 3 || fields[0] != "go" || fields[1] != "test" || fields[len(fields)-1] != "./..." {
		return nil, nil, "go-test recipe is not `[NAME=VALUE...] go test FLAGS ./...`"
	}
	flags = append([]string{}, fields[2:len(fields)-1]...)
	for _, flag := range flags {
		if flag == "./..." {
			return nil, nil, "go-test recipe names ./... twice"
		}
		for _, reserved := range reservedGoFlags {
			if flag == reserved || strings.HasPrefix(flag, reserved+"=") {
				return nil, nil, "go-test recipe sets " + reserved + ", which the adapter owns"
			}
		}
	}
	if len(flags) != 0 && !strings.HasPrefix(flags[0], "-") {
		return nil, nil, "go-test recipe names a target before its flags"
	}
	return env, flags, ""
}

// expandMake expands $(NAME) and ${NAME} from the Makefile's own variables;
// $$ is a literal dollar, which typeGoRecipe then refuses. Anything else, an
// undefined name or a make function, leaves the recipe untyped.
func expandMake(value string, variables map[string]string, depth int) (string, error) {
	if depth > maxExpandDepth {
		return "", errors.New("go-test recipe variables nest too deeply")
	}
	var out strings.Builder
	for index := 0; index < len(value); {
		if value[index] != '$' {
			out.WriteByte(value[index])
			index++
			continue
		}
		if strings.HasPrefix(value[index:], "$$") {
			out.WriteString("$")
			index += 2
			continue
		}
		match := makeReference.FindStringSubmatchIndex(value[index:])
		if match == nil || match[0] != 0 {
			return "", errors.New("go-test recipe uses a make function or a one-letter variable")
		}
		name := value[index+match[2] : index+match[3]]
		definition, defined := variables[name]
		if !defined {
			return "", fmt.Errorf("go-test recipe names undefined variable %s", name)
		}
		expanded, err := expandMake(definition, variables, depth+1)
		if err != nil {
			return "", err
		}
		out.WriteString(expanded)
		index += match[1]
	}
	return out.String(), nil
}
