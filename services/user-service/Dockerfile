FROM golang:1.22-alpine AS build
WORKDIR /app
COPY services/user-service/go.mod services/user-service/go.sum* ./
COPY proto/go.mod proto/go.sum proto/user.pb.go proto/user_grpc.pb.go /proto/
COPY services/user-service/cmd ./cmd
COPY services/user-service/internal ./internal
COPY services/user-service/migrations ./migrations
RUN CGO_ENABLED=0 GOOS=linux go build -o /bin/app ./cmd/user-service

FROM alpine:3.20
WORKDIR /app
COPY --from=build /bin/app /app/app
EXPOSE 50051
CMD ["/app/app"]
