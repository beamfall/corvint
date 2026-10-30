package workflow

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/Beamfall/corvint/internal/cem/cemcode"
	"github.com/Beamfall/corvint/internal/cem/gitauth"
	"github.com/Beamfall/corvint/internal/cem/gitrun"
	"github.com/Beamfall/corvint/internal/cem/publish"
	"github.com/Beamfall/corvint/internal/cem/verify"
	"github.com/Beamfall/corvint/internal/cem/wire"
)

// A recorded intent-scope deferral is a tracked, content-addressed file the
// change itself creates under DeferredIntentsDir. It is not a CEM wire member:
// the frozen patchSha256 and hunk list already bind every target-tree file
// except the CEM, so the seal binds the record and the PR diff, the hunk list,
// `cem status` and `cem report` show it (CEM-V1-643..646, V1-2809;
// experimental).
const (
	DeferredIntentsDir     = ".corvint/deferred-intents"
	DeferredIntentsProfile = "cem-deferred-intents/0"
	// MaxDeferredIntents bounds the entries of one record.
	MaxDeferredIntents = 4
	// MaxDeferredIntentReasonBytes bounds one entry's one-line reason.
	MaxDeferredIntentReasonBytes = 200
	// MaxDeferredIntentObligations bounds one entry's obligation count.
	MaxDeferredIntentObligations = 65536
	// MaxDeferredIntentsRecordBytes bounds one record file.
	MaxDeferredIntentsRecordBytes = 4096
	// maxDeferredIntentBytes bounds the intent document the counter reads.
	maxDeferredIntentBytes = 16 << 20
)

// DeferredIntent is one recorded deferral. Its fields are declared in the
// canonical (alphabetical) key order the encoding uses. BaseBlobOid is nil
// when the intent is absent at the map's base revision.
type DeferredIntent struct {
	BaseBlobOid     *string `json:"baseBlobOid"`
	ObligationCount int     `json:"obligationCount"`
	Path            string  `json:"path"`
	Reason          string  `json:"reason"`
}

type deferredIntentsRecord struct {
	DeferredIntents []DeferredIntent `json:"deferredIntents"`
	Profile         string           `json:"profile"`
}

// IntentObligationCounter returns the obligation count of one intent document
// or an error when the bytes are not an intent. The binary installs it (see
// SetIntentObligationCounter) so the CEM seams keep their dependency closure.
type IntentObligationCounter func(data []byte) (int, error)

// SetIntentObligationCounter installs the counter `cem defer` requires and
// `cem status`/`cem report` use to check a recorded count; without it the
// check reports each count NOT_CHECKED.
func (s *Session) SetIntentObligationCounter(counter IntentObligationCounter) {
	s.intentCounter = counter
}

// ValidateDeferredIntents applies the CEM-V1-643 entry rules.
func ValidateDeferredIntents(entries []DeferredIntent) error {
	if len(entries) == 0 || len(entries) > MaxDeferredIntents {
		return fmt.Errorf("a deferral record names 1..%d intents", MaxDeferredIntents)
	}
	for index, entry := range entries {
		if err := validateDeferredIntentPath(entry.Path); err != nil {
			return err
		}
		if index > 0 && entries[index-1].Path >= entry.Path {
			return errors.New("deferred intents must be strictly sorted by path")
		}
		if entry.BaseBlobOid != nil && !wire.IsGitOid(*entry.BaseBlobOid) {
			return fmt.Errorf("deferred intent %q baseBlobOid is not a full Git object ID", entry.Path)
		}
		if entry.ObligationCount < 1 || entry.ObligationCount > MaxDeferredIntentObligations {
			return fmt.Errorf("deferred intent %q obligationCount must be 1..%d", entry.Path, MaxDeferredIntentObligations)
		}
		if err := validateDeferredIntentReason(entry.Reason); err != nil {
			return fmt.Errorf("deferred intent %q %w", entry.Path, err)
		}
	}
	return nil
}

func validateDeferredIntentPath(path string) error {
	if err := wire.ValidatePath(path); err != nil {
		return fmt.Errorf("deferred intent path %q: %v", path, err)
	}
	if path == wire.ExcludedCEMPath || strings.HasPrefix(path, DeferredIntentsDir+"/") {
		return fmt.Errorf("deferred intent path %q names a CEM or deferral record", path)
	}
	return nil
}

func validateDeferredIntentReason(reason string) error {
	if reason == "" || len(reason) > MaxDeferredIntentReasonBytes || !utf8.ValidString(reason) {
		return fmt.Errorf("reason must be 1..%d bytes of UTF-8", MaxDeferredIntentReasonBytes)
	}
	for _, character := range reason {
		if unicode.IsControl(character) || character == ' ' || character == ' ' {
			return errors.New("reason must be one line without control characters")
		}
	}
	return nil
}

// EncodeDeferredIntents returns the canonical record bytes: compact JSON with
// alphabetical keys, no HTML escaping and one trailing newline.
func EncodeDeferredIntents(entries []DeferredIntent) ([]byte, error) {
	if err := ValidateDeferredIntents(entries); err != nil {
		return nil, err
	}
	var out bytes.Buffer
	encoder := json.NewEncoder(&out)
	encoder.SetEscapeHTML(false)
	if err := encoder.Encode(deferredIntentsRecord{DeferredIntents: entries, Profile: DeferredIntentsProfile}); err != nil {
		return nil, err
	}
	if out.Len() > MaxDeferredIntentsRecordBytes {
		return nil, fmt.Errorf("deferral record exceeds %d bytes", MaxDeferredIntentsRecordBytes)
	}
	return out.Bytes(), nil
}

// ParseDeferredIntents admits only canonical record bytes: the profile, the
// entry rules and byte equality with the re-encoding.
func ParseDeferredIntents(data []byte) ([]DeferredIntent, error) {
	if len(data) > MaxDeferredIntentsRecordBytes {
		return nil, fmt.Errorf("deferral record exceeds %d bytes", MaxDeferredIntentsRecordBytes)
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	var record deferredIntentsRecord
	if err := decoder.Decode(&record); err != nil {
		return nil, fmt.Errorf("deferral record is not JSON of profile %s: %v", DeferredIntentsProfile, err)
	}
	if record.Profile != DeferredIntentsProfile {
		return nil, fmt.Errorf("deferral record profile must be %s", DeferredIntentsProfile)
	}
	canonical, err := EncodeDeferredIntents(record.DeferredIntents)
	if err != nil {
		return nil, err
	}
	if !bytes.Equal(canonical, data) {
		return nil, errors.New("deferral record is not in canonical encoding")
	}
	return record.DeferredIntents, nil
}

// DeferredRecordPath returns the content-addressed repository path of record
// bytes: DeferredIntentsDir/<sha256 hex of the bytes>.json.
func DeferredRecordPath(data []byte) string {
	sum := sha256.Sum256(data)
	return DeferredIntentsDir + "/" + hex.EncodeToString(sum[:]) + ".json"
}

// DeferOptions are the inputs of `cem defer`.
type DeferOptions struct {
	MapPath string
	Intent  string
	Reason  string
}

// Defer records one intent-scope deferral for the change the map describes
// (CEM-V1-644). It reads the map's base revision, the intent's blob at that
// base (absent is recorded as null) and the intent's worktree bytes for the
// installed counter, merges the entry into the change's one created record
// (replacing an entry for the same path), publishes the new content-addressed
// record and removes the record it replaced. It never edits the map.
func (s *Session) Defer(ctx context.Context, options DeferOptions) (map[string]any, error) {
	if err := validateDeferredIntentPath(options.Intent); err != nil {
		return nil, invalidArguments("--intent: %v", err)
	}
	if err := validateDeferredIntentReason(options.Reason); err != nil {
		return nil, invalidArguments("--reason: %v", err)
	}
	raw, err := s.readMapBytes(options.MapPath)
	if err != nil {
		return nil, err
	}
	document, err := wire.ParseStable(raw)
	if err != nil {
		return nil, invalidArguments("cem defer reads only a %s map: %v", wire.StableSpec, err)
	}
	base := document.Change.BaseRevision
	if s.intentCounter == nil {
		return nil, invalidArguments("cem defer needs the intent obligation counter, which this host did not install")
	}
	if err := s.openRepository(); err != nil {
		return nil, err
	}
	var baseBlob *string
	entry, present, err := s.repository.LookupTreeEntry(ctx, base, options.Intent)
	if err != nil {
		return nil, err
	}
	if present {
		if entry.Type != "blob" {
			return nil, invalidArguments("--intent %q is not a file at the map's base revision", options.Intent)
		}
		oid := entry.OID
		baseBlob = &oid
	}
	data, err := s.workRoot.ReadBounded(options.Intent, maxDeferredIntentBytes, cemcode.InvalidArguments)
	if err != nil {
		return nil, err
	}
	count, err := s.intentCounter(data)
	if err != nil {
		return nil, invalidArguments("--intent %q is not an intent document: %v", options.Intent, err)
	}
	if count < 1 || count > MaxDeferredIntentObligations {
		return nil, invalidArguments("--intent %q has %d obligations; a deferral names 1..%d", options.Intent, count, MaxDeferredIntentObligations)
	}
	previous, entries, err := s.createdDeferralRecord(ctx, base)
	if err != nil {
		return nil, err
	}
	next := DeferredIntent{BaseBlobOid: baseBlob, ObligationCount: count, Path: options.Intent, Reason: options.Reason}
	if index := slices.IndexFunc(entries, func(e DeferredIntent) bool { return e.Path == next.Path }); index >= 0 {
		entries[index] = next
	} else {
		entries = append(entries, next)
	}
	slices.SortFunc(entries, func(a, b DeferredIntent) int { return strings.Compare(a.Path, b.Path) })
	if len(entries) > MaxDeferredIntents {
		return nil, invalidArguments("a change records at most %d deferred intents", MaxDeferredIntents)
	}
	encoded, err := EncodeDeferredIntents(entries)
	if err != nil {
		return nil, invalidArguments("%v", err)
	}
	record := DeferredRecordPath(encoded)
	if err := publish.Publish(publish.Output{Root: s.workRoot, Relative: record, Data: encoded}); err != nil {
		return nil, err
	}
	if previous != "" && previous != record {
		if err := os.Remove(filepath.Join(s.workRoot.Path(), filepath.FromSlash(previous))); err != nil {
			return nil, cemcode.New(cemcode.PublishFailed, "replaced deferral record %q could not be removed", previous)
		}
	}
	return map[string]any{
		"ok": true, "mutates": true, "tool": "cem-defer", "profile": DeferredIntentsProfile,
		"record": record, "deferredIntents": deferredIntentRows(entries), "count": len(entries),
	}, nil
}

// createdDeferralRecord returns the worktree's one record absent at base, with
// its entries; none returns "". A second created record or an invalid one is
// refused rather than guessed between.
func (s *Session) createdDeferralRecord(ctx context.Context, base string) (string, []DeferredIntent, error) {
	listing, err := os.ReadDir(filepath.Join(s.workRoot.Path(), filepath.FromSlash(DeferredIntentsDir)))
	if errors.Is(err, os.ErrNotExist) {
		return "", nil, nil
	}
	if err != nil {
		return "", nil, invalidArguments("%s is not a readable directory", DeferredIntentsDir)
	}
	found, entries := "", []DeferredIntent(nil)
	for _, item := range listing {
		if !strings.HasSuffix(item.Name(), ".json") {
			continue
		}
		path := DeferredIntentsDir + "/" + item.Name()
		if _, atBase, err := s.repository.LookupTreeEntry(ctx, base, path); err != nil {
			return "", nil, err
		} else if atBase {
			continue
		}
		if found != "" {
			return "", nil, invalidArguments("the worktree has more than one created deferral record: %q and %q", found, path)
		}
		data, err := s.workRoot.ReadBounded(path, MaxDeferredIntentsRecordBytes, cemcode.InvalidArguments)
		if err != nil {
			return "", nil, err
		}
		parsed, err := ParseDeferredIntents(data)
		if err != nil || DeferredRecordPath(data) != path {
			return "", nil, invalidArguments("created deferral record %q is invalid or misnamed; remove it and record again", path)
		}
		found, entries = path, parsed
	}
	return found, entries, nil
}

func deferredIntentRows(entries []DeferredIntent) []any {
	rows := make([]any, 0, len(entries))
	for _, entry := range entries {
		var oid any
		if entry.BaseBlobOid != nil {
			oid = *entry.BaseBlobOid
		}
		rows = append(rows, map[string]any{
			"path": entry.Path, "baseBlobOid": oid,
			"obligationCount": entry.ObligationCount, "reason": entry.Reason,
		})
	}
	return rows
}

// Deferred-intents issue codes reported beside the verifier result (CEM-V1-645).
const (
	DeferredIssueRecordNotCreated      = "deferred-intents-record-not-created"
	DeferredIssueRecordUnsupportedKind = "deferred-intents-record-unsupported-kind"
	DeferredIssueRecordNameMismatch    = "deferred-intents-record-name-mismatch"
	DeferredIssueRecordInvalid         = "deferred-intents-record-invalid"
	DeferredIssueMultipleRecords       = "deferred-intents-multiple-records"
	DeferredIssueUnreadable            = "deferred-intents-unreadable"
	DeferredIssueBaseMismatch          = "deferred-intent-base-mismatch"
	DeferredIssueAbsentAtTarget        = "deferred-intent-absent-at-target"
	DeferredIssueNotAnIntent           = "deferred-intent-not-an-intent"
	DeferredIssueCountMismatch         = "deferred-intent-count-mismatch"
	DeferredIssueExceedBound           = "deferred-intents-exceed-bound"
)

// deferredIntents checks the change's recorded deferral beside, never inside,
// the verifier result (CEM-V1-645). It returns nil when the map's hunks name
// no record and no bound was supplied. The record set is the map's hunk paths
// under DeferredIntentsDir; a record the change removes is not a deferral.
// Only an accepted result is checked: the target is then a verified commit
// whose diff from base the seal binds.
func (s *Session) deferredIntents(ctx context.Context, repository string, raw []byte, result verify.StableResult, limit *int) map[string]any {
	document, err := wire.ParseStable(raw)
	if err != nil {
		return nil
	}
	var records []string
	for _, hunk := range document.Change.Hunks {
		if strings.HasPrefix(hunk.Path, DeferredIntentsDir+"/") && !slices.Contains(records, hunk.Path) {
			records = append(records, hunk.Path)
		}
	}
	if len(records) == 0 && limit == nil {
		return nil
	}
	slices.Sort(records)
	bound := 0
	if limit != nil {
		bound = *limit
	}
	member := map[string]any{
		"profile": DeferredIntentsProfile, "record": nil, "count": 0, "max": bound,
		"entries": []any{}, "issues": []any{},
	}
	if result.Outcome != "ACCEPT" || result.TargetRevision == nil {
		member["state"] = "not-checked"
		return member
	}
	issues := []any{}
	issue := func(code, path string) { issues = append(issues, map[string]any{"code": code, "path": path}) }
	finish := func() map[string]any {
		if count := member["count"].(int); count > bound {
			issue(DeferredIssueExceedBound, "")
		}
		member["issues"] = issues
		member["state"] = "within-bound"
		if len(issues) != 0 {
			member["state"] = "refused"
		}
		return member
	}
	repo, err := gitauth.Open(repository, gitrun.NewDefaultBudget())
	if err != nil {
		issue(DeferredIssueUnreadable, "")
		return finish()
	}
	base, target := document.Change.BaseRevision, *result.TargetRevision
	type created struct {
		path    string
		entries []DeferredIntent
	}
	var valid []created
	for _, path := range records {
		_, baseFound, err := repo.LookupTreeEntry(ctx, base, path)
		if err != nil {
			issue(DeferredIssueUnreadable, path)
			continue
		}
		atTarget, targetFound, err := repo.LookupTreeEntry(ctx, target, path)
		switch {
		case err != nil:
			issue(DeferredIssueUnreadable, path)
			continue
		case !targetFound:
			continue // a removed record defers nothing
		case baseFound:
			issue(DeferredIssueRecordNotCreated, path)
			continue
		case atTarget.Type != "blob" || atTarget.Mode != "100644":
			issue(DeferredIssueRecordUnsupportedKind, path)
			continue
		}
		data, over, err := repo.BlobBytesWithin(ctx, atTarget.OID, MaxDeferredIntentsRecordBytes)
		switch {
		case err != nil:
			issue(DeferredIssueUnreadable, path)
			continue
		case over:
			issue(DeferredIssueRecordInvalid, path)
			continue
		case DeferredRecordPath(data) != path:
			issue(DeferredIssueRecordNameMismatch, path)
			continue
		}
		entries, err := ParseDeferredIntents(data)
		if err != nil {
			issue(DeferredIssueRecordInvalid, path)
			continue
		}
		valid = append(valid, created{path, entries})
	}
	if len(valid) > 1 {
		for _, record := range valid {
			issue(DeferredIssueMultipleRecords, record.path)
		}
		return finish()
	}
	if len(valid) == 0 {
		return finish()
	}
	member["record"], member["count"] = valid[0].path, len(valid[0].entries)
	rows := make([]any, 0, len(valid[0].entries))
	for _, entry := range valid[0].entries {
		rows = append(rows, s.checkDeferredIntent(ctx, repo, base, target, entry, issue))
	}
	member["entries"] = rows
	return finish()
}

// checkDeferredIntent checks one entry's base blob, its presence at target and,
// with the counter installed, its count against the target blob.
func (s *Session) checkDeferredIntent(ctx context.Context, repo *gitauth.Repository, base, target string, entry DeferredIntent, issue func(code, path string)) map[string]any {
	row := deferredIntentRows([]DeferredIntent{entry})[0].(map[string]any)
	row["baseState"], row["targetBlobOid"], row["countState"] = "MATCHED", nil, "NOT_CHECKED"
	atBase, baseFound, err := repo.LookupTreeEntry(ctx, base, entry.Path)
	switch {
	case err != nil:
		issue(DeferredIssueUnreadable, entry.Path)
		row["baseState"] = "NOT_CHECKED"
	case baseFound != (entry.BaseBlobOid != nil),
		baseFound && (atBase.Type != "blob" || atBase.OID != *entry.BaseBlobOid):
		issue(DeferredIssueBaseMismatch, entry.Path)
		row["baseState"] = "MISMATCH"
	}
	atTarget, targetFound, err := repo.LookupTreeEntry(ctx, target, entry.Path)
	switch {
	case err != nil:
		issue(DeferredIssueUnreadable, entry.Path)
		return row
	case !targetFound || atTarget.Type != "blob":
		issue(DeferredIssueAbsentAtTarget, entry.Path)
		return row
	}
	row["targetBlobOid"] = atTarget.OID
	if s.intentCounter == nil {
		return row
	}
	data, over, err := repo.BlobBytesWithin(ctx, atTarget.OID, maxDeferredIntentBytes)
	if err != nil || over {
		issue(DeferredIssueUnreadable, entry.Path)
		return row
	}
	count, err := s.intentCounter(data)
	switch {
	case err != nil:
		issue(DeferredIssueNotAnIntent, entry.Path)
		row["countState"] = "MISMATCH"
	case count != entry.ObligationCount:
		issue(DeferredIssueCountMismatch, entry.Path)
		row["countState"], row["observedCount"] = "MISMATCH", count
	default:
		row["countState"] = "VERIFIED"
	}
	return row
}
