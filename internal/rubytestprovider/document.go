// Package rubytestprovider is the experimental RSpec provider behind
// corvint-ruby-test-provider (docs/specs/live-proof-carrying-verification-v0.md
// LPCV-V0-090..094). It runs RSpec with its JSON formatter, binds the run to the
// host-observed test repository identity before and after, and records each
// example's file:line identity, full description, status and the Git blob IDs
// of its spec file and the conventional helper files. The document carries
// observations only; testvaliditydoc recomputes every projection from them.
package rubytestprovider

import (
	"crypto/sha1"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io/fs"
	"strconv"
	"strings"

	"github.com/Beamfall/corvint/internal/jstestprovider"
	"github.com/Beamfall/corvint/internal/tcq"
	"github.com/Beamfall/corvint/internal/testvalidity"
)

const (
	// Profile names the closed provider document (LPCV-V0-090).
	Profile = "corvint-ruby-test-provider/0"
	// Producer is the retained-evidence producer name (LPCV-V0-055).
	Producer = "corvint-ruby-test-provider"
	// HelperBasis states that the helper set is the conventional files present
	// before the run, not an observation of what RSpec loaded (LPCV-V0-092).
	HelperBasis = "conventional-paths"
	// MaxExamples bounds the retained examples; the rest are counted.
	MaxExamples = 2000
	// MaxSnapshotFiles bounds the spec-directory snapshot.
	MaxSnapshotFiles = 4096
	// MaxDescriptionBytes bounds one retained full description.
	MaxDescriptionBytes = 1024

	statusPassed  = "passed"
	statusFailed  = "failed"
	statusPending = "pending"
)

// Document is the closed corvint-ruby-test-provider/0 document.
type Document struct {
	Profile          string                                        `json:"profile"`
	Runner           string                                        `json:"runner"`
	RunnerVersion    string                                        `json:"runnerVersion,omitempty"`
	ProjectDirectory string                                        `json:"projectDirectory"`
	Command          []string                                      `json:"command,omitempty"`
	CommandWithheld  string                                        `json:"commandWithheld,omitempty"`
	ObjectFormat     string                                        `json:"objectFormat"`
	RepositoryBefore *jstestprovider.ApplicationRepositoryIdentity `json:"repositoryBefore,omitempty"`
	RepositoryAfter  *jstestprovider.ApplicationRepositoryIdentity `json:"repositoryAfter,omitempty"`
	HelperBasis      string                                        `json:"helperBasis"`
	HelperSet        string                                        `json:"helperSet"`
	HelpersUnbound   string                                        `json:"helpersUnbound,omitempty"`
	Helpers          []BoundFile                                   `json:"helpers"`
	Examples         []Example                                     `json:"examples"`
	ExamplesOmitted  int                                           `json:"examplesOmitted,omitempty"`
	Summary          *Summary                                      `json:"summary,omitempty"`
	ExitStatus       *int                                          `json:"exitStatus,omitempty"`
	JUnit            JUnit                                         `json:"junit"`
	Infrastructure   *Infrastructure                               `json:"infrastructure,omitempty"`
}

// BoundFile is one worktree-relative file and its Git blob object ID.
type BoundFile struct {
	Path string `json:"path"`
	Blob string `json:"blob"`
}

// Example is one RSpec example observation (LPCV-V0-092).
type Example struct {
	ID                  string `json:"id"`
	RSpecID             string `json:"rspecId"`
	File                string `json:"file"`
	Line                int    `json:"line"`
	Description         string `json:"description,omitempty"`
	DescriptionWithheld string `json:"descriptionWithheld,omitempty"`
	Status              string `json:"status"`
	SpecBlob            string `json:"specBlob,omitempty"`
	SpecBlobUnbound     string `json:"specBlobUnbound,omitempty"`
	Helpers             string `json:"helpers"`
}

// Summary is the RSpec report summary.
type Summary struct {
	Examples               int `json:"examples"`
	Failures               int `json:"failures"`
	Pending                int `json:"pending"`
	ErrorsOutsideOfExample int `json:"errorsOutsideOfExamples"`
}

// JUnit states whether a JUnit report was requested and produced. Only its
// digest is retained; it is not parsed.
type JUnit struct {
	State  string `json:"state"`
	Digest string `json:"digest,omitempty"`
	Reason string `json:"reason,omitempty"`
}

// Infrastructure is a failure that prevented a complete observation.
type Infrastructure struct {
	Reason string `json:"reason"`
	Detail string `json:"detail"`
}

// Validate refuses a decoded document that breaks the closed shape.
func Validate(document Document) error {
	if document.Profile != Profile || document.Runner != "rspec" || document.HelperBasis != HelperBasis {
		return errors.New("Ruby provider document has an unknown profile, runner or helper basis")
	}
	if document.ObjectFormat != "sha1" && document.ObjectFormat != "sha256" {
		return errors.New("Ruby provider document has an unknown object format")
	}
	if document.ExamplesOmitted < 0 || len(document.Examples) > MaxExamples || document.Examples == nil || document.Helpers == nil {
		return errors.New("Ruby provider document is out of bounds")
	}
	if !localSlashPath(document.ProjectDirectory) {
		return errors.New("Ruby provider project directory is not worktree-relative")
	}
	for _, helper := range document.Helpers {
		if !localSlashPath(helper.Path) || !blobPattern(document.ObjectFormat, helper.Blob) {
			return errors.New("Ruby provider helper is malformed")
		}
	}
	if HelperSetDigest(document.Helpers) != document.HelperSet {
		return errors.New("Ruby provider helper set digest does not match its helpers")
	}
	for _, example := range document.Examples {
		if example.Helpers != document.HelperSet || example.ID != example.File+":"+strconv.Itoa(example.Line) || example.Line < 1 {
			return errors.New("Ruby provider example identity is malformed")
		}
		if example.Status != statusPassed && example.Status != statusFailed && example.Status != statusPending && example.Status != "unknown" {
			return errors.New("Ruby provider example status is outside the closed set")
		}
		if (example.SpecBlob == "") == (example.SpecBlobUnbound == "") {
			return errors.New("Ruby provider example must carry exactly one of specBlob and specBlobUnbound")
		}
		if example.SpecBlob != "" && (!localSlashPath(example.File) || !blobPattern(document.ObjectFormat, example.SpecBlob)) {
			return errors.New("Ruby provider example spec blob is malformed")
		}
	}
	return nil
}

func localSlashPath(name string) bool {
	return name != "" && fs.ValidPath(name) && !strings.HasPrefix(name, ".git/") && name != ".git"
}

func blobPattern(format, value string) bool {
	want := 40
	if format == "sha256" {
		want = 64
	}
	if len(value) != want {
		return false
	}
	_, err := hex.DecodeString(value)
	return err == nil && strings.ToLower(value) == value
}

// BlobID is the Git blob object ID of data in the named object format, the
// value git hash-object --no-filters prints.
func BlobID(format string, data []byte) string {
	header := "blob " + strconv.Itoa(len(data)) + "\x00"
	if format == "sha256" {
		sum := sha256.New()
		sum.Write([]byte(header))
		sum.Write(data)
		return hex.EncodeToString(sum.Sum(nil))
	}
	sum := sha1.New()
	sum.Write([]byte(header))
	sum.Write(data)
	return hex.EncodeToString(sum.Sum(nil))
}

// HelperSetDigest binds the sorted helper set every example references.
func HelperSetDigest(helpers []BoundFile) string {
	var builder strings.Builder
	for _, helper := range helpers {
		builder.WriteString(helper.Path)
		builder.WriteByte(0)
		builder.WriteString(helper.Blob)
		builder.WriteByte('\n')
	}
	sum := sha256.Sum256([]byte(builder.String()))
	return "sha256:" + hex.EncodeToString(sum[:])
}

// ProjectExample recomputes one example's projection (LPCV-V0-093): the
// example is associated with its own file:line, execution follows its status,
// hygiene is not produced for Ruby, and strength is never measured here.
func ProjectExample(document Document, example Example) testvalidity.Projection {
	anchors := []string{example.ID}
	claim := testvalidity.ClaimFacts{AssociationState: tcq.AssociationAssociated, HygieneState: tcq.HygieneEligible, ReportState: reportState(example.Status), Anchors: anchors}
	input := testvalidity.Input{Claim: &claim}
	if document.Infrastructure != nil {
		input.Execution = &testvalidity.ExecutionFacts{Outcome: "INCOMPLETE", Cause: "INFRASTRUCTURE", Anchors: anchors}
	}
	projection := testvalidity.Project(input)
	projection.Hygiene = testvalidity.Axis{State: testvalidity.HygieneUnsupported, Reason: "ruby-hygiene-not-produced"}
	return projection
}

// ProjectRun recomputes the run projection from the retained observations.
func ProjectRun(document Document) testvalidity.Projection {
	execution := testvalidity.ExecutionFacts{}
	switch {
	case document.Infrastructure != nil:
		execution.Outcome, execution.Cause = "INCOMPLETE", "INFRASTRUCTURE"
	case len(document.Examples) == 0 && document.ExamplesOmitted == 0:
	case anyFailed(document.Examples):
		execution.Outcome, execution.Cause = "FAILED", "ASSERTION_OR_TEST"
	default:
		execution.Outcome = "PASSED"
	}
	return testvalidity.Project(testvalidity.Input{Execution: &execution})
}

func anyFailed(examples []Example) bool {
	for _, example := range examples {
		if example.Status != statusPassed && example.Status != statusPending {
			return true
		}
	}
	return false
}

func reportState(status string) string {
	switch status {
	case statusPassed:
		return tcq.ReportPassed
	case statusFailed:
		return tcq.ReportFailed
	case statusPending:
		return tcq.ReportSkipped
	}
	return ""
}

// Freshness binds a retained document to the worktree as it is now
// (LPCV-V0-094). read returns a worktree-relative file's bytes, fs.ErrNotExist
// for a removed file, or another error when the file cannot be read.
func Freshness(document Document, read func(name string) ([]byte, error)) testvalidity.Axis {
	bound := map[string]string{}
	for _, helper := range document.Helpers {
		bound[helper.Path] = helper.Blob
	}
	unknown := ""
	for _, example := range document.Examples {
		if example.SpecBlob == "" {
			if unknown == "" {
				unknown = "retained-spec-blob-unbound"
			}
			continue
		}
		bound[example.File] = example.SpecBlob
	}
	paths := make([]string, 0, len(bound))
	for name := range bound {
		paths = append(paths, name)
	}
	sortStrings(paths)
	for _, name := range paths {
		data, err := read(name)
		if errors.Is(err, fs.ErrNotExist) || err == nil && BlobID(document.ObjectFormat, data) != bound[name] {
			return testvalidity.Axis{State: testvalidity.FreshnessStale, Reason: "retained-digest-mismatch"}
		}
		if err != nil && unknown == "" {
			unknown = "retained-bound-source-unreadable"
		}
	}
	switch {
	case document.RepositoryBefore == nil || document.RepositoryAfter == nil:
		return testvalidity.Axis{State: testvalidity.FreshnessUnknown, Reason: "retained-repository-identity-unavailable"}
	case *document.RepositoryBefore != *document.RepositoryAfter:
		return testvalidity.Axis{State: testvalidity.FreshnessUnknown, Reason: "repository-changed-during-run"}
	case document.HelpersUnbound != "":
		return testvalidity.Axis{State: testvalidity.FreshnessUnknown, Reason: "retained-helper-set-unbound"}
	case unknown != "":
		return testvalidity.Axis{State: testvalidity.FreshnessUnknown, Reason: unknown}
	case len(paths) == 0:
		return testvalidity.Axis{State: testvalidity.FreshnessUnknown, Reason: "retained-identity-unbound"}
	}
	return testvalidity.Axis{State: testvalidity.FreshnessCurrent}
}

// Revision is the start revision when it is a well-formed object ID, else "".
func Revision(document Document) string {
	if document.RepositoryBefore == nil || !blobPattern(document.ObjectFormat, document.RepositoryBefore.Revision) {
		return ""
	}
	return document.RepositoryBefore.Revision
}

func sortStrings(values []string) {
	for i := 1; i < len(values); i++ {
		for j := i; j > 0 && values[j] < values[j-1]; j-- {
			values[j], values[j-1] = values[j-1], values[j]
		}
	}
}
