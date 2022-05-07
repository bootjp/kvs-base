// https://github.com/lni/dragonboat-example/blob/79f3d372190ee23705517f193dfe7cc839b34a14/ondisk/diskkv.go
// Copyright 2017-2019 Lei Ni (nilei81@gmail.com)
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package kvs

import (
	"bytes"
	"context"
	"crypto/md5"
	"encoding/binary"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"math/rand"
	"os"
	"os/signal"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"time"
	"unsafe"

	"github.com/lni/dragonboat/v3"
	"github.com/lni/dragonboat/v3/config"
	"github.com/lni/dragonboat/v3/logger"
	"github.com/lni/goutils/syncutil"

	"github.com/cockroachdb/pebble"

	sm "github.com/lni/dragonboat/v3/statemachine"
)

const (
	appliedIndexKey    string = "disk_kv_applied_index"
	testDBDirName      string = "data"
	currentDBFilename  string = "current"
	updatingDBFilename string = "current.updating"
	exampleClusterID   uint64 = 123
)

var (
	nodeID        int
	enableConsole bool
	join          bool
	clusterID     uint64
)

func init() {
	// https://github.com/golang/go/issues/17393
	if runtime.GOOS == "darwin" {
		signal.Ignore(syscall.Signal(0xd))
	}

	logger.GetLogger("raft").SetLevel(logger.ERROR)
	logger.GetLogger("rsm").SetLevel(logger.WARNING)
	logger.GetLogger("transport").SetLevel(logger.WARNING)
	logger.GetLogger("grpc").SetLevel(logger.WARNING)

	f := flag.NewFlagSet("", flag.ExitOnError)
	f.IntVar(&nodeID, "nodeid", 1, "NodeID to use")
	f.BoolVar(&enableConsole, "console", false, "is non interactive")
	f.BoolVar(&join, "join", false, "Joining a new node")
	f.Uint64Var(&clusterID, "clusterid", 1, "cluster id")
	_ = f.Parse(os.Args[1:]) // ignore error use flag.ExitOnError
}

func raftConfig(nodeID uint64) config.Config {
	return config.Config{
		NodeID:             nodeID,
		ClusterID:          exampleClusterID,
		ElectionRTT:        10,
		HeartbeatRTT:       1,
		CheckQuorum:        true,
		SnapshotEntries:    10,
		CompactionOverhead: 5,
	}
}
func nodeHostConfig(datadir, nodeAddr string) config.NodeHostConfig {
	return config.NodeHostConfig{
		WALDir:         datadir,
		NodeHostDir:    datadir,
		RTTMillisecond: 200,
		RaftAddress:    nodeAddr,
	}
}

var (
	Addresses = map[uint64]string{
		1: "localhost:63001",
		2: "localhost:63002",
		3: "localhost:63003",
	}
)

func Run() error {

	nodeAddr := Addresses[uint64(nodeID)]

	fmt.Fprintf(os.Stdout, "node address: %s\n", nodeAddr)

	rc := raftConfig(uint64(nodeID))

	datadir := filepath.Join("data", fmt.Sprintf("node%d", nodeID))

	nhc := nodeHostConfig(datadir, nodeAddr)

	nh, err := dragonboat.NewNodeHost(nhc)
	if err != nil {
		return err
	}

	if err := nh.StartOnDiskCluster(Addresses, join, NewDiskKV, rc); err != nil {
		fmt.Fprintf(os.Stderr, "failed to add cluster, %v\n", err)
		return err
	}
	raftStopper := syncutil.NewStopper()

	ch := make(chan string, 16)
	if enableConsole {
		Console(raftStopper, ch, nh)
	}
	raftStopper.RunWorker(func() {
		cs := nh.GetNoOPSession(exampleClusterID)
		for {
			select {
			case v, ok := <-ch:
				if !ok {
					return
				}
				msg := strings.Replace(v, "\n", "", 1)
				// input message must be in the following formats -
				// put key value
				// get key
				rt, key, val, ok := ParseCommand(msg)
				if !ok {
					fmt.Fprintf(os.Stderr, "invalid input\n")
					continue
				}
				ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
				kv := &KVData{
					Key: key,
					Val: val,
					OP:  PUT,
				}
				switch rt {
				case DELETE:
					kv.OP = DELETE
					data, err := json.Marshal(kv)
					if err != nil {
						fmt.Fprintf(os.Stderr, "Update Marhsal returned error %v\n", err)
						continue
					}
					_, err = nh.SyncPropose(ctx, cs, data)
					if err != nil {
						fmt.Fprintf(os.Stderr, "SyncPropose returned error %v\n", err)
						continue
					}
				case PUT:
					kv.OP = PUT
					data, err := json.Marshal(kv)
					if err != nil {
						fmt.Fprintf(os.Stderr, "Update Marhsal returned error %v\n", err)
						continue
					}
					_, err = nh.SyncPropose(ctx, cs, data)
					if err != nil {
						fmt.Fprintf(os.Stderr, "SyncPropose returned error %v\n", err)
						continue
					}
				case GET:
					kv.OP = GET
					result, err := nh.SyncRead(ctx, exampleClusterID, []byte(key))
					if err != nil {
						fmt.Fprintf(os.Stderr, "SyncRead returned error %v\n", err)
						continue
					} else {
						fmt.Fprintf(os.Stdout, "query key: %s, result: %s\n", key, result)
					}
				}
				cancel()
			case <-raftStopper.ShouldStop():
				return
			}
		}
	})
	raftStopper.Wait()
	return nil
}

//
// Note: this is an example demonstrating how to use the on disk state machine
// feature in Dragonboat. it assumes the underlying db only supports Get, Put
// and TakeSnapshot operations. this is not a demonstration on how to build a
// distributed key-value database.
//
func syncDir(dir string) (err error) {
	if runtime.GOOS == "windows" {
		return nil
	}
	fileInfo, err := os.Stat(dir)
	if err != nil {
		return err
	}
	if !fileInfo.IsDir() {
		return errors.New("is not dir")
	}
	df, err := os.Open(filepath.Clean(dir))
	if err != nil {
		return err
	}
	defer func() {
		if cerr := df.Close(); err == nil {
			err = cerr
		}
	}()
	return df.Sync()
}

type KVData struct {
	Key string
	Val []byte
	OP  RequestType
}

// pebbledb is a wrapper to ensure lookup() and close() can be concurrently
// invoked. IOnDiskStateMachine.Update() and close() will never be concurrently
// invoked.
type pebbledb struct {
	mu     sync.RWMutex
	db     *pebble.DB
	ro     *pebble.IterOptions
	wo     *pebble.WriteOptions
	syncwo *pebble.WriteOptions
	closed bool
}

func (r *pebbledb) lookup(query []byte) ([]byte, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	if r.closed {
		return nil, errors.New("db already closed")
	}
	val, closer, err := r.db.Get(query)
	if err != nil {
		return nil, err
	}
	defer closer.Close()
	if len(val) == 0 {
		return []byte(""), nil
	}
	buf := make([]byte, len(val))
	copy(buf, val)
	return buf, nil
}

func (r *pebbledb) close() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.closed = true
	if r.db != nil {
		r.db.Close()
	}
}

// createDB creates a PebbleDB DB in the specified directory.
func createDB(dbdir string) (*pebbledb, error) {
	ro := &pebble.IterOptions{}
	wo := &pebble.WriteOptions{Sync: false}
	syncwo := &pebble.WriteOptions{Sync: true}
	cache := pebble.NewCache(0)
	opts := &pebble.Options{
		MaxManifestFileSize: 1024 * 32,
		MemTableSize:        1024 * 32,
		Cache:               cache,
	}
	if err := os.MkdirAll(dbdir, 0755); err != nil {
		return nil, err
	}
	db, err := pebble.Open(dbdir, opts)
	if err != nil {
		return nil, err
	}
	cache.Unref()
	return &pebbledb{
		db:     db,
		ro:     ro,
		wo:     wo,
		syncwo: syncwo,
	}, nil
}

// functions below are used to manage the current data directory of Pebble DB.
func isNewRun(dir string) bool {
	fp := filepath.Join(dir, currentDBFilename)
	if _, err := os.Stat(fp); os.IsNotExist(err) {
		return true
	}
	return false
}

func getNodeDBDirName(clusterID uint64, nodeID uint64) string {
	part := fmt.Sprintf("node%d", nodeID)
	return filepath.Join(testDBDirName, part)
}

func getNewRandomDBDirName(dir string) string {
	part := "%d_%d"
	rn := rand.Uint64()
	ct := time.Now().UnixNano()
	return filepath.Join(dir, fmt.Sprintf(part, rn, ct))
}

func replaceCurrentDBFile(dir string) error {
	fp := filepath.Join(dir, currentDBFilename)
	tmpFp := filepath.Join(dir, updatingDBFilename)
	if err := os.Rename(tmpFp, fp); err != nil {
		return err
	}
	return syncDir(dir)
}

func saveCurrentDBDirName(dir string, dbdir string) error {
	h := md5.New()
	if _, err := h.Write([]byte(dbdir)); err != nil {
		return err
	}
	fp := filepath.Join(dir, updatingDBFilename)
	f, err := os.Create(fp)
	if err != nil {
		return err
	}
	defer func() {
		err = f.Close()
		if cerr := f.Close(); err != nil {
			err = cerr
		}
		if cerr := syncDir(dir); err != nil {
			err = cerr
		}
	}()
	if _, err := f.Write(h.Sum(nil)[:8]); err != nil {
		return err
	}
	if _, err := f.Write([]byte(dbdir)); err != nil {
		return err
	}
	if err := f.Sync(); err != nil {
		return err
	}
	return nil
}

func getCurrentDBDirName(dir string) (string, error) {
	fp := filepath.Join(dir, currentDBFilename)
	f, err := os.OpenFile(fp, os.O_RDONLY, 0755)
	if err != nil {
		return "", err
	}
	defer func() {
		if cerr := f.Close(); cerr != nil {
			err = cerr
		}
	}()
	data, err := ioutil.ReadAll(f)
	if err != nil {
		return "", err
	}
	if len(data) <= 8 {
		return "", errors.New("corrupted content")
	}

	crc := data[:8]
	content := data[8:]
	h := md5.New()
	if _, err := h.Write(content); err != nil {
		return "", err
	}
	if !bytes.Equal(crc, h.Sum(nil)[:8]) {
		return "", errors.New("corrupted content with not matched crc")
	}
	return string(content), nil
}

func createNodeDataDir(dir string) error {
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	return syncDir(filepath.Dir(dir))
}

func cleanupNodeDataDir(dir string) error {
	os.RemoveAll(filepath.Join(dir, updatingDBFilename))
	dbdir, err := getCurrentDBDirName(dir)
	if err != nil {
		return err
	}
	files, err := ioutil.ReadDir(dir)
	if err != nil {
		return err
	}
	for _, fi := range files {
		if !fi.IsDir() {
			continue
		}
		fmt.Printf("dbdir %s, fi.name %s, dir %s\n", dbdir, fi.Name(), dir)
		toDelete := filepath.Join(dir, fi.Name())
		if toDelete != dbdir {
			fmt.Printf("removing %s\n", toDelete)
			if err := os.RemoveAll(toDelete); err != nil {
				return err
			}
		}
	}
	return nil
}

// DiskKV is a state machine that implements the IOnDiskStateMachine interface.
// DiskKV stores key-value pairs in the underlying PebbleDB key-value store. As
// it is used as an example, it is implemented using the most basic features
// common in most key-value stores. This is NOT a benchmark program.
type DiskKV struct {
	clusterID   uint64
	nodeID      uint64
	lastApplied uint64
	db          unsafe.Pointer
	closed      bool
	aborted     bool
}

// NewDiskKV creates a new disk kv test state machine.
func NewDiskKV(clusterID uint64, nodeID uint64) sm.IOnDiskStateMachine {
	d := &DiskKV{
		clusterID: clusterID,
		nodeID:    nodeID,
	}
	return d
}

func (d *DiskKV) queryAppliedIndex(db *pebbledb) (uint64, error) {
	val, closer, err := db.db.Get([]byte(appliedIndexKey))
	if err != nil && err != pebble.ErrNotFound {
		return 0, err
	}
	defer func() {
		if closer != nil {
			closer.Close()
		}
	}()
	if len(val) == 0 {
		return 0, nil
	}
	return binary.LittleEndian.Uint64(val), nil
}

// Open opens the state machine and return the index of the last Raft Log entry
// already updated into the state machine.
func (d *DiskKV) Open(stopc <-chan struct{}) (uint64, error) {
	dir := getNodeDBDirName(d.clusterID, d.nodeID)
	if err := createNodeDataDir(dir); err != nil {
		return 0, err
	}
	var dbdir string
	if !isNewRun(dir) {
		if err := cleanupNodeDataDir(dir); err != nil {
			return 0, err
		}
		var err error
		dbdir, err = getCurrentDBDirName(dir)
		if err != nil {
			return 0, err
		}
		if _, err := os.Stat(dbdir); err != nil {
			if os.IsNotExist(err) {
				return 0, errors.New("db dir unexpectedly deleted")
			}
		}
	} else {
		dbdir = getNewRandomDBDirName(dir)
		if err := saveCurrentDBDirName(dir, dbdir); err != nil {
			return 0, err
		}
		if err := replaceCurrentDBFile(dir); err != nil {
			return 0, err
		}
	}
	db, err := createDB(dbdir)
	if err != nil {
		return 0, err
	}
	atomic.SwapPointer(&d.db, unsafe.Pointer(db))
	appliedIndex, err := d.queryAppliedIndex(db)
	if err != nil {
		return 0, err
	}

	d.lastApplied = appliedIndex
	return appliedIndex, nil
}

// Lookup queries the state machine.
func (d *DiskKV) Lookup(key interface{}) (interface{}, error) {
	db := (*pebbledb)(atomic.LoadPointer(&d.db))
	if db != nil {
		v, err := db.lookup(key.([]byte))
		if err == nil && d.closed {
			return nil, errors.New("lookup returned valid result when DiskKV is already closed")
		}
		if err == pebble.ErrNotFound {
			return v, nil
		}
		return v, err
	}
	return nil, errors.New("db closed")
}

// Update updates the state machine. In this example, all updates are put into
// a PebbleDB write batch and then atomically written to the DB together with
// the index of the last Raft Log entry. For simplicity, we always Sync the
// writes (db.wo.Sync=True). To get higher throughput, you can implement the
// Sync() method below and choose not to synchronize for every Update(). Sync()
// will periodically called by Dragonboat to synchronize the state.
func (d *DiskKV) Update(ents []sm.Entry) ([]sm.Entry, error) {
	log.Println("call update by ", d.nodeID, ents)
	if d.aborted {
		return nil, errors.New("update() called after abort set to true")
	}
	if d.closed {
		return nil, errors.New("update called after Close()")
	}
	db := (*pebbledb)(atomic.LoadPointer(&d.db))
	wb := db.db.NewBatch()
	defer wb.Close()
	for idx, e := range ents {
		dataKV := &KVData{}
		if err := json.Unmarshal(e.Cmd, dataKV); err != nil {
			return nil, err
		}
		switch dataKV.OP {
		case PUT:
			if err := wb.Set([]byte(dataKV.Key), dataKV.Val, db.wo); err != nil {
				ents[idx].Result = sm.Result{}
				continue
			}
			ents[idx].Result = sm.Result{Value: uint64(len(ents[idx].Cmd))}
		case DELETE:
			if err := wb.Delete([]byte(dataKV.Key), db.wo); err != nil {
				ents[idx].Result = sm.Result{}
				continue
			}
			ents[idx].Result = sm.Result{Value: uint64(len(ents[idx].Cmd))}
		}
	}
	// save the applied index to the DB.
	appliedIndex := make([]byte, 8)
	binary.LittleEndian.PutUint64(appliedIndex, ents[len(ents)-1].Index)

	if err := wb.Set([]byte(appliedIndexKey), appliedIndex, db.wo); err != nil {
		return nil, err
	}
	if err := db.db.Apply(wb, db.syncwo); err != nil {
		return nil, err
	}
	if d.lastApplied >= ents[len(ents)-1].Index {
		return nil, errors.New("lastApplied not moving forward")
	}
	d.lastApplied = ents[len(ents)-1].Index
	return ents, nil
}

// Sync synchronizes all in-core state of the state machine. Since the Update
// method in this example already does that every time when it is invoked, the
// Sync method here is a NoOP.
func (d *DiskKV) Sync() error {
	return nil
}

type diskKVCtx struct {
	db       *pebbledb
	snapshot *pebble.Snapshot
}

// PrepareSnapshot prepares snapshotting. PrepareSnapshot is responsible to
// capture a state identifier that identifies a point in time state of the
// underlying data. In this example, we use Pebble's snapshot feature to
// achieve that.
func (d *DiskKV) PrepareSnapshot() (interface{}, error) {
	if d.closed {
		return nil, errors.New("prepare snapshot called after Close()")
	}
	if d.aborted {
		return nil, errors.New("prepare snapshot called after abort")
	}

	db := (*pebbledb)(atomic.LoadPointer(&d.db))
	return &diskKVCtx{
		db:       db,
		snapshot: db.db.NewSnapshot(),
	}, nil
}

func iteratorIsValid(iter *pebble.Iterator) bool {
	return iter.Valid()
}

// saveToWriter saves all existing key-value pairs to the provided writer.
// As an example, we use the most straight forward way to implement this.
func (d *DiskKV) saveToWriter(db *pebbledb,
	ss *pebble.Snapshot, w io.Writer) error {
	iter := ss.NewIter(db.ro)
	defer iter.Close()
	values := make([]*KVData, 0)
	for iter.First(); iteratorIsValid(iter); iter.Next() {
		kv := &KVData{
			Key: string(iter.Key()),
			Val: iter.Value(),
		}
		values = append(values, kv)
	}
	count := uint64(len(values))
	sz := make([]byte, 8)
	binary.LittleEndian.PutUint64(sz, count)
	if _, err := w.Write(sz); err != nil {
		return err
	}
	for _, dataKv := range values {
		data, err := json.Marshal(dataKv)
		if err != nil {
			return err
		}
		binary.LittleEndian.PutUint64(sz, uint64(len(data)))
		if _, err := w.Write(sz); err != nil {
			return err
		}
		if _, err := w.Write(data); err != nil {
			return err
		}
	}
	return nil
}

// SaveSnapshot saves the state machine state identified by the state
// identifier provided by the input ctx parameter. Note that SaveSnapshot
// is not suppose to save the latest state.
func (d *DiskKV) SaveSnapshot(ctx interface{},
	w io.Writer, done <-chan struct{}) error {
	if d.closed {
		return errors.New("prepare snapshot called after Close()")
	}
	if d.aborted {
		return errors.New("prepare snapshot called after abort")
	}
	ctxdata := ctx.(*diskKVCtx)
	db := ctxdata.db
	db.mu.RLock()
	defer db.mu.RUnlock()
	ss := ctxdata.snapshot
	defer ss.Close()
	return d.saveToWriter(db, ss, w)
}

// RecoverFromSnapshot recovers the state machine state from snapshot. The
// snapshot is recovered into a new DB first and then atomically swapped with
// the existing DB to complete the recovery.
func (d *DiskKV) RecoverFromSnapshot(r io.Reader,
	done <-chan struct{}) error {
	if d.closed {
		return errors.New("recover from snapshot called after Close()")
	}
	dir := getNodeDBDirName(d.clusterID, d.nodeID)
	dbdir := getNewRandomDBDirName(dir)
	oldDirName, err := getCurrentDBDirName(dir)
	if err != nil {
		return err
	}
	db, err := createDB(dbdir)
	if err != nil {
		return err
	}
	sz := make([]byte, 8)
	if _, err := io.ReadFull(r, sz); err != nil {
		return err
	}
	total := binary.LittleEndian.Uint64(sz)
	wb := db.db.NewBatch()
	defer wb.Close()
	for i := uint64(0); i < total; i++ {
		if _, err := io.ReadFull(r, sz); err != nil {
			return err
		}
		toRead := binary.LittleEndian.Uint64(sz)
		data := make([]byte, toRead)
		if _, err := io.ReadFull(r, data); err != nil {
			return err
		}
		dataKv := &KVData{}
		if err := json.Unmarshal(data, dataKv); err != nil {
			return err
		}
		wb.Set([]byte(dataKv.Key), dataKv.Val, db.wo)
	}
	if err := db.db.Apply(wb, db.syncwo); err != nil {
		return err
	}
	if err := saveCurrentDBDirName(dir, dbdir); err != nil {
		return err
	}
	if err := replaceCurrentDBFile(dir); err != nil {
		return err
	}
	newLastApplied, err := d.queryAppliedIndex(db)
	if err != nil {
		return err
	}
	// when d.lastApplied == newLastApplied, it probably means there were some
	// dummy entries or membership change entries as part of the new snapshot
	// that never reached the SM and thus never moved the last applied index
	// in the SM snapshot.
	if d.lastApplied > newLastApplied {
		return errors.New("last applied not moving forward")
	}
	d.lastApplied = newLastApplied
	old := (*pebbledb)(atomic.SwapPointer(&d.db, unsafe.Pointer(db)))
	if old != nil {
		old.close()
	}
	parent := filepath.Dir(oldDirName)
	if err := os.RemoveAll(oldDirName); err != nil {
		return err
	}
	return syncDir(parent)
}

// Close closes the state machine.
func (d *DiskKV) Close() error {
	db := (*pebbledb)(atomic.SwapPointer(&d.db, unsafe.Pointer(nil)))
	if db != nil {
		d.closed = true
		db.close()
	} else {
		if d.closed {
			return errors.New("close called twice")
		}
	}
	return nil
}
