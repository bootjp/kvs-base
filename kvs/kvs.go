package kvs

import "github.com/dgraph-io/badger/v3"

type KVS interface {
	RawGet(key []byte) ([]byte, error)
	RawPut(key []byte, value []byte) error
	RawDelete(key []byte) error
}

func NewKVS(path string) (KVS, error) {
	db, err := badger.Open(badger.DefaultOptions(path))
	if err != nil {
		return nil, err
	}

	return &KVService{
		db: db,
	}, nil
}

func (k KVService) RawGet(key []byte) ([]byte, error) {
	var res []byte

	err := k.db.View(func(txn *badger.Txn) error {
		item, err := txn.Get(key)
		if err != nil {
			return err
		}
		return item.Value(func(val []byte) error {
			res = val
			return nil
		})
	})
	if err == badger.ErrKeyNotFound {
		return nil, nil
	}

	return res, err
}

func (k KVService) RawPut(key []byte, value []byte) error {
	return k.db.Update(func(txn *badger.Txn) error {
		err := txn.Set(key, value)
		return err
	})
}

func (k KVService) RawDelete(key []byte) error {
	return k.db.Update(func(txn *badger.Txn) error {
		err := txn.Delete(key)
		return err
	})
}

// Dumper debugging interface
type Dumper interface {
	Dump() string
}

type KVService struct {
	db *badger.DB
}
