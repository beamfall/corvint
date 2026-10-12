package kotlin

import (
	"path"
	"strings"
)

// javaReferrer is the part of a Java file needed to tell whether it can name a
// Kotlin declaration: its package, its imports, its qualified names and words.
type javaReferrer struct {
	packageName string
	imports     []string
	qualified   []string
	words       map[string]bool
}

// observeJava reads a Java file without lexing it. Words inside comments and
// strings are kept, which can only add a frontier, never remove one.
func observeJava(text string) javaReferrer {
	referrer := javaReferrer{words: map[string]bool{}}
	for _, statement := range strings.Split(text, ";") {
		fields := strings.Fields(strings.TrimPrefix(strings.TrimSpace(statement), "\ufeff"))
		if len(fields) < 2 {
			continue
		}
		switch fields[0] {
		case "package":
			if referrer.packageName == "" {
				referrer.packageName = fields[1]
			}
		case "import":
			name := fields[1]
			if name == "static" && len(fields) > 2 {
				name = fields[2]
			}
			referrer.imports = append(referrer.imports, strings.TrimSuffix(name, ".*"))
		}
	}
	for _, field := range strings.FieldsFunc(text, func(r rune) bool { return !identifierRune(r) && r != '.' }) {
		field = strings.Trim(field, ".")
		if strings.Contains(field, ".") {
			referrer.qualified = append(referrer.qualified, field)
		}
		for _, word := range strings.Split(field, ".") {
			referrer.words[word] = true
		}
	}
	return referrer
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
	names := []string{strings.TrimSuffix(strings.TrimSuffix(path.Base(observation.relative), ".kts"), ".kt") + "Kt"}
	for _, alias := range observation.aliases {
		names = append(names, alias[strings.LastIndex(alias, ".")+1:])
	}
	if _, rest, ok := strings.Cut(text, "@file:JvmName("); ok {
		if value, _, ok := strings.Cut(rest, ")"); ok {
			names = append(names, strings.Trim(strings.TrimSpace(value), "\""))
		}
	}
	return names
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
