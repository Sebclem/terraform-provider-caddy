# First increment implementation decisions

The implementation follows the approved design with these recorded rulings:

- The isolated worktree is under /tmp on feat/first-release rather than nested inside the main checkout. The branch belongs to the standalone provider repository; main remains separate.
- The small HTTP/TLS implementation stays together in client.go. The rest is divided into identity, guarded mutation, ordering, resource schemas, planning, state mapping and lifecycle files.
- A snapshot carries internal post-write evidence so an uncertain create can reconcile its attempted ID. An initial existing-ID collision is rejected. Unknown outcomes retain recoverable identity and report an error.
- State planning behavior is verified through real CLI acceptance tests rather than synthetic framework state construction. ID generation/replacement, ordering drift and import ownership are covered by both CLI runs.
- Three public resources share internal lifecycle plumbing. Their IDs, state, ownership and API operations remain independent. Closely coupled resource-layer changes were committed together to keep each commit buildable.
- Terraform/OpenTofu do not persist private-only metadata changes in a no-op plan. A computed configuration_revision fingerprint makes optional-field adoption a state-only update. Cost: an imported route with explicitly configured optional values can need one metadata-only apply before its next plan is empty. Required-only server/proxy imports already match without that step. This replaces the initial design's assumption that every matching import could immediately have an empty plan.

The revision hashes only owned field names, not their values. Actual ownership is private provider metadata. It is not a raw configuration input or a user-configurable management mask.

## Independent review

The final review found lifecycle cases now covered by regression tests: explicitly identified parent replacement propagates to descendants; cancellation and gateway errors after a possible commit retain recovery state; missing ordering anchors report drift so constraints can be repaired; ambiguous upstream correspondence never clones one nested ID twice. An empty null Caddy document requires external bootstrap because the API rejects subtree creation in that state.

Deferred minor: an endpoint ending in an empty query marker (`?`) is not rejected during provider validation and fails at the API boundary. Use the documented origin form without a query marker. This affects malformed endpoint input; normal HTTP/HTTPS origins are covered.

## Active delivery implementation

Work continues in the main checkout directory on `feat/active-delivery`, as requested. Detailed task refinements implement the already-approved scope without a new approval round. Shared schema, codec and engine changes land together to keep commits buildable; the cost is a larger feature commit. The provider has 12 public resource types; ID-less metrics and hostname bindings use explicit internal state identities and no importer.

Nested ownership includes both container presence and individual leaves. The revision fingerprints all owned paths, while unknown configuration keeps the planned fingerprint unknown until values resolve. This preserves metadata-only nested adoption and subsequent omission without inconsistent final plans. Correlated changes write the smallest common ancestor. Fixed forward-auth/error-page templates have strict import recognition, but managed template drift remains readable, schedules repair through the computed revision and does not block deletion.

The following native JSON decisions were established by fixtures and executable traffic:

- The two active certificate groups have disjoint names, and the default TLS policy selects the intended loaded certificate for every site. A separate connection-policy resource is unnecessary for this inventory. Cost if this assumption changes: overlapping/explicit-tag selection needs a later typed resource.
- Jellyfin's allowed exact `/metrics` request continues from its access-check subroute to the proxy. The example deliberately keeps that route nonterminal.
- A subroute error handler returns a new file-server error differently from the legacy server-level error handler. The high-level resource therefore adds a fixed original-status responder after a pass-through file server. Missing files preserve the original status and empty body; 403/404/500 behavior is compared with the legacy handler. Cost: non-missing-file I/O failures still use native Caddy propagation rather than promising full equivalence for every filesystem failure.
- Error-page import recognizes the complete fixed sequence by the top ID and accepts arbitrary or absent internal IDs. Updates preserve their existing presence/identity. Internal objects remain outside independent resource management. Cost: future template shape changes require an explicit compatibility decision.
- Named routes/relay, overlapping certificate selection and dynamic discovery remain outside active scope. Same-path certificate renewal/reload and same-filename writer reconfiguration remain operator responsibilities. Production cutover/service persistence is an operator handoff, not an action performed by this implementation.

See [active review](review-active-delivery.md) for independent findings, fixes and the remaining minor wording issue.
