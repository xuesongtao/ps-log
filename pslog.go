package pslog

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"runtime/debug"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"gitee.com/xuesongtao/gotool/base"
	plg "gitee.com/xuesongtao/ps-log/log"
	tl "gitee.com/xuesongtao/taskpool"
	tw "github.com/olekukonko/tablewriter"
)

const (
	taskPoolWorkMaxLifetime int64 = 6 * 3600 // task pool 中 work 最大存活默认时间
)

// Opt
type Opt func(*PsLog)

// WithAsync2Tos 异步处理 tos
func WithAsync2Tos() Opt {
	return func(pl *PsLog) {
		pl.async2Tos = true
	}
}

// WithTaskPoolSize 设置协程池大小
func WithTaskPoolSize(size int, workMaxLifetimeSec ...int64) Opt {
	return func(pl *PsLog) {
		defaultLife := taskPoolWorkMaxLifetime
		if len(workMaxLifetimeSec) > 0 {
			defaultLife = workMaxLifetimeSec[0]
		}
		pl.taskPool = tl.NewTaskPool("parse log", size, tl.WithPoolLogger(plg.Plg), tl.WithWorkerMaxLifeCycle(defaultLife))
	}
}

// WithCleanUpTime 设置清理 logMap 的周期
func WithCleanUpTime(dur time.Duration) Opt {
	return func(pl *PsLog) {
		pl.cleanUpTime = dur
	}
}

// PsLog 解析 log
type PsLog struct {
	tail          bool          // 是否已开启实时分析
	async2Tos     bool          // 是否异步处理 tos
	firstCallList bool          // 标记是否第一调用 List
	closed        int32         // 0-开 1-关
	cleanUpTime   time.Duration // 清理 logMap 的周期
	rwMu          sync.RWMutex
	taskPool      *tl.TaskPool        // 任务池
	handler       *Handler            // 处理部分
	watch         *Watch              // 文件监听
	watchCh       chan *WatchFileInfo // 文件监听文件内容
	closeCh       chan struct{}
	logMap        map[string]*FileInfo // key: 文件路径
}

// NewPsLog 是根据提供的 log path 进行逐行解析
// 注: 结束时需要调用 Close
func NewPsLog(opts ...Opt) (*PsLog, error) {
	fmt.Print(consoleLogo)
	obj := &PsLog{
		firstCallList: true,
		logMap:        make(map[string]*FileInfo),
		handler:       new(Handler),
		closeCh:       make(chan struct{}, 1),
	}

	for _, opt := range opts {
		opt(obj)
	}

	// 默认一小时清理 logMap 里过期的路径
	if obj.cleanUpTime == 0 {
		obj.cleanUpTime = time.Hour
	}

	if obj.taskPool == nil {
		obj.taskPool = tl.NewTaskPool("parse log", runtime.NumCPU(), tl.WithPoolLogger(plg.Plg), tl.WithWorkerMaxLifeCycle(taskPoolWorkMaxLifetime))
	}

	go obj.sentry()
	plg.Info("init ps-log is success")
	return obj, nil
}

// Register 注册处理器
func (p *PsLog) Register(handler *Handler) error {
	p.handler = handler
	return p.handler.Valid()
}

// AddPaths 添加 path, path 必须为文件全路径, 如果 path 已存在则跳过, 反之新增
// 根据 p.handler 进行处理
func (p *PsLog) AddPaths(paths ...string) error {
	path2HandlerMap := make(map[string]*Handler, len(paths))
	for _, path := range paths {
		path2HandlerMap[path] = nil
	}
	return p.addLogPath(path2HandlerMap)
}

// AddPath2Handler 单个添加, 如果 path 已存在则跳过, 反之新增
// 会根据文件对应的 Handler 进行处理, 如果为 Handler 为 nil, 会按 p.handler 来处理
func (p *PsLog) AddPath2Handler(path string, handler *Handler) error {
	return p.addLogPath(map[string]*Handler{path: handler})
}

// ReplacePath2Handler 新增文件对应的处理方法, 如果 path 已存在则替换, 反之新增
// 会根据文件对应的 Handler 进行处理, 如果为 Handler 为 nil, 会按 p.handler 来处理
func (p *PsLog) ReplacePath2Handler(path string, handler *Handler) error {
	return p.addLogPath(map[string]*Handler{path: handler}, false)
}

// AddPath2HandlerMap 添加文件对应的处理方法, 如果 path 已存在则跳过, 反之新增
// 会根据文件对应的 Handler 进行处理, 如果为 Handler 为 nil, 会按 p.handler 来处理
func (p *PsLog) AddPath2HandlerMap(path2HandlerMap map[string]*Handler) error {
	return p.addLogPath(path2HandlerMap)
}

// addLogPath 添加 log path, 同时添加监听 log path
func (p *PsLog) addLogPath(path2HandlerMap map[string]*Handler, existSkip ...bool) error {
	defaultExistSkip := true // 默认新增, 存在跳过
	if len(existSkip) > 0 {
		defaultExistSkip = existSkip[0]
	}

	// 预处理
	new, err := p.prePath2Handler(defaultExistSkip, path2HandlerMap)
	if err != nil {
		return err
	}
	// plg.Info("new:", base.ToString(new))
	// 加锁处理
	p.rwMu.Lock()
	defer p.rwMu.Unlock()
	for path, handler := range new {
		fileInfo, ok := p.logMap[path]
		if ok && defaultExistSkip {
			continue
		}

		// 保存 file
		if ok {
			fileInfo.Handler = handler
		} else {
			fileInfo = &FileInfo{Handler: handler}
			fileInfo.Parse(path)
		}
		fileInfo.initOffset()
		if p.tail && handler.Tail {
			// 直接监听对应的目录
			if err := p.watch.Add(fileInfo.FileName()); err != nil {
				return fmt.Errorf("p.watch.Add is failed, err: %v", err)
			}
		}
		p.logMap[path] = fileInfo
	}
	// plg.Info("logMap:", base.ToString(p.logMap))
	return nil
}

// prePath2Handler 预处理
func (p *PsLog) prePath2Handler(existSkip bool, path2HandlerMap map[string]*Handler) (map[string]*Handler, error) {
	// 验证加处理
	new := make(map[string]*Handler, len(path2HandlerMap))
	for path, handler := range path2HandlerMap {
		p.rwMu.RLock()
		_, ok := p.logMap[path]
		p.rwMu.RUnlock()
		if ok && existSkip {
			continue
		}

		path = filepath.Clean(path)
		// 处理 handler
		if handler == nil {
			if p.handler != nil {
				handler = p.handler
			} else {
				return nil, fmt.Errorf("%q no has handler", path)
			}
		}
		if err := handler.Valid(); err != nil {
			return nil, fmt.Errorf("%q handler is not ok, err: %v", path, err)
		}
		handler.init()

		// 判断下是否为目录
		st, err := os.Stat(path)
		if err != nil {
			return nil, fmt.Errorf("os.Stat %q is failed, err: %v", path, err)
		}
		if st.IsDir() {
			return nil, fmt.Errorf("%q is dir, it should file", path)
		}
		new[path] = handler
	}
	return new, nil
}

// Close 释放资源
func (p *PsLog) Close() {
	if p.HasClose() {
		// 已经关了的就退出, 防止重复关闭 chan panic
		return
	} else {
		atomic.StoreInt32(&p.closed, 1)
	}

	if p.watch != nil {
		p.watch.Close()
	}
	if p.taskPool != nil {
		p.taskPool.SafeClose()
	}
	filePool.Close()

	// close(p.watchCh) // p.watch.Close() 执行后, p.watchCh 会被关闭
	close(p.closeCh)
}

// TailLogs 实时解析 log
// watchSize 为监听到文件变化处理数据的 chan 的长度, 建议为监听文件的个数
func (p *PsLog) TailLogs(watchChSize ...int) error {
	p.tail = true

	size := 1 << 4
	if len(watchChSize) > 0 && watchChSize[0] > 0 {
		size = watchChSize[0]
	}

	// 初始化 watch
	watch, err := NewWatch()
	if err != nil {
		return fmt.Errorf("NewWatch is failed, err:%v", err)
	}
	p.watch = watch
	p.watchCh = make(chan *WatchFileInfo, size)
	p.watch.Watch(p.watchCh)

	// 开始监听
	go func() {
		defer p.final()

		// 退出情况
		// 1. watch 退出
		// 2. Close 后
		for watchInfo := range p.watchCh {
			p.rwMu.RLock()
			fileInfo, ok := p.logMap[watchInfo.Path]
			p.rwMu.RUnlock()
			if !ok {
				plg.Infof("%q is not exist", watchInfo.Path)
				continue
			}
			if !fileInfo.Handler.Tail {
				plg.Infof("%q no need tail", watchInfo.Path)
				continue
			}
			p.parseLog(false, fileInfo)
		}
		plg.Info("watchCh is closed")
	}()
	return nil
}

// cronLog 定时解析 log
// makeUpTail 标记是否补偿 tail 模式, 防止文件开启后没有变化
func (p *PsLog) CronLogs(makeUpTail ...bool) {
	defaultMakeUpTail := true
	if len(makeUpTail) > 0 {
		defaultMakeUpTail = makeUpTail[0]
	}
	// 减少锁的持续时间, 初始化一个临时的 map
	p.rwMu.RLock()
	tmpLogMap := make(map[string]*FileInfo, len(p.logMap))
	for path, fileInfo := range p.logMap {
		tmpLogMap[path] = fileInfo
	}
	p.rwMu.RUnlock()

	for _, fileInfo := range tmpLogMap {
		// 开机后偏移量一直相等, 为防止文件一直没有变化(但文件里有待处理的内容), 需要定时处理, 说明:
		// 	1. 如果在执行过程中未处理文件内容服务停了(漏处理), 重启后如果文件一直没有变化需要定时处理
		// 	2. 如果在执行过程中已处理文件内容, 未保存偏移量服务停了(已处理), 重启后如果文件一直没有变化需要定时处理
		if fileInfo.loadOffset() > fileInfo.loadBeginOffset() && fileInfo.Handler.Tail { // 已经实时处理过了
			continue
		}
		if fileInfo.loadOffset() == fileInfo.loadBeginOffset() && !defaultMakeUpTail { // 不需要补偿
			continue
		}

		// 定时更新的不是很频繁, 所有每次都保存 offset
		p.parseLog(true, fileInfo)
	}
}

// parseLog 解析文件
func (p *PsLog) parseLog(mustSaveOffset bool, fileInfo *FileInfo) {
	if p.HasClose() {
		plg.Warning("ps-log is closed")
		return
	}
	// 防止 tail 和 cron 对同一个文件进行操作
	fileInfo.mu.Lock()
	defer fileInfo.mu.Unlock()

	fh, err := filePool.Get(fileInfo.FileName(), os.O_RDONLY)
	if err != nil {
		plg.Errorf("filePool.Get %q is failed, err: %v", fileInfo.FileName(), err)
		return
	}
	defer filePool.Put(fh)

	f := fh.GetFile()
	st, err := f.Stat()
	if err != nil {
		plg.Error("f.Stat is failed, err:", err)
		return
	}

	fileSize := st.Size()
	plg.Infof("filename: %q, offset: %d, size: %d", fileInfo.FileName(), fileInfo.offset, fileSize)
	if fileSize == 0 || fileInfo.offset == fileSize {
		plg.Infof("offset: %d, fileSize: %d it will skip", fileInfo.offset, fileSize)
		return
	}
	handler := fileInfo.Handler
	// 这里单个文件, 循环采集
	if fileInfo.offset > fileSize && handler.LoopParse {
		fileInfo.offset = 0
	}
	_, err = f.Seek(fileInfo.offset, io.SeekStart)
	if err != nil {
		plg.Error("f.Seek is failed, err:", err)
		return
	}

	// 逐行读取
	rows := bufio.NewScanner(f)
	readSize := fileInfo.offset                   // 已读数
	dataMap := make(map[int]*LogHandlerBus, 1<<3) // key: target.no, 支持一个匹配规则多个处理方式
	for rows.Scan() {
		// 因为当前读为快照读, 所以需要保证本次读取内容小于 fileSize (快照时文件的大小)
		if readSize > fileSize {
			break
		}

		rowBytes := rows.Bytes()
		readSize += int64(len(rowBytes))
		// 处理行内容, 解决日志中可能出现的换行, 如: err stack
		if !handler.MergeRule.Append(rowBytes) {
			continue
		}
		p.handleLine(fileInfo, dataMap, handler.MergeRule.Line())
	}

	// 说明还有内容没有读取完
	if !handler.MergeRule.Null() {
		// plg.Infof("fileSize: %d, readSize: %d, residue: %d, total: %d", fileSize, readSize, residue, readSize+int64(residue))
		p.handleLine(fileInfo, dataMap, handler.MergeRule.Line())
	}

	// plg.Info("dataMap:", base.ToString(dataMap))
	if len(dataMap) > 0 {
		p.writer(dataMap)
	}

	// 保存偏移量
	fileInfo.storeOffset(fileSize)
	p.taskPool.Submit(func() {
		fileInfo.saveOffset(mustSaveOffset, fileSize)
	})
}

// handleLine 处理 line 内容
func (p *PsLog) handleLine(fileInfo *FileInfo, dataMap map[int]*LogHandlerBus, line []byte) {
	// 判断下是否需要过滤掉
	handler := fileInfo.Handler
	if handler == nil {
		return
	}
	if handler.targets.Null() {
		return
	}
	target, ok := handler.targets.GetTarget(line)
	if !ok {
		return
	}
	if target.excludes.Search(line) {
		return
	}

	// plg.Info("target:", base.ToString(target))
	// 按不同内容进行处理
	if handler, ok := dataMap[target.no]; !ok {
		bus := &LogHandlerBus{LogPath: fileInfo.FileName(), Ext: fileInfo.Handler.Ext, buf: new(bytes.Buffer), tos: target.To}
		bus.Write(line)
		dataMap[target.no] = bus
	} else {
		handler.Write(line)
	}
}

// HasClose 是否已经关闭
func (p *PsLog) HasClose() bool {
	return atomic.LoadInt32(&p.closed) == 1
}

// writer 写入目标, 默认同步处理
func (p *PsLog) writer(dataMap map[int]*LogHandlerBus) {
	for _, bus := range dataMap {
		if bus.skip() {
			continue
		}
		plg.Infof("writeTo msg:", bus.Msg)
		for _, to := range bus.tos {
			if p.async2Tos { // 异步
				tmpTo, tmpBus := to, bus
				p.taskPool.Submit(func() {
					tmpTo.WriteTo(tmpBus)
				})
				continue
			}
			to.WriteTo(bus)
		}
	}
}

func (p *PsLog) final() {
	if err := recover(); err != nil {
		plg.Errorf("recover err: %v, stack: %s", err, debug.Stack())
	}
	p.Close()
}

func (p *PsLog) sentry() {
	ticker := time.NewTicker(p.cleanUpTime)
	defer func() {
		ticker.Stop()
		p.final()
	}()

	for {
		select {
		case t, ok := <-ticker.C:
			if !ok {
				return
			}
			p.cleanUp(t)
		case <-p.closeCh:
			plg.Info("ps-log sentry is close")
			return
		}
	}
}

func (p *PsLog) cleanUp(t time.Time) {
	plg.Info("cleanUp is running")

	// 处理过期的 path
	p.rwMu.RLock()
	deleteKeys := make([]string, 0, len(p.logMap))
	for path, fileInfo := range p.logMap {
		if fileInfo.IsExpire(t) {
			deleteKeys = append(deleteKeys, path)
		}
	}
	p.rwMu.RUnlock()

	defer plg.Info("cleanUp path: ", strings.Join(deleteKeys, ",\n"))
	if len(deleteKeys) == 0 {
		return
	}

	p.rwMu.Lock()
	defer p.rwMu.Unlock()
	// 1. 移除监听的 path,
	// 2. 打开的文件句柄由 filePool 会根据 lru 淘汰时进行关闭, 不需在此处理
	for _, path := range deleteKeys {
		delete(p.logMap, path)
	}

	if p.watch != nil { // 如果只是 cron 的话, 此处为 nil
		p.watch.Remove(deleteKeys...)
	}
}

// List 返回待处理的内容
// printTarget 是否打印 TARGETS, EXCLUDES(因为这两个可能会很多), 默认 true
// 格式:
// --------------------------------------------------------------------------------------
// |  PATH | OPEN |      EXPIRE         |  TAIL | BEGIN  | OFFSET  | TARGETS | EXCLUDES |
// --------------------------------------------------------------------------------------
// |  xxxx |   2   | XXXX-XX-XX XX:XX:XX |  true | 0      | 100     |【 ERRO 】|         |
// ---------------------------------------------------------------------------------------
func (p *PsLog) List(printTarget ...bool) string {
	defaultPrintTarget := true
	if len(printTarget) > 0 {
		defaultPrintTarget = printTarget[0]
	}
	header := []string{"PATH", "OPEN", "EXPIRE", "TAIL", "BEGIN", "OFFSET"}
	if defaultPrintTarget {
		header = append(header, "TARGETS", "EXCLUDES")
	}
	buffer := new(bytes.Buffer)
	buffer.WriteByte('\n')

	table := tw.NewWriter(buffer)
	table.SetHeader(header)
	table.SetRowLine(true)
	table.SetCenterSeparator("|")
	// table.SetAutoMergeCells(true)
	p.rwMu.RLock()
	defer p.rwMu.RUnlock()
	for k, v := range p.logMap {
		tailStr := base.ToString(v.Handler.Tail)
		// 第一次调用不需要打印此内容
		if !p.firstCallList && v.Handler.Tail && v.loadBeginOffset() == v.loadOffset() {
			tailStr += "(may cron)" // 可能出现在定时监听里
		}
		data := []string{
			k,
			fmt.Sprint(filePool.GetFile2Open(v.FileName(), os.O_RDONLY)), // 记录当前文件打开的文件句柄
			v.Handler.ExpireAt.Format(base.DatetimeFmt),
			tailStr,
			base.ToString(v.loadBeginOffset()),
			base.ToString(v.loadOffset()),
		}
		if defaultPrintTarget {
			data = append(data, v.Handler.getTargetDump(), v.Handler.getExcludesDump())
		}
		table.Append(data)
	}
	table.Render()

	if p.firstCallList {
		p.firstCallList = false
	}
	return buffer.String()
}
