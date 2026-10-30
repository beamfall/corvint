package golang

import (
	"go/scanner"
	"go/token"
	"io/fs"
	"path"
	"sort"
	"strconv"
	"strings"
)

// embedDirective is the line-comment prefix of a //go:embed directive.
const embedDirective = "//go:embed"

// embedPatterns returns the patterns of every //go:embed directive in one Go
// file's body (AFP-V0-600). A directive is a line comment that starts its
// line, as the go command reads it; text inside a string literal or a block
// comment is no directive. It returns false when the file does not scan or a
// directive's arguments do not parse, so the caller resolves none of the
// file's directives and every file it might embed stays owned by none.
func embedPatterns(body []byte) ([]string, bool) {
	fileSet := token.NewFileSet()
	file := fileSet.AddFile("", -1, len(body))
	failed := false
	var lexer scanner.Scanner
	lexer.Init(file, body, func(token.Position, string) { failed = true }, scanner.ScanComments)
	var patterns []string
	for {
		position, kind, literal := lexer.Scan()
		if kind == token.EOF {
			break
		}
		if kind != token.COMMENT {
			continue
		}
		arguments, directive := strings.CutPrefix(literal, embedDirective)
		if !directive || (arguments != "" && arguments[0] != ' ' && arguments[0] != '\t') || !startsLine(body, file.Offset(position)) {
			continue
		}
		parsed, ok := embedArguments(arguments)
		if !ok {
			return nil, false
		}
		patterns = append(patterns, parsed...)
	}
	if failed {
		return nil, false
	}
	return patterns, true
}

// startsLine reports whether only spaces and tabs precede offset on its line.
func startsLine(body []byte, offset int) bool {
	for index := offset - 1; index >= 0 && body[index] != '\n'; index-- {
		if body[index] != ' ' && body[index] != '\t' {
			return false
		}
	}
	return true
}

// embedArguments splits one directive's arguments as the go command does:
// space-separated patterns, each optionally a Go string literal. It returns
// false for an empty list or a malformed literal.
func embedArguments(arguments string) ([]string, bool) {
	var patterns []string
	for {
		arguments = strings.TrimLeft(arguments, " \t")
		if arguments == "" {
			break
		}
		var pattern string
		switch arguments[0] {
		case '"', '`':
			quoted, err := strconv.QuotedPrefix(arguments)
			if err != nil {
				return nil, false
			}
			if pattern, err = strconv.Unquote(quoted); err != nil {
				return nil, false
			}
			arguments = arguments[len(quoted):]
			if arguments != "" && arguments[0] != ' ' && arguments[0] != '\t' {
				return nil, false
			}
		default:
			end := strings.IndexAny(arguments, " \t")
			if end < 0 {
				end = len(arguments)
			}
			pattern, arguments = arguments[:end], arguments[end:]
		}
		patterns = append(patterns, pattern)
	}
	return patterns, len(patterns) != 0
}

// embedTargets resolves the patterns of the package in directory against the
// sorted repository files, as the go command resolves them (AFP-V0-600): a
// pattern is matched component by component relative to the package
// directory; a matched file is embedded, and a matched directory embeds every
// file below it except those under a name beginning with "." or "_" (unless
// the pattern carries the "all:" prefix) or a version-control name, and
// except below a nested module root. A pattern the go command rejects makes
// the whole list unknown, so it returns nil: claiming less leaves a file owned
// by none, never selected less.
func embedTargets(directory string, patterns, files []string, moduleRoots map[string]bool) []string {
	prefix := directory + "/"
	if directory == "." {
		prefix = ""
	}
	start := sort.SearchStrings(files, prefix)
	below := files[start:]
	if end := sort.Search(len(below), func(index int) bool { return !strings.HasPrefix(below[index], prefix) }); end < len(below) {
		below = below[:end]
	}
	targets := make(map[string]bool)
	for _, pattern := range patterns {
		glob, all := strings.CutPrefix(pattern, "all:")
		if glob == "." || !fs.ValidPath(glob) {
			return nil
		}
		if _, err := path.Match(glob, ""); err != nil {
			return nil
		}
		depth := strings.Count(glob, "/") + 1
		for _, file := range below {
			components := strings.Split(file[len(prefix):], "/")
			if len(components) < depth {
				continue
			}
			if matched, _ := path.Match(glob, strings.Join(components[:depth], "/")); !matched {
				continue
			}
			if embeddable(prefix, components, depth, all, moduleRoots) {
				targets[file] = true
			}
		}
	}
	listed := make([]string, 0, len(targets))
	for file := range targets {
		listed = append(listed, file)
	}
	sort.Strings(listed)
	return listed
}

// embeddable reports whether a file below the package directory, whose
// leading depth components a pattern matched, is one the go command embeds.
func embeddable(prefix string, components []string, depth int, all bool, moduleRoots map[string]bool) bool {
	for index, component := range components {
		if index+1 < len(components) && moduleRoots[prefix+strings.Join(components[:index+1], "/")] {
			return false
		}
		switch component {
		case ".bzr", ".hg", ".git", ".svn":
			return false
		}
		if index >= depth && !all && (component[0] == '.' || component[0] == '_') {
			return false
		}
	}
	return true
}
