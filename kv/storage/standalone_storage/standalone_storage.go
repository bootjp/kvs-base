package standalone_storage

import (
	"errors"
	"github.com/bootjp/kvs-base/kv/config"
	"github.com/bootjp/kvs-base/kv/storage"
	"github.com/bootjp/kvs-base/kv/util/engine_util"
	"github.com/bootjp/kvs-base/proto/pkg/kvrpcpb"
	"github.com/dgraph-io/badger/v3"
)

var _ storage.Storage = &StandAloneStorage{}

// StandAloneStorage is an implementation of `Storage` for a single-node TinyKV instance. It does not
// communicate with other nodes and all data is stored locally.
type StandAloneStorage struct {
	db *badger.DB
}

func NewStandAloneStorage(conf *config.Config) (*StandAloneStorage, error) {
	db, err := badger.Open(badger.DefaultOptions(conf.DBPath))
	if err != nil {
		return nil, err
	}

	return &StandAloneStorage{db: db}, nil
}

func (s *StandAloneStorage) Start() error {
	return nil
}

func (s *StandAloneStorage) Stop() error {
	return s.db.Close()
}
func (s *StandAloneStorage) Reader(ctx *kvrpcpb.Context) (storage.StorageReader, error) {

	return StandAloneStorageReader{
		db: s.db,
	}, nil
}

func (s *StandAloneStorage) Write(ctx *kvrpcpb.Context, batch []storage.Modify) error {
	t := s.db.NewTransaction(true)
	for _, modify := range batch {
		err := t.Set(modify.Key(), modify.Value())
		if err != nil {
			return err
		}
	}
	return t.Commit()
}

type StandAloneStorageReader struct {
	db *badger.DB
}

func (s StandAloneStorageReader) GetCF(cf string, key []byte) ([]byte, error) {
	var value []byte
	err := s.db.View(func(txn *badger.Txn) error {
		v, err := txn.Get(key)
		if errors.Is(err, badger.ErrKeyNotFound) {
			value = nil
			return err
		}
		err = v.Value(func(val []byte) error {
			value = val
			return nil
		})
		if err != nil {
			return err
		}

		return nil
	})

	return value, err
}

func (s StandAloneStorageReader) IterCF(cf string) engine_util.DBIterator {
	return nil
}

func (s StandAloneStorageReader) Close() {
}
