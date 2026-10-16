# First increment implementation decisions

The implementation follows the approved design with these recorded rulings:

- The isolated worktree is under /tmp on feat/first-release rather than nested inside the main checkout. The branch belongs to the standalone provider repository; main remains separate.
- The small HTTP/TLS implementation stays together in client.go. The rest is divided into identity, guarded mutation, ordering, resource schemas, planning, state mapping and lifecycle files.
- A snapshot carries internal post-write evidence so an uncertain create can reconcile its attempted ID. An initial existing-ID collision is rejected. Unknown outcomes retain recoverable identity and report an error.
- State planning behavior is verified through real CLI acceptance tests rather than synthetic framework state construction. ID generation/replacement, ordering drift and import ownership are covered by both CLI runs.
- Three public resources share internal lifecycle plumbing. Their IDs, state, ownership and API operations remain independent. Closely coupled resource-layer changes were committed together to keep each commit buildable.
- Terraform/OpenTofu do not persist private-only metadata changes in a no-op plan. A computed configuration_revision fingerprint makes optional-field adoption a state-only update. Cost: an imported route with explicitly configured optional values can need one metadata-only apply before its next plan is empty. Required-only server/proxy imports already match without that step. This replaces the initial design's assumption that every matching import could immediately have an empty plan.

The revision hashes only owned field names, not their values. Actual ownership is private provider metadata. It is not a raw configuration input or a user-configurable management mask.
