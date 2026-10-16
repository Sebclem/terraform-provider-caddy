# Caddy provider design

Status: Draft for review, 2026-10-16. Covers the first release and agreed later increments.

## Purpose and scope

Build a standalone provider at `git@ssh.git.sebclem.fr:sebclem/terraform-provider-caddy.git`. Manage Caddy configuration through its live JSON administration API, with equivalent support for Terraform and OpenTofu. OpenTofu is the intended sole configuration manager of each target instance; Terraform remains a supported alternative. Do not use both tools to manage the same objects concurrently.

The public interface uses typed attributes and nested blocks. No raw JSON, Caddyfile fragments, arbitrary configuration paths or untyped configuration payloads are accepted. JSON serialization is internal.

Installation, plugins, service management, certificate file provisioning, DNS, monitoring services and other infrastructure remain outside this provider. Unix sockets are unsupported. Production migration is deferred until incremental releases cover the required configuration behavior.

This design was developed from the current discussion and repository configuration, independently of the deleted specification. Detailed schemas below are proposals for review; behavioral decisions explicitly agreed in the discussion are requirements.

## Delivery approach

The first increment provides a complete HTTP reverse-proxy lifecycle using three independently managed resources: server, route and reverse-proxy handler. Upstreams are typed nested settings of the handler, not separate resources. Future handlers are separate resource types with their own typed settings.

Routes are active immediately, including while empty or incomplete during apply. There is no activation resource, staging sentinel, deactivation sequence or multi-resource transaction. Every individual mutation must still be accepted by Caddy. Partial routing behavior during assembly and destruction is accepted.

Later increments cover the specific features used by this repository, in the order below. Each increment has focused design and tests. The first increment is not full parity with the Ansible configuration. Named routes and relay support follow the active migration and do not block it. No goal of covering every built-in Caddy module is implied.

## Provider configuration

Implement in Go using Terraform Plugin Framework. Separate HTTP transport, mutation logic and typed resource schemas. Pin toolchain and library versions in the implementation plan; release compatibility claims require executable tests.

| Attribute | Proposed contract |
| --- | --- |
| `endpoint` | Required HTTP or HTTPS admin URL; no production endpoint discovery. |
| `ca_file` | Optional PEM CA file for HTTPS; otherwise use system trust. |
| `client_certificate_file` | Optional PEM client certificate for mutual TLS. |
| `client_key_file` | Required together with the client certificate. |
| `request_timeout` | Positive duration, default `30s`. |
| `conflict_retries` | Nonnegative retry limit, default 5. |

Verify HTTPS certificates and server names. Reject mismatched certificate/key settings. Resource operations have a bounded two-minute deadline, including retries, and honor cancellation. Provider aliases may address different instances. Do not log certificate keys or full configuration bodies. Planning, refresh and import perform no mutations.

## Identity

Every initial resource has optional/computed `object_id` and computed `id`. Both expose the same effective Caddy `@id`.

If omitted, generate a UUID v4 once during creation. Do not generate IDs during plan or regenerate them during updates, refresh, retries or import. An explicitly supplied ID may be a non-UUID string. Validate it is nonempty and safely encode it in API requests. IDs must be unique across the instance; detect collisions rather than silently adopting existing objects.

Changing an established explicit identity requires replacement. Removing `object_id` from configuration retains the existing computed value. Ordinary content changes are in-place. References use the computed `id`.

For a failed or uncertain create, retain the attempted identity in recoverable state where supported and include it in a diagnostic when necessary. Never generate a second ID merely because the first write response was lost.

## First increment resource schemas

### caddy_http_server

| Attribute | Contract |
| --- | --- |
| `name` | Required server map key; changing it requires replacement in the first increment. |
| `object_id`, `id` | Common identity contract. |
| `listen` | Required nonempty ordered list of listening addresses; updated in place. |

Own the server identity and listening addresses. Preserve independently managed routes and other server fields on update. Create under the HTTP server map, initializing missing containers with guarded merges. Never replace an existing server of the same name during creation.

### caddy_http_route

| Attribute | Contract |
| --- | --- |
| `server_id` | Required reference to the parent server ID; changing parent requires replacement. |
| `object_id`, `id` | Common identity contract. |
| `match` | Optional ordered list of typed matcher blocks, initially containing a nonempty `hosts` list. |
| `terminal` | Optional/computed boolean; effective default false for new routes. |
| `group` | Optional/computed string; absent by default. |
| `before_id`, `after_id` | Optional, mutually exclusive sibling ordering constraints. |

The first increment supports host matching only. Multiple match blocks represent alternatives; future matcher fields within one block will combine according to Caddy matcher semantics. An omitted match collection on a new route matches all requests. An explicitly empty collection also expresses no matcher restriction.

Own route identity, supported matcher fields and configured flags. Preserve the handler collection during route updates. Unsupported matcher fields encountered during import are preserved internally when an owned-field update can safely leave them intact. If the provider cannot safely represent or preserve the imported object, import fails with a precise diagnostic. Do not invent a simplified typed representation or expose a raw payload.

### caddy_handler_reverse_proxy

| Attribute | Contract |
| --- | --- |
| `route_id` | Required reference to the parent route; changing parent requires replacement. |
| `object_id`, `id` | Common identity contract. |
| `upstream` | Required nonempty ordered list of typed blocks, each with required `dial` string. |
| `before_id`, `after_id` | Optional, mutually exclusive sibling ordering constraints. |

Own handler identity, the reverse-proxy discriminator and configured upstream settings. Transport, health checks and headers are future typed extensions. Do not expose them as JSON in the first increment.

Nested upstreams share the handler lifecycle. For equal-length lists, changed dial fields can be updated by position under a fresh guarded handler snapshot. Structural list changes replace the upstream list while preserving unchanged nested content where correspondence is unambiguous. Never persist array indexes as standalone resource identity. Preserve existing nested IDs; additional nested IDs are not required merely to edit the list.

## Typed usage

This example illustrates schemas, not registry publication. Installation documentation must provide a tested local installation path for both CLIs before registry publication.

```hcl
provider "caddy" {
  endpoint = "http://127.0.0.1:2019"
}

resource "caddy_http_server" "main" {
  name   = "main"
  listen = [":8080"]
}

resource "caddy_http_route" "app" {
  server_id = caddy_http_server.main.id
  terminal  = true

  match {
    hosts = ["app.example.test"]
  }
}

resource "caddy_handler_reverse_proxy" "app" {
  route_id = caddy_http_route.app.id

  upstream {
    dial = "127.0.0.1:9000"
  }
}
```

Resource references establish creation and reverse destruction dependencies. Ordering references also establish dependencies; normal CLI cycle diagnostics apply to cyclic references.

## Ownership and state

Updates preserve fields outside the resource's managed projection and preserve independently managed children. Removing a previously configured optional setting removes that setting from Caddy; omission of a never-configured setting preserves the live value. Required collections, such as upstreams, are managed as collections once configured. An explicit collection edit can remove collection members.

Use private provider metadata to distinguish configured ownership from observed optional/computed values. Store which fields were explicitly managed, not only their current values. Plan modification must make removal of a previously managed field an actionable change; omission must not silently retain it through optional/computed defaults. Test configuration-to-omission transitions explicitly. After removal, release ownership unless the schema requires the field. No management masks or raw snapshots appear in the public interface.

Refresh returns live typed values for supported fields; configured values then drive a corrective plan when drift exists. Treat inaccessible endpoints, malformed responses and authorization failures as errors, not absence. A resource found under the wrong parent is a conflict, not an invitation to move or delete it.

Deletion intentionally removes the entire owned object's subtree, including remaining children and fields not represented in state. Do not refuse deletion because children remain. Do not delete shared ancestors or siblings. A child already removed by parent deletion is successfully absent. Normal dependency ordering should delete children first, but correctness must not rely on it after manual or targeted operations.

## Partial updates and concurrency

Caddy's API exposes ID-relative addressing. Its PATCH operation replaces the targeted value rather than applying JSON Merge Patch; POST appends to arrays, while PUT can insert. ETag and If-Match headers support conditional writes, and transactions do not span multiple requests. These API facts govern the provider's mutation strategy. [Caddy API](https://caddyserver.com/docs/api)

For each operation:

1. Read live configuration, resolve identity and parent, and select the smallest safe mutation scope.
2. Compare the desired owned values with the snapshot. Equivalent values require no write.
3. Prepare a field mutation or merge correlated changes into their smallest common ancestor, preserving other live fields and children.
4. Guard the mutation using the exact ETag of a scope covering its affected data and any array movement that can invalidate addressing.
5. On HTTP 412, refetch and recompute; use bounded backoff and the operation deadline. Do not replay stale merged bodies.
6. Read back and verify the resulting owned values before reporting success.

Use an endpoint-scoped mutex within a provider process to limit contention. Server-side guards remain necessary. Validate ID-resolution and array-reordering races against real Caddy. Missing conditional-write support produces an error rather than an unguarded fallback.

A dial-only change must not replace a route or server. A server listener change must preserve routes. Correlated field updates may replace the smallest safe enclosing object. Reordering is allowed to replace the containing array once, using a fresh snapshot and guard. Guard scope may be larger than mutation scope.

Routine CRUD must not call `/load`, replace the configuration root, stop Caddy, invoke a shell or manage services. In-place means stable provider resource identity; it does not promise that Caddy avoids internal reprovisioning.

After an ambiguous write response, inspect the intended identity before retrying. Reconcile success when the intended result is confirmed; otherwise retry only after establishing a safe current state. Never blindly repeat an append. Failures retain recoverable state and actionable diagnostics. A later failed resource does not roll back earlier successful resources or restore a whole-instance snapshot.

## Ordering

`before_id` and `after_id` reference another object in the same sibling collection. Validate type, parent and existence. Reject both attributes together, self-reference and detectable contradictions. Each constraint means relative order, not immediate adjacency, allowing multiple siblings to share an anchor.

With no constraint, append during creation and preserve position on content updates. Removing a constraint releases ordering control without moving the object. If a constraint is satisfied, do not move the object unnecessarily. Otherwise insert immediately before or after the anchor in a freshly read array, preserving the relative order of other elements. Change position in one guarded array write; never delete and reinsert through separate API calls.

Keep requested ordering intent in state. Refresh checks whether the live ordering still satisfies it, and planning schedules an update when it does not. Do not infer a permanent ordering constraint merely from the current neighbor during import. Cross-resource constraints must converge; tests must cover shared anchors and distinguish satisfiable constraints from cycles.

## Import

Import accepts only the existing object's Caddy `@id`. Resources whose Caddy location has no `@id`, including the metrics singleton, have no import support. No path-based import, content matching, automatic ID assignment or migration feature for objects without IDs is supported.

Read the object, verify its type and discover its server name or parent ID. For child resources, the immediate parent must also have an ID so the typed parent reference can be represented; otherwise reject import with a clear diagnostic. Importing a parent does not import its children.

Populate typed state from live values and retain the existing ID. Optional settings start observed, without claiming previously unmanaged fields solely through import. Subsequent configuration establishes field ownership. Required attributes must match the imported object for a no-change plan. Refresh and import never mutate the instance.

## Validation and compatibility

Test the first increment against an isolated Caddy matching the Ansible inventory baseline, currently `v2.11.7`. Verify that version is available when building the fixture. Pin the exact test image or binary. Additional version support requires testing.

Run lifecycle scenarios with both Terraform and OpenTofu, not only framework unit tests. OpenTofu supports provider plugins and local provider installation workflows; validate the selected packaging through its actual CLI. [OpenTofu provider documentation](https://opentofu.org/docs/language/providers/), [plugin installation](https://opentofu.org/docs/cli/plugins/)

Required checks:

- Installation/init, validation, plan, apply, refresh, import and destroy on both CLIs.
- UUID generation, explicit IDs, collision rejection, stable identity and read-only imports.
- Empty live routes and independently created handlers, with no activation mechanism.
- Actual proxied requests, upstream changes and server listener changes.
- No-change plan and zero mutation requests on a second apply.
- Exact bounded mutation paths; preservation of other resources and unmanaged fields.
- Optional field ownership, omission/removal, import adoption and drift repair.
- Before/after insertion, movement, shared anchors, missing anchors and ordering drift.
- Concurrent changes, 412 retries, stale indexes and lost write responses without duplicate creates.
- Subtree deletion, children already absent and preservation of sibling objects.
- HTTP, HTTPS, custom trust, mutual TLS, cancellation and certificate errors.
- Invalid Caddy configuration: useful diagnostics and no successful state for rejected values.

Use unit tests for serialization and transport failures, real Caddy for API semantics, and CLI acceptance tests for state behavior. Record tested Terraform/OpenTofu versions per release. State updates must satisfy framework consistency requirements. [Terraform resource update guidance](https://developer.hashicorp.com/terraform/plugin/framework/resources/update)

## Later increments for the active repository

The later resource names describe intended typed boundaries. They must be checked against adapted Caddy v2.11.7 JSON before their schemas are finalized. Every later resource has the same guarded CRUD, drift and OpenTofu/Terraform lifecycle requirements as the first three resources. Support only behavior used by the repository; add other modules only through a separate design decision.

### Proxy options

Extend `caddy_handler_reverse_proxy` with typed nested HTTP transport, request header operations, request and response buffer sizes, flush interval and active health check. Upstreams remain static. A hostname in a static `dial` value is still a static upstream; the `dynamic_upstreams` modules for DNS A/AAAA and SRV discovery are excluded.

The transport must allow TLS for selected backends and explicitly requested certificate verification bypass, without making insecure verification a global default. Request headers must cover Unifi's upstream `Host` override. Buffer and flush settings must cover Speedtest's configured values, including its negative flush interval. The active health check must cover URI, redirect following, failure count and timeout. Additional hostnames in the legacy loader have no health check by default; migration examples must preserve that distinction.

Each nested option is updated through the smallest safe API scope within the reverse-proxy handler. A transport edit does not replace the route. The handler's `@id` remains stable.

### Routing and access control

Extend route matcher blocks with typed path, method and client-IP conditions. Conditions within a block are ANDed; matcher blocks are ORed. A typed `not` block represents Caddy's negated matcher sets, including the current whitelist and WebDAV rules. Use the actual `client_ip` matcher and a typed trusted-proxies setting on `caddy_http_server`, so forwarded addresses are trusted only from configured proxy ranges. Validate direct and proxied IPv4/IPv6 traffic, including spoofed headers from untrusted peers.

Introduce `caddy_handler_subroute`, with independently managed child `caddy_http_route` resources. A child route names its parent subroute instead of a server. Child handlers remain independent. Route ordering and identity rules apply within each child collection. This supports the Jellyfin exact `/metrics` path and fallback behavior without embedding opaque route arrays.

Introduce `caddy_handler_forward_auth`, `caddy_handler_basic_auth` and `caddy_handler_error`. The forward-auth resource has typed upstream, URI and copied-header settings and internally serializes Caddy's actual proxy precheck. Basic-auth accounts use typed username and password-hash fields, marked sensitive; hashes are not treated as plaintext. Error handlers cover status and message for the 403 denial cases. The provider must preserve authentication-before-application ordering through `before_id`/`after_id`. Compare adapted Caddyfile behavior for Authelia, VMagent, whitelist combinations, WebDAV methods and Jellyfin before claiming parity.

### TLS and redirects

Introduce an independent `caddy_tls_certificate` resource for an existing certificate/key file pair on the Caddy host. File provisioning, renewal and reloading after same-path content changes are operator responsibilities. Path changes update the configured file loader in place. Certificate resources are shared when sites use the same pair. No ACME issuance support is included.

Add typed server TLS/automatic-HTTPS settings needed to serve those certificates and provide HTTP-to-HTTPS redirects. The exact generated JSON, SNI certificate selection and redirect configuration must be established with a pinned Caddy adaptation fixture; do not assume a host-matched HTTP route alone configures TLS. Test that a site serves its intended certificate, redirects HTTP, and does not issue an unexpected certificate. Add an independent TLS connection-policy resource only if the fixture shows that an explicit policy is necessary to reproduce the current behavior.

### Logs and error pages

Introduce independent `caddy_log` and `caddy_log_binding` resources. A log owns a named destination and typed file-writer options, including path, mode, rotation size and local-time option. A binding owns one hostname-to-log association within a server and updates the corresponding map entry without replacing other host mappings. The provider must preserve the effective default-log filtering behavior of the current Caddyfile; any shared filter has one explicit owner.

Introduce a high-level `caddy_error_pages` resource attached to a subroute. It owns the error route and its ordered root, rewrite and file-server handlers, with typed page root and page URI. It generates stable internal IDs where supported, but its internal handlers are not separate public resources or individually importable. An error-page setting change updates this resource in place. Keep its request status behavior and missing-file behavior aligned with the adapted configuration. Error-page files themselves remain external.

### Metrics and later inactive features

Introduce a typed `caddy_http_metrics` resource for the current per-host setting. The underlying Caddy metrics node has no natural `@id`; this resource has a documented singleton state identity and **does not support import**, in line with the ID-only import rule. Creation and updates must preserve unrelated HTTP application settings.

Named routes and relay configuration are inactive in the current deployment. Add independent typed named-route and invoke-handler support, plus relay composition, in a later increment when needed. Their absence does not block migration of the currently active configuration.

### Migration coverage

Before cutover, use a safe fixture to render and adapt the existing Caddyfile and inventory. Produce typed OpenTofu examples for every active site pattern and verify route order, authentication, proxy settings, TLS, logs and errors against that fixture. No raw JSON or Caddyfile input appears in the provider configuration. Do not run the installation, DNS or monitoring tasks to obtain the fixture.

The cutover **rebuilds** the active Caddy configuration from OpenTofu. Downtime during the controlled rebuild is acceptable. Existing Caddyfile objects without `@id` are not imported or patched to add IDs. Retain a backup and an operator recovery procedure, then stop Ansible from writing and reloading that configuration. Apply the complete typed configuration and check HTTPS, redirects, authentication, access restrictions, proxy responses, health checks, logs and metrics. The provider does not own the admin listener or service startup settings.

## Operational handoff

Ansible currently writes and reloads a Caddyfile. Before production cutover, stop that configuration ownership and arrange startup persistence for API-managed configuration. The admin listener remains an external prerequisite because changing it through the same connection can disconnect the provider. This is an operator prerequisite, not service-management functionality in the provider. Caddy documents saved configuration and resume startup behavior. [Caddy API persistence](https://caddyserver.com/docs/api)

The first milestone is complete when its three resources satisfy the acceptance checks on both CLIs, with documented installation and usage. Full repository migration is a later milestone after the necessary typed features exist.
