# Project Management

## Repo Model
Monorepo:
- `services/user-service`
- `services/content-service`
- `services/stream-service`
- `gateway/user-gateway`
- `gateway/content-gateway`
- `gateway/stream-gateway`
- `proto`

## Labels
- `service:user`
- `service:content`
- `service:stream`
- `type:feature`
- `type:bug`
- `type:test`
- `type:infra`
- `type:nats`

## Milestone 1: Foundation
1. `docker-compose`: postgres + redis + nats
2. Shared `.proto` files
3. User service skeleton + migrations
4. Content service skeleton + migrations
5. Stream service skeleton + migrations

## Milestone 2: Core
1. User: `RegisterUser`, `LoginUser`
2. User: JWT + Redis cache
3. Content: `CreateMovie`, `GetMovie`, `ListMovies`
4. Stream: `StartStream`, `StopStream`
5. API gateway skeleton

## Milestone 3: Integration
1. `user.deleted` -> stream closes sessions
2. `stream.completed` -> user history
3. User registration email
4. Unit and integration tests

## Demo Scope
- `docker-compose up -d` runs
- All `proto/*.proto` exist
- DB migrations exist
- 2-3 methods are implemented end-to-end
- 1 NATS event works between services
- Gateways have basic routes

## Board
`Backlog -> In Progress -> Review -> Done`

## Branches
- `user/feature-auth`
- `content/feature-movies`
- `stream/feature-sessions`

## Commit Convention
- `feat(user): add RegisterUser gRPC handler`
- `feat(content): add movie migrations`
- `fix(stream): fix session status update`
- `chore(infra): add docker-compose nats config`
- `test(user): add auth usecase tests`
