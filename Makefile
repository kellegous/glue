PROTOC_GEN_GO_VERSION := v1.36.5
PROTOC_GEN_CONNECT_GO_VERSION := v1.19.1
PROTOC_VERSION := 34.1
GOLANGCI_LINT_VERSION := v2.13.1
GOIMPORTS_VERSION := v0.49.0

GO_MOD := $(shell go list -m)

GENERATED := \
	logging/yarder/yarder.pb.go \
	logging/yarder/yarder_connect/yarder.connect.go

.PHONY: generated test lint fmt clean validate

generated: $(GENERATED)

clean:
	rm -rf bin $(GENERATED)

test:
	go test ./...

lint: bin/golangci-lint
	bin/golangci-lint run

fmt: bin/goimports
	find . -type f -name '*.go' $(foreach file,$(GENERATED),-not -path './$(file)') \
		-exec bin/goimports -local $(GO_MOD) -w {} +

validate: generated test lint fmt

logging/yarder/yarder.pb.go: logging/yarder/yarder.proto bin/protoc-gen-go bin/protoc
	bin/protoc --proto_path=. \
		--plugin=protoc-gen-go=bin/protoc-gen-go \
		--go_out=. \
		--go_opt=module=$(GO_MOD) \
		$<

logging/yarder/yarder_connect/yarder.connect.go: logging/yarder/yarder.proto bin/protoc-gen-connect-go bin/protoc
	bin/protoc --proto_path=. \
		--plugin=protoc-gen-connect-go=bin/protoc-gen-connect-go \
		--connect-go_out=. \
		--connect-go_opt=module=$(GO_MOD) \
		--connect-go_opt=package_suffix=_connect \
		$<

bin/protoc:
	etc/download-protoc $(PROTOC_VERSION)

bin/protoc-gen-go:
	GOBIN="$(CURDIR)/bin" go install google.golang.org/protobuf/cmd/protoc-gen-go@$(PROTOC_GEN_GO_VERSION)

bin/protoc-gen-connect-go:
	GOBIN="$(CURDIR)/bin" go install connectrpc.com/connect/cmd/protoc-gen-connect-go@$(PROTOC_GEN_CONNECT_GO_VERSION)

bin/golangci-lint: Makefile
	GOBIN=$(abspath $(dir $@)) go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@$(GOLANGCI_LINT_VERSION)

bin/goimports:
	GOBIN="$(CURDIR)/bin" go install golang.org/x/tools/cmd/goimports@$(GOIMPORTS_VERSION)
