package kvs

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/Jille/raft-grpc-leader-rpc/rafterrors"
	pb "github.com/bootjp/kvs/proto"
)

func (r RPCInterface) Delete(_ context.Context, req *pb.DeleteRequest) (*pb.DeleteResponse, error) {
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

func (r RPCInterface) Put(_ context.Context, req *pb.AddDataRequest) (*pb.AddDataResponse, error) {
	if len(req.GetKey()) > KeyLimit {
		return &pb.AddDataResponse{
			Status:      pb.Status_ABORT,
			CommitIndex: r.Raft.AppliedIndex(),
		}, fmt.Errorf("reachd key size limit: %d max key size %d", len(req.Key), KeyLimit)
	}

	var tmp [KeyLimit]byte
	copy(tmp[:], req.Key)

	pair := Pair{Key: &tmp, Value: &req.Data, Expire: TTLtoTime(req.GetTtl().AsDuration())}
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

	resp := f.Response()
	if err, ok := resp.(error); ok {
		return &pb.AddDataResponse{
			CommitIndex: f.Index(),
			Status:      pb.Status_ABORT,
		}, errors.Unwrap(rafterrors.MarkRetriable(err))
	}

	ff := r.Raft.Barrier(time.Second)
	if err := ff.Error(); err != nil {
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

func (r RPCInterface) Get(_ context.Context, req *pb.GetDataRequest) (*pb.GetDataResponse, error) {
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

	// check expire
	if !v.Expire.NoExpire && v.Expire.Expire(time.Now().UTC()) {
		r.gcc <- *v
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

func (r RPCInterface) Transaction(_ context.Context, req *pb.TransactionRequest) (*pb.TransactionResponse, error) {

	r.KVS.mtx.RLock()
	defer r.KVS.mtx.RUnlock()

	for s, op := range req.GetPair() {
		if len(s) > KeyLimit {
			return &pb.TransactionResponse{
				Status: pb.Status_ABORT,
			}, fmt.Errorf("reachd key size limit: %d max key size %d", len(s), KeyLimit)
		}
		var tmp [KeyLimit]byte
		copy(tmp[:], s)

		value := op.GetData()
		pair := Pair{Key: &tmp, Value: &value, Expire: TTLtoTime(0), IsDelete: op.GetDelete()}
		e, err := EncodePair(pair)
		if err != nil {
			return &pb.TransactionResponse{
				Status: pb.Status_ABORT,
			}, err
		}

		// todo transaction type
		f := r.Raft.Apply(e, time.Second)
		if err := f.Error(); err != nil {
			return &pb.TransactionResponse{
				Status: pb.Status_ABORT,
			}, errors.Unwrap(rafterrors.MarkRetriable(err))
		}
	}

	return &pb.TransactionResponse{
		Status: pb.Status_COMMIT,
	}, nil
}
