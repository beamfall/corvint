package secretscreen

import (
	"encoding/json"
	"path"
	"regexp"
	"strings"
)

var argvCredentialFlag = regexp.MustCompile(`(?i)^--?(?:[a-z0-9]+[_-])*(?:` + writerAssignmentNames + `|pass)$`)

// MatchArgv screens literal decoded arguments, then credential flag/value
// boundaries. It does not tokenize, execute, expand variables or guess opaque
// encodings. A named credential flag makes its whole nonempty value sensitive.
func MatchArgv(argv []string) bool {
	for _, arg := range argv {
		if MatchString(arg) {
			return true
		}
	}
	encoded, _ := json.Marshal(argv)
	if MatchString(string(encoded)) {
		return true
	}
	login, curl, user := false, false, false
	for i, arg := range argv {
		lower := strings.ToLower(arg)
		if lower == "login" {
			login = true
		}
		if i == 0 && strings.ToLower(path.Base(arg)) == "curl" {
			curl = true
		}
		name, value, inline := strings.Cut(lower, "=")
		if !inline && i+1 < len(argv) {
			value = argv[i+1]
		}
		if argvCredentialFlag.MatchString(name) && value != "" {
			return true
		}
		if name == "-u" || name == "--user" {
			user = true
			if curl && strings.Contains(value, ":") {
				return true
			}
		}
		if name == "-p" && (login || (curl && user)) && value != "" {
			return true
		}
		// Scheme and credential may themselves be separate literal arguments.
		if lower == "bearer" && i+1 < len(argv) && MatchString("bearer "+argv[i+1]) {
			return true
		}
	}
	return false
}
