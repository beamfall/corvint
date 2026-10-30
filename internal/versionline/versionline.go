// Package versionline formats and tolerantly parses the plain `--version` line
// `NAME VERSION (build N, commit SHA[, modified])` (CRB-V0-003, PUB-V0-021,
// PUB-V0-053). A consumer reads the version prefix and the members it needs
// and ignores members it does not know, so a later member cannot break it.
package versionline

import (
	"regexp"
	"strings"
)

// Line is one parsed version line. Build and Commit are empty when absent.
type Line struct {
	Name, Version, Build, Commit string
	Modified                     bool
}

var (
	buildMember  = regexp.MustCompile(`^build (0|[1-9][0-9]*)$`)
	commitMember = regexp.MustCompile(`^commit ([0-9a-f]{40}|[0-9a-f]{64}|unknown)$`)
)

// Format renders the line. build "" or "0" is unstamped and omitted; commit ""
// is written as `commit unknown`.
func Format(name, version, build, commit string, modified bool) string {
	members := []string{}
	if build != "" && build != "0" {
		members = append(members, "build "+build)
	}
	if commit == "" {
		commit = "unknown"
	}
	members = append(members, "commit "+commit)
	if modified {
		members = append(members, "modified")
	}
	return name + " " + version + " (" + strings.Join(members, ", ") + ")"
}

// Parse reads `NAME VERSION (MEMBER, ...)`, the earlier `NAME VERSION (build N)`
// included. A malformed build or commit member is a parse failure; an unknown
// member is ignored.
func Parse(text string) (Line, bool) {
	var line Line
	text = strings.TrimSpace(text)
	open := strings.Index(text, " (")
	if open < 0 || !strings.HasSuffix(text, ")") || strings.ContainsAny(text, "\n\r") {
		return line, false
	}
	head := strings.Fields(text[:open])
	if len(head) != 2 {
		return line, false
	}
	line.Name, line.Version = head[0], head[1]
	for _, member := range strings.Split(text[open+2:len(text)-1], ", ") {
		switch {
		case strings.HasPrefix(member, "build"):
			match := buildMember.FindStringSubmatch(member)
			if match == nil || line.Build != "" {
				return Line{}, false
			}
			line.Build = match[1]
		case strings.HasPrefix(member, "commit"):
			match := commitMember.FindStringSubmatch(member)
			if match == nil || line.Commit != "" {
				return Line{}, false
			}
			line.Commit = match[1]
		case member == "modified":
			line.Modified = true
		}
	}
	return line, true
}

// DevelopmentVersion names an unreleased build of version as a pre-release of
// it (PUB-V0-052): 1.0.0 becomes 1.0.0-dev and 1.0.0-rc.3 becomes 1.0.0-rc.3.dev.
func DevelopmentVersion(version string) string {
	if strings.Contains(version, "-") {
		return version + ".dev"
	}
	return version + "-dev"
}

// SameRelease reports whether observed is release or its development form.
func SameRelease(observed, release string) bool {
	return observed == release || observed == DevelopmentVersion(release)
}
