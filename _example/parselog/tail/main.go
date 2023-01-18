package main

import (
	"fmt"
	"log"
	"os"
	"time"

	"gitee.com/xuesongtao/gotool/base"
	"gitee.com/xuesongtao/gotool/xfile"
	pslog "gitee.com/xuesongtao/ps-log"
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

	tmp := "log/test.log"
	handler := &pslog.Handler{
		CleanOffset: true,
		Change:      -1,
		Tail:        true,
		ExpireAt:    pslog.NoExpire,
		Targets: []*pslog.Target{
			{
				Content:  " ",
				Excludes: []string{},
				To:       []pslog.PsLogWriter{&pslog.Stdout{}},
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
			log.Println(err)
			return
		}
		defer fh.Close()
		for i := 0; i < 10; i++ {
			time.Sleep(10 * time.Millisecond)
			_, err := fh.AppendContent(time.Now().Format(base.DatetimeFmt+".000") + " " + fmt.Sprint(i) + "\n")
			if err != nil {
				log.Println("write err:", err)
			}
		}
		close(closeCh)
	}()

	if err := ps.AddPaths(tmp); err != nil {
		panic(err)
	}

	log.Println(ps.List())
	for range closeCh {
	}
}
