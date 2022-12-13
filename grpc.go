package kvs

import (
	"bytes"
	"context"
	"encoding/gob"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"time"

	"github.com/Jille/raft-grpc-leader-rpc/rafterrors"
	kvss "github.com/bootjp/kvs/kvs"
	pb "github.com/bootjp/kvs/proto"

	"github.com/hashicorp/raft"
)

var isDebug = os.Getenv("DEBUG") == "true"

func debugLog(a ...any) {
	if isDebug {
		log.Println(a...)
	}
}

func init() {
	debugLog("enable debug mode")
}

const KeyLimit = 512

type Pair struct {
	Key      []byte
	Value    []byte
	IsDelete bool
	Expire   Expire
}

type Expire struct {
	Time     time.Time
	NoExpire bool
}

func (e *Expire) Expire(t time.Time) bool {
	if e.NoExpire {
		return false
	}
	return t.After(e.Time)
}

type KV map[[KeyLimit]byte]*Pair

// check raft.FSM impl
var _ raft.FSM = &KVS{}

type KVS struct {
	db kvss.KVS
}

func NewKVS(path string) *KVS {
	db, err := kvss.NewKVS(path)
	if err != nil {
		panic(err)
	}
	return &KVS{
		db: db,
	}
}

var ErrEncode = errors.New("failed data encode")

func EncodePair(p Pair) ([]byte, error) {
	b := &bytes.Buffer{}
	e := gob.NewEncoder(b)

	if err := e.Encode(p); err != nil {
		return nil, ErrEncode
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

func cloneKV(kv KV) KV {
	cloned := KV{}

	for k, v := range kv {
		cloned[k] = v
	}

	return cloned
}

func (f *KVS) Apply(l *raft.Log) interface{} {
	p, err := DecodePair(l.Data)
	if err != nil {
		return err
	}

	// TODO mark it as deleted for performance. Remove from Map when creating snapshot
	switch {
	case p.IsDelete:
		err = f.db.RawDelete(p.Key)
		if err != nil {
			return err
		}
	default:
		err = f.db.RawPut(p.Key, p.Value)
		if err != nil {
			return err
		}
	}

	return true
}

func (f *KVS) Snapshot() (raft.FSMSnapshot, error) {
	// Make sure that any future calls to f.Apply() don't change the snapshot.
	//return &snapshot{cloneKV(f.db)}, nil
	return nil, nil
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
	db kvss.KVS
}

func (s *snapshot) Persist(sink raft.SnapshotSink) error {
	return nil
}

func (s *snapshot) Release() {}

type RPCInterface struct {
	KVS         *KVS
	Raft        *raft.Raft
	gcc         chan Pair
	Environment string
}

func (r RPCInterface) RawPut(ctx context.Context, req *pb.PutRequest) (*pb.PutResponse, error) {
	if len(req.GetKey()) > KeyLimit {
		return &pb.PutResponse{
			Status:      pb.Status_ABORT,
			CommitIndex: r.Raft.AppliedIndex(),
		}, fmt.Errorf("reachd key size limit: %d max key size %d", len(req.Key), KeyLimit)
	}

	pair := Pair{Key: req.Key, Value: req.Value, Expire: TTLtoTime(req.GetTtlSec())}
	e, err := EncodePair(pair)
	if err != nil {
		return &pb.PutResponse{
			Status: pb.Status_ABORT,
		}, err
	}

	f := r.Raft.Apply(e, time.Second)
	if err := f.Error(); err != nil {
		return &pb.PutResponse{
			CommitIndex: f.Index(),
			Status:      pb.Status_ABORT,
		}, errors.Unwrap(rafterrors.MarkRetriable(err))
	}

	resp := f.Response()
	if err, ok := resp.(error); ok {
		return &pb.PutResponse{
			CommitIndex: f.Index(),
			Status:      pb.Status_ABORT,
		}, errors.Unwrap(rafterrors.MarkRetriable(err))
	}

	ff := r.Raft.Barrier(1 * time.Second)
	if err := ff.Error(); err != nil {
		return &pb.PutResponse{
			CommitIndex: f.Index(),
			Status:      pb.Status_ABORT,
		}, errors.Unwrap(rafterrors.MarkRetriable(err))
	}

	return &pb.PutResponse{
		CommitIndex: f.Index(),
		Status:      pb.Status_COMMIT,
	}, nil
}

func (r RPCInterface) RawGet(ctx context.Context, req *pb.GetRequest) (*pb.GetResponse, error) {
	if len(req.GetKey()) > KeyLimit {
		return &pb.GetResponse{
			Key:         req.Key,
			Data:        nil,
			Error:       pb.GetDataError_FETCH_ERROR,
			ReadAtIndex: r.Raft.AppliedIndex(),
		}, fmt.Errorf("reachd key size limit: %d max key size %d", len(req.Key), KeyLimit)
	}

	v, err := r.KVS.db.RawGet(req.Key)
	if err != nil {
		if err == kvss.ErrNotFound {
			return &pb.GetResponse{
				Key:         req.Key,
				Data:        nil,
				Error:       pb.GetDataError_DATA_NOT_FOUND,
				ReadAtIndex: r.Raft.AppliedIndex(),
			}, nil
		}
		return &pb.GetResponse{
			Key:         req.Key,
			Data:        nil,
			Error:       pb.GetDataError_FETCH_ERROR,
			ReadAtIndex: r.Raft.AppliedIndex(),
		}, nil
	}

	return &pb.GetResponse{
		Key:         req.Key,
		Data:        v,
		Error:       pb.GetDataError_NO_ERROR,
		ReadAtIndex: r.Raft.AppliedIndex(),
	}, nil

}

func (r RPCInterface) RawDelete(ctx context.Context, req *pb.DeleteRequest) (*pb.DeleteResponse, error) {
	if len(req.GetKey()) > KeyLimit {
		return &pb.DeleteResponse{
			Status:      pb.Status_ABORT,
			CommitIndex: r.Raft.AppliedIndex(),
		}, fmt.Errorf("reachd key size limit: %d max key size %d", len(req.Key), KeyLimit)
	}

	pair := Pair{Key: req.Key, Value: nil, IsDelete: true}
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

func (r RPCInterface) RawScan(ctx context.Context, request *pb.ScanRequest) (*pb.ScanResponse, error) {
	//TODO implement me
	panic("implement me")
}

func NewRPCInterface(kvs *KVS, raft *raft.Raft) *RPCInterface {
	r := &RPCInterface{
		KVS:  kvs,
		Raft: raft,
	}
	return r
}

func TTLtoTime(d uint64) Expire {
	switch d {
	default:
		return Expire{
			Time: time.Now().UTC().Add(time.Duration(d) * time.Second),
		}
	case 0:
		return Expire{
			NoExpire: true,
		}
	}

}
