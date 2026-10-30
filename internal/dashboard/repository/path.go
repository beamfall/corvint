package repository

import (
	"path"
	"path/filepath"
	"runtime"
	"strings"
	"unicode"
	"unicode/utf8"
)

const (
	maxArgumentBytes = 4_096
	maxDirtyPaths    = 100_000
)

func validRoot(root string) bool {
	if runtime.GOOS != "darwin" && runtime.GOOS != "linux" && runtime.GOOS != "windows" {
		return false
	}
	if root == "" || len(root) > maxArgumentBytes || !utf8.ValidString(root) ||
		!filepath.IsAbs(root) || filepath.Clean(root) != root {
		return false
	}
	for _, character := range root {
		if character == 0 || unicode.IsControl(character) {
			return false
		}
	}
	return true
}

func validNativeAbsolutePath(value string) bool {
	if value == "" || !utf8.ValidString(value) || !filepath.IsAbs(value) || filepath.Clean(value) != value {
		return false
	}
	for _, character := range value {
		if character == 0 || unicode.IsControl(character) {
			return false
		}
	}
	return true
}

func validTracePath(value string) bool {
	if value == "" || len(value) > maxArgumentBytes || !utf8.ValidString(value) ||
		path.IsAbs(value) || path.Clean(value) != value || strings.Contains(value, "\\") || hasVolumePrefix(value) {
		return false
	}
	for _, component := range strings.Split(value, "/") {
		if component == "" || component == "." || component == ".." {
			return false
		}
	}
	for _, character := range value {
		if character == 0 || unicode.IsControl(character) {
			return false
		}
	}
	return true
}

func hasVolumePrefix(value string) bool {
	return len(value) >= 2 && ((value[1] == ':' && ((value[0] >= 'A' && value[0] <= 'Z') || (value[0] >= 'a' && value[0] <= 'z'))) || strings.HasPrefix(value, "//"))
}
