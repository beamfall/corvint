// SPDX-License-Identifier: Apache-2.0
package contract

import (
	"encoding/json"
	"fmt"
	"path"
	"regexp"
	"strconv"
	"strings"
)

// ReasonJestUnsupported is printed when a Jest instance's setup is one
// Corvint cannot read or does not support, so a selection runs the full
// command (JST-V1-008).
const ReasonJestUnsupported = "jest-unsupported-config"

// MinJestMajor is the lowest Jest major version a selection narrows on
// (JST-V1-008; design Open 1, set by V1-5455): Jest 27 made jest-circus the
// default test runner, whose --json report carries ancestorTitles and
// duration for every assertion and which skips a block's hooks when all
// its tests are skipped, as the inventory's collection run needs.
const MinJestMajor = 27

// The test runners and runner whose --json report keeps one assertion
// result per test with its ancestorTitles, title, status and duration.
var (
	jestTestRunners = map[string]bool{"jest-circus": true, "jest-circus/runner": true, "jest-jasmine2": true}
	jestRunners     = map[string]bool{"jest-runner": true}
)

// JestSetup is what JestSetupFrom read of one Jest instance: the installed
// jest package version ("" when it could not be read) and the instance
// config, a nil Config when it could not be read.
type JestSetup struct {
	Version    string
	ConfigName string
	Config     []byte
}

// JestSetupFrom reads instance's setup through read, which returns the
// bytes of a repository-relative path: the version of the nearest
// node_modules/jest/package.json at or above the instance root, the
// module `npx jest` runs, and the instance config. The contract reads no
// file itself; read bounds what it reads.
func JestSetupFrom(instance Instance, read func(rel string) ([]byte, error)) JestSetup {
	setup := JestSetup{ConfigName: instance.Config}
	for dir := instance.Root; ; dir = path.Dir(dir) {
		if raw, err := read(path.Join(dir, "node_modules", "jest", "package.json")); err == nil {
			var manifest struct {
				Version string `json:"version"`
			}
			if json.Unmarshal(raw, &manifest) == nil {
				setup.Version = manifest.Version
			}
			break
		}
		if dir == "." || dir == "/" || dir == "" {
			break
		}
	}
	if instance.Config != "" {
		if raw, err := read(instance.Config); err == nil {
			setup.Config = raw
		}
	}
	return setup
}

// Unsupported is the cause a selection in this setup runs the full
// command, or "" when it narrows: a nil setup (not read), a version that
// cannot be read or is below MinJestMajor, a config that cannot be read,
// or a config that sets testResultsProcessor (which rewrites the --json
// report) or a testRunner or runner other than Jest's own. Jest's
// `reporters` do not change the --json report and are not refused. A
// JavaScript or TypeScript config is read as text: a value set through a
// preset, a spread or a computed object is not seen, and a report that
// then lacks a selected test leaves it NOT_RUN (JST-V1-005).
func (s *JestSetup) Unsupported() string {
	if s == nil {
		return "the Jest setup was not read"
	}
	major, ok := jestMajor(s.Version)
	switch {
	case s.Version == "":
		return "the installed Jest version cannot be read (no node_modules/jest/package.json at or above the instance root)"
	case !ok:
		return fmt.Sprintf("the installed Jest version %q cannot be read", s.Version)
	case major < MinJestMajor:
		return fmt.Sprintf("Jest %s is below the minimum major version %d", s.Version, MinJestMajor)
	case s.Config == nil:
		return "config " + s.ConfigName + " cannot be read"
	}
	switch name := path.Base(s.ConfigName); {
	case name == "package.json":
		var manifest map[string]json.RawMessage
		if json.Unmarshal(s.Config, &manifest) != nil || manifest["jest"] == nil {
			return "config " + s.ConfigName + " has no readable jest member"
		}
		return jsonConfigCause(s.ConfigName, manifest["jest"])
	case strings.HasSuffix(name, ".json"):
		return jsonConfigCause(s.ConfigName, s.Config)
	case jsConfig(name):
		return textConfigCause(s.ConfigName, string(s.Config))
	}
	return "config " + s.ConfigName + " is not a JSON, JavaScript or TypeScript file"
}

// jestMajor is the major number of a semantic version.
func jestMajor(version string) (int, bool) {
	head, _, _ := strings.Cut(version, ".")
	major, err := strconv.Atoi(head)
	return major, err == nil && major >= 0
}

func jsConfig(name string) bool {
	for _, suffix := range []string{".js", ".cjs", ".mjs", ".ts", ".cts", ".mts"} {
		if strings.HasSuffix(name, suffix) {
			return true
		}
	}
	return false
}

// jsonConfigCause checks a JSON config object and each inline project of
// its projects member; a project named by path is a config Corvint does
// not read.
func jsonConfigCause(name string, raw []byte) string {
	var config map[string]json.RawMessage
	if json.Unmarshal(raw, &config) != nil {
		return "config " + name + " is not a JSON object"
	}
	if cause := jsonMembersCause(name, config); cause != "" {
		return cause
	}
	if config["projects"] == nil {
		return ""
	}
	var projects []json.RawMessage
	if json.Unmarshal(config["projects"], &projects) != nil {
		return "config " + name + " has a projects member that is not a list"
	}
	for _, project := range projects {
		var inline map[string]json.RawMessage
		if json.Unmarshal(project, &inline) != nil {
			return "config " + name + " names a project config Corvint does not read"
		}
		if cause := jsonMembersCause(name, inline); cause != "" {
			return cause
		}
	}
	return ""
}

func jsonMembersCause(name string, config map[string]json.RawMessage) string {
	if config["testResultsProcessor"] != nil {
		return "config " + name + " sets testResultsProcessor, which rewrites the --json report"
	}
	for key, allowed := range map[string]map[string]bool{"testRunner": jestTestRunners, "runner": jestRunners} {
		if config[key] == nil {
			continue
		}
		var value string
		if json.Unmarshal(config[key], &value) != nil || !allowed[value] {
			return "config " + name + " sets " + key + " to " + string(config[key]) + ", not Jest's own"
		}
	}
	return ""
}

var (
	// jestTextKey finds the members a text config may set: testRunner and
	// testResultsProcessor anywhere, runner only as an object key.
	jestTextKey = regexp.MustCompile(`\btestRunner\b|\btestResultsProcessor\b|(?:^|[\s{,'"])runner['"]?\s*:`)
	// jestTextValue reads `KEY: 'literal'` after a key.
	jestTextValue = regexp.MustCompile(`^['"]?\s*:\s*(?:'([^'\n]*)'|"([^"\n]*)")`)
)

// textConfigCause checks a JavaScript or TypeScript config as text: every
// mention of testResultsProcessor is refused, and testRunner and runner
// narrow only when set to a string literal naming Jest's own.
func textConfigCause(name, text string) string {
	for _, match := range jestTextKey.FindAllStringIndex(text, -1) {
		key := strings.Trim(text[match[0]:match[1]], " \t\r\n{,'\":")
		rest := text[match[1]:]
		allowed := jestTestRunners
		switch {
		case key == "testResultsProcessor":
			return "config " + name + " mentions testResultsProcessor, which rewrites the --json report"
		case strings.HasSuffix(key, "runner"):
			// The runner pattern consumed the colon; put it back for the
			// value reader.
			key, allowed, rest = "runner", jestRunners, ":"+rest
		}
		value := jestTextValue.FindStringSubmatch(rest)
		if value == nil || !allowed[value[1]+value[2]] {
			return "config " + name + " sets " + key + " to a value that is not a string literal naming Jest's own"
		}
	}
	return ""
}
