# IAAServer Bandwidth and User State Cache Review

## Conclusion
- Current level: `cache layer is basically absent`.
- User-state requests (example: `debug_val`) hit Mongo on every call, causing avoidable network and DB load.

## Current State
- `GET /debug_val` reads directly from Mongo (`FindOne`).
- `POST /debug_val_inc` writes directly to Mongo (`FindOneAndUpdate`).
- No in-memory cache, no Redis, no distributed session/state cache.
- Response includes `openid` each time (small but repeated payload).
- No clear response compression and no bandwidth observability.

## Key Gaps
1. Hot users repeatedly trigger cross-network DB traffic.
2. No cache hit-rate metrics, so optimization impact cannot be measured.
3. No async merge-write path, causing write amplification.
4. Gateway does not apply payload optimization strategy.

## Improvement Plan
### P0
1. Add Redis state cache.
   - Read path: cache-aside (Redis -> Mongo fallback -> backfill).
   - Write path for counters: `INCR` in Redis + async flush to Mongo.
2. Define cache keys and TTL policy.
   - Example key: `player:{openid}:debug_val`.
   - Add TTL jitter to reduce stampede risk.
3. Trim response payload where possible.
   - Return `openid` only when needed.

### P1
1. Batch and merge flush jobs to Mongo.
2. Enable gzip/br compression for JSON responses at gateway.
3. Track:
   - Cache hit rate
   - Redis/Mongo QPS
   - Ingress/egress bandwidth
   - Average response size

## Suggested Targets
- Cache hit rate on hot APIs: `> 90%`
- Mongo read QPS reduction: `>= 50%`
- Mongo write request reduction via merge-write: `30% - 70%`
- Avg response size reduction (trim + compression): `10% - 30%`

## Code Evidence
- `IAAServer/svr_game/main.go:143`
- `IAAServer/svr_game/main.go:205`
- `IAAServer/svr_game/main.go:219`
- `IAAServer/svr_gateway/main.go:151`
- `IAAServer/svr_game/main.go:65`
