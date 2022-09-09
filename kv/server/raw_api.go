package server

import (
	"context"
	"errors"
	"github.com/bootjp/kvs-base/kv/storage"
	"github.com/bootjp/kvs-base/kv/storage/standalone_storage"

	"github.com/bootjp/kvs-base/proto/pkg/kvrpcpb"
)

// The functions below are Server's Raw API. (implements TinyKvServer).
// Some helper methods can be found in sever.go in the current directory

// RawGet return the corresponding Get response based on RawGetRequest's CF and Key fields
func (server *Server) RawGet(_ context.Context, req *kvrpcpb.RawGetRequest) (*kvrpcpb.RawGetResponse, error) {
	v, err := server.storage.Reader(req.Context)
	if err != nil {
		return nil, err
	}
	raw, err := v.GetCF(req.GetCf(), req.GetKey())

	var notfound bool
	if err != nil {
		notfound = errors.Is(err, standalone_storage.ErrNotFound)
	}

	return &kvrpcpb.RawGetResponse{
		Value:    raw,
		NotFound: notfound,
	}, nil
}

// RawPut puts the target data into storage and returns the corresponding response
func (server *Server) RawPut(_ context.Context, req *kvrpcpb.RawPutRequest) (*kvrpcpb.RawPutResponse, error) {
	var m []storage.Modify
	m = append(m, storage.Modify{
		Data: storage.Put{
			Key:   req.GetKey(),
			Cf:    req.GetCf(),
			Value: req.GetValue(),
		},
	})
	p := &kvrpcpb.Context{}

	err := server.storage.Write(p, m)
	return nil, err
}

// RawDelete delete the target data from storage and returns the corresponding response
func (server *Server) RawDelete(ctx context.Context, req *kvrpcpb.RawDeleteRequest) (*kvrpcpb.RawDeleteResponse, error) {
	var m []storage.Modify

	m = append(m, storage.Modify{
		Data: storage.Delete{
			Key: req.GetKey(),
			Cf:  req.GetCf(),
		},
	})
	p := &kvrpcpb.Context{}

	err := server.storage.Write(p, m)

	if err != nil {
		return &kvrpcpb.RawDeleteResponse{
			Error: err.Error(),
		}, nil
	}

	return &kvrpcpb.RawDeleteResponse{}, nil

}

// RawScan scan the data starting from the start key up to limit. and return the corresponding result
func (server *Server) RawScan(_ context.Context, req *kvrpcpb.RawScanRequest) (*kvrpcpb.RawScanResponse, error) {
	reader, err := server.storage.Reader(req.Context)
	if err != nil {
		return nil, err
	}
	it := reader.IterCF(req.GetCf())
	defer it.Close()

	res := &kvrpcpb.RawScanResponse{
		Kvs: []*kvrpcpb.KvPair{},
	}
	cnt := uint32(1)

	for it.Seek(req.GetStartKey()); it.Valid(); it.Next() {
		item := it.Item()
		v, err := item.Value()
		if err != nil {
			return nil, err
		}
		res.Kvs = append(res.Kvs, &kvrpcpb.KvPair{
			Key:   item.Key(),
			Value: v,
		})
		if req.Limit <= cnt {
			break
		}
		cnt++
	}

	return res, nil
}
