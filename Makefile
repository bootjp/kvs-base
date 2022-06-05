SHELL=/bin/bash

build: clean
	go build -o kvs-infrastructure ./cmd/main.go

clean:
	rm -rf /tmp/my-raft-cluster/
	rm -f ./kvs-infrastructure
stop:
	-@killall kvs-infrastructure

run: stop build
	goreman start

test:
	go test -v ./...
	golangci-lint run

