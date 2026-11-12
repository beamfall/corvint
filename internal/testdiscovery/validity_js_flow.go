package testdiscovery

import (
	"strings"
)

// JSTestLibrary reports a test-framework module, whose calls never reach the code under test.
func JSTestLibrary(spec string) bool {
	return strings.HasPrefix(spec, "@testing-library/") || spec == "vitest" || spec == "@jest/globals" || spec == "jest" || spec == "bun:test"
}
