package lrfrepo

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/Beamfall/corvint/internal/cem/cemcode"
	"github.com/Beamfall/corvint/internal/cem/gitauth"
	"github.com/Beamfall/corvint/internal/cem/publish"
	"github.com/Beamfall/corvint/internal/cem/verify"
	"github.com/Beamfall/corvint/internal/cem/wire"
	"github.com/Beamfall/corvint/internal/lrf"
	"github.com/Beamfall/corvint/internal/pythonsyntax"
)

const (
	ocmSpec         = "ocm/0.1-experimental"
	claimExtractor  = "corvint-test-claim/1"
	maxClaims       = 512
	maxObligations  = 256
	maxReferences   = 64
	maxSelector     = 1024
	maxOCMBlob      = 4 << 20
	maxOCMBlobTotal = 16 << 20
)

var (
	requirementID = regexp.MustCompile(`^[A-Z][A-Z0-9-]{2,31}-[0-9]{3}$`)
	claimID       = regexp.MustCompile(`^claim:sha256:[0-9a-f]{64}$`)
	hunkID        = regexp.MustCompile(`^hunk:sha256:[0-9a-f]{64}$`)
	unknownReason = map[string]bool{
		"unassessed": true, "no-test-claim": true,
		"insufficient-evidence": true, "conflicting-evidence": true,
	}
)

type ocmSpan struct {
	start int64
	end   int64
}

type ocmIntent struct {
	path       string
	blobOID    string
	start      int64
	end        int64
	spanSHA256 string
	form       string
}

type ocmClaim struct {
	id         string
	extractor  string
	path       string
	blobOID    string
	selector   string
	span       ocmSpan
	spanSHA256 string
}

type ocmObligation struct {
	id          string
	disposition string
	reason      string
	hunkIDs     []string
	claimIDs    []string
}

type ocmDocument struct {
	spec        string
	target      string
	intent      ocmIntent
	cemMap      string
	cemPatch    string
	claims      []ocmClaim
	obligations []ocmObligation
}

type verifiedOCM struct {
	spec        string
	mapSHA256   string
	intent      ocmIntent
	obligations []lrf.Obligation
}

type ocmIntentContext struct {
	reader       *ocmBlobReader
	requirements []string
	scope        []byte
	statements   map[string][]byte
}

// verifyOptionalOCM is the `lrf` command's OCM leg. It rejects Python claims for
// the same reason the read slice does: no exact `python-ast/1` grammar exists in
// Go, and GPK-V0-014 admits only that grammar or an abstention -- never the Go
// host's own idea of Python syntax (GPK-V0-041, DR-0010).
func verifyOptionalOCM(ctx context.Context, root *publish.Root, repository *gitauth.Repository, cem *wire.Map, cemRaw []byte, verified *verifiedCEM, options Options) (*verifiedOCM, error) {
	if options.OCMPath == "" {
		return nil, nil
	}
	raw, err := readRootInput(root, options.OCMPath, maxOCMBytes, "map-unavailable")
	if err != nil {
		return nil, err
	}
	document, err := parseOCM(raw)
	if err != nil {
		return nil, err
	}
	if err := refuseDeclaredIntentForm(document); err != nil {
		return nil, err
	}
	return verifyOCM(ctx, repository, cem, cemRaw, verified, document, raw, options, true)
}

func parseOCM(raw []byte) (*ocmDocument, error) {
	if len(raw) > maxOCMBytes {
		return nil, fail("map-too-large", "OCM exceeds its size limit")
	}
	root, err := wire.Parse(raw)
	if err != nil {
		return nil, err
	}
	if !bytes.Equal(raw, append(canonicalValue(root), '\n')) {
		return nil, fail("noncanonical-map", "OCM bytes are not canonical")
	}
	return parseOCMDocument(root)
}

func parseOCMDocument(root wire.Value) (*ocmDocument, error) {
	document, err := parseOCMHeader(root)
	if err != nil {
		return nil, err
	}
	if err := parseOCMIntent(root, document); err != nil {
		return nil, err
	}
	if err := parseOCMClosure(root, document); err != nil {
		return nil, err
	}
	return document, nil
}

func parseOCMHeader(root wire.Value) (*ocmDocument, error) {
	object, err := exactObject(root, []string{"spec", "targetRevision", "intentScope", "cem", "claims", "obligations"}, "OCM")
	if err != nil {
		return nil, err
	}
	spec, err := stringMember(object, "spec")
	if err != nil || spec != ocmSpec {
		return nil, fail("unsupported-spec", "OCM profile is unsupported")
	}
	target, err := stringMember(object, "targetRevision")
	if err != nil || !wire.IsGitOid(target) {
		return nil, fail("invalid-target-revision", "target must be a full commit OID")
	}
	cemValue, _ := object.Get("cem")
	cemObject, err := exactObject(cemValue, []string{"mapSha256", "patchSha256"}, "cem")
	if err != nil {
		return nil, err
	}
	cemMap, mapErr := stringMember(cemObject, "mapSha256")
	cemPatch, patchErr := stringMember(cemObject, "patchSha256")
	if mapErr != nil || patchErr != nil || !wire.IsSha256(cemMap) || !wire.IsSha256(cemPatch) {
		return nil, fail("invalid-cem-digest", "CEM binding digest is invalid")
	}
	return &ocmDocument{spec: spec, target: target, cemMap: cemMap, cemPatch: cemPatch}, nil
}

func parseOCMIntent(root wire.Value, document *ocmDocument) error {
	intentValue, _ := root.Obj.Get("intentScope")
	intent, err := parseIntent(intentValue)
	if err != nil {
		return err
	}
	document.intent = intent
	return nil
}

func parseOCMClosure(root wire.Value, document *ocmDocument) error {
	claimsValue, _ := root.Obj.Get("claims")
	claims, err := parseClaims(claimsValue)
	if err != nil {
		return err
	}
	obligationsValue, _ := root.Obj.Get("obligations")
	obligations, err := parseObligations(obligationsValue, document.intent.form)
	if err != nil {
		return err
	}
	document.claims = claims
	document.obligations = obligations
	return nil
}

func parseIntent(value wire.Value) (ocmIntent, error) {
	fields := []string{"path", "blobOid", "span", "spanSha256"}
	if value.Kind == wire.KindObject && declaresIntentForm(value.Obj) {
		fields = append(fields, "form")
	}
	object, err := exactObject(value, fields, "intentScope")
	if err != nil {
		return ocmIntent{}, err
	}
	form, err := parseIntentForm(object)
	if err != nil {
		return ocmIntent{}, err
	}
	path, pathErr := stringMember(object, "path")
	blob, blobErr := stringMember(object, "blobOid")
	digest, digestErr := stringMember(object, "spanSha256")
	spanValue, _ := object.Get("span")
	span, spanErr := parseOCMSpan(spanValue, "intentScope.span")
	if pathErr != nil || !validOCMPath(path) {
		return ocmIntent{}, fail("invalid-path", "path is invalid")
	}
	if blobErr != nil || !wire.IsGitOid(blob) {
		return ocmIntent{}, fail("invalid-intent-blob", "intent blob OID is invalid")
	}
	if spanErr != nil {
		return ocmIntent{}, spanErr
	}
	if digestErr != nil || !wire.IsSha256(digest) {
		return ocmIntent{}, fail("invalid-span-digest", "intent span digest is invalid")
	}
	return ocmIntent{path: path, blobOID: blob, start: span.start, end: span.end, spanSHA256: digest, form: form}, nil
}

func parseClaims(value wire.Value) ([]ocmClaim, error) {
	if value.Kind != wire.KindArray || len(value.Arr) > maxClaims {
		return nil, fail("invalid-claims", "claims must be a bounded array")
	}
	claims := make([]ocmClaim, 0, len(value.Arr))
	previous := ""
	for index, item := range value.Arr {
		object, err := exactObject(item, []string{"id", "extractor", "path", "blobOid", "selector", "span", "spanSha256"}, "claim["+strconv.Itoa(index)+"]")
		if err != nil {
			return nil, err
		}
		id, idErr := stringMember(object, "id")
		extractor, extractorErr := stringMember(object, "extractor")
		path, pathErr := stringMember(object, "path")
		blob, blobErr := stringMember(object, "blobOid")
		selector, selectorErr := stringMember(object, "selector")
		digest, digestErr := stringMember(object, "spanSha256")
		spanValue, _ := object.Get("span")
		span, spanErr := parseOCMSpan(spanValue, "claim.span")
		switch {
		case idErr != nil || !claimID.MatchString(id):
			return nil, fail("fabricated-claim-id", "claim ID is not content-derived")
		case id <= previous:
			return nil, fail("duplicate-or-unsorted-claims", "claims must be sorted and unique")
		case extractorErr != nil || extractor != claimExtractor:
			return nil, fail("invalid-claim-extractor", "claim extractor is unsupported")
		case pathErr != nil || !validOCMPath(path):
			return nil, fail("invalid-path", "path is invalid")
		case blobErr != nil || !wire.IsGitOid(blob):
			return nil, fail("invalid-claim-blob", "claim blob OID is invalid")
		case selectorErr != nil || selector == "" || len([]byte(selector)) > maxSelector || strings.IndexFunc(selector, controlRune) >= 0:
			return nil, fail("invalid-selector", "claim selector is invalid")
		case spanErr != nil:
			return nil, spanErr
		case digestErr != nil || !wire.IsSha256(digest):
			return nil, fail("invalid-span-digest", "claim span digest is invalid")
		}
		claim := ocmClaim{id: id, extractor: extractor, path: path, blobOID: blob, selector: selector, span: span, spanSHA256: digest}
		if id != claimIdentity(claim) {
			return nil, fail("fabricated-claim-id", "claim ID is not content-derived")
		}
		claims = append(claims, claim)
		previous = id
	}
	return claims, nil
}

func parseObligations(value wire.Value, form string) ([]ocmObligation, error) {
	if value.Kind != wire.KindArray || len(value.Arr) > maxObligations {
		return nil, fail("invalid-obligations", "obligations must be a bounded array")
	}
	result := make([]ocmObligation, 0, len(value.Arr))
	seen := map[string]bool{}
	for index, item := range value.Arr {
		object, err := exactObject(item, []string{"id", "disposition", "reason", "hunkIds", "claimIds"}, "obligation["+strconv.Itoa(index)+"]")
		if err != nil {
			return nil, err
		}
		id, idErr := stringMember(object, "id")
		disposition, dispositionErr := stringMember(object, "disposition")
		reason, reasonErr := stringMember(object, "reason")
		hunksValue, _ := object.Get("hunkIds")
		claimsValue, _ := object.Get("claimIds")
		hunks, hunksErr := idArray(hunksValue, hunkID, "hunkIds")
		claims, claimsErr := idArray(claimsValue, claimID, "claimIds")
		if idErr != nil || !validObligationID(form, id) {
			return nil, fail("invalid-obligation-id", "obligation ID is invalid")
		}
		if seen[id] {
			return nil, fail("duplicate-obligation", "obligation IDs must be unique")
		}
		if dispositionErr != nil || reasonErr != nil || hunksErr != nil || claimsErr != nil {
			return nil, firstError(dispositionErr, reasonErr, hunksErr, claimsErr)
		}
		seen[id] = true
		result = append(result, ocmObligation{id: id, disposition: disposition, reason: reason, hunkIDs: hunks, claimIDs: claims})
	}
	return result, nil
}

func parseOCMSpan(value wire.Value, subject string) (ocmSpan, error) {
	object, err := exactObject(value, []string{"start", "end"}, subject)
	if err != nil {
		return ocmSpan{}, err
	}
	start, startOK := object.Get("start")
	end, endOK := object.Get("end")
	if !startOK || !endOK || start.Kind != wire.KindInt || end.Kind != wire.KindInt {
		return ocmSpan{}, fail("invalid-integer", "%s must contain non-negative integers", subject)
	}
	if end.Int <= start.Int {
		return ocmSpan{}, fail("invalid-span", "%s must be non-empty", subject)
	}
	return ocmSpan{start: start.Int, end: end.Int}, nil
}

func exactObject(value wire.Value, fields []string, subject string) (*wire.Object, error) {
	if value.Kind != wire.KindObject {
		return nil, fail("invalid-object", "%s must be an object", subject)
	}
	expected := make(map[string]bool, len(fields))
	for _, field := range fields {
		expected[field] = true
		if _, found := value.Obj.Get(field); !found {
			return nil, fail("missing-field", "%s is missing a required field", subject)
		}
	}
	for _, key := range value.Obj.Keys {
		if !expected[key] {
			return nil, fail("unknown-field", "%s has an unknown field", subject)
		}
	}
	return value.Obj, nil
}

func stringMember(object *wire.Object, name string) (string, error) {
	value, found := object.Get(name)
	if !found || value.Kind != wire.KindString {
		return "", fail("invalid-field", "%s must be a string", name)
	}
	return value.Str, nil
}

func idArray(value wire.Value, pattern *regexp.Regexp, subject string) ([]string, error) {
	if value.Kind != wire.KindArray || len(value.Arr) > maxReferences {
		return nil, fail("invalid-reference-array", "%s must be a bounded array", subject)
	}
	result := make([]string, 0, len(value.Arr))
	previous := ""
	for _, item := range value.Arr {
		if item.Kind != wire.KindString || !pattern.MatchString(item.Str) {
			return nil, fail("invalid-reference-id", "%s contains an invalid ID", subject)
		}
		if item.Str <= previous {
			return nil, fail("duplicate-or-unsorted-reference", "%s must be sorted and unique", subject)
		}
		result = append(result, item.Str)
		previous = item.Str
	}
	return result, nil
}

// canonicalValue delegates to the single shared canonical-JSON implementation;
// a local copy would fork every content address derived from it.
func canonicalValue(value wire.Value) []byte {
	return wire.CanonicalValue(value)
}

func claimIdentity(claim ocmClaim) string {
	var body strings.Builder
	body.WriteString(`{"blobOid":` + wire.CanonicalString(claim.blobOID))
	body.WriteString(`,"extractor":` + wire.CanonicalString(claim.extractor))
	body.WriteString(`,"path":` + wire.CanonicalString(claim.path))
	body.WriteString(`,"selector":` + wire.CanonicalString(claim.selector))
	body.WriteString(`,"span":{"end":` + strconv.FormatInt(claim.span.end, 10))
	body.WriteString(`,"start":` + strconv.FormatInt(claim.span.start, 10) + `}`)
	body.WriteString(`,"spanSha256":` + wire.CanonicalString(claim.spanSHA256) + `}`)
	digest := sha256.Sum256([]byte(body.String()))
	return "claim:sha256:" + hex.EncodeToString(digest[:])
}

func validOCMPath(path string) bool { return wire.ValidatePath(path) == nil }

func controlRune(value rune) bool { return value < 0x20 || value == 0x7f }

func firstError(errors ...error) error {
	for _, err := range errors {
		if err != nil {
			return err
		}
	}
	return nil
}

func verifyOCM(ctx context.Context, repository *gitauth.Repository, cem *wire.Map, cemRaw []byte, verified *verifiedCEM, document *ocmDocument, raw []byte, options Options, rejectPythonClaims bool) (*verifiedOCM, error) {
	if err := verifyOCMBinding(ctx, repository, cem, cemRaw, verified, document, options); err != nil {
		return nil, err
	}
	intentContext, err := verifyOCMIntent(ctx, repository, cem, document)
	if err != nil {
		return nil, err
	}
	return verifyOCMClosure(cem, document, raw, intentContext, rejectPythonClaims)
}

func verifyOCMBinding(ctx context.Context, repository *gitauth.Repository, cem *wire.Map, cemRaw []byte, verified *verifiedCEM, document *ocmDocument, options Options) error {
	if sha256Hex(cemRaw) != document.cemMap {
		return fail("cem-map-digest-mismatch", "CEM map digest does not match")
	}
	resolvedTarget, err := repository.Resolve(ctx, document.target)
	if gitBoundReached(err) {
		return err
	}
	if err != nil || resolvedTarget != document.target {
		return fail("repository-object-unavailable", "repository object unavailable")
	}
	if options.Target != "" {
		callerTarget, err := repository.Resolve(ctx, options.Target)
		if err != nil {
			return err
		}
		if callerTarget != document.target {
			return fail("target-mismatch", "caller target differs from OCM target")
		}
	}
	if cem.Spec == wire.Spec01 {
		if err := verifyLegacyOCMBinding(ctx, repository, cem, cemRaw, verified, document.target, options.ExpectedBase); err != nil {
			return err
		}
	} else if verified.target != document.target {
		return fail("target-mismatch", "caller target differs from OCM target")
	}
	if sha256Hex(verified.patch) != document.cemPatch {
		return fail("cem-patch-digest-mismatch", "CEM patch digest does not match")
	}
	return nil
}

func verifyOCMIntent(ctx context.Context, repository *gitauth.Repository, cem *wire.Map, document *ocmDocument) (*ocmIntentContext, error) {
	reader := &ocmBlobReader{ctx: ctx, repository: repository, cache: map[string][]byte{}}
	intentBlob, err := reader.blob(document.target, document.intent.path, document.intent.blobOID)
	if err != nil {
		return nil, err
	}
	derived, err := deriveIntent(document.intent.form, document.intent.path, document.intent.blobOID, intentBlob)
	if err != nil {
		return nil, err
	}
	if derived.intent != document.intent {
		return nil, fail("intent-scope-mismatch", "intent scope is stale or invalid")
	}
	if err := enforceBootstrap(ctx, repository, cem, document.intent.path); err != nil {
		return nil, err
	}
	return &ocmIntentContext{reader: reader, requirements: derived.requirements, scope: derived.scope, statements: derived.statements}, nil
}

func verifyOCMClosure(cem *wire.Map, document *ocmDocument, raw []byte, intent *ocmIntentContext, rejectPythonClaims bool) (*verifiedOCM, error) {
	if rejectPythonClaims {
		for _, claim := range document.claims {
			if strings.EqualFold(filepath.Ext(claim.path), ".py") {
				return nil, fail("unsupported-ocm-python-claims", "native Go OCM cannot exactly verify Python claim syntax")
			}
		}
	}
	anchors, claimPaths, err := verifyClaims(intent.reader, document.target, document.claims)
	if err != nil {
		return nil, err
	}
	obligations, err := verifyObligations(document, cem, intent.requirements, intent.scope, intent.statements, anchors, claimPaths)
	if err != nil {
		return nil, err
	}
	return &verifiedOCM{
		spec: document.spec, mapSHA256: sha256Hex(raw), intent: document.intent,
		obligations: obligations,
	}, nil
}

func verifyLegacyOCMBinding(ctx context.Context, repository *gitauth.Repository, cem *wire.Map, cemRaw []byte, verified *verifiedCEM, target, expectedBase string) error {
	entry, exists, err := repository.LookupTreeEntry(ctx, target, wire.ExcludedCEMPath)
	if err != nil {
		return err
	}
	if exists {
		if entry.Type != "blob" || entry.Mode != "100644" {
			return cemcode.New(cemcode.ExcludedArtifactMismatch, "target-side %s is not a regular 100644 blob", wire.ExcludedCEMPath)
		}
		stored, err := repository.BlobBytes(ctx, entry.OID)
		if err != nil {
			return err
		}
		if !bytes.Equal(stored, cemRaw) {
			return cemcode.New(cemcode.ExcludedArtifactMismatch, "target-side %s bytes differ from the verified CEM input", wire.ExcludedCEMPath)
		}
	}
	derived, err := repository.CanonicalDiff(ctx, verified.base, target)
	if err != nil {
		return err
	}
	if sha256Hex(derived) != cem.PatchSha256 {
		return cemcode.New(cemcode.PatchDigestMismatch, "derived OCM patch does not match patchSha256")
	}
	_, err = verify.Exact(ctx, repository, cem, derived, verify.ExactOptions{ExpectedBase: expectedBase, Target: target})
	return err
}

type ocmBlobReader struct {
	ctx        context.Context
	repository *gitauth.Repository
	cache      map[string][]byte
	total      int
}

func (reader *ocmBlobReader) blob(revision, path, oid string) ([]byte, error) {
	cacheKey := revision + "\x00" + path + "\x00" + oid
	if cached, found := reader.cache[cacheKey]; found {
		return cached, nil
	}
	entry, exists, err := reader.repository.LookupTreeEntry(reader.ctx, revision, path)
	if err != nil {
		return nil, err
	}
	if !exists || entry.OID != oid || entry.Type != "blob" || entry.Mode != "100644" && entry.Mode != "100755" {
		return nil, fail("repository-object-unavailable", "repository object unavailable")
	}
	blob, err := reader.repository.BlobBytes(reader.ctx, oid)
	if err != nil {
		return nil, err
	}
	if len(blob) > maxOCMBlob || len(blob) > maxOCMBlobTotal-reader.total {
		return nil, fail("repository-object-unavailable", "repository object unavailable")
	}
	reader.total += len(blob)
	reader.cache[cacheKey] = blob
	return blob, nil
}

func requirementsFromBlob(path, oid string, data []byte) (ocmIntent, []string, []byte, error) {
	if !utf8.Valid(data) {
		return ocmIntent{}, nil, nil, fail("invalid-intent", "intent scope must be UTF-8 Markdown")
	}
	lines := lineOffsets(data)
	fenced := fencedLines(data, lines)
	headings := headingStarts(data, lines, fenced, []byte("## Requirements"))
	if len(headings) != 1 {
		return ocmIntent{}, nil, nil, fail("invalid-requirements-section", "intent must contain exactly one ## Requirements heading")
	}
	start := headings[0]
	end := sectionEnd(data, lines, fenced, start)
	requirements := make([]string, 0)
	seen := map[string]bool{}
	for _, offset := range lines {
		if offset < start || offset >= end {
			continue
		}
		line := lineWithoutEnding(data, offset)
		identityStart := 3
		closingMarkers := [][]byte{[]byte("`: ")}
		if bytes.HasPrefix(line, []byte("- **")) {
			identityStart = 4
			closingMarkers = [][]byte{[]byte(":** "), []byte(".** ")}
		} else if !bytes.HasPrefix(line, []byte("- `")) {
			continue
		}
		closing := -1
		for _, marker := range closingMarkers {
			if candidate := bytes.Index(line[identityStart:], marker); candidate >= 0 && (closing < 0 || candidate < closing) {
				closing = candidate
			}
		}
		if closing < 0 {
			continue
		}
		identity := string(line[identityStart : identityStart+closing])
		if !requirementID.MatchString(identity) {
			continue
		}
		if seen[identity] {
			return ocmIntent{}, nil, nil, fail("duplicate-requirement", "Requirements section repeats an ID")
		}
		seen[identity] = true
		requirements = append(requirements, identity)
	}
	if len(requirements) == 0 {
		return ocmIntent{}, nil, nil, fail("missing-requirements", "Requirements section has no requirement IDs")
	}
	if len(requirements) > maxObligations {
		return ocmIntent{}, nil, nil, fail("too-many-obligations", "Requirements section exceeds the obligation limit")
	}
	scope := data[start:end]
	intent := ocmIntent{path: path, blobOID: oid, start: int64(start), end: int64(end), spanSHA256: sha256Hex(scope)}
	return intent, requirements, scope, nil
}

// headingStarts returns the offset of every unfenced line equal to heading.
func headingStarts(data []byte, lines []int, fenced []bool, heading []byte) []int {
	headings := make([]int, 0, 1)
	for number, start := range lines {
		line := lineWithoutEnding(data, start)
		if !fenced[number] && bytes.Equal(line, heading) {
			headings = append(headings, start)
		}
	}
	return headings
}

// sectionEnd returns the offset of the first unfenced level-2 heading after
// start, or the end of data.
func sectionEnd(data []byte, lines []int, fenced []bool, start int) int {
	for number, offset := range lines {
		if offset <= start || fenced[number] {
			continue
		}
		if line := lineWithoutEnding(data, offset); bytes.Equal(line, []byte("##")) || bytes.HasPrefix(line, []byte("## ")) {
			return offset
		}
	}
	return len(data)
}

// fencedLines marks every line that opens, closes, or sits inside a CommonMark
// fenced code block: a run of at least three backticks or tildes indented at most
// three spaces opens one (a backtick fence's info string holds no backtick), and
// only a run of the same character at least as long, followed by nothing but
// spaces or tabs, closes it. An unclosed fence runs to the end of the document.
func fencedLines(data []byte, lines []int) []bool {
	fenced := make([]bool, len(lines))
	char, length := byte(0), 0
	for number, start := range lines {
		line := lineWithoutEnding(data, start)
		fenced[number] = length > 0
		if length > 0 {
			if runChar, run, rest := markdownFenceRun(line); runChar == char && run >= length && len(bytes.Trim(rest, " \t")) == 0 {
				length = 0
			}
			continue
		}
		runChar, run, rest := markdownFenceRun(line)
		if run < 3 || runChar == '`' && bytes.IndexByte(rest, '`') >= 0 {
			continue
		}
		char, length = runChar, run
		fenced[number] = true
	}
	return fenced
}

// markdownFenceRun returns the backtick or tilde run that starts a line indented
// at most three spaces, its length, and the bytes after it.
func markdownFenceRun(line []byte) (byte, int, []byte) {
	indent := 0
	for indent < len(line) && indent < 4 && line[indent] == ' ' {
		indent++
	}
	if indent > 3 || indent == len(line) || line[indent] != '`' && line[indent] != '~' {
		return 0, 0, nil
	}
	char := line[indent]
	end := indent
	for end < len(line) && line[end] == char {
		end++
	}
	return char, end - indent, line[end:]
}

func lineOffsets(data []byte) []int {
	result := []int{0}
	for index, value := range data {
		if value == '\n' && index+1 < len(data) {
			result = append(result, index+1)
		}
	}
	return result
}

func lineWithoutEnding(data []byte, start int) []byte {
	end := bytes.IndexByte(data[start:], '\n')
	if end < 0 {
		end = len(data)
	} else {
		end += start
	}
	line := data[start:end]
	return bytes.TrimSuffix(line, []byte{'\r'})
}

func enforceBootstrap(ctx context.Context, repository *gitauth.Repository, cem *wire.Map, intentPath string) error {
	_, exists, err := repository.LookupTreeEntry(ctx, cem.BaseRevision, intentPath)
	if err != nil || exists {
		return err
	}
	found := false
	for _, hunk := range cem.Hunks {
		if hunk.Path != intentPath {
			continue
		}
		found = true
		if hunk.Disposition != "unknown" {
			return fail("bootstrap-intent-not-unknown", "bootstrap intent hunk must remain unknown")
		}
	}
	if !found {
		return fail("bootstrap-intent-not-unknown", "bootstrap intent hunk must remain unknown")
	}
	return nil
}

func verifyClaims(reader *ocmBlobReader, target string, claims []ocmClaim) (map[string][]byte, map[string]string, error) {
	anchors := make(map[string][]byte, len(claims))
	paths := make(map[string]string, len(claims))
	pythonValidity := map[string]bool{}
	blobs := newBlobIndexes()
	for _, claim := range claims {
		blob, err := reader.blob(target, claim.path, claim.blobOID)
		if err != nil {
			return nil, nil, err
		}
		if claim.span.start < 0 || claim.span.end > int64(len(blob)) || claim.span.start >= claim.span.end {
			return nil, nil, fail("claim-not-reextractable", "claim cannot be re-extracted at target")
		}
		anchor := blob[claim.span.start:claim.span.end]
		extractable := claimShapeExtractableIn(claim, blob, blobs)
		if strings.EqualFold(filepath.Ext(claim.path), ".py") {
			valid, found := pythonValidity[claim.blobOID]
			if !found {
				valid = pythonSyntaxValid(claim.path, blob)
				pythonValidity[claim.blobOID] = valid
			}
			extractable = extractable && valid
		}
		if sha256Hex(anchor) != claim.spanSHA256 || !extractable {
			return nil, nil, fail("claim-not-reextractable", "claim cannot be re-extracted at target")
		}
		anchors[claim.id] = append([]byte(nil), anchor...)
		paths[claim.id] = claim.path
	}
	return anchors, paths, nil
}

func verifyObligations(document *ocmDocument, cem *wire.Map, requirements []string, scope []byte, statements map[string][]byte, anchors map[string][]byte, claimPaths map[string]string) ([]lrf.Obligation, error) {
	if len(document.obligations) != len(requirements) {
		return nil, fail("obligation-set-mismatch", "obligations do not match requirements")
	}
	supported := map[string]bool{}
	for _, hunk := range cem.Hunks {
		if hunk.Disposition == "supported" {
			supported[hunk.ID] = true
		}
	}
	result := make([]lrf.Obligation, 0)
	for ordinal, obligation := range document.obligations {
		if obligation.id != requirements[ordinal] {
			return nil, fail("obligation-order-mismatch", "obligations must retain requirement order")
		}
		switch obligation.disposition {
		case "unknown":
			if !unknownReason[obligation.reason] || len(obligation.hunkIDs) != 0 || len(obligation.claimIDs) != 0 {
				return nil, fail("invalid-unknown", "unknown obligation has invalid fields")
			}
			continue
		case "linked":
			if obligation.reason != "change-and-test-linked" || len(obligation.hunkIDs) == 0 || len(obligation.claimIDs) == 0 {
				return nil, fail("invalid-linked", "linked obligation has invalid fields")
			}
		default:
			return nil, fail("invalid-linked", "linked obligation has invalid fields")
		}
		for _, identity := range obligation.hunkIDs {
			if !supported[identity] {
				return nil, fail("unknown-hunk-reference", "linked hunk is not verified and supported")
			}
		}
		paths := make([]string, 0, len(obligation.claimIDs))
		for _, identity := range obligation.claimIDs {
			anchor, found := anchors[identity]
			if !found {
				return nil, fail("unknown-claim-reference", "linked claim is absent")
			}
			if !containsExactRequirement(anchor, obligation.id) {
				return nil, fail("claim-obligation-mismatch", "claim anchor lacks the exact obligation ID")
			}
			paths = append(paths, claimPaths[identity])
		}
		sort.Strings(paths)
		paths = uniqueStrings(paths)
		statement, err := obligationStatement(statements, scope, obligation.id)
		if err != nil {
			return nil, err
		}
		result = append(result, lrf.Obligation{
			ID: obligation.id, Statement: statement, HunkIDs: append([]string(nil), obligation.hunkIDs...),
			ClaimPaths: paths, Ordinal: len(result),
		})
	}
	return result, nil
}

func claimExtractable(claim ocmClaim, blob []byte) bool {
	return claimExtractableIn(claim, blob, newBlobIndexes())
}

// claimExtractableIn is claimExtractable over a cache shared by every claim of
// one OCM operation, so a blob is derived once rather than once per claim.
func claimExtractableIn(claim ocmClaim, blob []byte, cache *blobIndexes) bool {
	if strings.EqualFold(filepath.Ext(claim.path), ".py") && !pythonSyntaxValid(claim.path, blob) {
		return false
	}
	return claimShapeExtractableIn(claim, blob, cache)
}

func claimShapeExtractable(claim ocmClaim, blob []byte) bool {
	return claimShapeExtractableIn(claim, blob, newBlobIndexes())
}

func claimShapeExtractableIn(claim ocmClaim, blob []byte, cache *blobIndexes) bool {
	start, end := int(claim.span.start), int(claim.span.end)
	anchor := blob[start:end]
	suffix := strings.ToLower(filepath.Ext(claim.path))
	switch suffix {
	case ".go":
		return extractableGoClaim(claim.selector, anchor, blob, start, end, cache.goIndex(claim.blobOID, blob))
	case ".py":
		return extractablePythonClaim(claim.selector, anchor, blob, start, end, cache.pythonIndex(claim.blobOID, blob))
	case ".js", ".jsx", ".ts", ".tsx", ".mjs", ".cjs":
		return extractableJSClaim(claim.selector, anchor, blob, start, end, cache.jsIndex(claim.blobOID, blob))
	default:
		return false
	}
}

func extractableGoClaim(selector string, anchor, blob []byte, start, end int, index *goBlobIndex) bool {
	if !strings.HasPrefix(selector, "test:") {
		return false
	}
	clean := index.clean
	if !bytes.Equal(clean[start:end], anchor) {
		return false
	}
	if !strings.Contains(selector, "/case:") {
		name := strings.TrimPrefix(selector, "test:")
		return string(anchor) == name && name == goTestName(name) && tokenPrefix(clean, start, "func ") &&
			tokenSuffix(clean, end, '(') && index.funcCount(name) == 1 &&
			lexicallyAnchored(claimFromIdentifier(name, "Test"), string(anchor), blob)
	}
	if start == 0 || end >= len(clean) || clean[start-1] != '"' || clean[end] != '"' {
		return false
	}
	lineStart := bytes.LastIndexByte(clean[:start], '\n') + 1
	prefix := clean[lineStart : start-1]
	if !goTableCaseTail.Match(prefix) {
		return false
	}
	var claimText string
	if err := json.Unmarshal(append(append([]byte{'"'}, anchor...), '"'), &claimText); err != nil {
		return false
	}
	parent := index.nearestTest(lineStart)
	expected := "test:" + parent + "/case:" + selectorFragment(claimText)
	return parent != "" && selector == expected && index.caseCount(expected) == 1 && lexicallyAnchored(claimText, string(anchor), blob)
}

func extractablePythonClaim(selector string, anchor, blob []byte, start, end int, index *pythonBlobIndex) bool {
	if !strings.HasPrefix(selector, "test:test_") {
		return false
	}
	clean := index.clean
	if !bytes.Equal(clean[start:end], anchor) {
		return false
	}
	base := strings.TrimSuffix(strings.TrimPrefix(selector, "test:"), "#doc")
	if !strings.HasSuffix(selector, "#doc") {
		return string(anchor) == base && tokenSuffix(clean, end, '(') && pythonDefPrefix(clean, start) && index.testCount(base) == 1 &&
			lexicallyAnchored(claimFromIdentifier(base, "test_"), string(anchor), blob)
	}
	if index.nearestTest(start) != base || index.testCount(base) != 1 || !index.firstStatement(start) {
		return false
	}
	quoted := bytes.HasPrefix(anchor, []byte(`"""`)) || bytes.HasPrefix(anchor, []byte(`'''`)) ||
		len(anchor) >= 2 && (anchor[0] == '"' && anchor[len(anchor)-1] == '"' || anchor[0] == '\'' && anchor[len(anchor)-1] == '\'')
	return quoted && lexicallyAnchored(pythonLiteralText(anchor), string(anchor), blob)
}

func pythonSyntaxValid(path string, blob []byte) bool {
	if len(blob) > maxOCMBlob {
		return false
	}
	return pythonsyntax.SourceSyntaxValid(path, blob)
}

func extractableJSClaim(selector string, anchor, blob []byte, start, end int, index *jsBlobIndex) bool {
	if !strings.HasPrefix(selector, "test:") || start < 1 || end >= len(blob) {
		return false
	}
	clean := index.clean
	if !bytes.Equal(clean[start:end], anchor) {
		return false
	}
	quote := clean[start-1]
	if quote != '"' && quote != '\'' && quote != '`' || clean[end] != quote {
		return false
	}
	claimText, ok := decodeJSString(anchor, quote)
	if !ok || selector != "test:"+selectorFragment(claimText) {
		return false
	}
	left := start - 128
	if left < 0 {
		left = 0
	}
	prefix := clean[left : start-1]
	return jsTestCallTail.Match(prefix) &&
		index.selectorCount(selector) == 1 && lexicallyAnchored(claimText, string(anchor), blob)
}

func goTestName(name string) string {
	if strings.HasPrefix(name, "Test") && len(name) > 4 && name[4] >= 'A' && name[4] <= 'Z' {
		return name
	}
	return ""
}

func tokenPrefix(blob []byte, start int, prefix string) bool {
	return start >= len(prefix) && string(blob[start-len(prefix):start]) == prefix
}

func tokenSuffix(blob []byte, end int, suffix byte) bool {
	for end < len(blob) && (blob[end] == ' ' || blob[end] == '\t') {
		end++
	}
	return end < len(blob) && blob[end] == suffix
}

func pythonDefPrefix(blob []byte, start int) bool {
	lineStart := bytes.LastIndexByte(blob[:start], '\n') + 1
	prefix := strings.TrimSpace(string(blob[lineStart:start]))
	return prefix == "def" || prefix == "async def"
}

func maskComments(data []byte, suffix string) []byte {
	masked := append([]byte(nil), data...)
	quote := byte(0)
	triple := []byte(nil)
	// last is the previous significant code byte (-1 at the start) and literal
	// records that it closed a string, template, or regex literal; together they
	// decide whether a JS `/` opens a regex literal or divides.
	regexes := suffix != ".py" && suffix != ".go"
	last, literal := -1, false
	// templates holds one open-brace count per JS `${` expression the scan is
	// inside; its closing `}` resumes the enclosing template literal, so a
	// quote or backtick in the expression cannot end the template early.
	var templates []int
	for index := 0; index < len(data); {
		if triple != nil {
			if bytes.HasPrefix(data[index:], triple) {
				index += len(triple)
				triple = nil
			} else {
				index++
			}
			continue
		}
		if quote != 0 {
			// A Go raw string has no escapes: `\` before its closing backtick
			// must not swallow it and leave later comments unmasked.
			if data[index] == '\\' && !(quote == '`' && suffix == ".go") {
				index += 2
			} else if quote == '`' && regexes && bytes.HasPrefix(data[index:], []byte("${")) {
				templates = append(templates, 0)
				quote = 0
				last, literal = index+1, false
				index += 2
			} else if data[index] == quote {
				quote = 0
				last, literal = index, true
				index++
			} else {
				index++
			}
			continue
		}
		if suffix == ".py" && (bytes.HasPrefix(data[index:], []byte(`"""`)) || bytes.HasPrefix(data[index:], []byte(`'''`))) {
			triple = append([]byte(nil), data[index:index+3]...)
			index += 3
			continue
		}
		if data[index] == '"' || data[index] == '\'' || data[index] == '`' {
			quote = data[index]
			index++
			continue
		}
		if suffix == ".py" && data[index] == '#' {
			end := bytes.IndexByte(data[index:], '\n')
			if end < 0 {
				end = len(data)
			} else {
				end += index
			}
			maskRange(masked, index, end)
			index = end
			continue
		}
		if suffix != ".py" && bytes.HasPrefix(data[index:], []byte("//")) {
			end := bytes.IndexByte(data[index:], '\n')
			if end < 0 {
				end = len(data)
			} else {
				end += index
			}
			maskRange(masked, index, end)
			index = end
			continue
		}
		if suffix != ".py" && bytes.HasPrefix(data[index:], []byte("/*")) {
			end := bytes.Index(data[index+2:], []byte("*/"))
			if end < 0 {
				end = len(data)
			} else {
				end += index + 4
			}
			maskRange(masked, index, end)
			index = end
			continue
		}
		if regexes && data[index] == '/' && jsSlashOpensRegex(data, last, literal) {
			end, closed := jsRegexEnd(data, index)
			// A regex candidate the line ends before closing cannot be read
			// exactly, so its rest of line is masked: fewer claims, never an
			// invented one.
			if !closed {
				maskRange(masked, index, end)
			}
			last, literal = end-1, true
			index = end
			continue
		}
		if top := len(templates) - 1; top >= 0 && data[index] == '{' {
			templates[top]++
		} else if top >= 0 && data[index] == '}' && templates[top] > 0 {
			templates[top]--
		} else if top >= 0 && data[index] == '}' {
			templates = templates[:top]
			quote = '`'
			index++
			continue
		}
		if data[index] > ' ' {
			last, literal = index, false
		}
		index++
	}
	return masked
}

// jsRegexKeywords end an expression position, so a `/` after one opens a regex.
var jsRegexKeywords = map[string]bool{
	"return": true, "typeof": true, "instanceof": true, "in": true, "of": true, "new": true,
	"delete": true, "void": true, "throw": true, "case": true, "do": true, "else": true,
	"yield": true, "await": true,
}

// jsSlashOpensRegex applies the lexical regex-versus-division rule: a `/` divides
// after an identifier, number, literal, `)`, `]`, `}`, or postfix `++`/`--`, and
// opens a regex literal anywhere else, including after the keywords above.
func jsSlashOpensRegex(data []byte, last int, literal bool) bool {
	if last < 0 {
		return true
	}
	if literal {
		return false
	}
	previous := data[last]
	if previous == ')' || previous == ']' || previous == '}' {
		return false
	}
	if (previous == '+' || previous == '-') && last > 0 && data[last-1] == previous {
		return false
	}
	if !jsIdentifierByte(previous) {
		return true
	}
	start := last
	for start > 0 && jsIdentifierByte(data[start-1]) {
		start--
	}
	return jsRegexKeywords[string(data[start:last+1])]
}

func jsIdentifierByte(value byte) bool {
	return value == '_' || value == '$' || value >= 0x80 || value >= '0' && value <= '9' || value >= 'a' && value <= 'z' || value >= 'A' && value <= 'Z'
}

// jsRegexEnd returns the offset after the closing `/` of the regex literal that
// opens at start, skipping escapes and `/` inside a `[...]` class, with closed
// true; or the offset of the line ending (or end of data) with closed false.
func jsRegexEnd(data []byte, start int) (int, bool) {
	class := false
	for index := start + 1; index < len(data); index++ {
		switch value := data[index]; {
		case value == '\n' || value == '\r':
			return index, false
		case value == '\\' && index+1 < len(data) && data[index+1] != '\n' && data[index+1] != '\r':
			index++
		case value == '[':
			class = true
		case value == ']':
			class = false
		case value == '/' && !class:
			return index + 1, true
		}
	}
	return len(data), false
}

func maskRange(data []byte, start, end int) {
	for index := start; index < end; index++ {
		if data[index] != '\n' && data[index] != '\r' {
			data[index] = ' '
		}
	}
}

func leadingIndent(line []byte) int {
	count := 0
	for _, value := range line {
		if value == ' ' {
			count++
			continue
		}
		if value == '\t' {
			count += 8 - count%8
			continue
		}
		break
	}
	return count
}

func decodeJSString(raw []byte, quote byte) (string, bool) {
	if quote == '`' && bytes.Contains(raw, []byte("${")) {
		return "", false
	}
	if quote == '"' {
		var decoded string
		if err := json.Unmarshal(append(append([]byte{'"'}, raw...), '"'), &decoded); err != nil {
			return "", false
		}
		return decoded, true
	}
	var output strings.Builder
	for index := 0; index < len(raw); index++ {
		if raw[index] == '\\' && index+1 < len(raw) && (raw[index+1] == '\'' || raw[index+1] == '\\' || raw[index+1] == '`') {
			index++
		}
		output.WriteByte(raw[index])
	}
	return output.String(), utf8.ValidString(output.String())
}

func selectorFragment(value string) string {
	var expanded strings.Builder
	characters := []rune(value)
	for index, character := range characters {
		if index > 0 && (characters[index-1] >= 'a' && characters[index-1] <= 'z' || characters[index-1] >= '0' && characters[index-1] <= '9') && character >= 'A' && character <= 'Z' {
			expanded.WriteByte(' ')
		}
		if character == '_' || character == '-' {
			expanded.WriteByte(' ')
		} else {
			expanded.WriteRune(character)
		}
	}
	words := regexp.MustCompile(`[A-Za-z][A-Za-z0-9]*`).FindAllString(expanded.String(), -1)
	for index, word := range words {
		word = strings.ToLower(word)
		if strings.HasSuffix(word, "ies") && len(word) > 5 {
			word = strings.TrimSuffix(word, "ies") + "y"
		} else if strings.HasSuffix(word, "s") && !strings.HasSuffix(word, "ss") && len(word) > 4 {
			word = strings.TrimSuffix(word, "s")
		}
		words[index] = word
	}
	fragment := strings.Trim(strings.Join(words, "-"), "-")
	if len(fragment) > 96 {
		fragment = strings.Trim(fragment[:96], "-")
	}
	if fragment == "" {
		return "unnamed"
	}
	return fragment
}

func claimFromIdentifier(value, prefix string) string {
	value = strings.TrimPrefix(value, prefix)
	return strings.Join(splitClaimWords(value), " ")
}

func splitClaimWords(value string) []string {
	var expanded strings.Builder
	characters := []rune(value)
	for index, character := range characters {
		if index > 0 && (characters[index-1] >= 'a' && characters[index-1] <= 'z' || characters[index-1] >= '0' && characters[index-1] <= '9') && character >= 'A' && character <= 'Z' {
			expanded.WriteByte(' ')
		}
		if character == '_' || character == '-' {
			expanded.WriteByte(' ')
		} else {
			expanded.WriteRune(character)
		}
	}
	words := regexp.MustCompile(`[A-Za-z][A-Za-z0-9]*`).FindAllString(expanded.String(), -1)
	for index, word := range words {
		word = strings.ToLower(word)
		if strings.HasSuffix(word, "ies") && len(word) > 5 {
			word = strings.TrimSuffix(word, "ies") + "y"
		} else if strings.HasSuffix(word, "s") && !strings.HasSuffix(word, "ss") && len(word) > 4 {
			word = strings.TrimSuffix(word, "s")
		}
		words[index] = word
	}
	return words
}

func lexicallyAnchored(claim, anchor string, blob []byte) bool {
	claim = strings.Join(strings.Fields(claim), " ")
	claimRunes := []rune(claim)
	if len(claimRunes) > 280 {
		claim = string(claimRunes[:280])
	}
	weak := wordSet("a an and are as at basic be by can case do does for from happy in is it of on or path simple smoke test that the this to verify verifies works")
	relations := wordSet("claim covers demonstrates governs implements proving proves should validates")
	terms := map[string]bool{}
	for _, word := range splitClaimWords(claim) {
		if !weak[word] && !relations[word] {
			terms[word] = true
		}
	}
	if claim == "" || len(terms) == 0 {
		return false
	}
	if len(terms) == 1 {
		for term := range terms {
			if len(term) < 5 || term == "thing" || term == "value" || term == "work" {
				return false
			}
		}
	}
	anchorTerms := map[string]bool{}
	for _, word := range splitClaimWords(anchor) {
		anchorTerms[word] = true
	}
	overlap := 0
	compactClaim := make([]string, 0, len(terms))
	for term := range terms {
		compactClaim = append(compactClaim, term)
		if anchorTerms[term] {
			overlap++
		}
	}
	sort.Strings(compactClaim)
	compactAnchor := strings.Join(splitClaimWords(anchor), "")
	needed := 2
	if len(terms) < needed {
		needed = len(terms)
	}
	supported := overlap >= needed || len(terms) == 1 && overlap == 1 ||
		len(strings.Join(compactClaim, "")) >= 7 && strings.Contains(compactAnchor, strings.Join(compactClaim, ""))
	if !supported {
		return false
	}
	return len(strings.TrimSpace(anchor)) >= 32 || bytes.Count(blob, []byte(anchor)) <= 1 || overlap >= 2
}

func wordSet(value string) map[string]bool {
	result := map[string]bool{}
	for _, word := range strings.Fields(value) {
		result[word] = true
	}
	return result
}

func pythonLiteralText(anchor []byte) string {
	text := string(anchor)
	for _, quote := range []string{`"""`, `'''`, `"`, `'`} {
		if strings.HasPrefix(text, quote) && strings.HasSuffix(text, quote) && len(text) >= 2*len(quote) {
			return text[len(quote) : len(text)-len(quote)]
		}
	}
	return text
}

func containsExactRequirement(anchor []byte, identity string) bool {
	token := []byte(identity)
	for offset := 0; offset <= len(anchor)-len(token); {
		found := bytes.Index(anchor[offset:], token)
		if found < 0 {
			return false
		}
		start := offset + found
		end := start + len(token)
		left := start == 0 || !requirementAdjacent(anchor[start-1])
		right := end == len(anchor) || !requirementAdjacent(anchor[end])
		if left && right {
			return true
		}
		offset = start + 1
	}
	return false
}

func requirementAdjacent(value byte) bool {
	return value >= 'A' && value <= 'Z' || value >= 'a' && value <= 'z' ||
		value >= '0' && value <= '9' || value == '_' || value == '-'
}

func requirementStatement(scope []byte, identity string) ([]byte, error) {
	prefixes := [][]byte{
		[]byte("- `" + identity + "`: "),
		[]byte("- **" + identity + ":** "),
		[]byte("- **" + identity + ".** "),
	}
	matches := make([][]byte, 0, 1)
	for _, prefix := range prefixes {
		for offset := 0; offset < len(scope); {
			found := bytes.Index(scope[offset:], prefix)
			if found < 0 {
				break
			}
			start := offset + found
			if start == 0 || scope[start-1] == '\n' {
				end := bytes.IndexByte(scope[start:], '\n')
				if end < 0 {
					end = len(scope)
				} else {
					end += start + 1
				}
				matches = append(matches, scope[start:end])
			}
			offset = start + 1
		}
	}
	if len(matches) != 1 {
		return nil, fail("invalid-requirement-prefix", "requirement line is absent or ambiguous")
	}
	return append([]byte(nil), matches[0]...), nil
}

func uniqueStrings(values []string) []string {
	if len(values) < 2 {
		return values
	}
	result := values[:1]
	for _, value := range values[1:] {
		if value != result[len(result)-1] {
			result = append(result, value)
		}
	}
	return result
}

// gitBoundReached reports a bounded-runner refusal (budget elapsed or exhausted,
// or caller cancellation). It says nothing about whether the object exists, so
// it must not be folded into repository-object-unavailable.
func gitBoundReached(err error) bool {
	switch cemcode.CodeOf(err) {
	case cemcode.GitTimeout, cemcode.GitBudgetExceeded, cemcode.GitCancelled:
		return true
	}
	return false
}
