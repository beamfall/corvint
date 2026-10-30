package analyzerdotnet

import "strings"

// The closed lexical atoms of the .NET matrix. Every value outside these sets
// is UNSUPPORTED_SCHEMA rather than normalized into range.

func dotnetName(value string) bool {
	if value == "" || over(len(value), MaxIdentifierBytes) {
		return false
	}
	if !asciiAlphanumeric(value[0]) {
		return false
	}
	for index := 0; index < len(value); index++ {
		character := value[index]
		if !asciiAlphanumeric(character) && !strings.ContainsRune("._+-", rune(character)) {
			return false
		}
	}
	return true
}

func asciiAlphanumeric(character byte) bool {
	switch {
	case character >= 'A' && character <= 'Z':
		return true
	case character >= 'a' && character <= 'z':
		return true
	case character >= '0' && character <= '9':
		return true
	}
	return false
}

func dotnetTFM(value string) bool { return value == "net8.0" || value == "net9.0" }

func dotnetRID(value string) bool {
	return value == "osx-arm64" || value == "linux-x64" || value == "win-x64"
}

func dotnetLang(value string) bool { return value == "12.0" || value == "13.0" }

func dotnetBoolean(value string) bool { return value == "true" || value == "false" }

func dotnetSDKVersion(value string) bool { return value == "8.0.423" || value == "9.0.316" }

// coreVersion is NUMBER.NUMBER.NUMBER where NUMBER is 0 or a nonzero digit run.
// No prefix, leading zero, prerelease, build, whitespace, or alternate spelling.
func coreVersion(value string) bool {
	parts := strings.Split(value, ".")
	if len(parts) != 3 {
		return false
	}
	for _, part := range parts {
		if !decimalNumber(part) {
			return false
		}
	}
	return true
}

func decimalNumber(value string) bool {
	if value == "" || over(len(value), 16) {
		return false
	}
	if value[0] == '0' {
		return len(value) == 1
	}
	for index := 0; index < len(value); index++ {
		if value[index] < '0' || value[index] > '9' {
			return false
		}
	}
	return true
}

// semicolonList splits a strictly sorted, duplicate-free, nonempty list of one
// atom. Sortedness is required rather than imposed: two orderings of the same
// set would otherwise be two spellings of one declaration.
func semicolonList(value string, admits func(string) bool) ([]string, bool) {
	if value == "" {
		return nil, false
	}
	parts := strings.Split(value, ";")
	for index, part := range parts {
		if !admits(part) {
			return nil, false
		}
		if index > 0 && parts[index-1] >= part {
			return nil, false
		}
	}
	return parts, true
}

// baseDirectory is the declaring input's parent directory, the anchor every
// relative Include in that input resolves against.
func baseDirectory(inputPath string) []string {
	segments := strings.Split(inputPath, "/")
	return segments[:len(segments)-1]
}

// resolveRelative turns one MSBuild relative Include into a logical path.
//
// A backslash rejects. MSBuild spells separators `\` by convention, but the
// profile's logical-path grammar forbids a backslash and forbids normalization
// aliases, so rewriting `..\Lib\Lib.csproj` into a logical path would mint a
// second spelling of one path. This profile is exact, not compatible.
func resolveRelative(base []string, raw string) (string, string) {
	if raw == "" || over(len(raw), MaxPathBytes) {
		return "", "INVALID_PATH"
	}
	if strings.ContainsAny(raw, "\\:%$*?[]") || strings.HasPrefix(raw, "/") || strings.HasSuffix(raw, "/") {
		return "", "INVALID_PATH"
	}
	resolved := append([]string(nil), base...)
	for _, segment := range strings.Split(raw, "/") {
		switch segment {
		case "":
			return "", "INVALID_PATH"
		case ".":
			continue
		case "..":
			if len(resolved) == 0 {
				return "", "INVALID_PATH"
			}
			resolved = resolved[:len(resolved)-1]
		default:
			if !pathSegmentOK(segment) {
				return "", "INVALID_PATH"
			}
			resolved = append(resolved, segment)
		}
	}
	joined := strings.Join(resolved, "/")
	if !pathOK(joined) {
		return "", "INVALID_PATH"
	}
	return joined, ""
}
