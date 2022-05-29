package kvs

import (
	"context"
	"fmt"
	"log"
	"reflect"
	"strconv"
	"testing"
	"time"

	grpcretry "github.com/grpc-ecosystem/go-grpc-middleware/retry"

	pb "github.com/bootjp/kvs/proto"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	_ "github.com/Jille/grpc-multi-resolver"
	_ "google.golang.org/grpc/health"
)

func TestDelete(t *testing.T) {
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
		t.Fatalf("Delete test failed: %v", resp.Data)
	}

	t.Log(want, "check ok")

}

func TestConsistency(t *testing.T) {
	c := client()

	key := []byte("test-key")

	fmt.Println("inlop")
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

func client() pb.KVSClient {
	retryOpts := []grpcretry.CallOption{
		grpcretry.WithBackoff(grpcretry.BackoffExponential(100 * time.Millisecond)),
		grpcretry.WithMax(1),
	}
	conn, err := grpc.Dial("multi:///localhost:50051,localhost:50052,localhost:50053",
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithDefaultCallOptions(grpc.WaitForReady(true)),
		grpc.WithUnaryInterceptor(grpcretry.UnaryClientInterceptor(retryOpts...)))
	if err != nil {
		log.Fatalf("dialing failed: %v", err)
	}
	return pb.NewKVSClient(conn)
}
