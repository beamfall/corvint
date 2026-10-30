package similar

import (
	"bufio"
	"fmt"
	"io"
	"regexp"
	"strconv"
	"strings"
)

// DigestSchema heads a text digest (TCL-V0-005).
const DigestSchema = "corvint-ticket-digest/0"

// DigestEntry is one digest line.
type DigestEntry struct {
	ID          string
	Title       string
	Recurrences int
}

// Digest is a parsed text digest.
type Digest struct {
	Tree    string
	Entries []DigestEntry
}

var digestHeader = regexp.MustCompile(`^# corvint-ticket-digest/0 tree ([0-9a-f]{64}) tickets ([0-9]+)$`)
var recurrenceSuffix = regexp.MustCompile(` \[recurrences ([0-9]+)\]$`)

// CleanTitle replaces control characters, so a title stays on one line.
func CleanTitle(title string) string {
	return strings.Map(func(r rune) rune {
		if r < 0x20 || r == 0x7f {
			return ' '
		}
		return r
	}, title)
}

// WriteDigest writes the text digest form of entries; each note is a `# `
// comment line after the header, which ParseDigest skips.
func WriteDigest(w io.Writer, tree string, entries []DigestEntry, notes ...string) error {
	bw := bufio.NewWriter(w)
	fmt.Fprintf(bw, "# %s tree %s tickets %d\n", DigestSchema, tree, len(entries))
	for _, n := range notes {
		fmt.Fprintf(bw, "# %s\n", CleanTitle(n))
	}
	for _, e := range entries {
		fmt.Fprintf(bw, "%s\t%s", e.ID, CleanTitle(e.Title))
		if e.Recurrences > 0 {
			fmt.Fprintf(bw, " [recurrences %d]", e.Recurrences)
		}
		bw.WriteString("\n")
	}
	return bw.Flush()
}

// ParseDigest reads a text digest; a missing or wrong header, a bad line
// or a count mismatch is an error.
func ParseDigest(r io.Reader) (Digest, error) {
	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 64*1024), 1<<20)
	if !sc.Scan() {
		if err := sc.Err(); err != nil {
			return Digest{}, err
		}
		return Digest{}, fmt.Errorf("digest is empty; want a %q header", "# "+DigestSchema)
	}
	head := digestHeader.FindStringSubmatch(sc.Text())
	if head == nil {
		return Digest{}, fmt.Errorf("digest header must be `# %s tree <sha256> tickets <count>`", DigestSchema)
	}
	want, _ := strconv.Atoi(head[2])
	d := Digest{Tree: head[1]}
	line := 1
	for sc.Scan() {
		line++
		text := sc.Text()
		if text == "" || strings.HasPrefix(text, "# ") {
			continue
		}
		id, title, ok := strings.Cut(text, "\t")
		if !ok || id == "" {
			return Digest{}, fmt.Errorf("digest line %d must be `ID<TAB>title`", line)
		}
		e := DigestEntry{ID: id, Title: title}
		if m := recurrenceSuffix.FindStringSubmatchIndex(title); m != nil {
			e.Recurrences, _ = strconv.Atoi(title[m[2]:m[3]])
			e.Title = title[:m[0]]
		}
		d.Entries = append(d.Entries, e)
	}
	if err := sc.Err(); err != nil {
		return Digest{}, err
	}
	if len(d.Entries) != want {
		return Digest{}, fmt.Errorf("digest header counts %d tickets but %d lines follow", want, len(d.Entries))
	}
	return d, nil
}

// Docs are the digest's entries as title-only docs.
func (d Digest) Docs() []Doc {
	docs := make([]Doc, len(d.Entries))
	for i, e := range d.Entries {
		docs[i] = Doc{ID: e.ID, Title: e.Title, Status: "OPEN", Recurrences: e.Recurrences}
	}
	return docs
}
