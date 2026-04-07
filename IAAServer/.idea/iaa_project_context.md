# IAA Server Project Context

Last updated: 2026-03-19  
Repo root: `d:\IAAProj\IAAServer`

## 1. Monorepo Layout

- `common`
  - `config/json_loader.go`: strict JSON loader (`DisallowUnknownFields`)
  - `mongo/client.go`: shared Mongo init/client/database/disconnect helper
  - `service/registry.go`: shared service metadata, lease registry, register/heartbeat client helpers
- `svr_gateway`
  - public entry
  - maintains service registry
  - handles `/register` and `/heartbeat`
  - routes login/game requests dynamically
- `svr_login`
  - WeChat login server
  - actively registers itself to gateway
  - sends periodic heartbeat
- `svr_game`
  - game business server
  - actively registers itself to gateway
  - sends periodic heartbeat
  - supports self-termination after being superseded
- `svr_supervisor`
  - control plane and process manager for Linux deployment
  - starts services, monitors health, restarts active instances, manages `svr_game` cutover
- `cmd_test`
  - `curl_tests.ps1`: integration test script
  - `client_patch/`: staged Unity client patch files
- `build_all_linux.bat`
  - builds Linux binaries for `svr_gateway`, `svr_login`, `svr_game`, `svr_supervisor`
- `run_all_linux.sh`
  - Linux deployment and operations entry script

## 2. Current Port Contract

Ports are config-driven. Current checked-in defaults:

- `svr_gateway/config.json`
  - `gateway_port = 8080`
- `svr_login/config.json`
  - `login_port = 8081`
- `svr_game/config.json`
  - `game_port = 8082`
- `svr_supervisor/config.json`
  - `control_port = 19090`

Default registration host for `svr_login` and `svr_game`:

- `report_ip = 127.0.0.1`

## 3. Runtime Architecture

### 3.1 Request flow

1. Client calls gateway `/login`
2. Gateway chooses an available `login` instance from the registry
3. Gateway forwards the request to the selected login instance
4. Login service validates WeChat login and returns JWT
5. Client calls non-login APIs via gateway with `Authorization: Bearer <token>`
6. Gateway validates JWT and injects `X-OpenID`
7. Gateway chooses an available `game` instance from the registry
8. Gateway forwards the request to the selected game instance

### 3.2 Service registration flow

1. `svr_login` and `svr_game` start listening
2. each service calls gateway `POST /register`
3. gateway returns a `lease_id`
4. each service periodically calls `POST /heartbeat`
5. gateway uses recent heartbeat to determine healthy active instances

### 3.3 Cutover model

- same `type` + same logical `id` can be registered again by a new process
- gateway immediately makes the new lease active
- previous lease becomes `superseded`
- superseded service is not killed by gateway
- superseded service learns this from heartbeat response and exits after delay

This is an instant cutover model, not a canary model.

## 4. Service API Status

### `svr_gateway`

- health:
  - `GET /livez`
  - `GET /readyz`
- control for service registry:
  - `POST /register`
  - `POST /heartbeat`
- routing:
  - external login path is config-driven by `login_route_path`
  - login upstream target path is config-driven by `login_target_path`
  - non-login routes require JWT and are proxied to game

### `svr_login`

- `POST /wxlogin`
- `GET /test`
- `POST /test`
- `GET /livez`
- `GET /readyz`

### `svr_game`

- `GET /debug_val`
  - read player `debug_val` by `openid`
  - no doc => returns `debug_val = 0`
- `POST /debug_val_inc`
  - atomic increment by `openid`
- `GET /livez`
- `GET /readyz`
- unmatched route => JSON 404

### `svr_supervisor`

- `POST /bootstrap`
- `POST /start`
- `POST /stop`
- `GET /status`
- `POST /deploy/game`
- `GET /livez`

## 5. Registry And Lease Model

Shared definitions are in `common/service/registry.go`.

Important concepts:

- service type:
  - `login`
  - `game`
- logical service ID:
  - stable identity such as `login-1` or `game-1`
- lease ID:
  - runtime instance identity returned by gateway
- lease state:
  - `active`
  - `superseded`
  - `unknown`

Gateway stores instances by:

- service type
- logical service ID

Selection policy:

- pick a random healthy active instance under the requested type

This is sufficient for stateless load balancing.

## 6. Config Contracts

### `svr_gateway/config.json`

Current fields:

- `gateway_port`
- `jwt_secret`
- `proxy_timeout_ms`
- `login_route_path`
- `login_target_path`
- `register_path`
- `heartbeat_path`
- `lease_timeout_sec`
- `supersede_delay_sec`

Gateway no longer has static upstream fields like:

- `login_host`
- `login_port`
- `game_host`
- `game_port`

### `svr_login/config.json`

Current fields:

- `wx_app_id`
- `wx_app_secret`
- `login_port`
- `jwt_secret`
- `gateway_host`
- `gateway_port`
- `gateway_register_path`
- `gateway_heartbeat_path`
- `report_ip`
- `server_id`

### `svr_game/config.json`

Current fields:

- `game_port`
- `main_collection`
- `gateway_host`
- `gateway_port`
- `gateway_register_path`
- `gateway_heartbeat_path`
- `report_ip`
- `server_id`

### `svr_supervisor/config.json`

Current fields:

- `control_host`
- `control_port`
- `runtime_root`
- `health_host`
- `health_check_interval_sec`
- `health_failure_threshold`
- `restart_backoff_sec`
- `default_game_service_id`

## 7. Mongo Contract

`svr_game` reads `mongo_config.json`.

Current checked-in values:

- `uri = mongodb://localhost:27017/`
- `database = iaa_game_db`
- `user = admin`
- `pwd = chilly123-`
- `auth_source = iaa_game_db`
- `auth_mechanism = SCRAM-SHA-256`

Current game data contract:

- collection name comes from `svr_game/config.json`
- current collection is `usr_data`
- main indexed field is `openid`
- startup creates a unique index on `openid`

Important correction:

- older context documents referred to collection `players`
- current code and config use `usr_data`

## 8. Logging

Services use `common/applog`.

Linux runtime log root is managed under:

- `linux_runtime/logs/`

Important files:

- `svr_supervisor.log`
- `svr_supervisor.stdout.log`
- instance logs created by supervisor

## 9. Deployment Model

### 9.1 Build

Build on Windows:

```bat
cd d:\IAAProj\IAAServer
build_all_linux.bat
```

Outputs:

- `linux_build/svr_gateway/`
- `linux_build/svr_login/`
- `linux_build/svr_game/`
- `linux_build/svr_supervisor/`

Each output directory contains the Linux binary and required JSON configs.

### 9.2 Release directories

Linux deployment uses versioned releases:

- `linux_runtime/releases/<release_name>/`

Rule:

- do not overwrite files in an existing imported release
- each deployment should use a new release name

### 9.3 Operations script

Main Linux entry:

- `run_all_linux.sh`

Supported commands:

- `import-release <release_name> [source_dir]`
- `start <base_release> [source_dir]`
- `deploy-game <release_name> [source_dir]`
- `status`
- `stop`

### 9.4 Startup order

Supervisor-managed startup order:

1. `svr_gateway`
2. `svr_login`
3. `svr_game`

This replaced the older static-dependency startup order.

## 10. Supervisor Responsibilities

`svr_supervisor` is the only intended long-running deployment manager.

Responsibilities:

- install and run from `linux_runtime/control/`
- bootstrap stack from a base release
- monitor `/readyz`
- restart unhealthy active services
- manage active and retiring `svr_game` instances
- execute `svr_game` cutover using `/deploy/game`

Current limitation:

- `svr_gateway` and `svr_login` are not hot-updated through cutover
- updating those services currently means full stop/start with a new base release

## 11. Operational Notes

- `svr_gateway` and `svr_login` must use the same `jwt_secret`
- gateway routing is now registry-driven, not config-driven upstream routing
- `svr_game` cutover is instant, not weighted
- superseded old game instances exit after `supersede_delay_sec`
- supervisor should be treated as the single source of runtime process state

## 12. Known Risks / Validation Notes

- Linux shell orchestration has been implemented but not fully end-to-end validated inside the Windows sandbox
- final verification still needs to be performed on the target Linux host
- MongoDB must be initialized before `svr_game` can pass readiness

## 13. Canonical Docs

For deployment procedure, prefer:

- `.idea/DEPLOY.md`

For architecture snapshot, prefer:

- `.idea/iaa_project_context.md`
