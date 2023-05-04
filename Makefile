.PHONY: all build test vet fmt verify clean

BIN := bin/fathom

all: build test

build:
	mkdir -p bin
	go build -o $(BIN) ./cmd/fathom

test:
	go test ./...

vet:
	go vet ./...

fmt:
	gofmt -w .

verify:
	bash scripts/verify.sh

clean:
	rm -rf bin coverage.out
