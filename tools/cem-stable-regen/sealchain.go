package main

import (
	"bytes"
	"fmt"
	"strconv"
	"strings"
)

// sealChain appends the CHANGES.md section, then re-binds the packet manifest,
// the stable cells copy and amendment index, REPOSITORY-ENVELOPE.md, the stable
// manifest and every structural-tuple packet to the changed packet files. Both
// the additive and the replace mode end here, so they seal the same chain.
// packetChanged maps packet-relative names to new bytes; stableCellEdits are
// the splices for the stable cells copy beyond the amendment index.
func sealChain(t *tree, packetManifestOld []byte, packetChanged map[string][]byte, casesOld, casesNew []byte, stableCellEdits []splice, status string, section func(changesOld []byte, outside map[string][]byte) []byte) (string, string, error) {
	// CHANGES.md records the predecessor digest of every file the
	// regeneration replaces, inside and outside the packet.
	changesOld, err := t.read(packetDir + "/" + changesName)
	if err != nil {
		return "", "", err
	}
	outside := map[string][]byte{}
	tuples, err := t.tuples()
	if err != nil {
		return "", "", err
	}
	outsideNames := []string{stableDir + "/manifest.json", stableDir + "/" + cellsName, stableDir + "/" + envelopeMD}
	for _, dir := range tuples {
		outsideNames = append(outsideNames, dir+"/TUPLE.json", dir+"/manifest.json")
	}
	for _, name := range outsideNames {
		if outside[name], err = t.read(name); err != nil {
			return "", "", err
		}
	}
	changesNew := append(append([]byte(nil), changesOld...), section(changesOld, outside)...)
	t.set(packetDir+"/"+changesName, changesNew)
	packetChanged[changesName] = changesNew

	// Packet manifest.
	manifest, _ := t.parse(packetDir + "/manifest.json")
	edits, err := rebindManifest(manifest, packetChanged)
	if err != nil {
		return "", "", err
	}
	for key, value := range map[string]string{"predecessorPublicManifestSHA256": digest(packetManifestOld), "status": status} {
		node := manifest.root.get(key)
		if node == nil {
			return "", "", fmt.Errorf("packet manifest has no %s", key)
		}
		edits = append(edits, replaceValue(node, quote(value)))
	}
	packetManifestNew, err := applySplices(manifest.src, edits)
	if err != nil {
		return "", "", err
	}
	t.set(packetDir+"/manifest.json", packetManifestNew)
	oldSum, newSum := digest(packetManifestOld), digest(packetManifestNew)

	// Stable cells copy and amendment index, its prose and the stable manifest.
	cells, _ := t.parse(stableDir + "/" + cellsName)
	amendment := cells.root.get("amendment")
	index := amendment.get("fullResultCases")
	cellsNew, err := applySplices(cells.src, append([]splice{
		replaceValue(amendment.get("publicManifestSHA256"), quote(newSum)),
		replaceValue(index.get("sha256"), quote(digest(casesNew))),
		replaceValue(index.get("bytes"), strconv.Itoa(len(casesNew))),
	}, stableCellEdits...))
	if err != nil {
		return "", "", err
	}
	t.set(stableDir+"/"+cellsName, cellsNew)
	prose, err := t.read(stableDir + "/" + envelopeMD)
	if err != nil {
		return "", "", err
	}
	prose = bytes.ReplaceAll(bytes.ReplaceAll(prose, []byte(oldSum), []byte(newSum)), []byte(digest(casesOld)), []byte(digest(casesNew)))
	t.set(stableDir+"/"+envelopeMD, prose)
	stable, _ := t.parse(stableDir + "/manifest.json")
	// The stable manifest may also list the packet's files: re-bind each listed
	// changed packet file and the packet manifest under its relative prefix.
	stableChanged := map[string][]byte{cellsName: cellsNew, envelopeMD: prose}
	if schema, ok := packetChanged[schemaName]; ok {
		stableChanged[schemaName] = schema
	}
	inStable := map[string]bool{}
	for _, entry := range stable.root.get("files").elems {
		inStable[entry.get("path").text(stable.src)] = true
	}
	prefix := strings.TrimPrefix(packetDir, stableDir+"/") + "/"
	for name, b := range packetChanged {
		if inStable[prefix+name] {
			stableChanged[prefix+name] = b
		}
	}
	if inStable[prefix+"manifest.json"] {
		stableChanged[prefix+"manifest.json"] = packetManifestNew
	}
	edits, err = rebindManifest(stable, stableChanged)
	if err != nil {
		return "", "", err
	}
	edits = append(edits, replaceValue(stable.root.get("publicAmendmentManifestSHA256"), quote(newSum)))
	stableNew, err := applySplices(stable.src, edits)
	if err != nil {
		return "", "", err
	}
	t.set(stableDir+"/manifest.json", stableNew)

	// Every sealed structural-tuple packet.
	for _, dir := range tuples {
		tuple, err := t.parse(dir + "/TUPLE.json")
		if err != nil {
			return "", "", err
		}
		inherited := tuple.root.get("inherited")
		edits := []splice{replaceValue(inherited.get("manifestSha256"), quote(newSum))}
		if prior := inherited.get("predecessorManifestSha256"); prior != nil {
			edits = append(edits, replaceValue(prior, quote(oldSum)))
		} else {
			edit, err := insertMember(tuple.src, inherited, "manifestSha256", "predecessorManifestSha256", func(string) string { return quote(oldSum) })
			if err != nil {
				return "", "", fmt.Errorf("%s/TUPLE.json: %w", dir, err)
			}
			edits = append(edits, edit)
		}
		files := inherited.get("files")
		for i, name := range files.keys {
			if b, ok := packetChanged[name]; ok {
				edits = append(edits, replaceValue(files.elems[i], quote(digest(b))))
			}
		}
		tupleNew, err := applySplices(tuple.src, edits)
		if err != nil {
			return "", "", err
		}
		t.set(dir+"/TUPLE.json", tupleNew)
		tm, _ := t.parse(dir + "/manifest.json")
		edits, err = rebindManifest(tm, map[string][]byte{"TUPLE.json": tupleNew})
		if err != nil {
			return "", "", err
		}
		tmNew, err := applySplices(tm.src, edits)
		if err != nil {
			return "", "", err
		}
		t.set(dir+"/manifest.json", tmNew)
	}
	return oldSum, newSum, nil
}
