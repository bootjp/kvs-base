package kvs

import (
	"github.com/bootjp/kvs-base/proto/pkg/tinykvpb"
	"github.com/pingcap-incubator/tinykv/proto/pkg/coprocessor"
	"github.com/pingcap-incubator/tinykv/proto/pkg/kvrpcpb"
	xcontext "golang.org/x/net/context"
)

type RPC struct {
	kvs *MemStore
}

func NewRPC() *RPC {
	return &RPC{
		kvs: NewKVS(),
	}

}

func (r *RPC) KvGet(c xcontext.Context, request *kvrpcpb.GetRequest) (*kvrpcpb.GetResponse, error) {
	//TODO implement me
	panic("implement me")
}

func (r *RPC) KvScan(c xcontext.Context, request *kvrpcpb.ScanRequest) (*kvrpcpb.ScanResponse, error) {
	//TODO implement me
	panic("implement me")
}

func (r *RPC) KvPrewrite(c xcontext.Context, request *kvrpcpb.PrewriteRequest) (*kvrpcpb.PrewriteResponse, error) {
	//TODO implement me
	panic("implement me")
}

func (r *RPC) KvCommit(c xcontext.Context, request *kvrpcpb.CommitRequest) (*kvrpcpb.CommitResponse, error) {
	//TODO implement me
	panic("implement me")
}

func (r *RPC) KvCheckTxnStatus(c xcontext.Context, request *kvrpcpb.CheckTxnStatusRequest) (*kvrpcpb.CheckTxnStatusResponse, error) {
	//TODO implement me
	panic("implement me")
}

func (r *RPC) KvBatchRollback(c xcontext.Context, request *kvrpcpb.BatchRollbackRequest) (*kvrpcpb.BatchRollbackResponse, error) {
	//TODO implement me
	panic("implement me")
}

func (r *RPC) KvResolveLock(c xcontext.Context, request *kvrpcpb.ResolveLockRequest) (*kvrpcpb.ResolveLockResponse, error) {
	//TODO implement me
	panic("implement me")
}

func (r *RPC) RawGet(c xcontext.Context, request *kvrpcpb.RawGetRequest) (*kvrpcpb.RawGetResponse, error) {
	v, err := r.kvs.Get(request.GetKey())
	if err != nil {
		return nil, err
	}
	return &kvrpcpb.RawGetResponse{
		Value: v,
	}, nil
}

func (r *RPC) RawPut(c xcontext.Context, request *kvrpcpb.RawPutRequest) (*kvrpcpb.RawPutResponse, error) {
	err := r.kvs.Set(request.Key, request.Value)
	if err != nil {
		return nil, err
	}
	return &kvrpcpb.RawPutResponse{}, nil
}

func (r *RPC) RawDelete(c xcontext.Context, request *kvrpcpb.RawDeleteRequest) (*kvrpcpb.RawDeleteResponse, error) {
	//TODO implement me
	panic("implement me")
}

func (r *RPC) RawScan(c xcontext.Context, request *kvrpcpb.RawScanRequest) (*kvrpcpb.RawScanResponse, error) {
	//TODO implement me
	panic("implement me")
}

func (r *RPC) Raft(server tinykvpb.TinyKv_RaftServer) error {
	//TODO implement me
	panic("implement me")
}

func (r *RPC) Snapshot(server tinykvpb.TinyKv_SnapshotServer) error {
	//TODO implement me
	panic("implement me")
}

func (r *RPC) Coprocessor(c xcontext.Context, request *coprocessor.Request) (*coprocessor.Response, error) {
	//TODO implement me
	panic("implement me")
}

//
//func (r RPCInterface) checkKeyLimit(l int) bool {
//	return l > KeyLimit
//}
//
//func (r RPCInterface) Delete(_ context.Context, req *pb.DeleteRequest) (*pb.DeleteResponse, error) {
//	if r.checkKeyLimit(len(req.GetKey())) {
//		return &pb.DeleteResponse{
//			Status:      pb.Status_ABORT,
//			CommitIndex: r.Raft.AppliedIndex(),
//		}, fmt.Errorf("reachd key size limit: %d max key size %d", len(req.Key), KeyLimit)
//	}
//
//	pair := Pair{Key: &req.Key, Value: nil, IsDelete: true}
//	e, err := EncodePair(pair)
//	if err != nil {
//		return &pb.DeleteResponse{
//			Status: pb.Status_ABORT,
//		}, err
//	}
//
//	f := r.Raft.Apply(e, time.Second)
//	r.Raft.Barrier(time.Second)
//
//	if err := f.Error(); err != nil {
//		return &pb.DeleteResponse{
//			CommitIndex: f.Index(),
//			Status:      pb.Status_ABORT,
//		}, errors.Unwrap(rafterrors.MarkRetriable(err))
//	}
//
//	return &pb.DeleteResponse{
//		CommitIndex: f.Index(),
//		Status:      pb.Status_COMMIT,
//	}, nil
//}
//
//var ErrInvalidRequest = errors.New("invalid request")
//
//func (r RPCInterface) Put(_ context.Context, req *pb.PutRequest) (*pb.PutResponse, error) {
//	if req.GetData() == nil || req.Data.GetKey() == nil || req.Data.GetValue() == nil {
//		return nil, ErrInvalidRequest
//	}
//	if r.checkKeyLimit(len(req.Data.GetKey())) {
//		return &pb.PutResponse{
//			Status:      pb.Status_ABORT,
//			CommitIndex: r.Raft.AppliedIndex(),
//		}, fmt.Errorf("reachd key size limit: %d max key size %d", len(req.GetData().Key), KeyLimit)
//	}
//	RPC := req.GetData().GetKey()
//	pair := Pair{Key: &RPC, Value: &req.GetData().Value, Expire: TTLtoTime(req.GetData().GetTtl().AsDuration())}
//	e, err := EncodePair(pair)
//	if err != nil {
//		return &pb.PutResponse{
//			Status: pb.Status_ABORT,
//		}, err
//	}
//
//	f := r.Raft.Apply(e, time.Second)
//	if err := f.Error(); err != nil {
//		return &pb.PutResponse{
//			CommitIndex: f.Index(),
//			Status:      pb.Status_ABORT,
//		}, errors.Unwrap(rafterrors.MarkRetriable(err))
//	}
//
//	resp := f.Response()
//	if err, ok := resp.(error); ok {
//		return &pb.PutResponse{
//			CommitIndex: f.Index(),
//			Status:      pb.Status_ABORT,
//		}, errors.Unwrap(rafterrors.MarkRetriable(err))
//	}
//
//	ff := r.Raft.Barrier(time.Second)
//	if err := ff.Error(); err != nil {
//		return &pb.PutResponse{
//			CommitIndex: f.Index(),
//			Status:      pb.Status_ABORT,
//		}, errors.Unwrap(rafterrors.MarkRetriable(err))
//	}
//
//	return &pb.PutResponse{
//		CommitIndex: f.Index(),
//		Status:      pb.Status_COMMIT,
//	}, nil
//}
//
//func (r RPCInterface) Get(_ context.Context, req *pb.GetRequest) (*pb.GetResponse, error) {
//	if req.GetKey() == nil {
//		return nil, ErrInvalidRequest
//	}
//	if r.checkKeyLimit(len(req.GetKey())) {
//		return &pb.GetResponse{
//			Key:         req.Key,
//			Data:        nil,
//			Error:       pb.GetDataError_FETCH_ERROR,
//			ReadAtIndex: r.Raft.AppliedIndex(),
//		}, fmt.Errorf("reachd key size limit: %d max key size %d", len(req.Key), KeyLimit)
//	}
//
//	var rspError = pb.GetDataError_FETCH_ERROR
//
//	v, err := r.MemStore.Get(&req.Key)
//	if err != nil {
//		if errors.Is(err, ErrNotFound) {
//			rspError = pb.GetDataError_DATA_NOT_FOUND
//		}
//
//		return &pb.GetResponse{
//			Key:         req.Key,
//			Data:        nil,
//			Error:       rspError,
//			ReadAtIndex: r.Raft.AppliedIndex(),
//		}, nil
//	}
//
//	return &pb.GetResponse{
//		Key:         req.Key,
//		Data:        *v.Value,
//		Error:       pb.GetDataError_NO_ERROR,
//		ReadAtIndex: r.Raft.AppliedIndex(),
//	}, nil
//
//}
//
//func (r RPCInterface) Transaction(_ context.Context, req *pb.TransactionRequest) (*pb.TransactionResponse, error) {
//
//	r.MemStore.mtx.RLock()
//	defer r.MemStore.mtx.RUnlock()
//
//	txs := Transaction{
//		Aborted: KV{},
//	}
//
//	for s, op := range req.GetPair() {
//		if len(s) > KeyLimit {
//			return &pb.TransactionResponse{
//				Status: pb.Status_ABORT,
//			}, fmt.Errorf("reachd key size limit: %d max key size %d", len(s), KeyLimit)
//		}
//
//		value := op.GetData()
//		RPC := []byte(s)
//		pair := Pair{Key: &RPC, Value: &value, Expire: TTLtoTime(0), IsDelete: op.GetDelete()}
//		txs.Pair = append(txs.Pair, pair)
//		// ignore error because when nodata set nil data
//		txs.Aborted[r.MemStore.hash(pair.Key)], _ = r.MemStore.Get(pair.Key)
//	}
//
//	e, err := EncodeTrans(txs)
//	if err != nil {
//		return &pb.TransactionResponse{
//			Status: pb.Status_ABORT,
//		}, err
//	}
//
//	f := r.Raft.Apply(e, time.Second)
//	if err := f.Error(); err != nil {
//		return &pb.TransactionResponse{
//			Status: pb.Status_ABORT,
//		}, errors.Unwrap(rafterrors.MarkRetriable(err))
//	}
//
//	b := r.Raft.Barrier(time.Second)
//	if err := b.Error(); err != nil {
//		return &pb.TransactionResponse{
//			Status: pb.Status_ABORT,
//		}, errors.Unwrap(rafterrors.MarkRetriable(err))
//	}
//
//	return &pb.TransactionResponse{
//		Status: pb.Status_COMMIT,
//	}, nil
//}
//
//type RPCInterface struct {
//	MemStore         *RPC
//	Raft        *raft.Raft
//	gcc         chan Pair
//	Environment string
//}
//
//const gcMaxBuffer = 65534
//
//const gcInterval = 500 * time.Millisecond
//
//func NewRPCInterface(kvs *RPC, raft *raft.Raft) *RPCInterface {
//	r := &RPCInterface{
//		MemStore:  kvs,
//		Raft: raft,
//		gcc:  make(chan Pair, gcMaxBuffer),
//	}
//	go (func(r *RPCInterface) {
//		debugLog("run gc")
//		ticker := time.NewTicker(gcInterval)
//		for {
//			select {
//			case v := <-r.gcc:
//				v.IsDelete = true
//				e, err := EncodePair(v)
//				if err != nil {
//					log.Println(err)
//				}
//				debugLog("apply delete")
//				_ = r.Raft.Apply(e, time.Second)
//			case <-ticker.C:
//				now := time.Now().UTC()
//				r.MemStore.mtx.RLock()
//				for _, pair := range r.MemStore.expire {
//					if !pair.Expire.Expire(now) {
//						continue
//					}
//					debugLog("detect expire key", pair)
//					r.gcc <- *pair
//				}
//				r.MemStore.mtx.RUnlock()
//			}
//		}
//	})(r)
//	return r
//}
