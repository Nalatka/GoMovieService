# Movie Streaming Platform

## Scope
- 3 microservices: `user-service`, `content-service`, `stream-service`
- 3 API gateways: one per service/member
- Transport: HTTP -> Gateway -> gRPC
- Service-to-service events: NATS

## Requirements
- Clean Architecture
- At least 12 gRPC methods per service
- PostgreSQL + Redis, migrations, transactions
- SMTP email (Gmail or Outlook)
- Unit and integration tests

## Ports
- `user-service`: `50051`
- `content-service`: `50052`
- `stream-service`: `50053`
- `nats`: `4222`
- `user-db`: `5433`, `content-db`: `5434`, `stream-db`: `5435`
- `user-redis`: `6380`, `content-redis`: `6381`, `stream-redis`: `6382`

## NATS Events
- `user.registered`: user -> content, stream
- `user.deleted`: user -> content, stream
- `stream.completed`: stream -> user
- `movie.rated`: content -> user
- `stream.started`: stream -> content

## Current Repo State
- `docker-compose.yml`
- `proto/user.proto`
- `proto/content.proto`
- `proto/stream.proto`
- `proto/NATS_EVENTS.md`
- `services/*/migrations/*.sql`
- `services/*/cmd/*/main.go`
- `services/*/internal/*`
- `gateway/*/cmd/*/main.go`
- `docs/project-management.md`

## Run
```bash
docker-compose up -d
```

## Generate gRPC
```bash
protoc --go_out=. --go-grpc_out=. proto/*.proto
```
