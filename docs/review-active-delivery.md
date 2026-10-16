# Active delivery review

An independent whole-branch review examined `9d5f1d1..78f2939`, the approved design/requirements, active roadmap and detailed task plan. The reviewer used read-only inspection and isolated real-Caddy/OpenTofu reproductions. Source implementation stayed in the requested checkout. No production actions or push occurred.

Five Important findings were reproduced before correction in one fix pass:

| Finding | Regression/evidence |
| --- | --- |
| Turning proxy TLS off preserved its native TLS container | `TestAccDisableProxyTLS`; container ownership now removes it. `TestAccNestedHeaderContainerRemoval` covers another nested container. |
| Matching nested ownership adoption was lost and later omission failed | `TestAccNestedOwnershipAdoption`; adoption makes zero writes, persists metadata and omission removes the timeout. `TestAccUnknownNestedSetting` preserves unknown-value planning correctness. |
| Omitting owned HTTPS removed the issuance safeguard | `TestAccHTTPSOwnershipRemoval`; an interceptor first rejected unsafe writes before Caddy. The corrected transition keeps `automatic_https.disable = true`. |
| Strict import recognition blocked managed template refresh/repair/delete | `TestManagedHandlerDrift` for error pages and forward-auth; strict recognition is import-only and owned drift can repair/delete. |
| Deleted named logs blocked binding refresh and recreation | `TestAccMissingBindingLogRecovery` with generated and explicit log IDs; computed target status enables dependency-driven repair. |

The reviewer graded correlated nested edits as Minor because guarding/merging preserved data. The executor regraded it Important against the explicit smallest-common-scope requirement and fixed it: `TestProxyCorrelatedHealthUpdate` first observed a whole-handler write, then verified the active-health node write.

The review's declined-to-judge cases were resolved as follows: complete error-page imports accept arbitrary/absent internal IDs with preservation; overlapping certificate/tag selection and inactive named routes/relay/dynamic discovery remain excluded by active scope; same-path certificate and writer lifecycle stays external; production cutover/service persistence remains the documented operator handoff. No ignored lifecycle defect is hidden behind those scope boundaries.

Deferred minor: the introductions of `docs/resources/handler_reverse_proxy.md` and `docs/resources/http_server.md` retain first-increment wording. Their later sections and the main README describe the implemented options. No functional blocker remains from that wording. The previously recorded empty endpoint query-marker validation minor also remains unchanged.

Final verification commands and compatibility evidence are recorded in [testing](testing.md). The post-fix release gate requires fresh full-suite runs with both CLIs, real Caddy, race checks, vet/build and formatting before completion.
