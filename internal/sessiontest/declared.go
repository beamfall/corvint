package sessiontest

import (
	"bytes"
	"errors"
	"fmt"
	"path/filepath"
	"sort"
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

// Declared is the read declaration in the slice form the manifest digests:
// GoEnv as sorted NAME=VALUE words, GoFlags as argv words placed between
// `go test` and the package patterns, and Languages sorted by name. A
// declaration that cannot be read carries the reason in Unreadable and
// nothing else.
type Declared struct {
	GoEnv      []string          `json:"goEnv"`
	GoFlags    []string          `json:"goFlags"`
	Languages  []LanguageCommand `json:"languages"`
	Unreadable string            `json:"unreadable"`
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

// declarationFile is the on-disk schema: a closed set of members, with each
// language mapped to an argv array or to {"none": reason}.
type declarationFile struct {
	Profile   string                    `json:"profile"`
	Go        *declarationGo            `json:"go"`
	Languages map[string]jsontext.Value `json:"languages"`
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

func readDeclaration(name string) (Declared, error) {
	raw, err := readRegular(name, MaxDocumentBytes)
	if err != nil {
		return Declared{}, err
	}
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
	return declared, nil
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
