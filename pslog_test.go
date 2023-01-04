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

func TestTail(t *testing.T) {
	ps, _ := NewPsLog()
	ps.TailLogs()

	tmp := tmpDir + "/test.log"
	handler := &Handler{
		Tail:     true,
		ExpireAt: NoExpire,
		Tos:      []io.Writer{os.Stdout},
		Targets:  []string{},
		Excludes: []string{},
	}
	ps.Register(handler)
	ps.AddPaths(tmp)

	xfile.PutContent(tmp, "\n")

	go func() {
		fh := xfile.NewFileHandle(tmp)
		if err := fh.Initf(os.O_CREATE | os.O_RDWR); err != nil {
			xlog.Error(err)
			return
		}
		f := fh.GetFile()
		for i := 0; i < 100; i++ {
			time.Sleep(time.Second)
			_, err := f.WriteString(time.Now().Format(base.DatetimeFmt+".000") + " " + fmt.Sprint(i) + "\n")
			if err != nil {
				xlog.Error("write err:", err)
			}
		}
	}()
	time.Sleep(time.Second * 15)
	ps.Close()
	time.Sleep(time.Second * 2)
}

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

	// xfile.PutContent(tmp, "\n")

	go func() {
		fh := xfile.NewFileHandle(tmp)
		if err := fh.Initf(os.O_CREATE | os.O_RDWR); err != nil {
			xlog.Error(err)
			return
		}
		f := fh.GetFile()
		for i := 0; i < 100; i++ {
			time.Sleep(time.Second)
			_, err := f.WriteString(time.Now().Format(base.DatetimeFmt+".000") + " " + fmt.Sprint(i) + "\n")
			if err != nil {
				xlog.Error("write err:", err)
			}
		}
	}()
	time.Sleep(time.Second * 15)
	ps.Close()
	time.Sleep(time.Second * 2)
}
