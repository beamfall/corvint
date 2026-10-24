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

// AnchorFile is one entry of a compact record's file table. An empty repository or revision is
// the record's source repository or revision.
type AnchorFile struct {
	Repository string `json:"repository,omitempty"`
	Revision   string `json:"revision,omitempty"`
	Path       string `json:"path"`
	Blob       string `json:"blob"`
	SHA256     string `json:"sha256"`
}

// AnchorDefaults are the authority, evidence kind and reason of every compact anchor that leaves
// that member empty.
type AnchorDefaults struct {
	Authority string `json:"authority,omitempty"`
	Kind      string `json:"evidence_kind,omitempty"`
	Reason    string `json:"reason,omitempty"`
}

// CompactAnchor is one anchor of a compact record: the file table index and the span, with the
// optional members of Anchor. File is required.
type CompactAnchor struct {
	File       *int   `json:"file"`
	Start      int    `json:"start_line"`
	End        int    `json:"end_line"`
	SpanSHA256 string `json:"span_sha256"`
	Symbol     string `json:"symbol,omitempty"`
	Region     string `json:"region,omitempty"`
	Authority  string `json:"authority,omitempty"`
	Kind       string `json:"evidence_kind,omitempty"`
	Reason     string `json:"reason,omitempty"`
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
	head, headErr := scanCompactHead(data)
	if head.Schema != AdoptionProviderSchemaV2 {
		return err
	}
	if headErr != nil {
		return fail("invalid closed JSON input: " + locateDecodeError(name, data, headErr))
	}
	if len(head.Files) > maxAnchorFiles {
		return fail("anchor file table bound exceeded")
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
		*a = Anchor{Repository: f.Repository, Revision: f.Revision, Path: f.Path, Blob: f.Blob, SHA256: f.SHA256,
			Start: c.Start, End: c.End, SpanSHA256: c.SpanSHA256, Symbol: c.Symbol, Region: c.Region,
			Authority: c.Authority, Kind: c.Kind, Reason: c.Reason}
		if a.Repository == "" {
			a.Repository = head.Source.ID
		}
		if a.Revision == "" {
			a.Revision = head.Source.Revision
		}
		if d := head.AnchorDefaults; d != nil {
			a.Authority = orDefault(a.Authority, d.Authority)
			a.Kind = orDefault(a.Kind, d.Kind)
			a.Reason = orDefault(a.Reason, d.Reason)
		}
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

// orDefault is value, or fallback when value is empty.
func orDefault(value, fallback string) string {
	if value == "" {
		return fallback
	}
	return value
}

// compactHead is the top-level members a compact record's anchors are expanded from.
type compactHead struct {
	Schema         string
	Source         Repository
	Files          []AnchorFile
	AnchorDefaults *AnchorDefaults
}

// scanCompactHead reads the schema, source, file table and anchor defaults of a record without
// decoding its other members, which it skips. The schema is reported even when a later member
// fails, so the caller can tell a malformed compact record from any other refused record.
func scanCompactHead(data []byte) (compactHead, error) {
	var head compactHead
	dec := jsontext.NewDecoder(bytes.NewReader(data))
	if tok, err := dec.ReadToken(); err != nil || tok.Kind() != '{' {
		return head, errors.Join(err, errors.New("provider record is not a JSON object"))
	}
	for dec.PeekKind() == '"' {
		tok, err := dec.ReadToken()
		if err != nil {
			return head, err
		}
		closed := json.RejectUnknownMembers(true)
		switch tok.String() {
		case "schema":
			err = json.UnmarshalDecode(dec, &head.Schema, closed)
		case "source":
			err = json.UnmarshalDecode(dec, &head.Source, closed)
		case "files":
			err = json.UnmarshalDecode(dec, &head.Files, closed)
		case "anchor_defaults":
			err = json.UnmarshalDecode(dec, &head.AnchorDefaults, closed)
		default:
			err = dec.SkipValue()
		}
		if err != nil {
			return head, err
		}
	}
	return head, nil
}
