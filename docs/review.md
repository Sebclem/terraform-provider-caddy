# First increment review and verification

A separate reviewer inspected 0f17c37..a01ddcb read-only against the approved specification and first-release plan. The review identified four important lifecycle issues. All were reproduced or characterized before the fix pass, then covered by passing regressions:

| Finding | Regression and result |
| --- | --- |
| Explicit-ID parent replacement could leave descendants missing after apply | TestAccParentReplacement failed on explicit IDs, then passed for generated and explicit IDs. TestAccRouteParentReplacement additionally verifies moving an explicit-ID route to another server while retaining live proxy traffic. |
| Cancellation or gateway failure after a possible write could discard recovery identity | TestMutatePostCommitCancellation and TestMutateGatewayAfterCommit failed before the fix and passed after it. Existing TestAccUncertainCreateRetainsID verifies recoverable CLI state. |
| Missing ordering anchor prevented refresh/repair | TestAccMissingAnchorRecovery failed before the fix and passed afterward. Missing anchors report unsatisfied order during Read; requested nonexistent anchors still fail during mutation. |
| Structural upstream edits could clone one nested ID twice | TestProxyStructuralIDs reproduced Caddy rejecting the duplicate-ID array; the corrected correspondence logic passes and preserves unique identities. |

The null-root bootstrap diagnostic was promoted from minor to an onboarding requirement. On isolated Caddy 2.11.7, default startup exposes a null configuration and guarded POST /config/apps returns 500. Root replacement remains forbidden. TestBootstrapDiagnostic failed on the unhelpful message and passed after adding an actionable external-bootstrap prerequisite. An admin-only Caddyfile was adapted with the pinned binary to confirm an object configuration.

Final verification ran on the fixed implementation:

- All 14 top-level acceptance scenarios with OpenTofu 1.12.6 and Terraform 1.15.9 against Caddy 2.11.7; each includes real provider processes and disposable loopback instances. Parent replacement contains generated/explicit-ID subcases.
- Full unit and API suites with each CLI matrix run.
- Race detector for the Caddy client, resource engine and fixtures.
- go vet, go build, Go/HCL formatting, and documentation links/code fences.
- Go, Caddy and OpenTofu downloaded archives checked against their official release checksums.

The post-review changes were verified with regressions and the full suites; there was one independent review and one fix pass, not a second review.

## Deferred minor

A trailing empty query marker in an endpoint URL (`http://host?`) bypasses the eager query check and fails at the API boundary. Use the documented origin form with no query marker. This affects malformed input; ordinary HTTP/HTTPS origins are covered.

## Scope rulings

Later features are intentionally outside this release, as required by incremental delivery; the active repository is not ready for complete migration yet. Compatibility claims cover only the tested version matrix. Imports of unsupported matcher shapes fail explicitly until the matcher increment is implemented. Null-root bootstrap remains external to preserve the approved root-mutation prohibition.

All implementation rulings and their trade-offs are recorded in [implementation notes](implementation-notes.md).
