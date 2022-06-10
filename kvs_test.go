package kvs

import (
	"context"
	"fmt"
	"log"
	"net"
	"os"
	"reflect"
	"strconv"
	"testing"
	"time"

	"github.com/hashicorp/raft"

	"google.golang.org/protobuf/types/known/durationpb"

	grpcretry "github.com/grpc-ecosystem/go-grpc-middleware/retry"

	pb "github.com/bootjp/kvs/proto"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/reflection"

	_ "github.com/Jille/grpc-multi-resolver"
	"github.com/Jille/raft-grpc-leader-rpc/leaderhealth"
	"github.com/Jille/raftadmin"
	_ "google.golang.org/grpc/health"
)

var hostformat = "localhost:5000%d"

var kvs map[string]*KVS
var node []*grpc.Server

func TestMain(m *testing.M) {
	kvs = make(map[string]*KVS)
	_ = createNode(3)
	fmt.Println("finish create node")
	code := m.Run()
	shutdown()
	os.Exit(code)
}

func shutdown() {
	for _, server := range node {
		server.Stop()
	}
}

func createNode(n int) []*grpc.Server {
	cfg := raft.Configuration{}
	for i := 0; i < n; i++ {
		fmt.Println("create node", fmt.Sprintf(hostformat, i))
		var suffrage raft.ServerSuffrage
		if i == 0 {
			suffrage = raft.Voter
		} else {
			suffrage = raft.Nonvoter
		}
		addr := fmt.Sprintf(hostformat, i)
		server := raft.Server{
			Suffrage: suffrage,
			ID:       raft.ServerID(strconv.Itoa(i)),
			Address:  raft.ServerAddress(addr),
		}
		cfg.Servers = append(cfg.Servers, server)
	}

	for i := 0; i < n; i++ {
		ctx := context.Background()
		addr := fmt.Sprintf(hostformat, i)
		_, port, err := net.SplitHostPort(addr)
		if err != nil {
			log.Fatalf("failed to parse local address (%q): %v", fmt.Sprintf(hostformat, i), err)
		}
		sock, err := net.Listen("tcp", fmt.Sprintf(":%s", port))
		if err != nil {
			log.Fatalf("failed to listen: %v", err)
		}

		store := NewKVS()

		kvs[strconv.Itoa(i)] = store
		r, tm, err := NewRaft(ctx, strconv.Itoa(i), addr, store, i == 0, cfg)
		if err != nil {
			log.Fatalf("failed to start raft: %v", err)
		}
		s := grpc.NewServer()
		pb.RegisterKVSServer(s, NewRPCInterface(store, r))
		tm.Register(s)
		leaderhealth.Setup(r, s, []string{"Example"})
		raftadmin.Register(s, r)
		reflection.Register(s)

		node = append(node, s)
		go func() {
			if err := s.Serve(sock); err != nil {
				log.Fatalf("failed to serve: %v", err)
			}
		}()
	}

	time.Sleep(10 * time.Second)

	return node
}

func Test_value_can_be_deleted(t *testing.T) {
	c := client()
	key := []byte("test-key")
	want := []byte("v")
	_, err := c.AddData(
		context.Background(),
		&pb.AddDataRequest{Key: key, Data: want},
	)
	if err != nil {
		log.Fatalf("Add RPC failed: %v", err)
	}
	_, err = c.AddData(context.TODO(), &pb.AddDataRequest{Key: key, Data: want})
	if err != nil {
		t.Fatalf("Add RPC failed: %v", err)
	}
	resp, err := c.GetData(context.TODO(), &pb.GetDataRequest{Key: key})
	if err != nil {
		t.Fatalf("Get RPC failed: %v", err)
	}

	if !reflect.DeepEqual(want, resp.Data) {
		t.Fatalf("consistency check failed want %v got %v", want, resp.Data)
	}

	_, err = c.DeleteData(context.TODO(), &pb.DeleteRequest{Key: key})
	if err != nil {
		t.Fatalf("Delete RPC failed: %v", err)
	}

	resp, err = c.GetData(context.TODO(), &pb.GetDataRequest{Key: key})
	if err != nil {
		t.Fatalf("Get RPC failed: %v", err)
	}
	if resp.Error != pb.GetDataError_DATA_NOT_FOUND {
		t.Fatalf("Delete test failed: %v %v", resp.Data, resp.Error)
	}

}

func Test_consistency_satisfy_write_after_read(t *testing.T) {
	c := client()

	key := []byte("test-key")

	for i := 0; i < 99999; i++ {
		want := []byte(strconv.Itoa(i))
		_, err := c.AddData(
			context.Background(),
			&pb.AddDataRequest{Key: key, Data: want},
		)
		if err != nil {
			log.Fatalf("AddWord RPC failed: %v", err)
		}
		_, err = c.AddData(context.TODO(), &pb.AddDataRequest{Key: key, Data: want})
		if err != nil {
			t.Fatalf("AddWord RPC failed: %v", err)
		}
		resp, err := c.GetData(context.TODO(), &pb.GetDataRequest{Key: key})
		if err != nil {
			t.Fatalf("GetWords RPC failed: %v", err)
		}

		if !reflect.DeepEqual(want, resp.Data) {
			t.Fatalf("consistency check failed want %v got %v", want, resp.Data)
		}
		t.Log(want, "check ok")
	}
}

func Test_does_not_retrieve_data_beyond_TTL(t *testing.T) {
	c := client()
	key := []byte("test-key")
	want := []byte("test-data")
	_, err := c.AddData(
		context.Background(),
		&pb.AddDataRequest{Key: key, Data: want, Ttl: durationpb.New(10 * time.Second)},
	)
	if err != nil {
		log.Fatalf("AddWord RPC failed: %v", err)
	}
	time.Sleep(9 * time.Second)
	resp, err := c.GetData(context.TODO(), &pb.GetDataRequest{Key: key})
	if err != nil {
		t.Fatalf("GetWords RPC failed: %v", err)
	}

	if !reflect.DeepEqual(want, resp.Data) {
		t.Fatalf("consistency check failed want %v got %v", want, resp.Data)
	}

	time.Sleep(1 * time.Second)
	resp, err = c.GetData(context.TODO(), &pb.GetDataRequest{Key: key})
	if err != nil {
		t.Fatalf("GetWords RPC failed: %v", err)
	}
	if resp.Error != pb.GetDataError_DATA_NOT_FOUND {
		t.Fatalf("Delete test failed: %v", resp.Data)
	}
}

func Test_no_data_in_map_after_gc(t *testing.T) {
	c := client()
	key := []byte("test-key")
	want := []byte("test-data")
	_, err := c.AddData(
		context.Background(),
		&pb.AddDataRequest{Key: key, Data: want, Ttl: durationpb.New(10 * time.Second)},
	)
	if err != nil {
		log.Fatalf("AddWord RPC failed: %v", err)
	}

	var tmp [KeyLimit]byte
	copy(tmp[:], key)

	time.Sleep(1 * time.Second)
	for nodeIndex, kv := range kvs {
		v, ok := kv.data[tmp]
		if !ok {
			t.Fatalf("node %s failed got data %v", nodeIndex, v)
		}
	}

	time.Sleep(30 * time.Second)

	for nodeIndex, kv := range kvs {
		v, ok := kv.data[tmp]
		if ok {
			t.Fatalf("gc failed %s got %v now time : %v", nodeIndex, v, time.Now().UTC())
		}
	}
}

func client() pb.KVSClient {
	retryOpts := []grpcretry.CallOption{
		grpcretry.WithBackoff(grpcretry.BackoffExponential(100 * time.Millisecond)),
		grpcretry.WithMax(1),
	}
	conn, err := grpc.Dial("multi:///localhost:50000,localhost:50001,localhost:50002",
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithDefaultCallOptions(grpc.WaitForReady(true)),
		grpc.WithUnaryInterceptor(grpcretry.UnaryClientInterceptor(retryOpts...)))
	if err != nil {
		log.Fatalf("dialing failed: %v", err)
	}
	return pb.NewKVSClient(conn)
}
