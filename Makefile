SHELL=/bin/bash

proto:
	protoc ./proto/service.proto --go_out=plugins=grpc:. --go_opt=paths=source_relative

build: clean proto
	GOOS=darwin GOARCH=arm64 go build

clean:
	rm -fr /tmp/my-raft-cluster/*
	mkdir -p /tmp/my-raft-cluster/node{A,B,C,D,E,F,G,H,I,J,K,L,M,N,O,P,K,Q,S,T,U}

stop:
	-@killall kvs-infrastructure

run: stop build
	./kvs-infrastructure --raft_bootstrap --raft_id=nodeA --address=localhost:50051 --raft_data_dir /tmp/my-raft-cluster &
	./kvs-infrastructure --raft_id=nodeB --address=localhost:50052 --raft_data_dir /tmp/my-raft-cluster &
	./kvs-infrastructure --raft_id=nodeC --address=localhost:50053 --raft_data_dir /tmp/my-raft-cluster &
	./kvs-infrastructure --raft_id=nodeD --address=localhost:50054 --raft_data_dir /tmp/my-raft-cluster &
	./kvs-infrastructure --raft_id=nodeE --address=localhost:50055 --raft_data_dir /tmp/my-raft-cluster &
	./kvs-infrastructure --raft_id=nodeF --address=localhost:50056 --raft_data_dir /tmp/my-raft-cluster &
	./kvs-infrastructure --raft_id=nodeG --address=localhost:50057 --raft_data_dir /tmp/my-raft-cluster &
	./kvs-infrastructure --raft_id=nodeH --address=localhost:50058 --raft_data_dir /tmp/my-raft-cluster &
	./kvs-infrastructure --raft_id=nodeI --address=localhost:50059 --raft_data_dir /tmp/my-raft-cluster &
	./kvs-infrastructure --raft_id=nodeJ --address=localhost:50060 --raft_data_dir /tmp/my-raft-cluster &
#	./kvs-infrastructure --raft_id=nodeK --address=localhost:50061 --raft_data_dir /tmp/my-raft-cluster &
#	./kvs-infrastructure --raft_id=nodeL --address=localhost:50062 --raft_data_dir /tmp/my-raft-cluster &
#	./kvs-infrastructure --raft_id=nodeM --address=localhost:50063 --raft_data_dir /tmp/my-raft-cluster &
#	./kvs-infrastructure --raft_id=nodeN --address=localhost:50064 --raft_data_dir /tmp/my-raft-cluster &
#	./kvs-infrastructure --raft_id=nodeO --address=localhost:50065 --raft_data_dir /tmp/my-raft-cluster &
#	./kvs-infrastructure --raft_id=nodeP --address=localhost:50066 --raft_data_dir /tmp/my-raft-cluster &
#	./kvs-infrastructure --raft_id=nodeQ --address=localhost:50067 --raft_data_dir /tmp/my-raft-cluster &
#	./kvs-infrastructure --raft_id=nodeR --address=localhost:50068 --raft_data_dir /tmp/my-raft-cluster &
#	./kvs-infrastructure --raft_id=nodeS --address=localhost:50069 --raft_data_dir /tmp/my-raft-cluster &
#	./kvs-infrastructure --raft_id=nodeT --address=localhost:50070 --raft_data_dir /tmp/my-raft-cluster &
	sleep 10
	raftadmin localhost:50051 add_voter nodeB localhost:50052 0
	raftadmin localhost:50051 add_voter nodeC localhost:50053 0
	raftadmin localhost:50051 add_voter nodeD localhost:50054 0
	raftadmin localhost:50051 add_voter nodeE localhost:50055 0
	raftadmin localhost:50051 add_voter nodeF localhost:50056 0
	raftadmin localhost:50051 add_voter nodeG localhost:50057 0
	raftadmin localhost:50051 add_voter nodeH localhost:50058 0
	raftadmin localhost:50051 add_voter nodeI localhost:50059 0
	raftadmin localhost:50051 add_voter nodeJ localhost:50060 0
	#raftadmin localhost:50051 add_voter nodeC localhost:50053 0
	#raftadmin localhost:50051 add_voter nodeB localhost:50052 0
	#raftadmin localhost:50051 add_voter nodeC localhost:50053 0
	#raftadmin localhost:50051 add_voter nodeB localhost:50052 0
	#raftadmin localhost:50051 add_voter nodeC localhost:50053 0
	#raftadmin localhost:50051 add_voter nodeB localhost:50052 0
	#raftadmin localhost:50051 add_voter nodeC localhost:50053 0
#
	raftadmin --leader multi:///localhost:50051,localhost:50052,localhost:50053,localhost:50054,localhost:50055,localhost:50056,localhost:50057,localhost:50058,localhost:50059,localhost:50060 get_configuration





