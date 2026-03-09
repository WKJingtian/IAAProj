# IAA Server Project Context (Standardized Snapshot)

Last updated: 2026-03-09  
Repo root: `d:\IAAProj\IAAServer`

## 1. Monorepo Layout

- `common`
  - `config/json_loader.go`: strict JSON loader (`DisallowUnknownFields`)
  - `mongo/client.go`: shared Mongo init/client/database/disconnect helper
- `svr_login`
  - WeChat login server, returns JWT
- `svr_gateway`
  - unified entry, login routing + non-login JWT auth + forward to game
- `svr_game`
  - game business server, current business APIs for `debug_val`
- `cmd_test`
  - `curl_tests.ps1`: gateway/login/game integration test script
  - `client_patch/`: staged Unity client patch files (for manual copy)
- `build_all.bat`
  - builds `svr_gateway`, `svr_login`, `svr_game` into exe

## 2. Current Port Contract (Config-Driven)

- `svr_login/config.json`
  - `login_port`, `wx_app_id`, `wx_app_secret`, `jwt_secret`
- `svr_gateway/config.json`
  - `gateway_port`, `login_host`, `login_port`, `game_host`, `game_port`
  - `jwt_secret`, `proxy_timeout_ms`, `login_route_path`, `login_target_path`
- `svr_game/config.json`
  - `game_port`

Port defaults are not hardcoded in service code; values are required from config.

## 3. Auth/Data Flow

1. Client calls gateway `/login`
2. Gateway forwards to login service (`/wxlogin` target path)
3. Login service validates WeChat code and returns `{openid, token, errMsg}`
4. Client calls non-login APIs via gateway with `Authorization: Bearer <token>`
5. Gateway validates JWT (`jwt_secret`), extracts `openid`, injects `X-OpenID`
6. Gateway forwards request to game service
7. Game service trusts `X-OpenID` and executes business logic

## 4. Service API Status

### `svr_login`

- `POST /wxlogin`
- `GET|POST /test`
- response style: includes `errMsg`

### `svr_gateway`

- login route:
  - configured by `login_route_path` (external path)
  - forwarded to `login_target_path` (upstream path)
  - compatibility accepts `/wxlogin`
- non-login route:
  - JWT required
  - forwards to `svr_game` with `X-OpenID`

### `svr_game`

- `GET /debug_val`
  - read player `debug_val` by `openid` (`X-OpenID`)
  - no doc => return `debug_val=0`
- `POST /debug_val_inc`
  - atomic increment `debug_val` by 1 (upsert)
- startup creates unique index on `players.openid`
- unmatched route => JSON 404

## 5. Mongo Contract

### Shared helper (`common/mongo`)

- supports `InitFromJSON`, `Init`, `Client`, `Database`, `Disconnect`
- connection pool and timeout knobs
- optional `user/pwd` auth (must appear together)

### `svr_game` data

- collection: `players`
- schema fields:
  - `openid` (unique)
  - `debug_val` (int64)
  - `created_at`, `updated_at` (UTC)

## 6. Key Operational Notes

- Startup order: `svr_login` -> `svr_game` -> `svr_gateway`
- `svr_gateway/config.json` must use same `jwt_secret` as `svr_login`
- Current checked-in `svr_gateway/config.json` has empty `jwt_secret`; fill before runtime
- `.gitignore` ignores `*.exe` and `.gocache/*`

## 7. Test and Build Aids

- Integration test script:
  - `cmd_test/curl_tests.ps1`
  - supports either direct token or login-first flow
- Batch build:
  - `build_all.bat` builds all three servers with local `.gocache`

## 8. Client Patch Staging (Unity China / WeChatWASM)

Prepared under:

- `cmd_test/client_patch/Assets/Scripts/`

Contains decoupled client classes:

- `WeChatLogin.cs` (orchestrator)
- `WeChatSdkAuthProvider.cs`
- `GatewayAuthClient.cs`
- `GameApiClient.cs`
- `AuthSession.cs`
- `AuthSessionStore.cs`

Copy instructions:

- `cmd_test/client_patch/APPLY_INSTRUCTIONS.md`
