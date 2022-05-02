package main

import (
	"context"
	"fmt"
	"log"
	"strconv"
	"time"

	"google.golang.org/grpc/credentials/insecure"

	_ "github.com/Jille/grpc-multi-resolver"
	pb "github.com/bootjp/kvs-infrastructure/proto"
	grpcretry "github.com/grpc-ecosystem/go-grpc-middleware/retry"
	"google.golang.org/grpc"
	_ "google.golang.org/grpc/health"
)

var keys = []string{"a", "b", "c"}

func main() {
	go func() {
		printloop()
	}()
	retryOpts := []grpcretry.CallOption{
		grpcretry.WithBackoff(grpcretry.BackoffExponential(100 * time.Millisecond)),
		grpcretry.WithMax(5),
	}
	conn, err := grpc.Dial("multi:///localhost:50051,localhost:50052,localhost:50053,localhost:50054,localhost:50055,localhost:50056,localhost:50057,localhost:50058,localhost:50059,localhost:50060",
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithDefaultCallOptions(grpc.WaitForReady(true)),
		grpc.WithUnaryInterceptor(grpcretry.UnaryClientInterceptor(retryOpts...)))
	if err != nil {
		log.Fatalf("dialing failed: %v", err)
	}
	defer conn.Close()
	c := pb.NewKVSClient(conn)

	for i := 0; 100 > i; i++ {
		for _, key := range keys {
			_, err := c.AddData(
				context.Background(),
				&pb.AddDataRequest{Key: []byte(key), Data: []byte(strconv.Itoa(i))},
			)
			if err != nil {
				log.Fatalf("AddWord RPC failed: %v", err)
			}
		}
	}
	for _, key := range keys {
		resp, err := c.GetData(context.Background(), &pb.GetDataRequest{Key: []byte(key)})
		if err != nil {
			log.Fatalf("GetWords RPC failed: %v", err)
		}
		fmt.Println(resp)
	}
	fmt.Println("---")

	time.Sleep(100 * time.Millisecond)
}

func printloop() {
	retryOpts := []grpcretry.CallOption{
		grpcretry.WithBackoff(grpcretry.BackoffExponential(10 * time.Millisecond)),
		grpcretry.WithMax(5),
	}
	conn, err := grpc.Dial("multi:///localhost:50051,localhost:50052,localhost:50053,localhost:50054,localhost:50055,localhost:50056,localhost:50057,localhost:50058,localhost:50059,localhost:50060",
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithDefaultCallOptions(grpc.WaitForReady(true)),
		grpc.WithUnaryInterceptor(grpcretry.UnaryClientInterceptor(retryOpts...)))
	if err != nil {
		log.Fatalf("dialing failed: %v", err)
	}
	defer conn.Close()
	c := pb.NewKVSClient(conn)

	for {
		for _, key := range keys {
			resp, err := c.GetData(context.Background(), &pb.GetDataRequest{Key: []byte(key)})
			if err != nil {
				log.Fatalf("GetWords RPC failed: %v", err)
			}
			fmt.Println(resp)
		}
		fmt.Println("---")

		time.Sleep(100 * time.Millisecond)
	}
}
