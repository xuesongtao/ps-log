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

	// 实时监听
	if err := ps.TailLogs(); err != nil {
		panic(err)
	}

	tmp := "log/test.log"
	handler := &pslog.Handler{
		CleanOffset: true,           // 重新加载时, 清理已保存的 文件偏移量
		Change:      -1,             // 每次都保存文件偏移量
		Tail:        true,           // 实时监听
		ExpireAt:    pslog.NoExpire, // 不过期
		Targets: []*pslog.Target{
			{
				Content:  " ",        // 目标内容
				Excludes: []string{}, // 排查内容
				To:       []pslog.PsLogWriter{&pslog.Stdout{}},
			},
		},
	}

	// 注册
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

	// 添加待监听的 path
	if err := ps.AddPaths(tmp); err != nil {
		panic(err)
	}

	// dump
	log.Println(ps.List())
	for range closeCh {
	}
}
