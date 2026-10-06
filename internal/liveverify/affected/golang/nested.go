package golang

import (
	"errors"
	"fmt"
	"io/fs"
	"path"
	"reflect"
	"regexp"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"

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

// nestedModules decides, per unlisted module, whether its packages can take
// part in a build of an observed module (AFP-V0-028, V1-0867). A module stays
// open when its manifest cannot be read whole within the bound or parsed; when
// it requires, replaces, or names a tool under an observed module path (other
// than one of its own packages); when a replace names a directory that is not
// another unlisted module of this repository (a relative replace that resolves
// to the root or an observed module is one); when a go.work at or above it
// below the root could add an observed module to its build; when an observed
// manifest or the root go.work requires, replaces, or names a tool under its
// module path, or replaces a module with its directory, since the observed
// build then compiles its packages against the observed modules; or when an
// observed manifest or the root go.work cannot be read and parsed the same way,
// or its module path disagrees with the one read, since then nothing can be
// ruled out. The reverse direction is untouched: a nested module's own files
// are never units, so a change inside one is never a bounded plan.
func nestedModules(root *affected.Source, directories []string, modules map[string]module) []nestedModule {
	nested := make(map[string]bool, len(directories))
	for _, directory := range directories {
		nested[directory] = true
	}
	observed, reach, unresolved := observedReach(root, modules, nested)
	evidence := make([]nestedModule, 0, len(directories))
	for _, directory := range directories {
		open := unresolved
		if open == "" {
			open = nestedModuleOpen(root, directory, observed, reach, nested)
		}
		evidence = append(evidence, nestedModule{Manifest: path.Join(directory, "go.mod"), Open: open})
	}
	return evidence
}

// incoming is what the observed manifests and the root go.work say about
// other modules: each module path they require or replace, or replace a module
// with, the tools they name, and each repository directory a replacement
// names, with the manifest that says so.
type incoming struct {
	paths       map[string]string
	tools       map[string]string
	directories map[string]string
}

// observedReach parses every observed manifest, and the root go.work when
// there is one, with the reader used for nested manifests. It returns the
// observed module paths, what those files draw into the observed build, and a
// reason when any of them leaves that unresolved.
func observedReach(root *affected.Source, modules map[string]module, nested map[string]bool) (map[string]bool, incoming, string) {
	reach := incoming{paths: map[string]string{}, tools: map[string]string{}, directories: map[string]string{}}
	observed := make(map[string]bool, len(modules))
	listed := make(map[string]bool, len(modules))
	for _, directory := range sortedKeys(modulesByDirectory(modules)) {
		owner := modules[directory]
		if owner.path == "" {
			return nil, reach, "an observed module path is unresolved"
		}
		manifest := path.Join(directory, "go.mod")
		parsed, reason := readManifest(root, manifest, false)
		if reason != "" {
			return nil, reach, "observed manifest " + manifest + " is " + reason
		}
		if parsed.module != owner.path {
			return nil, reach, "observed manifest " + manifest + " declares " + strconv.Quote(parsed.module) + ", read as " + strconv.Quote(owner.path)
		}
		if reason := reach.add(manifest, directory, parsed); reason != "" {
			return nil, reach, reason
		}
		observed[parsed.module] = true
		listed[directory] = true
	}
	_, err := root.Stat("go.work")
	if errors.Is(err, fs.ErrNotExist) {
		return observed, reach, unknownDirectory(reach, listed, nested)
	}
	if err != nil {
		return nil, reach, "root go.work is unreadable: " + err.Error()
	}
	work, reason := readManifest(root, "go.work", true)
	if reason != "" {
		return nil, reach, "root go.work is " + reason
	}
	uses := map[string]bool{}
	for _, use := range work.uses {
		if resolved := path.Clean(use); resolved == "." || affected.ValidRelativePath(resolved) {
			uses[resolved] = true
		}
	}
	if len(uses) == 0 || !reflect.DeepEqual(uses, listed) {
		return nil, reach, fmt.Sprintf("root go.work uses %q, observed as %q", sortedKeys(uses), sortedKeys(listed))
	}
	if reason := reach.add("go.work", ".", work); reason != "" {
		return nil, reach, reason
	}
	return observed, reach, unknownDirectory(reach, listed, nested)
}

// unknownDirectory names an observed directory replacement that is neither
// exactly an observed module directory nor at or below an unlisted module's
// directory. Its identity is then not established, since a differently cased
// or linked path can name a nested module, so nothing can be ruled out.
func unknownDirectory(reach incoming, listed, nested map[string]bool) string {
	for _, target := range sortedKeys(reach.directories) {
		if listed[target] {
			continue
		}
		known := false
		for current := target; current != "." && current != ""; current = path.Dir(current) {
			if nested[current] {
				known = true
				break
			}
		}
		if !known {
			return reach.directories[target] + ", which names no known module directory"
		}
	}
	return ""
}

// modulesByDirectory is the set of observed module directories.
func modulesByDirectory(modules map[string]module) map[string]bool {
	directories := make(map[string]bool, len(modules))
	for directory, owner := range modules {
		if owner.listed {
			directories[directory] = true
		}
	}
	return directories
}

// add records what one observed file draws into the observed build. A
// directory replacement that is not repository-relative, or leaves the
// repository, could name any directory, so it leaves the decision unresolved.
func (reach incoming) add(file, directory string, parsed manifestDependencies) string {
	for _, required := range parsed.requires {
		note(reach.paths, required, file+" requires "+required)
	}
	for _, tool := range parsed.tools {
		note(reach.tools, tool, file+" names the tool "+tool)
	}
	for _, replacement := range parsed.replaces {
		note(reach.paths, replacement.old, file+" replaces "+replacement.old)
		if !replacement.directory {
			note(reach.paths, replacement.target, file+" replaces "+replacement.old+" with "+replacement.target)
			continue
		}
		resolved, reason := repositoryDirectory(directory, replacement.target)
		if reason != "" {
			return file + ": " + reason
		}
		note(reach.directories, resolved, file+" replaces "+replacement.old+" with "+replacement.target)
	}
	return ""
}

// note keeps the first reason recorded for a key.
func note(reasons map[string]string, key, reason string) {
	if _, known := reasons[key]; !known {
		reasons[key] = reason
	}
}

func nestedModuleOpen(root *affected.Source, directory string, observed map[string]bool, reach incoming, nested map[string]bool) string {
	if reason := workspaceAbove(root, directory); reason != "" {
		return reason
	}
	parsed, reason := readManifest(root, path.Join(directory, "go.mod"), false)
	if reason != "" {
		return reason
	}
	if reason, drawn := reach.paths[parsed.module]; drawn {
		return reason
	}
	for _, tool := range sortedKeys(reach.tools) {
		if underModule(tool, []string{parsed.module}) {
			return reach.tools[tool]
		}
	}
	for _, target := range sortedKeys(reach.directories) {
		if target == directory || strings.HasPrefix(target, directory+"/") {
			return reach.directories[target]
		}
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

// readManifest reads one go.mod, or go.work when work is set, whole within
// the bound, and parses it; the reason says why it could not.
func readManifest(root *affected.Source, name string, work bool) (manifestDependencies, string) {
	info, err := root.Stat(name)
	if err != nil {
		return manifestDependencies{}, "unreadable: " + err.Error()
	}
	if info.Size() > maxNestedManifestBytes {
		return manifestDependencies{}, fmt.Sprintf("over-size: %d bytes above the %d byte bound", info.Size(), maxNestedManifestBytes)
	}
	body, err := root.Read(name)
	if err != nil {
		return manifestDependencies{}, "unreadable: " + err.Error()
	}
	if int64(len(body)) > maxNestedManifestBytes {
		return manifestDependencies{}, fmt.Sprintf("over-size: %d bytes above the %d byte bound", len(body), maxNestedManifestBytes)
	}
	parsed, err := parseManifest(body, work)
	if err != nil {
		return manifestDependencies{}, "unparsable: " + err.Error()
	}
	return parsed, ""
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
	resolved, reason := repositoryDirectory(directory, target)
	if reason != "" {
		return reason
	}
	if resolved == directory || nested[resolved] {
		return ""
	}
	return "replace target " + target + " resolves to " + resolved + ", which is not an unlisted module"
}

// repositoryDirectory resolves a replacement directory written in the
// manifest of the module at directory to a repository-relative path.
func repositoryDirectory(directory, target string) (string, string) {
	if strings.HasPrefix(target, "/") || strings.Contains(target, `\`) || (len(target) > 1 && target[1] == ':') {
		return "", "replace target " + target + " is not a repository-relative directory"
	}
	resolved := path.Join(directory, target)
	if resolved == ".." || strings.HasPrefix(resolved, "../") {
		return "", "replace target " + target + " lies outside the repository"
	}
	return resolved, ""
}

// manifestReplace is one replace directive: the replaced module path and the
// replacement, which is a directory when it carries no version or is written
// as one.
type manifestReplace struct {
	old       string
	target    string
	directory bool
}

// manifestDependencies is what a go.mod (or the root go.work) says about the
// modules a build of it may draw in.
type manifestDependencies struct {
	module   string
	requires []string
	replaces []manifestReplace
	tools    []string
	uses     []string
	seen     map[string]bool
}

// manifestToken is one lexical token of a go.mod line. A quoted token is never
// punctuation or an arrow.
type manifestToken struct {
	text   string
	quoted bool
}

func (token manifestToken) is(text string) bool { return !token.quoted && token.text == text }

func (token manifestToken) punctuation() bool {
	return !token.quoted && len(token.text) == 1 && strings.Contains(manifestPunctuation, token.text)
}

// manifestPunctuation is the go.mod lexer's single-character tokens.
const manifestPunctuation = "()[]{},"

// The go.mod and go.work verbs this reader knows. Any other verb is
// unparsable, so a directive a later toolchain adds keeps the frontier open.
var (
	manifestVerbs = map[string]bool{
		"module": true, "go": true, "toolchain": true, "godebug": true, "require": true,
		"exclude": true, "replace": true, "retract": true, "tool": true, "ignore": true,
	}
	workVerbs = map[string]bool{"go": true, "toolchain": true, "godebug": true, "use": true, "replace": true}
	// manifestBlocks and workBlocks are the verbs the go tool admits as a
	// parenthesized block (a module block is refused here, more strictly).
	manifestBlocks = map[string]bool{
		"godebug": true, "require": true, "exclude": true, "replace": true, "retract": true, "tool": true, "ignore": true,
	}
	workBlocks = map[string]bool{"godebug": true, "use": true, "replace": true}
	// canonicalVersion is a canonical module version: major.minor.patch with
	// an optional prerelease and +incompatible. The go tool also admits
	// shorthand forms it canonicalizes; refusing them only keeps a module open.
	canonicalVersion = regexp.MustCompile(`^v(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)` +
		`(-(0|[1-9][0-9]*|[0-9]*[A-Za-z-][0-9A-Za-z-]*)(\.(0|[1-9][0-9]*|[0-9]*[A-Za-z-][0-9A-Za-z-]*))*)?(\+incompatible)?$`)
	// goVersion and toolchainVersion are the go tool's own forms (x/mod/modfile).
	goVersion        = regexp.MustCompile(`^([1-9][0-9]*)\.(0|[1-9][0-9]*)(\.(0|[1-9][0-9]*))?([a-z]+[0-9]+)?$`)
	toolchainVersion = regexp.MustCompile(`^default$|^go1($|\.)`)
)

// parseManifest reads the go.mod grammar this decision needs, or the go.work
// grammar when work is set: line comments, interpreted strings (a raw string
// fails), one-line directives, and the blocks the go tool admits, with each directive's
// arguments checked as the go tool checks them. It fails rather than guessing
// on anything else.
func parseManifest(body []byte, work bool) (manifestDependencies, error) {
	verbs, blocks := manifestVerbs, manifestBlocks
	if work {
		verbs, blocks = workVerbs, workBlocks
	}
	parsed := manifestDependencies{seen: map[string]bool{}}
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
			if tokens[0].quoted || !verbs[tokens[0].text] {
				return manifestDependencies{}, fmt.Errorf("line %d: unknown directive %q", number+1, tokens[0].text)
			}
			verb, args = tokens[0].text, tokens[1:]
			if len(args) == 1 && args[0].is("(") {
				if !blocks[verb] {
					return manifestDependencies{}, fmt.Errorf("line %d: %s block", number+1, verb)
				}
				block = verb
				continue
			}
		}
		if err := parsed.add(verb, args); err != nil {
			return manifestDependencies{}, fmt.Errorf("line %d: %w", number+1, err)
		}
	}
	if block != "" {
		return manifestDependencies{}, errors.New("unterminated " + block + " block")
	}
	if !work && parsed.module == "" {
		return manifestDependencies{}, errors.New("no module directive")
	}
	return parsed, nil
}

func (parsed *manifestDependencies) add(verb string, args []manifestToken) error {
	for _, arg := range args {
		if verb != "retract" && arg.punctuation() {
			return fmt.Errorf("misplaced %q in %s directive", arg.text, verb)
		}
		// The go tool reserves quotes inside an unquoted argument.
		if !arg.quoted && strings.ContainsAny(arg.text, "\"'`") {
			return fmt.Errorf("quote in %s directive", verb)
		}
	}
	malformed := errors.New("malformed " + verb + " directive")
	switch verb {
	case "module", "go", "toolchain":
		// The go tool unquotes only a module path; it matches go and toolchain
		// versions, and godebug settings below, against the raw token.
		if parsed.seen[verb] || len(args) != 1 || args[0].text == "" || (verb != "module" && args[0].quoted) {
			return malformed
		}
		parsed.seen[verb] = true
		switch {
		case verb == "module":
			parsed.module = args[0].text
		case verb == "go" && !goVersion.MatchString(args[0].text),
			verb == "toolchain" && !toolchainVersion.MatchString(args[0].text):
			return malformed
		}
	case "godebug":
		if len(args) != 1 || args[0].quoted {
			return malformed
		}
		key, _, found := strings.Cut(args[0].text, "=")
		if !found || key == "" || strings.ContainsAny(args[0].text, "\"`',") {
			return malformed
		}
	case "require", "exclude":
		if len(args) != 2 || args[0].text == "" || !version(args[1]) || !majorMatches(args[0].text, args[1].text) {
			return malformed
		}
		if verb == "require" {
			parsed.requires = append(parsed.requires, args[0].text)
		}
	case "retract":
		single := len(args) == 1 && version(args[0])
		interval := len(args) == 5 && args[0].is("[") && version(args[1]) && args[2].is(",") && version(args[3]) && args[4].is("]")
		if !single && !interval {
			return malformed
		}
	case "tool", "ignore", "use":
		if len(args) != 1 || args[0].text == "" {
			return malformed
		}
		switch verb {
		case "tool":
			parsed.tools = append(parsed.tools, args[0].text)
		case "use":
			parsed.uses = append(parsed.uses, args[0].text)
		}
	case "replace":
		replacement, err := manifestReplacement(args)
		if err != nil {
			return err
		}
		parsed.replaces = append(parsed.replaces, replacement)
	}
	return nil
}

// pathMajor is a module path's major version suffix ("/v2", ".v1" for
// gopkg.in, or "" for none), and whether the path's suffix is valid, as
// x/mod/module.SplitPathVersion reads it.
func pathMajor(modulePath string) (string, bool) {
	if strings.HasPrefix(modulePath, "gopkg.in/") {
		end := strings.TrimSuffix(modulePath, "-unstable")
		prefix := strings.TrimRight(end, "0123456789")
		if !strings.HasSuffix(prefix, ".v") || len(prefix) < 3 || len(prefix) == len(end) {
			return "", false
		}
		major := modulePath[len(prefix)-2:]
		if major[2] == '0' && major != ".v0" {
			return "", false
		}
		return major, true
	}
	start := len(modulePath)
	dot := false
	for start > 0 && (modulePath[start-1] >= '0' && modulePath[start-1] <= '9' || modulePath[start-1] == '.') {
		dot = dot || modulePath[start-1] == '.'
		start--
	}
	if start <= 1 || start == len(modulePath) || modulePath[start-1] != 'v' || modulePath[start-2] != '/' {
		return "", true
	}
	major := modulePath[start-2:]
	if dot || len(major) <= 2 || major[2] == '0' || major == "/v1" {
		return "", false
	}
	return major, true
}

// majorMatches is x/mod/module.CheckPathMajor for a canonical version: the
// version's major matches the path's suffix, or, without one, is v0 or v1
// or carries +incompatible. An invalid path suffix never matches.
func majorMatches(modulePath, canonical string) bool {
	major, ok := pathMajor(modulePath)
	if !ok {
		return false
	}
	versionMajor, _, _ := strings.Cut(canonical, ".")
	major = strings.TrimSuffix(major, "-unstable")
	switch {
	case major == "":
		return versionMajor == "v0" || versionMajor == "v1" || strings.HasSuffix(canonical, "+incompatible")
	case major == ".v1" && strings.HasPrefix(canonical, "v0.0.0-"):
		return true
	default:
		return versionMajor == major[1:]
	}
}

// version admits only a canonical module version token.
func version(token manifestToken) bool {
	return !token.punctuation() && canonicalVersion.MatchString(token.text)
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
	if arrow < 1 || arrow > 2 || right < 1 || right > 2 || args[0].text == "" || args[arrow+1].text == "" ||
		(arrow == 2 && !version(args[1])) || (right == 2 && !version(args[arrow+2])) {
		return manifestReplace{}, errors.New("malformed replace directive")
	}
	// As the go tool requires: the old path has a valid major suffix that
	// matches its version, a target without a version is a directory, and a
	// directory target has no version.
	target := args[arrow+1].text
	if _, ok := pathMajor(args[0].text); !ok || (arrow == 2 && !majorMatches(args[0].text, args[1].text)) ||
		directoryPath(target) != (right == 1) {
		return manifestReplace{}, errors.New("malformed replace directive")
	}
	return manifestReplace{old: args[0].text, target: target, directory: right == 1}, nil
}

// directoryPath mirrors the go tool's test for a local replacement path.
func directoryPath(value string) bool {
	return value == "." || value == ".." || strings.HasPrefix(value, "./") || strings.HasPrefix(value, "../") ||
		strings.HasPrefix(value, "/") || strings.HasPrefix(value, `.\`) || strings.HasPrefix(value, `..\`) ||
		(len(value) > 1 && value[1] == ':')
}

// manifestLine splits one go.mod line into tokens as the go tool's lexer
// does, dropping a // comment: space, tab and carriage return separate tokens,
// each of ()[]{}, is a token, a token starting with a quote is a string, and
// any other run of printable runes is an identifier. Anything the go tool
// rejects, or that this reader is unsure of, is an error.
func manifestLine(line string) ([]manifestToken, error) {
	tokens := make([]manifestToken, 0, 4)
	for index := 0; index < len(line); {
		character, size := utf8.DecodeRuneInString(line[index:])
		switch {
		case character == ' ' || character == '\t' || character == '\r':
			index += size
		case strings.HasPrefix(line[index:], "//"):
			return tokens, nil
		case strings.HasPrefix(line[index:], "/*"):
			return nil, errors.New("block comment")
		case strings.ContainsRune(manifestPunctuation, character):
			tokens = append(tokens, manifestToken{text: string(character)})
			index += size
		case character == '`':
			// The go tool lexes a raw string, but no directive argument admits one.
			return nil, errors.New("raw string")
		case character == '"':
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
		case !identifierRune(character, size):
			return nil, fmt.Errorf("unexpected character %q", character)
		default:
			end := index
			for end < len(line) && !strings.HasPrefix(line[end:], "//") {
				next, width := utf8.DecodeRuneInString(line[end:])
				if !identifierRune(next, width) || strings.ContainsRune(manifestPunctuation, next) {
					break
				}
				if strings.HasPrefix(line[end:], "/*") {
					return nil, errors.New("block comment")
				}
				end += width
			}
			tokens = append(tokens, manifestToken{text: line[index:end]})
			index = end
		}
	}
	return tokens, nil
}

// identifierRune is the go.mod lexer's identifier rune: printable, not space,
// and valid UTF-8.
func identifierRune(character rune, size int) bool {
	return !(character == utf8.RuneError && size == 1) && !unicode.IsSpace(character) && unicode.IsPrint(character)
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
