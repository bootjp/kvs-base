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

proto: FORCE
	mkdir -p $(CURDIR)/bin
	(cd third_party/proto && ./generate_go.sh)
	GO111MODULE=on go build ./third_party/proto/pkg/...

FORCE: ;

submodule:
	git submodule add git@github.com:talent-plan/tinykv.git third_party
	git commit -m "add module"
	cd third_party/
	git -C third_party config core.sparsecheckout true
	echo proto/include/  > .git/modules/third_party/info/sparse-checkout
	echo proto/generate_go.sh >> .git/modules/third_party/info/sparse-checkout
	echo proto/tools/ >> .git/modules/third_party/info/sparse-checkout
	echo proto/proto/  >> .git/modules/third_party/info/sparse-checkout
	git -C third_party read-tree -mu HEAD

