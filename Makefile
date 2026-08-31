build:
	go build -ldflags "-s -w -X main.version=$(shell git describe --tags --always --dirty 2>/dev/null || echo dev)" -o bin/steward .

install:
	go install -ldflags "-s -w" .

test:
	go test ./...

vet:
	go vet ./...

fmt:
	gofmt -l .

clean:
	rm -rf bin dist

.PHONY: build install test vet fmt clean
