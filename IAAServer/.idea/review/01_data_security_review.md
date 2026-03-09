# IAAServer Data Security Review

## Conclusion
- Current level: `medium-to-weak`.
- Main risks are around secret management, trust boundaries, and error leakage.

## Current State
- Gateway validates JWT before forwarding to game service.
- Mongo client supports auth and connectivity checks.
- Config loader uses strict key validation (`DisallowUnknownFields`).

## Key Risks
1. Plaintext secrets are committed in config files (`wx_app_secret`, `jwt_secret`, Mongo user/pwd).
2. CORS is fully open (`Access-Control-Allow-Origin: *`) in all services.
3. `svr_game` trusts `X-OpenID` header directly, without proving request origin.
4. Internal errors are returned to clients via raw `err.Error()`.
5. Gateway-to-upstream traffic uses plain HTTP (no transport encryption or service identity check).
6. JWT validation is minimal (no `iss/aud/jti` policy, no key rotation plan).
7. No explicit rate limiting / abuse control path.

## Improvement Plan
### P0
1. Move secrets out of repo.
   - Keep only template configs in git.
   - Inject real values via env vars or a secret manager.
   - Rotate exposed secrets immediately.
2. Tighten service trust boundary.
   - Keep `svr_game` private-only (no direct public ingress).
   - Add service-to-service auth (mTLS or signed internal header).
3. Sanitize external error responses.
   - Return stable error codes/messages to clients.
   - Keep detailed failures only in logs.
4. Replace wildcard CORS with explicit allowlist.

### P1
1. Harden JWT policy.
   - Add `iss/aud/sub/jti`.
   - Shorten access token TTL and add key rotation (`kid`).
2. Add gateway rate limiting and failure controls.
3. Encrypt internal transport (HTTPS/mTLS).

## Code Evidence
- `IAAServer/svr_login/config.json:4`
- `IAAServer/svr_login/config.json:5`
- `IAAServer/svr_gateway/config.json:7`
- `IAAServer/svr_game/mongo_config.json:4`
- `IAAServer/svr_game/mongo_config.json:5`
- `IAAServer/svr_login/main.go:79`
- `IAAServer/svr_gateway/main.go:145`
- `IAAServer/svr_game/main.go:59`
- `IAAServer/svr_game/main.go:80`
- `IAAServer/svr_gateway/main.go:103`
- `IAAServer/svr_gateway/main.go:107`
- `IAAServer/svr_gateway/main.go:233`
- `IAAServer/svr_game/main.go:135`
- `IAAServer/svr_gateway/main.go:141`
- `IAAServer/common/config/json_loader.go:31`
