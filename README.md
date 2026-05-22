Movie Streaming Platform

In general:
3 microservices: user-service(Aidana), content-service(Yerkebulan), stream-service(Temirlan)

Project includes:
- At least 12 gRPC methods in each service
- PostgreSQL and Redis
- SMTP email 
- Unit and integration tests

Workflow:
When user opens frontend, frontend sends HTTP request to gateway. For example when user wants movies, frontend sends request to content gateway. Content gateway sends gRPC request to content service. Content service reads movies from PostgreSQL and returns response back to gateway and frontend.

NATS Events
- user.registered: user -> content, stream
- user.deleted: user -> content, stream
- stream.completed: stream -> user
- movie.rated: content -> user
- stream.started: stream -> content

Tests:
Unit tests check business logic directly, for example registration, login, movie creation, rating, starting and stopping stream.

Integration tests check gRPC flow. They start a test gRPC server and call service methods like a real client.


Redis cache stores:
user tokens
movie data
top movies list
active stream sessions

Grafana:
Grafana for monitoring. Prometheus is used for metrics, Loki is used for logs, and Tempo is used for traces. Grafana connects to all of them and shows data in one place. This helps us see service health, logs, and request tracing.

How to run:
docker-compose up -d