package main

import (
	"bytes"
	"compress/zlib"
	"errors"
	"os"
	"sort"
	"strconv"
	"strings"

	"github.com/Beamfall/corvint/internal/authoritystore"
	"github.com/Beamfall/corvint/internal/localauthority"
)

func evidenceFiles(in enrolledInput, handle string) (authoritystore.EvidenceManifest, map[string][]byte, error) {
	m := authoritystore.EvidenceManifest{Profile: authoritystore.EvidenceProfile, RootID: in.root.RootID, Epoch: in.root.Epoch, Generation: in.root.Generation, EnrollmentHandle: handle, CapsuleSHA256: digest(in.raw["objects.json"]), Base: in.enrollment.Binding.Base, Target: in.enrollment.Binding.Target, Tree: in.enrollment.Binding.Tree, ReaderGID: in.root.AuthorityGID}
	if len(in.capsule.Objects) == 0 {
		return m, nil, errors.New("capsule unavailable")
	}
	m.ObjectFormat = evidenceFormat(in.capsule)
	files := map[string][]byte{".git/HEAD": []byte(m.Target + "\n"), ".git/config": evidenceConfiguration(m.ObjectFormat)}
	for _, object := range in.capsule.Objects {
		header, body, ok := bytes.Cut(object.Data, []byte{0})
		if !ok {
			return m, nil, errors.New("object header")
		}
		kind, _, ok := strings.Cut(string(header), " ")
		if !ok {
			return m, nil, errors.New("object kind")
		}
		var compressed bytes.Buffer
		writer := zlib.NewWriter(&compressed)
		if _, err := writer.Write(object.Data); err != nil {
			return m, nil, err
		}
		if err := writer.Close(); err != nil {
			return m, nil, err
		}
		p := ".git/objects/" + object.OID[:2] + "/" + object.OID[2:]
		raw := compressed.Bytes()
		files[p] = raw
		m.Files = append(m.Files, authoritystore.EvidenceFile{Path: p, Size: strconv.Itoa(len(raw)), SHA256: digest(raw), OID: object.OID, Kind: kind, DecodedSize: strconv.Itoa(len(body))})
	}
	for _, p := range []string{".git/HEAD", ".git/config"} {
		raw := files[p]
		m.Files = append(m.Files, authoritystore.EvidenceFile{Path: p, Size: strconv.Itoa(len(raw)), SHA256: digest(raw)})
	}
	sort.Slice(m.Files, func(i, j int) bool { return m.Files[i].Path < m.Files[j].Path })
	if err := authoritystore.ValidateEvidenceManifest(m); err != nil {
		return m, nil, err
	}
	return m, files, nil
}

func publishEvidence(in enrolledInput, handle string) (authoritystore.EvidenceReference, error) {
	var ref authoritystore.EvidenceReference
	owner, e := strconv.ParseUint(in.root.AuthorityUID, 10, 32)
	if e != nil || uint32(os.Geteuid()) != uint32(owner) {
		return ref, errors.New("evidence owner")
	}
	gid, e := strconv.ParseUint(in.root.AuthorityGID, 10, 32)
	if e != nil || uint32(os.Getegid()) != uint32(gid) {
		return ref, errors.New("evidence group")
	}
	parent, e := authoritystore.OpenEvidencePublicationParent(uint32(owner), uint32(gid))
	if e != nil {
		return ref, e
	}
	defer parent.Close()
	manifest, files, e := evidenceFiles(in, handle)
	if e != nil {
		return ref, e
	}
	raw, e := localauthority.Canonical(manifest)
	if e != nil {
		return ref, e
	}
	if e = stageAndPublishEvidence(parent, handle, manifest, files, raw, uint32(owner), uint32(gid)); e != nil {
		return ref, e
	}
	return authoritystore.EvidenceReference{Profile: authoritystore.EvidenceReferenceProfile, EnrollmentHandle: handle, ManifestSHA256: digest(raw)}, nil
}
