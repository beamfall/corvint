// Package projectprofile holds the downstream-project conventions the kernel
// used to hardcode: how a project is recognised, which ledgers it keeps, and
// which verification command each changed path implies.  Kernel packages read
// this table; they never name a project.
package projectprofile

import "strings"

// Command maps a changed path to the verification command a profile prescribes
// for it.
type Command struct {
	Match  func(path string) bool
	Render func(path string) string
}

// Profile describes one downstream project.  Signals are the paths that must
// all be present for the profile to be recognised; FeaturesPath and
// ScenariosPath name its record ledgers; ADRPrefix is the directory its
// decision records live under; Gate is the command that closes a verification
// plan; Commands are its path-specific verification rules.
//
// WebAliasPrefix and WebAliasRoot carry the one bundler convention a
// reverse-import resolution needs and cannot derive: the module-specifier
// prefix a project's build tooling rewrites, and the repository-relative
// directory it rewrites it to.  `GPK-V0-027` requires that pair to be read
// from here and NOT hardcoded to any one repository's source root, so a
// profile that declares no alias resolves no aliased specifier at all rather
// than borrowing another project's root.
type Profile struct {
	ID             string
	Signals        []string
	FeaturesPath   string
	ScenariosPath  string
	ADRPrefix      string
	Gate           string
	WebAliasPrefix string
	WebAliasRoot   string
	Commands       []Command
}

// profiles is ordered most specific first; the last row is the fallback and
// must carry no signals.
var profiles = []Profile{
	{
		ID:            "beamfall",
		Signals:       []string{"testing/features.yaml", "testing/scenarios.yaml"},
		FeaturesPath:  "testing/features.yaml",
		ScenariosPath: "testing/scenarios.yaml",
		ADRPrefix:     "docs/adr/",
		Gate:          "make gate",
		// The Vite/tsconfig alias this project's web application declares.
		// It is data about beamfall, not a kernel default: the fallback
		// profile below leaves both fields empty, which is what keeps
		// `@/x` from resolving into beamfall's tree from an unrelated
		// repository.
		WebAliasPrefix: "@/",
		WebAliasRoot:   "internal/web/app/src/",
		Commands: []Command{
			{
				Match:  func(path string) bool { return strings.HasPrefix(path, "internal/web/app/") },
				Render: func(string) string { return "npm --prefix internal/web/app test" },
			},
			{
				Match: func(path string) bool {
					return strings.HasPrefix(path, "internal/") &&
						strings.Contains(strings.TrimPrefix(path, "internal/"), "/")
				},
				Render: func(path string) string {
					parts := strings.SplitN(path, "/", 3)
					return "go test ./internal/" + parts[1] + "/..."
				},
			},
			{
				Match:  func(path string) bool { return path == "testing/features.yaml" },
				Render: func(string) string { return "make coverage-ratchet" },
			},
			{
				Match:  func(path string) bool { return path == "testing/scenarios.yaml" },
				Render: func(string) string { return "make scenario-ledger STRICT=1" },
			},
			{
				Match:  func(path string) bool { return strings.HasPrefix(path, "script/") },
				Render: func(path string) string { return "script/tests-for-change.sh " + path },
			},
		},
	},
	{ID: "generic", Gate: "git diff --check"},
}

// Fallback is the profile used when no signalled profile is recognised.
func Fallback() Profile {
	return profiles[len(profiles)-1]
}

// Detect returns the first profile all of whose signals are present.
func Detect(present func(path string) bool) Profile {
	for _, profile := range profiles {
		if len(profile.Signals) != 0 && allPresent(profile.Signals, present) {
			return profile
		}
	}
	return Fallback()
}

// ByID returns the named profile, or the fallback when the name is unknown.
func ByID(id string) Profile {
	for _, profile := range profiles {
		if profile.ID == id {
			return profile
		}
	}
	return Fallback()
}

// Known reports whether id names a profile in the table.
func Known(id string) bool {
	for _, profile := range profiles {
		if profile.ID == id {
			return true
		}
	}
	return false
}

// Signals returns every path any profile is recognised by, in table order.
func Signals() []string {
	result := make([]string, 0)
	for _, profile := range profiles {
		for _, signal := range profile.Signals {
			if !contains(result, signal) {
				result = append(result, signal)
			}
		}
	}
	return result
}

// Prescribe returns the verification command a profile prescribes for path,
// together with the profile that prescribes it.
func Prescribe(path string) (string, Profile, bool) {
	for _, profile := range profiles {
		for _, command := range profile.Commands {
			if command.Match(path) {
				return command.Render(path), profile, true
			}
		}
	}
	return "", Fallback(), false
}

func allPresent(signals []string, present func(path string) bool) bool {
	for _, signal := range signals {
		if !present(signal) {
			return false
		}
	}
	return true
}

func contains(values []string, candidate string) bool {
	for _, value := range values {
		if value == candidate {
			return true
		}
	}
	return false
}
