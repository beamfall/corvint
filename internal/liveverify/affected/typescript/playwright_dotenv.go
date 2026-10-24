package typescript

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/Beamfall/corvint/internal/liveverify/affected"
)

// Environment file states and the resolution source for a value a dotenv file supplied
// (AFP-V0-405 to AFP-V0-408).
const (
	PlaywrightEnvironmentFilePresent = "PRESENT"
	PlaywrightEnvironmentFileAbsent  = "ABSENT"
	PlaywrightEnvironmentDotenv      = "DOTENV"
)

// PlaywrightEnvironmentFile is one env file a modelled top-level dotenv config() call reads,
// relative to the repository root, where the plan's argv runs. An ABSENT file has an empty
// digest (AFP-V0-406).
type PlaywrightEnvironmentFile struct {
	Path   string `json:"path"`
	SHA256 string `json:"sha256"`
	Site   string `json:"site"`
	State  string `json:"state"`
}

// PlaywrightEnvironmentResolution is the value the config reads for one recorded variable after
// the modelled dotenv loads run. File names the env file that supplied it, or is empty when the
// caller's value stands (AFP-V0-407).
type PlaywrightEnvironmentResolution struct {
	File   string `json:"file"`
	SHA256 string `json:"sha256"`
	Source string `json:"source"`
	State  string `json:"state"`
	Value  string `json:"value"`
}

// playwrightDotenvLoad is one modelled dotenv config() call: the file it reads and its call site.
type playwrightDotenvLoad struct {
	path string
	site string
}

var (
	playwrightDotenvImport  = regexp.MustCompile(`^import(?:\s+([A-Za-z_$][\w$]*)\s*(,)?|\s*(\{)|\s*\*\s*as\s+([A-Za-z_$][\w$]*))`)
	playwrightDotenvFrom    = regexp.MustCompile(`^[^;'"]*?\bfrom\s*(['"])([^'"\\\n]*)['"]`)
	playwrightDotenvBare    = regexp.MustCompile(`^import\s*(['"])([^'"\\\n]*)['"]`)
	playwrightDotenvRequire = regexp.MustCompile(`^(?:const|let|var)\s+([A-Za-z_$][\w$]*|\{[^{}'"]*\})\s*=\s*require\s*\(\s*(['"])([^'"\\\n]*)['"]\s*\)`)
	playwrightDotenvCall    = regexp.MustCompile(`^(?:([A-Za-z_$][\w$]*)|require\s*\(\s*(['"])dotenv['"]\s*\))\s*\.\s*config\s*\(`)
	playwrightDotenvStrict  = regexp.MustCompile(`^(['"])use strict['"]`)
)

// playwrightDotenvLoads models the config's dotenv use. Only a prelude of import declarations,
// require bindings and dotenv config() expression statements runs before any other config code,
// so a load in that prelude happens before the config reads process.env. A dotenv binding is a
// default or namespace import or a plain require binding of "dotenv". A config() call there is
// modelled when its argument is absent or an object literal with only a literal path and literal
// override:false, quiet or debug. The modelled spans are blanked in residual so the remaining
// writer checks still see every other loader. Any other dotenv use returns a frontier naming its
// call site, which the caller reports before its loader-name checks (AFP-V0-405, AFP-V0-408).
func playwrightDotenvLoads(configPath, clean string) (loads []playwrightDotenvLoad, residual, frontier string) {
	if len(playwrightTokenOffsets(clean, "dotenv")) == 0 {
		return nil, clean, ""
	}
	lexed, ok := playwrightLex(clean)
	if !ok {
		return nil, clean, "the config loads dotenv in a module the static profile cannot tokenize"
	}
	blanked := []byte(clean)
	blank := func(start, end int) {
		for index := start; index < end; index++ {
			if blanked[index] != '\n' {
				blanked[index] = ' '
			}
		}
	}
	bindings := []string{}
	site := func(offset int) string {
		return configPath + ":" + strconv.Itoa(1+strings.Count(clean[:offset], "\n"))
	}
	statementEnd := func(end int) bool {
		for ; end < len(clean) && (clean[end] == ' ' || clean[end] == '\t' || clean[end] == '\r'); end++ {
		}
		return end >= len(clean) || clean[end] == ';' || clean[end] == '\n'
	}
	// fail blanks the refused statement too, so residual holds only the uses left to the caller.
	fail := func(start, end int, detail string) ([]playwrightDotenvLoad, string, string) {
		blank(start, end)
		return nil, string(blanked), detail
	}
	cursor := 0
prelude:
	for {
		for cursor < len(clean) && strings.IndexByte(" \t\r\n;", clean[cursor]) >= 0 {
			cursor++
		}
		if cursor >= len(clean) {
			break
		}
		rest := clean[cursor:]
		switch {
		case playwrightDotenvStrict.MatchString(rest):
			cursor += len(playwrightDotenvStrict.FindString(rest))
		case playwrightDotenvBare.MatchString(rest):
			match := playwrightDotenvBare.FindStringSubmatch(rest)
			cursor += len(match[0])
		case playwrightDotenvImport.MatchString(rest):
			head := playwrightDotenvImport.FindStringSubmatch(rest)
			from := playwrightDotenvFrom.FindStringSubmatch(rest)
			if from == nil {
				break prelude
			}
			if from[2] == "dotenv" {
				name := head[1] + head[4]
				if name == "" || head[2] != "" || name == "type" {
					return fail(cursor, cursor+len(from[0]), "the config imports dotenv at "+site(cursor)+" in a form the static profile does not model")
				}
				bindings = append(bindings, name)
				blank(cursor, cursor+len(from[0]))
			}
			cursor += len(from[0])
		case playwrightDotenvRequire.MatchString(rest):
			match := playwrightDotenvRequire.FindStringSubmatch(rest)
			if !statementEnd(cursor + len(match[0])) {
				break prelude
			}
			if match[3] == "dotenv" {
				if strings.HasPrefix(match[1], "{") {
					return fail(cursor, cursor+len(match[0]), "the config requires dotenv at "+site(cursor)+" in a form the static profile does not model")
				}
				bindings = append(bindings, match[1])
				blank(cursor, cursor+len(match[0]))
			}
			cursor += len(match[0])
		case playwrightDotenvCall.MatchString(rest):
			match := playwrightDotenvCall.FindStringSubmatch(rest)
			if match[1] != "" && !slices.Contains(bindings, match[1]) {
				break prelude
			}
			open := cursor + len(match[0]) - 1
			closeAt, closed := lexed.closes[open]
			if !closed {
				return fail(cursor, open+1, "the config's dotenv config() call at "+site(cursor)+" is not a modelled expression statement")
			}
			if !statementEnd(closeAt + 1) {
				return fail(cursor, closeAt+1, "the config's dotenv config() call at "+site(cursor)+" is not a modelled expression statement")
			}
			file, modelled := playwrightDotenvArguments(clean[open+1 : closeAt])
			if !modelled {
				return fail(cursor, closeAt+1, "the config's dotenv config() call at "+site(cursor)+" has a computed or unmodelled argument, so the static profile does not know which file it loads or whether it overrides the variable")
			}
			loads = append(loads, playwrightDotenvLoad{path: file, site: site(cursor)})
			blank(cursor, closeAt+1)
			cursor = closeAt + 1
		default:
			break prelude
		}
	}
	residual = string(blanked)
	for _, name := range append([]string{"dotenv"}, bindings...) {
		for _, offset := range playwrightTokenOffsets(residual, name) {
			// A code token, or the module specifier 'dotenv' itself; other strings such as
			// 'dotenv/config' are left to the caller's loader checks.
			quoted := offset > 0 && offset+len(name) < len(clean) && strings.IndexByte(`'"`, clean[offset-1]) >= 0 && clean[offset+len(name)] == clean[offset-1]
			if lexed.code[offset] || quoted {
				return nil, residual, "the config uses dotenv at " + site(offset) + " outside a top-level config() call that runs before the rest of the config, so the static profile does not model which file it loads"
			}
		}
	}
	return loads, residual, ""
}

// playwrightDotenvArguments returns the file a config() call with argument raw reads. Only an
// absent argument or an object literal with a literal escape-free path, override:false, and
// literal quiet or debug flags is modelled; a path must stay inside the repository, must not name
// a .vault file or start with "~", and must already be clean apart from one leading "./", since
// dotenv passes it to the filesystem unchanged and `.env/` or `missing/../.env` can fail there
// where the cleaned path would not (AFP-V0-405).
func playwrightDotenvArguments(raw string) (string, bool) {
	file := ".env"
	if strings.TrimSpace(raw) == "" {
		return file, true
	}
	properties, ok := playwrightObjectProperties(raw)
	if !ok {
		return "", false
	}
	for key, value := range properties {
		switch key {
		case "path":
			literal, isString := playwrightString(value)
			if !isString || strings.ContainsRune(value, '\\') || strings.HasPrefix(literal, "~") {
				return "", false
			}
			file = strings.TrimPrefix(literal, "./")
			if path.Clean(file) != file {
				return "", false
			}
		case "override":
			if value != "false" {
				return "", false
			}
		case "quiet", "debug":
			if value != "true" && value != "false" {
				return "", false
			}
		default:
			return "", false
		}
	}
	return file, affected.ValidRelativePath(file) && file != "." && !strings.HasPrefix(file, "../") && !strings.HasSuffix(file, ".vault")
}

// observePlaywrightDotenv reads each modelled load's file under root. A missing file is ABSENT.
// An existing <file>.vault (which dotenv decrypts when DOTENV_KEY is set), an unreadable or
// non-UTF-8 file, or a line outside the modelled NAME=VALUE grammar returns a frontier naming
// the file (AFP-V0-406, AFP-V0-408).
func observePlaywrightDotenv(root string, loads []playwrightDotenvLoad) ([]PlaywrightEnvironmentFile, []map[string]string, string) {
	files := make([]PlaywrightEnvironmentFile, 0, len(loads))
	values := make([]map[string]string, 0, len(loads))
	for _, load := range loads {
		prefix := "the config's dotenv config() call at " + load.site + " reads " + load.path
		if _, err := os.Lstat(filepath.Join(root, filepath.FromSlash(load.path+".vault"))); err == nil || !errors.Is(err, fs.ErrNotExist) {
			return nil, nil, prefix + ", and " + load.path + ".vault exists or cannot be checked, which dotenv decrypts instead when DOTENV_KEY is set"
		}
		content, err := affected.ReadSource(root, load.path)
		if errors.Is(err, fs.ErrNotExist) {
			files = append(files, PlaywrightEnvironmentFile{Path: load.path, Site: load.site, State: PlaywrightEnvironmentFileAbsent})
			values = append(values, nil)
			continue
		}
		if err != nil || !utf8.Valid(content) {
			return nil, nil, prefix + ", which is unreadable, not a regular file or not UTF-8"
		}
		parsed, line := parsePlaywrightDotenv(string(content))
		if line != 0 {
			return nil, nil, prefix + ", whose line " + strconv.Itoa(line) + " is not a NAME=VALUE form the static profile models"
		}
		sum := sha256.Sum256(content)
		files = append(files, PlaywrightEnvironmentFile{Path: load.path, SHA256: hex.EncodeToString(sum[:]), Site: load.site, State: PlaywrightEnvironmentFilePresent})
		values = append(values, parsed)
	}
	return files, values, ""
}

// playwrightDotenvLine is the one line form whose value every dotenv release parses the same way:
// an optional-whitespace NAME=VALUE with no export prefix, no inline comment, no backslash or
// backtick anywhere in the value, and a value that is unquoted (not starting with a quote) or
// wholly single- or double-quoted with nothing after the closing quote (dotenv before 15 keeps
// the quotes when whitespace follows).
var playwrightDotenvLine = regexp.MustCompile(`^[ \t]*([\w.-]+)[ \t]*=[ \t]*(?:'([^'\\#` + "`" + `]*)'|"([^"\\#` + "`" + `]*)"|(?:([^'"` + "`" + `#\s\\][^#\\` + "`" + `]*?)?[ \t]*))$`)

// parsePlaywrightDotenv parses content after dotenv's CRLF/CR normalization. Blank and whole-line
// # comment lines are skipped; the last duplicate wins. It returns the 1-based number of the first
// line outside the modelled grammar, or 0 (AFP-V0-406, AFP-V0-408).
func parsePlaywrightDotenv(content string) (map[string]string, int) {
	content = strings.ReplaceAll(strings.ReplaceAll(content, "\r\n", "\n"), "\r", "\n")
	values := map[string]string{}
	for index, line := range strings.Split(content, "\n") {
		trimmed := strings.Trim(line, " \t")
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}
		match := playwrightDotenvLine.FindStringSubmatch(line)
		if match == nil || strings.ContainsFunc(line, func(character rune) bool {
			return character != ' ' && character != '\t' && (playwrightJSSpace(character) || character < ' ' || character == 0x7f)
		}) {
			return nil, index + 1
		}
		values[match[1]] = match[2] + match[3] + strings.TrimRight(match[4], " \t")
	}
	return values, 0
}

// resolvePlaywrightDotenv returns each input with the value the config reads after the loads run
// in order. dotenv never overrides a variable already present, so only an UNSET input takes the
// first file value (AFP-V0-407).
func resolvePlaywrightDotenv(inputs []PlaywrightEnvironmentInput, files []PlaywrightEnvironmentFile, values []map[string]string) []PlaywrightEnvironmentInput {
	resolved := append([]PlaywrightEnvironmentInput(nil), inputs...)
	for index := range resolved {
		input := &resolved[index]
		resolution := PlaywrightEnvironmentResolution{SHA256: input.SHA256, Source: input.Source, State: input.State, Value: input.Value}
		for at, file := range files {
			value, found := values[at][input.Name]
			if !found || resolution.State == PlaywrightEnvironmentSet {
				continue
			}
			sum := sha256.Sum256([]byte(value))
			resolution = PlaywrightEnvironmentResolution{File: file.Path, SHA256: hex.EncodeToString(sum[:]), Source: PlaywrightEnvironmentDotenv, State: PlaywrightEnvironmentSet, Value: value}
		}
		input.Resolved = &resolution
	}
	return resolved
}
