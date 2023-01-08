package main

import (
	"fmt"
	"os"
	"time"

	"gitee.com/xuesongtao/gotool/xfile"
	pslog "gitee.com/xuesongtao/ps-log"
	"gitee.com/xuesongtao/xlog"
)

func main() {
	ps, err := pslog.NewPsLog(pslog.WithAsync2Tos())
	if err != nil {
		panic(err)
	}
	defer ps.Close()

	if err := ps.TailLogs(); err != nil {
		panic(err)
	}

	handler := &pslog.Handler{
		Change:   -1,
		Tail:     true,
		ExpireAt: pslog.NoExpire,
		Targets: []*pslog.Target{
			{
				Content:  "",
				Excludes: []string{},
				To:       os.Stdout,
			},
		},
	}
	if err := ps.Register(handler); err != nil {
		panic(err)
	}
	go func() {
		fh := xfile.NewFileHandle("./log/test.log")
		if err := fh.Initf(os.O_RDWR | os.O_TRUNC); err != nil {
			xlog.Error(err)
			return
		}
		f := fh.GetFile()
		for i := 0; i < 30; i++ {
			time.Sleep(time.Second)
			// _, err := f.WriteString(time.Now().Format(base.DatetimeFmt+".000") + " " + fmt.Sprint(i) + "\n")
			_, err := f.WriteString(fmt.Sprint(i) + "\n")
			if err != nil {
				xlog.Error("write err:", err)
			}
		}
	}()

	if err := ps.AddPaths("./log/test.log"); err != nil {
		panic(err)
	}
	<-make(chan int, 1)
}
