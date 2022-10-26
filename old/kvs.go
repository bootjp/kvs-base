package kvs

import (
	"context"
	"errors"
)

// KVService a simple ket-value store service
type KVService interface {
	Get(ctx context.Context, key Key) (Value, error)
	Put(ctx context.Context, Key Key, value Value) error
	RawGet(ctx context.Context, key Key) (Value, error)
	RawPut(ctx context.Context, key Key, value Value) error
	Delete(ctx context.Context, key Key) error
	CheckExpire()
	Dump() interface{}
}

type OldKVService interface {
	RawGet(key Key) (Value, error)
	RawPut(key Key, value Value) error
	Delete(key Key) error
	CheckExpire()
	Dump() interface{}
}

type Key []byte
type Value []byte

var ErrNotFound = errors.New("not found")
