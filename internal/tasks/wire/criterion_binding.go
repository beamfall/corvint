package wire

// Criterion captures are optional read artifacts. Their producer identity is a
// byte binding, not actor authentication or proof that a local binary is safe.
const CriterionCaptureProfile = "taskman-criterion-capture/0"
const CriterionVerificationProfile = "taskman-criterion-verification/0"
const CriterionCaptureResultProfile = "taskman-criterion-capture-result/0"
const MaxCriterionCaptureBytes = 4 << 20

type CriterionIdentity struct {
	ExecutableSHA256 Digest `json:"executableSha256"`
	Version          string `json:"version"`
	Build            string `json:"build"`
}

func (i CriterionIdentity) Value() Value {
	return ObjectValue(NewObject().Set("executableSha256", String(string(i.ExecutableSHA256))).Set("version", String(i.Version)).Set("build", String(i.Build)))
}
func readCriterionIdentity(r *Reader) CriterionIdentity {
	r.Closed("executableSha256", "version", "build")
	return CriterionIdentity{r.Field("executableSha256").Digest(), r.Field("version").Identifier(), string(r.Field("build").Size())}
}

type CriterionCapture struct {
	Producer                                      CriterionIdentity
	Policy, Ticket, Queue, Attempt, ClaimedTicket string
}

func (c CriterionCapture) Value() Value {
	return ObjectValue(NewObject().Set("profile", String(CriterionCaptureProfile)).Set("producer", c.Producer.Value()).Set("claimedTicket", String(c.ClaimedTicket)).Set("policy", String(c.Policy)).Set("ticket", String(c.Ticket)).Set("queue", String(c.Queue)).Set("attempt", String(c.Attempt)))
}
func ReadCriterionCapture(v Value) (CriterionCapture, error) {
	r := NewReader(v, "/")
	if e := r.Profile(CriterionCaptureProfile); e != nil {
		return CriterionCapture{}, e
	}
	r.Closed("profile", "producer", "claimedTicket", "policy", "ticket", "queue", "attempt")
	r.Field("profile").Exact(CriterionCaptureProfile)
	c := CriterionCapture{Producer: readCriterionIdentity(r.Field("producer")), ClaimedTicket: r.Field("claimedTicket").String(), Policy: r.Field("policy").String(), Ticket: r.Field("ticket").String(), Queue: r.Field("queue").String(), Attempt: r.Field("attempt").String()}
	if len(Encode(v)) > MaxCriterionCaptureBytes {
		r.Fail(CodeLimitExceeded, "criterion capture bound")
	}
	return c, r.Err()
}
func DecodeCriterionCapture(raw []byte) (CriterionCapture, error) {
	if len(raw) > MaxCriterionCaptureBytes {
		return CriterionCapture{}, Errorf(CodeLimitExceeded, "/", "criterion capture bound")
	}
	v, e := Parse(raw)
	if e != nil {
		return CriterionCapture{}, e
	}
	return ReadCriterionCapture(v)
}

type CriterionSnapshot struct {
	HeadSeq                                                    Size
	HeadReceiptSHA256, IntentTreeSHA256, PrimaryWorktreeSHA256 Digest
}

func (s CriterionSnapshot) Value() Value {
	return ObjectValue(NewObject().Set("kind", String("HISTORICAL")).Set("headSeq", String(string(s.HeadSeq))).Set("headReceiptSha256", String(string(s.HeadReceiptSHA256))).Set("intentTreeSha256", String(string(s.IntentTreeSHA256))).Set("primaryWorktreeSha256", String(string(s.PrimaryWorktreeSHA256))))
}
func readCriterionSnapshot(r *Reader) CriterionSnapshot {
	r.Closed("kind", "headSeq", "headReceiptSha256", "intentTreeSha256", "primaryWorktreeSha256")
	r.Field("kind").Exact("HISTORICAL")
	return CriterionSnapshot{r.Field("headSeq").Size(), r.Field("headReceiptSha256").Digest(), r.Field("intentTreeSha256").Digest(), r.Field("primaryWorktreeSha256").Digest()}
}
func (s CriterionSnapshot) Envelope() *Snapshot {
	return &Snapshot{HeadSeq: &s.HeadSeq, HeadReceiptSha256: &s.HeadReceiptSHA256, IntentTreeSha256: &s.IntentTreeSHA256, PrimaryWorktreeSha256: &s.PrimaryWorktreeSHA256}
}

type CriterionBinding struct {
	Snapshot                                          CriterionSnapshot
	TicketID                                          TicketID
	AcceptanceRevision                                Count
	AcceptanceCriteria                                []string
	AttemptID                                         string
	Generation                                        Size
	Phase, BaseCommit                                 string
	CandidateTreeOID                                  *string
	PolicySHA256, PolicyContentIDSHA256, ConfigSHA256 Digest
	RuntimeID                                         string
	LeaseExpiresAt                                    Timestamp
}

func (b CriterionBinding) Value() Value {
	return ObjectValue(NewObject().Set("snapshot", b.Snapshot.Value()).Set("ticketId", String(b.TicketID.Raw)).Set("acceptanceRevision", String(string(b.AcceptanceRevision))).Set("acceptanceCriteria", Strings(b.AcceptanceCriteria)).Set("attemptId", String(b.AttemptID)).Set("generation", String(string(b.Generation))).Set("phase", String(b.Phase)).Set("baseCommit", String(b.BaseCommit)).Set("candidateTreeOid", StringOrNull(b.CandidateTreeOID)).Set("policySha256", String(string(b.PolicySHA256))).Set("policyContentIdSha256", String(string(b.PolicyContentIDSHA256))).Set("configSha256", String(string(b.ConfigSHA256))).Set("runtimeId", String(b.RuntimeID)).Set("leaseExpiresAt", String(string(b.LeaseExpiresAt))))
}
func readCriterionBinding(r *Reader) CriterionBinding {
	r.Closed("snapshot", "ticketId", "acceptanceRevision", "acceptanceCriteria", "attemptId", "generation", "phase", "baseCommit", "candidateTreeOid", "policySha256", "policyContentIdSha256", "configSha256", "runtimeId", "leaseExpiresAt")
	return CriterionBinding{Snapshot: readCriterionSnapshot(r.Field("snapshot")), TicketID: r.Field("ticketId").TicketID(), AcceptanceRevision: r.Field("acceptanceRevision").Count(), AcceptanceCriteria: r.Field("acceptanceCriteria").Strings(MaxAcceptanceCriteria, true, (*Reader).String), AttemptID: r.Field("attemptId").Identifier(), Generation: r.Field("generation").Size(), Phase: r.Field("phase").Identifier(), BaseCommit: r.Field("baseCommit").OID(), CandidateTreeOID: r.Field("candidateTreeOid").StringOrNull((*Reader).OID), PolicySHA256: r.Field("policySha256").Digest(), PolicyContentIDSHA256: r.Field("policyContentIdSha256").Digest(), ConfigSHA256: r.Field("configSha256").Digest(), RuntimeID: r.Field("runtimeId").Exact("external-agent"), LeaseExpiresAt: r.Field("leaseExpiresAt").Timestamp()}
}

type CriterionVerification struct {
	Producer, Verifier CriterionIdentity
	CaptureSHA256      Digest
	Binding            CriterionBinding
}

func (v CriterionVerification) Value() Value {
	return ObjectValue(NewObject().Set("profile", String(CriterionVerificationProfile)).Set("producer", v.Producer.Value()).Set("verifier", v.Verifier.Value()).Set("captureSha256", String(string(v.CaptureSHA256))).Set("binding", v.Binding.Value()))
}
func ReadCriterionVerification(v Value) (CriterionVerification, error) {
	r := NewReader(v, "/")
	if e := r.Profile(CriterionVerificationProfile); e != nil {
		return CriterionVerification{}, e
	}
	r.Closed("profile", "producer", "verifier", "captureSha256", "binding")
	r.Field("profile").Exact(CriterionVerificationProfile)
	out := CriterionVerification{readCriterionIdentity(r.Field("producer")), readCriterionIdentity(r.Field("verifier")), r.Field("captureSha256").Digest(), readCriterionBinding(r.Field("binding"))}
	return out, r.Err()
}
func CriterionCaptureResult(c CriterionCapture, v CriterionVerification) Value {
	return ObjectValue(NewObject().Set("profile", String(CriterionCaptureResultProfile)).Set("capture", c.Value()).Set("verification", v.Value()))
}
func ReadCriterionCaptureResult(v Value) (CriterionCapture, CriterionVerification, error) {
	r := NewReader(v, "/")
	if e := r.Profile(CriterionCaptureResultProfile); e != nil {
		return CriterionCapture{}, CriterionVerification{}, e
	}
	r.Closed("profile", "capture", "verification")
	r.Field("profile").Exact(CriterionCaptureResultProfile)
	if e := r.Err(); e != nil {
		return CriterionCapture{}, CriterionVerification{}, e
	}
	c, e := ReadCriterionCapture(r.Field("capture").Value())
	if e != nil {
		return c, CriterionVerification{}, e
	}
	verified, e := ReadCriterionVerification(r.Field("verification").Value())
	if e != nil {
		return c, verified, e
	}
	return c, verified, r.Err()
}
