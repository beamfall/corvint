package java

import (
	"path"
	"strings"

	"github.com/Beamfall/corvint/internal/liveverify/affected/kotlin"
)

// jvmReferrer is the part of a Kotlin file needed to tell whether it can name
// a Java type, or be named by one: its package, imports, qualified names, words
// and file facade.
type jvmReferrer struct {
	packageName string
	imports     []string
	qualified   []string
	words       map[string]bool
	facade      string
}

// kotlinReferrer reads a Kotlin file without a Kotlin parser. Headers and
// qualified names come from the text with comments blanked by the Kotlin
// plugin's lexer, which nests block comments and reads raw strings, and from
// the raw text when it does not lex. Words come from the raw text, so a word
// inside a comment or string can only add a frontier, never remove one.
func kotlinReferrer(relative, text string) jvmReferrer {
	text = strings.TrimPrefix(text, "\ufeff")
	header := text
	if clean, ok := kotlin.SourceText(text); ok {
		header = clean
	}
	referrer := jvmReferrer{words: map[string]bool{}, facade: kotlinFacade(relative)}
	referrer.qualified = qualifiedReferences(chains(header))
	for _, line := range strings.Split(strings.ReplaceAll(header, "\r", "\n"), "\n") {
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		rest := fields[1:]
		for index, field := range rest {
			if field == "as" {
				rest = rest[:index]
				break
			}
		}
		name := strings.ReplaceAll(strings.Join(rest, ""), "`", "")
		name, _, _ = strings.Cut(name, ";")
		switch fields[0] {
		case "package":
			if referrer.packageName == "" {
				referrer.packageName = name
			}
		case "import":
			referrer.imports = append(referrer.imports, strings.TrimSuffix(name, ".*"))
		}
	}
	for _, token := range tokenize(text) {
		if identifier(token) {
			referrer.words[token] = true
		}
	}
	return referrer
}

// kotlinFacade is the JVM class holding a Kotlin file's top-level members:
// the capitalised file stem with Kt appended, other characters as _.
func kotlinFacade(relative string) string {
	stem := strings.TrimSuffix(strings.TrimSuffix(path.Base(relative), ".kts"), ".kt")
	var facade strings.Builder
	for index, r := range stem {
		switch {
		case index == 0 && r >= 'a' && r <= 'z':
			facade.WriteRune(r - 'a' + 'A')
		case identifierRune(r) && r != '$':
			facade.WriteRune(r)
		default:
			facade.WriteByte('_')
		}
	}
	return facade.String() + "Kt"
}

// markKotlinDependents raises FrontierKotlinDependent on a Java unit when a
// Kotlin file can reach one of its top-level types: it imports the type (an
// alias import needs no later mention), or it shares, imports from or
// qualifies into the type's package and mentions the type's simple name.
func markKotlinDependents(observed *observation, kotlin []jvmReferrer) {
	packageName := observed.packageName
	for _, referrer := range kotlin {
		for _, name := range observed.types {
			if strings.Contains(name, ".") {
				continue
			}
			full := qualify(packageName, name)
			if referencesType(referrer, packageName, full, name) {
				observed.unit.Frontier = []string{FrontierKotlinDependent}
				return
			}
		}
	}
}

func referencesType(referrer jvmReferrer, packageName, full, name string) bool {
	for _, imported := range referrer.imports {
		if imported == full || strings.HasPrefix(imported, full+".") {
			return true
		}
	}
	if !referrer.words[name] {
		return false
	}
	if referrer.packageName == packageName {
		return true
	}
	for _, imported := range referrer.imports {
		if imported == packageName {
			return true
		}
	}
	for _, qualified := range referrer.qualified {
		if strings.HasPrefix(qualified, full) {
			return true
		}
	}
	return false
}
