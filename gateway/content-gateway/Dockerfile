FROM golang:1.22-alpine AS build
WORKDIR /app
COPY gateway/content-gateway/go.mod gateway/content-gateway/go.sum* ./
COPY proto /proto
COPY gateway/content-gateway/cmd ./cmd
COPY gateway/content-gateway/internal ./internal
RUN CGO_ENABLED=0 GOOS=linux go build -o /bin/app ./cmd/content-gateway

FROM alpine:3.20
WORKDIR /app
COPY --from=build /bin/app /app/app
EXPOSE 8082
CMD ["/app/app"]
