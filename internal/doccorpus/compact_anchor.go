package doccorpus

import (
	"bytes"
	"encoding/json/jsontext"
	json "encoding/json/v2"
	"errors"
	"strconv"
)

// AdoptionProviderSchemaV2 is the compact adoption provider record (DCP-V1-487): a
// corvint-corpus-adoption-provider/1 record whose anchors name a per-record file table entry by
// index instead of repeating the file's identity, and may take their authority, evidence kind
// and reason from the record's anchor defaults. The build expands every compact anchor to the
// /1 anchor it stands for before any admission check, so the record is admitted exactly as the
// /1 record holding the expanded anchors.
const AdoptionProviderSchemaV2 = "corvint-corpus-adoption-provider/2"

// AnchorFile is one entry of a compact record's file table. An absent repository or revision is
// the record's source repository or revision; a present one, even empty, is kept.
type AnchorFile struct {
	Repository *string `json:"repository,omitempty"`
	Revision   *string `json:"revision,omitempty"`
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
	Authority  *string `json:"authority,omitempty"`
	Kind       *string `json:"evidence_kind,omitempty"`
	Reason     *string `json:"reason,omitempty"`
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
// reports it verbatim (DCP-V1-488).
type compactAnchorError string

func (e compactAnchorError) Error() string { return string(e) }

// decodeProviderRecord decodes one provider record or shard. Every schema but the compact
// adoption schema decodes exactly as before, in one pass. A compact record is expanded to the
// /1 adoption record it stands for (DCP-V1-487, DCP-V1-488).
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
