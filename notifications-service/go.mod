module notifications-service

go 1.27.3

require (
	github.com/gocql/gocql v1.7.0
	github.com/golang-jwt/jwt/v5 v5.2.2
	google.golang.org/grpc v1.85.0
	google.golang.org/protobuf v1.36.12
	stashr/proto v0.0.0-00010101000000-000000000000
	stashr/shared v0.0.0
)

require (
	github.com/golang/snappy v0.0.3 // indirect
	github.com/grpc-ecosystem/grpc-gateway/v2 v2.34.0 // indirect
	github.com/hailocab/go-hostpool v0.0.0-20160125115350-e80d13ce29ed // indirect
	golang.org/x/net v0.62.0 // indirect
	golang.org/x/sys v0.49.0 // indirect
	golang.org/x/text v0.44.0 // indirect
	google.golang.org/genproto/googleapis/api v0.0.0-20261116203732-fd2c7689a808 // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20261110000943-09e21b237f2c // indirect
	gopkg.in/inf.v0 v0.9.1 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
)

replace stashr/proto => ../proto

replace stashr/shared => ../shared
