// Package similar ranks tickets against new text for the Tasks duplicate
// check (docs/specs/corvint-tasks-cli-v0.md, TCL-V0-001 to TCL-V0-009). It
// is the context compiler's lexical BM25 applied to four ticket fields; it
// reads and writes nothing itself.
package similar

import (
	"fmt"
	"regexp"
	"sort"
	"strings"
)

// Field names, in evidence order.
const (
	FieldID     = "id"
	FieldTitle  = "title"
	FieldBody   = "body"
	FieldLabels = "labels"
	FieldPaths  = "paths"
)

// TitleGain is the title field's weight over an unnormalised field credit.
const TitleGain = 2.0

// IDGain is the ID field's weight: text that names a ticket points at it.
const IDGain = 4.0

// Duplicate threshold of the create guard (TCL-V0-004).
const (
	DuplicateSimilarity = 0.60
	DuplicateMinTerms   = 3
)

// Doc is one ticket as the ranking sees it.
type Doc struct {
	ID     string
	Title  string
	Status string
	// Body is the ticket body and acceptance criteria, as one text.
	Body   string
	Labels []string
	Paths  []string
	// Recurrences is the count of `Recurrence YYYY-MM-DD: ` body lines.
	Recurrences int
}

// Match is one query term a candidate matched.
type Match struct {
	Term   string
	Fields []string
	IDF    float64
	TF     int
}

// Candidate is one ranked ticket.
type Candidate struct {
	Doc        *Doc
	Score      float64
	Similarity float64
	Matches    []Match
}

// Duplicate reports whether c reaches the create guard's threshold.
func (c Candidate) Duplicate() bool {
	return c.Similarity >= DuplicateSimilarity && len(c.Matches) >= DuplicateMinTerms && c.Anchored()
}

// Anchored reports whether a matched query term is in the candidate's title
// or ID: a duplicate shares what the item is, not only body vocabulary
// (TCL-V0-990).
func (c Candidate) Anchored() bool {
	for _, m := range c.Matches {
		for _, f := range m.Fields {
			if f == FieldTitle || f == FieldID {
				return true
			}
		}
	}
	return false
}

// Reason is the evidence line in the context compiler's form.
func (c Candidate) Reason() string {
	if len(c.Matches) == 0 {
		return "no query term matched"
	}
	return fmt.Sprintf("%d distinct query terms; rarest `%s` (idf %.2f); bm25 %.2f; similarity %.2f", len(c.Matches), c.Matches[0].Term, c.Matches[0].IDF, c.Score, c.Similarity)
}

type posting struct {
	doc    int32
	fields uint8 // bit per field: id, title, body, labels, paths
	bodyTF uint32
}

// Index is the inverted index of a ticket set.
type Index struct {
	docs     []Doc
	lengths  []int
	average  float64
	postings map[string][]posting
}

const (
	bitID = 1 << iota
	bitTitle
	bitBody
	bitLabels
	bitPaths
)

// NewIndex indexes docs; the slice is kept, not copied.
func NewIndex(docs []Doc) *Index {
	x := &Index{docs: docs, lengths: make([]int, len(docs)), postings: map[string][]posting{}}
	terms := newFieldTerms()
	fields := map[string]*posting{}
	slots := make([]posting, 0, 256)
	total := 0
	for i := range docs {
		d := &docs[i]
		clear(fields)
		slots = slots[:0]
		mark := func(bit uint8) func(string, uint32) {
			return func(term string, tf uint32) {
				p := fields[term]
				if p == nil {
					if len(slots) == cap(slots) {
						// Growing would move the slots the map points at.
						slots = make([]posting, 0, 2*cap(slots))
					}
					slots = append(slots, posting{doc: int32(i)})
					p = &slots[len(slots)-1]
					fields[term] = p
				}
				p.fields |= bit
				if bit == bitBody {
					p.bodyTF = tf
					x.lengths[i] += int(tf)
				}
			}
		}
		terms.each(d.ID, mark(bitID))
		terms.each(d.Title, mark(bitTitle))
		terms.each(d.Body, mark(bitBody))
		for _, label := range d.Labels {
			terms.each(label, mark(bitLabels))
		}
		for _, path := range d.Paths {
			terms.each(path, mark(bitPaths))
		}
		total += x.lengths[i]
		for term, p := range fields {
			x.postings[term] = append(x.postings[term], *p)
		}
	}
	x.average = 1
	if len(docs) > 0 && total > 0 {
		x.average = float64(total) / float64(len(docs))
	}
	return x
}

// Len is the number of indexed docs.
func (x *Index) Len() int { return len(x.docs) }

// Filter admits a doc into a ranking; nil admits every doc.
type Filter func(*Doc) bool

// Query is the text a ranking matches: a new ticket's title and body, or
// free text as Title alone.
type Query struct {
	Title string
	Body  string
}

// QueryTitleGain is a query title term's weight over a body term: the title
// says what the item is, the body mostly how it was found.
const QueryTitleGain = 2.0

// MaxBodyTerms bounds the query body terms to the rarest ones, so a long
// body does not rank long tickets by sheer vocabulary overlap.
const MaxBodyTerms = 24

type queryTerm struct {
	term string
	gain float64
	idf  float64
	list []posting
}

// terms are the title terms, then the MaxBodyTerms rarest other body terms
// (idf descending, then term), each with its postings.
func (x *Index) terms(q Query) []queryTerm {
	term := func(t string, gain float64) queryTerm {
		list := x.postings[t]
		return queryTerm{term: t, gain: gain, idf: idf(len(x.docs), len(list)), list: list}
	}
	var out []queryTerm
	seen := map[string]bool{}
	for _, t := range queryTerms(q.Title) {
		seen[t] = true
		out = append(out, term(t, QueryTitleGain))
	}
	var body []queryTerm
	for _, t := range queryTerms(q.Body) {
		if !seen[t] {
			body = append(body, term(t, 1))
		}
	}
	sort.SliceStable(body, func(i, j int) bool { return body[i].idf > body[j].idf })
	if len(body) > MaxBodyTerms {
		body = body[:MaxBodyTerms]
	}
	return append(out, body...)
}

// Rank returns at most limit candidates for free text, as RankQuery.
func (x *Index) Rank(text string, filter Filter, limit int) []Candidate {
	return x.RankQuery(Query{Title: text}, filter, limit)
}

// RankQuery returns at most limit candidates for query, score descending then
// ID ascending; a doc that matches no query term is never a candidate.
func (x *Index) RankQuery(query Query, filter Filter, limit int) []Candidate {
	terms := x.terms(query)
	type acc struct {
		score   float64
		matched float64
		matches []Match
	}
	byDoc := map[int32]*acc{}
	mass := 0.0
	for _, qt := range terms {
		term, weight := qt.term, qt.idf
		mass += qt.gain * weight
		for _, p := range qt.list {
			d := &x.docs[p.doc]
			if filter != nil && !filter(d) {
				continue
			}
			a := byDoc[p.doc]
			if a == nil {
				a = &acc{}
				byDoc[p.doc] = a
			}
			m := Match{Term: term, IDF: weight, TF: int(p.bodyTF)}
			if p.fields&bitID != 0 {
				a.score += qt.gain * IDGain * fieldScore(weight)
				m.Fields = append(m.Fields, FieldID)
			}
			if p.fields&bitTitle != 0 {
				a.score += qt.gain * TitleGain * fieldScore(weight)
				m.Fields = append(m.Fields, FieldTitle)
			}
			if p.fields&bitBody != 0 {
				a.score += qt.gain * bodyScore(weight, float64(p.bodyTF), float64(x.lengths[p.doc]), x.average)
				m.Fields = append(m.Fields, FieldBody)
			}
			if p.fields&bitLabels != 0 {
				a.score += qt.gain * fieldScore(weight)
				m.Fields = append(m.Fields, FieldLabels)
			}
			if p.fields&bitPaths != 0 {
				a.score += qt.gain * fieldScore(weight)
				m.Fields = append(m.Fields, FieldPaths)
			}
			a.matched += qt.gain * weight
			a.matches = append(a.matches, m)
		}
	}
	out := make([]Candidate, 0, len(byDoc))
	for doc, a := range byDoc {
		sort.SliceStable(a.matches, func(i, j int) bool {
			if a.matches[i].IDF != a.matches[j].IDF {
				return a.matches[i].IDF > a.matches[j].IDF
			}
			return a.matches[i].Term < a.matches[j].Term
		})
		c := Candidate{Doc: &x.docs[doc], Score: a.score, Matches: a.matches}
		if mass > 0 {
			c.Similarity = a.matched / mass
		}
		out = append(out, c)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Score != out[j].Score {
			return out[i].Score > out[j].Score
		}
		return out[i].Doc.ID < out[j].Doc.ID
	})
	if limit >= 0 && len(out) > limit {
		out = out[:limit]
	}
	return out
}

var recurrenceLine = regexp.MustCompile(`(?m)^Recurrence [0-9]{4}-[0-9]{2}-[0-9]{2}: `)

// Recurrences counts body lines that begin `Recurrence YYYY-MM-DD: `
// (TCL-V0-008).
func Recurrences(body string) int { return len(recurrenceLine.FindAllStringIndex(body, -1)) }

// RecurrenceLine is the body line `ticket recur` appends (TCL-V0-007).
func RecurrenceLine(date, context string) string { return "Recurrence " + date + ": " + context }

// HasRecurrence reports whether body has a recurrence line for context on
// any date.
func HasRecurrence(body, context string) bool {
	for _, line := range strings.Split(body, "\n") {
		if recurrenceLine.MatchString(line) && strings.HasSuffix(line, ": "+context) && len(line) == len("Recurrence 2006-01-02: ")+len(context) {
			return true
		}
	}
	return false
}

// ValidContext reports why context cannot be a recurrence line, or "".
func ValidContext(context string) string {
	if strings.TrimSpace(context) == "" {
		return "--context must be non-empty"
	}
	if strings.ContainsAny(context, "\r\n") {
		return "--context must be one line"
	}
	return ""
}
