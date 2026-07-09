module github.com/DarkInno/gotenancy/examples/grpc

go 1.23.0

replace github.com/DarkInno/gotenancy => ../..

require (
	github.com/DarkInno/gotenancy v0.0.0
	github.com/DarkInno/gotenancy/rpc/grpc v0.0.0
	google.golang.org/grpc v1.75.1
)

require (
	golang.org/x/net v0.41.0 // indirect
	golang.org/x/sys v0.33.0 // indirect
	golang.org/x/text v0.26.0 // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20250707201910-8d1bb00bc6a7 // indirect
	google.golang.org/protobuf v1.36.6 // indirect
)

replace github.com/DarkInno/gotenancy/rpc/grpc => ../../rpc/grpc
