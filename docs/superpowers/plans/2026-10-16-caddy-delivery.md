# Caddy Provider Delivery Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development or superpowers:executing-plans to implement an approved increment task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Cover the active Ansible repository's Caddy behavior through incremental, typed OpenTofu/Terraform releases, then prepare a rebuild cutover.

**Architecture:** Reuse the first release's transport, mutation engine and ownership rules. Add typed settings to resources when they share a lifecycle, and independent resources for routes, handlers, certificate loading, logs and log bindings. A high-level error-page resource owns its fixed internal handler sequence.

**Tech Stack:** Go and Terraform Plugin Framework; pinned Caddy v2.11.7 fixture and both CLI acceptance suites from the [first release plan](2026-10-16-caddy-first-release.md).

**Spec:** [Design](../../../spec/design.md), especially “Later increments for the active repository”; [requirements](../../../spec/requirements.md).

## Global Constraints

- All increments inherit the design's typed-interface, stable-ID, partial-update, subtree-delete and both-CLI requirements.
- Static upstreams only. HTTP/HTTPS connection only. No route activation/deactivation.
- Import only where an existing Caddy `@id` can identify the resource; no metrics import.
- Admin listener and certificate provisioning/renewal/reload remain external.
- Limit scope to current repository features. Named routes/relay are deferred and do not block active migration.
- Rebuild at cutover; downtime is acceptable. Planning or tests must never mutate production.

## Review Focus

- Caddyfile directive order differs from source order: pin effective auth/access/error behavior with adapter fixtures in Increment 2.
- The existing YAML loader has inheritance rules and custom fragments: validate resolved behavior, not README examples, in the fixture task and cutover task.
- A certificate path edit and file contents changing at the same path have different operational behavior: distinguish them in Increment 3 tests/docs.
- Per-host logging requires both destination and routing, and shared default filters can duplicate records: verify destinations in Increment 4.
- Singleton metrics and hostname log bindings do not naturally carry IDs: maintain the strict import rule in Increments 4–5.

## How to use this roadmap

The first release already has an executable task plan. Later sections below specify deliverables, exact target files and acceptance boundaries. Before implementing each later increment, produce a short detailed task plan grounded in its adapted fixture and the actual preceding code. This prevents promising unverified JSON schemas while still fixing the approved feature scope now. A completed release must remain usable while later increments are pending.

## Shared fixture task: Capture the active legacy behavior

**Files:** Create `testdata/legacy/{README.md,manifest.yaml}`, sanitized rendered `.caddy` fixtures and adapted `.json` fixtures; create `internal/acceptance/legacy_test.go`. Read the Ansible repository's `playbooks/templates/Caddyfile.j2`, `playbooks/config/home/`, inventory variables and actual Caddy loader source. Never read/decrypt vault credentials for fixtures.

- [ ] Build a sanitized configuration manifest from the active YAML inputs and loader behavior. Preserve explicit versus inherited settings and hostname spelling; replace secrets with synthetic password hashes. Report any unavailable loader or unresolved Jinja source instead of guessing.
- [ ] Adapt with pinned Caddy v2.11.7 in a temporary fixture. Retain route and handler array order; normalize only irrelevant generated names/IDs. Run `CADDY_BIN=/absolute/path/to/caddy go test ./internal/acceptance -run TestLegacyFixture -v` and require PASS for repeatable rendering/adaptation.
- [ ] Commit fixtures and evidence. This task is independent of production installation/DNS/monitoring and should land before claims of feature parity.

## Increment 1: First working release

- [x] Execute [First Release Implementation Plan](2026-10-16-caddy-first-release.md), Tasks 1–10.
- [x] Require real proxy traffic, in-place/selective edits, import, ordering, subtree deletion and zero-write second applies with both CLIs.

## Increment 2: Proxy settings, routing and authentication

**Files:** Extend `internal/provider/resource_reverse_proxy.go`, `resource_route.go`, `resource_server.go`; create `resource_subroute.go`, `resource_forward_auth.go`, `resource_basic_auth.go`, `resource_error.go` and matching tests/docs. Extend `internal/acceptance/proxy_options_test.go`, `access_test.go` and fixtures.

- [ ] Add typed nested HTTP transport, request-header operations, request/response buffers, flush interval and active-health settings. Pin JSON values to the fixtures for TLS verification bypass on Frigate/Proxmox/Unifi, Unifi Host override, Speedtest 35MiB buffers/negative flush, and health URI/follow-redirects/fails=3/timeout=20s. Observe backend health probes and verify unrelated handler fields survive an edit.
- [ ] Extend typed match objects with paths, methods, client IP ranges and a nonrecursive typed negated matcher set sufficient for repository expressions. Extend the server with trusted-proxy CIDRs. Verify method AND nonlocal-IP logic and direct/trusted/untrusted IPv4/IPv6 request behavior.
- [ ] Add independent subroute and child-route support. Preserve existing server_id configuration; add mutually exclusive subroute_id, with exactly one parent required. Parent changes replace; child content/order edits are in-place. Test Jellyfin's exact `/metrics` handling against the legacy fixture rather than assuming fallback reaches the app.
- [ ] Add typed forward-auth (upstream, URI, copied headers), basic-auth (sensitive username/hash account collection) and error (status/message) resources. Forward-auth must preserve original method/body, copy selected successful auth headers and return non-success auth responses. Verify auth/access ordering and VMagent basic authentication with synthetic credentials.
- [ ] Run focused tests plus both CLI acceptance suites; require exact selective-update assertions for nested health/transport edits and a zero-write second apply. Commit each independently tested feature task, then its docs.

## Increment 3: Existing-file TLS and redirects

**Files:** Create `internal/provider/resource_tls_certificate.go`, matching tests/docs and `internal/acceptance/tls_test.go`; extend `resource_server.go`; create test-only certificate generation helpers. Add an independent TLS connection-policy resource only if required by the pinned legacy fixture.

- [ ] Finalize typed certificate-file schema from Caddy's loader: identity, certificate path, key path and any fixture-required format/tags. Paths refer to the Caddy host; do not read them from the runner. Share a file pair across sites using references/dependencies.
- [ ] Establish native JSON TLS policies and automatic-HTTPS flags from the fixture. Add typed server configuration that serves loaded certificates and creates HTTP-to-HTTPS redirects without ACME issuance. Use synthetic certificates and isolated ports to verify SNI selection and redirects.
- [ ] Verify path updates retain resource identity and unrelated certificate entries, invalid/missing host files produce Caddy diagnostics, and same-path file content changes produce no claimed provider refresh. Document the external renewal/reload responsibility.
- [ ] Run focused API/behavior tests and both CLI suites; require zero-write second apply and stable IDs. Commit implementation and TLS examples.

## Increment 4: Logs and high-level error pages

**Files:** Create `internal/provider/resource_log.go`, `resource_log_binding.go`, `resource_error_pages.go`, matching tests/docs, `internal/acceptance/logs_test.go`, `error_pages_test.go` and synthetic error-page files.

- [ ] Add typed named-log and nested file-writer settings. Use the fixture's effective mode/rotation values rather than unverified unit conversions. Check actual access record destinations, file permissions and rotation configuration.
- [ ] Add independent hostname binding resources referencing server/log IDs. Bindings own one map entry; they cannot import because the underlying value has no Caddy `@id`. Use a stable composite internal state identity for CRUD, never fabricate a Caddy ID. Change one binding without erasing sibling mappings. Handle shared default-log filtering explicitly and verify records are not accidentally duplicated.
- [ ] Add high-level error pages referencing a subroute, with typed root/page URI and generated stable IDs on its owned error route/internal objects where supported. Import by the top owned error route's ID only when the entire fixed sequence is recognized; reject partial/nonconforming objects. Internal children are never individually managed/imported.
- [ ] Verify error responses for 403/404/5xx and missing files against legacy behavior, root/page updates in place, log update independence and complete owned-subtree deletion. Prevent two resources from claiming the same error-page sequence or hostname binding.
- [ ] Run focused behavior tests and both CLI acceptance suites; commit implementation, fixture comparisons and usage docs.

## Increment 5: Metrics and complete active configuration

**Files:** Create `internal/provider/resource_http_metrics.go`, matching tests/docs and `internal/acceptance/metrics_test.go`; add `examples/home/` and extend legacy parity tests.

- [ ] Add typed per_host metric settings under HTTP configuration, with a documented singleton state identity and no importer. Preserve unrelated HTTP app fields; verify requests produce the expected per-host observations and toggling affects only metrics.
- [ ] Compose typed resources for every active legacy pattern. Do not require a YAML parser inside the provider; configuration transformation belongs to examples/modules. External certificate/error files stay external. Capture every remaining fixture mismatch with the owning increment and fix it before parity is claimed.
- [ ] Run the full fixture behavior matrix, all core lifecycle tests and both CLI suites. Require a zero-change second plan, no raw configuration inputs and documentation listing the exact tested CLI/Caddy versions.
- [ ] Commit complete examples and record that active configuration parity is verified. Named-route/relay absence is an intentional later scope boundary.

## Increment 6: Rebuild handoff

**Files:** Create `docs/migration.md` and sanitized `examples/home/README.md`; no changes to the Ansible installation role are implied.

- [ ] Document the controlled maintenance workflow: backup/export, external admin endpoint and API-compatible startup persistence, stop Ansible configuration writes/reloads, remove old configuration under operator control, apply typed OpenTofu configuration and validate service behavior. Downtime is acceptable; there is no ID-preparation/import step.
- [ ] Rehearse against a disposable instance containing sanitized legacy configuration, then rebuild through OpenTofu. Verify HTTPS/redirects, auth, restrictions, proxies, health checks, logs and metrics; repeat plan/apply with no changes. Test documented operator recovery separately from provider CRUD.
- [ ] Commit the runbook and rehearsal evidence. Production mutation requires a separately authorized cutover; writing this roadmap does not execute it.

## Deferred increment: Named routes and relay

- [ ] When needed, design independent typed named-route resources and invoke handlers, then relay composition and tests. Use a synthetic relay fixture because relay mode is currently inactive. No raw JSON escape hatch or production upstream is inferred.

## Self-review

Every later-feature section of the design maps to an increment above. All increments reuse first-release ID, ownership, guarded mutation and both-CLI checks. Remaining unknowns are native JSON details explicitly resolved by pinned fixtures before each detailed implementation plan; no untested parity is claimed. The provider does not acquire installation, admin-listener, file-renewal or external-service responsibilities through this roadmap.
