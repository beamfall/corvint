package doccorpus

import (
	"bytes"
	"encoding/json/jsontext"
	json "encoding/json/v2"
	"errors"
	"strconv"
)

// AdoptionProviderSchemaV2 is the compact adoption provider record (DCX-V1-487): a
// corvint-corpus-adoption-provider/1 record whose anchors name a per-record file table entry by
// index instead of repeating the file's identity, and may take their authority, evidence kind
// and reason from the record's anchor defaults. The build expands every compact anchor to the
// /1 anchor it stands for before any admission check, so the record is admitted exactly as the
// /1 record holding the expanded anchors.
const AdoptionProviderSchemaV2 = "corvint-corpus-adoption-provider/2"

// AnchorFile is one entry of a compact record's file table. An absent repository or revision is
// the record's source repository or revision; a present one, even empty, is kept.
type AnchorFile struct {
	Repository *string `json:"repository,omitzero"`
	Revision   *string `json:"revision,omitzero"`
	Path       string  `json:"path"`
	Blob       string  `json:"blob"`
	SHA256     string  `json:"sha256"`
}

// AnchorDefaults are the authority, evidence kind and reason of every compact anchor that omits
// that member.
type AnchorDefaults struct {
	Authority string `json:"authority,omitempty"`
	Kind      string `json:"evidence_kind,omitempty"`
	Reason    string `json:"reason,omitempty"`
}

// CompactAnchor is one anchor of a compact record: the file table index and the span, with the
// optional members of Anchor. File is required; an absent authority, kind or reason is the
// record's default, and a present one, even empty, is kept.
type CompactAnchor struct {
	File       *int    `json:"file"`
	Start      int     `json:"start_line"`
	End        int     `json:"end_line"`
	SpanSHA256 string  `json:"span_sha256"`
	Symbol     string  `json:"symbol,omitempty"`
	Region     string  `json:"region,omitempty"`
	Authority  *string `json:"authority,omitzero"`
	Kind       *string `json:"evidence_kind,omitzero"`
	Reason     *string `json:"reason,omitzero"`
}

// compactRecord is the closed wire form of a compact record: a ProviderRecord plus its file
// table and anchor defaults, which no other provider schema admits.
type compactRecord struct {
	ProviderRecord `json:",inline"`
	Files          []AnchorFile    `json:"files"`
	AnchorDefaults *AnchorDefaults `json:"anchor_defaults,omitempty"`
}

// maxAnchorFiles bounds one compact record's file table.
const maxAnchorFiles = MaxCorpusRecords

// compactAnchorError is the reason a compact anchor cannot be expanded; a located refusal
// reports it verbatim (DCX-V1-488).
type compactAnchorError string

func (e compactAnchorError) Error() string { return string(e) }

// decodeProviderRecord decodes one provider record or shard. Every schema but the compact
// adoption schema decodes exactly as before, in one pass. A compact record is expanded to the
// /1 adoption record it stands for (DCX-V1-487, DCX-V1-488).
func decodeProviderRecord(name string, data []byte, record *ProviderRecord) error {
	err := decodeNamed(name, data, record, MaxProviderBytes)
	if err == nil && record.Schema != AdoptionProviderSchemaV2 || len(data) > MaxProviderBytes {
		return err
	}
	// A compact record with any anchor fails the closed /1 decode at its first "file" member.
	var schema string
	if scanTopLevel(data, func(member string, dec *jsontext.Decoder) error {
		if member == "schema" {
			return json.UnmarshalDecode(dec, &schema)
		}
		return dec.SkipValue()
	}) != nil || schema != AdoptionProviderSchemaV2 {
		return err
	}
	var head compactHead
	closed := json.RejectUnknownMembers(true)
	if err := scanTopLevel(data, func(member string, dec *jsontext.Decoder) error {
		switch member {
		case "source":
			return json.UnmarshalDecode(dec, &head.Source, closed)
		case "files":
			return json.UnmarshalDecode(dec, &head.Files, closed)
		case "anchor_defaults":
			return json.UnmarshalDecode(dec, &head.AnchorDefaults, closed)
		}
		return dec.SkipValue()
	}); err != nil {
		return fail("invalid closed JSON input: " + locateDecodeError(name, data, err))
	}
	if len(head.Files) > maxAnchorFiles {
		return fail("invalid closed JSON input: " + boundedText(name, 1024) + ": /files: anchor file table bound exceeded")
	}
	expand := json.UnmarshalFromFunc(func(dec *jsontext.Decoder, a *Anchor) error {
		var c CompactAnchor
		if err := json.UnmarshalDecode(dec, &c); err != nil {
			return err
		}
		if c.File == nil {
			return compactAnchorError("compact anchor lacks its file index")
		}
		if *c.File < 0 || *c.File >= len(head.Files) {
			return compactAnchorError("compact anchor file index " + strconv.Itoa(*c.File) + " is outside the file table of " + strconv.Itoa(len(head.Files)) + " entries")
		}
		f := head.Files[*c.File]
		var d AnchorDefaults
		if head.AnchorDefaults != nil {
			d = *head.AnchorDefaults
		}
		*a = Anchor{Repository: orDefault(f.Repository, head.Source.ID), Revision: orDefault(f.Revision, head.Source.Revision),
			Path: f.Path, Blob: f.Blob, SHA256: f.SHA256, Start: c.Start, End: c.End, SpanSHA256: c.SpanSHA256,
			Symbol: c.Symbol, Region: c.Region, Authority: orDefault(c.Authority, d.Authority),
			Kind: orDefault(c.Kind, d.Kind), Reason: orDefault(c.Reason, d.Reason)}
		return nil
	})
	var wire compactRecord
	if err := decodeNamed(name, data, &wire, MaxProviderBytes, json.WithUnmarshalers(expand)); err != nil {
		return err
	}
	*record = wire.ProviderRecord
	record.Schema = AdoptionProviderSchema
	return nil
}

// orDefault is *value when present, else fallback.
func orDefault(value *string, fallback string) string {
	if value == nil {
		return fallback
	}
	return *value
}

// compactHead is the top-level members a compact record's anchors are expanded from.
type compactHead struct {
	Source         Repository
	Files          []AnchorFile
	AnchorDefaults *AnchorDefaults
}

// scanTopLevel calls visit with each top-level member name of a JSON object record, positioned at
// its value, which visit must consume. Errors keep the decoder's absolute JSON pointer.
func scanTopLevel(data []byte, visit func(member string, dec *jsontext.Decoder) error) error {
	dec := jsontext.NewDecoder(bytes.NewReader(data))
	if tok, err := dec.ReadToken(); err != nil || tok.Kind() != '{' {
		return errors.Join(err, errors.New("provider record is not a JSON object"))
	}
	for dec.PeekKind() == '"' {
		tok, err := dec.ReadToken()
		if err != nil {
			return err
		}
		if err = visit(tok.String(), dec); err != nil {
			return err
		}
	}
	return nil
}

// EncodeCompactProvider writes record, a corvint-corpus-adoption-provider/1 record, as the
// compact /2 record that expands to it (DCX-V1-530): its anchors index a file table built in
// first-appearance order, a file omits its repository and revision when they are the record's
// source ones, and an anchor omits its authority, kind and reason when they equal defaults
// (every member is written when defaults is nil).
func EncodeCompactProvider(record ProviderRecord, defaults *AnchorDefaults) ([]byte, error) {
	if record.Schema != AdoptionProviderSchema {
		return nil, fail("compact encoding needs a " + AdoptionProviderSchema + " record")
	}
	type fileKey struct{ repository, revision, path, blob, sha256 string }
	index := map[fileKey]int{}
	files := []AnchorFile{}
	unlessEqual := func(value, implied string) *string {
		if value == implied {
			return nil
		}
		return &value
	}
	var d AnchorDefaults
	if defaults != nil {
		d = *defaults
	}
	compact := json.MarshalToFunc(func(enc *jsontext.Encoder, a Anchor) error {
		key := fileKey{a.Repository, a.Revision, a.Path, a.Blob, a.SHA256}
		i, ok := index[key]
		if !ok {
			if len(files) == maxAnchorFiles {
				return errors.New("anchor file table bound exceeded")
			}
			i = len(files)
			index[key] = i
			files = append(files, AnchorFile{Repository: unlessEqual(a.Repository, record.Source.ID), Revision: unlessEqual(a.Revision, record.Source.Revision), Path: a.Path, Blob: a.Blob, SHA256: a.SHA256})
		}
		c := CompactAnchor{File: &i, Start: a.Start, End: a.End, SpanSHA256: a.SpanSHA256, Symbol: a.Symbol, Region: a.Region,
			Authority: &a.Authority, Kind: &a.Kind, Reason: &a.Reason}
		if defaults != nil {
			c.Authority, c.Kind, c.Reason = unlessEqual(a.Authority, d.Authority), unlessEqual(a.Kind, d.Kind), unlessEqual(a.Reason, d.Reason)
		}
		return json.MarshalEncode(enc, c)
	})
	options := json.JoinOptions(json.Deterministic(true), json.WithMarshalers(compact))
	// The first pass fills the file table; the second writes the record with it.
	if _, err := json.Marshal(record, options); err != nil {
		return nil, fail("compact encoding failed: " + boundedText(err.Error(), 256))
	}
	record.Schema = AdoptionProviderSchemaV2
	raw, err := json.Marshal(compactRecord{ProviderRecord: record, Files: files, AnchorDefaults: defaults}, options)
	if err != nil {
		return nil, fail("compact encoding failed: " + boundedText(err.Error(), 256))
	}
	return append(raw, '\n'), nil
}

// CompactProvider is docs corpus compact-provider (DCX-V1-531): it rewrites the /1 adoption
// provider record or shard data, read from the input name, as its compact /2 encoding, with the most frequent anchor
// authority, kind and reason as the defaults (ties to the smallest value), and refuses unless
// the result decodes back to exactly the record data holds.
func CompactProvider(name string, data []byte) ([]byte, error) {
	var record ProviderRecord
	if err := decodeProviderRecord(name, data, &record); err != nil {
		return nil, err
	}
	var schema string
	if scanTopLevel(data, func(member string, dec *jsontext.Decoder) error {
		if member == "schema" {
			return json.UnmarshalDecode(dec, &schema)
		}
		return dec.SkipValue()
	}) != nil || schema != AdoptionProviderSchema {
		return nil, fail("compact-provider input must be a " + AdoptionProviderSchema + " record")
	}
	defaults, err := frequentAnchorDefaults(record)
	if err != nil {
		return nil, err
	}
	raw, err := EncodeCompactProvider(record, defaults)
	if err != nil {
		return nil, err
	}
	if len(raw) > MaxProviderBytes {
		return nil, fail("output bound exceeded")
	}
	var expanded ProviderRecord
	if err := decodeProviderRecord(name, raw, &expanded); err != nil {
		return nil, fail("compact record does not expand to the input record")
	}
	want, err := json.Marshal(record, json.Deterministic(true))
	if err != nil {
		return nil, err
	}
	got, err := json.Marshal(expanded, json.Deterministic(true))
	if err != nil || !bytes.Equal(got, want) {
		return nil, fail("compact record does not expand to the input record")
	}
	return raw, nil
}

// frequentAnchorDefaults is the most frequent authority, kind and reason over every anchor of
// record, each tie going to the smallest value, or nil when record has no anchor.
func frequentAnchorDefaults(record ProviderRecord) (*AnchorDefaults, error) {
	counts := [3]map[string]int{{}, {}, {}}
	anchors := 0
	count := json.MarshalToFunc(func(enc *jsontext.Encoder, a Anchor) error {
		anchors++
		for i, v := range [3]string{a.Authority, a.Kind, a.Reason} {
			counts[i][v]++
		}
		return enc.WriteToken(jsontext.Null)
	})
	if _, err := json.Marshal(record, json.WithMarshalers(count)); err != nil {
		return nil, fail("compact encoding failed: " + boundedText(err.Error(), 256))
	}
	if anchors == 0 {
		return nil, nil
	}
	var top [3]string
	for i, values := range counts {
		best := -1
		for v, n := range values {
			if n > best || n == best && v < top[i] {
				top[i], best = v, n
			}
		}
	}
	return &AnchorDefaults{Authority: top[0], Kind: top[1], Reason: top[2]}, nil
}
