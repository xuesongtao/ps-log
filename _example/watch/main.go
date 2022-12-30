package main

import (
	"fmt"

	pslog "gitee.com/xuesongtao/ps-log"
	"gitee.com/xuesongtao/xlog"
)

func main() {
	w, err := pslog.NewWatch(10)
	if err != nil {
		xlog.Panic(err)
	}

	handleFn := func(bus *pslog.WatchBus) {
		fmt.Println(bus.FileInfo.Path)
	}
	dir := "/Users/xuesongtao/goProject/src/myGo/ps-log/_example/watch"
	if err := w.Add(dir+"/tmp", dir+"/tmp1/test.txt", dir+"/tmp2/test.txt"); err != nil {
		panic(err)
	}
	go w.Watch(handleFn)
	<-make(chan int, 1)
}
