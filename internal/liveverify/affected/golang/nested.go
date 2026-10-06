package golang

import (
	"errors"
	"fmt"
	"io/fs"
	"path"
	"strconv"
	"strings"

	"github.com/Beamfall/corvint/internal/liveverify/affected"
)

// maxNestedManifestBytes bounds one nested go.mod this plugin reads to decide
// whether that module can reach an observed module (V1-0867); tests lower it.
var maxNestedManifestBytes int64 = 1 << 20

// nestedModule is the evidence for one go.mod below the root that no go.work
// use lists: the manifest read through the plugin's own source, and why it
// keeps the nested-module frontier open. An empty Open closes the frontier for
// that module: its packages cannot import a package of an observed module, so
// no change to an observed module's source can reach them through an import.
type nestedModule struct {
	Manifest string
	Open     string
}

// nestedModules decides, per unlisted module, whether its packages can import
// an observed module (AFP-V0-008, V1-0867). A module stays open when its
// manifest cannot be read whole within the bound or parsed, when it requires,
// replaces, or names a tool under an observed module path (other than one of
// its own packages), when a replace names a
// directory that is not another unlisted module of this repository (a relative
// replace that resolves to the root or an observed module is one), when a
// go.work at or above it below the root could add an observed module to its
// build, or when an observed module's own path is unknown, since then no
// requirement can be ruled out. The reverse direction is untouched: a nested
// module's own files are never units, so a change inside one is never a
// bounded plan.
func nestedModules(root *affected.Source, directories []string, modules map[string]module) []nestedModule {
	nested := make(map[string]bool, len(directories))
	for _, directory := range directories {
		nested[directory] = true
	}
	observed, unresolved := observedModulePaths(modules)
	evidence := make([]nestedModule, 0, len(directories))
	for _, directory := range directories {
		open := "an observed module path is unresolved"
		if !unresolved {
			open = nestedModuleOpen(root, directory, observed, nested)
		}
		evidence = append(evidence, nestedModule{Manifest: path.Join(directory, "go.mod"), Open: open})
	}
	return evidence
}

// observedModulePaths lists the observed module paths, and reports whether any
// observed module's path could not be read.
func observedModulePaths(modules map[string]module) (map[string]bool, bool) {
	paths := make(map[string]bool, len(modules))
	for _, owner := range modules {
		if !owner.listed {
			continue
		}
		if owner.path == "" {
			return nil, true
		}
		paths[owner.path] = true
	}
	return paths, false
}

func nestedModuleOpen(root *affected.Source, directory string, observed, nested map[string]bool) string {
	if reason := workspaceAbove(root, directory); reason != "" {
		return reason
	}
	manifest := path.Join(directory, "go.mod")
	info, err := root.Stat(manifest)
	if err != nil {
		return "unreadable: " + err.Error()
	}
	if info.Size() > maxNestedManifestBytes {
		return fmt.Sprintf("over-size: %d bytes above the %d byte bound", info.Size(), maxNestedManifestBytes)
	}
	body, err := root.Read(manifest)
	if err != nil {
		return "unreadable: " + err.Error()
	}
	if int64(len(body)) > maxNestedManifestBytes {
		return fmt.Sprintf("over-size: %d bytes above the %d byte bound", len(body), maxNestedManifestBytes)
	}
	parsed, err := parseManifest(body)
	if err != nil {
		return "unparsable: " + err.Error()
	}
	for _, required := range parsed.requires {
		if observed[required] {
			return "requires " + required
		}
	}
	for _, tool := range parsed.tools {
		if underModule(tool, sortedKeys(observed)) && !underModule(tool, []string{parsed.module}) {
			return "names the tool " + tool
		}
	}
	for _, replacement := range parsed.replaces {
		if observed[replacement.old] {
			return "replaces " + replacement.old
		}
		if !replacement.directory {
			if observed[replacement.target] {
				return "replaces " + replacement.old + " with " + replacement.target
			}
			continue
		}
		if reason := replaceDirectoryOpen(directory, replacement.target, nested); reason != "" {
			return reason
		}
	}
	return ""
}

// workspaceAbove names a go.work in directory or any ancestor below the root.
// The go tool finds the nearest one upward, and a workspace can add an
// observed module to the nested module's build list without a require line.
func workspaceAbove(root *affected.Source, directory string) string {
	for current := directory; current != "." && current != ""; current = path.Dir(current) {
		workspace := path.Join(current, "go.work")
		_, err := root.Stat(workspace)
		if errors.Is(err, fs.ErrNotExist) {
			continue
		}
		if err != nil {
			return "unreadable workspace " + workspace + ": " + err.Error()
		}
		return "workspace " + workspace + " may use an observed module"
	}
	return ""
}

// replaceDirectoryOpen resolves a directory replacement against the nested
// module's directory. Only a target that is another unlisted module of this
// repository stays closed, since that module's own manifest is judged too.
func replaceDirectoryOpen(directory, target string, nested map[string]bool) string {
	if strings.HasPrefix(target, "/") || strings.Contains(target, `\`) || (len(target) > 1 && target[1] == ':') {
		return "replace target " + target + " is not a repository-relative directory"
	}
	resolved := path.Join(directory, target)
	if resolved == ".." || strings.HasPrefix(resolved, "../") {
		return "replace target " + target + " lies outside the repository"
	}
	if resolved == directory || nested[resolved] {
		return ""
	}
	return "replace target " + target + " resolves to " + resolved + ", which is not an unlisted module"
}

// manifestReplace is one replace directive: the replaced module path and the
// replacement, which is a directory when it carries no version or is written
// as one.
type manifestReplace struct {
	old       string
	target    string
	directory bool
}

// manifestDependencies is what a nested go.mod says about the modules its
// packages may import.
type manifestDependencies struct {
	module   string
	requires []string
	replaces []manifestReplace
	tools    []string
}

// manifestToken is one lexical token of a go.mod line. A quoted token is never
// a parenthesis or an arrow.
type manifestToken struct {
	text   string
	quoted bool
}

func (token manifestToken) is(text string) bool { return !token.quoted && token.text == text }

// The go.mod verbs this reader knows. Any other verb is unparsable, so a
// directive a later toolchain adds keeps the frontier open.
var manifestVerbs = map[string]bool{
	"module": true, "go": true, "toolchain": true, "godebug": true, "require": true,
	"exclude": true, "replace": true, "retract": true, "tool": true, "ignore": true,
}

// parseManifest reads the go.mod grammar this decision needs: line comments,
// interpreted and raw strings, one-line directives, and parenthesized blocks.
// It fails rather than guessing on anything else.
func parseManifest(body []byte) (manifestDependencies, error) {
	var parsed manifestDependencies
	block := ""
	for number, line := range strings.Split(string(body), "\n") {
		tokens, err := manifestLine(line)
		if err != nil {
			return manifestDependencies{}, fmt.Errorf("line %d: %w", number+1, err)
		}
		if len(tokens) == 0 {
			continue
		}
		verb, args := block, tokens
		switch {
		case block != "" && tokens[0].is(")"):
			if len(tokens) != 1 {
				return manifestDependencies{}, fmt.Errorf("line %d: text after a block close", number+1)
			}
			block = ""
			continue
		case block == "":
			if tokens[0].quoted || !manifestVerbs[tokens[0].text] {
				return manifestDependencies{}, fmt.Errorf("line %d: unknown directive %q", number+1, tokens[0].text)
			}
			verb, args = tokens[0].text, tokens[1:]
			if len(args) == 1 && args[0].is("(") {
				if verb == "module" {
					return manifestDependencies{}, fmt.Errorf("line %d: module block", number+1)
				}
				block = verb
				continue
			}
		}
		for _, arg := range args {
			if arg.is("(") || arg.is(")") {
				return manifestDependencies{}, fmt.Errorf("line %d: misplaced parenthesis", number+1)
			}
		}
		if err := parsed.add(verb, args); err != nil {
			return manifestDependencies{}, fmt.Errorf("line %d: %w", number+1, err)
		}
	}
	if block != "" {
		return manifestDependencies{}, errors.New("unterminated " + block + " block")
	}
	if parsed.module == "" {
		return manifestDependencies{}, errors.New("no module directive")
	}
	return parsed, nil
}

func (parsed *manifestDependencies) add(verb string, args []manifestToken) error {
	switch verb {
	case "module":
		if parsed.module != "" || len(args) != 1 || args[0].text == "" {
			return errors.New("malformed module directive")
		}
		parsed.module = args[0].text
	case "require":
		if len(args) != 2 {
			return errors.New("malformed require directive")
		}
		parsed.requires = append(parsed.requires, args[0].text)
	case "tool":
		if len(args) != 1 {
			return errors.New("malformed tool directive")
		}
		parsed.tools = append(parsed.tools, args[0].text)
	case "replace":
		replacement, err := manifestReplacement(args)
		if err != nil {
			return err
		}
		parsed.replaces = append(parsed.replaces, replacement)
	}
	return nil
}

// manifestReplacement reads `old [version] => new [version]`. A replacement
// without a version, or one written as a directory path, is a directory.
func manifestReplacement(args []manifestToken) (manifestReplace, error) {
	arrow := -1
	for index, arg := range args {
		if arg.is("=>") {
			arrow = index
			break
		}
	}
	right := len(args) - arrow - 1
	if arrow < 1 || arrow > 2 || right < 1 || right > 2 {
		return manifestReplace{}, errors.New("malformed replace directive")
	}
	target := args[arrow+1].text
	return manifestReplace{old: args[0].text, target: target, directory: right == 1 || directoryPath(target)}, nil
}

// directoryPath mirrors the go tool's test for a local replacement path.
func directoryPath(value string) bool {
	return value == "." || value == ".." || strings.HasPrefix(value, "./") || strings.HasPrefix(value, "../") ||
		strings.HasPrefix(value, "/") || strings.HasPrefix(value, `.\`) || strings.HasPrefix(value, `..\`) ||
		(len(value) > 1 && value[1] == ':')
}

// manifestLine splits one go.mod line into tokens, dropping a // comment.
func manifestLine(line string) ([]manifestToken, error) {
	tokens := make([]manifestToken, 0, 4)
	for index := 0; index < len(line); {
		switch character := line[index]; {
		case character == ' ' || character == '\t' || character == '\r':
			index++
		case strings.HasPrefix(line[index:], "//"):
			return tokens, nil
		case character == '(' || character == ')':
			tokens = append(tokens, manifestToken{text: string(character)})
			index++
		case character == '"' || character == '`':
			end := quotedEnd(line, index)
			if end < 0 {
				return nil, errors.New("unterminated string")
			}
			value, err := strconv.Unquote(line[index : end+1])
			if err != nil {
				return nil, fmt.Errorf("invalid string: %w", err)
			}
			tokens = append(tokens, manifestToken{text: value, quoted: true})
			index = end + 1
		default:
			end := index
			for end < len(line) && !strings.ContainsRune(" \t\r()\"`", rune(line[end])) && !strings.HasPrefix(line[end:], "//") {
				end++
			}
			tokens = append(tokens, manifestToken{text: line[index:end]})
			index = end
		}
	}
	return tokens, nil
}

// quotedEnd is the index of the quote that closes the string opening at start,
// or -1 when the line ends first.
func quotedEnd(line string, start int) int {
	quote := line[start]
	for index := start + 1; index < len(line); index++ {
		switch {
		case line[index] == quote:
			return index
		case quote == '"' && line[index] == '\\':
			index++
		}
	}
	return -1
}
