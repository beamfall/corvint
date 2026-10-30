# Decision 0330: preserve future commercial licensing options

- Date: 2026-09-22
- Owner: Russell Lewis
- Status: accepted preservation and manual-acceptance policy
- Authority: the owner asked that licensing and release choices preserve future revenue options,
  then instructed implementation of the contributor-licensing safeguard and expressly directed
  the agent to perform the review without requiring hired counsel.

## Decision

Keep the AGPL-3.0-or-later product and existing Apache-2.0 interoperability boundary from
[decision 0002](0002-future-publication-transition.md). Paid services and compliant hosting remain
possible; an alternative commercial implementation license depends on rights actually controlled.
Previously granted open-source rights remain in force for compliant recipients.

Add a versioned, non-assignment contributor agreement explicitly permitting alternative
commercial licensing. The agreement identifies the actual recipient, limits each acceptance to named
commits and immutable terms, covers copyright and necessary patent permissions, and requires
employer/client/co-author authority and third-party disclosure. It preserves contributor ownership
and does not manufacture rights in third-party material or earlier submissions.

Require an authenticated, revision-bound PR acceptance and an authorized maintainer acknowledgment
for outside AGPL product contributions, or an equivalent recorded sufficient grant. Retain exact
text, contribution bytes and accepted-to-merged mappings privately; new merge content needs rights
coverage. Apache-only contributions retain their existing terms. No paid service or hired-counsel
approval is required. Publishing the procedure does not claim an executed agreement or clearance.

## Acceptance evidence and limits

Review the agreement, contribution instructions, license map, and provenance record together for
consistent scope, explicit consent, immutable contribution identity, and accurate acceptance status.
Run the repository documentation checks and required gate; these establish repository consistency,
not guaranteed legal enforceability. The owner accepted an agent-performed review against the
primary sources below; it is not a professional legal opinion. A commercial release still needs
an actual rights/dependency inventory; the existing sole-author
attestation is evidence to verify, not a substitute for that work.

Corvint context and CEM receipts retain discovery gaps. No executable Go test can witness consent,
copyright ownership, or legal review. Such obligations must remain unknown rather than acquire
invented software-test evidence. Professional legal approval and actual contributor acceptances
remain NOT_PRODUCED; professional approval is not an activation requirement.

## Compatibility, failure modes, and rollback

This decision changes incoming-contribution policy, not runtime behavior or existing outbound
licenses. Missing authority or acceptance blocks the affected product contribution. A stale
acceptance cannot cover later commits, and publication cannot retroactively relicense older work.
Do not widen the Apache boundary or ship a proprietary edition merely because this policy exists.

The owner may publish a new agreement version and policy in a later explicit decision. That cannot
withdraw rights already granted by a public license or an executed agreement. Superseded agreement
versions and actual acceptance records must remain identifiable for the work they govern.

## Review basis

The review compared the grant structure with the [Apache individual CLA](https://www.apache.org/licenses/icla.pdf),
the [Apache corporate CLA](https://www.apache.org/licenses/cla-corporate.pdf), and
[Harmony's combined template](https://www.harmonyagreements.org/docs/ha-combined-v1), especially
its retained-ownership, sublicensing, outbound option five, moral-rights, and successor provisions.
These are comparison sources, not claims that Corvint uses an unchanged or endorsed template.
The [GNU licensing FAQ](https://www.gnu.org/licenses/gpl-faq.en.html#ReleaseUnderGPLAndNF) distinguishes
copyright-owner relicensing from downstream public-license rights. The actual repository LICENSE
and LICENSE-APACHE-2.0 remain the outbound terms.

The resulting agreement adds reciprocal commitments for the accepted contribution's public
license and rights-record handling, scoped moral-rights consent, successor obligations, explicit
two-party acceptance, and no silent version changes. It does not select a jurisdiction from an
assumed residence. Electronic attribution, contributor capacity and actual ownership still need
case-specific verification; a PR comment or AI review cannot certify those facts.
