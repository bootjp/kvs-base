package route

import (
	kvs "github.com/bootjp/kvs/old"
	"github.com/buraksezer/consistent"
	"github.com/cespare/xxhash/v2"
	"sync"
)

type ConsistentHashRouter struct {
	NodeMap    map[NodeID]*Node
	mu         sync.RWMutex
	consistent *consistent.Consistent
}

type hasher struct{}

func (h hasher) Sum64(data []byte) uint64 {
	return xxhash.Sum64(data)
}

func NewConsistentHashRouter() *ConsistentHashRouter {
	cfg := consistent.Config{
		Hasher:            hasher{},
		PartitionCount:    3,
		ReplicationFactor: 3,
		Load:              1.25,
	}

	return &ConsistentHashRouter{
		NodeMap:    make(map[NodeID]*Node),
		mu:         sync.RWMutex{},
		consistent: consistent.New(nil, cfg),
	}
}

func (r *ConsistentHashRouter) AddNode(node *Node) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.NodeMap[node.ID] = node
	r.consistent.Add(node)
	return nil
}

func (r *ConsistentHashRouter) RemoveNode(node *Node) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.NodeMap[node.ID] = nil
	r.consistent.Remove(node.String())
	return nil
}

func (r *ConsistentHashRouter) LookupNode(k *kvs.Key) (*Node, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	m := r.consistent.LocateKey(*k)
	n := r.NodeMap[NodeID(m.String())]
	return n, nil
}
