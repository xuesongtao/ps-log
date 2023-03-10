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
	ps, _ := pslog.NewPsLog(pslog.WithCleanUpTime(time.Second))
	defer ps.Close()

	tmp := "log/test.log"

	handler := &pslog.Handler{
		CleanOffset: true,
		Change:      -1, // 每次都持久化 offset
		// Tail:     true,     // 实时监听
		// ExpireAt: pslog.NoExpire, // 文件句柄不过期
		ExpireAt: time.Now().Add(5 * time.Second), // 文件句柄不过期
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

	closeCh := make(chan struct{})
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

		// 防止很快就结束
		time.Sleep(time.Minute)
		close(closeCh)
	}()

	if err := ps.AddPaths(tmp); err != nil {
		panic(err)
	}

	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			ps.CronLogs()
		case <-closeCh:
			goto stopFor
		}
	}

stopFor:
	log.Println(ps.List())
	log.Println("end...")
}
