// SPDX-License-Identifier: Apache-2.0
package testid

import "strings"

// FromUnit reads a legacy execution-unit key at its own granularity
// (CTI-V1-012): `go:<import path>` is the Go package-level ID of the
// directory goDir resolves, `typescript:<runner>:<path>` for Vitest, Jest
// or Playwright is the file-level ID `<runner>:<path>`, and
// `ruby:rspec:<stem>`, `ruby:minitest:<stem>` and `ruby:rails-test:<stem>`
// are the file-level IDs of `<stem>.rb` (the Ruby plugin drops the
// extension; a Rails test is a Minitest test). Any other key, such as a
// `python:<module>` unit, which names a module rather than a file, or a
// nested-module `go-module:<dir>` unit, which names a runner instance, has
// no ID and false is returned; its reader keeps the unit key.
func FromUnit(unit string, goDir func(importPath string) (string, bool)) (ID, bool) {
	language, rest, ok := strings.Cut(unit, ":")
	if !ok {
		return ID{}, false
	}
	var id ID
	var err error
	switch language {
	case "go":
		if goDir == nil {
			return ID{}, false
		}
		dir, ok := goDir(rest)
		if !ok {
			return ID{}, false
		}
		id, err = New(Go, dir)
	case "typescript":
		runner, file, ok := strings.Cut(rest, ":")
		if !ok || (runner != Vitest && runner != Jest && runner != Playwright) {
			return ID{}, false
		}
		id, err = New(runner, file)
	case "ruby":
		kind, stem, ok := strings.Cut(rest, ":")
		runner := map[string]string{"rspec": RSpec, "minitest": Minitest, "rails-test": Minitest}[kind]
		if !ok || runner == "" || stem == "" {
			return ID{}, false
		}
		id, err = New(runner, stem+".rb")
	default:
		return ID{}, false
	}
	return id, err == nil
}
