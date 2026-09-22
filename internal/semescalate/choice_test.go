package semescalate

import (
	"bytes"
	"context"
	"fmt"
	"strings"
	"sync"
	"testing"
)

func calibratedChoiceSpy(response string) *providerSpy {
	return &providerSpy{
		caps: Capabilities{
			ModelRevision: "typed-model@r1", CalibrationDigest: "choice-cal-1", ResponseSchema: ChoiceResponseSchema,
		},
		response: []byte(response),
	}
}

func choiceInput() []Span {
	return []Span{{
		Handle: "a.go#L1-L3", Authorized: true,
		Body: []byte("func TestLogin(t *testing.T) { login(); audit() }"),
	}}
}

func choiceQuestion() ChoiceQuestion {
	return ChoiceQuestion{
		ID: "anchor", Instructions: "Which supplied anchor best supports the named gap?",
		Options: []ChoiceOption{
			{ID: "login_call", Handle: "a.go#L1-L3", Excerpt: "login()"},
			{ID: "test_name", Handle: "a.go#L1-L3", Excerpt: "TestLogin"},
		},
		MinimumConfidencePPM: 500_000,
	}
}

func choiceResponse(choice string, login, testName, none uint32) string {
	return fmt.Sprintf(`{"questionId":"anchor","choice":%q,"probabilities":{"login_call":%d,"test_name":%d,"none":%d}}`, choice, login, testName, none)
}

type blockingChoiceProvider struct {
	response []byte
	entered  chan struct{}
	release  chan struct{}
}

func (p *blockingChoiceProvider) Describe() Capabilities {
	return Capabilities{ModelRevision: "typed-model@r1", CalibrationDigest: "choice-cal-1", ResponseSchema: ChoiceResponseSchema}
}

func (p *blockingChoiceProvider) Invoke(_ context.Context, _ []byte, _ Limits) ([]byte, Observation, error) {
	close(p.entered)
	<-p.release
	return p.response, Observation{}, nil
}

func TestTypedChoiceAdmitsOnlyMechanicallySuppliedOption(t *testing.T) {
	t.Run("SEG-018 typed choice request and SEG-020 reconstructed admission", func(t *testing.T) {
		spy := calibratedChoiceSpy(choiceResponse("login_call", 800_000, 100_000, 100_000))
		out := newGate(spy).EscalateChoice(namedGap(), choiceInput(), choiceQuestion())
		if spy.calls != 1 || len(out.Candidates) != 1 {
			t.Fatalf("calls=%d outcome=%+v", spy.calls, out)
		}
		candidate := out.Candidates[0]
		if candidate.Handle != "a.go#L1-L3" || candidate.Excerpt != "login()" || candidate.Authority != Inferred || out.Frontier != FrontierUnknown {
			t.Fatalf("candidate/frontier = %+v / %s", candidate, out.Frontier)
		}
		if out.Call == nil || out.Call.Decision == nil || out.Call.Decision.ConfidencePPM != 700_000 || out.Call.Decision.Abstained || out.Call.ParsedDigest == "" {
			t.Fatalf("decision receipt = %+v", out.Call)
		}
		if !bytes.Contains(spy.requests[0], []byte(`"schema":"`+ChoiceRequestSchema+`"`)) ||
			!bytes.Contains(spy.requests[0], []byte(`"probabilityScalePpm":1000000`)) {
			t.Fatalf("typed request missing schema/scale: %s", spy.requests[0])
		}
	})
}

func TestChoiceQuestionRefusesUnanchoredOrUnboundedInputsBeforeCall(t *testing.T) {
	t.Run("SEG-018 question preflight", func(t *testing.T) {
		for name, tc := range map[string]struct {
			mutate func(*ChoiceQuestion)
			want   Reason
		}{
			"bad id":             {func(q *ChoiceQuestion) { q.ID = "Anchor" }, UnsupportedInput},
			"reserved option":    {func(q *ChoiceQuestion) { q.Options[0].ID = ChoiceAbstain }, UnsupportedInput},
			"duplicate id":       {func(q *ChoiceQuestion) { q.Options[1].ID = q.Options[0].ID }, UnsupportedInput},
			"duplicate anchor":   {func(q *ChoiceQuestion) { q.Options[1].Excerpt = q.Options[0].Excerpt }, UnsupportedInput},
			"fabricated excerpt": {func(q *ChoiceQuestion) { q.Options[0].Excerpt = "deleteEverything()" }, UnsupportedInput},
			"too few options":    {func(q *ChoiceQuestion) { q.Options = q.Options[:1] }, UnsupportedInput},
			"threshold overflow": {func(q *ChoiceQuestion) { q.MinimumConfidencePPM = ProbabilityScalePPM + 1 }, UnsupportedInput},
			"secret instruction": {func(q *ChoiceQuestion) { q.Instructions = `password = "hunter2-correct-horse"` }, SecretRisk},
		} {
			t.Run(name, func(t *testing.T) {
				question := choiceQuestion()
				question.Options = append([]ChoiceOption(nil), question.Options...)
				tc.mutate(&question)
				spy := calibratedChoiceSpy(choiceResponse("login_call", 800_000, 100_000, 100_000))
				out := newGate(spy).EscalateChoice(namedGap(), choiceInput(), question)
				if spy.calls != 0 || out.Route.Reason != string(tc.want) || out.Call != nil {
					t.Fatalf("calls=%d route=%+v", spy.calls, out.Route)
				}
			})
		}
	})
}

func TestChoiceRefusesSecretHandleBeforeCall(t *testing.T) {
	t.Run("SEG-018 transmitted question fields are screened", func(t *testing.T) {
		const handle = `password = "hunter2-correct-horse"`
		gap := namedGap()
		gap.EvidenceHandles = []string{handle}
		spans := choiceInput()
		spans[0].Handle = handle
		question := choiceQuestion()
		question.Options = append([]ChoiceOption(nil), question.Options...)
		question.Options[0].Handle = handle
		question.Options[1].Handle = handle
		spy := calibratedChoiceSpy(choiceResponse("login_call", 800_000, 100_000, 100_000))
		out := newGate(spy).EscalateChoice(gap, spans, question)
		if spy.calls != 0 || out.Route.Reason != string(SecretRisk) || out.Call != nil {
			t.Fatalf("calls=%d route=%+v", spy.calls, out.Route)
		}
	})
}

func TestChoiceSnapshotsEvidenceBeforeProviderLatency(t *testing.T) {
	t.Run("SEG-018 selected evidence is immutable for request and verification", func(t *testing.T) {
		provider := &blockingChoiceProvider{
			response: []byte(choiceResponse("login_call", 800_000, 100_000, 100_000)),
			entered:  make(chan struct{}),
			release:  make(chan struct{}),
		}
		spans := choiceInput()
		done := make(chan Outcome, 1)
		go func() {
			done <- newGate(provider).EscalateChoice(namedGap(), spans, choiceQuestion())
		}()
		<-provider.entered
		for i := range spans[0].Body {
			spans[0].Body[i] = 'x'
		}
		close(provider.release)
		out := <-done
		if len(out.Candidates) != 1 || out.Candidates[0].Excerpt != "login()" {
			t.Fatalf("caller mutation changed admission: %+v", out)
		}
	})
}

func TestChoiceResponseRequiresExactCompleteDistribution(t *testing.T) {
	valid := choiceResponse("login_call", 800_000, 100_000, 100_000)
	t.Run("SEG-019 exact probability schema", func(t *testing.T) {
		for name, response := range map[string]string{
			"trailing value":        valid + `{}`,
			"folded key":            `{"QuestionId":"anchor","choice":"login_call","probabilities":{"login_call":800000,"test_name":100000,"none":100000}}`,
			"duplicate top key":     `{"questionId":"anchor","questionId":"anchor","choice":"login_call","probabilities":{"login_call":800000,"test_name":100000,"none":100000}}`,
			"duplicate probability": `{"questionId":"anchor","choice":"login_call","probabilities":{"login_call":700000,"login_call":800000,"test_name":100000,"none":100000}}`,
			"missing probability":   `{"questionId":"anchor","choice":"login_call","probabilities":{"login_call":900000,"none":100000}}`,
			"extra probability":     `{"questionId":"anchor","choice":"login_call","probabilities":{"login_call":700000,"test_name":100000,"other":100000,"none":100000}}`,
			"null probability":      `{"questionId":"anchor","choice":"login_call","probabilities":{"login_call":null,"test_name":100000,"none":900000}}`,
			"fraction":              `{"questionId":"anchor","choice":"login_call","probabilities":{"login_call":800000.5,"test_name":99999.5,"none":100000}}`,
			"mass mismatch":         choiceResponse("login_call", 700_000, 100_000, 100_000),
			"choice not maximum":    choiceResponse("test_name", 800_000, 100_000, 100_000),
			"maximum tie":           choiceResponse("login_call", 450_000, 450_000, 100_000),
			"wrong question":        strings.Replace(valid, `"anchor"`, `"other"`, 1),
		} {
			t.Run(name, func(t *testing.T) {
				out := newGate(calibratedChoiceSpy(response)).EscalateChoice(namedGap(), choiceInput(), choiceQuestion())
				if len(out.Candidates) != 0 || out.Call == nil || strings.Join(out.Call.Rejections, ",") != "SCHEMA_INVALID" || out.Call.Decision != nil || out.Call.ParsedDigest != "" {
					t.Fatalf("invalid response admitted: %+v", out)
				}
			})
		}
	})
}

func TestChoiceConfidenceAndExplicitAbstentionAdmitNothing(t *testing.T) {
	t.Run("SEG-019 low confidence abstains", func(t *testing.T) {
		question := choiceQuestion()
		question.MinimumConfidencePPM = 200_000
		out := newGate(calibratedChoiceSpy(choiceResponse("login_call", 500_000, 400_000, 100_000))).EscalateChoice(namedGap(), choiceInput(), question)
		if len(out.Candidates) != 0 || out.Call.Decision.ConfidencePPM != 100_000 || !out.Call.Decision.Abstained || strings.Join(out.Call.Rejections, ",") != "DECISION_LOW_CONFIDENCE" {
			t.Fatalf("low-confidence outcome = %+v", out)
		}
	})
	t.Run("SEG-019 reserved option abstains", func(t *testing.T) {
		question := choiceQuestion()
		question.MinimumConfidencePPM = 0
		out := newGate(calibratedChoiceSpy(choiceResponse(ChoiceAbstain, 200_000, 100_000, 700_000))).EscalateChoice(namedGap(), choiceInput(), question)
		if len(out.Candidates) != 0 || !out.Call.Decision.Abstained || strings.Join(out.Call.Rejections, ",") != "DECISION_ABSTAINED" {
			t.Fatalf("explicit-abstain outcome = %+v", out)
		}
	})
}

func TestChoiceAndProposalSchemasCannotCross(t *testing.T) {
	t.Run("SEG-021 schema-aware eligibility", func(t *testing.T) {
		choiceSpy := calibratedChoiceSpy(choiceResponse("login_call", 800_000, 100_000, 100_000))
		if out := newGate(choiceSpy).Escalate(namedGap(), choiceInput()); choiceSpy.calls != 0 || out.Route.Reason != string(NoCalibratedModel) {
			t.Fatalf("choice provider entered proposal path: calls=%d route=%+v", choiceSpy.calls, out.Route)
		}
		proposalSpy := calibratedSpy(`{"proposals":[]}`)
		if out := newGate(proposalSpy).EscalateChoice(namedGap(), choiceInput(), choiceQuestion()); proposalSpy.calls != 0 || out.Route.Reason != string(NoCalibratedModel) {
			t.Fatalf("proposal provider entered choice path: calls=%d route=%+v", proposalSpy.calls, out.Route)
		}
		legacySpy := calibratedSpy(`{"proposals":[]}`)
		newGate(legacySpy).Escalate(namedGap(), choiceInput())
		if legacySpy.calls != 1 || bytes.Contains(legacySpy.requests[0], []byte(`"question"`)) || !bytes.Contains(legacySpy.requests[0], []byte(`"responseSchema":"`+ProposalSchema+`"`)) {
			t.Fatalf("legacy request changed: %s", legacySpy.requests[0])
		}
	})
}

func TestChoiceQuestionIdentityInvalidatesCachedDerivation(t *testing.T) {
	t.Run("SEG-021 complete question identity", func(t *testing.T) {
		spy := calibratedChoiceSpy(choiceResponse("login_call", 800_000, 100_000, 100_000))
		gate := newGate(spy)
		question := choiceQuestion()
		gate.EscalateChoice(namedGap(), choiceInput(), question)
		if cached := gate.EscalateChoice(namedGap(), choiceInput(), question); spy.calls != 1 || cached.Route.Reason != string(CachedDerivation) {
			t.Fatalf("unchanged question not cached: calls=%d route=%+v", spy.calls, cached.Route)
		}
		question.MinimumConfidencePPM--
		gate.EscalateChoice(namedGap(), choiceInput(), question)
		question.Options = append([]ChoiceOption(nil), question.Options...)
		question.Options[0].Excerpt = "func TestLogin"
		gate.EscalateChoice(namedGap(), choiceInput(), question)
		question.Options[0], question.Options[1] = question.Options[1], question.Options[0]
		gate.EscalateChoice(namedGap(), choiceInput(), question)
		if spy.calls != 4 {
			t.Fatalf("question mutations reused derivation: calls=%d", spy.calls)
		}

		inputs := append(choiceInput(), Span{Handle: "b.go#L1", Body: []byte("logout()"), Authorized: true})
		base := choiceQuestion()
		_, digest, reason := prepareChoiceQuestion(base, inputs)
		if reason != "" {
			t.Fatal(reason)
		}
		base.Options = append([]ChoiceOption(nil), base.Options...)
		base.Options[0].Handle, base.Options[0].Excerpt = "b.go#L1", "logout()"
		_, changed, reason := prepareChoiceQuestion(base, inputs)
		if reason != "" || digest == changed {
			t.Fatalf("option handle/excerpt missing from identity: reason=%s", reason)
		}
	})
}

func TestConcurrentChoiceCallsPreserveBudgetAndCacheInvariants(t *testing.T) {
	t.Run("SEG-015 concurrent reservations cannot overspend", func(t *testing.T) {
		spy := calibratedChoiceSpy(choiceResponse("login_call", 800_000, 100_000, 100_000))
		cfg := Config{Provider: spy, Verifiers: []Verifier{excerptVerifier{}}, Budget: roomyBudget(), CostMicrosPerCall: 10}
		cfg.Budget.Calls = 1
		gate := New(cfg)
		questions := []ChoiceQuestion{choiceQuestion(), choiceQuestion()}
		questions[1].MinimumConfidencePPM--
		start := make(chan struct{})
		outcomes := make([]Outcome, len(questions))
		var wg sync.WaitGroup
		for i := range questions {
			wg.Add(1)
			go func(i int) {
				defer wg.Done()
				<-start
				outcomes[i] = gate.EscalateChoice(namedGap(), choiceInput(), questions[i])
			}(i)
		}
		close(start)
		wg.Wait()
		refused := 0
		for _, out := range outcomes {
			if out.Route.Reason == string(BudgetExhausted) {
				refused++
			}
		}
		if spy.calls != 1 || refused != 1 {
			t.Fatalf("calls=%d budget refusals=%d outcomes=%+v", spy.calls, refused, outcomes)
		}
	})

	t.Run("SEG-017 concurrent identical derivations call once", func(t *testing.T) {
		spy := calibratedChoiceSpy(choiceResponse("login_call", 800_000, 100_000, 100_000))
		gate := newGate(spy)
		start := make(chan struct{})
		outcomes := make([]Outcome, 2)
		var wg sync.WaitGroup
		for i := range outcomes {
			wg.Add(1)
			go func(i int) {
				defer wg.Done()
				<-start
				outcomes[i] = gate.EscalateChoice(namedGap(), choiceInput(), choiceQuestion())
			}(i)
		}
		close(start)
		wg.Wait()
		cached := 0
		for _, out := range outcomes {
			if out.Route.Reason == string(CachedDerivation) {
				cached++
			}
		}
		if spy.calls != 1 || cached != 1 {
			t.Fatalf("calls=%d cached=%d outcomes=%+v", spy.calls, cached, outcomes)
		}
	})
}
