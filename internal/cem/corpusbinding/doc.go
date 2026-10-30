// Package corpusbinding holds only the CEM-V1-201 committed-corpus binding test.
// That test reads Git objects, and a package declared in
// .corvint/test-read-scopes.json can never read .git under the full-CI Landlock
// wrapper (AFP-V0-023), so it lives in this undeclared package, which full CI
// runs unconfined from a full current-history checkout (V1-1334). The exact
// historical fixtures retained by decision 0587 resolve their original binding
// objects from a bounded sparse fixture imported only into a temporary repository.
package corpusbinding
