package workqueue

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
	"testing"
)

// Rehash the actual raw shape, including malformed nested objects, independently
// of the production typed writers. Never restore a field deliberately removed.
func repairHash(kind, profile string, body any) string {
	raw := append([]byte(kind+"\x00"+profile+"\x00"), canonicalAny(body)...)
	digest := sha256.Sum256(raw)
	return hex.EncodeToString(digest[:])
}
func repairBind(object map[string]any, field, kind, profile string) {
	if _, ok := object[field].(string); !ok {
		return
	}
	body := make(map[string]any, len(object))
	for key, value := range object {
		if key != field {
			body[key] = value
		}
	}
	object[field] = kind + ":sha256:" + repairHash(kind, profile, body)
}
func repairRehash(root map[string]any) {
	if details, ok := root["details"].([]any); ok {
		for _, value := range details {
			detail, ok := value.(map[string]any)
			if !ok {
				continue
			}
			if _, ok := detail["payloadSha256"].(string); ok {
				detail["payloadSha256"] = repairHash("work-queue-detail-payload", "work-queue-detail-payload/0", detail["payload"])
			}
			repairBind(detail, "detailId", "work-queue-detail-record", "work-queue-detail-record/0")
		}
	}
	if source, ok := root["repositorySource"].(map[string]any); ok {
		repairBind(source, "id", "repository-source", "repository-source/0")
	}
	profile, _ := root["profile"].(string)
	kinds := map[string]string{PolicyProfile: "work-queue-policy", DetailsProfile: "work-queue-details", CheckpointProfile: "work-queue-checkpoint"}
	kind := kinds[profile]
	if kind == "" { // A changed profile still receives a hash in its fixture's identity domain.
		id, _ := root["id"].(string)
		kind = strings.Split(id, ":")[0]
		profile = map[string]string{"work-queue-policy": PolicyProfile, "work-queue-details": DetailsProfile, "work-queue-checkpoint": CheckpointProfile}[kind]
	}
	repairBind(root, "id", kind, profile)
}
func repairRoot(raw []byte) map[string]any {
	var result map[string]any
	if err := json.Unmarshal(raw, &result); err != nil {
		panic(err)
	}
	return result
}
func repairWire(root map[string]any) []byte {
	repairRehash(root)
	return append(canonicalAny(root), '\n')
}
func repairPolicy() *Policy {
	p := &Policy{AccessContextID: "access:corvint:local", AdapterPath: "script/queue", AdapterProfile: "repository-work-queue-adapter/0", DetailLimit: "1", MappingVersion: "v1", Operations: PolicyOperations{Details: []string{"z", "a"}, Snapshot: []string{"snapshot"}, Verify: []string{"verify"}}, Profile: PolicyProfile, QueueAuthorityID: testQueue, RepositoryAuthorityID: testRepository, ScopeID: "scope:corvint:worklist"}
	p.RefreshIdentity()
	return p
}
func repairDetails() *DetailsDocument {
	d := Detail{Payload: DetailPayload{AcceptanceCriteria: []string{}, EvidenceHandles: []string{}}, RepositoryAuthorityID: testRepository, TicketID: "ticket:corvint:worklist:a", TicketVersionID: "ticket-version:sha256:" + testZeroDigest}
	RefreshDetail(&d)
	document := &DetailsDocument{Details: []Detail{d}, SnapshotID: "work-queue-snapshot:sha256:" + testZeroDigest}
	RefreshDetails(document)
	return document
}
func repairCheckpoint() *CheckpointDocument {
	s := testSnapshot()
	d := &CheckpointDocument{Checkpoint: s.Checkpoint, PolicyID: s.PolicyID, RepositorySource: s.RepositorySource, SnapshotID: s.ID}
	RefreshCheckpoint(d)
	return d
}
func repairError(t *testing.T, err error, code string) {
	t.Helper()
	e, ok := err.(*Error)
	if !ok || e.Code != code {
		t.Fatalf("want %s, got %v", code, err)
	}
}

func TestProtocolRequiredShapeRepair(t *testing.T) {
	t.Run("WQO-V0-001", func(t *testing.T) {
		fixtures := []struct {
			name  string
			raw   []byte
			parse func([]byte) error
			paths []string
		}{
			{"policy", repairPolicy().Canonical(), func(raw []byte) error { _, e := ParsePolicy(raw); return e }, []string{"", "operations"}},
			{"details", repairDetails().Canonical(), func(raw []byte) error { _, e := ParseDetails(raw); return e }, []string{"", "details/0", "details/0/payload"}},
			{"checkpoint", repairCheckpoint().Canonical(), func(raw []byte) error { _, e := ParseCheckpoint(raw); return e }, []string{"", "checkpoint", "repositorySource"}},
		}
		for _, fixture := range fixtures {
			t.Run(fixture.name, func(t *testing.T) {
				if err := fixture.parse(fixture.raw); err != nil {
					t.Fatal(err)
				}
				for _, path := range fixture.paths {
					original := repairObjectAt(repairRoot(fixture.raw), path)
					for key, value := range original {
						for _, mode := range []string{"missing", "null", "wrong-kind"} {
							if mode == "null" && path == "details/0/payload" && (key == "body" || key == "title" || key == "owner" || key == "displayKey") {
								continue
							}
							t.Run(path+"/"+key+"/"+mode, func(t *testing.T) {
								root := repairRoot(fixture.raw)
								object := repairObjectAt(root, path)
								switch mode {
								case "missing":
									delete(object, key)
								case "null":
									object[key] = nil
								default:
									if _, ok := value.(map[string]any); ok {
										object[key] = []any{}
									} else {
										object[key] = map[string]any{}
									}
								}
								repairError(t, fixture.parse(repairWire(root)), CodeMalformedInput)
							})
						}
					}
					t.Run(path+"/unknown", func(t *testing.T) {
						root := repairRoot(fixture.raw)
						repairObjectAt(root, path)["unknown"] = true
						repairError(t, fixture.parse(repairWire(root)), CodeMalformedInput)
					})
				}
			})
		}
	})
}
func repairObjectAt(root map[string]any, path string) map[string]any {
	var value any = root
	if path == "" {
		return root
	}
	for _, part := range strings.Split(path, "/") {
		if part == "0" {
			value = value.([]any)[0]
		} else {
			value = value.(map[string]any)[part]
		}
	}
	return value.(map[string]any)
}

func TestProtocolHostileRepair(t *testing.T) {
	t.Run("WQO-V0-027", func(t *testing.T) {
		for _, field := range []string{"body", "title", "acceptanceCriteria", "displayKey", "owner", "evidenceHandles"} {
			for _, r := range []rune{0, 1, 0x7f, 0x85, 0xfeff, 0x061c, 0x200e, 0x200f, 0x202a, 0x202b, 0x202c, 0x202d, 0x202e, 0x2066, 0x2067, 0x2068, 0x2069} {
				t.Run(fmt.Sprintf("%s/U+%04X", field, r), func(t *testing.T) {
					root := repairRoot(repairDetails().Canonical())
					payload := repairObjectAt(root, "details/0/payload")
					text := "secret" + string(r) + "value"
					payload[field] = text
					if field == "acceptanceCriteria" || field == "evidenceHandles" {
						payload[field] = []any{text}
					}
					_, err := ParseDetails(repairWire(root))
					repairError(t, err, "HOSTILE_INPUT")
					if strings.Contains(err.Error(), text) {
						t.Fatal("hostile input leaked")
					}
				})
			}
		}
		for _, field := range []string{"body", "displayKey"} {
			t.Run(field+"/limit", func(t *testing.T) {
				root := repairRoot(repairDetails().Canonical())
				limit := 256 << 10
				if field == "displayKey" {
					limit = 32 << 10
				}
				repairObjectAt(root, "details/0/payload")[field] = strings.Repeat("x", limit+1)
				_, err := ParseDetails(repairWire(root))
				repairError(t, err, CodeInputLimit)
			})
		}
		t.Run("identifier", func(t *testing.T) {
			p := repairPolicy()
			p.MappingVersion = "secret\u202evalue"
			p.RefreshIdentity()
			_, err := ParsePolicy(p.Canonical())
			repairError(t, err, "HOSTILE_INPUT")
		})
		t.Run("detail-count", func(t *testing.T) {
			root := repairRoot(repairDetails().Canonical())
			details := make([]any, 513)
			for i := range details {
				details[i] = root["details"].([]any)[0]
			}
			root["details"] = details
			_, err := ParseDetails(repairWire(root))
			repairError(t, err, CodeInputLimit)
		})
	})
}

func TestProtocolPositiveRepair(t *testing.T) {
	t.Run("WQO-V0-001 canonical protocol acceptance", func(t *testing.T) {
		d := repairDetails()
		if _, err := ParseDetails(d.Canonical()); err != nil {
			t.Fatal(err)
		}
		d.Details = []Detail{}
		RefreshDetails(d)
		if _, err := ParseDetails(d.Canonical()); err != nil {
			t.Fatal(err)
		}
		p := repairPolicy()
		parsed, err := ParsePolicy(p.Canonical())
		if err != nil || !bytes.Equal(parsed.Canonical(), p.Canonical()) {
			t.Fatalf("semantic argv order: %v", err)
		}
		c := repairCheckpoint()
		c.RepositorySource.ObjectFormat = "sha256"
		c.RepositorySource.Commit = testZeroDigest
		c.RepositorySource.Tree = testZeroDigest
		RefreshRepositorySource(&c.RepositorySource)
		RefreshCheckpoint(c)
		if _, err := ParseCheckpoint(c.Canonical()); err != nil {
			t.Fatal(err)
		}
	})
	t.Run("WQO-V0-027 allowed detail prose controls", func(t *testing.T) {
		d := repairDetails()
		text := "prose\tline\nreturn\r"
		d.Details[0].Payload.Body = &text
		RefreshDetail(&d.Details[0])
		RefreshDetails(d)
		if _, err := ParseDetails(d.Canonical()); err != nil {
			t.Fatal(err)
		}
	})
}

func TestProtocolLineSeparatorProseRoundTrips(t *testing.T) {
	t.Run("WQO-V0-001 written prose with U+2028 is canonical without optional escapes", func(t *testing.T) {
		d := repairDetails()
		text := "line\u2028paragraph\u2029end"
		d.Details[0].Payload.Body = &text
		RefreshDetail(&d.Details[0])
		RefreshDetails(d)
		if _, err := ParseDetails(d.Canonical()); err != nil || bytes.Contains(d.Canonical(), []byte(`\u2028`)) {
			t.Fatalf("err = %v, canonical = %q", err, d.Canonical())
		}
	})
}

func TestProtocolGrammarRepair(t *testing.T) {
	t.Run("WQO-V0-001", func(t *testing.T) {
		for _, tc := range []struct {
			name, path, key string
			value           any
		}{
			{"access-kind", "", "accessContextId", "scope:corvint:local"},
			{"queue-kind", "", "queueAuthorityId", "repo:corvint"},
			{"repo-kind", "", "repositoryAuthorityId", "queue:corvint:local"},
			{"scope-kind", "", "scopeId", "ticket:corvint:local:a"},
			{"operation-null-element", "operations", "verify", []any{nil}},
			{"operation-number-element", "operations", "verify", []any{1}},
			{"operation-empty", "operations", "verify", []any{}},
			{"mapping-empty", "", "mappingVersion", ""},
			{"count-leading-zero", "", "detailLimit", "01"},
			{"count-overflow", "", "detailLimit", "2147483648"},
		} {
			t.Run(tc.name, func(t *testing.T) {
				root := repairRoot(repairPolicy().Canonical())
				repairObjectAt(root, tc.path)[tc.key] = tc.value
				_, err := ParsePolicy(repairWire(root))
				repairError(t, err, CodeMalformedInput)
			})
		}
		for _, tc := range []struct {
			name, key string
			value     any
		}{
			{"OID-size", "commit", testZeroDigest}, {"OID-case", "tree", strings.Repeat("A", 40)}, {"format", "objectFormat", "sha512"}, {"digest", "materializationSha256", "bogus"}, {"source-kind", "id", "ticket-version:sha256:" + testZeroDigest},
		} {
			t.Run(tc.name, func(t *testing.T) {
				root := repairRoot(repairCheckpoint().Canonical())
				source := repairObjectAt(root, "repositorySource")
				source[tc.key] = tc.value
				// Preserve the deliberately invalid source ID while binding the enclosing document.
				if tc.key == "id" {
					repairBind(root, "id", "work-queue-checkpoint", CheckpointProfile)
				} else {
					repairRehash(root)
				}
				_, err := ParseCheckpoint(append(canonicalAny(root), '\n'))
				repairError(t, err, CodeMalformedInput)
			})
		}
		t.Run("source-identity", func(t *testing.T) {
			root := repairRoot(repairCheckpoint().Canonical())
			repairObjectAt(root, "repositorySource")["id"] = "repository-source:sha256:" + testZeroDigest
			repairBind(root, "id", "work-queue-checkpoint", CheckpointProfile)
			_, err := ParseCheckpoint(append(canonicalAny(root), '\n'))
			repairError(t, err, CodeConflicted)
		})
		for _, field := range []string{"acceptanceCriteria", "evidenceHandles"} {
			for _, tc := range []struct {
				name  string
				value any
			}{
				{"null-element", []any{nil}}, {"number-element", []any{1}}, {"duplicate", []any{"a", "a"}}, {"unsorted", []any{"z", "a"}},
			} {
				t.Run(field+"/"+tc.name, func(t *testing.T) {
					root := repairRoot(repairDetails().Canonical())
					repairObjectAt(root, "details/0/payload")[field] = tc.value
					_, err := ParseDetails(repairWire(root))
					repairError(t, err, CodeMalformedInput)
				})
			}
		}
		for _, field := range []string{"ticketId", "repositoryAuthorityId", "ticketVersionId", "payloadSha256", "detailId"} {
			t.Run(field+"/typed-grammar", func(t *testing.T) {
				root := repairRoot(repairDetails().Canonical())
				detail := repairObjectAt(root, "details/0")
				detail[field] = "wrong:kind"
				if field != "detailId" {
					repairBind(detail, "detailId", "work-queue-detail-record", "work-queue-detail-record/0")
				}
				repairBind(root, "id", "work-queue-details", DetailsProfile)
				_, err := ParseDetails(append(canonicalAny(root), '\n'))
				repairError(t, err, CodeMalformedInput)
			})
		}
		for _, raw := range [][]byte{[]byte("{}"), []byte("{}\n\n"), []byte("{ }\n"), []byte("{\"id\":1,\"id\":1}\n"), []byte("{\"id\":\"\xff\"}\n"), []byte("{\"id\":\"\\ud800\"}\n")} {
			_, err := ParseDetails(raw)
			repairError(t, err, CodeMalformedInput)
		}
	})
}

func TestProtocolHostileBoundariesRepair(t *testing.T) {
	t.Run("WQO-V0-027", func(t *testing.T) {
		for _, field := range []string{"displayKey", "owner", "evidenceHandles"} {
			for _, control := range []string{"\t", "\n", "\r"} {
				t.Run(fmt.Sprintf("%s/%q", field, control), func(t *testing.T) {
					root := repairRoot(repairDetails().Canonical())
					value := any("label" + control)
					if field == "evidenceHandles" {
						value = []any{value}
					}
					repairObjectAt(root, "details/0/payload")[field] = value
					_, err := ParseDetails(repairWire(root))
					repairError(t, err, "HOSTILE_INPUT")
				})
			}
		}
		t.Run("empty-label", func(t *testing.T) {
			root := repairRoot(repairDetails().Canonical())
			repairObjectAt(root, "details/0/payload")["owner"] = ""
			_, err := ParseDetails(repairWire(root))
			repairError(t, err, CodeMalformedInput)
		})
		t.Run("detail-byte-bound", func(t *testing.T) {
			root := repairRoot(repairDetails().Canonical())
			payload := repairObjectAt(root, "details/0/payload")
			payload["acceptanceCriteria"] = []any{strings.Repeat("a", 256<<10), strings.Repeat("b", 256<<10), strings.Repeat("c", 256<<10), strings.Repeat("d", 256<<10)}
			_, err := ParseDetails(repairWire(root))
			repairError(t, err, CodeInputLimit)
		})
		t.Run("valid-maxima", func(t *testing.T) {
			root := repairRoot(repairDetails().Canonical())
			payload := repairObjectAt(root, "details/0/payload")
			payload["body"] = strings.Repeat("x", 256<<10)
			payload["owner"] = strings.Repeat("y", 32<<10)
			if _, err := ParseDetails(repairWire(root)); err != nil {
				t.Fatal(err)
			}
		})
		t.Run("confusables-distinct", func(t *testing.T) {
			left := repairRoot(repairDetails().Canonical())
			right := repairRoot(repairDetails().Canonical())
			repairObjectAt(left, "details/0/payload")["title"] = "A"
			repairObjectAt(right, "details/0/payload")["title"] = "А"
			a, err := ParseDetails(repairWire(left))
			if err != nil {
				t.Fatal(err)
			}
			b, err := ParseDetails(repairWire(right))
			if err != nil {
				t.Fatal(err)
			}
			if a.ID == b.ID {
				t.Fatal("Unicode was normalized")
			}
		})
		for _, field := range []string{"accessContextId", "mappingVersion"} {
			t.Run(field, func(t *testing.T) {
				root := repairRoot(repairPolicy().Canonical())
				root[field] = "secret\u202evalue"
				_, err := ParsePolicy(repairWire(root))
				repairError(t, err, "HOSTILE_INPUT")
			})
		}
		t.Run("operation", func(t *testing.T) {
			root := repairRoot(repairPolicy().Canonical())
			repairObjectAt(root, "operations")["verify"] = []any{"secret\ufeffvalue"}
			_, err := ParsePolicy(repairWire(root))
			repairError(t, err, "HOSTILE_INPUT")
		})
	})
}

func TestProtocolHostileGrammarPropagationRepair(t *testing.T) {
	t.Run("WQO-V0-027", func(t *testing.T) {
		for _, r := range []rune{0, 1, 0x7f, 0x85, 0xfeff, 0x061c, 0x200e, 0x200f, 0x202a, 0x202b, 0x202c, 0x202d, 0x202e, 0x2066, 0x2067, 0x2068, 0x2069} {
			t.Run(fmt.Sprintf("U+%04X", r), func(t *testing.T) {
				repairError(t, ValidateIdentifier("x"+string(r)), "HOSTILE_INPUT")
				if r <= 0x1f || r == 0x7f || (r >= 0x80 && r <= 0x9f) {
					repairError(t, ValidatePath("x"+string(r)), "HOSTILE_INPUT")
				} else if err := ValidatePath("x" + string(r)); err != nil {
					t.Fatal("Path grammar changed:", err)
				}
				root := repairRoot(repairDetails().Canonical())
				detail := repairObjectAt(root, "details/0")
				detail["ticketVersionId"] = "ticket-version:sha256:" + string(r)
				repairBind(detail, "detailId", "work-queue-detail-record", "work-queue-detail-record/0")
				repairBind(root, "id", "work-queue-details", DetailsProfile)
				_, err := ParseDetails(append(canonicalAny(root), '\n'))
				repairError(t, err, "HOSTILE_INPUT")
			})
		}
		t.Run("unknown-key-redacted", func(t *testing.T) {
			root := repairRoot(repairPolicy().Canonical())
			secret := "private\u202evalue"
			root[secret] = true
			_, err := ParsePolicy(repairWire(root))
			repairError(t, err, CodeMalformedInput)
			if strings.Contains(err.Error(), secret) {
				t.Fatal("unknown key leaked")
			}
		})
	})
}

func TestProtocolStructureBeforeIdentityRepair(t *testing.T) {
	t.Run("WQO-V0-001", func(t *testing.T) {
		for _, where := range []string{"later-detail", "document"} {
			t.Run(where, func(t *testing.T) {
				root := repairRoot(repairDetails().Canonical())
				first := repairObjectAt(root, "details/0")
				first["detailId"] = "work-queue-detail-record:sha256:" + testZeroDigest
				if where == "later-detail" {
					second := repairObjectAt(repairRoot(repairDetails().Canonical()), "details/0")
					second["detailId"] = "work-queue-detail-record:sha256:" + strings.Repeat("f", 64)
					second["ticketId"] = true
					root["details"] = []any{first, second}
				} else {
					root["snapshotId"] = true
				}
				repairBind(root, "id", "work-queue-details", DetailsProfile)
				_, err := ParseDetails(append(canonicalAny(root), '\n'))
				repairError(t, err, CodeMalformedInput)
			})
		}
	})
}

func TestSnapshotPathWireBoundaryRepair(t *testing.T) {
	t.Run("WQO-V0-027", func(t *testing.T) {
		for _, r := range []rune{0, 1, 0x7f, 0x85, 0xfeff, 0x061c, 0x202e, 0x2069} {
			t.Run(fmt.Sprintf("U+%04X", r), func(t *testing.T) {
				snapshot := testSnapshot(testTicket("one", 1))
				path := "dir/x" + string(r) + ".go"
				snapshot.Tickets[0].TouchPaths = []string{path}
				RefreshSnapshot(snapshot)
				parsed, err := ParseSnapshot(snapshot.Canonical())
				if r <= 0x1f || r == 0x7f || (r >= 0x80 && r <= 0x9f) {
					repairError(t, err, "HOSTILE_INPUT")
					return
				}
				if err != nil {
					t.Fatal("accepted Path wire rejected:", err)
				}
				if parsed.Tickets[0].TouchPaths[0] != path || !bytes.Equal(parsed.Canonical(), snapshot.Canonical()) {
					t.Fatal("Path wire was transformed")
				}
			})
		}
	})
}
