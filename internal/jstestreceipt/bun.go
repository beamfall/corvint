package jstestreceipt

import (
	"errors"
)

// ProducerBuild is the provider binary's own build identity (LPCV-V0-256).
type ProducerBuild struct {
	Name      string `json:"name"`
	Module    string `json:"module"`
	Revision  string `json:"revision"`
	Modified  bool   `json:"modified"`
	GoVersion string `json:"goVersion"`
}

// BunRun is the bun profile's receipt member (LPCV-V0-256): what ran, against which bun and
// lockfile, with which preloads, concurrency and declared services.
type BunRun struct {
	Producer            ProducerBuild     `json:"producer"`
	Executable          string            `json:"executable"`
	ExecutableDigest    string            `json:"executableDigest"`
	Version             string            `json:"version"`
	Lockfile            string            `json:"lockfile"`
	LockfileDigest      string            `json:"lockfileDigest"`
	LockfileVersion     int               `json:"lockfileVersion"`
	Preload             []string          `json:"preload"`
	PreloadDigests      map[string]string `json:"preloadDigests"`
	MaxConcurrency      int               `json:"maxConcurrency"`
	TestNamePattern     string            `json:"testNamePattern,omitempty"`
	Isolation           string            `json:"isolation,omitempty"`
	Services            []string          `json:"services,omitempty"`
	ServiceBoundary     string            `json:"serviceBoundary,omitempty"`
	ExitObserved        bool              `json:"exitObserved"`
	ExitStatus          int               `json:"exitStatus"`
	RepositoryUnchanged *bool             `json:"repositoryUnchanged,omitempty"`
}

// bunMemberShapeError refuses a bun member on any receipt but an unprofiled unit one
// (LPCV-V0-256); otherwise it returns next.
func bunMemberShapeError(r Receipt, next error) error {
	if r.Bun != nil && (r.Profile != "" || r.Kind != "unit") {
		return errors.New("bun-member-requires-unit-default-profile")
	}
	return next
}
