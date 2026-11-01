package dogfoodflow

import (
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/Beamfall/corvint/internal/trace"
)

// testFunction finds the top-level test, example and fuzz functions that
// go test -run selects from; a benchmark runs only under -bench.
var testFunction = regexp.MustCompile(`(?m)^func\s+((?:Test|Example|Fuzz)\w*)\s*\(`)

// valuedTestFlags are the go test flags whose value is the next word, so the
// word is not read as a package.
var valuedTestFlags = map[string]bool{"-count": true, "-timeout": true, "-p": true, "-parallel": true, "-cpu": true, "-tags": true, "-bench": true, "-benchtime": true, "-skip": true, "-shuffle": true, "-coverprofile": true, "-covermode": true, "-coverpkg": true, "-o": true, "-exec": true, "-vet": true, "-test.count": true, "-test.timeout": true, "-test.skip": true}

// maxRunScanFiles bounds the test files one verify line's packages may read.
const maxRunScanFiles = 4096

// emptyRunLine is a verify input line whose go test -run pattern selects no
// top-level test in any package directory it names (DCW-V0-922).
type emptyRunLine struct {
	line     int
	text     string
	pattern  string
	packages string
}

// emptyRunLineOf returns the first verify input line, file or inline, whose
// go test -run pattern matches no test function name in the _test.go files
// of the local package directories it names under root, or ok=false when
// every such line selects a test. A line is checked only when every fact is
// known: a go test command with one -run pattern that compiles, and package
// arguments that are all ./ directories inside root; anything else (a
// script, an import path, go -C, an unreadable directory) is left to the run.
func emptyRunLineOf(root string, options ChangeOptions) (emptyRunLine, bool) {
	text := []byte(options.Verify)
	if options.VerifyFile != "" {
		data, err := readPrefix(options.VerifyFile, maxVerifyFileBytes)
		if err != nil {
			return emptyRunLine{}, false
		}
		text = data
	}
	for index, line := range strings.Split(string(text), "\n") {
		if strings.TrimLeft(line, " \t\n\v\f\r") == "" {
			continue
		}
		pattern, packages, ok := goTestRun(verifyLineArguments(line))
		if !ok {
			continue
		}
		if matched, known := runSelectsTest(root, pattern, packages); known && !matched {
			return emptyRunLine{line: index + 1, text: line, pattern: pattern, packages: strings.Join(packages, " ")}, true
		}
	}
	return emptyRunLine{}, false
}

// goTestRun is the -run pattern and ./ package arguments of a verify
// argument pair whose command contains go test, or ok=false.
func goTestRun(pair []string) (pattern string, packages []string, ok bool) {
	if len(pair) != 2 {
		return "", nil, false
	}
	var argv []string
	if pair[0] == "--verify-argv-json" {
		parsed, err := trace.ParseVerificationArgv([]byte(pair[1]))
		if err != nil {
			return "", nil, false
		}
		argv = parsed
	} else {
		argv = strings.Fields(pair[1])
	}
	start := -1
	for i := 0; i+1 < len(argv); i++ {
		if (argv[i] == "go" || strings.HasSuffix(argv[i], "/go")) && argv[i+1] == "test" {
			start = i + 2
			break
		}
		if argv[i] == "go" {
			// go -C dir test, or another go flag before test: the
			// directory is not known here.
			return "", nil, false
		}
	}
	if start < 0 {
		return "", nil, false
	}
	runs := 0
	for i := start; i < len(argv); i++ {
		word := argv[i]
		switch {
		case word == "-run" || word == "-test.run":
			if i+1 >= len(argv) {
				return "", nil, false
			}
			pattern, runs, i = argv[i+1], runs+1, i+1
		case strings.HasPrefix(word, "-run=") || strings.HasPrefix(word, "-test.run="):
			pattern, runs = word[strings.IndexByte(word, '=')+1:], runs+1
		case word == "--" || word == "-args":
			i = len(argv)
		case strings.HasPrefix(word, "./") || word == ".":
			packages = append(packages, word)
		case valuedTestFlags[word]:
			i++
		case !strings.HasPrefix(word, "-"):
			// An import path: the package set is not known here.
			return "", nil, false
		}
	}
	return pattern, packages, runs == 1 && pattern != "" && len(packages) > 0
}

// runSelectsTest reports whether pattern's top-level element matches a test
// function name in the _test.go files of packages (./dir, or ./dir/... for
// the tree below it) under root; known is false when a directory cannot be
// read, leaves root, or the pattern does not compile, so the line is left to
// the run.
func runSelectsTest(root, pattern string, packages []string) (matched, known bool) {
	top := topRunElement(pattern)
	if top == "" {
		return true, true
	}
	expression, err := regexp.Compile(top)
	if err != nil {
		return false, false
	}
	scanned := 0
	for _, pkg := range packages {
		recursive := strings.HasSuffix(pkg, "/...")
		dir := filepath.Join(root, filepath.FromSlash(strings.TrimSuffix(pkg, "/...")))
		if rel, err := filepath.Rel(root, dir); err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
			return false, false
		}
		info, err := os.Stat(dir)
		if err != nil || !info.IsDir() {
			return false, false
		}
		found, ok := false, true
		walk := func(path string, entry fs.DirEntry, err error) error {
			if err != nil {
				ok = false
				return fs.SkipAll
			}
			if entry.IsDir() {
				name := entry.Name()
				if path != dir && (!recursive || name == "testdata" || name == "vendor" || strings.HasPrefix(name, ".") || strings.HasPrefix(name, "_")) {
					return fs.SkipDir
				}
				return nil
			}
			if !strings.HasSuffix(entry.Name(), "_test.go") {
				return nil
			}
			if scanned++; scanned > maxRunScanFiles {
				ok = false
				return fs.SkipAll
			}
			data, err := os.ReadFile(path)
			if err != nil {
				ok = false
				return fs.SkipAll
			}
			for _, match := range testFunction.FindAllSubmatch(data, -1) {
				if expression.Match(match[1]) {
					found = true
					return fs.SkipAll
				}
			}
			return nil
		}
		_ = filepath.WalkDir(dir, walk)
		if !ok {
			return false, false
		}
		if found {
			return true, true
		}
	}
	return false, true
}

// topRunElement is the part of a -run pattern before its first slash
// outside brackets and parentheses: the element go test matches against
// top-level test names (go help testflag).
func topRunElement(pattern string) string {
	depth, class := 0, false
	for i, r := range pattern {
		switch {
		case class && r == ']':
			class = false
		case class:
		case r == '[':
			class = true
		case r == '(':
			depth++
		case r == ')' && depth > 0:
			depth--
		case r == '/' && depth == 0:
			return pattern[:i]
		}
	}
	return pattern
}
