package kvs

import (
	"github.com/stretchr/testify/assert"
	"os"
	"testing"
)

var kvs KVS

func init() {
	k, err := NewKVS(os.TempDir())
	if err != nil {
		panic(err)
	}
	kvs = k
}

func Test(t *testing.T) {
	err := kvs.RawPut([]byte("foo"), []byte("bar"))
	if err != nil {
		t.Fatal(err)
	}
	res, err := kvs.RawGet([]byte("foo"))
	if err != nil {
		t.Fatal(err)
	}

	assert.Equal(t, []byte("bar"), res)

	kvs.RawDelete([]byte("foo"))

	res, err = kvs.RawGet([]byte("foo"))
	if err != nil {
		t.Fatal(err)
	}

	assert.Nil(t, res)
}
