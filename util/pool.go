package util

import "sync"

// BufPool provides reusable byte buffers for network I/O, reducing
// GC pressure on hot paths like bidirectional copy loops.
var BufPool = sync.Pool{
	New: func() interface{} {
		buf := make([]byte, DefaultBufSize)
		return &buf
	},
}

// GetBuf retrieves a buffer from the pool.  Callers must return it
// with [PutBuf] when finished.
func GetBuf() *[]byte {
	return BufPool.Get().(*[]byte)
}

// PutBuf returns a buffer to the pool for reuse.
func PutBuf(buf *[]byte) {
	if buf == nil {
		return
	}
	BufPool.Put(buf)
}
