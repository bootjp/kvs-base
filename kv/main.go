package main

import (
	"flag"
	"github.com/bootjp/kvs-base/kv/config"
	"github.com/bootjp/kvs-base/kv/server"
	"github.com/bootjp/kvs-base/kv/storage/standalone_storage"
	"github.com/bootjp/kvs-base/log"
	"github.com/bootjp/kvs-base/proto/pkg/tinykvpb"
	"net"
	_ "net/http/pprof"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/keepalive"
)

var (
	schedulerAddr = flag.String("scheduler", "", "scheduler address")
	storeAddr     = flag.String("addr", "", "store address")
	dbPath        = flag.String("path", "", "directory path of db")
	logLevel      = flag.String("loglevel", "", "the level of log")
)

func main() {
	flag.Parse()
	conf := config.NewDefaultConfig()
	if *schedulerAddr != "" {
		conf.SchedulerAddr = *schedulerAddr
	}
	if *storeAddr != "" {
		conf.StoreAddr = *storeAddr
	}
	if *dbPath != "" {
		conf.DBPath = *dbPath
	}
	if *logLevel != "" {
		conf.LogLevel = *logLevel
	}

	log.SetLevelByString(conf.LogLevel)
	log.SetFlags(log.Ldate | log.Ltime | log.Lmicroseconds | log.Lshortfile)
	log.Infof("Server started with conf %+v", conf)

	store, err := standalone_storage.NewStandAloneStorage(conf)
	if err != nil {
		log.Fatal(err)
	}
	// var err error
	// if conf.Raft {
	//	store = raft_storage.NewRaftStorage(conf)
	//} else {
	//	if store, err = standalone_storage.NewStandAloneStorage(conf); err != nil {
	//		log.Fatal(err)
	//	}
	//}
	if err := store.Start(); err != nil {
		log.Fatal(err)
	}
	sv := server.NewServer(store)

	var alivePolicy = keepalive.EnforcementPolicy{
		MinTime:             2 * time.Second, // If a client pings more than once every 2 seconds, terminate the connection
		PermitWithoutStream: true,            // Allow pings even when there are no active streams
	}

	grpcServer := grpc.NewServer(
		grpc.KeepaliveEnforcementPolicy(alivePolicy),
		grpc.InitialWindowSize(1<<30),
		grpc.InitialConnWindowSize(1<<30),
		grpc.MaxRecvMsgSize(10*1024*1024),
	)
	tinykvpb.RegisterTinyKvServer(grpcServer, sv)
	listenAddr := conf.StoreAddr[strings.IndexByte(conf.StoreAddr, ':'):]
	l, err := net.Listen("tcp", listenAddr)
	if err != nil {
		log.Fatal(err)
	}
	handleSignal(grpcServer)

	err = grpcServer.Serve(l)
	if err != nil {
		log.Fatal(err)
	}
	log.Info("Server stopped.")
}

func handleSignal(grpcServer *grpc.Server) {
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh,
		syscall.SIGHUP,
		syscall.SIGINT,
		syscall.SIGTERM,
		syscall.SIGQUIT)
	go func() {
		sig := <-sigCh
		log.Infof("Got signal [%s] to exit.", sig)
		grpcServer.Stop()
	}()
}
