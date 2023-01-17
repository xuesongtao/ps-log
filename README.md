# ps-log 日志分析

#### 项目背景

1. 测试/开发环境, 出现了 error log等不能被开发感知, 当反馈到开发时时间间隔较长, 如何解决?
    > **解决**: 定时(如:10m)去解析 log 中每行包含 error 的内容, 再进行对应的处理(如: 发邮件, 发es等)
2. 定时去分析 log 吗, 需要实时感知 error log 怎么办呢?
    > **解决**: 通过文件事件通知来感知文件变化呀, 有变化的时候就去查看文件了内容
3. 实时感知的文件需要起多个监听任务吗? 需要多次打卡相同的文件怎么处理呢?
    > **解决**: 不需要,只需要1个监听者,1个处理者; 可以通过池化文件句柄

#### 介绍

```go
go get -u gitee.com/xuesongtao/ps-log
```

1. 支持**定时/实时**去解析多个 log 文件, 采集完后会根据配置进行采集位置的持久化保存(即: 文件偏移量保存), 便于停机后重启防止出现重复采集现象
2. 支持 log `行内容` 多个匹配规则, 匹配的内容支持不同的处理方式(支持同步/异步处理)
3. 采用文件池将频繁使用的句柄进行缓存, 采用 `tire` 树缓存匹配规则提高匹配效率

![简易流程图](https://gitee.com/xuesongtao/ps-log/raw/master/ps-log.png)

#### 使用

##### 实时监听

```go
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
  Change:   -1,
  Tail:     true,
  ExpireAt: pslog.NoExpire,
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
  for i := 0; i < 30; i++ {
   time.Sleep(time.Second)
   _, err := fh.AppendContent(time.Now().Format(base.DatetimeFmt+".000") + " " + fmt.Sprint(i) + "\n")
   // _, err := f.WriteString(fmt.Sprint(i) + "\n")
   if err != nil {
    log.Println("write err:", err)
   }
  }
  close(closeCh)
 }()

 if err := ps.AddPaths(tmp); err != nil {
  panic(err)
 }
 for range closeCh {}
}
```

#### 其他
- 采集服务示例: [gitee](https://gitee.com/xuesongtao/collect-log.git)

- 欢迎大佬们指正, 希望大佬给❤️，to [gitee](https://gitee.com/xuesongtao/ps-log.git), [github](https://github.com/xuesongtao/ps-log.git)
