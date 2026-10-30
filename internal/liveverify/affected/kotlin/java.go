package kotlin

import (
	"path"
	"regexp"
	"strings"
	"unicode"

	"github.com/Beamfall/corvint/internal/liveverify/affected/java/javalex"
)

// javaReferrer is the part of a Java file needed to tell whether it can name a
// Kotlin declaration: its package, its imports, its qualified names and words.
type javaReferrer struct {
	packageName string
	imports     []string
	qualified   []string
	words       map[string]bool
}

// observeJava reads a Java file without a Java parser. Headers and qualified
// names come from the text with comments blanked by the Java lexer when it
// lexes, and from the raw text otherwise; qualified names tolerate whitespace
// and comments around each dot. Words come from the raw text, so a word inside
// a comment or string can only add a frontier, never remove one.
func observeJava(text string) javaReferrer {
	text = strings.TrimPrefix(text, "\ufeff")
	header := text
	// Java text is lexed as javac does: Unicode escapes are translated,
	// CR ends a line comment and block comments do not nest.
	if clean, _, ok := javalex.Blank(text); ok {
		header = clean
	}
	referrer := javaReferrer{words: map[string]bool{}}
	for _, statement := range strings.Split(header, ";") {
		fields := strings.Fields(statement)
		if len(fields) < 2 {
			continue
		}
		switch fields[0] {
		case "package":
			if referrer.packageName == "" {
				referrer.packageName = strings.Join(fields[1:], "")
			}
		case "import":
			rest := fields[1:]
			if rest[0] == "static" && len(rest) > 1 {
				rest = rest[1:]
			}
			referrer.imports = append(referrer.imports, strings.TrimSuffix(strings.Join(rest, ""), ".*"))
		}
	}
	for _, field := range strings.FieldsFunc(spacedDots.ReplaceAllString(header, "."), func(r rune) bool { return !identifierRune(r) && r != '.' }) {
		if field = strings.Trim(field, "."); strings.Contains(field, ".") {
			referrer.qualified = append(referrer.qualified, field)
		}
	}
	for _, field := range strings.FieldsFunc(text, func(r rune) bool { return !identifierRune(r) }) {
		referrer.words[field] = true
	}
	return referrer
}

// spacedDots matches a dot with the whitespace around it, which Java allows
// inside a qualified name.
var spacedDots = regexp.MustCompile(`\s*\.\s*`)

// fileJvmName matches the @file:JvmName annotation, simple or qualified, up to
// its opening parenthesis.
var fileJvmName = regexp.MustCompile(`@file\s*:\s*(?:kotlin\s*\.\s*jvm\s*\.\s*)?JvmName\s*\(`)

// fileJvmNameValue reads the @file:JvmName value. The annotation is matched in
// the text with comments blanked, so a comment may sit anywhere inside it, and
// the string is read from the raw text at the same offset, which the lexer
// keeps. Text that does not lex is matched raw.
func fileJvmNameValue(text string) (string, bool) {
	if !strings.Contains(text, "JvmName") {
		return "", false
	}
	clean, ok := sourceText(text)
	if !ok {
		clean = text
	}
	for _, match := range fileJvmName.FindAllStringIndex(clean, -1) {
		at := match[1]
		for at < len(text) && text[at] != '"' && unicode.IsSpace(rune(clean[at])) {
			at++
		}
		if at >= len(text) || text[at] != '"' {
			continue
		}
		if end := strings.IndexByte(text[at+1:], '"'); end > 0 {
			return text[at+1 : at+1+end], true
		}
	}
	return "", false
}

// markJavaDependents gives FrontierJavaDependent to every Kotlin unit a Java
// file can reach. Java resolves Kotlin declarations by their JVM names: a class
// by its own name and a top-level function or property through its file facade
// (FileKt, or the @file:JvmName value). The Java dependents belong to another
// plugin and no edge crosses namespaces, so a reached unit keeps scope UNKNOWN.
func markJavaDependents(observations []sourceObservation, java []javaReferrer, texts map[string]string) {
	if len(java) == 0 {
		return
	}
	for index := range observations {
		observation := &observations[index]
		if observation.config {
			continue
		}
		names := jvmNames(observation, texts[observation.relative])
		for _, referrer := range java {
			if referencesDeclaration(referrer, observation.packageName, names) {
				observation.unit.Frontier = []string{FrontierJavaDependent}
				break
			}
		}
	}
}

func jvmNames(observation *sourceObservation, text string) []string {
	names := []string{fileFacade(observation.relative)}
	for _, alias := range observation.aliases {
		names = append(names, alias[strings.LastIndex(alias, ".")+1:])
	}
	if name, ok := fileJvmNameValue(text); ok {
		names = append(names, name)
	}
	return names
}

// fileFacade is the JVM class holding a file's top-level members: the
// capitalised file stem with Kt appended, other characters as _.
func fileFacade(relative string) string {
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

func referencesDeclaration(referrer javaReferrer, packageName string, names []string) bool {
	for _, name := range names {
		full := name
		if packageName != "" {
			full = packageName + "." + name
		}
		for _, imported := range referrer.imports {
			if imported == full || strings.HasPrefix(imported, full+".") {
				return true
			}
		}
		if !referrer.words[name] {
			continue
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
			if qualified == full || strings.HasPrefix(qualified, full+".") {
				return true
			}
		}
	}
	return false
}
