package kvs

import (
	"bytes"
	"encoding/gob"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"sync"
	"time"

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
	Key      *[KeyLimit]byte
	Value    *[]byte
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
	mtx    sync.RWMutex
	data   KV
	expire KV
}

func NewKVS() *KVS {
	s := &KVS{}
	s.data = map[[KeyLimit]byte]*Pair{}
	s.expire = map[[KeyLimit]byte]*Pair{}
	return s
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
	f.mtx.Lock()
	defer f.mtx.Unlock()
	p, err := DecodePair(l.Data)
	if err != nil {
		return err
	}

	// TODO mark it as deleted for performance. Remove from Map when creating snapshot
	if p.IsDelete {
		debugLog("delete", *p.Key)
		delete(f.data, *p.Key)
		delete(f.expire, *p.Key)
		return true
	}

	f.data[*p.Key] = &p
	if !p.Expire.NoExpire {
		f.expire[*p.Key] = &p
	}

	return true
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
	KVS         *KVS
	Raft        *raft.Raft
	gcc         chan Pair
	Environment string
}

const gcMaxBuffer = 65534

const gcInterval = 500 * time.Millisecond

func NewRPCInterface(kvs *KVS, raft *raft.Raft) *RPCInterface {
	r := &RPCInterface{
		KVS:  kvs,
		Raft: raft,
		gcc:  make(chan Pair, gcMaxBuffer),
	}
	go (func(r *RPCInterface) {
		debugLog("run gc")
		ticker := time.NewTicker(gcInterval)
		for {
			select {
			case v := <-r.gcc:
				v.IsDelete = true
				e, err := EncodePair(v)
				if err != nil {
					log.Println(err)
				}
				debugLog("apply delete")
				_ = r.Raft.Apply(e, time.Second)
			case <-ticker.C:
				now := time.Now().UTC()
				r.KVS.mtx.RLock()
				for _, pair := range r.KVS.expire {
					if !pair.Expire.Expire(now) {
						continue
					}
					debugLog("detect expire key", pair)
					r.gcc <- *pair
				}
				r.KVS.mtx.RUnlock()
			}
		}
	})(r)
	return r
}

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
