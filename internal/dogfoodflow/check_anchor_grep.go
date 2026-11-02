package dogfoodflow

import (
	"bufio"
	"bytes"
	"io"
	"sort"
	"strconv"
	"strings"
)

// anchorGrep runs the prepareAnchors read of BASE. A draft keeps its one
// `git grep -o`. The check's unanchorable search (DCW-V0-165) reads the same
// matches in two passes instead (DCW-V0-984): `git grep -l` lists the BASE
// files that hold a pattern, one `git cat-file --batch` reads those blobs, and
// each occurrence of a pattern prints one record in the form `git grep -z -n -o`
// prints, `BASE:path NUL line NUL pattern`. `-o` searches the rest of a line
// again for every pattern after each match, so long one-line documents with
// many matches made that one grep 41 s of a 45 s check binding on 2026-11-01
// (492 patterns). Every line holding a pattern still yields a record, and a
// pattern inside an earlier, overlapping match is no longer missed, so the
// candidates are a superset of the `-o` ones, each still re-checked by
// verify.SpanAnchors. A record set over the flow's output limit, a blob over it or a
// failed read reports bounded false, as an oversized `-o` read did.
func (p *proposal) anchorGrep(arguments, patterns []string) (string, int, bool) {
	if !p.basenames {
		return p.boundedGit(arguments...)
	}
	listing := []string{}
	for _, argument := range arguments {
		switch argument {
		case "-n", "-o":
			continue
		case "grep":
			listing = append(listing, argument, "-l")
			continue
		}
		listing = append(listing, argument)
	}
	listed, status, bounded := p.boundedGit(listing...)
	if status != 0 || !bounded {
		return "", status, bounded
	}
	names := []string{}
	for _, name := range strings.Split(listed, "\x00") {
		if name == "" {
			continue
		}
		name, named := strings.CutPrefix(name, p.base+":")
		if !named || strings.ContainsAny(name, "\n\r") {
			return "", 0, false
		}
		names = append(names, name)
	}
	if len(names) == 0 {
		return "", 1, true
	}
	records, ok := p.anchorRecords(names, patterns)
	if !ok {
		return "", 0, false
	}
	return records, 0, true
}

// anchorRecords reads each named BASE blob with one `git cat-file --batch` and
// returns the records of every pattern occurrence, in path order and then
// offset order.
func (p *proposal) anchorRecords(names, patterns []string) (string, bool) {
	var request bytes.Buffer
	for _, name := range names {
		request.WriteString(p.base + ":" + name + "\n")
	}
	command := p.gitCommand(true, "cat-file", "--batch")
	command.Stdin = &request
	stdout, err := command.StdoutPipe()
	if err != nil {
		return "", false
	}
	p.checkpoint()
	if command.Start() != nil {
		return "", false
	}
	limit := p.gitOutputLimit
	if limit <= 0 {
		limit = maxGitOutputBytes
	}
	reader := bufio.NewReader(stdout)
	var records strings.Builder
	complete := true
	for _, name := range names {
		blob, read := readBatchBlob(reader, limit)
		if !read {
			complete = false
			break
		}
		if !appendAnchorRecords(&records, p.base+":"+name, blob, patterns, limit) {
			complete = false
			break
		}
	}
	_, _ = io.Copy(io.Discard, reader)
	if command.Wait() != nil {
		complete = false
	}
	p.checkpoint()
	return records.String(), complete
}

// readBatchBlob reads one `cat-file --batch` blob of at most limit bytes.
func readBatchBlob(reader *bufio.Reader, limit int) ([]byte, bool) {
	header, err := reader.ReadString('\n')
	if err != nil {
		return nil, false
	}
	fields := strings.Fields(header)
	if len(fields) != 3 || fields[1] != "blob" {
		return nil, false
	}
	size, err := strconv.Atoi(fields[2])
	if err != nil || size < 0 || size > limit {
		return nil, false
	}
	blob := make([]byte, size+1)
	if _, err := io.ReadFull(reader, blob); err != nil || blob[size] != '\n' {
		return nil, false
	}
	return blob[:size], true
}

// appendAnchorRecords appends a record for each occurrence of each pattern in
// blob, ordered by offset, and reports false once the records pass limit.
func appendAnchorRecords(records *strings.Builder, object string, blob []byte, patterns []string, limit int) bool {
	type hit struct {
		offset  int
		pattern string
	}
	hits := []hit{}
	for _, pattern := range patterns {
		needle := []byte(pattern)
		if len(needle) == 0 {
			continue
		}
		for from := 0; ; {
			index := bytes.Index(blob[from:], needle)
			if index < 0 {
				break
			}
			hits = append(hits, hit{from + index, pattern})
			from += index + len(needle)
		}
	}
	sort.SliceStable(hits, func(i, j int) bool { return hits[i].offset < hits[j].offset })
	line, counted := 1, 0
	for _, found := range hits {
		line += bytes.Count(blob[counted:found.offset], []byte{'\n'})
		counted = found.offset
		records.WriteString(object + "\x00" + strconv.Itoa(line) + "\x00" + found.pattern + "\n")
		if records.Len() > limit {
			return false
		}
	}
	return true
}
