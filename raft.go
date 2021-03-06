package kvs

import (
	"context"
	"fmt"

	transport "github.com/Jille/raft-grpc-transport"
	"github.com/hashicorp/raft"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

func init() {
	// todo suppress linter for develop
	_ = maxSnapshot
}

const maxSnapshot = 3

func NewRaft(_ context.Context, myID, myAddress string, fsm raft.FSM, bootstrap bool, cfg raft.Configuration) (
	*raft.Raft, *transport.Manager, error) {
	c := raft.DefaultConfig()
	c.LocalID = raft.ServerID(myID)

	// this config is for development
	ldb := raft.NewInmemStore()
	sdb := raft.NewInmemStore()
	fss := raft.NewInmemSnapshotStore()
	// baseDir := filepath.Join(*raftDir, myID)
	// fss, err := raft.NewFileSnapshotStore(baseDir, maxSnapshot, os.Stderr)
	// if err != nil {
	// 	return nil, nil, fmt.Errorf(`raft.NewFileSnapshotStore(%q, ...): %w`, baseDir, err)
	// }

	tm := transport.New(raft.ServerAddress(myAddress), []grpc.DialOption{
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	})

	r, err := raft.NewRaft(c, fsm, ldb, sdb, fss, tm.Transport())
	if err != nil {
		return nil, nil, fmt.Errorf("raft.NewRaft: %w", err)
	}

	if bootstrap {
		f := r.BootstrapCluster(cfg)
		if err := f.Error(); err != nil {
			return nil, nil, fmt.Errorf("raft.Raft.BootstrapCluster: %w", err)
		}
	}

	return r, tm, nil
}
