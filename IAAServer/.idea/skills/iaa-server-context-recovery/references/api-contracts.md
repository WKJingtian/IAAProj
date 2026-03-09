# API Contracts

## Login Service (`svr_login`)

### `POST /wxlogin`

Form fields:

- `code`
- `appid`

Success response shape:

- `openid`
- `token`
- `errMsg` (empty string)

Error response shape:

- `errMsg` non-empty

## Gateway (`svr_gateway`)

### Login routing

- External login path controlled by `login_route_path` (default `/login`)
- Forwarded to login upstream path `login_target_path` (default `/wxlogin`)
- `/wxlogin` is accepted as compatibility path

### Non-login routing

- Requires `Authorization: Bearer <jwt>`
- Verifies JWT with configured secret
- Extracts `openid` claim
- Forwards to game upstream with `X-OpenID` header

## Game Service (`svr_game`)

All endpoints expect `X-OpenID` from gateway.

### `GET /debug_val`

Behavior:

- Read player by `openid`
- Return `debug_val=0` if player doc not found

Response:

- `openid`
- `debug_val`
- `errMsg`

### `POST /debug_val_inc`

Behavior:

- Atomic increment (`$inc`) with upsert
- Updates `updated_at`, sets `created_at` on insert

Response:

- `openid`
- incremented `debug_val`
- `errMsg`
