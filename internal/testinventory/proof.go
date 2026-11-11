// SPDX-License-Identifier: Apache-2.0
package testinventory

import (
	"bytes"
	json "encoding/json/v2"
	"fmt"
	"path"
	"regexp"
	"slices"
	"sort"
	"strings"

	"github.com/Beamfall/corvint/internal/testid"
)

// An executing runner's instance keeps the default test-file names only
// when the reader proves it from the tree (TIV-V1-005, TIV-V1-008). Every
// other instance is unproven: Declared.Pattern names why, and the verdict
// judges each file of the runner's languages under its root that the record
// does not bind as unassigned. The proof is deliberately narrow, so a config
// the reader cannot evaluate fails closed rather than open.

// packageJSON is the npm manifest a JavaScript instance may be declared by
// (a "jest" member, or a script that runs vitest) when its directory holds no
// config file of the runner.
const packageJSON = "package.json"

// patternMembers lists, per executing runner, the config members that set or
// may move a test-file pattern, or the root it applies under. A JavaScript
// config naming one as an identifier or string, anywhere, is unproven.
// Playwright testDir is not one: it only narrows where the default names
// apply, and a testDir outside the root fails the lister.
var patternMembers = map[string][]string{
	testid.Vitest:     {"include", "includeSource", "projects", "workspace", "typecheck", "dir", "root"},
	testid.Jest:       {"testMatch", "testRegex", "projects", "preset", "rootDir", "roots"},
	testid.Playwright: {"testMatch"},
}

// runnerModules lists, per JavaScript runner, the modules a provable config
// may import: the runner's own package. A type-only import is erased and is
// not checked.
var runnerModules = map[string][]string{
	testid.Vitest:     {"vitest", "vitest/config"},
	testid.Jest:       {"jest", "@jest/types"},
	testid.Playwright: {"@playwright/test"},
}

// identityHelpers lists, per JavaScript runner, the functions a provable
// config may call: the runner's identity helper, which returns its argument.
var identityHelpers = map[string][]string{
	testid.Vitest:     {"defineConfig"},
	testid.Playwright: {"defineConfig"},
}

// workspacePrefixes are the file name prefixes Vitest auto-discovers beside
// its config as a workspace or projects file, whose members set their own
// patterns.
var workspacePrefixes = []string{"vitest.workspace.", "vitest.projects."}

// maxCauses bounds the causes Pattern lists.
const maxCauses = 4

// proof collects why an instance is unproven, in the order found.
type proof struct {
	causes []string
	seen   map[string]bool
	docs   bool
	any    bool
}

func (p *proof) add(cause string) {
	if p.seen == nil {
		p.seen = map[string]bool{}
	}
	if !p.seen[cause] {
		p.seen[cause] = true
		p.causes = append(p.causes, cause)
	}
}

// pattern is Pattern's text: the first maxCauses causes joined by "; ".
func (p *proof) pattern() string {
	if len(p.causes) <= maxCauses {
		return strings.Join(p.causes, "; ")
	}
	return strings.Join(p.causes[:maxCauses], "; ") + fmt.Sprintf("; and %d more", len(p.causes)-maxCauses)
}

// extensions is the unproven instance's candidate extension list: nil for a
// proven or static instance, ["*"] when any file may be a test.
func (p *proof) extensions(runner string) []string {
	if len(p.causes) == 0 {
		return nil
	}
	if p.any {
		return []string{"*"}
	}
	if runner == testid.Pytest {
		if p.docs {
			return []string{".py", ".txt", ".rst", ".md"}
		}
		return []string{".py"}
	}
	return jsExtensions
}

// proveDefault judges whether instance keeps the runner's default test-file
// names. It returns the unproven causes (Pattern) and the candidate
// extensions, both empty when proven. Static runners and Go are not judged.
func proveDefault(runner, root, config string, files map[string]string, read func(name string, limit int) ([]byte, error)) (string, []string) {
	var p proof
	switch runner {
	case testid.Vitest, testid.Jest, testid.Playwright:
		proveJavaScript(&p, runner, root, config, files, read)
	case testid.Pytest:
		provePytest(&p, root, config, files, read)
	default:
		return "", nil
	}
	return p.pattern(), p.extensions(runner)
}

func proveJavaScript(p *proof, runner, root, config string, files map[string]string, read func(name string, limit int) ([]byte, error)) {
	if path.Base(config) == packageJSON {
		if runner == testid.Jest {
			p.add("declared by the package.json jest member, which the reader does not evaluate")
		} else {
			p.add("declared by a package.json script that runs vitest with no vitest config")
		}
		return
	}
	data, err := read(config, maxConfigProbe)
	switch {
	case err != nil:
		p.add("an unreadable config")
	case strings.HasSuffix(config, ".json"):
		proveJSONConfig(p, runner, data)
	default:
		proveJSConfig(p, runner, data)
	}
	if runner == testid.Vitest {
		for name := range files {
			if path.Dir(name) != root {
				continue
			}
			for _, prefix := range workspacePrefixes {
				if strings.HasPrefix(path.Base(name), prefix) {
					p.add("workspace file " + name)
				}
			}
		}
	}
	if runner == testid.Jest || runner == testid.Vitest {
		proveManifests(p, runner, root, files, read)
	}
}

// proveManifests proves the package.json manifests in root and every
// directory above it: for Jest the root's has no jest member, and no script
// in any of them runs the runner with a flag outside its safe list, forwards
// a flag to it through a package manager, or runs it through a command the
// reader cannot model (scriptFlags), since a script's --testMatch, --config
// or --root moves the test files the config sets. An unreadable manifest is
// unproven.
func proveManifests(p *proof, runner, root string, files map[string]string, read func(name string, limit int) ([]byte, error)) {
	for dir := root; ; dir = path.Dir(dir) {
		manifest := path.Join(dir, packageJSON)
		if _, ok := files[manifest]; ok {
			members, err := readManifest(manifest, read)
			switch {
			case err != nil:
				p.add("an unreadable " + manifest)
			default:
				if runner == testid.Jest && dir == root && members.jest {
					p.add(manifest + " sets the jest member")
				}
				names := make([]string, 0, len(members.scripts))
				for name := range members.scripts {
					names = append(names, name)
				}
				sort.Strings(names)
				for _, name := range names {
					for _, cause := range scriptFlags(runner, members.scripts[name], members.scripts) {
						p.add(fmt.Sprintf("%s script %q %s", manifest, name, cause))
					}
				}
			}
		}
		if dir == "." {
			return
		}
	}
}

// safeScriptFlags lists, per runner, the command-line flags a package.json
// script may pass without moving the test files the config sets: output,
// reporting, coverage, watch, parallelism, timeout, snapshot and filtering
// flags. A long flag is compared lowercased with its dashes and any "no-"
// prefix removed, so --testMatch and --test-match are one flag; a Vitest
// flag may also be a dotted coverage or sequence option. Every other flag,
// --testMatch, --testRegex, --dir, --root, --rootDir, --roots, --config and
// -c among them, is unproven, and so is Jest's --runTestsByPath, which runs
// the files it names whatever their names (--findRelatedTests only narrows
// the configured files).
var safeScriptFlags = map[string]map[string]bool{
	testid.Jest: flagSet("-i", "-u", "-b", "-o", "-t", "-w", "-e",
		"ci", "coverage", "collectcoverage", "collectcoveragefrom", "coverageprovider", "coveragereporters",
		"coveragedirectory", "watch", "watchall", "runinband", "passwithnotests", "silent", "verbose",
		"detectopenhandles", "forceexit", "bail", "colors", "json", "outputfile", "reporters",
		"updatesnapshot", "testtimeout", "logheapusage", "cache", "maxworkers", "onlychanged",
		"changedsince", "lastcommit", "testnamepattern", "usestderr", "nostacktrace", "errorondeprecated",
		"expand", "notify", "randomize", "seed", "showseed", "shard", "watchman", "detectleaks", "debug",
		"findrelatedtests", "testlocationinresults", "workeridlememorylimit",
		"openhandlestimeout", "env", "testenvironment"),
	testid.Vitest: flagSet("-u", "-w", "-t",
		"run", "watch", "coverage", "reporter", "outputfile", "silent", "passwithnotests", "ui", "open",
		"globals", "environment", "pool", "threads", "isolate", "bail", "retry", "update", "testtimeout",
		"hooktimeout", "testnamepattern", "changed", "shard", "maxworkers", "minworkers", "maxconcurrency",
		"clearscreen", "hideskippedtests", "logheapusage", "allowonly", "dangerouslyignoreunhandlederrors",
		"color", "fileparallelism", "inspect", "inspectbrk", "cache", "expandsnapshotdiff",
		"printconsoletrace", "disableconsoleintercept"),
}

// safeDottedFlags lists the Vitest option groups a dotted flag may set.
var safeDottedFlags = flagSet("coverage", "sequence")

// unsafeVitestCommands are the Vitest commands that collect other files
// than its test include (benchmarks, type tests).
var unsafeVitestCommands = flagSet("bench", "typecheck")

func flagSet(names ...string) map[string]bool {
	set := map[string]bool{}
	for _, name := range names {
		set[name] = true
	}
	return set
}

// scriptFlags returns what makes a package.json script unproven for runner,
// each as the text after `script "name"` in the cause. scripts are the
// manifest's scripts, which a package-manager run may name. The script is
// split into commands at shell operators; in each:
//
//   - The runner is the first word whose base name, less a version and a
//     .js, .mjs or .cjs suffix, is the runner (jest, npx jest@29,
//     node_modules/.bin/vitest, jest/bin/jest.js); each argument after it
//     that is a flag outside safeScriptFlags, holds a shell expansion (it
//     may expand to a flag) or a backslash or brace (shellCommands models
//     neither escapes nor brace expansion), and for Vitest a bench or
//     typecheck command, is unproven. Positional arguments filter the
//     configured files and are allowed.
//   - A package-manager script run (npm, yarn, pnpm or bun, a concurrently
//     npm:name word, or npm-run-all, run-s or run-p) that forwards a
//     "-"-prefixed word, a word holding an expansion, backslash or brace, or
//     for Vitest a bench or typecheck positional, is unproven
//     (forwardedFlags), unless the script it runs is one of scripts that
//     reaches neither the runner nor a package manager. A run before the
//     runner (yarn jest, npm exec jest --) is judged on the words before the
//     runner only.
//   - With neither, a modelled wrapper (scriptWrappers: sh -c,
//     concurrently, nodemon --exec) is unproven when a word of it that
//     holds no whitespace mentions the runner (a backslash-escaped word,
//     say), and any other command that mentions the runner anywhere is a
//     wrapper the reader cannot model, and is unproven.
//   - A modelled wrapper before the runner whose words reach it is also
//     unproven for each "-"-prefixed word after a standalone "--" and each
//     {@}, {*} or {N} placeholder in a word that reaches it (wrapperArgs),
//     since concurrently passes those words into the command it runs.
//
// A word mentions the runner with any backslash removed.
//
// Every quoted word holding whitespace is also split and scanned again as a
// script, since a wrapper may run it as a command.
func scriptFlags(runner, script string, scripts map[string]string) []string {
	var found []string
	scanScript(runner, script, scripts, 0, &found)
	return found
}

// scriptWrappers are the commands that run a quoted argument as a command
// or a script.
var scriptWrappers = flagSet("sh", "bash", "zsh", "dash", "ksh", "concurrently", "nodemon", "cross-env-shell")

// maxScriptDepth bounds how deep scanScript splits quoted words again.
const maxScriptDepth = 8

func scanScript(runner, script string, scripts map[string]string, depth int, found *[]string) {
	if depth > maxScriptDepth {
		*found = append(*found, "runs "+runner+" through quoting nested too deeply")
		return
	}
	spaced := func(word string) bool { return strings.ContainsAny(word, " \t\n\r") }
	mentions := func(word string) bool { return mentionsRunner(runner, word) }
	isRunner := func(word string) bool { return !strings.HasPrefix(word, "-") && runnerWord(word) == runner }
	isWrapper := func(word string) bool { return scriptWrappers[runnerWord(word)] }
	for _, words := range shellCommands(script) {
		for _, word := range words {
			if spaced(word) {
				scanScript(runner, word, scripts, depth+1, found)
			}
		}
		// A script run before the runner forwards only the words before it;
		// the runner judges the words after it.
		run, wrapper := slices.IndexFunc(words, scriptRunWord), slices.IndexFunc(words, isWrapper)
		at := slices.IndexFunc(words, isRunner)
		if run >= 0 && (at < 0 || run < at) {
			end := len(words)
			if at >= 0 {
				end = at
			}
			*found = append(*found, forwardedFlags(runner, words[run:end], scripts)...)
		}
		switch {
		case at >= 0:
			*found = append(*found, runnerArgs(runner, words[at+1:])...)
		case run >= 0:
		case wrapper >= 0:
			for _, word := range words[wrapper+1:] {
				if !spaced(word) && mentions(word) {
					*found = append(*found, fmt.Sprintf("runs %s through %s with %s", runner, words[wrapper], word))
				}
			}
		case slices.ContainsFunc(words, mentions):
			*found = append(*found, fmt.Sprintf("runs %s through the unmodelled command %s", runner, words[0]))
		}
		if wrapper >= 0 && (at < 0 || wrapper < at) {
			*found = append(*found, wrapperArgs(runner, words[wrapper], words[wrapper+1:], depth)...)
		}
	}
}

// mentionsRunner reports whether word names runner, case aside and with any
// backslash removed (j\est is jest to the shell).
func mentionsRunner(runner, word string) bool {
	return strings.Contains(strings.ToLower(strings.ReplaceAll(word, `\`, "")), runner)
}

// argumentPlaceholder matches a concurrently or npm-run-all argument
// placeholder, which takes the words after a standalone "--".
var argumentPlaceholder = regexp.MustCompile(`\{(@|\*|[0-9]+)\}`)

// wrapperArgs judges the words after a modelled wrapper whose words reach
// runner (a word mentions it or starts a package-manager script run): each
// "-"-prefixed word after a standalone "--" may reach the runner as a flag,
// and so may the words a {@}, {*} or {N} placeholder in a word that reaches
// it takes, so either is unproven.
func wrapperArgs(runner, wrapper string, words []string, depth int) []string {
	reaches := func(word string) bool { return reachesRunner(runner, word, depth+1) }
	if !slices.ContainsFunc(words, reaches) {
		return nil
	}
	var found []string
	if dash := slices.Index(words, "--"); dash >= 0 {
		for _, word := range words[dash+1:] {
			if strings.HasPrefix(word, "-") && word != "--" {
				name, _, _ := strings.Cut(word, "=")
				found = append(found, fmt.Sprintf("runs %s through %s with %s after --", runner, wrapper, name))
			}
		}
	}
	for _, word := range words {
		if placeholder := argumentPlaceholder.FindString(word); placeholder != "" && reaches(word) {
			found = append(found, fmt.Sprintf("runs %s through %s with the placeholder %s", runner, wrapper, placeholder))
		}
	}
	return found
}

// runnerArgs judges the arguments a script passes the runner.
func runnerArgs(runner string, args []string) []string {
	var found []string
	positional := 0
	for _, arg := range args {
		switch {
		case strings.ContainsAny(arg, "$`"):
			found = append(found, "runs "+runner+" with the expansion "+arg)
		case strings.ContainsAny(arg, `\{`):
			// shellCommands models neither a backslash escape nor a brace
			// expansion: \-\-config or {--config,ci.js} may be a flag.
			found = append(found, "runs "+runner+" with the escaped or braced word "+arg)
		case arg == "--":
		case strings.HasPrefix(arg, "-"):
			if !safeScriptFlag(runner, arg) {
				name, _, _ := strings.Cut(arg, "=")
				found = append(found, "runs "+runner+" with "+name)
			}
		default:
			if positional == 0 && runner == testid.Vitest && unsafeVitestCommands[arg] {
				found = append(found, "runs "+runner+" with the "+arg+" command")
			}
			positional++
		}
	}
	return found
}

// packageManagers run a package.json script by name; runAllCommands run
// several, each argument a script name that may carry its own arguments.
var (
	packageManagers = flagSet("npm", "yarn", "pnpm", "bun")
	runAllCommands  = flagSet("npm-run-all", "run-s", "run-p")
)

// npmScriptCommands are the npm commands that run a script: the run aliases
// (the script is the next word) and the ones that name their script.
var npmScriptCommands = map[string]string{
	"run": "", "run-script": "", "rum": "", "urn": "",
	"test": "test", "t": "test", "tst": "test", "start": "start", "stop": "stop", "restart": "restart",
}

// managerCommands are the yarn, pnpm and bun commands that run no script, so
// the flags after them are the package manager's own.
var managerCommands = flagSet("install", "i", "add", "remove", "rm", "uninstall", "un", "ci", "upgrade", "up",
	"update", "audit", "outdated", "why", "info", "list", "ls", "pack", "publish", "version", "init", "link",
	"unlink", "config", "cache", "login", "logout", "prune", "dedupe", "import", "licenses", "help")

// scriptRunWord reports a word that starts a package-manager script run: a
// package manager, a run-all command, or a concurrently "npm:name" word.
func scriptRunWord(word string) bool {
	if strings.HasPrefix(word, "-") {
		return false
	}
	name := runnerWord(word)
	if manager, _, ok := strings.Cut(word, ":"); ok && packageManagers[manager] {
		return true
	}
	return packageManagers[name] || runAllCommands[name]
}

// forwardedFlags judges a package-manager script run (words from the
// package manager on): each word it forwards to the script that is
// "-"-prefixed or holds what the reader does not model ($, a backtick, a
// backslash or a brace, any of which may become a flag), and for Vitest a
// forwarded bench or typecheck positional (the script's vitest may take it as
// its command), is unproven, unless the script is resolved and exempt
// (exemptScript). npm forwards the positionals after the script name, and
// only the flags after "--"; a flag before "--" may move the script it runs,
// so it leaves the target unresolved. yarn, pnpm and bun forward every word
// after the script name, which is the first word after the manager (or after
// run), and is unresolved when a flag precedes it. A run-all command forwards
// the words inside each quoted script argument, and the words after "--" to
// every script.
func forwardedFlags(runner string, words []string, scripts map[string]string) []string {
	if manager, name, ok := strings.Cut(words[0], ":"); ok && packageManagers[manager] {
		words = append([]string{manager, "run", name}, words[1:]...)
	}
	manager, args := runnerWord(words[0]), words[1:]
	type run struct {
		target                string
		forwarded, positional []string
	}
	var runs []run
	flagLike := func(word string) bool {
		return word != "--" && (strings.HasPrefix(word, "-") || strings.ContainsAny(word, "$`\\{"))
	}
	split := func(words []string) (flags, positional []string) {
		for _, word := range words {
			switch {
			case flagLike(word):
				flags = append(flags, word)
			case word != "--":
				positional = append(positional, word)
			}
		}
		return flags, positional
	}
	forward := func(target string, words []string) {
		flags, positional := split(words)
		runs = append(runs, run{target: target, forwarded: flags, positional: positional})
	}
	switch {
	case runAllCommands[manager]:
		dash := slices.Index(args, "--")
		if dash < 0 {
			dash = len(args)
		} else {
			forward("", args[dash+1:])
		}
		for _, arg := range args[:dash] {
			if fields := strings.Fields(arg); len(fields) > 1 && !strings.HasPrefix(arg, "-") {
				forward(fields[0], fields[1:])
			}
		}
	case manager == "npm":
		dash := slices.Index(args, "--")
		before, after := args, []string(nil)
		if dash >= 0 {
			before, after = args[:dash], args[dash+1:]
		}
		script, ok := "", false
		if len(before) > 0 {
			script, ok = npmScriptCommands[before[0]]
		}
		if !ok {
			// Not a script run (npm exec, npm install): only the words after
			// "--" are forwarded.
			if dash >= 0 {
				flags, _ := split(after)
				runs = append(runs, run{forwarded: flags})
			}
			break
		}
		rest := before[1:]
		target := script
		if script == "" && len(rest) > 0 && !strings.HasPrefix(rest[0], "-") {
			target, rest = rest[0], rest[1:]
		}
		_, positional := split(rest)
		if slices.ContainsFunc(before[1:], flagLike) {
			target = ""
		}
		flags, afterPositional := split(after)
		runs = append(runs, run{target: target, forwarded: flags, positional: append(positional, afterPositional...)})
	default:
		at := 0
		if len(args) > 0 && (args[0] == "run" || args[0] == "run-script") {
			at = 1
		}
		switch {
		case at >= len(args):
			return nil
		case strings.HasPrefix(args[at], "-"):
			first := slices.IndexFunc(args, func(arg string) bool { return !strings.HasPrefix(arg, "-") })
			if first < 0 {
				return nil
			}
			forward("", args[first+1:])
		case at == 0 && managerCommands[args[0]]:
			return nil
		default:
			forward(args[at], args[at+1:])
		}
	}
	var found []string
	for _, r := range runs {
		var commands []string
		if runner == testid.Vitest {
			for _, word := range r.positional {
				if unsafeVitestCommands[word] {
					commands = append(commands, word)
				}
			}
		}
		if len(r.forwarded)+len(commands) == 0 || (r.target != "" && exemptScript(runner, r.target, scripts)) {
			continue
		}
		to := ""
		if r.target != "" {
			to = fmt.Sprintf(" to %q", r.target)
		}
		for _, flag := range r.forwarded {
			name, _, _ := strings.Cut(flag, "=")
			found = append(found, fmt.Sprintf("forwards %s through %s%s", name, manager, to))
		}
		for _, command := range commands {
			found = append(found, fmt.Sprintf("forwards the %s command through %s%s", command, manager, to))
		}
	}
	return found
}

// exemptScript reports whether forwarded flags cannot reach runner through
// the script target: it is one of scripts, and no word of it (quoted words
// split again) mentions the runner or starts a package-manager script run.
func exemptScript(runner, target string, scripts map[string]string) bool {
	text, ok := scripts[target]
	return ok && !reachesRunner(runner, text, 0)
}

func reachesRunner(runner, script string, depth int) bool {
	if depth > maxScriptDepth {
		return true
	}
	for _, words := range shellCommands(script) {
		for _, word := range words {
			if scriptRunWord(word) || mentionsRunner(runner, word) ||
				(strings.ContainsAny(word, " \t\n\r") && reachesRunner(runner, word, depth+1)) {
				return true
			}
		}
	}
	return false
}

func runnerWord(word string) string {
	base := path.Base(word)
	if at := strings.Index(base, "@"); at > 0 {
		base = base[:at]
	}
	for _, suffix := range []string{".js", ".mjs", ".cjs"} {
		base = strings.TrimSuffix(base, suffix)
	}
	return base
}

func safeScriptFlag(runner, arg string) bool {
	name, _, _ := strings.Cut(arg, "=")
	if !strings.HasPrefix(name, "--") {
		return safeScriptFlags[runner][name]
	}
	name = strings.TrimPrefix(name, "--")
	if group, _, dotted := strings.Cut(name, "."); dotted {
		return runner == testid.Vitest && safeDottedFlags[normalFlag(group)]
	}
	return safeScriptFlags[runner][normalFlag(strings.TrimPrefix(name, "no-"))]
}

func normalFlag(name string) string { return strings.ToLower(strings.ReplaceAll(name, "-", "")) }

// shellCommands splits a script into the words of each command: words end
// at unquoted whitespace, quotes group and are removed, and an unquoted ;,
// &, |, ( or ) ends the command.
func shellCommands(script string) [][]string {
	var commands [][]string
	var words []string
	var word strings.Builder
	inWord := false
	endWord := func() {
		if inWord {
			words = append(words, word.String())
			word.Reset()
			inWord = false
		}
	}
	endCommand := func() {
		endWord()
		if len(words) > 0 {
			commands = append(commands, words)
			words = nil
		}
	}
	for i := 0; i < len(script); i++ {
		switch c := script[i]; c {
		case '\'', '"':
			end := strings.IndexByte(script[i+1:], c)
			if end < 0 {
				end = len(script) - i - 1
			}
			word.WriteString(script[i+1 : i+1+end])
			inWord = true
			i += end + 1
		case ' ', '\t', '\n', '\r':
			endWord()
		case ';', '&', '|', '(', ')':
			endCommand()
		default:
			word.WriteByte(c)
			inWord = true
		}
	}
	endCommand()
	return commands
}

// proveJSONConfig proves a jest.config.json: a JSON object naming no
// pattern member.
func proveJSONConfig(p *proof, runner string, data []byte) {
	var object map[string]any
	if err := json.Unmarshal(data, &object); err != nil || object == nil {
		p.add("a config that is not a JSON object")
		return
	}
	for _, member := range patternMembers[runner] {
		if _, ok := object[member]; ok {
			p.add("sets " + member)
		}
	}
}

// proveJSConfig proves a JavaScript or TypeScript config from its tokens:
// no import but the runner's own package, no call but the identity helper,
// no spread, function, constructor, computed key or computed member, and no
// pattern member named as an identifier or string anywhere (comments
// aside). The one spread allowed is a Playwright device descriptor,
// `...devices['Name']` or `...devices.Name`.
func proveJSConfig(p *proof, runner string, data []byte) {
	tokens, cause := lexJS(data)
	if cause != "" {
		p.add(cause)
		return
	}
	members := map[string]bool{}
	for _, member := range patternMembers[runner] {
		members[member] = true
	}
	allowed := map[string]bool{}
	for _, module := range runnerModules[runner] {
		allowed[module] = true
	}
	helpers := map[string]bool{}
	for _, helper := range identityHelpers[runner] {
		helpers[helper] = true
	}
	at := func(index int) jsToken {
		if index < 0 || index >= len(tokens) {
			return jsToken{}
		}
		return tokens[index]
	}
	module := func(name string) {
		if !allowed[name] {
			p.add("imports " + name)
		}
	}
	typeOnly := false
	for index, token := range tokens {
		previous, next := at(index-1), at(index+1)
		if (token.kind == tokenIdent || token.kind == tokenString) && members[token.text] {
			p.add("sets " + token.text)
		}
		switch {
		case token.is(tokenIdent, "import"):
			switch {
			case next.is(tokenPunct, "("):
				if at(index+2).kind == tokenString && at(index+3).is(tokenPunct, ")") {
					module(at(index + 2).text)
				} else {
					p.add("a dynamic import")
				}
			case next.kind == tokenString:
				module(next.text)
			case next.is(tokenPunct, "."):
				p.add("import.meta")
			case next.is(tokenIdent, "type") && !at(index+2).is(tokenIdent, "from") && !at(index+2).is(tokenPunct, ","):
				typeOnly = true
			}
		case token.is(tokenIdent, "from") && next.kind == tokenString:
			if !typeOnly {
				module(next.text)
			}
			typeOnly = false
		case token.is(tokenIdent, "require") && next.is(tokenPunct, "("):
			if at(index+2).kind == tokenString && at(index+3).is(tokenPunct, ")") {
				module(at(index + 2).text)
			} else {
				p.add("a dynamic require")
			}
		case token.is(tokenIdent, "new"):
			p.add("a constructor call")
		case token.is(tokenIdent, "function"), token.is(tokenPunct, "=>"):
			p.add("a function")
		case token.is(tokenIdent, "class"):
			p.add("a class")
		case token.is(tokenPunct, "..."):
			device := runner == testid.Playwright && next.is(tokenIdent, "devices") &&
				((at(index+2).is(tokenPunct, "[") && at(index+3).kind == tokenString && at(index+4).is(tokenPunct, "]")) ||
					(at(index+2).is(tokenPunct, ".") && at(index+3).kind == tokenIdent))
			if !device {
				p.add("a spread")
			}
		case token.is(tokenPunct, "("):
			switch {
			case previous.kind == tokenIdent && notCallKeywords[previous.text]:
			case previous.is(tokenIdent, "import"), previous.is(tokenIdent, "require"):
			case previous.kind == tokenIdent:
				if !helpers[previous.text] || at(index-2).is(tokenPunct, ".") || at(index-2).is(tokenPunct, "?.") {
					p.add("calls " + previous.text)
				}
			case previous.is(tokenPunct, ")"), previous.is(tokenPunct, "]"), previous.is(tokenPunct, ">"), previous.is(tokenPunct, "?."), previous.kind == tokenTemplate:
				p.add("a call")
			}
		case token.kind == tokenTemplate && (previous.kind == tokenIdent || previous.is(tokenPunct, ")") || previous.is(tokenPunct, "]")) && !notCallKeywords[previous.text]:
			p.add("a tagged template")
		case token.is(tokenPunct, "["):
			literal := (next.kind == tokenString || next.kind == tokenNumber) && at(index+2).is(tokenPunct, "]")
			switch {
			case previous.is(tokenPunct, "{") || previous.is(tokenPunct, ","):
				// After "{" or "," a "[" opens an array element or, directly
				// inside an object literal, a computed key.
				if !literal && objectContext(tokens, index) {
					p.add("a computed key")
				}
			case previous.kind == tokenIdent && !notCallKeywords[previous.text], previous.is(tokenPunct, ")"), previous.is(tokenPunct, "]"), previous.is(tokenPunct, "?."):
				if !literal {
					p.add("a computed member")
				}
			}
		}
	}
}

// objectContext reports whether the "[" at index sits directly inside an
// object literal (a computed key) rather than an array.
func objectContext(tokens []jsToken, index int) bool {
	depth := 0
	for i := index - 1; i >= 0; i-- {
		switch {
		case tokens[i].is(tokenPunct, ")"), tokens[i].is(tokenPunct, "]"), tokens[i].is(tokenPunct, "}"):
			depth++
		case tokens[i].is(tokenPunct, "("), tokens[i].is(tokenPunct, "["), tokens[i].is(tokenPunct, "{"):
			if depth == 0 {
				return tokens[i].is(tokenPunct, "{")
			}
			depth--
		}
	}
	return false
}

// notCallKeywords are the identifiers a "(" or "[" may follow without
// making a call or a member access.
var notCallKeywords = map[string]bool{
	"if": true, "for": true, "while": true, "switch": true, "catch": true, "return": true, "typeof": true,
	"void": true, "delete": true, "await": true, "in": true, "of": true, "instanceof": true, "case": true,
	"do": true, "else": true, "yield": true, "satisfies": true, "as": true, "export": true, "default": true,
}

// The kinds of a JavaScript token.
const (
	tokenIdent = iota + 1
	tokenString
	tokenTemplate
	tokenNumber
	tokenRegexp
	tokenPunct
)

type jsToken struct {
	kind int
	text string
}

func (token jsToken) is(kind int, text string) bool { return token.kind == kind && token.text == text }

// lexJS splits a JavaScript or TypeScript config into tokens, dropping
// comments. It returns a cause instead when the source holds what the
// proof cannot read: an escape outside a regular expression (an escaped
// identifier or string could spell a member), a template expression, or an
// unterminated comment, string or template. A string token's text is its
// content.
func lexJS(src []byte) ([]jsToken, string) {
	var tokens []jsToken
	regexAllowed := func() bool {
		if len(tokens) == 0 {
			return true
		}
		last := tokens[len(tokens)-1]
		switch last.kind {
		case tokenPunct:
			return last.text != ")" && last.text != "]" && last.text != "}"
		case tokenIdent:
			return notCallKeywords[last.text]
		}
		return false
	}
	for i := 0; i < len(src); {
		c := src[i]
		switch {
		case c == ' ' || c == '\t' || c == '\n' || c == '\r' || c == '\f' || c == '\v':
			i++
		case c == '/' && i+1 < len(src) && src[i+1] == '/':
			for i < len(src) && src[i] != '\n' {
				i++
			}
		case c == '/' && i+1 < len(src) && src[i+1] == '*':
			end := bytes.Index(src[i+2:], []byte("*/"))
			if end < 0 {
				return nil, "an unterminated comment"
			}
			i += end + 4
		case c == '\'' || c == '"' || c == '`':
			j := i + 1
			for j < len(src) && src[j] != c {
				switch {
				case src[j] == '\\':
					return nil, "an escape in a string"
				case c != '`' && src[j] == '\n':
					return nil, "an unterminated string"
				case c == '`' && src[j] == '$' && j+1 < len(src) && src[j+1] == '{':
					return nil, "a template expression"
				}
				j++
			}
			if j >= len(src) {
				return nil, "an unterminated string"
			}
			kind := tokenString
			if c == '`' {
				kind = tokenTemplate
			}
			tokens = append(tokens, jsToken{kind: kind, text: string(src[i+1 : j])})
			i = j + 1
		case c == '/' && regexAllowed():
			j, class := i+1, false
			for j < len(src) && (class || src[j] != '/') {
				switch src[j] {
				case '\\':
					j++
				case '[':
					class = true
				case ']':
					class = false
				case '\n':
					return nil, "an unterminated regular expression"
				}
				j++
			}
			if j >= len(src) {
				return nil, "an unterminated regular expression"
			}
			j++
			for j < len(src) && identPart(src[j]) {
				j++
			}
			tokens = append(tokens, jsToken{kind: tokenRegexp, text: string(src[i:j])})
			i = j
		case c == '\\':
			return nil, "an escape outside a string"
		case identStart(c):
			j := i + 1
			for j < len(src) && identPart(src[j]) {
				j++
			}
			tokens = append(tokens, jsToken{kind: tokenIdent, text: string(src[i:j])})
			i = j
		case c >= '0' && c <= '9':
			j := i + 1
			for j < len(src) && (identPart(src[j]) || src[j] == '.') {
				j++
			}
			tokens = append(tokens, jsToken{kind: tokenNumber, text: string(src[i:j])})
			i = j
		case bytes.HasPrefix(src[i:], []byte("...")):
			tokens = append(tokens, jsToken{kind: tokenPunct, text: "..."})
			i += 3
		case bytes.HasPrefix(src[i:], []byte("=>")), bytes.HasPrefix(src[i:], []byte("?.")) && !(i+2 < len(src) && src[i+2] >= '0' && src[i+2] <= '9'):
			tokens = append(tokens, jsToken{kind: tokenPunct, text: string(src[i : i+2])})
			i += 2
		default:
			tokens = append(tokens, jsToken{kind: tokenPunct, text: string(c)})
			i++
		}
	}
	return tokens, ""
}

func identStart(c byte) bool {
	return c == '_' || c == '$' || (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || c >= 0x80
}

func identPart(c byte) bool { return identStart(c) || (c >= '0' && c <= '9') }

// manifestMembers is what a package.json says about the JavaScript
// runners: a "jest" member, whether a script runs vitest, and its scripts.
type manifestMembers struct {
	jest    bool
	vitest  bool
	scripts map[string]string
}

var vitestWord = regexp.MustCompile(`(^|[^A-Za-z0-9_@/-])vitest($|[^A-Za-z0-9_-])`)

func readManifest(name string, read func(name string, limit int) ([]byte, error)) (manifestMembers, error) {
	data, err := read(name, maxConfigProbe)
	if err != nil {
		return manifestMembers{}, err
	}
	var members map[string]any
	if err := json.Unmarshal(data, &members); err != nil {
		return manifestMembers{}, err
	}
	_, jest := members["jest"]
	result := manifestMembers{jest: jest, scripts: map[string]string{}}
	scripts, _ := members["scripts"].(map[string]any)
	for name, value := range scripts {
		if text, ok := value.(string); ok {
			result.scripts[name] = text
			result.vitest = result.vitest || vitestWord.MatchString(text)
		}
	}
	return result, nil
}

// pytestSafeOptions match the addopts options that cannot add a collected
// file: output, reporting, strictness, failure and import-mode options.
// Anything else, a positional path, -p, --doctest-modules or
// --doctest-glob included, leaves the instance unproven.
var pytestSafeOptions = regexp.MustCompile(`^(-[qvsxl]+|-r[A-Za-z]+|--(strict-markers|strict-config|strict|quiet|verbose|exitfirst|showlocals|no-header|no-summary|disable-warnings|disable-pytest-warnings|lf|ff|nf|sw|stepwise|last-failed|failed-first|new-first)|--(color|tb|durations|durations-min|maxfail|import-mode|capture|show-capture|log-level|log-cli-level|code-highlight|verbosity)=[A-Za-z0-9_.-]+)$`)

// conftestHooks match a conftest.py hook or variable that may collect a file
// under any name: a collection hook or a plugin load.
var conftestHooks = regexp.MustCompile(`\b(pytest_collect_file|pytest_pycollect_makemodule|pytest_collect_directory|pytest_ignore_collect|pytest_plugins)\b`)

// provePytest proves a pytest instance: every pytest config in its root
// that declares pytest (provePytestConfig; the instance's own config and,
// since which one pytest reads depends on its version, any other) sets no
// python_files and no testpaths and holds only pytestSafeOptions in its
// addopts, and no conftest.py under the root defines a collection hook or
// loads a plugin. Doctest files (.txt, .rst, .md) are candidates when a
// doctest option appears or cannot be ruled out; any file is one when a
// conftest.py hook may collect it.
func provePytest(p *proof, root, config string, files map[string]string, read func(name string, limit int) ([]byte, error)) {
	provePytestConfig(p, "", config, read)
	for _, base := range configNames[testid.Pytest] {
		name := path.Join(root, base)
		if _, ok := files[name]; !ok || name == config || !declaresPytest(name, read) {
			continue
		}
		provePytestConfig(p, name+" ", name, read)
	}
	for name := range files {
		if path.Base(name) != "conftest.py" || !under(root, name) {
			continue
		}
		source, err := read(name, maxConfigProbe)
		switch {
		case err != nil:
			p.add("an unreadable " + name)
			p.any = true
		case conftestHooks.Match(source):
			p.add(name + " defines " + string(conftestHooks.Find(source)))
			p.any = true
		}
	}
}

func provePytestOptions(p *proof, prefix, value string) {
	value = strings.NewReplacer("[", " ", "]", " ", ",", " ", `"`, " ", "'", " ").Replace(value)
	for _, option := range strings.Fields(value) {
		if pytestSafeOptions.MatchString(option) {
			continue
		}
		p.add(prefix + "addopts " + option)
		p.docs = true
	}
}
