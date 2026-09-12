.PHONY: fmt vet test build install

fmt:
	gofmt -w cmd internal

vet:
	go vet ./...

test:
	go test ./...

build:
	go build -o bin/open-agent-clock ./cmd/open-agent-clock

install:
	go install ./cmd/open-agent-clock
