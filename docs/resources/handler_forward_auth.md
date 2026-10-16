# caddy_handler_forward_auth

Required `route_id`, static `upstream` dial and `uri`; optional `copy_headers` list. Supports UUID/default `object_id`, sibling ordering and ID-only import of a recognized native forward-auth sequence.

Serializes a reverse proxy precheck using GET, forwarding the original method/URI in `X-Forwarded-Method` and `X-Forwarded-Uri`. A successful 2xx response continues with the original method and body. Selected auth response headers replace request headers; absent selected headers remove incoming values. Non-2xx responses return directly to the client. Order this handler before the application proxy. Caddy's legacy adaptation authenticates before local-IP denial; the home example preserves this order.
