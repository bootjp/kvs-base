package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net"
	"os"

	"github.com/hashicorp/raft"

	"github.com/bootjp/kvs"
	pb "github.com/bootjp/kvs/proto"

	"github.com/Jille/raftadmin"

	"github.com/Jille/raft-grpc-leader-rpc/leaderhealth"
	"google.golang.org/grpc"
	_ "google.golang.org/grpc/health"
	"google.golang.org/grpc/reflection"

	_ "github.com/Jille/grpc-multi-resolver"
)

var (
	myAddr        = flag.String("address", "localhost:50051", "TCP host+port for this node")
	raftId        = flag.String("raft_id", "", "Node id used by Raft")
	raftBootstrap = flag.Bool("raft_bootstrap", false, "Whether to bootstrap the Raft cluster")
)

func main() {
	flag.Parse()

	if *raftId == "" {
		log.Fatalf("flag --raft_id is required")
	}

	ctx := context.Background()
	_, port, err := net.SplitHostPort(*myAddr)
	if err != nil {
		log.Fatalf("failed to parse local address (%q): %v", *myAddr, err)
	}
	sock, err := net.Listen("tcp", fmt.Sprintf(":%s", port))
	if err != nil {
		log.Fatalf("failed to listen: %v", err)
	}

	dir, err := os.CreateTemp(os.TempDir(), fmt.Sprintf("raft-%s", *raftId))
	if err != nil {
		log.Fatalf("failed to create temp dir: %v", err)
	}

	store := kvs.NewKVS(dir.Name())
	r, tm, err := kvs.NewRaft(ctx, *raftId, *myAddr, store, *raftBootstrap, raft.Configuration{
		Servers: []raft.Server{
			{
				Suffrage: raft.Voter,
				ID:       "1",
				Address:  "localhost:50000",
			},
			{
				Suffrage: raft.Nonvoter,
				ID:       "2",
				Address:  "localhost:50001",
			},
			{
				Suffrage: raft.Nonvoter,
				ID:       "3",
				Address:  "localhost:50002",
			},
		},
	})
	if err != nil {
		log.Fatalf("failed to start raft: %v", err)
	}
	s := grpc.NewServer()
	pb.RegisterKVSServer(s, kvs.NewRPCInterface(store, r))
	tm.Register(s)
	leaderhealth.Setup(r, s, []string{"Example"})
	raftadmin.Register(s, r)
	reflection.Register(s)
	if err := s.Serve(sock); err != nil {
		log.Fatalf("failed to serve: %v", err)
	}
}
