SHELL=/bin/bash

build: clean
	GOOS=darwin GOARCH=arm64 go build -o kvs-infrastructure ./cli/main.go

clean:
	rm -fr ./data
	mkdir ./data
	rm -f ./kvs-infrastructure
stop:
	-@killall kvs-infrastructure

run: stop build
	bash ./run.sh

