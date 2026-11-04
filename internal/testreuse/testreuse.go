// SPDX-License-Identifier: Apache-2.0

// Package testreuse decides when a passed Go test result may stand for a
// later run whose inputs are unchanged (AFP-V0-2320 to AFP-V0-2333,
// docs/specs/affected-plan-reuse-v0.md).
//
// A package is eligible only when its tests read, below the repository root,
// nothing but what its input-closure digest names: its declared read scope
// (AFP-V0-023), which CI's test-confine wrapper enforces, its own directory and
// the files compiled into its test binary. The digest binds each of those
// paths to its Git blob, plus the toolchain, the argv and the environment the
// result was obtained under. Anything the package cannot prove, an
// undeclared scope, an UNBOUNDED_READER unit, a split package, a dirty or
// untracked input or any read error, runs the test: missing evidence never
// becomes a reused pass.
package testreuse

import (
	"crypto/sha256"
	"encoding/hex"
	json "encoding/json/v2"
	"errors"
	"fmt"
	"slices"
	"sort"
	"strconv"
	"strings"
	"time"
)

// Profiles of the documents this package reads and writes.
const (
	DigestProfile   = "corvint-test-input-digest/0"
	IdentityProfile = "corvint-test-reuse-identity/0"
	RecordProfile   = "corvint-test-reuse-record/0"
	DecisionProfile = "corvint-test-reuse-decision/0"
)

// Decisions (AFP-V0-2325).
const (
	DecisionReuse = "REUSE"
	DecisionRun   = "RUN"
)

// Reasons. Every unit of a decision names exactly one (AFP-V0-2325); a
// skipped package of a record names one of the eligibility reasons
// (AFP-V0-2323).
const (
	// ReasonRecordDigestEqual: an admitted record passed this package under
	// the same identity with the same input-closure digest.
	ReasonRecordDigestEqual = "RECORD_DIGEST_EQUAL"
	// ReasonUndeclaredReadScope: no declared read scope bounds the package's
	// reads, so CI runs it unconfined and its inputs are unbounded.
	ReasonUndeclaredReadScope = "UNDECLARED_READ_SCOPE"
	// ReasonUnboundedReader: the affected graph selects the unit on any path.
	ReasonUnboundedReader = "UNBOUNDED_READER"
	// ReasonSlicedUnit: CI may split the package into test slices
	// (AFP-V0-041), so no single invocation proves the whole package.
	ReasonSlicedUnit = "SLICED_UNIT"
	// ReasonDigestError: an input of the digest could not be bound to
	// immutable Git content; Detail names the cause.
	ReasonDigestError = "DIGEST_ERROR"
	// ReasonNoRecord: no admitted record names the package as passed.
	ReasonNoRecord = "NO_RECORD"
	// ReasonIdentityChanged: a record passed the package under another
	// toolchain, argv, environment or harness.
	ReasonIdentityChanged = "IDENTITY_CHANGED"
	// ReasonDigestChanged: a record passed the package with another digest.
	ReasonDigestChanged = "DIGEST_CHANGED"
	// ReasonNotPassed: a record's run did not pass the package (record only).
	ReasonNotPassed = "NOT_PASSED"
)

// Record admission reasons (AFP-V0-2324).
const (
	RecordAdmitted       = "ADMITTED"
	RecordUntrusted      = "UNTRUSTED_SOURCE"
	RecordInvalid        = "RECORD_INVALID"
	RecordIdentityForged = "IDENTITY_DIGEST_MISMATCH"
)

// Source kinds a record may name (AFP-V0-2323).
const (
	SourceMain        = "main"
	SourcePullRequest = "pull_request"
	SourceLocal       = "local"
)

// Digest error details (AFP-V0-2321).
const (
	ErrorDirtyInput        = "dirty-input"
	ErrorUntrackedInput    = "untracked-input"
	ErrorIrregularInput    = "irregular-input"
	ErrorExternalPackage   = "external-package"
	ErrorCgoInput          = "cgo-input"
	ErrorGoListPackage     = "go-list-error"
	ErrorClosureUnlisted   = "closure-unlisted"
	ErrorPackageUnlisted   = "package-unlisted"
	ErrorScopeForm         = "scope-form"
	ErrorDeclarationDirty  = "declaration-dirty"
	ErrorDeclarationBroken = "declaration-invalid"
)

// IdentityEnv names the environment values, as `go env` resolves them, that
// join the identity. GOFLAGS carries build tags and -race when set there; the
// argv carries the rest of the flags.
var IdentityEnv = []string{
	"CC", "CGO_CFLAGS", "CGO_CPPFLAGS", "CGO_CXXFLAGS", "CGO_ENABLED", "CGO_LDFLAGS", "CXX",
	"GO386", "GOAMD64", "GOARCH", "GOARM", "GOARM64", "GODEBUG", "GOEXPERIMENT", "GOFLAGS", "GOMIPS", "GOMIPS64", "GOOS",
	"GOPPC64", "GORISCV64", "GOTOOLCHAIN", "GOVERSION", "GOWASM", "GOWORK",
}

// Input is one path a digest binds: Mode and Object are the Git tree
// entry's, or "absent" for a declared path the tree does not hold and "dir"
// for a declared directory's own marker.
type Input struct {
	Path   string `json:"path"`
	Mode   string `json:"mode"`
	Object string `json:"object"`
}

// Identity is what the result was obtained under besides repository content
// (AFP-V0-2322).
type Identity struct {
	Profile string            `json:"profile"`
	Env     map[string]string `json:"env"`
	Argv    []string          `json:"argv"`
	Harness []Input           `json:"harness"`
}

// Digest returns the identity's canonical digest.
func (identity Identity) Digest() (string, error) {
	return canonicalDigest(identity)
}

// Argv flags (AFP-V0-2322). A build flag changes which files the test binary
// is built from, so it is also passed to go list; a run flag changes only how
// the binary runs. Any other flag is refused: its effect on the result or the
// closure is not known.
var (
	argvBuildFlags      = map[string]bool{"race": true, "msan": true, "asan": true, "cover": true, "trimpath": true}
	argvBuildValueFlags = map[string]bool{"tags": true, "covermode": true, "coverpkg": true}
	argvRunValueFlags   = map[string]bool{"timeout": true, "p": true, "parallel": true, "cpu": true}
	argvRunFlags        = map[string]bool{"v": true, "json": true}
)

// CheckArgv refuses an argv that runs part of a package, reports Go's own
// cached result or carries a flag whose effect is not known: a pass under it
// is never a whole-package pass recorded at this run (AFP-V0-2322). The argv
// must carry -count=1.
func CheckArgv(argv []string) error {
	_, err := splitArgv(argv)
	return err
}

// BuildFlags returns the argv's build flags, each value joined to its flag,
// in argv order, for go list.
func BuildFlags(argv []string) []string {
	flags, _ := splitArgv(argv)
	return flags
}

func splitArgv(argv []string) ([]string, error) {
	var build []string
	countOne := false
	for index := 0; index < len(argv); index++ {
		word := argv[index]
		if !strings.HasPrefix(word, "-") {
			return nil, fmt.Errorf("argv word %q is not a flag; name packages elsewhere", word)
		}
		name, value, hasValue := strings.Cut(strings.TrimLeft(word, "-"), "=")
		name = strings.TrimPrefix(name, "test.")
		takeValue := func() (string, error) {
			if hasValue {
				return value, nil
			}
			if index+1 >= len(argv) {
				return "", fmt.Errorf("argv flag %q needs a value", word)
			}
			index++
			return argv[index], nil
		}
		switch {
		case name == "count":
			if !hasValue || value != "1" {
				return nil, fmt.Errorf("argv word %q: write the count as -count=1", word)
			}
			countOne = true
		case argvBuildFlags[name] && !hasValue:
			build = append(build, "-"+name)
		case argvBuildValueFlags[name]:
			v, err := takeValue()
			if err != nil {
				return nil, err
			}
			if v == "" || strings.HasPrefix(v, "-") || (name == "covermode" && v != "set" && v != "count" && v != "atomic") {
				return nil, fmt.Errorf("argv flag %q has invalid value %q", word, v)
			}
			build = append(build, "-"+name+"="+v)
		case argvRunValueFlags[name]:
			v, err := takeValue()
			if err != nil {
				return nil, err
			}
			if name == "timeout" {
				duration, err := time.ParseDuration(v)
				if err != nil || duration < 0 {
					return nil, fmt.Errorf("argv flag %q has invalid duration %q", word, v)
				}
			} else {
				values := []string{v}
				if name == "cpu" {
					values = strings.Split(v, ",")
				}
				for _, value := range values {
					n, err := strconv.Atoi(value)
					if err != nil || n < 1 {
						return nil, fmt.Errorf("argv flag %q has invalid positive integer %q", word, value)
					}
				}
			}
		case argvRunFlags[name] && !hasValue:
		default:
			return nil, fmt.Errorf("argv word %q runs part of a package or has an effect reuse does not model; only whole-package runs with known flags are recorded or reused", word)
		}
	}
	if !countOne {
		return nil, errors.New("argv must carry -count=1, so a pass is this run's own execution, not Go's cached result")
	}
	return build, nil
}

type digestDocument struct {
	Profile  string   `json:"profile"`
	Package  string   `json:"package"`
	Identity string   `json:"identity"`
	Scope    []string `json:"scope"`
	Inputs   []Input  `json:"inputs"`
}

// PackageDigest is the outcome of digesting one package: Digest when it is
// eligible, else Reason and Detail.
type PackageDigest struct {
	Package string
	Digest  string
	Reason  string
	Detail  string
	Inputs  int
}

// DigestPackage computes the input-closure digest of package importPath over
// snapshot (AFP-V0-2321) after the eligibility checks of AFP-V0-2320.
func DigestPackage(snapshot *Snapshot, importPath, identityDigest string) PackageDigest {
	result := PackageDigest{Package: importPath}
	fail := func(reason, detail string) PackageDigest {
		result.Reason, result.Detail = reason, detail
		return result
	}
	if snapshot.DeclarationError != "" {
		return fail(ReasonDigestError, snapshot.DeclarationError)
	}
	listed, ok := snapshot.Packages[importPath]
	if !ok {
		return fail(ReasonDigestError, ErrorPackageUnlisted+": go list did not list "+importPath)
	}
	directory, ok := snapshot.relative(listed.Dir)
	if !ok {
		return fail(ReasonDigestError, ErrorExternalPackage+": "+importPath+" is outside the repository")
	}
	scope, declared := snapshot.Scopes[directory]
	if !declared {
		return fail(ReasonUndeclaredReadScope, directory)
	}
	if snapshot.Unbounded[importPath] {
		return fail(ReasonUnboundedReader, importPath)
	}
	if snapshot.Sliced[importPath] {
		return fail(ReasonSlicedUnit, importPath)
	}
	inputs := map[string]Input{}
	var detail string
	addFile := func(relative string) bool {
		entry, tracked := snapshot.Entries[relative]
		switch {
		case !tracked:
			detail = ErrorUntrackedInput + ": " + relative
			return false
		case entry.Mode != "100644" && entry.Mode != "100755":
			detail = ErrorIrregularInput + ": " + relative + " has mode " + entry.Mode
			return false
		}
		inputs[relative] = Input{Path: relative, Mode: entry.Mode, Object: entry.Object}
		return true
	}
	addTree := func(prefix string) bool {
		for _, relative := range snapshot.under(prefix) {
			if !addFile(relative) {
				return false
			}
		}
		return true
	}
	// The package's own directory subtree: the confinement grants it whole.
	if !addTree(directory) {
		return fail(ReasonDigestError, detail)
	}
	for _, entry := range scope {
		trimmed := strings.TrimSuffix(entry, "/")
		if strings.HasSuffix(entry, "/") {
			if _, file := snapshot.Entries[trimmed]; file {
				return fail(ReasonDigestError, ErrorScopeForm+": "+entry+" names a file")
			}
			files := snapshot.under(trimmed)
			if len(files) == 0 {
				inputs[entry] = Input{Path: entry, Mode: "absent", Object: "absent"}
				continue
			}
			inputs[entry] = Input{Path: entry, Mode: "dir", Object: "dir"}
			if !addTree(trimmed) {
				return fail(ReasonDigestError, detail)
			}
			continue
		}
		if len(snapshot.under(trimmed)) != 0 {
			return fail(ReasonDigestError, ErrorScopeForm+": "+entry+" names a directory")
		}
		if _, tracked := snapshot.Entries[trimmed]; !tracked {
			inputs[entry] = Input{Path: entry, Mode: "absent", Object: "absent"}
			continue
		}
		if !addFile(trimmed) {
			return fail(ReasonDigestError, detail)
		}
	}
	for _, name := range moduleFiles {
		if _, tracked := snapshot.Entries[name]; tracked {
			if !addFile(name) {
				return fail(ReasonDigestError, detail)
			}
		} else {
			inputs[name] = Input{Path: name, Mode: "absent", Object: "absent"}
		}
	}
	closure, err := snapshot.closure(importPath)
	if err != nil {
		return fail(ReasonDigestError, err.Error())
	}
	for _, member := range closure {
		if member.Standard {
			continue
		}
		memberDirectory, ok := snapshot.relative(member.Dir)
		if !ok {
			return fail(ReasonDigestError, ErrorExternalPackage+": "+member.ImportPath+" is outside the repository")
		}
		if len(member.CgoFiles) != 0 {
			return fail(ReasonDigestError, ErrorCgoInput+": "+member.ImportPath+" uses cgo, whose includes go list does not name")
		}
		if memberDirectory == directory {
			continue
		}
		for _, name := range member.compiled() {
			if !addFile(joinRelative(memberDirectory, name)) {
				return fail(ReasonDigestError, detail)
			}
		}
	}
	// A dirty, untracked or ignored worktree path at or below any input
	// is content the tree does not pin.
	var directories []string
	directories = append(directories, directory)
	for _, entry := range scope {
		if strings.HasSuffix(entry, "/") {
			directories = append(directories, strings.TrimSuffix(entry, "/"))
		}
	}
	if path, dirty := snapshot.dirtyWithin(inputs, directories); dirty {
		return fail(ReasonDigestError, ErrorDirtyInput+": "+path)
	}
	document := digestDocument{Profile: DigestProfile, Package: importPath, Identity: identityDigest, Scope: append([]string{}, scope...)}
	if document.Scope == nil {
		document.Scope = []string{}
	}
	for _, input := range inputs {
		document.Inputs = append(document.Inputs, input)
	}
	sort.Slice(document.Inputs, func(i, j int) bool { return document.Inputs[i].Path < document.Inputs[j].Path })
	digest, err := canonicalDigest(document)
	if err != nil {
		return fail(ReasonDigestError, err.Error())
	}
	result.Digest, result.Inputs = digest, len(document.Inputs)
	return result
}

// moduleFiles join every digest, present or absent.
var moduleFiles = []string{"go.mod", "go.sum", "go.work", "go.work.sum"}

func joinRelative(directory, name string) string {
	if directory == "." || directory == "" {
		return name
	}
	return directory + "/" + name
}

func canonicalDigest(value any) (string, error) {
	body, err := json.Marshal(value, json.Deterministic(true))
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(body)
	return "sha256:" + hex.EncodeToString(sum[:]), nil
}

func sortedKeys[V any](set map[string]V) []string {
	keys := make([]string, 0, len(set))
	for key := range set {
		keys = append(keys, key)
	}
	slices.Sort(keys)
	return keys
}
