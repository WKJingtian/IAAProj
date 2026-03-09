# IAAServer Horizontal Scaling Review

## Conclusion
- Current level: `scalable in theory, medium-to-high practical effort`.
- Login and game services are near-stateless, but gateway upstream design and config model limit easy scaling.

## Per-Service Status
1. `svr_login`
   - No local session state; naturally suitable for multi-instance deployment.
   - Needs shared secret/config source across replicas.
2. `svr_gateway`
   - Config points to single login/game host+port.
   - Reverse proxy is built as single-target per upstream.
3. `svr_game`
   - App logic is mostly stateless; state is in Mongo.
   - Relies on gateway-injected `X-OpenID`, so network topology must prevent bypass.
4. `Mongo`
   - Single logical datastore becomes central bottleneck as app replicas grow.

## Key Gaps
1. No service discovery and no dynamic instance health management.
2. No upstream pool/load-balancing logic in gateway.
3. Config is local-file based; scaling requires manual config edits and restarts.
4. No visible sharding/cold-hot data strategy for long-term data scale.
5. `main_collection` struct tag style is incorrect and risky with strict JSON decode.

## Improvement Plan
### P0
1. Run gateway/login/game as multi-instance behind L4/L7 load balancers.
2. Upgrade gateway from single target to upstream pool with health checks and fail-out.
3. Externalize config/secrets to a shared source (env/config center).

### P1
1. Introduce service discovery (Consul/etcd/Kubernetes Service).
2. Scale data tier (replica/sharding strategy keyed by `openid` or hash).
3. Add end-to-end tracing for cross-node tail-latency debugging.

## Code Evidence
- `IAAServer/svr_gateway/config.json:3`
- `IAAServer/svr_gateway/config.json:5`
- `IAAServer/svr_gateway/main.go:101`
- `IAAServer/svr_gateway/main.go:125`
- `IAAServer/svr_gateway/main.go:126`
- `IAAServer/svr_game/main.go:80`
- `IAAServer/svr_game/main.go:205`
- `IAAServer/common/config/json_loader.go:31`
- `IAAServer/svr_game/main.go:28`
