module gomovieservice/gateway/content-gateway

go 1.22

require (
	github.com/Nalatka/GoMovieService/proto v0.0.0
	github.com/golang-jwt/jwt/v5 v5.3.0
	github.com/google/uuid v1.6.0
	google.golang.org/grpc v1.64.1
)

require (
	golang.org/x/net v0.33.0 // indirect
	golang.org/x/sys v0.28.0 // indirect
	golang.org/x/text v0.21.0 // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20240528184218-531527333157 // indirect
	google.golang.org/protobuf v1.36.6 // indirect
)

replace github.com/Nalatka/GoMovieService/proto => ../../proto
