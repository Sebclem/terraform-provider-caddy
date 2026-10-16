# Caddy Provider First Release Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Ship three typed Caddy resources with selective live updates, UUID identity, ordering and ID-based import, verified with OpenTofu and Terraform.

**Architecture:** A Go provider separates transport, identity resolution, guarded mutations and resource state mapping. Independent server, route and reverse-proxy resources share these internal components; upstreams remain nested. CLI integration tests execute the built provider against isolated Caddy processes and capture its API writes.

**Tech Stack:** Go 1.27.2, Terraform Plugin Framework v1.19.0, terraform-plugin-go v0.31.0, google/uuid v1.6.0, Go standard-library tests; Caddy v2.11.7, OpenTofu v1.12.6 and Terraform v1.15.9 for the first compatibility matrix. These are exact initial pins, not claims of compatibility before tests run.

**Spec:** [Caddy provider design](../../../spec/design.md), [requirements](../../../spec/requirements.md). The companion delivery plan covers later increments.

## Global Constraints

- "The public interface uses typed attributes and nested blocks. No raw JSON, Caddyfile fragments, arbitrary configuration paths or untyped configuration payloads are accepted."
- "OpenTofu is the intended sole configuration manager of each target instance; Terraform remains a supported alternative."
- "Routes are active immediately, including while empty or incomplete during apply."
- "There is no activation resource, staging sentinel, deactivation sequence or multi-resource transaction."
- "If omitted, generate a UUID v4 once during creation."
- "Import accepts only the existing object's Caddy `@id`."
- "Deletion intentionally removes the entire owned object's subtree, including remaining children and fields not represented in state."
- "Routine CRUD must not call `/load`, replace the configuration root, stop Caddy, invoke a shell or manage services."
- HTTP/HTTPS only; verify HTTPS; custom CA and paired client certificate/key supported. Defaults: request timeout `30s`, conflict retries 5, whole operation deadline two minutes.
- Use the smallest safe mutation scope, exact ETag/If-Match guards and stable IDs; preserve unrelated content on updates.
- No product dependencies are installed, production services changed or migration performed while writing this plan.

## Review Focus

- Encoded IDs, malformed admin URLs and reverse-proxy redirects must never target another object or forward client credentials unexpectedly: Tasks 1–2.
- A connection lost after a committed create must not generate a second UUID or duplicate an array entry: Tasks 3 and 9.
- Optional fields transitioning from configured to omitted must produce a real update without erasing never-owned fields: Tasks 5 and 9.
- Array movement between ID resolution and a write must not update a different sibling: Tasks 3–4 and 9.
- Unknown parent/anchor references and imported optional state must converge to an empty second plan on both CLIs: Tasks 6–9.

## File responsibilities and interfaces

Module: `git.sebclem.fr/sebclem/terraform-provider-caddy`. The SSH hostname used by Git is not the Go module path. Provider type: `caddy`. Use protocol 6. Until publication, use local-only source address `local.sebclem.fr/sebclem/caddy` with an explicit filesystem mirror; this does not assert that the hostname implements a registry.

- `main.go`: serve `provider.New(version)`; no configuration business logic.
- `internal/caddy/client.go`, `tls.go`, `errors.go`: HTTP transport, bounded JSON decoding, certificate configuration and classified errors.
- `internal/caddy/document.go`, `identity.go`: lossless JSON values, snapshot paths, unique ID and parent discovery.
- `internal/caddy/mutate.go`, `ordering.go`: snapshot-driven conditional edits and relative ordering.
- `internal/provider/provider.go`: provider schema, connection validation and resource registration.
- `internal/provider/state.go`, `identity.go`, `ordering.go`: ownership metadata, UUID planning rules and ordering drift state.
- `internal/provider/resource_server.go`, `resource_route.go`, `resource_reverse_proxy.go`: typed schemas and lifecycle mapping, with matching `_test.go` files.
- `internal/testutil/caddy.go`, `recorder.go`, `cli.go`: isolated processes, synthetic backends, API recorder and CLI execution.
- `internal/acceptance/*_test.go`: black-box acceptance; never call a production endpoint.
- `testdata/first-release/`: typed configurations and expected behavior; internal JSON fixtures are allowed.
- `tools/versions.env`, `tools/README.md`, `Makefile`, `.gitignore`: pins, reproducible commands and generated artifact exclusions.
- `examples/basic/`, `docs/resources/`, `README.md`: local installation and tested resource examples.

Shared types, defined in Task 1 unless otherwise noted:

- `type Value = any`, decoded with `json.Decoder.UseNumber`; `type Path []string`.
- `type Options struct { Endpoint, CAFile, ClientCertificateFile, ClientKeyFile string; RequestTimeout time.Duration; ConflictRetries int }`.
- `type Snapshot struct { Value Value; ETag string; Path Path }`.
- `NewClient(Options) (*Client, error)`; `(*Client).Get(context.Context, Path) (Snapshot, error)`; `(*Client).Write(context.Context, string, Path, Value, string) error`.
- Task 2: `type Object struct { ID string; Path, ParentPath Path; Value map[string]any }`; `Resolve(Snapshot, string) (Object, error)`; `RequireParent(Object, Object, string) error`. Collection names are internal constants, never user-supplied paths.
- Task 3: `type Decision struct { Method string; Path Path; Body Value; Done bool }`; `type Reconcile func(Snapshot) (Decision, error)`; `(*Client).Mutate(context.Context, Path, Reconcile) error`. Reconcile must be safe to call repeatedly on fresh snapshots.
- Task 4: `type Position struct { BeforeID, AfterID string }`; `Reorder([]Value, string, Position) ([]Value, bool, error)`; `OrderSatisfied([]Value, string, Position) (bool, error)`.
- Task 5: `type Ownership map[string]bool`; `type MutationIntent struct { Set map[string]Value; Remove []string }`; `PlanFields(map[string]Value, map[string]Value, Ownership) (MutationIntent, Ownership, error)`. Key absence means omitted configuration; unknown configuration is handled by framework mapping before invoking this pure helper.
- Test helpers: `StartCaddy(*testing.T) *Fixture`, `(*Fixture).Endpoint string`, `(*Fixture).Requests() []Request`, `(*Fixture).ResetRequests()`, `RunCLI(context.Context, string, string, ...string) (CLIResult, error)`. `CLIResult` contains exit code, stdout and stderr. Tests access internal types directly.

## Execution prerequisites

At execution start use the worktree skill; existing checkout is on `main` and clean at plan preparation. Use the provider repository, not the Ansible checkout, for implementation. Read any newly added AGENTS.md instructions. Acquire missing tools in project-local ignored directories or `/tmp`; do not overwrite system tools. OpenTofu is currently absent. Docker socket access failed, so use a downloaded, checksum-verified Caddy binary rather than requiring Docker. Downloads and writes to the sibling repository may require sandbox approval.

The exact upstream versions are confirmed by the [framework module](https://github.com/hashicorp/terraform-plugin-framework/blob/v1.19.0/go.mod), [Caddy module](https://github.com/caddyserver/caddy/blob/v2.11.7/go.mod) and [OpenTofu release](https://github.com/opentofu/opentofu/releases/tag/v1.12.6). Resolve normal Go dependencies and commit go.sum; no Caddy Go dependency is required in the provider binary. Use standard Go subprocess acceptance tests for both CLIs, so no Terraform-only testing library or automatic Terraform download is required.

## Task 1: HTTP transport and isolated test fixture

**Files:** Create `go.mod`, `go.sum`, `internal/caddy/{client,tls,errors}_test.go`, corresponding implementation files, `internal/testutil/{caddy,recorder}.go`, `tools/versions.env`, `tools/README.md`, `.gitignore`, `Makefile`.

**Interfaces:** Produces the common HTTP types and methods above, `StartCaddy`, request recorder and `make test-api`. Consumers do not construct their own HTTP clients.

- [ ] Write failing `TestClientConditionalWrite`, `TestClientTransport` and `TestClientDeadline` tests: check exact If-Match preservation, JSON content type, missing ETag rejection for guarded operations, HTTPS system/custom trust, mutual TLS, certificate/key pairing, invalid URL schemes, JSON errors, cancellation and the `30s` default. Redirects return errors instead of following to a different origin. A legitimate JSON string containing secret-looking values must not appear in diagnostics.
- [ ] Establish the module and pin the versions above; download verified test binaries. Run `go test ./internal/caddy -run 'TestClient' -v`; expect assertion failures for unimplemented behavior, not download or missing-tool failures.
- [ ] Implement `NewClient`, `Get` and `Write`, typed HTTP status errors, UseNumber decoding and bounded error bodies. Preserve decimal JSON values without float64 conversion. Implement the fixture with loopback-only random ports, a temp config, captured output and cleanup. Test process startup may use a config file; provider CRUD may not spawn processes.
- [ ] Run `go test ./internal/caddy -run 'TestClient' -v` and `make test-api`; expect PASS including one real guarded field update against Caddy v2.11.7. Record fixture commands and SHA256 verification procedure.
- [ ] Commit: `feat: add guarded Caddy HTTP client and API test fixture`.

## Task 2: Identity and structural discovery

**Files:** Create `internal/caddy/{document,identity}.go`, matching tests and `testdata/first-release/identity.json`.

**Interfaces:** Consumes Snapshot/Path; produces Object, Resolve and RequireParent. IDs are discovered from live JSON, never cached array indexes.

- [ ] Write failing `TestResolveID` cases for unique nested IDs, duplicate IDs, missing IDs, wrong type/parent, escaped path segments, arrays reordered between reads and large integers unchanged after round trip. Parent discovery for handlers must identify the containing route, not an arbitrary ancestor.
- [ ] Run `go test ./internal/caddy -run 'TestResolveID' -v`; expect FAIL on missing resolution behavior.
- [ ] Implement the JSON traversal, unique-ID checks and expected collection validation. Preserve unknown fields and nested IDs. Encode path segments individually, reject ambiguous unrepresentable IDs rather than altering them.
- [ ] Run the tests and `make test-api`; expect PASS and direct GET `/id/<encoded-id>` returns the same object as structural discovery.
- [ ] Commit: `feat: resolve Caddy object identity and parent relationships`.

## Task 3: Guarded mutations and uncertain results

**Files:** Create `internal/caddy/mutate.go`, `mutate_test.go`; extend `internal/testutil/recorder.go` to inject 412 and disconnect-after-commit faults.

**Interfaces:** Consumes client/snapshot/identity; produces Decision, Reconcile and Client.Mutate. Resources supply fresh-snapshot edit functions, not precomputed bodies.

- [ ] Write failing `TestMutateConflictRetry`, `TestMutateLostResponse`, `TestMutateBootstrap`, `TestMutateNoop` and `TestMutateDeadline`. Assert five retries after the initial attempt; fresh recomputation preserves concurrent sibling edits; no duplicate append after a lost response; existing ID collisions are errors; absent app/server containers can be created without replacing `/config/`; a second equivalent request writes nothing.
- [ ] Run `go test ./internal/caddy -run 'TestMutate' -v`; expect FAIL for unimplemented guarded reconciliation.
- [ ] Implement endpoint-scoped synchronization, bounded jitter and two-minute operation deadline. Use exact ETags covering target and array membership, re-resolve on every retry, reconcile ambiguous writes by identity and desired projection. Confirmed absence can be retried; unverifiable outcomes produce recoverable diagnostics. Retain the same attempted UUID throughout.
- [ ] Run those tests and `make test-api`; require real 412 handling plus an ID-resolution/reorder race test. Inspect recorder assertions: no root replacement, `/load`, `/stop` or unguarded writes. A larger read guard is allowed while the mutation remains small.
- [ ] Commit: `feat: reconcile conditional Caddy configuration mutations`.

## Task 4: Ordered collections

**Files:** Create `internal/caddy/ordering.go`, `ordering_test.go`.

**Interfaces:** Consumes live arrays and Object identities; produces Position, Reorder and OrderSatisfied. Task 3 performs the single guarded array replacement.

- [ ] Write failing `TestReorder` cases: append, before, after, satisfied relative order with intervening siblings, shared anchors, release constraint, self-reference, missing/wrong-parent anchor, contradictory constraints and preserved sibling order. Assert stable IDs and no delete/insert mutation pair.
- [ ] Run `go test ./internal/caddy -run 'TestReorder' -v`; expect FAIL until ordering exists.
- [ ] Implement relative-order semantics from the spec. Array replacement is used only for an actual move; field edits do not call Reorder unnecessarily. Recompute after 412 rather than replaying indexes.
- [ ] Run unit tests and the real Caddy movement case in `make test-api`; expect PASS with one guarded write per successful move.
- [ ] Commit: `feat: support stable before and after ordering`.

## Task 5: Provider schema and state ownership

**Files:** Create `main.go`, `internal/provider/{provider,state,identity,ordering}.go`, matching tests, and `internal/testutil/cli.go`.

**Interfaces:** Consumes Options and mutation types; produces `New(version string) func() provider.Provider`, `PlanFields`, ownership metadata codec and common plan modifiers. Registration is extended as Tasks 6–8 land. Expose optional/computed `object_id`, computed `id`, and optional ordering attributes on eligible resources.

- [ ] Write failing `TestProviderSchema`, `TestOwnershipTransitions`, `TestIDPlan` and `TestOrderingDriftPlan`. Assertions: no raw/dynamic configuration fields, required endpoint, paired TLS files, UUID v4 generated only in Create, explicit-ID change replaces, omitted established ID stays stable, configured-to-omitted optional values remove live fields, never-configured values survive, constraint drift plans an update.
- [ ] Run `go test ./internal/provider -run 'TestProviderSchema|TestOwnershipTransitions|TestIDPlan|TestOrderingDriftPlan' -v`; expect FAIL for missing implementation.
- [ ] Implement the provider configuration and private JSON metadata containing owned-field names. For optional collections needing observed import values, use typed optional/computed nested attributes (`match = [{ hosts = [...] }]`), avoiding computed nested blocks. Keep required upstreams as typed `upstream { dial = ... }` blocks. For removal, plan the public empty/default equivalent and delete the JSON field; omit never-owned values by retaining their computed observed state.
- [ ] Add computed `order_satisfied` on ordered resources: Read sets actual satisfaction and ModifyPlan sets true when a configured constraint needs repair. It is provider status, not a user configuration field. Omitted constraints are satisfied. Import does not invent anchors. Verify this produces a real update without modifying a known configured value.
- [ ] Run focused tests and a protocol-6 schema validation test; expect PASS. Do not proceed to resource CRUD while null/empty/import or ordering-plan cases fail.
- [ ] Commit: `feat: add typed provider schema and field ownership tracking`.

## Task 6: HTTP server lifecycle

**Files:** Create `internal/provider/resource_server.go`, `resource_server_test.go`, `testdata/first-release/server.tf`; register resource in `provider.go`.

**Interfaces:** Consumes client, ownership and identity helpers; produces `NewHTTPServerResource() resource.Resource`. Initial schema is name, listen, optional/computed object_id and computed id.

- [ ] Write failing `TestServerLifecycle` and `TestServerImport` assertions: generate/reuse UUID, strict name/ID collision errors, listen update retains routes and unknown fields, import discovers name/read-only, rename replaces, delete removes whole subtree but retains siblings, confirmed absence succeeds, inaccessible endpoint does not remove state.
- [ ] Run `go test ./internal/provider -run 'TestServer' -v`; expect FAIL for missing lifecycle behavior.
- [ ] Implement schema, Configure/Create/Read/Update/Delete/ImportState with context deadlines and typed diagnostics. Initialize only missing shared maps through guarded merges. Server identity creation uses map placement; routine subsequent reads/writes use ID-relative paths where safe. New first-release servers explicitly set `automatic_https.disable=true` as their HTTP-only creation default; imports preserve live TLS/automatic-HTTPS settings. The TLS increment adds typed controls. Add a real host-matched high-port HTTP test proving that no automatic certificate issuance starts.
- [ ] Run focused tests and real API server lifecycle. Assert listener-only change targets `/id/<server-id>/listen`, not the whole server. Expect PASS and no mutation during import/refresh.
- [ ] Commit: `feat: manage Caddy HTTP servers`.

## Task 7: Host routes and ordering lifecycle

**Files:** Create `internal/provider/resource_route.go`, `resource_route_test.go`, `testdata/first-release/route.tf`; register in `provider.go`.

**Interfaces:** Consumes server identity, ownership and ordering; produces `NewHTTPRouteResource() resource.Resource`. Schema: server_id, match, terminal, group, object_id, id, before_id, after_id and computed order_satisfied.

- [ ] Write failing `TestRouteLifecycle`, `TestRouteImport` and `TestRouteOrdering`: empty active route is accepted; match uses OR across typed host sets; terminal/group removal is actionable; handler children survive changes; parent movement requires replacement; import discovers server_id and retains observed optional values; wrong-parent IDs error.
- [ ] Run `go test ./internal/provider -run 'TestRoute' -v`; expect FAIL for missing lifecycle behavior.
- [ ] Implement guarded collection append/insert, owned-field updates and one-request moves. No activation sentinel/resource. Import requires an ID-bearing server; unsupported matcher shapes must either be safely preserved or explicitly rejected.
- [ ] Run focused tests plus real API route tests. Expect PASS, unchanged handler IDs and no-op second update. Anchor changes and detected order drift must update in place.
- [ ] Commit: `feat: manage host routes and relative ordering`.

## Task 8: Reverse proxy lifecycle and selective upstream edits

**Files:** Create `internal/provider/resource_reverse_proxy.go`, matching tests, `testdata/first-release/proxy.tf`; register in `provider.go`.

**Interfaces:** Consumes route identity, ownership and ordering; produces `NewReverseProxyResource() resource.Resource`. Schema: route_id, upstream blocks with dial, object_id, id, before_id, after_id and computed order_satisfied.

- [ ] Write failing `TestProxyLifecycle`, `TestProxyImport` and `TestProxySelectiveUpdate`: actual request reaches synthetic backend A, changed dial reaches B, resource/route IDs stay stable, nested unknown fields survive a dial-only change, structural upstream edits preserve other handler fields, wrong handler type/parent import fails, move does not delete/recreate.
- [ ] Run `go test ./internal/provider -run 'TestProxy' -v`; expect FAIL until implementation.
- [ ] Implement static upstream serialization and guarded updates. A single existing upstream dial edit targets `/id/<handler-id>/upstreams/0/dial` under an adequate guard. No transport, health or raw configuration fields in this increment.
- [ ] Run focused tests and real proxy requests. Expect PASS, exact selective path and preserved server/route/other-handler data.
- [ ] Commit: `feat: manage reverse proxy handlers with selective updates`.

## Task 9: Both-CLI lifecycle and failure acceptance

**Files:** Create `internal/acceptance/{lifecycle,ownership,ordering,recovery}_test.go`; extend `cli.go`, `testdata/first-release/` and `Makefile`.

**Interfaces:** Consumes built provider, Fixture and RunCLI; produces `make test-acceptance` accepting `IAC_BIN` and `CADDY_BIN`. Test source is explicitly local.sebclem.fr/sebclem/caddy in both CLIs, with a temporary filesystem mirror and no production backend.

- [ ] Write failing `TestAccLifecycle`, `TestAccOwnership`, `TestAccOrdering`, `TestAccRecovery`, parameterized by IAC_BIN. Assert plan actions are Update for ordinary edits and replacement only for documented identity/parent changes; import is read-only; second `plan -detailed-exitcode` is 0 and apply writes zero mutations.
- [ ] Run the suite once per pinned CLI; expect failures exposing any unfinished behavior. A skipped/missing executable does not count as compatibility passing.
- [ ] Complete runner and fixtures: build provider into a temporary mirror version `0.1.0`, run init/validate/plan/apply/show/refresh-only/import/destroy, parse plan JSON, record HTTP writes and IDs. Include omitted nested matcher state, unknown references resolved during apply, shared anchors, externally reordered arrays, subtree disappearance, rejected config and disconnect after committed create. Seed external mutations only through test fixtures.
- [ ] Run `IAC_BIN=/absolute/path/to/tofu CADDY_BIN=/absolute/path/to/caddy make test-acceptance` and the same command with Terraform. Expect PASS, no leaked child processes, no sensitive diagnostics, no duplicate objects and all missing/unreachable cases distinguished. Fix any failed core requirement before release.
- [ ] Commit: `test: verify provider lifecycle with OpenTofu and Terraform`.

## Task 10: Reproducible usage and release validation

**Files:** Create `examples/basic/main.tf`, `docs/resources/{http_server,http_route,handler_reverse_proxy}.md`, `docs/testing.md`, `README.md`; finalize Makefile.

**Interfaces:** Consumes the tested public schema and local mirror; produces `make check` and documented `make test-acceptance` for each CLI. Build target emits `bin/terraform-provider-caddy`; generated binaries/state remain ignored.

- [ ] Write the basic typed example with static loopback backend, explicit source, generated UUIDs and an optional ordering example. Document read-only ID import, parent-ID requirement, subtree deletion, HTTP/HTTPS connection setup, supported test versions and current feature limits.
- [ ] Run `go test ./...`, `go vet ./...`, `go build ./...` and formatting checks. `go test ./...` may skip opt-in process acceptance, but release checks must separately run it explicitly for both CLIs. Document every skip.
- [ ] Validate/apply the shipped example with each CLI against a temp Caddy instance; require an empty second plan and zero extra mutations. Confirm schema inspection contains no raw configuration or activation resource.
- [ ] Commit: `docs: document first Caddy provider release and compatibility checks`.

## Self-review and completion gate

Identity: Tasks 2, 5–9. Transport: Task 1. Partial updates/concurrency: Tasks 3, 6–9. Ordering: Tasks 4, 5, 7–9. Owned-field transitions: Tasks 5, 7, 9. Import and subtree deletion: Tasks 6–9. Both-CLI compatibility and installation: Tasks 9–10. Later-feature requirements are deliberately assigned to the companion delivery plan.

Do not mark the milestone complete until all required real API and both-CLI checks pass. Any Caddy rejection of an intermediate empty structure must be solved with the smallest valid object; do not add staging/activation. Stop and report an incompatible API/state contract rather than silently dropping a requirement. No production cutover or push is part of this plan.
