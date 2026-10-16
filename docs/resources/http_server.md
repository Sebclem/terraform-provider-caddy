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

`trusted_proxies` optionally takes static IP/CIDR strings; Caddy trusts forwarded client addresses only from these peers. `https = true` adds a default TLS connection policy and enables HTTP-to-HTTPS redirects with `automatic_https.disable_certificates = true`. Reference certificate resources through `depends_on` to load files before serving. No issuer/ACME configuration is created. Default HTTP/HTTPS ports are Caddy's externally configured app ports (80/443); isolated tests use external bootstrap ports. Existing TLS policies survive ordinary listener/setting updates. New servers default to HTTP; importing a server observes optional settings without claiming them until configured.
