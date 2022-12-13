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
	assert.NoError(t, err)

	res, err := kvs.RawGet([]byte("foo"))
	assert.NoError(t, err)

	assert.Equal(t, []byte("bar"), res)
	assert.NoError(t, kvs.RawDelete([]byte("foo")))

	res, err = kvs.RawGet([]byte("foo"))
	assert.Equal(t, err, ErrNotFound)
	assert.Nil(t, res)

	res, err = kvs.RawGet([]byte("aaaaaa"))
	assert.Equal(t, err, ErrNotFound)
}
