package pslog

import (
	"bytes"
	"sync"

	"gitee.com/xuesongtao/gotool/xfile"
)

var (
	filePool    = xfile.NewFilePool()                                             // 文件池
	syncBufPool = sync.Pool{New: func() interface{} { return new(bytes.Buffer) }} // 临时 buf
)

// newStrBuf
func newStrBuf(size ...int) *bytes.Buffer {
	obj := syncBufPool.Get().(*bytes.Buffer)
	if len(size) > 0 {
		obj.Grow(size[0])
	}
	return obj
}

// putStrBuf
func putStrBuf(buf *bytes.Buffer) {
	if buf.Len() > 0 {
		buf.Reset()
	}
	syncBufPool.Put(buf)
}
