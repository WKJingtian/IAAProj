# IAAServer High Concurrency Review

## Conclusion
- Current level: `baseline OK, upper bound limited`.
- The first bottlenecks under burst traffic are logging I/O, upstream timeout control, and DB access frequency.

## Current State
- All services use Go `net/http` request-per-goroutine model.
- Mongo client is configured with pool sizes, timeouts, and read/write retries.
- Counter update path uses atomic Mongo update (`FindOneAndUpdate + $inc`).
- DB calls in `svr_game` use short context timeouts (3s), limiting request pile-up.

## Key Bottlenecks
1. Services use `ListenAndServe` default server config (no `ReadTimeout/WriteTimeout/IdleTimeout`).
2. Login service calls WeChat using `http.Get` default client (no global timeout).
3. Logger does `file.Sync()` on every write, which can be a major throughput limiter.
4. No explicit overload controls (rate limit, bulkhead, circuit breaker).
5. No metrics baseline for QPS, p95/p99, or saturation.

## Improvement Plan
### P0
1. Use explicit `http.Server` with timeout policy in all services.
2. Replace `http.Get` with a dedicated `http.Client` + timeout + context deadline.
3. Make logging async (buffered queue + batch flush), keep crash-safe flush on shutdown.

### P1
1. Add overload protection at gateway and critical dependencies.
2. Add metrics and load test baseline:
   - QPS, error rate, p95/p99 latency, DB pool usage.
3. Reduce hot-path DB calls using state cache (see review 04).

## Code Evidence
- `IAAServer/svr_gateway/main.go:275`
- `IAAServer/svr_game/main.go:277`
- `IAAServer/svr_login/main.go:190`
- `IAAServer/svr_login/main.go:132`
- `IAAServer/common/applog/logger.go:27`
- `IAAServer/common/applog/logger.go:32`
- `IAAServer/common/mongo/client.go:71`
- `IAAServer/common/mongo/client.go:76`
- `IAAServer/common/mongo/client.go:79`
- `IAAServer/svr_game/main.go:139`
- `IAAServer/svr_game/main.go:201`
- `IAAServer/svr_game/main.go:205`
