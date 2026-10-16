# Caddy provider for OpenTofu and Terraform

Manage a running Caddy instance through typed OpenTofu/Terraform resources and its live JSON administration API. Resources support stable UUID identity, in-place updates, selective guarded writes and import by existing Caddy `@id`.

The active-site feature set includes static proxy options and health checks, nested routes, trusted client IPs, forward/basic authentication, existing-file TLS with redirects, access logs, error pages and per-host metrics. See the [49-site example](examples/home/README.md) and [rebuild runbook](docs/migration.md). Inactive named routes and relay remain deferred. New servers default to HTTP; `https = true` enables loaded-certificate serving and redirects while disabling certificate issuance.

## Caddy prerequisite

Caddy must have an existing configuration object. Starting `caddy run` without any configuration exposes a null document that Caddy cannot initialize through a subtree API write. Initialize Caddy externally, for example with this admin-only Caddyfile:

```caddyfile
{
  admin 127.0.0.1:2019
}
```

Use the existing service workflow to load that bootstrap before applying resources. The provider can then create missing application/server containers. During a rebuild, retain the configuration object and admin endpoint; remove the old application configuration under operator control. Root configuration replacement remains outside provider CRUD.

## Build and local installation

Use Go 1.27.2. Run `make build` to create `bin/terraform-provider-caddy`. Until registry publication, install through a filesystem mirror with the explicit source address `local.sebclem.fr/sebclem/caddy`. That address is a local development identifier; it does not require a registry service.

For Linux amd64, from the repository root:

```sh
make build
mkdir -p .tools/provider-mirror/local.sebclem.fr/sebclem/caddy/0.1.0/linux_amd64
cp bin/terraform-provider-caddy .tools/provider-mirror/local.sebclem.fr/sebclem/caddy/0.1.0/linux_amd64/terraform-provider-caddy_v0.1.0
```

Create `.tools/cli.tfrc` with an absolute mirror path:

```hcl
provider_installation {
  filesystem_mirror {
    path    = "/absolute/path/to/terraform-provider-caddy/.tools/provider-mirror"
    include = ["local.sebclem.fr/sebclem/caddy"]
  }
}
```

For another platform, use the matching `GOOS_GOARCH` directory and executable suffix. Set `TF_CLI_CONFIG_FILE` to the absolute `.tools/cli.tfrc` path for either CLI. Then use the [basic example](examples/basic/main.tf):

```sh
cd examples/basic
tofu init
tofu plan
tofu apply
```

The same configuration works with `terraform`. Use one CLI/state owner for an instance. Supply an externally configured admin endpoint and a running backend; the example defaults to ports 2019 and 9000, and listens on 8080. Test routing with `curl -H 'Host: app.example.test' http://127.0.0.1:8080/`.

## Resources and lifecycle

- [caddy_http_server](docs/resources/http_server.md): listeners, trusted proxies and HTTPS.
- [caddy_http_route](docs/resources/http_route.md): typed matchers, parent server/subroute and relative order.
- [caddy_handler_reverse_proxy](docs/resources/handler_reverse_proxy.md): static upstreams, HTTP/TLS transport, headers, buffers and health.
- [caddy_handler_subroute](docs/resources/handler_subroute.md): independently managed nested routes.
- [caddy_handler_forward_auth](docs/resources/handler_forward_auth.md), [caddy_handler_basic_auth](docs/resources/handler_basic_auth.md), [caddy_handler_error](docs/resources/handler_error.md).
- [caddy_tls_certificate](docs/resources/tls_certificate.md): existing certificate/key files on the Caddy host.
- [caddy_log](docs/resources/log.md) and [caddy_log_binding](docs/resources/log_binding.md): named destinations and per-host routing.
- [caddy_error_pages](docs/resources/error_pages.md): shared server-level or per-subroute root/rewrite/file-server error handling.
- [caddy_http_metrics](docs/resources/http_metrics.md): singleton per-host observations.

Configuration uses typed HCL attributes/blocks. For ID-bearing resources, optional `object_id` defaults to a UUID v4 generated at creation; `id` exposes the same stable Caddy identifier. Changing content updates the existing resource. Changing an explicit identity, server name or parent replaces it. Omitting a previously explicit identity preserves it.

`before_id` and `after_id` are mutually exclusive and refer to siblings. They enforce relative order, not adjacency. Removing the constraint preserves the current position. References should use other resources' `id` values so the CLI can order dependencies and reject reference cycles.

Updates preserve independently managed children and unowned settings. Deleting an object removes its entire subtree, including remaining children. Routes are active throughout creation/destruction, including when empty. Applies are multiple independent operations; earlier successful changes are retained if a later operation fails.

Conditional writes use ETags, bounded retries and readback verification. A dial-only edit changes the dial field, not its route/server. API errors omit configuration bodies; inspect Caddy's own logs for detailed validation errors. For uncertain creates, the diagnostic includes the attempted ID and state retains that identity when readback cannot confirm the result. Inspect/recover that object before retrying. No whole-configuration rollback occurs.

## Provider connection

`endpoint` is required and must be an HTTP/HTTPS origin without credentials, path, query or fragment. `ca_file` optionally adds trusted PEM CAs. `client_certificate_file` and `client_key_file` must be supplied together for mutual TLS. HTTPS verification is always enabled. `request_timeout` defaults to `30s`; `conflict_retries` defaults to 5 (range 0–100). Create/update/delete operations are bounded to two minutes.

The provider does not configure the admin listener. Installation, services, file deployment, certificate renewal and reloading files changed at the same path remain external. Unix sockets are unsupported.

## Import and state

Import is read-only and accepts only an existing Caddy `@id`:

```sh
tofu import caddy_http_server.main existing-server-id
tofu import caddy_http_route.app existing-route-id
tofu import caddy_handler_reverse_proxy.app existing-proxy-id
```

Parents must have IDs. Import does not adopt children automatically. Only the current typed shapes are supported; unsupported matcher structures fail with a diagnostic. Objects without IDs cannot be imported. Hostname log bindings and metrics expose provider state identities and have no importer.

Optional settings are observed at import and become managed when configured. The computed `configuration_revision` records field ownership without exposing settings or secrets. The first apply after importing a route with configured optional settings can show a state-only metadata update; it performs no Caddy write if live values already match. Subsequent plans are empty. `order_satisfied` exposes whether a configured sibling order needs repair.

See [testing](docs/testing.md), the [design](spec/design.md) and [implementation notes](docs/implementation-notes.md).
