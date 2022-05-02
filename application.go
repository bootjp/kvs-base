package main

import (
	"bytes"
	"context"
	"encoding/gob"
	"fmt"
	"io"
	"log"
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

type KVS struct {
	mtx  sync.RWMutex
	data KV
}

func EncodePair(p Pair) ([]byte, error) {
	b := &bytes.Buffer{}
	e := gob.NewEncoder(b)

	err := e.Encode(p)
	if err != nil {
		return nil, fmt.Errorf("failed data encode: %v", err)
	}

	return b.Bytes(), nil
}

func DecodePair(b []byte) (Pair, error) {
	var pair Pair
	bv := bytes.NewBuffer(b)
	d := gob.NewDecoder(bv)

	err := d.Decode(&pair)
	if err != nil {
		return Pair{}, fmt.Errorf("failed restore: %v", err)
	}

	return pair, nil
}

var _ raft.FSM = &KVS{}

func cloneKV(kv KV) KV {
	cloned := KV{}

	for k, v := range kv {
		cloned[k] = v
	}

	return cloned
}

func (f *KVS) Apply(l *raft.Log) interface{} {
	f.mtx.Lock()
	defer f.mtx.Unlock()
	//  real kv data
	p, err := DecodePair(l.Data)
	if err != nil {
		log.Fatal(err)
	}

	f.data[*p.Key] = &p

	return nil
}

func (f *KVS) Snapshot() (raft.FSMSnapshot, error) {
	// Make sure that any future calls to f.Apply() don't change the snapshot.
	return &snapshot{cloneKV(f.data)}, nil
}

func (f *KVS) Restore(r io.ReadCloser) error {
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
	wordTracker *KVS
	raft        *raft.Raft
}

func (r rpcInterface) AddData(ctx context.Context, req *pb.AddDataRequest) (*pb.AddDataResponse, error) {
	if len(req.GetKey()) > KeyLimit {
		return &pb.AddDataResponse{
			Status:      pb.Status_ABORT,
			CommitIndex: r.raft.AppliedIndex(),
		}, fmt.Errorf("reachd key size limit: %d max key size %d", len(req.Key), KeyLimit)
	}

	var tmp [KeyLimit]byte
	copy(tmp[:], req.Key[:])

	pair := Pair{Key: &tmp, Value: &req.Data}
	e, err := EncodePair(pair)
	if err != nil {
		return &pb.AddDataResponse{
			Status: pb.Status_ABORT,
		}, err
	}

	f := r.raft.Apply(e, time.Second)
	if err := f.Error(); err != nil {
		return &pb.AddDataResponse{
			CommitIndex: f.Index(),
			Status:      pb.Status_ABORT,
		}, rafterrors.MarkRetriable(err)
	}

	return &pb.AddDataResponse{
		CommitIndex: f.Index(),
		Status:      pb.Status_COMMIT,
	}, nil
}

func (r rpcInterface) GetData(ctx context.Context, req *pb.GetDataRequest) (*pb.GetDataResponse, error) {
	r.wordTracker.mtx.RLock()
	defer r.wordTracker.mtx.RUnlock()

	if len(req.GetKey()) > KeyLimit {
		// todo handling error response
		return &pb.GetDataResponse{
			Key:         req.Key,
			Data:        nil,
			Error:       pb.GetDataError_FETCH_ERROR,
			ReadAtIndex: r.raft.AppliedIndex(),
		}, fmt.Errorf("reachd key size limit: %d max key size %d", len(req.Key), KeyLimit)
	}

	var tmp [KeyLimit]byte
	copy(tmp[:], req.Key[:])

	v, ok := r.wordTracker.data[tmp]
	if !ok {
		return &pb.GetDataResponse{
			Key:         req.Key,
			Data:        nil,
			Error:       pb.GetDataError_DATA_NOT_FOUND,
			ReadAtIndex: r.raft.AppliedIndex(),
		}, nil
	}

	return &pb.GetDataResponse{
		Key:         req.Key,
		Data:        *v.Value,
		Error:       pb.GetDataError_NO_ERROR,
		ReadAtIndex: r.raft.AppliedIndex(),
	}, nil

}
