.DEFAULT_GOAL := help

GO ?= go
PROTOC ?= protoc
CONFIG ?= services/benefit-service/configs/local.yaml

.PHONY: help run build fmt fmt-check vet test check proto

help:
	@printf '%s\n' \
		'make run        - Run benefit-service (HTTP + gRPC)' \
		'make build      - Check that all Go packages build' \
		'make fmt        - Format Go files' \
		'make fmt-check  - Check Go formatting without changing files' \
		'make vet        - Run go vet' \
		'make test       - Run Go tests' \
		'make check      - Run formatting, vet, tests and build checks' \
		'make proto      - Generate Go code from benefit.proto and rules.proto'

run:
	$(GO) run ./services/benefit-service/cmd/server -config "$(CONFIG)"

build:
	$(GO) build ./...

fmt:
	gofmt -w .

fmt-check:
	@files=$$(gofmt -l .) || exit $$?; \
	if [ -n "$$files" ]; then \
		printf '%s\n' "$$files"; \
		exit 1; \
	fi

vet:
	$(GO) vet ./...

test:
	$(GO) test ./...

check: fmt-check vet test build

proto:
	mkdir -p gen/go
	$(PROTOC) -I api/proto \
		--go_out=gen/go --go_opt=paths=source_relative \
		--go-grpc_out=gen/go --go-grpc_opt=paths=source_relative \
		benefit/v1/benefit.proto \
		rules/v1/rules.proto

migrate-up:
	@. ./.env && \
		migrate -path services/rules-service/migrations \
			-database "$$RULES_DATABASE_URL" up

migrate-down:
	@. ./.env && \
		migrate -path services/rules-service/migrations \
			-database "$$RULES_DATABASE_URL" down 1
.PHONY: migrate-up migrate-down