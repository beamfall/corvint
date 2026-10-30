package main

import "github.com/Beamfall/corvint/internal/liveverify/mutate"

// proveAttackReport describes only the bounded selected mutant set. Its
// completion and counts never strengthen the legacy test-kills-mutant claim.
type proveAttackReport struct {
	Status                    string                `json:"status"`
	SelectedMutantSetComplete bool                  `json:"selected_mutant_set_complete"`
	Mutants                   int                   `json:"mutants"`
	Killed                    int                   `json:"killed"`
	Survived                  int                   `json:"survived"`
	Uncompilable              int                   `json:"uncompilable"`
	Skipped                   int                   `json:"skipped"`
	CheckoutRevision          string                `json:"checkout_revision,omitempty"`
	Survivors                 []proveAttackSurvivor `json:"survivors"`
	Detail                    string                `json:"detail"`
}

type proveAttackSurvivor struct {
	proveWitnessMutant
	Line int `json:"line"`
}

func withoutAttackTestsFlag(arguments []string) ([]string, int) {
	kept := make([]string, 0, len(arguments))
	count, positional := 0, false
	for _, argument := range arguments {
		if argument == "--" {
			positional = true
		}
		if argument == "--attack-tests" && !positional {
			count++
			continue
		}
		kept = append(kept, argument)
	}
	return kept, count
}

func discloseUnrunAttacks(verdicts map[int]mutationVerdict) {
	for index, judged := range verdicts {
		if judged.attack == nil {
			judged.attack = &proveAttackReport{Status: "NOT_RUN", Detail: judged.detail, Survivors: []proveAttackSurvivor{}}
			verdicts[index] = judged
		}
	}
}

func attackReport(report mutate.Report, changed, revision string, cited map[string]citedBlob) *proveAttackReport {
	result := &proveAttackReport{
		Status: "NOT_RUN", Mutants: report.Mutants, Killed: report.Killed,
		Survived: report.Survived, Uncompilable: report.Uncompilable, Skipped: report.Skipped,
		CheckoutRevision: revision, Survivors: []proveAttackSurvivor{}, Detail: report.Detail,
	}
	if report.Mutants > 0 {
		result.Status = "PARTIAL"
		if report.Skipped == 0 && report.Killed+report.Survived+report.Uncompilable == report.Mutants {
			result.Status = "COMPLETED"
			result.SelectedMutantSetComplete = true
		}
	}
	for _, survivor := range report.Survivors {
		result.Survivors = append(result.Survivors, proveAttackSurvivor{
			proveWitnessMutant: proveWitnessMutant{Path: changed, Blob: cited[changed].oid,
				ByteSpan:         proveWitnessSpan{Start: survivor.Start, End: survivor.End},
				MutationOperator: survivor.Operator},
			Line: survivor.Line,
		})
	}
	return result
}
