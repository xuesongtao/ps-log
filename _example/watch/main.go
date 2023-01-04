package main

import (
	"fmt"

	pslog "gitee.com/xuesongtao/ps-log"
	"gitee.com/xuesongtao/xlog"
)

func main() {
	w, err := pslog.NewWatch()
	if err != nil {
		xlog.Panic(err)
	}
	defer w.Close()

	dir := "/Users/xuesongtao/goProject/src/myGo/ps-log/_example/watch"
	watchList := []string{
		dir + "/tmp",
		dir + "/tmp1/test.txt",
		dir + "/tmp2/test.txt",
	}
	if err := w.Add(watchList...); err != nil {
		panic(err)
	}
	handleCh := make(chan *pslog.WatchFileInfo, 3)
	w.Watch(handleCh)

	for c := range handleCh {
		fmt.Println(c.Path)
	}
	<-make(chan int, 1)
}
