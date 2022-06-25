package kvs

import (
	"bytes"
	"context"
	"encoding/gob"
	"errors"
	"fmt"
	"io"
	"time"

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

// check raft.FSM impl
var _ raft.FSM = &KVS{}

func (k *KVS) Apply(l *raft.Log) interface{} {
	p, err := DecodePair(l.Data)
	if err != nil {
		return err
	}

	// TODO mark it as deleted for performance. Remove from Map when creating snapshot
	if p.IsDelete {
		err := k.Delete(p.Key)
		if err != nil {
			return err
		}
	} else {
		err := k.Set(&p)
		if err != nil {
			return err
		}
	}

	return true
}

func (k *KVS) Snapshot() (raft.FSMSnapshot, error) {
	// Make sure that any future calls to f.Apply() don't change the snapshot.
	return &snapshot{cloneKV(k.data)}, nil
}

func (k *KVS) Restore(r io.ReadCloser) error {
	var decodedMap KV
	d := gob.NewDecoder(r)

	if err := d.Decode(&decodedMap); err != nil {
		return fmt.Errorf("failed restore: %w", err)
	}

	return nil
}

type snapshot struct {
	data KV
}

func (s *snapshot) Persist(sink raft.SnapshotSink) error {
	b := &bytes.Buffer{}
	e := gob.NewEncoder(b)

	err := e.Encode(s.data)
	if err != nil {
		return fmt.Errorf("failed data encode: %w", err)
	}

	_, err = sink.Write(b.Bytes())
	if err != nil {
		_ = sink.Cancel()
		return fmt.Errorf("sink.Write(): %w", err)
	}
	return errors.Unwrap(sink.Close())
}

func (s *snapshot) Release() {}

func TTLtoTime(d time.Duration) Expire {
	switch d.Milliseconds() {
	default:
		return Expire{
			Time: time.Now().UTC().Add(d),
		}
	case 0:
		return Expire{
			NoExpire: true,
		}
	}

}
