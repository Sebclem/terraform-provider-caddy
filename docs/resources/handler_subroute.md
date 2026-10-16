# caddy_handler_subroute

An independent `subroute` handler. Required `route_id` attaches it to a route; child `caddy_http_route` resources reference its `id` through `subroute_id`. Supports `object_id`, `before_id`/`after_id`, UUID defaults and ID-only import. Updates preserve child routes and error pages; deletion removes the entire subtree.
