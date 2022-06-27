package kvs

import (
	"bytes"
	"encoding/gob"
	"errors"
	"fmt"
	"hash/fnv"
	"log"
	"os"
	"sync"
	"time"
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
	Key      *[]byte
	Value    *[]byte
	IsDelete bool
	Expire   Expire
}

type Transaction struct {
	Pair []Pair
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

type KV map[uint64]*Pair

type KVS struct {
	mtx    sync.RWMutex
	data   KV
	expire KV

	txMtx sync.Mutex
	txKey map[uint64]struct{}
}

var ErrNotFound = errors.New("not found")

func (k *KVS) Get(key *[]byte) (*Pair, error) {
	k.mtx.RLock()
	defer k.mtx.RUnlock()

	v, ok := k.data[k.hash(key)]
	if !ok {
		return nil, ErrNotFound
	}

	// check expire
	if !v.Expire.NoExpire && v.Expire.Expire(time.Now().UTC()) {
		return nil, ErrNotFound
	}

	return v, nil
}

func (k *KVS) Delete(key *[]byte) error {
	k.mtx.Lock()
	defer k.mtx.Unlock()

	delete(k.data, k.hash(key))
	delete(k.expire, k.hash(key))
	debugLog("delete", key)

	return nil
}

func (k *KVS) Set(p *Pair) error {
	k.mtx.Lock()
	defer k.mtx.Unlock()

	po := k.hash(p.Key)

	k.data[po] = p
	if !p.Expire.NoExpire {
		k.expire[k.hash(p.Key)] = p
	}

	return nil
}

const transactionLockWait = time.Millisecond * 100

var ErrTransactionAbort = errors.New("transaction abort")

var ErrFailedDecode = errors.New("failed decode")

func (k *KVS) handlePair(b []byte) error {
	p, err := DecodePair(b)
	if err != nil {
		return ErrFailedDecode
	}

	// TODO mark it as deleted for performance. Remove from Map when creating snapshot
	if p.IsDelete {
		if err := k.Delete(p.Key); err != nil {
			return err
		}

	} else {
		if err := k.Set(&p); err != nil {
			return err
		}
	}

	return nil
}

func (k *KVS) handleTransaction(b []byte) error {
	k.mtx.Lock()
	defer k.mtx.Unlock()

	t, err := DecodeTrans(b)
	if err != nil {
		return ErrFailedDecode
	}

	keys := map[uint64]struct{}{}
	for _, pair := range t.Pair {
		keys[k.hash(pair.Key)] = struct{}{}
	}

	for !k.txMtx.TryLock() {
		time.Sleep(transactionLockWait)
	}
	defer k.txMtx.Unlock()

	k.txKey = keys

	// todo rollback
	for i, pair := range t.Pair {
		switch {
		case pair.IsDelete:
			err = k.Delete(pair.Key)
		default:
			err = k.Set(&t.Pair[i])
		}
		if err != nil {
			return ErrTransactionAbort
		}
	}
	// todo prevent dirty read by mark as committed

	k.txKey = map[uint64]struct{}{}

	return nil
}

func (k *KVS) hash(b *[]byte) uint64 {
	h := fnv.New64()
	if _, err := h.Write(*b); err != nil {
		panic(err)
	}
	return h.Sum64()
}

func NewKVS() *KVS {
	s := &KVS{}
	s.data = KV{}
	s.expire = KV{}
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

func EncodeTrans(p Transaction) ([]byte, error) {
	b := &bytes.Buffer{}
	e := gob.NewEncoder(b)

	if err := e.Encode(p); err != nil {
		return nil, ErrEncode
	}

	return b.Bytes(), nil
}

func DecodeTrans(b []byte) (Transaction, error) {
	var transaction Transaction
	bv := bytes.NewBuffer(b)
	d := gob.NewDecoder(bv)

	if err := d.Decode(&transaction); err != nil {
		return Transaction{}, fmt.Errorf("failed restore: %w", err)
	}

	return transaction, nil
}

func cloneKV(kv KV) KV {
	cloned := KV{}

	for k, v := range kv {
		cloned[k] = v
	}

	return cloned
}
