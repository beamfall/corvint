package main

import (
	"regexp"
	"runtime/debug"
)

// producerUnknown is the explicit marker for an identity member this binary
// cannot state (PUB-V0-028). It is never a guessed value.
const producerUnknown = "unknown"

// producerUnstamped is the build member of a binary built without the
// -ldflags "-X main.build=N" stamp, which --version prints as build 0
// (PUB-V0-021, PUB-V0-029).
const producerUnstamped = "unstamped"

var producerRevision = regexp.MustCompile(`^(?:[0-9a-f]{40}|[0-9a-f]{64})$`)

// producerIdentity is the opt-in `producer` member of the impact and
// test-validity receipts (PUB-V0-027). Every member is a string: a stated
// value or an explicit marker, never an inferred one.
type producerIdentity struct {
	Version     string `json:"version"`
	Build       string `json:"build"`
	Commit      string `json:"commit"`
	CommitState string `json:"commitState"`
}

// currentProducer reads this binary's identity: the compiled VERSION, the
// stamped build number, and the Go toolchain's vcs build settings.
func currentProducer() producerIdentity {
	info, _ := debug.ReadBuildInfo()
	return producerFrom(version, build, info)
}

// producerFrom states each member from its own source only (PUB-V0-028). A
// release archive is built with -buildvcs=false, so its commit is unknown
// here and is named by the release manifest instead (PUB-V0-029).
func producerFrom(versionValue, buildValue string, info *debug.BuildInfo) producerIdentity {
	result := producerIdentity{Version: versionValue, Build: buildValue, Commit: producerUnknown, CommitState: producerUnknown}
	if buildValue == "" || buildValue == "0" {
		result.Build = producerUnstamped
	}
	if info == nil {
		return result
	}
	settings := map[string]string{}
	for _, setting := range info.Settings {
		settings[setting.Key] = setting.Value
	}
	if settings["vcs"] != "git" || !producerRevision.MatchString(settings["vcs.revision"]) {
		return result
	}
	result.Commit = settings["vcs.revision"]
	switch settings["vcs.modified"] {
	case "false":
		result.CommitState = "clean"
	case "true":
		result.CommitState = "modified"
	}
	return result
}

// members is the map form that the impact receipt's canonical encoder reads.
func (identity producerIdentity) members() map[string]any {
	return map[string]any{"version": identity.Version, "build": identity.Build, "commit": identity.Commit, "commitState": identity.CommitState}
}
