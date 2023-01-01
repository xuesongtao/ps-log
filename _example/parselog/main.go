package main

import (
	"fmt"
	"os"

	"gitee.com/xuesongtao/gotool/xfile"
	pslog "gitee.com/xuesongtao/ps-log"
	"gitee.com/xuesongtao/xlog"
)

func main() {
	ps, err := pslog.NewPsLog(pslog.WithTail(true))
	if err != nil {
		panic(err)
	}
	go func() {
		fh := xfile.NewFileHandle("./log/test.log")
		if err := fh.Initf(os.O_CREATE | os.O_APPEND | os.O_RDWR); err != nil {
			xlog.Error(err)
			return
		}
		for i := 0; i < 100; i++ {
			// time.Sleep(time.Second)
			_, err := fh.GetFile().WriteString(fmt.Sprint(i) + "\n")
			if err != nil {
				xlog.Error("write err:", err)
			}
		}
	}()
	ps.AddLogPaths("/Users/xuesongtao/goProject/src/myGo/ps-log/_example/parselog/log/test.log")
	ps.TailLog()
	<-make(chan int, 1)
}
