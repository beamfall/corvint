package doccorpus

import (
	"bytes"
	"context"
	json "encoding/json/v2"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"runtime"
	"slices"
	"sort"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/Beamfall/corvint/internal/cem/gitauth"
	"github.com/Beamfall/corvint/internal/cem/wire"
	"github.com/Beamfall/corvint/internal/contextindex"
	"github.com/Beamfall/corvint/internal/liveverify/affected"
	"github.com/Beamfall/corvint/internal/testvaliditydoc"
)

// ExtractorProfileSchema names a repository-declared extractor profile and
// ExtractorInventorySchema the generated inventory it produces (DCX-V1-390).
const (
	ExtractorProfileSchema   = "corvint-extractor-profile/1"
	ExtractorInventorySchema = "corvint-extractor-inventory/1"
)

// Extractor profile bounds (DCX-V1-390).
const (
	MaxExtractors       = 64
	MaxExtractorFiles   = 64
	MaxExtractorPattern = 4096
	maxExtractorCapture = 512
	// maxExtractorGroups bounds each pattern's capture groups, so a line's matches stay small.
	maxExtractorGroups = 16
)

// ExtractorProfile is project-owned configuration read from a Git blob at the extracted revision.
// Its subjects use only Core's generic subject kinds; a profile adds no taxonomy (DCX-V1-390).
type ExtractorProfile struct {
	Schema     string      `json:"schema"`
	Provider   string      `json:"provider"`
	Version    string      `json:"version"`
	Extractors []Extractor `json:"extractors"`
}

// Extractor is one declarative, line-oriented pattern over a set of repository files.
type Extractor struct {
	ID           string               `json:"id"`
	Subject      string               `json:"subject"`
	Role         string               `json:"role,omitempty"`
	Files        []string             `json:"files"`
	Pattern      string               `json:"pattern"`
	Candidate    string               `json:"candidate,omitempty"`
	FieldPattern string               `json:"field_pattern,omitempty"`
	EndPattern   string               `json:"end_pattern,omitempty"`
	ToolOutput   *ExtractorToolOutput `json:"tool_output,omitempty"`
}

// ExtractorToolOutput pins framework tool output committed to the repository: the tool that
// produced it and the SHA-256 of its exact bytes (DCX-V1-394). The tool is never run.
type ExtractorToolOutput struct {
	Tool   string `json:"tool"`
	SHA256 string `json:"sha256"`
}

// ExtractorInput is one extraction: the repository root, the full commit whose blobs are read and
// the repository path of the profile at that commit. With ProfileSHA256 set, Profile is instead
// the absolute local path of a caller-owned profile outside the repository, admitted only when its
// bytes hash to ProfileSHA256 (DCX-V1-478).
type ExtractorInput struct {
	Root, Revision, Profile string
	ProfileSHA256           string
	// Schema "2" emits the provider member as a corvint-corpus-adoption-provider/1 record for a
	// corvint-corpus-input/2 manifest, bounded by MaxCorpusRecords and MaxProviderBytes instead
	// of MaxRecords and MaxBytes (DCX-V1-560). Empty or "1" keeps the provider/1 record.
	Schema string
}

// ExtractorProfileOriginCaller marks an inventory produced from a caller-owned profile.
const ExtractorProfileOriginCaller = "caller"

// ExtractorProfileRef pins the profile the inventory was produced from: a repository profile by
// its path and Git blob, a caller-owned one by its origin, the SHA-256 of its exact bytes, its
// canonical document and that document's canonical SHA-256 (DCX-V1-503). The local path of a
// caller-owned profile is never recorded.
type ExtractorProfileRef struct {
	Origin          string            `json:"origin,omitempty"`
	Path            string            `json:"path,omitempty"`
	Blob            string            `json:"blob,omitempty"`
	SHA256          string            `json:"sha256"`
	CanonicalSHA256 string            `json:"canonical_sha256,omitempty"`
	Document        *ExtractorProfile `json:"document,omitempty"`
}

// ExtractorInventory is generated, never reviewed. Provider is a complete
// corvint-corpus-provider/1 record, or under --schema 2 a corvint-corpus-adoption-provider/1
// record (DCX-V1-560), a project can commit and declare; Gaps lists everything the profile could
// not turn into evidence (DCX-V1-395).
type ExtractorInventory struct {
	Schema      string              `json:"schema"`
	Status      string              `json:"status"`
	Repository  string              `json:"repository"`
	Revision    string              `json:"revision"`
	Profile     ExtractorProfileRef `json:"profile"`
	Provider    ProviderRecord      `json:"provider"`
	Gaps        []ExtractorGap      `json:"gaps"`
	Limitations []string            `json:"limitations"`
}

// ExtractorGap is one unreadable file, unsupported shape or unresolved reference.
type ExtractorGap struct {
	Extractor string  `json:"extractor"`
	Path      string  `json:"path,omitempty"`
	Line      int     `json:"line,omitempty"`
	Kind      string  `json:"kind"`
	Reason    string  `json:"reason"`
	Evidence  *Anchor `json:"evidence,omitempty"`
}

var extractorLimitations = []string{
	"line-oriented declarative patterns only: no language is parsed, so a declaration spread over lines, built dynamically or produced by metaprogramming is a gap or is not seen",
	"one match per line; a line with several matches is a multiple-matches gap",
	"handlers join symbol subjects of this profile by exact captured name; an unmatched or multiply defined name is a gap, never a guess",
	"framework tool output is read only as a committed Git blob whose SHA-256 the profile pins; no tool is run",
	"profile subject types are configuration over Core's generic subject kinds and grant no authority",
	"every subject and relation is generated evidence; none is reviewed",
}

var (
	extractorSubjects = words("endpoint ui_surface data_entity job event symbol")
	extractorRoles    = words("declares produces consumes")
	extractorProvider = regexp.MustCompile(`^[a-z][a-z0-9-]{0,63}$`)
	extractorID       = regexp.MustCompile(`^[a-z0-9][a-z0-9_-]{0,63}$`)
	extractorDigest   = regexp.MustCompile(`^[0-9a-f]{64}$`)
	// extractorGroups names the capture groups each subject requires and admits.
	extractorGroups = map[string][2][]string{
		"endpoint":    {{"method", "path"}, {"handler"}},
		"ui_surface":  {{"path"}, {"handler"}},
		"data_entity": {{"entity"}, nil},
		"job":         {{"name"}, nil},
		"event":       {{"name"}, nil},
		"symbol":      {{"name"}, nil},
	}
)

type compiledExtractor struct {
	Extractor
	pattern, candidate, field, end *regexp.Regexp
}

// extractorSite is one matched line: the subject it names, an optional handler and its anchor.
type extractorSite struct {
	ex      *compiledExtractor
	kind    string
	name    string
	parent  string // data_entity field: its entity subject name
	handler string
	anchor  Anchor
	derived string
}

type extractorFile struct {
	path, blob string
	data       []byte
	digest     string   // SHA-256 of data, computed once per file
	lines      []string // data split after each newline, computed once per file
}

type extractorProducer struct {
	provider, repository, revision string
	// callerProfile names a caller-owned profile's digest in the subjects capability, present or
	// absent, so the provider record a corpus admits still names the profile it was produced
	// from (DCX-V1-503).
	callerProfile string
	inv           *ExtractorInventory
	sites         []extractorSite
	// limit bounds sites, gaps, matched files per extractor, subjects and relations; schema is
	// the emitted provider record schema (DCX-V1-560).
	limit  int
	schema string
}

// bound refuses an inventory whose count of what has crossed the record bound, naming the count
// reached, the bound and the extractor and path that crossed it, and for the provider/1 family
// the --schema 2 bound that admits more (DCX-V1-561).
func (p *extractorProducer) bound(what string, count int, extractor, path string) error {
	at := ""
	if extractor != "" {
		at = " at extractor " + strconv.Quote(extractor)
		if path != "" {
			at += " path " + path
		}
	}
	message := fmt.Sprintf("extractor inventory exceeds the record bound: %d %s%s, over the bound of %d", count, what, at, p.limit)
	if p.schema == ProviderSchema {
		message += fmt.Sprintf("; --schema 2 emits an adoption provider record for a manifest /2, which admits %d", MaxCorpusRecords)
	}
	return fail(message)
}

// over reports the first of sites or gaps beyond the record bound.
func (p *extractorProducer) over(ex *compiledExtractor, f *extractorFile) error {
	path := ""
	if f != nil {
		path = f.path
	}
	if len(p.sites) > p.limit {
		return p.bound("sites", len(p.sites), ex.ID, path)
	}
	if len(p.inv.Gaps) > p.limit {
		return p.bound("gaps", len(p.inv.Gaps), ex.ID, path)
	}
	return nil
}

// BuildExtractorInventory reads a repository-declared extractor profile and the files it names
// from Git at a full commit and emits the generated corvint-extractor-inventory/1 record
// (DCX-V1-390..398). It never runs a tool, never reads the working tree and writes nothing.
func BuildExtractorInventory(ctx context.Context, input ExtractorInput) ([]byte, error) {
	if !wire.IsGitOid(input.Revision) {
		return nil, fail("extraction requires a full commit revision")
	}
	caller := input.ProfileSHA256 != ""
	limit, schema := MaxRecords, ProviderSchema
	switch input.Schema {
	case "", "1":
	case "2":
		limit, schema = MaxCorpusRecords, AdoptionProviderSchema
	default:
		return nil, fail("--schema must be 1 or 2")
	}
	switch {
	case caller && !extractorDigest.MatchString(input.ProfileSHA256):
		return nil, fail("--profile-sha256 must be 64 lowercase hexadecimal digits")
	case caller && (!filepath.IsAbs(input.Profile) || filepath.Clean(input.Profile) != input.Profile):
		return nil, fail("a caller-owned extractor profile must be a clean absolute path outside the repository")
	case !caller && !affected.ValidRelativePath(input.Profile):
		message := "extractor profile is not a canonical repository-relative path"
		if filepath.IsAbs(input.Profile) {
			message += "; a caller-owned profile outside the repository also needs --profile-sha256"
		}
		return nil, fail(message)
	}
	repository, err := contextindex.CorpusRepositoryID(ctx, input.Root, input.Revision)
	if err != nil {
		return nil, err
	}
	auth, err := gitauth.Open(input.Root, newGitBudget(ctx))
	if err != nil {
		return nil, err
	}
	release := auth.BeginObjectSession()
	defer release()
	var raw []byte
	var ref ExtractorProfileRef
	name := input.Profile
	if caller {
		name = "caller extractor profile"
		if raw, err = readCallerProfile(auth.Root, input.Profile, input.ProfileSHA256); err != nil {
			return nil, err
		}
		ref = ExtractorProfileRef{Origin: ExtractorProfileOriginCaller, SHA256: input.ProfileSHA256}
	} else {
		entry, ok, err := auth.LookupTreeEntry(ctx, input.Revision, input.Profile)
		if err != nil {
			return nil, err
		}
		if !ok || !regularBlob(entry) {
			return nil, fail("extractor profile is not a regular Git blob at the revision: " + input.Profile)
		}
		var over bool
		raw, over, err = auth.BlobBytesWithin(ctx, entry.OID, MaxBytes)
		if err != nil {
			return nil, err
		}
		if over {
			return nil, fail("extractor profile exceeds the input bound")
		}
		ref = ExtractorProfileRef{Path: input.Profile, Blob: entry.OID, SHA256: Digest(raw)}
	}
	var profile ExtractorProfile
	if err := decodeNamed(name, raw, &profile, MaxBytes); err != nil {
		return nil, err
	}
	extractors, err := compileExtractorProfile(profile)
	if err != nil {
		return nil, err
	}
	limitations := extractorLimitations
	if caller {
		canonical, err := Encode(profile)
		if err != nil {
			return nil, err
		}
		ref.CanonicalSHA256, ref.Document = Digest(canonical), &profile
		limitations = append(slices.Clone(extractorLimitations), callerProfileLimitation)
	}
	p := &extractorProducer{provider: profile.Provider, limit: limit, schema: schema}
	paths, err := extractorPaths(ctx, auth, input.Revision, extractors, p)
	if err != nil {
		return nil, err
	}
	all := []string{}
	for _, list := range paths {
		all = append(all, list...)
	}
	batched := newPathPrefetch(input.Revision)
	if err := batched.resolve(ctx, auth, all); err != nil {
		return nil, err
	}
	if err := batched.read(ctx, auth, batched.regularOIDs(all), MaxBytes, maxPinnedInputBytes); err != nil {
		return nil, err
	}
	source := Repository{ID: repository, Revision: input.Revision}
	inv := &ExtractorInventory{Schema: ExtractorInventorySchema, Status: "generated", Repository: repository, Revision: input.Revision, Profile: ref, Gaps: []ExtractorGap{}, Limitations: limitations}
	p.repository, p.revision, p.inv = repository, input.Revision, inv
	if caller {
		p.callerProfile = "caller-owned extractor profile sha256:" + input.ProfileSHA256
	}
	for i := range extractors {
		ex := &extractors[i]
		if len(paths[i]) == 0 {
			p.gap(ex, nil, 0, "no-files-matched", "no file at the revision matches the extractor's file patterns")
			continue
		}
		for _, current := range paths[i] {
			file, kind, reason, err := extractorRead(ctx, auth, batched, input.Revision, current)
			if err != nil {
				return nil, err
			}
			if file == nil {
				p.inv.Gaps = append(p.inv.Gaps, ExtractorGap{Extractor: ex.ID, Path: current, Kind: kind, Reason: reason})
				continue
			}
			if ex.ToolOutput != nil && file.digest != ex.ToolOutput.SHA256 {
				p.gap(ex, file, 0, "tool-output-digest-mismatch", "the committed tool output does not match the profile's pinned SHA-256; it is not read")
				continue
			}
			if err := p.file(ex, file); err != nil {
				return nil, err
			}
		}
	}
	record, err := p.record(source, profile.Version)
	if err != nil {
		return nil, err
	}
	inv.Provider = record
	if len(inv.Gaps) > p.limit {
		return nil, p.bound("gaps", len(inv.Gaps), "", "")
	}
	// DCX-V1-560: the provider member is committed unchanged as one provider input, so it must
	// fit the input bound build admits for its schema.
	encoded, err := json.Marshal(record, json.Deterministic(true))
	if err != nil {
		return nil, err
	}
	if limit := encodingLimit(record); len(encoded)+1 > limit {
		message := fmt.Sprintf("extractor provider record is %d bytes, over the %s input bound of %d bytes", len(encoded)+1, record.Schema, limit)
		if schema == ProviderSchema {
			message += fmt.Sprintf("; --schema 2 emits an adoption provider record, which admits %d bytes", MaxProviderBytes)
		}
		return nil, fail(message)
	}
	return Encode(inv)
}

const callerProfileLimitation = "the extractor profile is caller-owned configuration pinned by SHA-256, not declared by the repository; sources are still read only from Git at the revision"

// callerProfileSupported reports whether this platform has the no-follow component reader a
// caller-owned profile is read through; elsewhere it is refused (DCX-V1-478).
var callerProfileSupported = runtime.GOOS == "darwin" || runtime.GOOS == "linux"

// readCallerProfileFile is the pinned no-follow read; tests observe the window after the check.
var readCallerProfileFile = testvaliditydoc.ReadAbsoluteFileBounded

// readCallerProfile reads a caller-owned profile outside the repository root, no larger than the
// input bound, and admits it only when its exact bytes hash to digest (DCX-V1-478, DCX-V1-504).
// The path is resolved once; every ancestor of the resolved directory is compared with the
// repository root by filesystem identity, so a case or hard-link alias of the root is still
// inside, and the file is then read along that symlink-free path with every directory component
// pinned and no symlink followed, so a component replaced by a symlink after the check is
// refused. Nothing is written.
func readCallerProfile(repositoryRoot, name, digest string) ([]byte, error) {
	if !callerProfileSupported {
		return nil, fail("caller-owned extractor profiles need the no-follow file reader, which this platform lacks; commit the profile and pass its repository path")
	}
	unavailable := &Error{Code: "corpus-input-unavailable", Message: "caller-owned extractor profile missing, unsafe or over bound"}
	dir, err := filepath.EvalSymlinks(filepath.Dir(name))
	if err != nil {
		return nil, unavailable
	}
	repository, err := os.Stat(repositoryRoot)
	if err != nil {
		return nil, err
	}
	for ancestor := dir; ; ancestor = filepath.Dir(ancestor) {
		info, err := os.Lstat(ancestor)
		if err != nil {
			return nil, unavailable
		}
		if os.SameFile(info, repository) {
			return nil, fail("a caller-owned extractor profile must lie outside the extracted repository; commit it and pass its repository path without --profile-sha256")
		}
		if parent := filepath.Dir(ancestor); parent == ancestor {
			break
		}
	}
	raw, err := readCallerProfileFile(filepath.Join(dir, filepath.Base(name)), MaxBytes)
	if err != nil {
		return nil, unavailable
	}
	if got := Digest(raw); got != digest {
		return nil, fail("caller-owned extractor profile does not match --profile-sha256 (file sha256 " + got + ")")
	}
	return raw, nil
}

// compileExtractorProfile admits the closed profile shape and compiles its patterns (DCX-V1-390).
func compileExtractorProfile(profile ExtractorProfile) ([]compiledExtractor, error) {
	if profile.Schema != ExtractorProfileSchema {
		return nil, fail("extractor profile schema must be " + ExtractorProfileSchema)
	}
	if !extractorProvider.MatchString(profile.Provider) || !safeText(profile.Version, 64) {
		return nil, fail("extractor profile needs a lowercase provider id and a bounded version")
	}
	if len(profile.Extractors) == 0 || len(profile.Extractors) > MaxExtractors {
		return nil, fail("extractor profile needs 1.." + strconv.Itoa(MaxExtractors) + " extractors")
	}
	seen := map[string]bool{}
	out := make([]compiledExtractor, 0, len(profile.Extractors))
	for _, ex := range profile.Extractors {
		where := "extractor " + strconv.Quote(ex.ID) + ": "
		if !extractorID.MatchString(ex.ID) || seen[ex.ID] {
			return nil, fail("extractor ids must be unique lowercase identifiers")
		}
		seen[ex.ID] = true
		if !extractorSubjects[ex.Subject] {
			return nil, fail(where + "subject must be endpoint, ui_surface, data_entity, job, event or symbol")
		}
		if (ex.Subject == "job" || ex.Subject == "event") != (ex.Role != "") || ex.Role != "" && !extractorRoles[ex.Role] {
			return nil, fail(where + "job and event extractors need a role of declares, produces or consumes; other subjects admit none")
		}
		if ex.Subject != "data_entity" && (ex.FieldPattern != "" || ex.EndPattern != "") {
			return nil, fail(where + "field_pattern and end_pattern are data_entity options")
		}
		if len(ex.Files) == 0 || len(ex.Files) > MaxExtractorFiles {
			return nil, fail(where + "needs 1.." + strconv.Itoa(MaxExtractorFiles) + " file patterns")
		}
		for _, pattern := range ex.Files {
			if !extractorFilePattern(pattern) {
				return nil, fail(where + "invalid file pattern " + strconv.Quote(pattern))
			}
		}
		if ex.ToolOutput != nil {
			if len(ex.Files) != 1 || extractorGlob(ex.Files[0]) || !safeText(ex.ToolOutput.Tool, 256) || !extractorDigest.MatchString(ex.ToolOutput.SHA256) {
				return nil, fail(where + "tool_output needs one literal file, a tool name and a lowercase SHA-256")
			}
		}
		c := compiledExtractor{Extractor: ex}
		var err error
		groups := extractorGroups[ex.Subject]
		if c.pattern, err = extractorRegexp(ex.Pattern, groups[0], groups[1]); err != nil {
			return nil, fail(where + "pattern " + err.Error())
		}
		if ex.Candidate != "" {
			if c.candidate, err = extractorRegexp(ex.Candidate, nil, nil); err != nil {
				return nil, fail(where + "candidate " + err.Error())
			}
		}
		if ex.FieldPattern != "" {
			if c.field, err = extractorRegexp(ex.FieldPattern, []string{"field"}, nil); err != nil {
				return nil, fail(where + "field_pattern " + err.Error())
			}
		}
		if ex.EndPattern != "" {
			if ex.FieldPattern == "" {
				return nil, fail(where + "end_pattern requires field_pattern")
			}
			if c.end, err = extractorRegexp(ex.EndPattern, nil, nil); err != nil {
				return nil, fail(where + "end_pattern " + err.Error())
			}
		}
		out = append(out, c)
	}
	return out, nil
}

// extractorRegexp compiles one bounded RE2 pattern whose named groups are exactly the required
// ones plus any admitted optional ones.
func extractorRegexp(pattern string, required, optional []string) (*regexp.Regexp, error) {
	if pattern == "" || len(pattern) > MaxExtractorPattern || !utf8.ValidString(pattern) {
		return nil, fail("must be a non-empty UTF-8 pattern of at most " + strconv.Itoa(MaxExtractorPattern) + " bytes")
	}
	re, err := regexp.Compile(pattern)
	if err != nil {
		return nil, fail("is not a valid RE2 pattern")
	}
	if re.NumSubexp() > maxExtractorGroups {
		return nil, fail("has more than " + strconv.Itoa(maxExtractorGroups) + " capture groups")
	}
	named := map[string]bool{}
	for _, name := range re.SubexpNames() {
		if name == "" {
			continue
		}
		if !slices.Contains(required, name) && !slices.Contains(optional, name) || named[name] {
			return nil, fail("names an unadmitted or repeated group " + strconv.Quote(name))
		}
		named[name] = true
	}
	for _, name := range required {
		if !named[name] {
			return nil, fail("lacks the required named group " + strconv.Quote(name))
		}
	}
	return re, nil
}

func extractorGlob(pattern string) bool { return strings.ContainsAny(pattern, "*?[") }

// extractorFilePattern admits a canonical relative path whose segments are literal names, path.Match
// patterns or a whole `**` segment.
func extractorFilePattern(pattern string) bool {
	if !extractorGlob(pattern) {
		return affected.ValidRelativePath(pattern)
	}
	if len(pattern) > 1024 || strings.HasPrefix(pattern, "/") || strings.ContainsAny(pattern, "\\\x00\r\n\t") {
		return false
	}
	for _, segment := range strings.Split(pattern, "/") {
		if segment == "" || segment == "." || segment == ".." {
			return false
		}
		if segment == "**" {
			continue
		}
		if strings.Contains(segment, "**") {
			return false
		}
		if _, err := path.Match(segment, ""); err != nil {
			return false
		}
	}
	return true
}

// extractorMatch reports whether name matches a validated pattern; `**` matches zero or more
// whole segments.
func extractorMatch(pattern, name string) bool {
	return extractorMatchParts(strings.Split(pattern, "/"), strings.Split(name, "/"))
}

// extractorMatchParts is a dynamic program over pattern and name segments, so any number of `**`
// segments costs O(len(pattern)*len(name)) segment matches instead of backtracking.
// reach[j] reports whether the pattern prefix consumed so far matches name[:j].
func extractorMatchParts(pattern, name []string) bool {
	reach := make([]bool, len(name)+1)
	reach[0] = true
	next := make([]bool, len(name)+1)
	for _, segment := range pattern {
		clear(next)
		if segment == "**" {
			seen := false
			for j := range reach {
				seen = seen || reach[j]
				next[j] = seen
			}
		} else {
			for j := 1; j <= len(name); j++ {
				if reach[j-1] {
					next[j], _ = path.Match(segment, name[j-1])
				}
			}
		}
		reach, next = next, reach
	}
	return reach[len(name)]
}

// extractorPaths resolves each extractor's files in path order: literal paths are kept as named
// (a missing one becomes a gap when read) and patterns share one bounded walk of the revision tree
// that descends only into directories some pattern can match below, skipping node_modules.
func extractorPaths(ctx context.Context, auth *gitauth.Repository, revision string, extractors []compiledExtractor, p *extractorProducer) ([][]string, error) {
	paths := make([][]string, len(extractors))
	var globs [][]string
	for _, ex := range extractors {
		for _, pattern := range ex.Files {
			if extractorGlob(pattern) {
				globs = append(globs, strings.Split(pattern, "/"))
			}
		}
	}
	var tree []string
	if len(globs) > 0 {
		var err error
		if tree, err = extractorTree(ctx, auth, revision, globs); err != nil {
			return nil, err
		}
	}
	for i, ex := range extractors {
		set := map[string]bool{}
		for _, pattern := range ex.Files {
			if !extractorGlob(pattern) {
				set[pattern] = true
				continue
			}
			for _, name := range tree {
				if extractorMatch(pattern, name) {
					set[name] = true
				}
			}
		}
		for name := range set {
			paths[i] = append(paths[i], name)
		}
		sort.Strings(paths[i])
		if len(paths[i]) > p.limit {
			return nil, p.bound("matched files", len(paths[i]), ex.ID, "")
		}
	}
	return paths, nil
}

// extractorTree lists the revision's non-tree paths in one verified walk whose Git operations
// grow with directory depth, not directory count (DCX-V1-520), pruning every directory below
// which no glob can match (DCX-V1-521) within the entry and tree-byte bounds (DCX-V1-522).
func extractorTree(ctx context.Context, auth *gitauth.Repository, revision string, globs [][]string) ([]string, error) {
	names, err := auth.RevisionPaths(ctx, revision, affected.MaxWalkEntries, func(dir string) bool {
		if path.Base(dir) == "node_modules" {
			return false
		}
		parts := strings.Split(dir, "/")
		for _, glob := range globs {
			if extractorGlobBelow(glob, parts) {
				return true
			}
		}
		return false
	})
	if err != nil {
		return nil, fail("extraction cannot enumerate the revision: " + err.Error())
	}
	return names, nil
}

// extractorGlobBelow reports whether a path below dir can match glob. Until the glob's first `**`
// each segment matches exactly one path segment, so dir must match those segments and leave the
// file segment below it; after a `**` any directory can hold a match.
func extractorGlobBelow(glob, dir []string) bool {
	for i, part := range dir {
		if glob[i] == "**" {
			return true
		}
		if i == len(glob)-1 {
			return false
		}
		if ok, _ := path.Match(glob[i], part); !ok {
			return false
		}
	}
	return true
}

// extractorRead reads one file as a regular UTF-8 Git blob within the input bound, or names the
// gap that kept it unread (DCX-V1-395).
func extractorRead(ctx context.Context, auth *gitauth.Repository, batched *pathPrefetch, revision, current string) (*extractorFile, string, string, error) {
	entry, ok, err := batched.lookup(ctx, auth, revision, current)
	if err != nil {
		return nil, "", "", err
	}
	if !ok {
		return nil, "missing-file", "the named file does not exist at the revision", nil
	}
	if !regularBlob(entry) {
		return nil, "not-regular-blob", "the file is a " + entry.Type + " with mode " + entry.Mode + ", not a regular Git blob", nil
	}
	data, over, err := batched.blobWithin(ctx, auth, entry.OID, MaxBytes)
	if err != nil {
		return nil, "", "", err
	}
	if over {
		return nil, "file-over-bound", "the file exceeds the " + strconv.Itoa(MaxBytes) + "-byte input bound", nil
	}
	if !utf8.Valid(data) || bytes.IndexByte(data, 0) >= 0 {
		return nil, "not-utf8", "the file is not NUL-free UTF-8 text", nil
	}
	return &extractorFile{path: current, blob: entry.OID, data: data, digest: Digest(data), lines: strings.SplitAfter(string(data), "\n")}, "", "", nil
}

// anchor pins one line of f. It reuses the file's precomputed lines and digest, so anchoring
// costs the line, not the file (the same bytes span() would select).
func (p *extractorProducer) anchor(ex *compiledExtractor, f *extractorFile, line int, reason string) (Anchor, error) {
	if line < 1 || line > len(f.lines) {
		return Anchor{}, fail("anchor span out of range")
	}
	excerpt := []byte(f.lines[line-1])
	if len(excerpt) > maxSpanBytes || len(bytes.TrimSpace(excerpt)) == 0 {
		return Anchor{}, fail("vacuous or oversized anchor")
	}
	kind := "source"
	if ex.ToolOutput != nil {
		kind = "imported"
	}
	return Anchor{Repository: p.repository, Revision: p.revision, Path: f.path, Blob: f.blob, SHA256: f.digest, Start: line, End: line, SpanSHA256: Digest(excerpt), Authority: "external-provider", Kind: kind, Reason: reason}, nil
}

// gap records one gap; a positive line anchors it when that line is not vacuous.
func (p *extractorProducer) gap(ex *compiledExtractor, f *extractorFile, line int, kind, reason string) {
	g := ExtractorGap{Extractor: ex.ID, Kind: kind, Reason: reason}
	if f != nil {
		g.Path = f.path
	}
	if line > 0 {
		g.Line = line
		if a, err := p.anchor(ex, f, line, "extractor "+ex.ID+" "+kind); err == nil {
			g.Evidence = &a
		}
	}
	p.inv.Gaps = append(p.inv.Gaps, g)
}

// capture returns a named group's text, or false when it is absent, empty, over the bound or
// secret-shaped.
func extractorCapture(re *regexp.Regexp, match []string, name string) (string, bool) {
	i := re.SubexpIndex(name)
	if i < 0 || i >= len(match) {
		return "", false
	}
	value := strings.TrimSpace(match[i])
	return value, safeText(value, maxExtractorCapture) && identifier(value)
}

// file applies one extractor to one file line by line (DCX-V1-391, DCX-V1-395). The record bound
// is enforced as each line is scanned, so a dense file is refused without materializing it.
func (p *extractorProducer) file(ex *compiledExtractor, f *extractorFile) error {
	entity, entityOpen := "", false
	for n, raw := range f.lines {
		if err := p.over(ex, f); err != nil {
			return err
		}
		line := n + 1
		text := strings.TrimRight(raw, "\r\n")
		if strings.TrimSpace(text) == "" {
			continue
		}
		if len(raw) > maxSpanBytes {
			if ex.pattern.MatchString(text) {
				entityOpen = false // an unread declaration ends the open entity rather than extending it
			}
			if ex.pattern.MatchString(text) || ex.candidate != nil && ex.candidate.MatchString(text) {
				p.gap(ex, f, 0, "line-over-bound", "line "+strconv.Itoa(line)+" exceeds the anchor span bound")
			}
			continue
		}
		// Two matches are enough to detect ambiguity; never materialize the rest.
		matches := ex.pattern.FindAllStringSubmatch(text, 2)
		if len(matches) > 1 {
			// An ambiguous entity declaration ends the open entity, so later fields are not
			// attached to the previous one (a guess).
			entityOpen = false
			p.gap(ex, f, line, "multiple-matches", "the line matches the pattern more than once; one match per line is admitted")
			continue
		}
		if len(matches) == 1 {
			if ex.Subject == "data_entity" {
				name, ok := extractorCapture(ex.pattern, matches[0], "entity")
				if !ok {
					p.gap(ex, f, line, "invalid-capture", "a captured value is empty, over the bound or secret-shaped")
					entityOpen = false
					continue
				}
				entity, entityOpen = name, ex.field != nil
				p.site(ex, f, line, "data_entity", name, "", "")
				continue
			}
			p.match(ex, f, line, matches[0])
			continue
		}
		if ex.field != nil {
			if entityOpen && ex.end != nil && ex.end.MatchString(text) {
				entityOpen = false
				continue
			}
			if fields := ex.field.FindAllStringSubmatch(text, 2); len(fields) > 0 {
				switch {
				case !entityOpen:
					p.gap(ex, f, line, "field-outside-entity", "a field line matches outside any entity")
				case len(fields) > 1:
					p.gap(ex, f, line, "multiple-matches", "the line matches the field pattern more than once; one match per line is admitted")
				default:
					if field, ok := extractorCapture(ex.field, fields[0], "field"); ok {
						p.site(ex, f, line, "data_entity", entity+"."+field, entity, "")
					} else {
						p.gap(ex, f, line, "invalid-capture", "a captured value is empty, over the bound or secret-shaped")
					}
				}
				continue
			}
		}
		if ex.candidate != nil && ex.candidate.MatchString(text) {
			p.gap(ex, f, line, "unsupported-shape", "the line looks like a declaration but does not match the supported pattern")
		}
	}
	return p.over(ex, f)
}

func (p *extractorProducer) match(ex *compiledExtractor, f *extractorFile, line int, match []string) {
	var name string
	ok := true
	switch ex.Subject {
	case "endpoint":
		method, okMethod := extractorCapture(ex.pattern, match, "method")
		route, okPath := extractorCapture(ex.pattern, match, "path")
		name, ok = strings.ToUpper(method)+" "+route, okMethod && okPath
	case "ui_surface":
		name, ok = extractorCapture(ex.pattern, match, "path")
	default:
		name, ok = extractorCapture(ex.pattern, match, "name")
	}
	handler := ""
	if i := ex.pattern.SubexpIndex("handler"); i >= 0 && strings.TrimSpace(match[i]) != "" {
		var okHandler bool
		handler, okHandler = extractorCapture(ex.pattern, match, "handler")
		ok = ok && okHandler
	}
	if !ok {
		p.gap(ex, f, line, "invalid-capture", "a captured value is empty, over the bound or secret-shaped")
		return
	}
	kind := ex.Subject
	if ex.Role == "produces" || ex.Role == "consumes" {
		kind = "interaction"
	}
	p.site(ex, f, line, kind, name, "", handler)
}

func (p *extractorProducer) site(ex *compiledExtractor, f *extractorFile, line int, kind, name, parent, handler string) {
	a, err := p.anchor(ex, f, line, "extractor "+ex.ID+" matched "+ex.Subject)
	if err != nil {
		p.gap(ex, f, 0, "anchor-bound", "line "+strconv.Itoa(line)+" cannot be anchored: "+err.Error())
		return
	}
	derived := "source-derived"
	if ex.ToolOutput != nil {
		derived = "imported"
	}
	p.sites = append(p.sites, extractorSite{ex: ex, kind: kind, name: name, parent: parent, handler: handler, anchor: a, derived: derived})
}

// extractorSubject accumulates one subject's anchors across its sites.
type extractorSubject struct {
	subject Subject
	sites   []*extractorSite
}

// record turns the matched sites into one corvint-corpus-provider/1 record (DCX-V1-391..393).
func (p *extractorProducer) record(source Repository, version string) (ProviderRecord, error) {
	subjects := map[string]*extractorSubject{}
	byName := map[string][]string{} // kind+"\x00"+name → subject IDs
	add := func(s *extractorSite, kind, name string) *extractorSubject {
		id := p.provider + ":" + kind + ":" + name
		if kind == "interaction" {
			id = p.provider + ":interaction:" + s.ex.ID + ":" + s.anchor.Path + ":" + strconv.Itoa(s.anchor.Start)
			name = s.ex.Role + " " + s.name + " at " + s.anchor.Path + ":" + strconv.Itoa(s.anchor.Start)
		}
		entry, ok := subjects[id]
		if !ok {
			entry = &extractorSubject{subject: Subject{ID: id, Kind: kind, Name: name, Provider: p.provider, Evidence: Evidence{Derivation: s.derived, Trust: "generated", State: "supported", Freshness: "fresh", Anchors: []Anchor{}, Limitations: []string{}}}}
			subjects[id] = entry
			byName[kind+"\x00"+name] = append(byName[kind+"\x00"+name], id)
		}
		entry.sites = append(entry.sites, s)
		ev := &entry.subject.Evidence
		if ev.Derivation != s.derived {
			ev.Derivation = "generated"
		}
		if len(ev.Anchors) < 64 {
			ev.Anchors = append(ev.Anchors, s.anchor)
		} else if len(ev.Limitations) == 0 {
			ev.Limitations = append(ev.Limitations, "more than 64 sites match; only the first 64 are anchored")
			p.inv.Gaps = append(p.inv.Gaps, ExtractorGap{Extractor: s.ex.ID, Path: s.anchor.Path, Line: s.anchor.Start, Kind: "anchor-bound", Reason: "subject " + id + " has more than 64 sites; later sites are not anchored"})
		}
		return entry
	}
	// Declarations first, so produce and consume sites resolve against every declared name.
	for i := range p.sites {
		s := &p.sites[i]
		if s.kind != "interaction" {
			add(s, s.kind, s.name)
		}
	}
	relations := map[string]Relation{}
	// relate records a relation whose site s anchors one end; other is the subject at the other
	// end, whose anchors the relation also carries, so it cites both ends (DCX-V1-392).
	relate := func(from, to, kind string, s *extractorSite, other *extractorSubject) {
		id := p.provider + ":relation:" + Digest([]byte(from + "\x00" + kind + "\x00" + to))[:32]
		if r, ok := relations[id]; ok {
			if len(r.Evidence.Anchors) < 64 && !slices.Contains(r.Evidence.Anchors, s.anchor) {
				r.Evidence.Anchors = append(r.Evidence.Anchors, s.anchor)
				relations[id] = r
			}
			return
		}
		ev := Evidence{Derivation: s.derived, Trust: "generated", State: "supported", Freshness: "fresh", Anchors: []Anchor{s.anchor}, Limitations: []string{}}
		if other.subject.Evidence.Derivation != s.derived {
			ev.Derivation = "generated"
		}
		for _, a := range other.subject.Evidence.Anchors {
			if len(ev.Anchors) == 64 {
				break
			}
			if !slices.Contains(ev.Anchors, a) {
				ev.Anchors = append(ev.Anchors, a)
			}
		}
		relations[id] = Relation{ID: id, From: from, To: to, Type: kind, Provider: p.provider, Evidence: ev}
	}
	for i := range p.sites {
		s := &p.sites[i]
		switch {
		case s.kind == "interaction":
			target := byName[s.ex.Subject+"\x00"+s.name]
			if len(target) == 0 {
				p.inv.Gaps = append(p.inv.Gaps, ExtractorGap{Extractor: s.ex.ID, Path: s.anchor.Path, Line: s.anchor.Start, Kind: "undeclared-reference", Reason: "no declared " + s.ex.Subject + " subject is named " + s.name, Evidence: &s.anchor})
				continue
			}
			site := add(s, "interaction", s.name)
			kind := "calls"
			if s.ex.Role == "consumes" {
				kind = "depends_on"
			}
			relate(site.subject.ID, target[0], kind, s, subjects[target[0]])
		case s.parent != "":
			if entity := byName["data_entity\x00"+s.parent]; len(entity) == 1 {
				relate(p.provider+":data_entity:"+s.name, entity[0], "related_to", s, subjects[entity[0]])
			}
		case s.handler != "":
			handlers := byName["symbol\x00"+s.handler]
			switch {
			case len(handlers) == 0:
				p.inv.Gaps = append(p.inv.Gaps, ExtractorGap{Extractor: s.ex.ID, Path: s.anchor.Path, Line: s.anchor.Start, Kind: "unresolved-handler", Reason: "no symbol subject of this profile is named " + s.handler, Evidence: &s.anchor})
			case len(subjects[handlers[0]].sites) > 1:
				p.inv.Gaps = append(p.inv.Gaps, ExtractorGap{Extractor: s.ex.ID, Path: s.anchor.Path, Line: s.anchor.Start, Kind: "ambiguous-handler", Reason: "symbol " + s.handler + " is defined at more than one site", Evidence: &s.anchor})
			default:
				relate(handlers[0], p.provider+":"+s.kind+":"+s.name, "implements", s, subjects[handlers[0]])
			}
		}
	}
	record := ProviderRecord{Schema: p.schema, ID: p.provider, Version: version, Source: source, Subjects: []Subject{}, Claims: []Claim{}, Relations: []Relation{}, Journeys: []Journey{}, Observations: []ObservationLink{}}
	for _, entry := range subjects {
		if !identifier(entry.subject.ID) || !textOK(entry.subject.Name) {
			return ProviderRecord{}, fail("extracted subject identity exceeds the identifier bound: " + entry.subject.Kind)
		}
		record.Subjects = append(record.Subjects, entry.subject)
	}
	sort.Slice(record.Subjects, func(i, j int) bool { return record.Subjects[i].ID < record.Subjects[j].ID })
	for _, r := range relations {
		record.Relations = append(record.Relations, r)
	}
	sort.Slice(record.Relations, func(i, j int) bool { return record.Relations[i].ID < record.Relations[j].ID })
	if len(record.Subjects) > p.limit {
		return ProviderRecord{}, p.bound("subjects", len(record.Subjects), "", "")
	}
	if len(record.Relations) > p.limit {
		return ProviderRecord{}, p.bound("relations", len(record.Relations), "", "")
	}
	subjectsState, subjectsReason := "present", "subjects matched by the repository-declared extractor profile"
	if p.callerProfile != "" {
		subjectsReason = "subjects matched by the " + p.callerProfile
	}
	if len(record.Subjects) == 0 {
		subjectsState, subjectsReason = "absent", "no extractor pattern matched a readable file"
		if p.callerProfile != "" {
			subjectsReason = "no pattern of the " + p.callerProfile + " matched a readable file"
		}
	}
	record.Capabilities = []CapabilityDeclaration{
		{Name: "subjects", State: subjectsState, Reason: subjectsReason},
		{Name: "relations", State: "present", Reason: "implements, calls, depends_on and related_to relations derived from matched sites; unresolved ones are inventory gaps"},
		{Name: "claims", State: "unsupported", Reason: "extractor profiles emit subjects and relations only"},
		{Name: "journeys", State: "unsupported", Reason: "extractor profiles emit subjects and relations only"},
		{Name: "observations", State: "unsupported", Reason: "extractor profiles run nothing"},
		{Name: "gaps", State: "not-collected", Reason: "extractor gaps are reported in the " + ExtractorInventorySchema + " gaps member"},
	}
	return record, nil
}
