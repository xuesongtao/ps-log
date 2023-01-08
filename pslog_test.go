package pslog

import (
	"fmt"
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
		Targets: []*Target{
			{
				Content:  "",
				Excludes: []string{},
				To:       os.Stdout,
			},
		},
	}
	if err := ps.Register(handler); err != nil {
		t.Fatal(err)
	}
	if err := ps.AddPaths(tmp); err != nil {
		t.Fatal(err)
	}

	closeCh := make(chan struct{})
	go func() {
		fh := xfile.NewFileHandle(tmp)
		if err := fh.Initf(os.O_WRONLY | os.O_TRUNC); err != nil {
			xlog.Error(err)
			return
		}
		defer fh.Close()

		f := fh.GetFile()
		for i := 0; i < 10; i++ {
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
	time.Sleep(time.Second * 10)
	ps.Close()
}
