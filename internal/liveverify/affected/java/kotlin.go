package java

import "strings"

// jvmReferrer is the part of a Kotlin file needed to tell whether it can name
// a Java type: its package, its imports, its qualified names and its words.
type jvmReferrer struct {
	packageName string
	imports     []string
	qualified   []string
	words       map[string]bool
}

// kotlinReferrer reads a Kotlin file without lexing it. Words inside comments
// and strings are kept, which can only add a frontier, never remove one.
func kotlinReferrer(text string) jvmReferrer {
	referrer := jvmReferrer{words: map[string]bool{}, qualified: qualifiedReferences(text)}
	for _, line := range strings.Split(text, "\n") {
		fields := strings.Fields(strings.TrimPrefix(line, "\ufeff"))
		if len(fields) < 2 {
			continue
		}
		switch fields[0] {
		case "package":
			if referrer.packageName == "" {
				referrer.packageName = strings.Trim(fields[1], "`;")
			}
		case "import":
			referrer.imports = append(referrer.imports, strings.TrimSuffix(strings.Trim(fields[1], "`;"), ".*"))
		}
	}
	for _, token := range tokenize(text) {
		if identifier(token) {
			referrer.words[token] = true
		}
	}
	return referrer
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
