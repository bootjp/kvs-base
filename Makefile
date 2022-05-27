SHELL=/bin/bash

build: clean
	GOOS=darwin GOARCH=arm64 go build -o kvs-infrastructure

clean:
	rm -fr raftexample-*
	rm -f ./kvs-infrastructure
stop:
	-@killall kvs-infrastructure

run: stop build
	./kvs-infrastructure --id 1 --cluster http://127.0.0.1:12379,http://127.0.0.1:22379,http://127.0.0.1:32379 --port 12380 &
	./kvs-infrastructure --id 2 --cluster http://127.0.0.1:12379,http://127.0.0.1:22379,http://127.0.0.1:32379 --port 22380 &
	./kvs-infrastructure --id 3 --cluster http://127.0.0.1:12379,http://127.0.0.1:22379,http://127.0.0.1:32379 --port 32380 &
