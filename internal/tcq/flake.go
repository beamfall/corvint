package tcq

// terminalOutcome lists the TCQ-V0-049 statuses that count as an outcome of
// running a test. A skipped row states that the test did not run, so it
// contributes nothing to a divergence.
var terminalOutcome = map[string]bool{ReportPassed: true, ReportFailed: true, ReportError: true}

// Flaky is the shared TCQ-V0-049 rule: one test's terminal statuses, observed at
// one target revision under one environment variant across two or more
// observations, diverge when more than one distinct status appears. Every
// Corvint test provider derives its flake qualification from this function;
// the caller supplies the grouping and TCQ supplies the judgement.
func Flaky(statuses []string) bool {
	distinct := map[string]bool{}
	for _, status := range statuses {
		if terminalOutcome[status] {
			distinct[status] = true
		}
	}
	return len(distinct) > 1
}

// flakyKeys implements TCQ-V0-050 for one dynamic invocation: execution keys
// whose rows, pooled across the current observation and every comparable prior,
// come from at least two observations and diverge under Flaky. A single
// observation can never be flaky here; its repeated rows stay TCQ-V0-034
// ambiguity.
func flakyKeys(current observation, priors []observation) map[string]bool {
	statuses := map[string][]string{}
	contributors := map[string]int{}
	for _, observed := range comparableObservations(current, priors) {
		for key, observedStatuses := range rowStatuses(observed) {
			statuses[key] = append(statuses[key], observedStatuses...)
			contributors[key]++
		}
	}
	flaky := map[string]bool{}
	for key, pooled := range statuses {
		if contributors[key] > 1 && Flaky(pooled) {
			flaky[key] = true
		}
	}
	return flaky
}

// comparableObservations keeps the priors whose declared variant is
// byte-identical to the current one's. Two undeclared variants are comparable;
// a declared variant is never comparable with an undeclared one, because an
// unknown environment cannot be shown equal to a known one.
func comparableObservations(current observation, priors []observation) []observation {
	comparable := []observation{current}
	variant := current.variantKey()
	for _, prior := range priors {
		if prior.variantKey() == variant {
			comparable = append(comparable, prior)
		}
	}
	return comparable
}

func rowStatuses(observed observation) map[string][]string {
	statuses := map[string][]string{}
	for _, row := range observed.rows {
		key := row.Obj.Values["executionKeySha256"].Str
		statuses[key] = append(statuses[key], row.Obj.Values["status"].Str)
	}
	return statuses
}
