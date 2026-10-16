# caddy_http_server

Manages a Caddy HTTP server. New servers in this increment explicitly disable automatic HTTPS. Imported servers retain their existing TLS/automatic-HTTPS configuration.

| Setting | Type | Behavior |
| --- | --- | --- |
| `name` | Required string | Server map key; changing it replaces the server. |
| `listen` | Required nonempty list of strings | Listening addresses; updated in place. |
| `object_id` | Optional string | Explicit Caddy ID, otherwise generated UUID v4. Changing an explicit ID replaces the server. |
| `id` | Computed string | Effective Caddy ID. |
| `configuration_revision` | Computed string | Fingerprint of last-applied field ownership. |

Listener updates preserve routes and other settings. Deletion removes the server and its whole subtree. Shared HTTP application containers and sibling servers remain present.

Import uses the existing server's Caddy ID. The server map name and listening addresses are discovered without writing. Required HCL arguments must match those values for an empty plan.

```hcl
resource "caddy_http_server" "main" {
  name   = "main"
  listen = ["127.0.0.1:8080"]
}
```
