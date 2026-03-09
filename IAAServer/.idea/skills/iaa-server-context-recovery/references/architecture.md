# Architecture Map

## Service Graph

Client -> `svr_gateway` -> (`svr_login` for login) OR (`svr_game` for business)

## Responsibilities

- `svr_login`
  - WeChat code exchange
  - JWT issuance (`openid` claim)
- `svr_gateway`
  - route `/login` to login upstream
  - verify JWT on non-login
  - inject `X-OpenID`
  - reverse proxy to game upstream
- `svr_game`
  - business logic only
  - trust gateway identity header
  - persist player data to MongoDB
- `common`
  - shared config loader and Mongo helper

## Identity Boundary

- JWT logic boundary is gateway/login side.
- `svr_game` does not parse JWT directly.
- `svr_game` trusts `X-OpenID` from gateway.

## Persistence Boundary

- Mongo is owned by game service logic.
- `common/mongo` offers shared connection lifecycle + pool controls.
