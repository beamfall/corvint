package pathbrief

import (
	"cmp"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"regexp"
	"slices"
	"strings"

	"github.com/Beamfall/corvint/internal/gokernel"
	"github.com/Beamfall/corvint/internal/specindex"
)

// ExportStats is the record of one export run that TBC-V1-012 puts in the build log.
type ExportStats struct {
	InputCommit  string `json:"inputCommit"`
	OutputSHA256 string `json:"outputSha256"`
	OutputBytes  int    `json:"outputBytes"`
	// RecordsRead counts the sealed records the links come from.
	RecordsRead int `json:"recordsRead"`
	// RecordsSkipped counts files under the record directory that are not a readable sealed
	// record: another name, not JSON, another protocol, or a record commit the repository lacks.
	RecordsSkipped int `json:"recordsSkipped"`
	Links          int `json:"links"`
	// LinksWithoutRequirement counts entries whose span holds no requirement definition.
	LinksWithoutRequirement int `json:"linksWithoutRequirement"`
	// LinksPathAbsentAtInput counts entries whose path is not a file at the input commit.
	LinksPathAbsentAtInput int `json:"linksPathAbsentAtInput"`
	// LinksDropped counts links left out because the basis names no evidence entry of its record,
	// or the pinned blob is not in the repository or is shorter than the span.
	LinksDropped int `json:"linksDropped"`
	Paths        int `json:"paths"`
	Evidence     int `json:"evidence"`
}

// recordDirectory holds the sealed change records the export reads.
const recordDirectory = ".corvint/changes"

var recordName = regexp.MustCompile(`^` + regexp.QuoteMeta(recordDirectory) + `/([0-9a-f]{40})\.cem\.json$`)

// requirementID is the identifier shape of the requirement index generator.
var requirementID = regexp.MustCompile(`\b[A-Z][A-Z0-9-]*-[0-9]{3,}\b`)

type sealedRecord struct {
	Spec  string `json:"spec"`
	Hunks []struct {
		Path  string `json:"path"`
		Basis []struct {
			EvidenceID string `json:"evidenceId"`
			Relation   string `json:"relation"`
		} `json:"basis"`
	} `json:"hunks"`
	Evidence []struct {
		ID      string `json:"id"`
		Path    string `json:"path"`
		BlobOid string `json:"blobOid"`
		Span    *struct {
			Start int `json:"start"`
			End   int `json:"end"`
		} `json:"span"`
	} `json:"evidence"`
}

type evidenceKey struct {
	file       string
	start, end int
	blobOid    string
}

type linkKey struct {
	path, commit string
	evidence     evidenceKey
}

// Export reads the sealed `cem/1.0` records at one commit of the repository at root and returns
// the bytes of the requirement-link file (TBC-V1-012). It writes nothing. The same input commit
// gives the same bytes.
func Export(ctx context.Context, root, revision string) ([]byte, ExportStats, error) {
	r := repo{ctx: ctx, root: root}
	stats := ExportStats{}
	input, err := r.resolve(revision)
	if err != nil {
		return nil, stats, errors.New("the input revision is not a commit of the repository")
	}
	stats.InputCommit = input
	listed, err := r.tree(input, recordDirectory)
	if err != nil {
		return nil, stats, err
	}
	var shas []string
	recordBlob := map[string]string{}
	for name, oid := range listed {
		match := recordName.FindStringSubmatch(name)
		if match == nil {
			stats.RecordsSkipped++
			continue
		}
		shas = append(shas, match[1])
		recordBlob[match[1]] = oid
	}
	slices.Sort(shas)
	commitNames := make([]string, len(shas))
	for i, sha := range shas {
		commitNames[i] = sha + "^{commit}"
	}
	present, err := r.objects(commitNames)
	if err != nil {
		return nil, stats, err
	}
	links := map[linkKey]bool{}
	var commits []string
	for i, sha := range shas {
		record := sealedRecord{}
		data, ok := r.blob(recordBlob[sha])
		if present[i] != sha || !ok || json.Unmarshal(data, &record) != nil || record.Spec != "cem/1.0" {
			stats.RecordsSkipped++
			continue
		}
		stats.RecordsRead++
		commits = append(commits, sha)
		evidence := map[string]evidenceKey{}
		for _, entry := range record.Evidence {
			if entry.ID == "" || entry.Path == "" || entry.Span == nil || entry.Span.Start < 0 ||
				entry.Span.End < entry.Span.Start || !validOID(entry.BlobOid) {
				continue
			}
			evidence[entry.ID] = evidenceKey{file: entry.Path, start: entry.Span.Start, end: entry.Span.End, blobOid: entry.BlobOid}
		}
		for _, hunk := range record.Hunks {
			for _, basis := range hunk.Basis {
				if basis.Relation != "specification" && basis.Relation != "decision" {
					continue
				}
				key, ok := evidence[basis.EvidenceID]
				if !ok || hunk.Path == "" || strings.ContainsAny(hunk.Path, "\n\r") {
					stats.LinksDropped++
					continue
				}
				links[linkKey{path: hunk.Path, commit: sha, evidence: key}] = true
			}
		}
	}
	times, err := r.committerTimes(commits)
	if err != nil {
		return nil, stats, err
	}

	// The requirement IDs of each distinct span, read at its pinned blob.
	requirements := map[evidenceKey][]string{}
	definitions := map[string]*blobDefinitions{}
	for key := range links {
		if _, done := requirements[key.evidence]; done {
			continue
		}
		blob, seen := definitions[key.evidence.blobOid]
		if !seen {
			blob = readDefinitions(r, key.evidence.blobOid)
			definitions[key.evidence.blobOid] = blob
		}
		requirements[key.evidence] = blob.inside(key.evidence.start, key.evidence.end)
	}
	ordered := make([]linkKey, 0, len(links))
	for key := range links {
		if requirements[key.evidence] == nil {
			stats.LinksDropped++
			continue
		}
		ordered = append(ordered, key)
	}
	slices.SortFunc(ordered, func(a, b linkKey) int {
		return cmp.Or(cmp.Compare(a.path, b.path), cmp.Compare(a.commit, b.commit), compareEvidence(a.evidence, b.evidence))
	})

	file := LinkFile{Schema: LinksSchema, InputCommit: input, Commits: []LinkCommit{}, Evidence: []LinkEvidence{}, Paths: []LinkPath{}}
	commitIndex := map[string]int{}
	for _, sha := range commits {
		commitIndex[sha] = len(file.Commits)
		file.Commits = append(file.Commits, LinkCommit{SHA: sha, CommitterTime: times[sha]})
	}
	evidenceKeys := make([]evidenceKey, 0, len(requirements))
	for key, ids := range requirements {
		if ids != nil {
			evidenceKeys = append(evidenceKeys, key)
		}
	}
	slices.SortFunc(evidenceKeys, compareEvidence)
	evidenceIndex := map[evidenceKey]int{}
	for _, key := range evidenceKeys {
		evidenceIndex[key] = len(file.Evidence)
		file.Evidence = append(file.Evidence, LinkEvidence{File: key.file, Start: key.start, End: key.end,
			BlobOid: key.blobOid, Requirements: requirements[key]})
	}

	atInput, err := r.tree(input)
	if err != nil {
		return nil, stats, err
	}
	var blobNames []string
	for i, key := range ordered {
		if i == 0 || ordered[i-1].path != key.path || ordered[i-1].commit != key.commit {
			blobNames = append(blobNames, key.commit+":"+key.path)
		}
	}
	blobs, err := r.objects(blobNames)
	if err != nil {
		return nil, stats, err
	}
	next := 0
	for i, key := range ordered {
		newPath := i == 0 || ordered[i-1].path != key.path
		if newPath {
			file.Paths = append(file.Paths, LinkPath{Path: key.path})
		}
		entry := &file.Paths[len(file.Paths)-1]
		if newPath || ordered[i-1].commit != key.commit {
			entry.Records = append(entry.Records, LinkRecord{Commit: commitIndex[key.commit], Blob: blobs[next]})
			next++
		}
		record := &entry.Records[len(entry.Records)-1]
		record.Evidence = append(record.Evidence, evidenceIndex[key.evidence])
		stats.Links++
		if len(requirements[key.evidence]) == 0 {
			stats.LinksWithoutRequirement++
		}
		if _, tracked := atInput[key.path]; !tracked {
			stats.LinksPathAbsentAtInput++
		}
	}
	stats.Paths, stats.Evidence = len(file.Paths), len(file.Evidence)

	data, err := gokernel.CanonicalJSON(file)
	if err != nil {
		return nil, stats, err
	}
	data = append(data, '\n')
	sum := sha256.Sum256(data)
	stats.OutputSHA256, stats.OutputBytes = hex.EncodeToString(sum[:]), len(data)
	return data, stats, nil
}

func compareEvidence(a, b evidenceKey) int {
	return cmp.Or(cmp.Compare(a.file, b.file), cmp.Compare(a.start, b.start), cmp.Compare(a.end, b.end), cmp.Compare(a.blobOid, b.blobOid))
}

// blobDefinitions is one pinned blob scanned for requirement definition lines.
type blobDefinitions struct {
	text        string
	definitions *specindex.Definitions
	lineStart   []int
}

func readDefinitions(r repo, oid string) *blobDefinitions {
	data, ok := r.blob(oid)
	if !ok {
		return nil
	}
	text := string(data)
	blob := &blobDefinitions{text: text, definitions: specindex.NewDefinitions(text)}
	offset := 0
	for _, line := range strings.SplitAfter(text, "\n") {
		blob.lineStart = append(blob.lineStart, offset)
		offset += len(line)
	}
	return blob
}

// inside returns, in ascending order, every requirement ID whose definition begins inside the
// byte span: the ID stands in the span on the line the index generator takes as its definition.
// It returns nil when the blob is unreadable or shorter than the span, and an empty list when the
// span defines nothing.
func (b *blobDefinitions) inside(start, end int) []string {
	if b == nil || end > len(b.text) {
		return nil
	}
	ids := []string{}
	for _, id := range requirementID.FindAllString(b.text[start:end], -1) {
		if slices.Contains(ids, id) {
			continue
		}
		line := b.definitions.Line(id)
		if line == 0 {
			continue
		}
		from := b.lineStart[line-1]
		at := from + strings.Index(b.text[from:], id)
		if at >= start && at+len(id) <= end {
			ids = append(ids, id)
		}
	}
	slices.Sort(ids)
	return ids
}
