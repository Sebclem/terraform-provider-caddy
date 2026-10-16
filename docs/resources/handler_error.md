# caddy_handler_error

Required `route_id`, integer `status_code` (400–599) and string `message`. Raises a Caddy HTTP error, which a surrounding `caddy_error_pages` resource can handle. Supports UUID/default `object_id`, sibling ordering and import by existing error handler ID.
