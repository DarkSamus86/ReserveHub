module darksamus.com/ReserveHub/workspace-service

go 1.27.1

require (
	darksamus.com/ReserveHub/provider-service v0.0.0
	github.com/google/uuid v1.6.0
	google.golang.org/grpc v1.85.0
)

require (
	golang.org/x/net v0.62.0 // indirect
	golang.org/x/sys v0.49.0 // indirect
	golang.org/x/text v0.43.0 // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20261104224025-d1e00e9f33ab // indirect
	google.golang.org/protobuf v1.36.12 // indirect
)

replace darksamus.com/ReserveHub/provider-service => ../provider-service
