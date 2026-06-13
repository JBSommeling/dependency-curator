.PHONY: build test lint docker clean

BINARY=dependency-curator
GOFLAGS=-trimpath

build:
	go build $(GOFLAGS) -o $(BINARY) ./cmd/action/

test:
	go test ./... -v -race -count=1

lint:
	golangci-lint run ./...

docker:
	docker build -t dependency-curator .

clean:
	rm -f $(BINARY)
	go clean -testcache
