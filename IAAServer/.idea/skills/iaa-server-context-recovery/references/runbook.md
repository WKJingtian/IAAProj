# Runbook

## Build

Option A: one-click build

```bat
build_all.bat
```

Option B: module-by-module

```powershell
cd svr_login; go build ./...
cd ../svr_game; go build ./...
cd ../svr_gateway; go build ./...
```

## Startup Order

1. `svr_login`
2. `svr_game`
3. `svr_gateway`

## Smoke Test

Use:

```powershell
.\cmd_test\curl_tests.ps1 -Token "YOUR_JWT_TOKEN"
```

or login-first mode:

```powershell
.\cmd_test\curl_tests.ps1 -AppID "wx_xxx" -Code "wx_code"
```

## Fast Sanity Checklist

- Gateway and login JWT secrets match.
- Mongo config (`uri/database`) is valid for game service.
- Gateway can reach login/game hosts and ports.
- `GET /debug_val` and `POST /debug_val_inc` return valid JSON.

## Known Failure Patterns

- `401` on non-login routes:
  - missing/invalid bearer token
  - gateway and login secret mismatch
- game startup fails:
  - Mongo unreachable
  - invalid mongo auth pair (`user`/`pwd`)
  - config JSON unknown fields due strict loader
