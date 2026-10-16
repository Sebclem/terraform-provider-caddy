# Active Caddy Implementation Plan

> REQUIRED SUB-SKILL: superpowers:executing-plans. Approved scope: caddy-delivery.md; execution inline in current checkout.

**Goal:** Deliver all active-site behavior, excluding inactive relay/named routes.
**Architecture:** Extend existing typed resource engine, retain ID/ownership/guarded mutation semantics; typed codecs for new fields, dedicated state handling for ID-less resources.
**Tech Stack:** Existing pinned Go/framework/Caddy and both CLIs.
**Spec:** spec/design.md and spec/requirements.md.

## Global constraints
Typed inputs only, UUID defaults, @id-only imports, static upstreams, subtree deletion, externally managed files/admin endpoint, no production writes.

## Review focus
Parent replacements with explicit IDs; optional field removal after import; nested partial writes preserving unknown sibling fields; forged forwarded auth headers; ID-less resources' collision and recovery behavior.

### Task 1: Legacy fixtures
- [x] Capture all resolved sites using actual loader method and synthetic credentials; adapt real template.
- [x] Test repeatable adaptation (`TestLegacyFixture`) with Caddy 2.11.7.
- [x] Record loader inheritance and effective execution order; commit fixtures.

### Task 2: Routing, proxy options and auth
Files: resource_model.go, resource_schema.go, engine.go, typed_fields.go, handler_templates.go, feature acceptance tests.
Interfaces: existing managed and caddyResource; additional typed fields use schema-defined attr.Value codecs. Matchers add paths/methods/client_ip_ranges/not (one negation level). Routes take exactly one server_id/subroute_id. Handler constructors select fixed native handler types.
- [x] Add failing CLI tests for child routes, typed TLS transport/health/header/buffer options, trusted proxies, basic auth, error and forward-auth resources; assert traffic and second-plan convergence.
- [x] Add schema-defined codecs; use Caddy's adapted forward-auth response routing, preserving original request/body and deleting missing auth headers.
- [x] Extend engine type/parent checks and nested minimal edits; retain independent children.
- [x] Run API and both-CLI lifecycle, selective update/import tests, then document resources and commit.

### Task 3: TLS
Files: typed_fields.go, engine.go, TLS tests/docs.
Interfaces: certificate entries live under apps/tls/certificates/load_files; UUID per pair. Server https boolean emits TLS connection policies and disable_certificates automatic HTTPS mode; HTTP redirects enabled by Caddy. Certificate IDs in server depends_on establish ordering.
- [x] Add failing real HTTPS/SNI/redirect tests with synthetic files; path update and missing file rejection.
- [x] Implement file loader CRUD with existing engine and guarded ancestor creation. Never inspect remote files from provider.
- [x] Verify both CLIs, no issuance, no-change second plan; document same-path renewal/reload and commit.

### Task 4: Logs/errors
Files: typed_fields.go, handler_templates.go, resource_settings.go and acceptance tests/docs.
Interfaces: named logs own logging/logs/name; explicit singleton default exclusions belong to a log resource named default. Host binding owns server/logs/logger_names/hostname. Error-page resource owns one subroute/errors/routes entry and internal sequence.
- [x] Add failing traffic tests for access record destination/mode, sibling hostname retention, generated error pages including missing file, import rejection of nonconforming sequences.
- [x] Implement guarded map/singleton settings CRUD, no import for bindings; deterministic composite state identity; preserve siblings.
- [x] Verify import, in-place updates, subtree deletion, both CLIs; document and commit.

### Task 5: Metrics, examples and parity
Files: resource_settings.go, examples/home, parity/metrics tests and docs.
Interfaces: metrics owns apps/http/metrics only; singleton ID http-metrics, no import.
- [x] Add failing metrics traffic/toggle tests and example lifecycle tests.
- [x] Implement per_host setting and typed examples composed from resolved manifest; cover all 49 sites, four synthetic basic-auth accounts, certificate groups, independent site subroutes, ordered auth/access/proxy routes and Jellyfin metrics access-check/proxy continuation.
- [x] Full both-CLI matrix, zero-change second plans, vet/race; commit evidence.

### Task 6: Rebuild handoff and review
- [x] Document export, stop Ansible writes, externally clear old app/logging config, API persistence/resume, apply and recover under operator control.
- [x] Rehearse sanitized legacy rebuild and recovery on isolated Caddy only; record evidence.
- [x] Fresh whole-branch review; fix important findings with regression tests; final verification and commit. Keep changes local, no production cutover or push.
