package dogfoodflow

import (
	"bytes"
	"context"
	"errors"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/Beamfall/corvint/internal/cem/patch"
	"github.com/Beamfall/corvint/internal/cem/wire"
)

// RecordClaimsOptions names the inputs of `corvint dogfood record-claims
// BASE --plan PLAN --claims FILE [--draft DRAFT]` (CKC-V0-002).
type RecordClaimsOptions struct {
	Root   string
	Base   string
	Plan   string
	Draft  string
	Claims string
}

// RecordClaims records a reviewed citation plan as content-keyed claims
// (CKC-V0-001 to CKC-V0-003). It writes only the --claims file, and only
// once every row resolved to a current hunk.
func RecordClaims(ctx context.Context, options RecordClaimsOptions, stdout, stderr io.Writer) (int, error) {
	run := &proposal{
		flow:    flow{ctx: ctx, prefix: "dogfood-record-claims", root: options.Root, stderr: stderr},
		options: ProposeOptions{Root: options.Root, Base: options.Base}, stdout: stdout,
		blobs: map[string][]byte{}, lines: map[string][][]byte{},
	}
	return boundary(ctx, func() int { return run.record(options) }, func() {})
}

// plannedRow is one resolved plan row.
type plannedRow struct {
	line int
	hunk *patch.Hunk
	row  claimRow
}

func (p *proposal) record(options RecordClaimsOptions) int {
	p.requireRoot()
	p.requireTrustedAncestry()
	if options.Plan == "" || options.Claims == "" {
		p.say("%s: REFUSE claims-arguments\n  record-claims needs --plan PLAN and --claims FILE (CKC-V0-002)\n", p.prefix)
		p.exit(2)
	}
	base, status := p.git(true, "rev-parse", "--verify", "-q", options.Base+"^{commit}")
	if status != 0 {
		p.say("%s: REFUSE base-unavailable BASE=%s\n  fix: pass BASE as a commit of this repository, preferably the full origin/main ID\n", p.prefix, options.Base)
		p.exit(2)
	}
	p.base = chomp(base)
	p.target = p.gitValue("rev-parse", "HEAD^{commit}")
	parsed, reason, detail := p.readCanonicalPatch()
	if parsed == nil {
		p.refusePatchUnavailable(reason, detail)
	}
	ordinals := p.recordOrdinals(parsed)
	// Every BASE file the record reads comes from one batched read rather
	// than two Git processes a file (CKC-V0-012).
	names := planPaths(options.Plan, options.Draft)
	for _, hunk := range parsed.Hunks {
		names = append(names, hunk.DisplayPath)
	}
	p.prefetchBlobs(p.base, names)
	planRows, deferred := p.readClaimPlan(options.Plan, "plan", parsed, ordinals)
	var draftRows []plannedRow
	if options.Draft != "" {
		draftRows, _ = p.readClaimPlan(options.Draft, "draft", parsed, ordinals)
	}
	prior := p.priorClaims(options.Claims, parsed)
	file := &claimsFile{Schema: claimsSchema, Base: p.base, Deferred: deferred}
	byHunk := map[*patch.Hunk]*claim{}
	for _, planned := range planRows {
		entry := byHunk[planned.hunk]
		if entry == nil {
			entry = p.newClaim(planned.hunk)
			byHunk[planned.hunk] = entry
		}
		entry.Rows = append(entry.Rows, planned.row)
	}
	kept := func(entry *claim, row claimRow) bool {
		for _, existing := range entry.Rows {
			if existing.Kind == "cite" && existing.tombKey() == row.tombKey() {
				return true
			}
		}
		for _, existing := range entry.Tombstones {
			if existing.tombKey() == row.tombKey() {
				return true
			}
		}
		return false
	}
	bury := func(hunk *patch.Hunk, row claimRow) {
		entry := byHunk[hunk]
		if entry == nil || row.Kind != "cite" || row.Text == "" || kept(entry, row) {
			return
		}
		entry.Tombstones = append(entry.Tombstones, row)
	}
	for _, drafted := range draftRows {
		bury(drafted.hunk, drafted.row)
	}
	for hunk, rows := range prior {
		for _, row := range rows {
			bury(hunk, row)
		}
	}
	tombstones, rows := 0, 0
	for _, hunk := range parsed.Hunks {
		if entry := byHunk[hunk]; entry != nil {
			file.Claims = append(file.Claims, *entry)
			tombstones += len(entry.Tombstones)
			rows += len(entry.Rows) + len(entry.Tombstones)
		}
	}
	if len(file.Claims) > maxClaims || rows > maxClaimRows {
		p.say("%s: REFUSE claims-over-bound\n  %d claims and %d rows exceed the %d-claim, %d-row bound; nothing was written (CKC-V0-001)\n", p.prefix, len(file.Claims), rows, maxClaims, maxClaimRows)
		p.exit(2)
	}
	data, err := encodeClaims(file)
	if err == nil && len(data) > maxClaimsBytes {
		err = errors.New("over bound")
	}
	if err == nil {
		err = writeClaimsFile(options.Claims, data)
	}
	if err != nil {
		p.say("%s: REFUSE claims-unwritable\n  %s could not be written whole (%v); nothing was recorded (CKC-V0-001)\n", p.prefix, strconv.Quote(options.Claims), err)
		p.exit(2)
	}
	p.say("%s: RECORDED claims=%d rows=%d tombstones=%d deferred=%d base=%s file=%s\n", p.prefix, len(file.Claims), rows-tombstones, tombstones, len(deferred), p.base, strconv.Quote(options.Claims))
	p.say("  next: after a fix commit or a rebase, corvint dogfood propose-citations BASE --refresh %s carries these rows to the current hunk IDs (CKC-V0-004)\n", strconv.Quote(options.Claims))
	return 0
}

// recordOrdinals maps plan ordinals to the hunks of a prepared map that is
// for BASE and still matches the canonical patch, or nil (CKC-V0-002,
// DCW-V0-382).
func (p *proposal) recordOrdinals(parsed *patch.Patch) []*patch.Hunk {
	data, err := readPrefix(filepath.Join(p.root, ".corvint", "change.cem.json"), maxPlanBytes)
	if err != nil {
		return nil
	}
	baseRevision, hunks, ok := preparedHunks(data)
	if !ok || baseRevision != p.base || p.mapStale(hunks) {
		return nil
	}
	byRange := map[string]*patch.Hunk{}
	for _, hunk := range parsed.Hunks {
		byRange[rangeKey(hunk.DisplayPath, hunk.OldRange.Start, hunk.OldRange.Count)] = hunk
	}
	ordered := make([]*patch.Hunk, len(hunks))
	for index, hunk := range hunks {
		if ordered[index] = byRange[rangeKey(hunk.path, int64(hunk.oldStart), int64(hunk.oldCount))]; ordered[index] == nil {
			return nil
		}
	}
	return ordered
}

// readClaimPlan reads a plan or draft and resolves every row to a current
// hunk, refusing the whole record on any row it cannot (CKC-V0-002).
func (p *proposal) readClaimPlan(name, label string, parsed *patch.Patch, ordinals []*patch.Hunk) ([]plannedRow, []string) {
	refuse := func(line int, why string) {
		p.say("%s: REFUSE claims-%s-row %s row %d\n  %s; nothing was written (CKC-V0-002)\n", p.prefix, label, label, line, why)
		p.say("  fix: corvint dogfood propose-citations %s --refresh PLAN keys every row by its current full hunk ID; review it, then rerun record-claims\n", p.options.Base)
		p.exit(2)
	}
	var data []byte
	err := errors.New("not a regular file")
	if isRegular(name) {
		data, err = readPrefix(name, maxPlanBytes+1)
	}
	if err != nil || len(data) > maxPlanBytes || hasForbiddenControl(data) {
		p.say("%s: REFUSE claims-%s-unavailable\n  --%s %s is not a readable regular citation plan of at most %d bytes; nothing was written (CKC-V0-002)\n", p.prefix, label, label, strconv.Quote(name), maxPlanBytes)
		p.exit(2)
	}
	byID := map[string]*patch.Hunk{}
	for _, hunk := range parsed.Hunks {
		byID[hunk.ID] = hunk
	}
	rows, deferred := []plannedRow{}, []string{}
	for index, line := range textLines(data) {
		if strings.TrimSpace(line) == "" {
			continue
		}
		fields := strings.Split(line, "\t")
		if fields[0] == deferredRowKind {
			deferred = append(deferred, line)
			continue
		}
		if !nonEmptyFields(line, 4) {
			refuse(index+1, "it is not a four-field SELECTOR PATH START:END RELATION or SELECTOR unknown REASON NOTE row")
		}
		var hunk *patch.Hunk
		switch {
		case strings.HasPrefix(fields[0], wire.HunkPrefix):
			hunk = byID[fields[0]]
		case ordinal.MatchString(fields[0]):
			if value, _ := strconv.Atoi(fields[0]); ordinals != nil && value <= len(ordinals) {
				hunk = ordinals[value-1]
			}
		}
		if hunk == nil {
			refuse(index+1, "selector "+strconv.Quote(fields[0])+" names no hunk of the current BASE..HEAD patch (an ordinal resolves only through a prepared map for BASE that still matches it)")
		}
		row := claimRow{Kind: "cite", Path: fields[1], Lines: fields[2], Relation: fields[3]}
		if unknownRow(fields) {
			row = claimRow{Kind: "unknown", Reason: fields[2], Note: fields[3]}
		} else if _, _, ok := parseLineSpan(fields[2]); ok {
			text, found := spanText(p.baseLines(fields[1]), fields[2])
			if !found {
				refuse(index+1, "its span "+fields[2]+" is outside "+fields[1]+" at BASE")
			}
			row.Text = sha256Hex(text)
		}
		if !row.valid() {
			refuse(index+1, "it is not a valid cite row (a registered relation and a START:END span) or reasoned unknown row (DCW-V0-019)")
		}
		rows = append(rows, plannedRow{line: index + 1, hunk: hunk, row: row})
	}
	if len(deferred) > 0 {
		if _, _, valid := splitDeferredRows([]byte(strings.Join(deferred, "\n") + "\n")); !valid {
			refuse(0, "its deferred rows are not valid deferred<TAB>PATH<TAB>REASON rows (DCW-V0-370)")
		}
	}
	return rows, deferred
}

// planPaths returns the PATH field of every row of the named plans that it can
// read. It only chooses what to prefetch, so an unreadable plan adds nothing
// and readClaimPlan still refuses it.
func planPaths(names ...string) []string {
	var paths []string
	for _, name := range names {
		if name == "" || !isRegular(name) {
			continue
		}
		data, err := readPrefix(name, maxPlanBytes+1)
		if err != nil || len(data) > maxPlanBytes {
			continue
		}
		for _, line := range textLines(data) {
			if fields := strings.Split(line, "\t"); len(fields) == 4 && fields[1] != "unknown" {
				paths = append(paths, fields[1])
			}
		}
	}
	return paths
}

// prefetchMovedBase batches the reads a claims compile makes when BASE moved
// since the file was recorded: every claim and row path at both commits
// (CKC-V0-012). A compile at an unmoved BASE reads no file.
func (p *proposal) prefetchMovedBase(file *claimsFile) {
	if file.Base == p.base {
		return
	}
	var names []string
	for _, entry := range file.Claims {
		names = append(names, entry.Path)
		for _, rows := range [][]claimRow{entry.Rows, entry.Tombstones} {
			for _, row := range rows {
				if row.Kind == "cite" {
					names = append(names, row.Path)
				}
			}
		}
	}
	p.prefetchBlobs(file.Base, names)
	p.prefetchBlobs(p.base, names)
}

// newClaim keys hunk by path, content, BASE range and anchor (CKC-V0-001).
func (p *proposal) newClaim(hunk *patch.Hunk) *claim {
	entry := &claim{Path: hunk.DisplayPath, Content: hunk.ContentSha256, OldStart: hunk.OldRange.Start, OldCount: hunk.OldRange.Count, Occurrences: 1}
	records := p.baseLines(hunk.DisplayPath)
	needle, _ := rangeRecords(records, hunk.OldRange.Start, hunk.OldRange.Count)
	entry.Anchor = sha256Hex(bytes.Join(needle, nil))
	if hunk.OldRange.Count > 0 {
		entry.Occurrences = len(lineOccurrences(records, needle))
	}
	return entry
}

// priorClaims returns, by current hunk, the rows of an earlier claims file at
// name whose claim compiles exactly onto that hunk, so a row the author
// dropped since becomes a tombstone, and the tombstones of every claim that
// matches or overlaps it (CKC-V0-003). An absent file
// has none; anything at name but a regular file, or an unreadable claims
// file, refuses.
func (p *proposal) priorClaims(name string, parsed *patch.Patch) map[*patch.Hunk][]claimRow {
	info, err := os.Lstat(name)
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	if err != nil || !info.Mode().IsRegular() {
		p.say("%s: REFUSE claims-unwritable\n  --claims %s exists and is not a regular file; nothing was written (CKC-V0-009)\n", p.prefix, strconv.Quote(name))
		p.exit(2)
	}
	data, err := readPrefix(name, maxClaimsBytes+1)
	var file *claimsFile
	why := "it could not be read"
	if err == nil {
		file, why = decodeClaims(data)
	}
	if file == nil {
		p.say("%s: REFUSE claims-unreadable\n  --claims %s is not a valid %s claims file: %s; move it aside to record afresh (CKC-V0-009)\n", p.prefix, strconv.Quote(name), claimsSchema, why)
		p.exit(2)
	}
	p.prefetchMovedBase(file)
	compiler := &claimsCompiler{recordedBase: file.Base, currentBase: p.base, blob: p.blob, records: map[string][][]byte{}}
	prior := map[*patch.Hunk][]claimRow{}
	for _, result := range compiler.compile(file.Claims, parsed.Hunks) {
		// Only an exactly matched claim's rows can have been dropped by the
		// author; a tombstone keys on cited text, so it needs no relocation
		// and stays on every hunk its claim matches or overlaps.
		if result.class == claimCarried {
			prior[result.hunks[0]] = append(prior[result.hunks[0]], result.rows...)
		}
		for _, hunk := range result.hunks {
			prior[hunk] = append(prior[hunk], result.claim.Tombstones...)
		}
	}
	return prior
}

// writeClaimsFile replaces name with data, mode 0600, through a temporary
// file in its directory and a rename (CKC-V0-001).
func writeClaimsFile(name string, data []byte) error {
	temporary, err := os.CreateTemp(filepath.Dir(name), ".claims-*.tmp")
	if err != nil {
		return err
	}
	cleanup := func(err error) error {
		_ = os.Remove(temporary.Name())
		return err
	}
	if err := temporary.Chmod(0o600); err != nil {
		_ = temporary.Close()
		return cleanup(err)
	}
	if _, err := temporary.Write(data); err != nil {
		_ = temporary.Close()
		return cleanup(err)
	}
	if err := temporary.Close(); err != nil {
		return cleanup(err)
	}
	if err := os.Rename(temporary.Name(), name); err != nil {
		return cleanup(err)
	}
	return nil
}
