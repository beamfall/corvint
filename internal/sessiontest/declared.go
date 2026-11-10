package sessiontest

import (
	"bytes"
	"errors"
	"fmt"
	"path"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"

	"encoding/json/jsontext"
	json "encoding/json/v2"
)

// The project-owned full-suite declaration (STR-V0-003, STR-V0-006): one
// bounded JSON file naming the Go full-suite environment and flags and each
// other language's full command, or the project's gate-scoped assertion
// that no gate step, driver or CI job runs a suite for the language.
// `corvint test` reads it as a plain file; it never interprets the Makefile,
// and a maintained gate (tools/test-commands-check) keeps the Go entry equal
// to what `make go-test` actually runs.
const (
	DeclarationPath    = ".github/cishards/test-commands.json"
	DeclarationProfile = "corvint-test-commands/0"
)

// LanguageCommand is one declared language: either Argv, the full command
// run from the repository root, or None, the reason the project asserts
// that the gate runs no suite for the language (STR-V0-004).
type LanguageCommand struct {
	Language string   `json:"language"`
	Argv     []string `json:"argv"`
	None     string   `json:"none"`
}

// ProviderExecutable is the only test provider a declaration may name
// (STR-V0-8701): the JavaScript and TypeScript provider, whose `unit`
// subcommand runs Vitest and whose `e2e` subcommand runs Playwright.
const ProviderExecutable = "corvint-js-test-provider"

// The runners a declared provider may execute (STR-V0-8701). The names are
// the runner segment of the affected plan's "typescript:<runner>:<path>"
// unit ids.
const (
	ProviderRunnerPlaywright = "playwright"
	ProviderRunnerVitest     = "vitest"
)

// providerSubcommands maps each admitted runner to the provider subcommand
// its declared argv must name.
var providerSubcommands = map[string]string{ProviderRunnerPlaywright: "e2e", ProviderRunnerVitest: "unit"}

// ProviderCommand is one declared test provider (STR-V0-8701): Argv is the
// provider invocation without the selected files, run from the repository
// root; `corvint test` appends the selected test files of Runner.
type ProviderCommand struct {
	Argv   []string `json:"argv"`
	Runner string   `json:"runner"`
}

// Declared is the read declaration in the slice form the manifest digests:
// GoEnv as sorted NAME=VALUE words, GoFlags as argv words placed between
// `go test` and the package patterns, Languages sorted by name and Providers
// sorted by runner. Providers is absent when none is declared, so such a
// declaration keeps its digest. A declaration that cannot be read carries the
// reason in Unreadable and nothing else.
type Declared struct {
	GoEnv      []string          `json:"goEnv"`
	GoFlags    []string          `json:"goFlags"`
	Languages  []LanguageCommand `json:"languages"`
	Providers  []ProviderCommand `json:"providers,omitempty"`
	Unreadable string            `json:"unreadable"`
}

// Provider returns the declared provider argv for runner, if any.
func (d Declared) Provider(runner string) ([]string, bool) {
	for _, entry := range d.Providers {
		if entry.Runner == runner {
			return entry.Argv, true
		}
	}
	return nil, false
}

// Command returns the declared entry for language, if any.
func (d Declared) Command(language string) (LanguageCommand, bool) {
	for _, entry := range d.Languages {
		if entry.Language == language {
			return entry, true
		}
	}
	return LanguageCommand{}, false
}

// GoCacheOverride returns goEnv with a declared GOCACHE replaced by the
// caller's CORVINT_GOCACHE when environ sets it non-empty (STR-V0-174,
// V1-3155), the knob the Makefile's `CORVINT_GOCACHE ?=` and script/heavy.sh
// already use, so `corvint test`, `make go-test` and the Makefile's other Go
// targets share one cache per lane or host instead of the declared
// /tmp path. goEnv without a GOCACHE entry (the ci profile's) is returned as
// is. The manifest keeps the declared value; only the launch, and the
// receipt that records it, carry the override.
func GoCacheOverride(goEnv, environ []string) []string {
	cache := ""
	for _, entry := range environ {
		if value, ok := strings.CutPrefix(entry, "CORVINT_GOCACHE="); ok {
			cache = value
		}
	}
	if cache == "" {
		return goEnv
	}
	out := make([]string, len(goEnv))
	for index, entry := range goEnv {
		if strings.HasPrefix(entry, "GOCACHE=") {
			entry = "GOCACHE=" + cache
		}
		out[index] = entry
	}
	return out
}

// WithGoParallel returns d with its Go `-p` flag set to n (STR-V0-173,
// V1-3149): a `-p VALUE` pair or `-p=VALUE` word is replaced in place, and
// without one `-p n` is appended. The declaration file is unchanged; the
// manifest digests the result, so every shard of one manifest and the
// receipt's argv state the parallelism that ran. An unreadable declaration
// is returned as is.
func (d Declared) WithGoParallel(n int) Declared {
	if d.Unreadable != "" {
		return d
	}
	value := strconv.Itoa(n)
	flags := make([]string, 0, len(d.GoFlags)+2)
	replaced := false
	for index := 0; index < len(d.GoFlags); index++ {
		switch word := d.GoFlags[index]; {
		case (word == "-p" || word == "--p") && index+1 < len(d.GoFlags):
			flags = append(flags, word, value)
			index++
			replaced = true
		case strings.HasPrefix(word, "-p=") || strings.HasPrefix(word, "--p="):
			name, _, _ := strings.Cut(word, "=")
			flags = append(flags, name+"="+value)
			replaced = true
		default:
			flags = append(flags, word)
		}
	}
	if !replaced {
		flags = append(flags, "-p", value)
	}
	d.GoFlags = flags
	return d
}

// declarationFile is the on-disk schema: a closed set of members, with each
// language mapped to an argv array or to {"none": reason} and each provider
// runner mapped to an argv array.
type declarationFile struct {
	Profile   string                    `json:"profile"`
	Go        *declarationGo            `json:"go"`
	Languages map[string]jsontext.Value `json:"languages"`
	Providers map[string][]string       `json:"providers"`
}

type declarationGo struct {
	Env   map[string]string `json:"env"`
	Flags []string          `json:"flags"`
}

// ReadDeclared reads root's declaration. Any problem (absence, size, an
// unknown or duplicate member, a malformed entry) is returned as Unreadable
// rather than as a partial declaration.
func ReadDeclared(root string) Declared {
	return ReadDeclaredFile(filepath.Join(root, filepath.FromSlash(DeclarationPath)))
}

// ReadDeclaredFile reads one declaration file by path (the repository gate
// reads a copy under test this way).
func ReadDeclaredFile(name string) Declared {
	declared, err := readDeclaration(name)
	if err != nil {
		return Declared{GoEnv: []string{}, GoFlags: []string{}, Languages: []LanguageCommand{}, Unreadable: DeclarationPath + ": " + err.Error()}
	}
	return declared
}

// ParseDeclared parses declaration bytes the caller has already read under its own file rules
// (adopt --setup, ADS-V0-802), with the same refusals as ReadDeclared.
func ParseDeclared(raw []byte) Declared {
	declared, err := parseDeclaration(raw)
	if err != nil {
		return Declared{GoEnv: []string{}, GoFlags: []string{}, Languages: []LanguageCommand{}, Unreadable: DeclarationPath + ": " + err.Error()}
	}
	return declared
}

func readDeclaration(name string) (Declared, error) {
	raw, err := readRegular(name, MaxDocumentBytes)
	if err != nil {
		return Declared{}, err
	}
	return parseDeclaration(raw)
}

func parseDeclaration(raw []byte) (Declared, error) {
	if len(raw) > MaxDocumentBytes {
		return Declared{}, fmt.Errorf("exceeds %d bytes", MaxDocumentBytes)
	}
	var parsed declarationFile
	if err := json.Unmarshal(raw, &parsed, json.RejectUnknownMembers(true)); err != nil {
		return Declared{}, err
	}
	if parsed.Profile != DeclarationProfile {
		return Declared{}, fmt.Errorf("profile is %q, want %q", parsed.Profile, DeclarationProfile)
	}
	if parsed.Go == nil {
		return Declared{}, errors.New("go is required")
	}
	declared := Declared{GoEnv: []string{}, GoFlags: []string{}, Languages: []LanguageCommand{}}
	for name, value := range parsed.Go.Env {
		if !EnvName(name) || !plain(value) {
			return Declared{}, fmt.Errorf("go.env has an invalid variable %q", name)
		}
		if name == "PATH" {
			// The go executable is found on the caller's PATH, so a declared
			// PATH would not select it.
			return Declared{}, errors.New("go.env may not declare PATH")
		}
		declared.GoEnv = append(declared.GoEnv, name+"="+value)
	}
	sort.Strings(declared.GoEnv)
	if err := words("go.flags", parsed.Go.Flags); err != nil {
		return Declared{}, err
	}
	declared.GoFlags = append(declared.GoFlags, parsed.Go.Flags...)
	for language, value := range parsed.Languages {
		if !plain(language) || language == "go" || strings.Contains(language, ":") {
			return Declared{}, fmt.Errorf("languages has an invalid language name %q", language)
		}
		entry, err := languageCommand(language, value)
		if err != nil {
			return Declared{}, err
		}
		declared.Languages = append(declared.Languages, entry)
	}
	sort.Slice(declared.Languages, func(i, j int) bool { return declared.Languages[i].Language < declared.Languages[j].Language })
	for runner, argv := range parsed.Providers {
		if err := providerCommand(runner, argv); err != nil {
			return Declared{}, err
		}
		declared.Providers = append(declared.Providers, ProviderCommand{Argv: argv, Runner: runner})
	}
	sort.Slice(declared.Providers, func(i, j int) bool { return declared.Providers[i].Runner < declared.Providers[j].Runner })
	return declared, nil
}

// providerCommand admits one providers entry (STR-V0-8701): a runner of the
// closed set, whose argv names the provider executable and that runner's
// subcommand in plain words, is a one-shot run and does not end the
// provider's flags before the selected files are appended. The Vitest --dir must be
// a repository-relative directory, since the command runs from the
// repository root and the planner compares it with repository-relative test
// paths.
func providerCommand(runner string, argv []string) error {
	field := "providers." + runner
	subcommand, ok := providerSubcommands[runner]
	if !ok {
		return fmt.Errorf("providers has an unsupported runner %q: want %s or %s", runner, ProviderRunnerPlaywright, ProviderRunnerVitest)
	}
	if err := words(field, argv); err != nil {
		return err
	}
	if len(argv) < 2 || path.Base(filepath.ToSlash(argv[0])) != ProviderExecutable || argv[1] != subcommand {
		return fmt.Errorf("%s: argv must start with %s %s", field, ProviderExecutable, subcommand)
	}
	for index, word := range argv[2:] {
		if name, _, _ := strings.Cut(flagName(word), "="); name == "watch" {
			return fmt.Errorf("%s: a watch run never ends; declare the one-shot command", field)
		}
		// The provider stops reading flags at a bare "--", so the selected
		// files appended after it would be ignored. A "--" that is one word
		// of the server or Playwright command is that flag's value.
		if before := flagName(argv[index+1]); word == "--" && before != "server-arg" && before != "test-arg" {
			return fmt.Errorf("%s: a bare -- ends the provider's flags, so the selected files appended after it would be ignored; pass each command word as a flag value", field)
		}
	}
	if _, ok := providerDir(argv); runner == ProviderRunnerVitest && !ok {
		return fmt.Errorf("%s: --dir must be a repository-relative directory", field)
	}
	return nil
}

// flagName returns word without its one or two leading dashes, or "" when
// word is not a flag.
func flagName(word string) string {
	name := strings.TrimPrefix(strings.TrimPrefix(word, "-"), "-")
	if name == word || name == "" {
		return ""
	}
	return name
}

// providerDir returns the repository-relative slash directory whose Vitest
// suite a declared `unit` argv runs: the last --dir value, "." without one.
// Every `unit` flag but --retain takes a value, and flag parsing ends at the
// first word that is not a flag. It reports false for a --dir with no value
// or one that is absolute or leaves the repository.
func providerDir(argv []string) (string, bool) {
	dir := "."
	for index := 2; index < len(argv); index++ {
		word := flagName(argv[index])
		if word == "" {
			break
		}
		name, value, assigned := strings.Cut(word, "=")
		if !assigned && name != "retain" {
			if index++; index < len(argv) {
				value = argv[index]
			} else if name == "dir" {
				return "", false
			}
		}
		if name == "dir" {
			dir = value
		}
	}
	dir = filepath.ToSlash(dir)
	if dir == "" || path.IsAbs(dir) || filepath.IsAbs(dir) || strings.Contains(dir, ":") {
		return "", false
	}
	if dir = path.Clean(dir); dir == ".." || strings.HasPrefix(dir, "../") {
		return "", false
	}
	return dir, true
}

// languageCommand reads one language value: a non-empty argv array, or an
// object whose only member is a non-empty "none" reason.
func languageCommand(language string, value jsontext.Value) (LanguageCommand, error) {
	entry := LanguageCommand{Language: language}
	switch trimmed := bytes.TrimSpace(value); {
	case len(trimmed) != 0 && trimmed[0] == '[':
		if err := json.Unmarshal(trimmed, &entry.Argv); err != nil {
			return entry, fmt.Errorf("languages.%s: %w", language, err)
		}
		if len(entry.Argv) == 0 {
			return entry, fmt.Errorf("languages.%s: argv is empty", language)
		}
		if err := words("languages."+language, entry.Argv); err != nil {
			return entry, err
		}
	case len(trimmed) != 0 && trimmed[0] == '{':
		var none struct {
			None string `json:"none"`
		}
		if err := json.Unmarshal(trimmed, &none, json.RejectUnknownMembers(true)); err != nil {
			return entry, fmt.Errorf("languages.%s: %w", language, err)
		}
		if strings.TrimSpace(none.None) == "" {
			return entry, fmt.Errorf("languages.%s: none needs a reason", language)
		}
		entry.None = none.None
	default:
		return entry, fmt.Errorf("languages.%s: want an argv array or {\"none\": reason}", language)
	}
	return entry, nil
}

func words(field string, values []string) error {
	for _, value := range values {
		if !plain(value) {
			return fmt.Errorf("%s: every word must be non-empty without whitespace or control characters, got %q", field, value)
		}
	}
	return nil
}

// EnvName reports whether name is a portable environment variable name
// ([A-Za-z_][A-Za-z0-9_]*), the only shape a shell reads as an assignment
// and exec sets as a variable alike.
func EnvName(name string) bool {
	for i, r := range name {
		switch {
		case r == '_', r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z':
		case r >= '0' && r <= '9' && i != 0:
		default:
			return false
		}
	}
	return name != ""
}

// plain reports whether value is a non-empty word with no whitespace (of any
// script) and no control character, so it crosses exec and the gate's
// comparison unchanged.
func plain(value string) bool {
	if value == "" {
		return false
	}
	for _, r := range value {
		if unicode.IsSpace(r) || unicode.IsControl(r) || r == utf8.RuneError {
			return false
		}
	}
	return true
}
