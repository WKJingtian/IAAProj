# Config Contracts

## Global Rules

- Config files are loaded via `common/config.LoadJSONConfig`.
- Unknown JSON fields are rejected.
- Empty or malformed config should fail startup early.

## `svr_login/config.json`

Required keys:

- `login_port` (int: 1..65535)
- `wx_app_id` (string)
- `wx_app_secret` (string)
- `jwt_secret` (string)

## `svr_gateway/config.json`

Required keys:

- `gateway_port` (int: 1..65535)
- `login_host` (string)
- `login_port` (int: 1..65535)
- `game_host` (string)
- `game_port` (int: 1..65535)
- `jwt_secret` (string)

Optional keys with internal default:

- `proxy_timeout_ms` (defaults to 10000 if <= 0)
- `login_route_path` (defaults to `/login` when empty)
- `login_target_path` (defaults to `/wxlogin` when empty)

## `svr_game/config.json`

Required keys:

- `game_port` (int: 1..65535)

## `svr_game/mongo_config.json`

Required keys:

- `uri`
- `database`

Optional with defaults:

- `app_name`
- pool/timeouts (`max_pool_size`, `min_pool_size`, `max_connecting`, `max_conn_idle_ms`, `connect_timeout_ms`, `server_selection_timeout_ms`, `socket_timeout_ms`, `ping_timeout_ms`)

Auth keys:

- `user`, `pwd` must be provided together (or both omitted)

## Critical Coupling

- `svr_gateway.jwt_secret` must equal `svr_login.jwt_secret`.
