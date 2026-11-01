package dogfoodflow

import (
	"slices"
	"sort"
	"strconv"
	"strings"

	"github.com/Beamfall/corvint/internal/cem/patch"
)

// prefetchNamesPerRead bounds the pathspecs one ls-tree read names, so the
// argument list stays far below any platform limit.
const prefetchNamesPerRead = 256

// prefetchBlobs fills the draft's blob cache for every name at commit with one
// ls-tree read per prefetchNamesPerRead names and one cat-file --batch read
// per output bound, in place of the two git processes blob spends on each
// (V1-4095). It caches exactly what blob would: a regular file's bytes, or nil
// for a name that is absent or not a regular file. A read that fails or
// overflows its bound caches nothing, and a blob too large for one bounded
// read is left uncached, so blob reads those as before.
func (p *proposal) prefetchBlobs(commit string, names []string) {
	wanted := []string{}
	for _, name := range names {
		if _, cached := p.blobs[commit+":"+name]; !cached && name != "" {
			wanted = append(wanted, name)
		}
	}
	sort.Strings(wanted)
	wanted = slices.Compact(wanted)
	type entry struct {
		name string
		size int
	}
	objects := map[string][]entry{}
	order := []string{}
	for start := 0; start < len(wanted); start += prefetchNamesPerRead {
		chunk := wanted[start:min(start+prefetchNamesPerRead, len(wanted))]
		listed, status, bounded := p.boundedGit(append([]string{"--literal-pathspecs", "ls-tree", "-z", "-l", "--full-tree", commit, "--"}, chunk...)...)
		if status != 0 || !bounded {
			continue
		}
		notBlob := map[string]bool{}
		for _, record := range strings.Split(listed, "\x00") {
			meta, name, ok := strings.Cut(record, "\t")
			fields := strings.Fields(meta)
			if !ok || len(fields) != 4 {
				continue
			}
			notBlob[name] = true
			size, err := strconv.Atoi(fields[3])
			if !strings.HasPrefix(fields[0], "100") || fields[1] != "blob" || err != nil {
				continue
			}
			if _, seen := objects[fields[2]]; !seen {
				order = append(order, fields[2])
			}
			objects[fields[2]] = append(objects[fields[2]], entry{name, size})
			notBlob[name] = false
		}
		for _, name := range chunk {
			// ls-tree printed nothing (absent) or a non-blob entry for it.
			if other, listed := notBlob[name]; !listed || other {
				p.blobs[commit+":"+name] = nil
			}
		}
	}
	limit := p.gitOutputLimit
	if limit <= 0 {
		limit = maxGitOutputBytes
	}
	for len(order) > 0 {
		batch, total := []string{}, 0
		for len(order) > 0 {
			cost := objects[order[0]][0].size + len(order[0]) + 32
			if cost > limit {
				order = order[1:]
				continue
			}
			if total+cost > limit {
				break
			}
			batch, total, order = append(batch, order[0]), total+cost, order[1:]
		}
		if len(batch) == 0 {
			break
		}
		command := p.gitCommand(true, "cat-file", "--batch")
		command.Stdin = strings.NewReader(strings.Join(batch, "\n") + "\n")
		p.checkpoint()
		output, status, bounded := p.readGit(command)
		if status != 0 || !bounded {
			continue
		}
		for _, oid := range batch {
			header, rest, ok := strings.Cut(output, "\n")
			fields := strings.Fields(header)
			if !ok || len(fields) != 3 || fields[0] != oid || fields[1] != "blob" {
				break
			}
			size, err := strconv.Atoi(fields[2])
			if err != nil || size < 0 || len(rest) < size+1 || rest[size] != '\n' {
				break
			}
			data := []byte(rest[:size])
			for _, named := range objects[oid] {
				p.blobs[commit+":"+named.name] = data
			}
			output = rest[size+1:]
		}
	}
}

// hunkPaths names each parsed hunk's display path.
func hunkPaths(hunks []*patch.Hunk) []string {
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
