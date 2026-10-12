package main

import (
	"encoding/json"
	"regexp"
)

// Stable map provenance and role (CEM-V1-093, CEM-V1-095, decision 0525):
// optional root members validated after every required member. Neither is
// read for authority, ranking or any axis, nor echoed into the result.

type stableProvenance struct {
	SourceSpec    string `json:"sourceSpec"`
	SourceBlobOid string `json:"sourceBlobOid"`
	SourceSha256  string `json:"sourceSha256"`
	BindCommit    string `json:"bindCommit"`
	Converter     string `json:"converter"`
}

var stableConverter = regexp.MustCompile(`^corvint-cem-convert/[0-9]+(\.[0-9]+)*$`)

func validSHA1OID(s string) bool { return len(s) == 40 && lowerHex(s) }

func decodeStableProvenanceRole(root map[string]json.RawMessage) (*stableProvenance, string, *cemError) {
	var p *stableProvenance
	if raw, ok := root["provenance"]; ok {
		o, e := object03(raw, []string{"sourceSpec", "sourceBlobOid", "sourceSha256", "bindCommit", "converter"}, nil)
		if e != nil {
			return nil, "", e
		}
		p = &stableProvenance{}
		if p.SourceSpec, e = stableString(o, "sourceSpec", func(s string) bool { return s == "cem/0.1" || s == "cem/0.2" || s == "cem/0.3" }); e != nil {
			return nil, "", e
		}
		if p.SourceBlobOid, e = stableString(o, "sourceBlobOid", validSHA1OID); e != nil {
			return nil, "", e
		}
		if p.SourceSha256, e = stableString(o, "sourceSha256", validDigest); e != nil {
			return nil, "", e
		}
		if p.BindCommit, e = stableString(o, "bindCommit", validSHA1OID); e != nil {
			return nil, "", e
		}
		if p.Converter, e = stableString(o, "converter", stableConverter.MatchString); e != nil {
			return nil, "", e
		}
	}
	role := ""
	if _, ok := root["role"]; ok {
		r, e := stableString(root, "role", func(s string) bool { return s == "change" || s == "bind-chore" || s == "seal-chore" })
		if e != nil {
			return nil, "", e
		}
		role = r
	}
	return p, role, nil
}
