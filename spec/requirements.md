# Caddy provider requirements

## Scope

Build a standalone provider in `git@ssh.git.sebclem.fr:sebclem/terraform-provider-caddy.git` to manage existing Caddy instances through the native JSON administration API.

Migrate only Caddy configuration from the Ansible repository. Installation, service management, file provisioning, DNS and other components remain outside the provider.

## Configuration and updates

- Expose typed Terraform attributes and nested blocks. Do not accept raw JSON configuration inputs.
- Apply live configuration changes through the Caddy JSON API.
- Support in-place updates with stable resource identity.
- Support partial updates that modify only the necessary configuration scope and preserve unrelated configuration.
- Use stable Caddy `@id` identifiers wherever supported.
- Use a hybrid resource model: independently managed components with typed nested settings, rather than a resource for every individual setting.

## Terraform and OpenTofu compatibility

The provider MUST support both Terraform and OpenTofu. OpenTofu compatibility is a release requirement, not an assumed consequence of Terraform support.

- Provide the same resource schemas, configuration semantics and lifecycle behavior with both tools.
- Verify provider installation and initialization, validation, planning, apply, refresh, import and destroy with both tools.
- Test in-place updates, partial updates, drift detection and a no-change plan after a successful apply with both tools.
- Document the supported and tested versions of Terraform and OpenTofu for each release.
- Document provider installation and usage for both tools; do not require Terraform-only tooling for normal OpenTofu usage.

## Incremental delivery

Deliver features incrementally. The proposed first milestone covers connection to an existing admin API, an HTTP server, a host-matched route and a reverse-proxy handler with typed upstream settings. Validate lifecycle operations and selective updates against an isolated Caddy instance.

Subsequent increments cover only repository-used behavior: static proxy options and nested health checks; typed matchers, trusted proxies, child routes and authentication; existing-file certificates and HTTP-to-HTTPS redirects; named logs, independent hostname bindings and a high-level error-page resource; per-host metrics. Named routes and relay support are deferred until needed. Production migration follows coverage of the active behavior.

## Agreed lifecycle and connection decisions

- Servers, routes and handlers are independently managed; handler settings such as upstreams are typed nested configuration.
- No activation or deactivation mechanism is provided. Incomplete active routes during apply are acceptable.
- Import is supported only for existing objects with Caddy `@id`. Objects without IDs, including metrics and hostname log bindings, cannot be imported.
- Support mutually exclusive `before_id` and `after_id` ordering constraints.
- Generate UUID v4 identifiers when `object_id` is omitted; preserve established IDs across updates.
- Support HTTP and HTTPS, including custom CA trust and optional mutual TLS. Unix sockets are excluded.
- Updates preserve unrelated fields and independent children. Deletion removes the entire resource subtree.
- Assume OpenTofu is the sole configuration manager of an instance. Terraform is a supported alternative.
- Rebuild the active configuration from OpenTofu at cutover; downtime is acceptable. Do not import or add IDs to the current ID-less Caddyfile objects.
- Keep the Caddy admin listener outside provider management. Existing certificate files and their renewal/reload workflow remain operator responsibilities.
- Support only features used by this repository; named routes and relay support can follow the active migration.

The detailed design is in [Caddy provider design](design.md). It is approved for implementation planning; the implementation plans are in `docs/superpowers/plans/`.
