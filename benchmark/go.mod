module github.com/acoBOYZ/grpc-pipe-go/benchmark

go 1.24.6

require (
	github.com/acoBOYZ/grpc-pipe-go v0.0.0
	google.golang.org/grpc v1.74.2
	google.golang.org/protobuf v1.36.7
)

require (
	golang.org/x/net v0.40.0 // indirect
	golang.org/x/sys v0.33.0 // indirect
	golang.org/x/text v0.25.0 // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20250528174236-200df99c418a // indirect
)

// use the local root module (../) for imports like github.com/acoBOYZ/grpc-pipe-go/pipe
replace github.com/acoBOYZ/grpc-pipe-go => ../
