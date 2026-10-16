# caddy_handler_reverse_proxy

Manages a reverse-proxy handler within a route. Static upstream addresses can be IP addresses or hostnames; DNS-based dynamic backend discovery is outside this release.

| Setting | Type | Behavior |
| --- | --- | --- |
| `route_id` | Required string | Parent route ID; changing it replaces the handler. |
| `upstream` | One or more typed blocks | Each has required nonempty `dial` string with a Caddy network address. |
| `before_id` / `after_id` | Optional string | One relative sibling-handler constraint, mutually exclusive. |
| `object_id` | Optional string | Explicit Caddy ID or generated UUID v4; identity changes replace. |
| `id` | Computed string | Effective Caddy ID. |
| `order_satisfied` | Computed bool | Whether live handler order satisfies the constraint. |
| `configuration_revision` | Computed string | Last-applied field ownership fingerprint. |

A single existing upstream dial change uses a guarded dial-field update. Structural upstream-list changes replace that list; other handler fields and nested content with unambiguous correspondence are preserved. HTTP transport is Caddy's default; typed TLS transport and health options are later increments.

Import uses the existing handler's Caddy ID. It must be a reverse-proxy handler with representable static upstreams and an ID-bearing parent route. Deletion removes this handler's complete subtree.

```hcl
resource "caddy_handler_reverse_proxy" "app" {
  route_id = caddy_http_route.app.id
  upstream {
    dial = "backend.example.test:9000"
  }
}
```
