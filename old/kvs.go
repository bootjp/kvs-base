package kvs

import "errors"

// KVService a simple ket-value store service, Network layer, not concerned with Raft
type KVService interface {
	Get(key Key) (Value, error)
	Set(key Key, value Value) error
	Delete(key Key) error
	CheckExpire()
	Dump() interface{}
}

type Key []byte
type Value []byte

var ErrNotFound = errors.New("not found")
