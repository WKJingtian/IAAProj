# IAA Server Deployment Guide

Last updated: 2026-03-19
Repo root: `d:\IAAProj\IAAServer`

## 1. Purpose

This document defines the deployment contract for the Linux server stack so MCP or other automation can execute the flow deterministically.

Current deployment model:

- build on Windows
- upload `linux_build/` and `run_all_linux.sh` to Linux
- use `svr_supervisor` as the long-running control plane
- use release directories for versioned deployment
- `svr_game` updates use cutover, not canary

## 2. Services

- `svr_gateway`
  - public entry
  - current default port: `8080`
- `svr_login`
  - login service
  - current default port: `8081`
- `svr_game`
  - game service
  - current default port: `8082`
- `svr_supervisor`
  - health monitor and process manager
  - current control port: `19090`

## 3. Runtime Architecture

### 3.1 Registration model

- `svr_login` and `svr_game` actively register to `svr_gateway`
- gateway exposes:
  - `POST /register`
  - `POST /heartbeat`
- registration fields:
  - `type`
  - `id`
  - `host`
  - `port`
- gateway maintains:
  - type index
  - logical service ID
  - active lease
  - superseded lease

### 3.2 Cutover model

- `svr_game` update is instant cutover
- new `svr_game` starts from a new release directory
- new instance registers with the same logical `server_id`
- gateway immediately routes new traffic to the new lease
- old `svr_game` receives `superseded` from heartbeat
- old `svr_game` exits after the configured delay
- old instance is not killed immediately

### 3.3 Supervisor model

`svr_supervisor` is the only long-running control process.

Responsibilities:

- start `svr_gateway`
- start `svr_login`
- start the active `svr_game`
- monitor `/readyz`
- restart unhealthy active processes
- manage release state
- perform `svr_game` cutover

Non-goals for now:

- hot update for `svr_gateway`
- hot update for `svr_login`

`svr_gateway` and `svr_login` currently update through full stop/start of the stack.

## 4. Required Files

### 4.1 Produced by build

`build_all_linux.bat` produces:

- `linux_build/svr_gateway/svr_gateway`
- `linux_build/svr_gateway/config.json`
- `linux_build/svr_login/svr_login`
- `linux_build/svr_login/config.json`
- `linux_build/svr_game/svr_game`
- `linux_build/svr_game/config.json`
- `linux_build/svr_game/mongo_config.json`
- `linux_build/svr_supervisor/svr_supervisor`
- `linux_build/svr_supervisor/config.json`

### 4.2 Required on Linux host

Upload these items together:

- `linux_build/`
- `run_all_linux.sh`

## 5. Directory Contract On Linux

Assume deployment root is a directory containing `run_all_linux.sh`.

Important paths:

- `./linux_build/`
  - uploaded build artifacts
- `./linux_runtime/releases/<release>/`
  - imported immutable release snapshot
- `./linux_runtime/control/`
  - current supervisor binary and config
- `./linux_runtime/logs/`
  - service and supervisor logs
- `./linux_runtime/pids/`
  - supervisor pid
- `./linux_runtime/state/`
  - supervisor state files

Rule:

- do not modify files inside an imported release directory in place
- every update should create a new release directory

## 6. Build Contract

Run on Windows from repo root:

```bat
cd d:\IAAProj\IAAServer
build_all_linux.bat
```

Optional ARM build:

```bat
build_all_linux.bat arm64
```

Build assumptions:

- Go is available in PATH
- cross-compile target is Linux
- `CGO_ENABLED=0`
- output directory is recreated on each build

## 7. Linux Prerequisites

- Linux with `bash`
- `curl`
- executable permission support
- MongoDB already installed and initialized

Recommended checks:

```bash
bash --version
curl --version
mongosh --version
```

## 8. MongoDB Contract

Current project config expects:

- uri: `mongodb://localhost:27017/`
- database: `iaa_game_db`
- user: `admin`
- password: `chilly123-`
- auth source: `iaa_game_db`
- auth mechanism: `SCRAM-SHA-256`

Initialization example:

```javascript
use iaa_game_db

db.createUser({
  user: "admin",
  pwd: "chilly123-",
  roles: [
    { role: "readWrite", db: "iaa_game_db" },
    { role: "dbAdmin", db: "iaa_game_db" }
  ],
  mechanisms: ["SCRAM-SHA-256"]
})
```

After service startup:

- `svr_game` automatically uses collection `usr_data`
- `svr_game` automatically creates the unique `openid` index

## 9. Current Port Contract

Default checked-in config values:

- gateway: `8080`
- login: `8081`
- game: `8082`
- supervisor control: `19090`

Default local registration host:

- `127.0.0.1`

If the production machine needs external registration IP, adjust:

- `svr_login/config.json`
- `svr_game/config.json`

before building or before importing a release.

## 10. Commands

`run_all_linux.sh` supports:

- `import-release <release_name> [source_dir]`
- `start <base_release> [source_dir]`
- `deploy-game <release_name> [source_dir]`
- `status`
- `stop`

Default `source_dir` is `./linux_build`.

## 11. Standard Operations

### 11.1 First deployment

On Windows:

```bat
cd d:\IAAProj\IAAServer
build_all_linux.bat
```

Upload:

- `linux_build/`
- `run_all_linux.sh`

On Linux:

```bash
chmod +x run_all_linux.sh
./run_all_linux.sh start r20260319_1
```

Expected effect:

- release `r20260319_1` is imported if missing
- supervisor is installed into `linux_runtime/control/`
- supervisor starts
- supervisor bootstraps the stack
- startup order is:
  - gateway
  - login
  - game

### 11.2 View status

```bash
./run_all_linux.sh status
```

Expected result:

- current base release
- active services
- active game instance
- retiring game instance if any

### 11.3 Deploy a new `svr_game`

This is the standard online update flow.

1. Build new binaries on Windows.
2. Upload the new `linux_build/` to Linux.
3. Execute:

```bash
./run_all_linux.sh deploy-game r20260320_1
```

Expected effect:

- release `r20260320_1` is imported if missing
- supervisor launches a new `svr_game` from that release
- new `svr_game` becomes active
- gateway routes new game traffic to the new lease immediately
- old `svr_game` becomes retiring
- old `svr_game` self-terminates after the supersede delay

Operational note:

- this is cutover, not traffic-splitting

### 11.4 Update `svr_gateway` or `svr_login`

These services do not currently support hot cutover.

Recommended flow:

1. Build and upload new `linux_build/`
2. Stop the stack
3. Start the stack with the new base release

Commands:

```bash
./run_all_linux.sh stop
./run_all_linux.sh start r20260320_1
```

### 11.5 Full shutdown

```bash
./run_all_linux.sh stop
```

Expected effect:

- supervisor stops managed services
- supervisor process exits

## 12. Health and Logging

### 12.1 Health endpoints

- gateway: `/livez`, `/readyz`
- login: `/livez`, `/readyz`
- game: `/livez`, `/readyz`
- supervisor control: `/livez`, `/status`

Supervisor health policy is driven by:

- `health_check_interval_sec`
- `health_failure_threshold`
- `restart_backoff_sec`

### 12.2 Logs

Primary log root:

- `linux_runtime/logs/`

Important files:

- `linux_runtime/logs/svr_supervisor.log`
- `linux_runtime/logs/svr_supervisor.stdout.log`
- service logs created by supervisor-managed instances

Operational rule:

- inspect supervisor logs first when startup or cutover fails

## 13. Automation Notes For MCP

Recommended automation sequence for first deployment:

1. Build on Windows with `build_all_linux.bat`
2. Upload `linux_build/` and `run_all_linux.sh`
3. Run `chmod +x run_all_linux.sh`
4. Run `./run_all_linux.sh start <release>`
5. Poll `./run_all_linux.sh status`

Recommended automation sequence for `svr_game` update:

1. Build new release
2. Upload new `linux_build/`
3. Run `./run_all_linux.sh deploy-game <release>`
4. Poll `./run_all_linux.sh status`
5. Check `linux_runtime/logs/`

Recommended automation sequence for full stack update:

1. Build new release
2. Upload new `linux_build/`
3. Run `./run_all_linux.sh stop`
4. Run `./run_all_linux.sh start <release>`
5. Poll `./run_all_linux.sh status`

Automation safety rules:

- never overwrite binaries inside an already imported release directory
- always create a new release name for a new build
- use `status` after every state-changing command
- if `deploy-game` fails, do not delete the previous release automatically
- read logs before retrying repeated restarts

## 14. Troubleshooting

### 14.1 Supervisor does not start

Check:

- `linux_runtime/logs/svr_supervisor.stdout.log`
- `linux_runtime/logs/svr_supervisor.log`
- executable permission on `linux_runtime/control/svr_supervisor`

### 14.2 Game cannot become ready

Check:

- MongoDB reachability
- MongoDB user and `auth_source`
- `linux_build/svr_game/mongo_config.json`
- game logs under `linux_runtime/logs/`

### 14.3 Gateway has no upstream target

Check:

- `svr_login` and `svr_game` started successfully
- registration host and port are correct
- gateway logs
- heartbeat is not timing out

### 14.4 `deploy-game` succeeds but old process does not exit

Check:

- gateway heartbeat response path
- gateway `supersede_delay_sec`
- old game logs
- whether the old process is still sending heartbeat

### 14.5 `deploy-game` returns curl 500 and gateway logs show `Authorization header is required`

This usually means the shell script did not reach `svr_supervisor` at all and instead hit `svr_gateway`.

Check:

- `linux_runtime/control/config.json`
- `linux_build/svr_supervisor/config.json`
- whether `control_port` is still `19090`
- whether `control_host` points to the supervisor machine

Quick verification:

```bash
curl -v http://127.0.0.1:19090/status
curl -v http://127.0.0.1:8080/status
```

Expected result:

- supervisor control port should return a JSON object containing `state` and `processes`
- gateway port should not be used as supervisor control endpoint

### 14.6 `/usr/bin/env: 'bash\r': No such file or directory`

This means `run_all_linux.sh` was uploaded with Windows CRLF line endings.

Check:

- whether the first line of `run_all_linux.sh` still contains `\r`
- whether the Linux copy of the script is the latest uploaded version

Fix:

```bash
sed -i 's/\r$//' run_all_linux.sh
chmod +x run_all_linux.sh
```

Operational rule:

- always upload the LF version of `run_all_linux.sh`

### 14.7 `permission denied` when starting `svr_gateway` / `svr_login` / `svr_game`

Typical error:

```text
start svr_gateway failed: fork/exec .../releases/<release>/svr_gateway/svr_gateway: permission denied
```

This means the imported release binary does not have the executable bit.

Check:

- `ls -l linux_runtime/releases/<release>/svr_gateway/svr_gateway`
- `ls -l linux_runtime/releases/<release>/svr_login/svr_login`
- `ls -l linux_runtime/releases/<release>/svr_game/svr_game`
- whether the deployment filesystem is mounted with `noexec`

Fix:

```bash
chmod +x linux_runtime/releases/<release>/svr_gateway/svr_gateway
chmod +x linux_runtime/releases/<release>/svr_login/svr_login
chmod +x linux_runtime/releases/<release>/svr_game/svr_game
```

Important note:

- new versions of `run_all_linux.sh` already apply `chmod +x` during `import-release`
- new versions of `svr_supervisor` also force `0755` before `exec`
- already imported old releases keep their old permissions until fixed manually or re-imported under a new release name

### 14.8 `permission denied` when starting `svr_supervisor`

This is different from service binary permission failure. The path will point to:

- `linux_build/svr_supervisor/svr_supervisor`, or
- `linux_runtime/control/svr_supervisor`

Check:

- `ls -l linux_build/svr_supervisor/svr_supervisor`
- `ls -l linux_runtime/control/svr_supervisor`
- `namei -l linux_runtime/control/svr_supervisor`
- whether the parent filesystem is mounted with `noexec`

Fix:

```bash
chmod +x linux_build/svr_supervisor/svr_supervisor
chmod +x linux_runtime/control/svr_supervisor
```

### 14.9 `deploy-game` returns 500 but only a generic curl error is visible

This was previously hard to debug because the shell script did not print the response body.

Current behavior:

- new versions of `run_all_linux.sh` print the HTTP status and response body from `svr_supervisor`
- new versions of `svr_supervisor` also log the underlying `deploy-game` error

If the machine still shows only a generic curl failure, check whether you have uploaded:

- the latest `run_all_linux.sh`
- the latest `linux_build/svr_supervisor/svr_supervisor`

### 14.10 `deploy-game` fails because `svr_game` instance directory does not contain CSV data

Symptom:

- `svr_game` starts from `linux_runtime/instances/svr_game/<instance>/`
- runtime `config.json` exists
- required `Data_*.csv` files are missing in that instance directory
- `svr_game` then fails while loading static data because `data_dir` is still `.`

Root cause:

- old versions of `svr_supervisor` only wrote runtime `config.json`
- they did not copy static CSV files from the release directory into the runtime instance directory

Current behavior:

- new versions of `svr_supervisor` copy `svr_game/*.csv` from the release into the runtime instance directory before starting the process

If the machine still has the old behavior, upload the latest:

- `linux_build/svr_supervisor/svr_supervisor`

### 14.11 `stop` was executed, but port `19090` still appears to be in use

Things to distinguish:

- if `curl http://127.0.0.1:19090/status` returns the expected supervisor JSON, then the old supervisor is still running
- if `svr_supervisor.log` contains `control listen on http://127.0.0.1:19090` followed by bind failure, there may be another supervisor or another program already occupying the port

Check:

```bash
curl -s http://127.0.0.1:19090/status
ss -ltnp | grep 19090
ps -ef | grep svr_supervisor
```

Operational note:

- newer `svr_supervisor` builds return `control_pid` in `/status`
- newer `run_all_linux.sh` validates that the control endpoint PID matches the recorded `SUPERVISOR_PID`
- if Linux is still running an older supervisor binary, you may still need one manual cleanup before the improved checks take effect

## 15. Practical Preflight Checklist

Before assuming a new deployment bug, check these first:

1. `run_all_linux.sh` on Linux is the latest uploaded version and uses LF line endings.
2. `linux_build/svr_supervisor/svr_supervisor` on Linux is the latest uploaded binary.
3. `curl http://127.0.0.1:19090/status` reaches the expected supervisor control endpoint.
4. release binaries under `linux_runtime/releases/<release>/` all have execute permission.
5. `linux_runtime/control/svr_supervisor` itself has execute permission.
6. `svr_game` runtime instance directories contain the required `Data_*.csv`.
7. if `deploy-game` returns 500, read the response body and `svr_supervisor.log` before retrying.

## 16. Current Limitations

- no hot update flow for `svr_gateway`
- no hot update flow for `svr_login`
- Linux shell orchestration was not end-to-end verified in the Windows sandbox
- production validation must still be done on the target Linux machine

## 17. Suggested Future Extensions

- add a dedicated rollback command
- add retention cleanup for old releases
- add structured JSON logs
- add systemd integration for supervisor
- add alerting on repeated restart failures
