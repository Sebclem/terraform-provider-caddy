# caddy_handler_basic_auth

Required `route_id` and nonempty `accounts = [{username = "...", password_hash = "..."}]`. Accounts and child credential fields are sensitive. Password hashes use bcrypt; the provider never hashes plaintext credentials. State contains the supplied hashes, so protect the state backend. Supports UUID/default `object_id`, sibling ordering and import by an existing authentication handler ID. Order before the application proxy.
