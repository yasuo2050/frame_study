# Makefile

.PHONY: proto
proto:
	@echo "Generating protobuf code..."
	@protoc --go_out=. --go-grpc_out=. --grpc-gateway_out=. \
		-I proto \
		proto/greeter.proto
	@echo "Done."