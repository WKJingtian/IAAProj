---
name: iaa-svr-game-debugval
description: Maintain and extend svr_game debug value APIs in the IAA architecture. Use when changing behavior of `GET /debug_val` and `POST /debug_val_inc`, Mongo player schema/index logic keyed by openid, or gateway-to-game identity header flow.
---

# IAA svr_game Debug Value Maintenance

Read context first:

- `../../iaa_project_context.md`

## Current Baseline (already implemented)

- `GET /debug_val`
  - returns current player debug value
  - returns `0` when player doc does not exist
- `POST /debug_val_inc`
  - atomic increment with upsert
- startup ensures unique index on `players.openid`

## Workflow for Any Change

1. Validate identity boundary.
- Keep identity source as `X-OpenID` from gateway.
- Do not reintroduce JWT parsing in game service.

2. Validate persistence invariants.
- Keep player key as `openid`.
- Preserve unique index behavior for `players.openid`.
- Keep atomic update semantics for increment endpoint.

3. Keep response contract stable.
- JSON fields: `openid`, `debug_val`, `errMsg`.
- Use consistent HTTP status and error messaging style.

4. Run regression checks.
- Build modules: `svr_game`, `svr_gateway`, `svr_login`.
- Run `cmd_test/curl_tests.ps1` for end-to-end checks.

## Extension Patterns

- Add new player stat endpoint:
  - mirror current handler style (method check, header check, timeout, Mongo op, JSON response)
- Add batch/stateless read endpoint:
  - keep all player scoping anchored to `openid`
- Add write endpoint:
  - prefer single-document atomic operations where possible

## Guardrails

- Keep config loader strictness (`common/config`) assumptions in mind.
- Keep port naming contract unchanged:
  - `gateway_port`, `login_port`, `game_port`
- Keep gateway secret coupling with login secret.

## Done Checklist

- [ ] Behavior and schema changes are reflected in `../../iaa_project_context.md`
- [ ] Endpoints still work through gateway with bearer token
- [ ] No direct JWT handling was added to game service
- [ ] Build checks pass for all three services
