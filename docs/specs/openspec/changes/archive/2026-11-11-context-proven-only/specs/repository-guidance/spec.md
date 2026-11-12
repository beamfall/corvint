## MODIFIED Requirements

### Requirement: RGV-V0-001 Review is the only guidance command

The experimental `review --base FULL_SHA [--max-refs N]` command MUST emit canonical JSON with
authority labelled inferred and `mutates` false. The `features` and `overview` commands MUST NOT
exist: a call to either gets the usage refusal and exit 2. The review document MUST NOT carry a
`features` or `changedFeatures` member, languages, entrypoints, manifests or `nextCalls`.

#### Scenario: Removed verbs

- **WHEN** a caller runs `corvint features` or `corvint overview`
- **THEN** it gets the usage refusal and exit 2

#### Scenario: Review carries no inferred features

- **WHEN** a caller runs `corvint review --base FULL_SHA` on a clean worktree
- **THEN** the document holds `review.affected`, `review.affectedFull` and the branch overlap,
  and holds no `features` or `changedFeatures` member

### Requirement: RGV-V0-007 Review composes affected advice

Review MUST require a full immutable ancestor base and compose actual affected-plan advice over
immutable target sources. Omitted sources MUST remain unknown, and guidance MUST NOT close test
obligations.

#### Scenario: Base is not an ancestor

- **WHEN** the base is not an ancestor of HEAD
- **THEN** review refuses and emits no document

## REMOVED Requirements

### Requirement: RGV-V0-003 Discovery reuses the Genesis inventory for features

**Reason**: the inferred feature list is cut (decision 0597); review still reads the inventory for
RGV-V0-013. **Migration**: none; feature maps come from flows (decision 0596 G25).

### Requirement: RGV-V0-004 Bounded feature heuristics

**Reason**: regex feature discovery is cut (decision 0597). **Migration**: none.

### Requirement: RGV-V0-005 Overview composition

**Reason**: `overview` is cut (decision 0597). **Migration**: `corvint affected` and `corvint
impact` for change questions.

### Requirement: RGV-V0-006 Suggested nextCalls

**Reason**: only `features` and `overview` emitted them (decision 0597). **Migration**: none.

### Requirement: RGV-V0-955 Untracked-only dirt for overview and features

**Reason**: both commands are cut (decision 0597). **Migration**: none; `review` still refuses
every dirty state under RGV-V0-002.

### Requirement: RGV-V0-956 Feature IDs and featureCall

**Reason**: `features` and `feature` are cut (decision 0597). **Migration**: none.
