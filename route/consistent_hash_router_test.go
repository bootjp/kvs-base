package route

import (
	kvs "github.com/bootjp/kvs/old"
	"testing"
)

func TestConsistent(t *testing.T) {

	m := map[NodeID]*Node{
		"1": {ID: NodeID("1")},
		"2": {ID: NodeID("2")},
		"3": {ID: NodeID("3")},
	}

	c := NewConsistentHashRouter(&RouterConfig{NodeSize: 3})
	c.AddNode(m["1"])
	c.AddNode(m["2"])
	c.AddNode(m["3"])

	k := kvs.Key("test")
	res, err := c.LookupNode(&k)
	if err != nil {
		t.Fatal(err)
	}

	if v, err := c.LookupNode(&k); err != nil || v != res {
		t.Fatal("not equal")
	}

}
