package postmergehost

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"regexp"
	"sort"
	"strings"
)

// GraphProfile is the only accepted workflow-graph wire profile.
const GraphProfile = "corvint-postmerge-host-graph/0"

// CredentialClass names one host-owned credential kind (PCH-V0-001).
type CredentialClass struct {
	ID           string `json:"id"`
	SecretPrefix string `json:"secretPrefix"`
	OutwardWrite bool   `json:"outwardWrite"`
	Meaning      string `json:"meaning"`
}

// Step is one node of the host-neutral post-merge workflow graph.
type Step struct {
	ID              string   `json:"id"`
	Order           int      `json:"order"`
	After           []string `json:"after"`
	Credentials     []string `json:"credentials"`
	Commands        []string `json:"commands"`
	PendingCommands []string `json:"pendingCommands,omitempty"`
	Attestation     string   `json:"attestation"`
}

// Graph is the decoded protocol/postmerge-host/workflow-graph.json.
type Graph struct {
	Profile           string            `json:"profile"`
	StepMarker        string            `json:"stepMarker"`
	CredentialClasses []CredentialClass `json:"credentialClasses"`
	Steps             []Step            `json:"steps"`
}

// RequiredSteps is the issue-ordered step vocabulary the graph must declare.
var RequiredSteps = []string{
	"trigger", "intake", "delta", "follow-up-item", "authoring",
	"trusted-validation", "draft-change-requests", "findings", "metrics",
}

var (
	idPattern      = regexp.MustCompile(`^[a-z][a-z0-9-]{0,63}$`)
	prefixPattern  = regexp.MustCompile(`^CORVINT_PM_[A-Z_]+_$`)
	commandPattern = regexp.MustCompile(`^corvint(-[a-z]+)*( [a-z][a-z-]*){1,2}$`)
)

// ParseGraph strictly decodes and validates a workflow graph.
func ParseGraph(data []byte) (*Graph, error) {
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	var g Graph
	if err := dec.Decode(&g); err != nil {
		return nil, fmt.Errorf("graph-invalid: %w", err)
	}
	if _, err := dec.Token(); err != io.EOF {
		return nil, fmt.Errorf("graph-invalid: trailing data")
	}
	return &g, g.validate()
}

func (g *Graph) validate() error {
	bad := func(format string, args ...any) error {
		return fmt.Errorf("graph-invalid: "+format, args...)
	}
	if g.Profile != GraphProfile || g.StepMarker != "CORVINT_PM_STEP" {
		return bad("profile or marker")
	}
	classes := map[string]bool{}
	prefixes := map[string]bool{}
	for _, c := range g.CredentialClasses {
		if !idPattern.MatchString(c.ID) || !prefixPattern.MatchString(c.SecretPrefix) || classes[c.ID] || prefixes[c.SecretPrefix] {
			return bad("credential class %q", c.ID)
		}
		for p := range prefixes {
			if strings.HasPrefix(p, c.SecretPrefix) || strings.HasPrefix(c.SecretPrefix, p) {
				return bad("overlapping secret prefix %q", c.SecretPrefix)
			}
		}
		classes[c.ID], prefixes[c.SecretPrefix] = true, true
	}
	if len(g.Steps) != len(RequiredSteps) {
		return bad("step count")
	}
	seen := map[string]int{}
	for i, s := range g.Steps {
		if s.ID != RequiredSteps[i] || s.Order != i+1 {
			return bad("step %d must be %q", i+1, RequiredSteps[i])
		}
		for _, a := range s.After {
			if _, ok := seen[a]; !ok {
				return bad("step %q depends on later or unknown %q", s.ID, a)
			}
		}
		for _, c := range s.Credentials {
			if !classes[c] {
				return bad("step %q credential %q", s.ID, c)
			}
		}
		for _, list := range [][]string{s.After, s.Credentials, s.Commands, s.PendingCommands} {
			if !sort.StringsAreSorted(list) || hasDuplicate(list) {
				return bad("step %q lists must be sorted and unique", s.ID)
			}
		}
		for _, c := range append(append([]string{}, s.Commands...), s.PendingCommands...) {
			if !commandPattern.MatchString(c) {
				return bad("step %q command %q", s.ID, c)
			}
		}
		switch {
		case s.ID == "authoring" && s.Attestation != "required":
			return bad("authoring requires attestation")
		case s.ID != "authoring" && s.Attestation != "none":
			return bad("step %q attestation", s.ID)
		}
		seen[s.ID] = i
	}
	for _, c := range g.Step("authoring").Credentials {
		if g.Class(c).OutwardWrite {
			return bad("authoring may not hold outward-write class %q", c)
		}
	}
	return nil
}

func hasDuplicate(list []string) bool {
	for i := 1; i < len(list); i++ {
		if list[i] == list[i-1] {
			return true
		}
	}
	return false
}

// Step returns the named step or nil.
func (g *Graph) Step(id string) *Step {
	for i := range g.Steps {
		if g.Steps[i].ID == id {
			return &g.Steps[i]
		}
	}
	return nil
}

// Class returns the named credential class or a zero value.
func (g *Graph) Class(id string) CredentialClass {
	for _, c := range g.CredentialClasses {
		if c.ID == id {
			return c
		}
	}
	return CredentialClass{}
}

// SecretClass classifies a secret name by its declared prefix.
func (g *Graph) SecretClass(name string) (string, bool) {
	for _, c := range g.CredentialClasses {
		if strings.HasPrefix(name, c.SecretPrefix) && len(name) > len(c.SecretPrefix) {
			return c.ID, true
		}
	}
	return "", false
}
