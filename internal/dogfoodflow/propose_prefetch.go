package dogfoodflow

import (
	"bytes"
	"strconv"
	"strings"

	"github.com/Beamfall/corvint/internal/cem/patch"
)

// Tests disable prefetching to compare the former per-file path byte for byte.
var prefetchBlobReads = true

// prefetchRefreshBlobs batches the plan's evidence before carry, relocation and
// vet read it. These paths need not occur in the changed hunks or anchor search.
// CKC-V0-012: use the same bounded reader on Git without %(objectmode).
func (p *proposal) prefetchRefreshBlobs() {
	paths := []string{}
	for _, row := range p.refreshRows {
		fields := strings.Split(row.rest, "\t")
		if len(fields) == 3 && fields[0] != "unknown" {
			paths = append(paths, fields[0])
		}
	}
	p.prefetchBlobs(p.base, paths)
	p.prefetchBlobs(p.target, paths)
	p.prefetchBlobs(p.priorBase, paths)
}

// cacheBlobBatch publishes only a complete, exactly framed batch. A missing,
// truncated, duplicate or unexpected object leaves every requested blob for
// the ordinary reader to classify; no successful prefix can hide a bad tail.
func (p *proposal) cacheBlobBatch(commit string, data []byte, byObject map[string][]string) {
	decoded := map[string][]byte{}
	for len(data) > 0 {
		header, rest, ok := bytes.Cut(data, []byte{'\n'})
		fields := strings.Fields(string(header))
		if !ok || len(fields) != 3 || fields[1] != "blob" || byObject[fields[0]] == nil {
			return
		}
		if _, duplicate := decoded[fields[0]]; duplicate {
			return
		}
		size, err := strconv.Atoi(fields[2])
		if err != nil || size < 0 || size >= len(rest) || rest[size] != '\n' {
			return
		}
		decoded[fields[0]] = rest[:size:size]
		data = rest[size+1:]
	}
	if len(decoded) != len(byObject) {
		return
	}
	for oid, blob := range decoded {
		for _, name := range byObject[oid] {
			p.blobs[commit+":"+name] = blob
		}
	}
}

// patchHunkPaths names each parsed hunk display path, the BASE read set the
// draft prefetches through prefetchBlobs before it walks the hunks (V1-4095).
func patchHunkPaths(hunks []*patch.Hunk) []string {
	paths := make([]string, 0, len(hunks))
	for _, hunk := range hunks {
		paths = append(paths, hunk.DisplayPath)
	}
	return paths
}

// matchedPaths names every path a candidate span line is on.
func matchedPaths(matches map[string][]spanLine) []string {
	paths := []string{}
	for _, found := range matches {
		for _, line := range found {
			paths = append(paths, line.path)
		}
	}
	return paths
}
