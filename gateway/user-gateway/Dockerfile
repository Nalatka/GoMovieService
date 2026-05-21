FROM golang:1.22-alpine AS build
WORKDIR /app
COPY gateway/user-gateway/go.mod gateway/user-gateway/go.sum* ./
COPY proto/go.mod proto/go.sum proto/user.pb.go proto/user_grpc.pb.go /proto/
COPY gateway/user-gateway/cmd ./cmd
RUN CGO_ENABLED=0 GOOS=linux go build -o /bin/app ./cmd/user-gateway

FROM alpine:3.20
WORKDIR /app
COPY --from=build /bin/app /app/app
EXPOSE 8081
CMD ["/app/app"]
