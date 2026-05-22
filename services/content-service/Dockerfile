FROM golang:1.22-alpine AS build
WORKDIR /app
COPY services/content-service/go.mod services/content-service/go.sum* ./
COPY proto /proto
COPY services/content-service/cmd ./cmd
COPY services/content-service/internal ./internal
COPY services/content-service/migrations ./migrations
RUN CGO_ENABLED=0 GOOS=linux go build -o /bin/app ./cmd/content-service

FROM alpine:3.20
WORKDIR /app
COPY --from=build /bin/app /app/app
EXPOSE 50052
CMD ["/app/app"]
