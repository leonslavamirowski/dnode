include Makefile.ledger

PROTO_IN_DIR=./vm-proto/protos/
PROTO_OUT_VM_DIR=./x/vm/internal/types/vm_grpc/
PROTO_OUT_DS_DIR=./x/vm/internal/types/ds_grpc/
PROTOBUF_VM_FILES=./vm-proto/protos/vm.proto
PROTOBUF_DS_FILES=./vm-proto/protos/data-source.proto

all: protos install
install: protos go.sum
		GO111MODULE=on go install -tags "$(build_tags)" ./cmd/wbd
		GO111MODULE=on go install -tags "$(build_tags)" ./cmd/wbcli
go.sum: go.mod
		@echo "--> Ensure dependencies have not been modified"
		GO111MODULE=on go mod verify
protos:
	mkdir -p ${PROTO_OUT_VM_DIR}
	mkdir -p ${PROTO_OUT_DS_DIR}
	protoc -I ${PROTO_IN_DIR} --go_out=plugins=grpc:$(PROTO_OUT_VM_DIR) $(PROTOBUF_VM_FILES)
	protoc -I ${PROTO_IN_DIR} --go_out=plugins=grpc:$(PROTO_OUT_DS_DIR) $(PROTOBUF_DS_FILES)

## deps: Install missing dependencies. Runs `go get` internally. e.
deps:
	@echo "  >  Checking if there is any missing dependencies..."
	go get -u github.com/golang/protobuf/protoc-gen-go
