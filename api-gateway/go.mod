module api-gateway

go 1.27.3

require (
	github.com/golang-jwt/jwt/v5 v5.2.1
	github.com/grpc-ecosystem/grpc-gateway/v2 v2.34.0
	golang.org/x/time v0.16.0
	google.golang.org/grpc v1.85.0
	google.golang.org/protobuf v1.36.12
	gopkg.in/yaml.v3 v3.0.1
	stashr/proto v0.0.0
)

require (
	golang.org/x/net v0.62.0 // indirect
	golang.org/x/sys v0.49.0 // indirect
	golang.org/x/text v0.44.0 // indirect
	google.golang.org/genproto/googleapis/api v0.0.0-20261116203732-fd2c7689a808 // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20261110000943-09e21b237f2c // indirect
)

replace stashr/proto => ../proto
