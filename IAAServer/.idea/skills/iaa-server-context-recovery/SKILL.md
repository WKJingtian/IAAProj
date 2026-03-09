---
name: iaa-server-context-recovery
description: Recover working context quickly for the IAA server monorepo after session loss or workspace reopen. Use when you need architecture map, config contracts, API behavior, run/test commands, and known invariants for `common`, `svr_login`, `svr_gateway`, and `svr_game`.
---

# IAA Server Context Recovery

Read these reference files in order for fast recovery:

1. `references/architecture.md`
2. `references/config-contracts.md`
3. `references/api-contracts.md`
4. `references/runbook.md`

Also keep this canonical snapshot in sync:

- `../../iaa_project_context.md`

## Recovery Workflow

1. Confirm repo/module layout.
- Verify folders: `common`, `svr_login`, `svr_gateway`, `svr_game`.
- Verify test/tooling: `cmd_test`, `build_all.bat`.

2. Validate critical runtime coupling.
- `svr_gateway.jwt_secret` must match `svr_login.jwt_secret`.
- Gateway non-login routes must inject `X-OpenID`.
- Game APIs must rely on `X-OpenID`, not direct JWT parsing.

3. Validate current business endpoint status.
- `svr_game` must expose `GET /debug_val` and `POST /debug_val_inc`.
- Players collection unique index on `openid` should be ensured on startup.

4. Run build and smoke checks.
- Build modules individually (`go build ./...`) or use `build_all.bat`.
- Use `cmd_test/curl_tests.ps1` for login + debug API flow.

5. If context changed, update references immediately.
- Update `../../iaa_project_context.md`.
- Update corresponding reference file in this skill.

## Guardrails

- Keep port names config-driven:
  - `gateway_port`, `login_port`, `game_port`
- Keep `common/config` strict loader behavior unchanged unless intentional.
- Keep `common/mongo` initialization contract stable for all services.
