# caddy_http_route

Manages one route within an HTTP server. Routes are live immediately; no activation resource is required.

| Setting | Type | Behavior |
| --- | --- | --- |
| `server_id` | Required string | Parent server ID; changing it replaces the route. |
| `match` | Optional typed list | Each object has required nonempty `hosts` strings. Objects are ORed. Empty means unrestricted. |
| `terminal` | Optional bool | Stop further routing after this route; false when absent on a new route. |
| `group` | Optional string | Mutually exclusive route group. |
| `before_id` / `after_id` | Optional string | One sibling ordering constraint, mutually exclusive. |
| `object_id` | Optional string | Explicit ID or generated UUID v4; identity changes replace. |
| `id` | Computed string | Effective Caddy ID. |
| `order_satisfied` | Computed bool | Whether the current relative order satisfies the constraint. |
| `configuration_revision` | Computed string | Last-applied field ownership fingerprint. |

Optional fields never configured are observed and preserved. Removing a previously configured field removes it from Caddy. Route edits preserve independently managed handlers. Deletion removes the route and all handlers within it.

Import uses the route's Caddy ID and discovers its parent server ID. The parent must have an ID. Only host-only matcher sets are representable in this release. First apply after import can record optional-field ownership without a Caddy write, followed by an empty plan.

```hcl
resource "caddy_http_route" "app" {
  server_id = caddy_http_server.main.id
  terminal  = true
  match     = [{ hosts = ["app.example.test", "alias.example.test"] }]
}
```

For an overlapping fallback route, set `before_id = caddy_http_route.fallback.id` on the more specific route. For the reverse relationship use `after_id`. An existing satisfied constraint does not force adjacency, and a move retains the route ID and handlers.
