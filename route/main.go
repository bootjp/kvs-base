package route

import (
	kvs "github.com/bootjp/kvs/old"
)

type NodeID string
type Node struct {
	ID NodeID
}

func (n Node) String() string {
	return string(n.ID)
}

type Router interface {
	LookupNode(k *kvs.Key) (*Node, error)
	AddNode(node *Node) error
	RemoveNode(node *Node) error
}
