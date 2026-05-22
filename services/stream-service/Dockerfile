FROM golang:1.22-alpine AS build
WORKDIR /app
COPY services/stream-service/go.mod services/stream-service/go.sum* ./
COPY proto /proto
COPY services/stream-service/cmd ./cmd
COPY services/stream-service/internal ./internal
RUN CGO_ENABLED=0 GOOS=linux go build -o /bin/app ./cmd/stream-service

FROM alpine:3.20
WORKDIR /app
COPY --from=build /bin/app /app/app
EXPOSE 50053
CMD ["/app/app"]
