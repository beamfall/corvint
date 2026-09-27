package releasecandidate

import (
	"bytes"
	"cmp"
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"regexp"
	"slices"
	"strings"
	"syscall"
	"unicode"
	"unicode/utf8"
)

// The stable-readiness record (SRR-V1, accepted by decision 0422) binds one
// verified Core candidate to operator-supplied gate, platform, compliance,
// external and policy evidence digests before any tag or publication. It
// never runs a gate: missing evidence is recorded as NOT_RUN, never PASS.
const (
	readinessProfile     = "corvint-stable-readiness-record/1"
	readinessDecision    = "0420"
	pinnedToolchain      = "go1.27.1"
	firstStableCandidate = "1.0.0-rc.1"
)

// openOutputDirectory and temporaryText are indirect so a test can change a
// path between resolving and opening it, and choose the temporary name.
var (
	decisionPattern     = regexp.MustCompile(`^[0-9]{4}$`)
	storeReleasePattern = regexp.MustCompile(`^[a-z0-9]+(-[a-z0-9]+)*$`)
	goToolchainProbe    = probeLocalToolchain
	openOutputDirectory = os.OpenRoot
	temporaryText       = rand.Text
)

type ReadinessRecord struct {
	Profile       string                 `json:"profile"`
	Identity      ReadinessIdentity      `json:"identity"`
	Core          ReadinessCore          `json:"core"`
	StoreRelease  *ReadinessStoreRelease `json:"storeRelease"`
	Vulnerability ReadinessVulnerability `json:"vulnerability"`
	Rows          []ReadinessRow         `json:"rows"`
}

type ReadinessIdentity struct {
	Version     string `json:"version"`
	Tag         string `json:"tag"`
	BuildNumber string `json:"buildNumber"`
	GoVersion   string `json:"goVersion"`
	Commit      string `json:"commit"`
	Tree        string `json:"tree"`
}

type ReadinessCore struct {
	CandidateProfile string  `json:"candidateProfile"`
	ChecksumsSHA256  string  `json:"checksumsSha256"`
	Archives         []Asset `json:"archives"`
	Reproducibility  string  `json:"reproducibility"`
}

type ReadinessStoreRelease struct {
	ReleaseID       string `json:"releaseId"`
	CandidateSHA256 string `json:"candidateSha256"`
}

type ReadinessVulnerability struct {
	Decision          string `json:"decision"`
	RequireDirectives int    `json:"requireDirectives"`
	Toolchain         string `json:"toolchain"`
	Status            string `json:"status"`
}

type ReadinessRow struct {
	ID       string `json:"id"`
	Status   string `json:"status"`
	SHA256   string `json:"sha256"`
	Decision string `json:"decision"`
	Reason   string `json:"reason"`
}

// ReadinessEvidence is one operator-supplied row: a status, the evidence file
// whose digest is recorded, and the accepting decision or reason.
type ReadinessEvidence struct {
	Status   string
	Path     string
	Decision string
	Reason   string
}

type ReadinessOptions struct {
	CandidateDirectory   string
	SourceRoot           string
	StoreReleaseID       string
	StoreCandidateSHA256 string
	Evidence             map[string]ReadinessEvidence
}

// readinessRule is one catalogue row. A fixed row is written by the builder
// and must match exactly; any other row takes operator evidence or records
// missing. Only platform rows admit FALLBACK.
type readinessRule struct {
	id       string
	fixed    bool
	fallback bool
	missing  ReadinessRow
}

var readinessCatalogue = []readinessRule{
	operatorRule("gate/full-gate"),
	operatorRule("gate/interop-gate"),
	operatorRule("gate/focused-docs"),
	operatorRule("gate/companion-release"),
	operatorRule("gate/release-checklist-pre-promotion"),
	platformRule("platform/darwin-arm64/lifecycle", ""),
	platformRule("platform/darwin-arm64/host-lifecycle", ""),
	platformRule("platform/linux-amd64/lifecycle", readinessDecision),
	platformRule("platform/linux-amd64/host-lifecycle", readinessDecision),
	operatorRule("compliance/legal-files"),
	operatorRule("external/untouched-repository"),
	operatorRule("policy/rollback-exercise"),
	{id: "policy/signing", missing: ReadinessRow{ID: "policy/signing", Status: "NOT_RUN", Reason: "signing selection not recorded"}},
	fixedRule("policy/native-performance", readinessDecision, "GOC-V0-005: native performance is unmeasured"),
	fixedRule("owner/toolchain-security-review", readinessDecision, "subsequent owner action: compare the recorded toolchain with Go security releases before tagging"),
	fixedRule("owner/tag", "", "subsequent owner action"),
	fixedRule("owner/publication", "", "subsequent owner action"),
	fixedRule("owner/promotion", "", "subsequent owner action"),
}

func operatorRule(id string) readinessRule {
	return readinessRule{id: id, missing: ReadinessRow{ID: id, Status: "NOT_RUN", Reason: "evidence not supplied"}}
}

func platformRule(id, decision string) readinessRule {
	return readinessRule{id: id, fallback: true, missing: ReadinessRow{ID: id, Status: "FALLBACK", Decision: decision, Reason: "native lifecycle evidence not supplied (PRS-V1-004)"}}
}

func fixedRule(id, decision, reason string) readinessRule {
	return readinessRule{id: id, fixed: true, missing: ReadinessRow{ID: id, Status: "NOT_RUN", Decision: decision, Reason: reason}}
}

// readinessRules returns the catalogue for one version. Decision 0420 fixes
// 1.0.0-rc.1 signing as "No signing"; every other version records its own
// selection.
func readinessRules(version string) []readinessRule {
	rules := append([]readinessRule(nil), readinessCatalogue...)
	if version != firstStableCandidate {
		return rules
	}
	for index := range rules {
		if rules[index].id == "policy/signing" {
			rules[index] = fixedRule("policy/signing", readinessDecision, "No signing: SHA256SUMS only; publisher identity NOT_VERIFIED")
		}
	}
	return rules
}

// BuildReadinessRecord assembles the canonical record from one verified
// candidate, the source root and operator evidence. It runs no gate and
// writes nothing; the caller owns the output path.
func BuildReadinessRecord(ctx context.Context, options ReadinessOptions) (*ReadinessRecord, []byte, error) {
	candidate, err := VerifyContext(ctx, options.CandidateDirectory)
	if err != nil {
		return nil, nil, err
	}
	identity, core := readinessBinding(candidate)
	source := Options{SourceRoot: options.SourceRoot, Scratch: os.DevNull, Version: identity.Version}
	build, err := sourceBuildNumber(ctx, source, identity.Commit, identity.Tree)
	if err != nil {
		return nil, nil, err
	}
	if build != identity.BuildNumber {
		return nil, nil, fmt.Errorf("source build number %s does not equal candidate build %s", build, identity.BuildNumber)
	}
	vulnerability, err := readinessVulnerability(ctx, source, identity)
	if err != nil {
		return nil, nil, err
	}
	store, err := readinessStore(options.StoreReleaseID, options.StoreCandidateSHA256)
	if err != nil {
		return nil, nil, err
	}
	rows, err := readinessRows(readinessRules(identity.Version), options.Evidence)
	if err != nil {
		return nil, nil, err
	}
	record := &ReadinessRecord{Profile: readinessProfile, Identity: identity, Core: core, StoreRelease: store, Vulnerability: vulnerability, Rows: rows}
	raw, err := canonicalJSON(record)
	if err != nil {
		return nil, nil, err
	}
	return record, raw, nil
}

func readinessBinding(candidate *VerifiedCandidate) (ReadinessIdentity, ReadinessCore) {
	manifest := candidate.Manifest
	source := manifest.Sources[0]
	identity := ReadinessIdentity{Version: manifest.Version, Tag: "v" + manifest.Version, BuildNumber: manifest.BuildNumber, GoVersion: manifest.GoVersion, Commit: source.Commit, Tree: source.Tree}
	archives := []Asset{}
	for _, asset := range manifest.Assets {
		if asset.Role == "core-archive" {
			archives = append(archives, asset)
		}
	}
	core := ReadinessCore{CandidateProfile: manifest.Profile, ChecksumsSHA256: digest(candidate.files["SHA256SUMS"]), Archives: archives, Reproducibility: "PASS"}
	return identity, core
}

// readinessVulnerability applies decision 0420 rule (c): the Core go.mod at
// the bound commit has no require directive and the local toolchain is the
// pinned one. No scanner and no network are consulted.
func readinessVulnerability(ctx context.Context, source Options, identity ReadinessIdentity) (ReadinessVulnerability, error) {
	goMod, err := runSourceGit(ctx, source, "show", identity.Commit+":go.mod")
	if err != nil {
		return ReadinessVulnerability{}, fmt.Errorf("read Core go.mod at %s", identity.Commit)
	}
	toolchain, err := goToolchainProbe(ctx, source.SourceRoot)
	if err != nil {
		return ReadinessVulnerability{}, fmt.Errorf("probe local Go toolchain: %w", err)
	}
	requires := requireDirectives(goMod)
	return ReadinessVulnerability{Decision: readinessDecision, RequireDirectives: requires, Toolchain: toolchain, Status: vulnerabilityStatus(requires, toolchain, identity.GoVersion)}, nil
}

func vulnerabilityStatus(requires int, toolchain, candidateToolchain string) string {
	if requires == 0 && toolchain == pinnedToolchain && candidateToolchain == pinnedToolchain {
		return "PASS"
	}
	return "FAIL"
}

// requireDirectives counts single-line and block require directives. Space,
// tab and carriage return separate words, as in the go.mod lexer.
func requireDirectives(goMod string) int {
	count := 0
	for _, line := range strings.Split(goMod, "\n") {
		code, _, _ := strings.Cut(line, "//")
		words := strings.FieldsFunc(code, func(r rune) bool { return r == ' ' || r == '\t' || r == '\r' || r == '(' })
		if len(words) > 0 && words[0] == "require" {
			count++
		}
	}
	return count
}

func probeLocalToolchain(ctx context.Context, directory string) (string, error) {
	command := exec.CommandContext(ctx, "go", "env", "GOVERSION")
	command.Dir = directory
	command.Env = append(os.Environ(), "GOTOOLCHAIN=local")
	out, err := command.Output()
	return strings.TrimSpace(string(out)), err
}

func readinessStore(releaseID, candidateSHA256 string) (*ReadinessStoreRelease, error) {
	if releaseID == "" && candidateSHA256 == "" {
		return nil, nil
	}
	if !storeReleasePattern.MatchString(releaseID) || !digestPattern.MatchString(candidateSHA256) {
		return nil, fmt.Errorf("store release binding needs both a release id and a candidateSha256")
	}
	return &ReadinessStoreRelease{ReleaseID: releaseID, CandidateSHA256: candidateSHA256}, nil
}

func readinessRows(rules []readinessRule, evidence map[string]ReadinessEvidence) ([]ReadinessRow, error) {
	byID := map[string]readinessRule{}
	for _, rule := range rules {
		byID[rule.id] = rule
	}
	for id := range evidence {
		if rule, known := byID[id]; !known || rule.fixed {
			return nil, fmt.Errorf("readiness row %q does not take operator evidence", id)
		}
	}
	rows := make([]ReadinessRow, 0, len(rules))
	for _, rule := range rules {
		row, err := readinessRow(rule, evidence)
		if err != nil {
			return nil, err
		}
		rows = append(rows, row)
	}
	return rows, nil
}

func readinessRow(rule readinessRule, evidence map[string]ReadinessEvidence) (ReadinessRow, error) {
	supplied, present := evidence[rule.id]
	if !present {
		return rule.missing, nil
	}
	row := ReadinessRow{ID: rule.id, Status: supplied.Status, Decision: supplied.Decision, Reason: supplied.Reason}
	if supplied.Path != "" {
		raw, err := readRegular(supplied.Path, maxInputBytes)
		if err != nil {
			return ReadinessRow{}, err
		}
		row.SHA256 = digest(raw)
	}
	return row, validateReadinessRow(rule, row)
}

// validateReadinessRow is the status grammar: PASS and FAIL carry an evidence
// digest; NOT_RUN carries none and names its decision or reason; FALLBACK is
// admitted only on platform rows and also names its decision or reason. A
// platform row is never NOT_RUN, and a platform row whose default names a
// decision keeps that decision while it falls back (SRR-V1-007). Every row's
// reason must be legible, and only a reason with a letter or digit explains.
func validateReadinessRow(rule readinessRule, row ReadinessRow) error {
	if !legible(row.Reason) {
		return fmt.Errorf("readiness row %s has a reason with an invalid or hidden character", rule.id)
	}
	explained := row.Decision != "" || strings.IndexFunc(row.Reason, meaningful) >= 0
	valid := map[string]bool{
		"PASS":     digestPattern.MatchString(row.SHA256),
		"FAIL":     digestPattern.MatchString(row.SHA256),
		"NOT_RUN":  row.SHA256 == "" && explained && !rule.fallback,
		"FALLBACK": rule.fallback && explained && (row.SHA256 == "" || digestPattern.MatchString(row.SHA256)) && (rule.missing.Decision == "" || row.Decision == rule.missing.Decision),
	}[row.Status]
	if !valid || row.ID != rule.id || (row.Decision != "" && !decisionPattern.MatchString(row.Decision)) {
		return fmt.Errorf("readiness row %s has an invalid %s shape", rule.id, row.Status)
	}
	if rule.fixed && row != rule.missing {
		return fmt.Errorf("readiness row %s differs from its fixed value", rule.id)
	}
	return nil
}

// VerifyReadinessRecord refuses a malformed or non-canonical record, re-binds
// it to the verified candidate and recomputes every evidence digest from the
// operator-named files (row id to path).
func VerifyReadinessRecord(ctx context.Context, raw []byte, candidateDirectory string, evidence map[string]string) (*ReadinessRecord, error) {
	var record ReadinessRecord
	if err := decodeClosed(raw, &record); err != nil {
		return nil, fmt.Errorf("readiness record: %w", err)
	}
	canonical, err := canonicalJSON(record)
	if err != nil || !bytes.Equal(canonical, raw) || record.Profile != readinessProfile {
		return nil, fmt.Errorf("readiness record is not canonical %s", readinessProfile)
	}
	candidate, err := VerifyContext(ctx, candidateDirectory)
	if err != nil {
		return nil, err
	}
	identity, core := readinessBinding(candidate)
	if record.Identity != identity || !reflect.DeepEqual(record.Core, core) {
		return nil, fmt.Errorf("readiness record does not bind the verified candidate")
	}
	if err := verifyReadinessClaims(record); err != nil {
		return nil, err
	}
	if err := verifyReadinessEvidence(record.Rows, evidence); err != nil {
		return nil, err
	}
	return &record, nil
}

func verifyReadinessClaims(record ReadinessRecord) error {
	if record.StoreRelease != nil {
		store, err := readinessStore(record.StoreRelease.ReleaseID, record.StoreRelease.CandidateSHA256)
		if err != nil {
			return err
		}
		if store == nil {
			return fmt.Errorf("store release binding is empty; record null instead")
		}
	}
	vulnerability := record.Vulnerability
	if vulnerability.Decision != readinessDecision || vulnerability.RequireDirectives < 0 || vulnerability.Status != vulnerabilityStatus(vulnerability.RequireDirectives, vulnerability.Toolchain, record.Identity.GoVersion) {
		return fmt.Errorf("readiness vulnerability rule is inconsistent")
	}
	rules := readinessRules(record.Identity.Version)
	if len(record.Rows) != len(rules) {
		return fmt.Errorf("readiness row catalogue is not closed")
	}
	for index, rule := range rules {
		if err := validateReadinessRow(rule, record.Rows[index]); err != nil {
			return err
		}
	}
	return nil
}

func verifyReadinessEvidence(rows []ReadinessRow, evidence map[string]string) error {
	digested := map[string]string{}
	for _, row := range rows {
		if row.SHA256 != "" {
			digested[row.ID] = row.SHA256
		}
	}
	if len(evidence) != len(digested) {
		return fmt.Errorf("readiness evidence files do not match the digest-bearing rows")
	}
	for id, want := range digested {
		raw, err := readRegular(evidence[id], maxInputBytes)
		if err != nil || digest(raw) != want {
			return fmt.Errorf("readiness row %s evidence digest does not reproduce", id)
		}
	}
	return nil
}

// ReadReadinessEvidence parses the operator evidence file (SRR-V1-012): one
// row per line, ROW<TAB>STATUS<TAB>PATH<TAB>DECISION<TAB>REASON, with absent
// values left empty. A relative PATH is appended to the file's directory as
// spelled, not cleaned, so the kernel resolves a ".." after the symlinks before
// it, as opening the path would. A CRLF line ending is read as LF.
func ReadReadinessEvidence(path string) (map[string]ReadinessEvidence, error) {
	raw, err := readRegular(path, maxInputBytes)
	if err != nil {
		return nil, err
	}
	directory, _ := filepath.Split(path)
	evidence := map[string]ReadinessEvidence{}
	number := 0
	for line := range strings.Lines(string(raw)) {
		number++
		fields := strings.Split(strings.TrimSuffix(strings.TrimSuffix(line, "\n"), "\r"), "\t")
		if len(fields) != 5 {
			return nil, fmt.Errorf("evidence file line %d does not have five tab-separated fields", number)
		}
		if _, duplicate := evidence[fields[0]]; duplicate {
			return nil, fmt.Errorf("evidence file names row %q twice", fields[0])
		}
		location := fields[2]
		if location != "" && !filepath.IsAbs(location) {
			location = directory + location
		}
		evidence[fields[0]] = ReadinessEvidence{Status: fields[1], Path: location, Decision: fields[3], Reason: fields[4]}
	}
	return evidence, nil
}

// WriteReadinessRecord builds the record and publishes it at output. An
// existing output is refused, never replaced, and so is an output inside the
// candidate or the source root, which build must not write (SRR-V1-011,
// SRR-V1-012). A file name the record cannot be published under is refused
// before the build. The output's directory is resolved and opened once before
// the guard, and the write goes through that handle, so a path component
// replaced during the build cannot redirect it. After the open the directory is
// resolved again, that settled path must still be the handle's, and the guard
// walks it, so an ancestor swapped for a link before the open is walked through
// the link's target.
func WriteReadinessRecord(ctx context.Context, options ReadinessOptions, output string) (*ReadinessRecord, error) {
	spelled, name := filepath.Split(output)
	if err := outputName(name); err != nil {
		return nil, err
	}
	directory, err := physical(spelled)
	if err != nil {
		return nil, unresolvedDirectory(cmp.Or(spelled, "."), err)
	}
	handle, err := openOutputDirectory(directory)
	if err != nil {
		return nil, err
	}
	defer handle.Close()
	if err := sameDirectory(handle, directory, cmp.Or(spelled, ".")); err != nil {
		return nil, err
	}
	settled, err := filepath.EvalSymlinks(directory)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", errOutputDirectory, err)
	}
	if err := sameDirectory(handle, settled, cmp.Or(spelled, ".")); err != nil {
		return nil, err
	}
	if err := refuseInside(output, settled, options.CandidateDirectory, options.SourceRoot); err != nil {
		return nil, err
	}
	record, raw, err := BuildReadinessRecord(ctx, options)
	if err != nil {
		return nil, err
	}
	return record, publishNoReplace(handle, name, raw)
}

// errOutputDirectory refuses an output whose directory, as spelled, is not the
// directory that was resolved, checked and opened for the write.
var errOutputDirectory = errors.New("readiness record directory as spelled is not the directory resolved for the write")

// sameDirectory refuses unless the resolved directory, taken as it stands, and
// the directory as spelled are the directory the handle holds. EvalSymlinks
// follows up to 255 links but the kernel far fewer, so a long chain can resolve
// while the spelled path, which the command reports, cannot be opened. A
// Windows junction never reaches this check: physical refuses it.
func sameDirectory(handle *os.Root, directory, spelled string) error {
	opened, err := handle.Stat(".")
	if err != nil {
		return err
	}
	resolved, resolvedErr := os.Lstat(directory)
	reached, reachedErr := os.Stat(spelled)
	if err := errors.Join(resolvedErr, reachedErr); err != nil {
		return fmt.Errorf("%w: %w", errOutputDirectory, err)
	}
	if !os.SameFile(opened, resolved) || !os.SameFile(opened, reached) {
		return fmt.Errorf("%w: %s", errOutputDirectory, spelled)
	}
	return nil
}

// hiddenCharacters are the classes a reason must not carry, because they can
// hide, reorder or blank what it says: controls, format characters, line and
// paragraph separators, private-use code points, noncharacters, variation
// selectors, and code points that render as nothing.
var hiddenCharacters = []*unicode.RangeTable{unicode.Cc, unicode.Cf, unicode.Zl, unicode.Zp, unicode.Co, unicode.Noncharacter_Code_Point, unicode.Variation_Selector, unicode.Other_Default_Ignorable_Code_Point}

// legible reports whether a reason is valid UTF-8 with no hidden character.
func legible(reason string) bool {
	return utf8.ValidString(reason) && strings.IndexFunc(reason, hidden) < 0
}

func hidden(r rune) bool { return unicode.IsOneOf(hiddenCharacters, r) }

// meaningful is a letter or a digit; a reason with neither explains nothing.
func meaningful(r rune) bool { return unicode.IsLetter(r) || unicode.IsNumber(r) }

// physical returns directory as an absolute path free of symlinks. A relative
// directory is appended to the working directory's own resolved path, and every
// component, ".." included, is resolved in order as a write resolves it. A
// directory rooted on a drive or a separator but not absolute (Windows "C:x"
// or "\x") is refused rather than guessed. Go reads a Windows junction or
// volume mount point as neither a directory nor a symlink, so EvalSymlinks
// fails with ENOTDIR at one that has a component or a separator after it. The
// directory filepath.Split leaves ends in a separator, so every junction on an
// output directory fails here.
func physical(directory string) (string, error) {
	if filepath.IsAbs(directory) {
		return filepath.EvalSymlinks(directory)
	}
	if filepath.VolumeName(directory) != "" || strings.HasPrefix(filepath.ToSlash(directory), "/") {
		return "", errors.New("rooted but not absolute")
	}
	working, err := os.Getwd()
	if err != nil {
		return "", err
	}
	working, err = filepath.EvalSymlinks(working)
	if err != nil {
		return "", err
	}
	return filepath.EvalSymlinks(working + string(filepath.Separator) + directory)
}

// unresolvedDirectory names the output directory, as spelled, that physical
// could not resolve. On Windows, ENOTDIR is ERROR_PATH_NOT_FOUND, which reads
// "The system cannot find the path specified" and is also what a missing drive
// gives, so the possible causes are named here.
func unresolvedDirectory(spelled string, err error) error {
	if errors.Is(err, syscall.ENOTDIR) {
		return fmt.Errorf("readiness record directory %s does not resolve as a directory: it passes through a file, a Windows junction or volume mount point, or a missing Windows drive: %w", spelled, err)
	}
	return fmt.Errorf("readiness record directory %s does not resolve: %w", spelled, err)
}

func refuseInside(output, directory string, roots ...string) error {
	for _, root := range roots {
		inside, err := within(directory, root)
		if err != nil {
			return err
		}
		if inside {
			return fmt.Errorf("readiness record %s is inside %s, which build must not write", output, root)
		}
	}
	return nil
}

// within reports whether directory, an absolute path free of symlinks, is root
// or lies below it. It compares file identity at each parent, so a symlink or a
// case alias of root cannot hide the overlap. A mount alias of a directory
// below root (a bind mount, a Windows subst drive, a network mount) has its own
// parents, so it is outside this guard. The parents are found by path, not
// from an open handle, because os.Root cannot open a handle's parent and Go has
// no portable openat. The caller resolves directory after opening it and checks
// it against the handle, so an ancestor swapped for a link before the open is
// walked through its target. A writer that controls an ancestor of root itself
// is outside this guard.
func within(directory, root string) (bool, error) {
	rootInfo, err := os.Stat(root)
	if err != nil {
		return false, nil
	}
	for {
		info, err := os.Stat(directory)
		if err != nil {
			return false, err
		}
		if os.SameFile(info, rootInfo) {
			return true, nil
		}
		parent := filepath.Dir(directory)
		if parent == directory {
			return false, nil
		}
		directory = parent
	}
}

// publishNoReplace writes raw to a temporary file in the directory handle and
// hard-links it to name there, so an existing file is refused and a partial
// record never appears at name. Every step goes through the handle.
func publishNoReplace(handle *os.Root, name string, raw []byte) error {
	path := filepath.Join(handle.Name(), name)
	temporary, file, err := createTemporary(handle, name)
	if err != nil {
		return err
	}
	defer handle.Remove(temporary)
	_, writeErr := file.Write(raw)
	syncErr := file.Sync()
	closeErr := file.Close()
	if err := errors.Join(writeErr, syncErr, closeErr); err != nil {
		return err
	}
	if err := handle.Link(temporary, name); err != nil {
		return linkFailure(path, err)
	}
	return nil
}

// linkFailure words a refused publication by its cause: a file already at
// path, which is kept, or any other link error, such as a filesystem without
// hard links.
func linkFailure(path string, err error) error {
	if errors.Is(err, os.ErrExist) {
		return fmt.Errorf("readiness record %s already exists and was not replaced: %w", path, err)
	}
	return fmt.Errorf("readiness record %s was not published: linking its temporary file failed: %w", path, err)
}

// temporaryAttempts bounds the random names tried for the temporary file.
const temporaryAttempts = 16

// maxNameBytes is NAME_MAX on Linux and macOS. A name within it also fits the
// 255 UTF-16 units Windows allows.
const maxNameBytes = 255

// outputName refuses a file name the record cannot be published under: none,
// a dot entry, or one whose temporary name would exceed maxNameBytes.
func outputName(name string) error {
	if slices.Contains([]string{"", ".", ".."}, name) {
		return fmt.Errorf("readiness record file name %q is empty or a dot entry", name)
	}
	if len(temporaryName(name)) > maxNameBytes {
		return fmt.Errorf("readiness record file name %s is too long: its temporary name would exceed %d bytes", name, maxNameBytes)
	}
	return nil
}

// temporaryName is a random hidden name beside name.
func temporaryName(name string) string { return "." + name + "." + temporaryText() }

// createTemporary creates a new file with a random name beside name,
// refusing any name that already exists.
func createTemporary(handle *os.Root, name string) (string, *os.File, error) {
	for range temporaryAttempts {
		temporary := temporaryName(name)
		file, err := handle.OpenFile(temporary, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
		if errors.Is(err, os.ErrExist) {
			continue
		}
		return temporary, file, err
	}
	return "", nil, fmt.Errorf("readiness record %s: no unused temporary name", name)
}

// VerifyReadinessFile verifies the record at path, then refuses rows that
// differ from the rows the evidence file builds, so a record cannot relabel
// its evidence (a FAIL log as PASS, say) and still verify (SRR-V1-012).
func VerifyReadinessFile(ctx context.Context, path, candidateDirectory string, evidence map[string]ReadinessEvidence) (*ReadinessRecord, error) {
	raw, err := readRegular(path, maxInputBytes)
	if err != nil {
		return nil, err
	}
	record, err := VerifyReadinessRecord(ctx, raw, candidateDirectory, evidencePaths(evidence))
	if err != nil {
		return nil, err
	}
	rows, err := readinessRows(readinessRules(record.Identity.Version), evidence)
	if err != nil {
		return nil, err
	}
	if !reflect.DeepEqual(rows, record.Rows) {
		return nil, fmt.Errorf("readiness record rows differ from the evidence file")
	}
	return record, nil
}

func evidencePaths(evidence map[string]ReadinessEvidence) map[string]string {
	paths := map[string]string{}
	for id, supplied := range evidence {
		if supplied.Path != "" {
			paths[id] = supplied.Path
		}
	}
	return paths
}
