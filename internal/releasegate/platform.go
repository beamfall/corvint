package releasegate

func unsupportedGitReport() Report {
	return Report{
		Checks:   Checks{Parity: NotRun, Safety: Unsupported, Performance: NotRun, Packaging: NotRun, CorvintDogfood: NotRun, BeamfallDogfood: NotRun},
		Findings: []Finding{{Kind: "git-process-containment-unsupported", Detail: gitContainmentUnsupportedReason()}},
	}
}
