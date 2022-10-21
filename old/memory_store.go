package kvs

import (
	"hash/fnv"
	"sync"
	"time"
)

type KV map[uint64]Value

type MemStore struct {
	mtx    sync.RWMutex
	data   KV
	expire KV

	expireFunc func(kv Key) error

	txMtx sync.Mutex
	txKey map[uint64]struct{}
}

func (k *MemStore) Get(key Key) (Value, error) {
	k.mtx.RLock()
	defer k.mtx.RUnlock()

	v, ok := k.data[k.hash(key)]
	if !ok {
		return nil, ErrNotFound
	}

	return v, nil
}

func (k *MemStore) CheckExpire() {
	now := time.Now().UTC()
	defer k.mtx.RLock()
	for kk, v := range k.expire {
		_, _, _ = now, v, kk
		err := k.expireFunc(Key(""))
		if err != nil {
			return
		}
	}
}

func (k *MemStore) Delete(key Key) error {
	k.mtx.Lock()
	defer k.mtx.Unlock()
	delete(k.data, k.hash(key))
	delete(k.expire, k.hash(key))

	return nil
}

func (k *MemStore) Set(key Key, value Value) error {
	k.mtx.Lock()
	defer k.mtx.Unlock()

	po := k.hash(key)

	k.data[po] = value

	return nil
}

func (k *MemStore) hash(b []byte) uint64 {
	h := fnv.New64()
	if _, err := h.Write(b); err != nil {
		panic(err)
	}
	return h.Sum64()
}

func (k *MemStore) Dump() interface{} {

	var clone = KV{}
	for k, v := range k.data {
		clone[k] = v
	}

	return clone
}

func NewKVS() *MemStore {
	s := &MemStore{}
	s.data = KV{}
	s.expire = KV{}
	return s
}
