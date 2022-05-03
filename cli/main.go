package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
	"time"

	"github.com/bootjp/kvs-infrastructure"

	"github.com/lni/dragonboat/v3"
	"github.com/lni/dragonboat/v3/config"
	"github.com/lni/dragonboat/v3/logger"
	"github.com/lni/goutils/syncutil"
)

func main() {
	nodeID := flag.Int("nodeid", 1, "NodeID to use")
	addr := flag.String("addr", "", "Nodehost address")
	enableConsole := flag.Bool("console", false, "is non interactive")
	join := flag.Bool("join", false, "Joining a new node")
	flag.Parse()

	// https://github.com/golang/go/issues/17393
	if runtime.GOOS == "darwin" {
		signal.Ignore(syscall.Signal(0xd))
	}
	initialMembers := make(map[uint64]string)
	if !*join {
		for idx, v := range kvs.Addresses {
			initialMembers[uint64(idx+1)] = v
		}
	}
	var nodeAddr string
	if len(*addr) != 0 {
		nodeAddr = *addr
	} else {
		nodeAddr = initialMembers[uint64(*nodeID)]
	}
	fmt.Fprintf(os.Stdout, "node address: %s\n", nodeAddr)
	logger.GetLogger("raft").SetLevel(logger.ERROR)
	logger.GetLogger("rsm").SetLevel(logger.WARNING)
	logger.GetLogger("transport").SetLevel(logger.WARNING)
	logger.GetLogger("grpc").SetLevel(logger.WARNING)
	rc := config.Config{
		NodeID:             uint64(*nodeID),
		ClusterID:          1,
		ElectionRTT:        150,
		HeartbeatRTT:       100,
		CheckQuorum:        true,
		SnapshotEntries:    10,
		CompactionOverhead: 5,
	}
	datadir := filepath.Join("data", fmt.Sprintf("node%d", *nodeID))
	fmt.Println(datadir)
	nhc := config.NodeHostConfig{
		WALDir:         datadir,
		NodeHostDir:    datadir,
		RTTMillisecond: 200,
		RaftAddress:    nodeAddr,
	}
	nh, err := dragonboat.NewNodeHost(nhc)
	if err != nil {
		panic(err)
	}
	if err := nh.StartOnDiskCluster(initialMembers, *join, kvs.NewDiskKV, rc); err != nil {
		fmt.Fprintf(os.Stderr, "failed to add cluster, %v\n", err)
		os.Exit(1)
	}
	raftStopper := syncutil.NewStopper()

	ch := make(chan string, 16)
	if *enableConsole {
		kvs.Console(raftStopper, ch, nh)
	}
	raftStopper.RunWorker(func() {
		cs := nh.GetNoOPSession(1)
		for {
			select {
			case v, ok := <-ch:
				if !ok {
					return
				}
				msg := strings.Replace(v, "\n", "", 1)
				// input message must be in the following formats -
				// put key value
				// get key
				rt, key, val, ok := kvs.ParseCommand(msg)
				if !ok {
					fmt.Fprintf(os.Stderr, "invalid input\n")
					continue
				}
				ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
				switch rt {
				case kvs.PUT:
					kv := &kvs.KVData{
						Key: key,
						Val: val,
					}
					data, err := json.Marshal(kv)
					if err != nil {
						panic(err)
					}
					_, err = nh.SyncPropose(ctx, cs, data)
					if err != nil {
						fmt.Fprintf(os.Stderr, "SyncPropose returned error %v\n", err)
					}
				case kvs.GET:
					result, err := nh.SyncRead(ctx, 1, []byte(key))
					if err != nil {
						fmt.Fprintf(os.Stderr, "SyncRead returned error %v\n", err)
					} else {
						fmt.Fprintf(os.Stdout, "query key: %s, result: %s\n", key, result)
					}
				}
				cancel()
			case <-raftStopper.ShouldStop():
				return
			}
		}
	})
	raftStopper.Wait()
}
