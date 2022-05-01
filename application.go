package main

import (
	"bytes"
	"context"
	"encoding/gob"
	"fmt"
	"io"
	"sync"
	"time"

	"github.com/Jille/raft-grpc-leader-rpc/rafterrors"
	pb "github.com/bootjp/kvs-infrastructure/proto"
	"github.com/hashicorp/raft"
)

const KeyLimit = 512

type Pair struct {
	Key   *[KeyLimit]byte
	Value *[]byte
}

type KV map[[KeyLimit]byte]*Pair

// wordTracker keeps track of the three longest words it ever saw.
type wordTracker struct {
	mtx   sync.RWMutex
	words [3]string
	data  KV
}

var _ raft.FSM = &wordTracker{}

func cloneKV(kv KV) KV {
	var cloned KV

	for k, v := range kv {
		cloned[k] = v
	}

	return cloned
}

func (f *wordTracker) Apply(l *raft.Log) interface{} {
	f.mtx.Lock()
	defer f.mtx.Unlock()
	//  real kv data
	_ = string(l.Data)

	return nil
}

func (f *wordTracker) Snapshot() (raft.FSMSnapshot, error) {
	// Make sure that any future calls to f.Apply() don't change the snapshot.
	return &snapshot{cloneKV(f.data)}, nil
}

func (f *wordTracker) Restore(r io.ReadCloser) error {
	var decodedMap KV
	d := gob.NewDecoder(r)

	err := d.Decode(&decodedMap)
	if err != nil {
		return fmt.Errorf("failed restore: %v", err)
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
		return fmt.Errorf("failed data encode: %v", err)
	}

	_, err = sink.Write(b.Bytes())
	if err != nil {
		_ = sink.Cancel()
		return fmt.Errorf("sink.Write(): %v", err)
	}
	return sink.Close()
}

func (s *snapshot) Release() {}

type rpcInterface struct {
	wordTracker *wordTracker
	raft        *raft.Raft
}

func (r rpcInterface) AddData(ctx context.Context, req *pb.AddDataRequest) (*pb.AddDataResponse, error) {
	f := r.raft.Apply(req.GetData(), time.Second)
	if err := f.Error(); err != nil {
		return nil, rafterrors.MarkRetriable(err)
	}
	return &pb.AddDataResponse{
		CommitIndex: f.Index(),
	}, nil
}

func (r rpcInterface) GetData(ctx context.Context, req *pb.GetDataRequest) (*pb.GetDataResponse, error) {
	r.wordTracker.mtx.RLock()
	defer r.wordTracker.mtx.RUnlock()

	if len(req.GetKey()) > KeyLimit {
		// todo handling error response
		return nil, fmt.Errorf("reachd key size limit: %d max key size %d", len(req.Key), KeyLimit)
	}

	var tmp [512]byte
	copy(tmp[:], req.Key[:])

	v, ok := r.wordTracker.data[tmp]
	if !ok {
		// data not found
		// todo add error code by proto
		return &pb.GetDataResponse{
			Key:         req.Key,
			Data:        []byte("NO DATA"),
			ReadAtIndex: r.raft.AppliedIndex(),
		}, nil
	}

	return &pb.GetDataResponse{
		Key:         req.Key,
		Data:        *v.Value,
		ReadAtIndex: r.raft.AppliedIndex(),
	}, nil

}
