// Package repro defines the corvint-repro/0 reproduction bundle: one recorded corvint
// invocation with its inputs pinned by revision and digest, what it observed, and the
// expectations a later build is judged against (docs/specs/reproduction-bundle-v0.md).
package repro

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"path"
	"regexp"
	"strings"

	"github.com/Beamfall/corvint/internal/jsonend"
)

// Profile names the bundle schema (RPB-V0-001).
const Profile = "corvint-repro/0"

// Bounds keep a bundle small enough to attach to an issue (RPB-V0-002).
const (
	MaxBundleBytes     = 4 << 20
	MaxArguments       = 64
	MaxArgumentBytes   = 4096
	MaxInputFiles      = 32
	MaxExpectations    = 32
	MaxExpectationText = 4096
	MaxStdoutBytes     = 64 << 10
	MaxStderrBytes     = 16 << 10
)

// Expectation kinds (RPB-V0-003).
const (
	ExitCode          = "exit-code"
	StdoutContains    = "stdout-contains"
	StdoutNotContains = "stdout-not-contains"
	StdoutSame        = "stdout-same"
	StderrContains    = "stderr-contains"
)

var (
	revisionPattern = regexp.MustCompile(`^(?:[0-9a-f]{40}|[0-9a-f]{64})$`)
	digestPattern   = regexp.MustCompile(`^[0-9a-f]{64}$`)
)

// Bundle is one recorded invocation. Every member is required; the encoding is closed.
type Bundle struct {
	Profile      string        `json:"profile"`
	RecordedBy   Build         `json:"recordedBy"`
	Command      Command       `json:"command"`
	Inputs       Inputs        `json:"inputs"`
	Observed     Observation   `json:"observed"`
	Expectations []Expectation `json:"expectations"`
}

// Build is the --version --json identity of a binary (PUB-V0-050..052).
type Build struct {
	Version        string `json:"version"`
	DisplayVersion string `json:"displayVersion"`
	Build          string `json:"build"`
	Commit         string `json:"commit"`
	CommitState    string `json:"commitState"`
	Release        bool   `json:"release"`
}

// Command is the corvint argument vector after the global --root preamble.
type Command struct {
	Arguments []string `json:"arguments"`
}

// Inputs pin the repository by commit and tree, and each declared file by SHA-256.
type Inputs struct {
	Repository Repository  `json:"repository"`
	Files      []InputFile `json:"files"`
}

// Repository is the clean checkout the command ran in.
type Repository struct {
	Commit string `json:"commit"`
	Tree   string `json:"tree"`
}

// InputFile is a repository-relative file the command reads beyond the committed tree.
type InputFile struct {
	Path   string `json:"path"`
	SHA256 string `json:"sha256"`
}

// Observation is what the recording build produced. Stdout is kept only when it fits.
type Observation struct {
	ExitCode        int    `json:"exitCode"`
	StdoutSHA256    string `json:"stdoutSha256"`
	StdoutBytes     int    `json:"stdoutBytes"`
	Stdout          string `json:"stdout"`
	StdoutTruncated bool   `json:"stdoutTruncated"`
	Stderr          string `json:"stderr"`
	StderrTruncated bool   `json:"stderrTruncated"`
}

// Expectation is one check a replay passes or fails. Text is used by the *-contains kinds,
// Exit by exit-code; stdout-same compares against the recorded stdout digest.
type Expectation struct {
	ID   string `json:"id"`
	Kind string `json:"kind"`
	Text string `json:"text,omitempty"`
	Exit *int   `json:"exit,omitempty"`
}

// Result is one expectation's outcome on a replay (RPB-V0-007).
type Result struct {
	ID       string `json:"id"`
	Kind     string `json:"kind"`
	Status   string `json:"status"`
	Expected string `json:"expected"`
	Observed string `json:"observed"`
}

// Digest is the lower-case hex SHA-256 of data.
func Digest(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

// Observe builds the Observation for one run, bounding the kept text.
func Observe(exitCode int, stdout, stderr []byte) Observation {
	observation := Observation{ExitCode: exitCode, StdoutSHA256: Digest(stdout), StdoutBytes: len(stdout)}
	observation.Stdout, observation.StdoutTruncated = bounded(stdout, MaxStdoutBytes)
	observation.Stderr, observation.StderrTruncated = bounded(stderr, MaxStderrBytes)
	return observation
}

func bounded(data []byte, limit int) (string, bool) {
	if len(data) <= limit {
		return string(data), false
	}
	return strings.ToValidUTF8(string(data[:limit]), ""), true
}

// Decode parses and validates a bundle; any unknown member, missing member or bound
// violation is an error naming the member (RPB-V0-002).
func Decode(data []byte) (Bundle, error) {
	var bundle Bundle
	if len(data) > MaxBundleBytes {
		return bundle, fmt.Errorf("bundle exceeds %d bytes", MaxBundleBytes)
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&bundle); err != nil {
		return bundle, fmt.Errorf("bundle is not a %s object: %v", Profile, err)
	}
	if !jsonend.Done(decoder) {
		return bundle, fmt.Errorf("bundle has trailing data")
	}
	return bundle, bundle.Validate()
}

// Validate checks every member against the closed schema.
func (bundle Bundle) Validate() error {
	if bundle.Profile != Profile {
		return fmt.Errorf("profile %q, want %q", bundle.Profile, Profile)
	}
	if bundle.RecordedBy.Version == "" || bundle.RecordedBy.DisplayVersion == "" || bundle.RecordedBy.Commit == "" || bundle.RecordedBy.CommitState == "" {
		return fmt.Errorf("recordedBy is incomplete")
	}
	if err := ValidateArguments(bundle.Command.Arguments); err != nil {
		return err
	}
	if !revisionPattern.MatchString(bundle.Inputs.Repository.Commit) || !revisionPattern.MatchString(bundle.Inputs.Repository.Tree) {
		return fmt.Errorf("inputs.repository needs a full commit and tree")
	}
	if bundle.Inputs.Files == nil || len(bundle.Inputs.Files) > MaxInputFiles {
		return fmt.Errorf("inputs.files must be a list of at most %d files", MaxInputFiles)
	}
	seen := map[string]bool{}
	for _, file := range bundle.Inputs.Files {
		if err := ValidatePath(file.Path); err != nil {
			return err
		}
		if seen[file.Path] || !digestPattern.MatchString(file.SHA256) {
			return fmt.Errorf("inputs.files: %s is repeated or has no SHA-256", file.Path)
		}
		seen[file.Path] = true
	}
	observed := bundle.Observed
	if !digestPattern.MatchString(observed.StdoutSHA256) || observed.StdoutBytes < 0 || len(observed.Stdout) > MaxStdoutBytes || len(observed.Stderr) > MaxStderrBytes {
		return fmt.Errorf("observed is malformed")
	}
	if !observed.StdoutTruncated && (len(observed.Stdout) != observed.StdoutBytes || Digest([]byte(observed.Stdout)) != observed.StdoutSHA256) {
		return fmt.Errorf("observed.stdout disagrees with its digest")
	}
	return ValidateExpectations(bundle.Expectations)
}

// ValidateArguments refuses an empty, oversized or self-referential command (RPB-V0-004).
func ValidateArguments(arguments []string) error {
	if len(arguments) == 0 || len(arguments) > MaxArguments {
		return fmt.Errorf("command.arguments must hold 1 to %d arguments", MaxArguments)
	}
	for _, argument := range arguments {
		if len(argument) > MaxArgumentBytes || strings.ContainsRune(argument, 0) {
			return fmt.Errorf("command.arguments has an oversized or NUL argument")
		}
	}
	if arguments[0] == "--root" || strings.HasPrefix(arguments[0], "--root=") {
		return fmt.Errorf("command.arguments must not carry --root; the replay supplies it")
	}
	if arguments[0] == "repro" {
		return fmt.Errorf("command.arguments must not record repro itself")
	}
	return nil
}

// ValidatePath admits a clean, relative, slash-separated repository path.
func ValidatePath(name string) error {
	if name == "" || strings.HasPrefix(name, "/") || strings.Contains(name, "\\") || path.Clean(name) != name || name == "." || name == ".." || strings.HasPrefix(name, "../") {
		return fmt.Errorf("input path %q is not a clean repository-relative path", name)
	}
	return nil
}

// ValidateExpectations requires at least one well-formed expectation with unique IDs.
func ValidateExpectations(expectations []Expectation) error {
	if len(expectations) == 0 || len(expectations) > MaxExpectations {
		return fmt.Errorf("expectations must hold 1 to %d entries", MaxExpectations)
	}
	ids := map[string]bool{}
	for _, expectation := range expectations {
		if expectation.ID == "" || ids[expectation.ID] {
			return fmt.Errorf("expectation id %q is empty or repeated", expectation.ID)
		}
		ids[expectation.ID] = true
		switch expectation.Kind {
		case ExitCode:
			if expectation.Exit == nil || *expectation.Exit < 0 || *expectation.Exit > 255 || expectation.Text != "" {
				return fmt.Errorf("expectation %s: exit-code needs exit 0..255 and no text", expectation.ID)
			}
		case StdoutContains, StdoutNotContains, StderrContains:
			if expectation.Text == "" || len(expectation.Text) > MaxExpectationText || expectation.Exit != nil {
				return fmt.Errorf("expectation %s: %s needs text of 1 to %d bytes", expectation.ID, expectation.Kind, MaxExpectationText)
			}
		case StdoutSame:
			if expectation.Text != "" || expectation.Exit != nil {
				return fmt.Errorf("expectation %s: stdout-same takes no text or exit", expectation.ID)
			}
		default:
			return fmt.Errorf("expectation %s: unknown kind %q", expectation.ID, expectation.Kind)
		}
	}
	return nil
}

// Evaluate judges each expectation against one replay's full output (RPB-V0-007).
func Evaluate(bundle Bundle, exitCode int, stdout, stderr []byte) []Result {
	results := make([]Result, 0, len(bundle.Expectations))
	for _, expectation := range bundle.Expectations {
		result := Result{ID: expectation.ID, Kind: expectation.Kind}
		pass := false
		switch expectation.Kind {
		case ExitCode:
			result.Expected, result.Observed = fmt.Sprint(*expectation.Exit), fmt.Sprint(exitCode)
			pass = exitCode == *expectation.Exit
		case StdoutContains:
			result.Expected, result.Observed = expectation.Text, presence(stdout, expectation.Text)
			pass = bytes.Contains(stdout, []byte(expectation.Text))
		case StdoutNotContains:
			result.Expected, result.Observed = "absent: "+expectation.Text, presence(stdout, expectation.Text)
			pass = !bytes.Contains(stdout, []byte(expectation.Text))
		case StderrContains:
			result.Expected, result.Observed = expectation.Text, presence(stderr, expectation.Text)
			pass = bytes.Contains(stderr, []byte(expectation.Text))
		case StdoutSame:
			result.Expected, result.Observed = "sha256:"+bundle.Observed.StdoutSHA256, "sha256:"+Digest(stdout)
			pass = Digest(stdout) == bundle.Observed.StdoutSHA256
		}
		result.Status = "FAIL"
		if pass {
			result.Status = "PASS"
		}
		results = append(results, result)
	}
	return results
}

func presence(haystack []byte, needle string) string {
	if bytes.Contains(haystack, []byte(needle)) {
		return "present"
	}
	return "absent"
}
