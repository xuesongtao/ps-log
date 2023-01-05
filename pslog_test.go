package pslog

import (
	"fmt"
	"io"
	"os"
	"testing"
	"time"

	"gitee.com/xuesongtao/gotool/base"
	"gitee.com/xuesongtao/gotool/xfile"
	"gitee.com/xuesongtao/xlog"
)

var (
	tmpDir = "./tmp"
)

func TestTailSaveOffset(t *testing.T) {
	ps, _ := NewPsLog()
	ps.TailLogs()

	tmp := tmpDir + "/test1.log"
	handler := &Handler{
		Change:   -1,
		Tail:     true,
		ExpireAt: NoExpire,
		Tos: []io.Writer{
			os.Stdout,
		},
		Targets:  []string{},
		Excludes: []string{}}
	ps.Register(handler)
	ps.AddPaths(tmp)

	closeCh := make(chan struct{})
	go func() {
		fh := xfile.NewFileHandle(tmp)
		if err := fh.Initf(os.O_WRONLY | os.O_TRUNC); err != nil {
			xlog.Error(err)
			return
		}
		defer fh.Close()

		f := fh.GetFile()
		for i := 0; i < 30; i++ {
			time.Sleep(time.Microsecond)
			_, err := f.WriteString(time.Now().Format(base.DatetimeFmt+".000") + " " + fmt.Sprint(i) + "\n")
			if err != nil {
				xlog.Error("write err:", err)
			}
		}
		close(closeCh)
	}()

	for range closeCh {
	}
	ps.Close()
	time.Sleep(time.Second * 5)
}
