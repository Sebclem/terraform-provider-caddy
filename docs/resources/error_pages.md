# caddy_error_pages

Configure exactly one of `server_id` (shared error pages for every site on that HTTP server) or `subroute_id` (error pages for one subroute). `root` (directory on the Caddy host) and `page_uri` are required; `page_uri` is typically `"/{http.error.status_code}.html"`. The resource owns one error route and its fixed vars-root, rewrite, file-server and original-status fallback sequence. Root/URI edits preserve the top route ID. On creation, internal IDs derive from the top ID with `-root`, `-rewrite`, `-files`, `-fallback` suffixes; children are not separately managed/imported.

Supports UUID/default `object_id`. Import uses only the top error route's existing ID and requires the complete recognized sequence; internal IDs may be arbitrary or absent and are preserved on update; partial/custom error handlers are rejected. The selected server or subroute must have no other error routes when this resource is created. Deletion removes the owned error route subtree. Files stay external. Caddy serves error files using the original error status; a missing file returns the original error status and empty body, matching the legacy server-level error handler. The internal file server uses pass-through for missing files followed by a fixed original-status responder, because Caddy subroute error handlers otherwise propagate a new 404.

For shared pages, declare this resource once alongside the server:

```hcl
resource "caddy_error_pages" "shared" {
  server_id = caddy_http_server.main.id
  root      = "/srv"
  page_uri  = "/{http.error.status_code}.html"
}
```

Subroute error handlers take precedence over the server fallback. Remove per-site declarations when all sites should use the shared handler. Moving an error resource to a different parent replaces that error route; changing its root or URI updates it in place. Import by the error route's existing `@id` discovers whether its parent is a server or subroute.
