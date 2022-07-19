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
	// Your Code Here (1).
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

	//return nil, nil
}

// RawPut puts the target data into storage and returns the corresponding response
func (server *Server) RawPut(_ context.Context, req *kvrpcpb.RawPutRequest) (*kvrpcpb.RawPutResponse, error) {
	// Your Code Here (1).
	// Hint: Consider using Storage.Modify to store data to be modified
	return nil, nil
}

// RawDelete delete the target data from storage and returns the corresponding response
func (server *Server) RawDelete(ctx context.Context, req *kvrpcpb.RawDeleteRequest) (*kvrpcpb.RawDeleteResponse, error) {
	// Your Code Here (1).
	// Hint: Consider using Storage.Modify to store data to be deleted
	m := []storage.Modify{}

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
	// Your Code Here (1).
	// Hint: Consider using reader.IterCF
	return nil, nil
}
