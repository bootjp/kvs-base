package standalone_storage

import (
	"errors"
	"github.com/Connor1996/badger"
	"github.com/bootjp/kvs-base/kv/config"
	"github.com/bootjp/kvs-base/kv/storage"
	"github.com/bootjp/kvs-base/kv/util/engine_util"
	"github.com/bootjp/kvs-base/proto/pkg/kvrpcpb"
)

var _ storage.Storage = &StandAloneStorage{}

// StandAloneStorage is an implementation of `Storage` for a single-node TinyKV instance. It does not
// communicate with other nodes and all data is stored locally.
type StandAloneStorage struct {
	db *badger.DB
}

func NewStandAloneStorage(conf *config.Config) (*StandAloneStorage, error) {
	opt := badger.DefaultOptions
	opt.Dir = conf.DBPath
	opt.ValueDir = conf.DBPath

	db, err := badger.Open(opt)
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
		//key := append([]byte(modify.Cf()), []byte("_")...)
		//key = append(key, modify.Key()...)
		switch modify.Data.(type) {
		case storage.Put:
			err := t.Set(engine_util.KeyWithCF(modify.Cf(), modify.Key()), modify.Value())
			if err != nil {
				return err
			}
		case storage.Delete:
			err := t.Delete(engine_util.KeyWithCF(modify.Cf(), modify.Key()))
			if err != nil {
				return err
			}
		}

	}
	return t.Commit()
}

type StandAloneStorageReader struct {
	db *badger.DB
}

var ErrNotFound = errors.New("not found")

func (s StandAloneStorageReader) GetCF(cf string, key []byte) ([]byte, error) {
	var value []byte
	err := s.db.View(func(txn *badger.Txn) error {
		v, err := txn.Get(engine_util.KeyWithCF(cf, key))
		if errors.Is(err, badger.ErrKeyNotFound) {
			value = nil
			return ErrNotFound
		}
		value, err = v.Value()
		if err != nil {
			return err
		}
		//value

		return nil
	})

	return value, err
}

func (s StandAloneStorageReader) IterCF(cf string) engine_util.DBIterator {
	txn := s.db.NewTransaction(false)
	return engine_util.NewCFIterator(cf, txn)
}

func (s StandAloneStorageReader) Close() {
}

type StandAloneStorageIterator struct {
	it     *badger.Iterator
	prefix string
}

//
//type Item struct {
//	item   *badger.Item
//	prefix string
//}
//
//func (i *Item) String() string {
//	return i.item.String()
//}
//
//func (i *Item) Key() []byte {
//	return i.item.Key()[len(i.prefix):]
//}
//
//func (i *Item) KeyCopy(dst []byte) []byte {
//	return i.item.KeyCopy(dst)[len(i.prefix):]
//}
//
//func (i *Item) Value() ([]byte, error) {
//	var ret []byte
//	err := i.item.Value(func(val []byte) error
//		ret = val
//		return nil
//	})
//	return ret, err
//}
//
//func (i *Item) ValueSize() int {
//	return int(i.item.ValueSize())
//}
//
//func (i *Item) ValueCopy(dst []byte) ([]byte, error) {
//	return i.item.ValueCopy(dst)
//}
//
//func (it *StandAloneStorageIterator) Item() engine_util.DBItem {
//	return &Item{
//		item:   it.it.Item(),
//		prefix: it.prefix,
//	}
//}
//
//func (it *StandAloneStorageIterator) Valid() bool { return it.it.ValidForPrefix([]byte(it.prefix)) }
//
//func (it *StandAloneStorageIterator) ValidForPrefix(prefix []byte) bool {
//	return it.it.ValidForPrefix(append([]byte(it.prefix), prefix...))
//}
//
//func (it *StandAloneStorageIterator) Close() {
//	it.it.Close()
//}
//
//func (it *StandAloneStorageIterator) Next() {
//	it.it.Next()
//}
//
//func (it *StandAloneStorageIterator) Seek(key []byte) {
//	it.it.Seek(append([]byte(it.prefix), key...))
//}
//
//func (it *StandAloneStorageIterator) Rewind() {
//	it.it.Rewind()
//}
