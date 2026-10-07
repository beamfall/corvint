// Package rootalias holds the operator-declared repository root spelling shared by multi-root
// commands: `ALIAS=ABSOLUTE_ROOT` (docs/specs/mcp-multi-root-v0.md, MMR-V0-001, MMR-V0-002) and the
// MCPV0-001 bounds every declared root keeps. The application map's second root (AMAP-V0-016)
// reuses it unchanged.
package rootalias

import (
	"path/filepath"
	"strings"
	"unicode"
	"unicode/utf8"
)

const (
	// MaxRoots caps the repositories one command may declare (MMR-V0-002).
	MaxRoots = 32
	// MaxAliasBytes bounds one alias; aliases are [a-z][a-z0-9-]* (MMR-V0-002).
	MaxAliasBytes = 32
	// maxRootBytes bounds one declared root (MCPV0-001).
	maxRootBytes = 4096
)

// Split reads ALIAS=ABSOLUTE_ROOT. A value is aliased only when the text before its first '=' is a
// valid alias; an absolute path starts with '/' or a drive letter followed by ':', neither of which
// an alias admits, so no plain absolute root is ever read as an alias.
func Split(value string) (alias, root string, aliased bool) {
	alias, root, found := strings.Cut(value, "=")
	if !found || !Valid(alias) {
		return "", value, false
	}
	return alias, root, true
}

// Valid reports whether alias matches ^[a-z][a-z0-9-]{0,31}$ (MMR-V0-002).
func Valid(alias string) bool {
	if alias == "" || len(alias) > MaxAliasBytes || alias[0] < 'a' || alias[0] > 'z' {
		return false
	}
	for index := 1; index < len(alias); index++ {
		character := alias[index]
		if (character < 'a' || character > 'z') && (character < '0' || character > '9') && character != '-' {
			return false
		}
	}
	return true
}

// ValidRoot reports whether root keeps the MCPV0-001 bounds: absolute, clean, valid UTF-8 of at
// most 4,096 bytes, and free of control characters.
func ValidRoot(root string) bool {
	if root == "" || len(root) > maxRootBytes || !utf8.ValidString(root) || !filepath.IsAbs(root) || filepath.Clean(root) != root {
		return false
	}
	for _, character := range root {
		if character == 0 || unicode.IsControl(character) {
			return false
		}
	}
	return true
}
