package corpusindex

import (
	"bytes"
	"context"
	stdjson "encoding/json"
	json "encoding/json/v2"
	"fmt"
	"github.com/Beamfall/corvint/internal/doccorpus"
	"sort"
)

const RecordingSchema = "corvint-corpus-question-recording/1"

type Question struct {
	ID      string            `json:"id"`
	Tool    string            `json:"tool"`
	Request doccorpus.Request `json:"request"`
	Answer  doccorpus.Receipt `json:"answer"`
}
type Recording struct {
	Schema    string     `json:"schema"`
	Questions []Question `json:"questions"`
}
type Difference struct {
	ID                string   `json:"id"`
	Tool              string   `json:"tool"`
	State             string   `json:"state"`
	Missing           []string `json:"missing"`
	Extra             []string `json:"extra"`
	Changed           []string `json:"changed"`
	SemanticAgreement bool     `json:"semantic_agreement"`
	Failure           string   `json:"failure,omitempty"`
}
type Counts struct {
	Agreement int `json:"agreement"`
	Different int `json:"different"`
	Refused   int `json:"refused"`
}
type ParityReport struct {
	Schema          string            `json:"schema"`
	IndexSHA256     string            `json:"index_sha256"`
	RecordingSHA256 string            `json:"recording_sha256"`
	Questions       []Difference      `json:"questions"`
	Tools           map[string]Counts `json:"tools"`
	Limitations     []string          `json:"limitations"`
}

func Compare(ctx context.Context, r *Reader, raw []byte) (ParityReport, error) {
	report := ParityReport{Schema: "corvint-corpus-switch-parity/1", IndexSHA256: r.Digest(), RecordingSHA256: doccorpus.Digest(raw), Questions: []Difference{}, Tools: map[string]Counts{}, Limitations: []string{"strict complete semantic receipt comparison; no transport normalization", "agreement covers supplied recordings only; no authority to retire the previous server"}}
	var recording Recording
	if len(raw) > doccorpus.MaxCorpusBytes || json.Unmarshal(raw, &recording, json.RejectUnknownMembers(true)) != nil || recording.Schema != RecordingSchema || len(recording.Questions) > 4096 {
		return report, fmt.Errorf("invalid bounded question recording")
	}
	var original map[string]any
	decoder := stdjson.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	if e := decoder.Decode(&original); e != nil {
		return report, e
	}
	originalAnswers := map[string]any{}
	rows, ok := original["questions"].([]any)
	if !ok {
		return report, fmt.Errorf("question list required")
	}
	for _, value := range rows {
		row, ok := value.(map[string]any)
		if !ok {
			return report, fmt.Errorf("question object required")
		}
		id, _ := row["id"].(string)
		originalAnswers[id] = row["answer"]
	}
	seen := map[string]bool{}
	for _, question := range recording.Questions {
		if ctx.Err() != nil {
			return report, ctx.Err()
		}
		if question.ID == "" || len(question.ID) > 1024 || seen[question.ID] || question.Tool == "" || len(question.Tool) > 256 {
			return report, fmt.Errorf("duplicate question or invalid tool identifier")
		}
		seen[question.ID] = true
		input, e := Encode(question.Request)
		if e != nil {
			return report, e
		}
		q, e := ParseRequest(input)
		if e != nil {
			return report, e
		}
		d := Difference{ID: question.ID, Tool: question.Tool, State: "different", Missing: []string{}, Extra: []string{}, Changed: []string{}}
		actual, e := r.Query(ctx, q)
		counts := report.Tools[d.Tool]
		if e != nil {
			d.State = "refused"
			d.Failure = e.Error()
			counts.Refused++
		} else {
			want, e := canonicalSemantic(originalAnswers[question.ID])
			if e != nil {
				return report, e
			}
			got, e := canonicalSemantic(actual)
			if e != nil {
				return report, e
			}
			d.SemanticAgreement = bytes.Equal(want, got)
			left, e := answerRecords(want)
			if e != nil {
				return report, e
			}
			right, e := answerRecords(got)
			if e != nil {
				return report, e
			}
			for id, b := range left {
				if other, ok := right[id]; !ok {
					d.Missing = append(d.Missing, id)
				} else if !bytes.Equal(b, other) {
					d.Changed = append(d.Changed, id)
				}
			}
			for id := range right {
				if _, ok := left[id]; !ok {
					d.Extra = append(d.Extra, id)
				}
			}
			sort.Strings(d.Missing)
			sort.Strings(d.Extra)
			sort.Strings(d.Changed)
			if d.SemanticAgreement {
				d.State = "agreement"
				counts.Agreement++
			} else {
				counts.Different++
			}
		}
		report.Tools[d.Tool] = counts
		report.Questions = append(report.Questions, d)
	}
	sort.Slice(report.Questions, func(i, j int) bool { return report.Questions[i].ID < report.Questions[j].ID })
	return report, nil
}

// Object member order and Go concrete types are not semantic differences.
// UseNumber prevents canonical comparison from rounding recorded large integers.
func canonicalSemantic(v any) ([]byte, error) {
	b, e := Encode(v)
	if e != nil {
		return nil, e
	}
	decoder := stdjson.NewDecoder(bytes.NewReader(b))
	decoder.UseNumber()
	var value any
	if e := decoder.Decode(&value); e != nil {
		return nil, e
	}
	return stdjson.Marshal(value)
}
func answerRecords(raw []byte) (map[string][]byte, error) {
	out := map[string][]byte{}
	var fields map[string]any
	decoder := stdjson.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	if e := decoder.Decode(&fields); e != nil {
		return nil, e
	}
	rows, _ := fields["results"].([]any)
	buckets := map[string][][]byte{}
	strictIDs := map[string]bool{}
	for _, v := range rows {
		row, ok := v.(map[string]any)
		id := ""
		strict := false
		if ok {
			id, _ = row["id"].(string)
			strict = id != ""
			if rel, ok := row["relation"].(map[string]any); ok {
				rid, _ := rel["id"].(string)
				direction, _ := row["direction"].(string)
				if rid != "" {
					id = "dependency:" + direction + ":" + rid
					strict = true
				}
			}
			if link, ok := row["link"].(map[string]any); ok {
				rid, _ := link["id"].(string)
				if rid != "" {
					id = "observation:" + rid
					strict = true
				}
			}
			if id == "" {
				subject, _ := row["subject"].(string)
				kind, _ := row["kind"].(string)
				if subject != "" && kind != "" {
					id = "gap:" + subject + ":" + kind
				}
			}
			if id == "" {
				metric, _ := row["metric"].(string)
				if metric != "" {
					id = "metric:" + metric
				}
			}
			if id == "" {
				capability, _ := row["capability"].(string)
				if capability != "" {
					id = "capability:" + capability
				}
			}
		}
		b, e := stdjson.Marshal(v)
		if e != nil {
			return nil, e
		}
		if id == "" {
			id = "anonymous-sha256:" + doccorpus.Digest(b)
		}
		buckets[id] = append(buckets[id], b)
		strictIDs[id] = strictIDs[id] || strict
	}
	for id, values := range buckets {
		if strictIDs[id] {
			if len(values) != 1 {
				return nil, fmt.Errorf("duplicate answer record identity: %s", id)
			}
			out[id] = values[0]
			continue
		}
		// Weak records keep the same keys when another identical occurrence appears.
		occurrences := map[string]int{}
		for _, b := range values {
			digest := doccorpus.Digest(b)
			occurrences[digest]++
			key := fmt.Sprintf("%s:%s:%06d", id, digest, occurrences[digest])
			out[key] = b
		}
	}
	return out, nil
}
