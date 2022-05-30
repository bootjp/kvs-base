package kvs

import (
	"bytes"
	"context"
	"encoding/gob"
	"errors"
	"fmt"
	"io"
	"log"
	"sync"
	"time"

	"github.com/Jille/raft-grpc-leader-rpc/rafterrors"
	pb "github.com/bootjp/kvs/proto"
	"github.com/hashicorp/raft"
)

const KeyLimit = 512

type Pair struct {
	Key      *[KeyLimit]byte
	Value    *[]byte
	IsDelete bool
}

type KV map[[KeyLimit]byte]*Pair

type KVS struct {
	mtx  sync.RWMutex
	data KV
}

func NewKVS() *KVS {
	wt := &KVS{}
	wt.data = map[[KeyLimit]byte]*Pair{}
	return wt
}

func EncodePair(p Pair) ([]byte, error) {
	b := &bytes.Buffer{}
	e := gob.NewEncoder(b)

	if err := e.Encode(p); err != nil {
		return nil, fmt.Errorf("failed data encode: %w", err)
	}

	return b.Bytes(), nil
}

func DecodePair(b []byte) (Pair, error) {
	var pair Pair
	bv := bytes.NewBuffer(b)
	d := gob.NewDecoder(bv)

	if err := d.Decode(&pair); err != nil {
		return Pair{}, fmt.Errorf("failed restore: %w", err)
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
	// real kv data
	p, err := DecodePair(l.Data)
	if err != nil {
		log.Println(err)
	}

	if p.IsDelete {
		delete(f.data, *p.Key)
		return nil
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

type RPCInterface struct {
	KVS  *KVS
	Raft *raft.Raft
}

func (r RPCInterface) DeleteData(_ context.Context, req *pb.DeleteRequest) (*pb.DeleteResponse, error) {
	if len(req.GetKey()) > KeyLimit {
		return &pb.DeleteResponse{
			Status:      pb.Status_ABORT,
			CommitIndex: r.Raft.AppliedIndex(),
		}, fmt.Errorf("reachd key size limit: %d max key size %d", len(req.Key), KeyLimit)
	}

	var tmp [KeyLimit]byte
	copy(tmp[:], req.Key)

	pair := Pair{Key: &tmp, Value: nil, IsDelete: true}
	e, err := EncodePair(pair)
	if err != nil {
		return &pb.DeleteResponse{
			Status: pb.Status_ABORT,
		}, err
	}

	f := r.Raft.Apply(e, time.Second)
	if err := f.Error(); err != nil {
		return &pb.DeleteResponse{
			CommitIndex: f.Index(),
			Status:      pb.Status_ABORT,
		}, errors.Unwrap(rafterrors.MarkRetriable(err))
	}

	return &pb.DeleteResponse{
		CommitIndex: f.Index(),
		Status:      pb.Status_COMMIT,
	}, nil
}

func (r RPCInterface) AddData(_ context.Context, req *pb.AddDataRequest) (*pb.AddDataResponse, error) {
	if len(req.GetKey()) > KeyLimit {
		return &pb.AddDataResponse{
			Status:      pb.Status_ABORT,
			CommitIndex: r.Raft.AppliedIndex(),
		}, fmt.Errorf("reachd key size limit: %d max key size %d", len(req.Key), KeyLimit)
	}

	var tmp [KeyLimit]byte
	copy(tmp[:], req.Key)

	pair := Pair{Key: &tmp, Value: &req.Data}
	e, err := EncodePair(pair)
	if err != nil {
		return &pb.AddDataResponse{
			Status: pb.Status_ABORT,
		}, err
	}

	f := r.Raft.Apply(e, time.Second)
	if err := f.Error(); err != nil {
		return &pb.AddDataResponse{
			CommitIndex: f.Index(),
			Status:      pb.Status_ABORT,
		}, errors.Unwrap(rafterrors.MarkRetriable(err))
	}

	return &pb.AddDataResponse{
		CommitIndex: f.Index(),
		Status:      pb.Status_COMMIT,
	}, nil
}

func (r RPCInterface) GetData(_ context.Context, req *pb.GetDataRequest) (*pb.GetDataResponse, error) {
	r.KVS.mtx.RLock()
	defer r.KVS.mtx.RUnlock()

	if len(req.GetKey()) > KeyLimit {
		return &pb.GetDataResponse{
			Key:         req.Key,
			Data:        nil,
			Error:       pb.GetDataError_FETCH_ERROR,
			ReadAtIndex: r.Raft.AppliedIndex(),
		}, fmt.Errorf("reachd key size limit: %d max key size %d", len(req.Key), KeyLimit)
	}

	var tmp [KeyLimit]byte
	copy(tmp[:], req.Key)

	v, ok := r.KVS.data[tmp]
	if !ok {
		return &pb.GetDataResponse{
			Key:         req.Key,
			Data:        nil,
			Error:       pb.GetDataError_DATA_NOT_FOUND,
			ReadAtIndex: r.Raft.AppliedIndex(),
		}, nil
	}

	return &pb.GetDataResponse{
		Key:         req.Key,
		Data:        *v.Value,
		Error:       pb.GetDataError_NO_ERROR,
		ReadAtIndex: r.Raft.AppliedIndex(),
	}, nil

}
