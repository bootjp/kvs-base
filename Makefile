SHELL=/bin/bash

build: clean
	go build -o kvs-infrastructure ./cmd/main.go

clean:
	rm -rf /tmp/my-raft-cluster/
	rm -f ./kvs-infrastructure
	go clean -testcache

stop:
	-@killall kvs-infrastructure

run: stop build
	DEBUG="true" goreman start

test: clean build
	go test -v ./...
	golangci-lint run

heavy_test: clean build
	bash ./hammer.sh

