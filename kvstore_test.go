// Copyright 2016 The etcd Authors
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

package main

import (
	"reflect"
	"testing"
)

//func Test_kvstore_store(t *testing.T) {
//	tm := map[string][]byte{}
//	s := &kvstore{kvStore: tm}
//
//	v, ok := s.Lookup("foo")
//	if !(ok == false && v == nil) {
//		t.Fatalf("foo has unexpected value, got %v %v", v, []byte{})
//	}
//
//	s.Propose("foo", []byte("bar"))
//	if !reflect.DeepEqual(v, "bar") {
//		t.Fatalf("foo has unexpected value, got %s", v)
//	}
//
//	s.Propose("foo", []byte("2"))
//	if !reflect.DeepEqual(v, "2") {
//		t.Fatalf("foo has unexpected value, got %s", v)
//	}
//}

//func Test_kvstore_delete(t *testing.T) {
//
//	// raft provides a commit stream for the proposals from the http api
//	proposeC := make(chan string)
//	defer close(proposeC)
//	confChangeC := make(chan raftpb.ConfChange)
//	defer close(confChangeC)
//
//	tm := KVStore{}
//	s := &kvstore{
//		kvStore:  tm,
//		proposeC: make(chan string),
//		mu:       &sync.RWMutex{},
//	}
//
//	v, ok := s.Lookup("foo")
//	if !(ok == false && v == nil) {
//		t.Fatalf("foo has unexpected value, got %v %v", v, []byte{})
//	}
//	//
//	s.Propose("foo", []byte("bar"))
//	if !reflect.DeepEqual(v, "bar") {
//		t.Fatalf("foo has unexpected value, got %s", v)
//	}
//
//	//s.Propose("foo", []byte("2"))
//	//if !reflect.DeepEqual(v, "2") {
//	//	t.Fatalf("foo has unexpected value, got %s", v)
//	//}
//}

func Test_kvstore_snapshot(t *testing.T) {
	tm := KVStore{"foo": []byte("bar")}
	s := &kvstore{kvStore: tm}

	v, _ := s.Lookup("foo")
	if !reflect.DeepEqual(v, []byte("bar")) {
		t.Fatalf("foo has unexpected value, got %s", v)
	}

	data, err := s.getSnapshot()
	if err != nil {
		t.Fatal(err)
	}
	s.kvStore = nil

	if err := s.recoverFromSnapshot(data); err != nil {
		t.Fatal(err)
	}
	v, _ = s.Lookup("foo")
	if !reflect.DeepEqual(v, []byte("bar")) {
		t.Fatalf("foo has unexpected value, got %s", v)
	}
	if !reflect.DeepEqual(s.kvStore, tm) {
		t.Fatalf("store expected %+v, got %+v", tm, s.kvStore)
	}
}
