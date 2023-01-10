package main

import (
	"fmt"
	"io"
	"os"
	"time"

	"gitee.com/xuesongtao/gotool/base"
	"gitee.com/xuesongtao/gotool/xfile"
	pslog "gitee.com/xuesongtao/ps-log"
	"gitee.com/xuesongtao/xlog"
)

func main() {
	ps, err := pslog.NewPsLog(pslog.WithAsync2Tos(), pslog.WithPreCleanOffset())
	if err != nil {
		panic(err)
	}
	defer ps.Close()

	if err := ps.TailLogs(); err != nil {
		panic(err)
	}

	tmp := "log/test.log"
	handler := &pslog.Handler{
		Change:   -1,
		Tail:     true,
		ExpireAt: pslog.NoExpire,
		Targets: []*pslog.Target{
			{
				Content:  " ",
				Excludes: []string{},
				To:       []io.Writer{os.Stdout},
			},
		},
	}
	if err := ps.Register(handler); err != nil {
		panic(err)
	}
	closeCh := make(chan int)
	go func() {
		fh := xfile.NewFileHandle(tmp)
		if err := fh.Initf(os.O_RDWR | os.O_APPEND | os.O_TRUNC); err != nil {
			xlog.Error(err)
			return
		}
		for i := 0; i < 30; i++ {
			time.Sleep(time.Second)
			_, err := fh.AppendContent(time.Now().Format(base.DatetimeFmt+".000") + " " + fmt.Sprint(i) + "\n")
			// _, err := f.WriteString(fmt.Sprint(i) + "\n")
			if err != nil {
				xlog.Error("write err:", err)
			}
		}
		close(closeCh)
	}()

	if err := ps.AddPaths(tmp); err != nil {
		panic(err)
	}

	for range closeCh {}
}
