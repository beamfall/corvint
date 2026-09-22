package frontier

import (
	"bytes"
	"reflect"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/wp3codec"
)

// FuzzVerifyDocumentAgreesWithSeal checks the independent-consumer verifier
// (CF-V0-028) against the producer's sealing path on arbitrary bytes. A
// document VerifyDocument accepts must be exactly its own canonical encoding.
// Any bytes the closed decoder admits are resealed with a recomputed identity,
// so the fuzzer explores past the content hash: when seal completes such a
// document, VerifyDocument must accept the sealed bytes and decode them back
// to the sealed document.
func FuzzVerifyDocumentAgreesWithSeal(f *testing.F) {
	zero := strings.Repeat("0", 64)
	document := Document{
		FrontierState: StateEmpty,
		ID:            documentIDPrefix + zero,
		Inputs:        Inputs{CEMSHA256: zero, LRFSHA256: zero, OCMSHA256: zero, TCQID: "tcq:sha256:" + zero, TestMode: TestModeStatic},
		Items:         []Item{},
		Scope: Scope{BaseRevision: zero[:40], IntentBlobOID: zero[:40], IntentPath: "docs/intent.md", IntentSpan: Span{Start: 0, End: 512},
			IntentSpanSHA256: zero, ObjectFormat: "sha1", PatchSHA256: zero, TargetRevision: zero[:40]},
		UniverseID: universeIDPrefix + zero,
	}
	document.UniverseID, _ = UniverseID(document.Scope)
	for _, sealed := range []Document{document, openSeedDocument(document)} {
		_, encoded, err := seal(sealed)
		if err != nil {
			f.Fatal(err)
		}
		f.Add(encoded)
	}
	f.Fuzz(func(t *testing.T, data []byte) {
		root, parseErr := wp3codec.Parse(bytes.TrimSuffix(data, []byte{'\n'}))
		verified := VerifyDocument(data) == nil
		if parseErr != nil {
			if verified {
				t.Fatalf("verified %q that the codec cannot parse", data)
			}
			return
		}
		decoded, decodeErr := decodeDocument(root)
		if decodeErr != nil {
			if verified {
				t.Fatalf("verified %q that the closed decoder refuses: %v", data, decodeErr)
			}
			return
		}
		if verified {
			if encoded, err := CanonicalBytes(decoded); err != nil || !bytes.Equal(encoded, data) {
				t.Fatalf("verified %q whose canonical encoding is %q (%v)", data, encoded, err)
			}
		}
		sealedDocument, sealed, sealErr := seal(decoded)
		if sealErr != nil {
			return
		}
		if err := VerifyDocument(sealed); err != nil {
			t.Fatalf("seal produced %q that does not verify: %v", sealed, err)
		}
		reparsed, err := wp3codec.Parse(sealed[:len(sealed)-1])
		if err != nil {
			t.Fatalf("sealed %q does not reparse: %v", sealed, err)
		}
		if again, err := decodeDocument(reparsed); err != nil || !reflect.DeepEqual(again, sealedDocument) {
			t.Fatalf("sealed %q decodes as %+v (%v), want %+v", sealed, again, err, sealedDocument)
		}
	})
}

// openSeedDocument adds one item of every kind, each carrying its first
// admitted reason and the disposition that reason fixes.
func openSeedDocument(document Document) Document {
	document.FrontierState = StateOpen
	document.Items = []Item{}
	for _, kind := range []string{KindHunkBasis, KindIntentChange, KindIntentTest} {
		reason := reasonOrders[kind][0]
		chosen := reasonDispositions[kind][reason]
		document.Items = append(document.Items, Item{
			AuthorityClass: expectedItemAuthority(kind, reason), Kind: kind,
			NextAction: chosen.action, Reasons: []string{reason}, RelatedIDs: []string{"a", "b"},
			ResolutionClass: chosen.resolution, SubjectID: "subject",
		})
		item := &document.Items[len(document.Items)-1]
		item.ID, _ = ItemID(item.Kind, item.SubjectID, document.UniverseID)
	}
	return document
}
