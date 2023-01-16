package pslog

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"runtime/debug"
	"sync"
	"time"

	"gitee.com/xuesongtao/gotool/base"
	tl "gitee.com/xuesongtao/taskpool"
	tw "github.com/olekukonko/tablewriter"
)

var (
	noHandlerErr = errors.New("handler is null, you can call Register first")
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
func WithTaskPoolSize(size int) Opt {
	return func(pl *PsLog) {
		pl.taskPool = tl.NewTaskPool("parse log", size, tl.WithPoolLogger(plg))
	}
}

// WithPreCleanOffset 是否预先清理文件偏移量
func WithPreCleanOffset() Opt {
	return func(pl *PsLog) {
		pl.preCleanOffset = true
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
	tail           bool // 是否需要实时分析
	async2Tos      bool // 是否异步处理 tos
	closed         bool
	preCleanOffset bool          // 是否需要先清理已经保存的 offset
	cleanUpTime    time.Duration // 清理 logMap 的周期
	rwMu           sync.RWMutex
	taskPool       *tl.TaskPool        // 任务池
	handler        *Handler            // 处理部分
	watch          *Watch              // 文件监听
	watchCh        chan *WatchFileInfo // 文件监听文件内容
	closeCh        chan struct{}
	logMap         map[string]*FileInfo // key: 文件路径
}

// NewPsLog 是根据提供的 log path 进行逐行解析
// 注: 结束时需要调用 Close
func NewPsLog(opts ...Opt) (*PsLog, error) {
	obj := &PsLog{
		logMap:  make(map[string]*FileInfo),
		handler: new(Handler),
		closeCh: make(chan struct{}, 1),
	}

	for _, opt := range opts {
		opt(obj)
	}

	// 默认一小时清理 logMap 里过期的路径
	if obj.cleanUpTime == 0 {
		obj.cleanUpTime = time.Hour
	}

	if obj.taskPool == nil {
		obj.taskPool = tl.NewTaskPool("parse log", runtime.NumCPU(), tl.WithPoolLogger(plg))
	}

	go obj.sentry()
	return obj, nil
}

// Register 注册处理器
func (p *PsLog) Register(handler *Handler) error {
	p.handler = handler
	return nil
}

// AddPaths 添加 path, path 必须为文件全路径
// 根据 p.handler 进行处理
func (p *PsLog) AddPaths(paths ...string) error {
	if p.handler == nil {
		return noHandlerErr
	}
	path2HandlerMap := make(map[string]*Handler, len(paths))
	for _, path := range paths {
		path2HandlerMap[path] = p.handler
	}
	return p.addLogPath(path2HandlerMap)
}

// AddPath2Handler 单个添加
// 会根据文件对应的 Handler 进行处理, 如果为 Handler 为 nil, 会按 p.handler 来处理
func (p *PsLog) AddPath2Handler(path string, handler *Handler) error {
	return p.addLogPath(map[string]*Handler{path: handler})
}

// AddPath2HandlerMap 添加文件对应的处理方法
// 会根据文件对应的 Handler 进行处理, 如果为 Handler 为 nil, 会按 p.handler 来处理
func (p *PsLog) AddPath2HandlerMap(path2HandlerMap map[string]*Handler) error {
	return p.addLogPath(path2HandlerMap)
}

// prePath2Handler 预处理
func (p *PsLog) prePath2Handler(path2HandlerMap map[string]*Handler) (map[string]*Handler, error) {
	tmp := p.cloneLogMap()

	// 验证加处理
	new := make(map[string]*Handler, len(path2HandlerMap))
	for path, handler := range path2HandlerMap {
		path = filepath.Clean(path)
		if _, ok := tmp[path]; ok {
			continue
		}

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

// addLogPath 添加 log path, 同时添加监听 log path
func (p *PsLog) addLogPath(path2HandlerMap map[string]*Handler) error {
	new, err := p.prePath2Handler(path2HandlerMap)
	if err != nil {
		return err
	}

	// 加锁处理
	p.rwMu.Lock()
	defer p.rwMu.Unlock()
	for path, handler := range new {
		// 保存 file
		fileInfo := &FileInfo{Handler: handler}
		fileInfo.Parse(path)
		fileInfo.initOffset()
		p.logMap[path] = fileInfo
		if p.tail && handler.Tail {
			if err := p.watch.Add(path); err != nil {
				return fmt.Errorf("p.watch.Add is failed, err: %v", err)
			}
		}
	}
	return nil
}

// Close 释放资源
func (p *PsLog) Close() {
	p.rwMu.Lock()
	defer p.rwMu.Unlock()

	if p.closed { // 已经关了的就退出, 防止重复关闭 chan panic
		return
	}

	if p.watch != nil {
		p.watch.Close()
	}

	filePool.Close()

	if p.taskPool != nil {
		p.taskPool.SafeClose()
	}

	// close(p.watchCh) // p.watch.Close() 执行后, p.watchCh 会被关闭
	close(p.closeCh)
	p.closed = true
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
			p.parseLog(fileInfo) // 防止在解析的时候, fileInfo 变化
		}
		plg.Info("watchCh is closed")
	}()
	return nil
}

// cronLog 定时解析 log
func (p *PsLog) CronLogs() {
	p.rwMu.RLock()
	tmpLogMap := p.logMap
	p.rwMu.RUnlock()

	for _, fileInfo := range tmpLogMap {
		if fileInfo.Handler.Tail { // 跳过实时监听的
			continue
		}
		p.parseLog(fileInfo)
	}
}

// parseLog 解析文件
func (p *PsLog) parseLog(fileInfo *FileInfo) {
	// 先处理下是否需要清理 offset
	p.cleanOffset(fileInfo)
	fh, err := filePool.Get(fileInfo.FileName(), os.O_RDONLY)
	if err != nil {
		plg.Errorf("filePool.Get %q is failed, err: %v", fileInfo.FileName(), err)
		return
	}
	defer filePool.Put(fh)

	f := fh.GetFile()
	st, err := f.Stat()
	if err != nil {
		plg.Error("f.Stat %q is failed, err: %v", fileInfo.FileName(), err)
		return
	}

	fileSize := st.Size()
	plg.Infof("filename: %q, offset: %d, size: %d", fileInfo.FileName(), fileInfo.offset, fileSize)
	if fileSize == 0 || fileInfo.offset > fileSize {
		return
	}

	_, err = f.Seek(fileInfo.offset, io.SeekStart)
	if err != nil {
		plg.Error("f.Seek is failed, err:", err)
		return
	}

	// 逐行读取
	rows := bufio.NewScanner(f)
	readSize := fileInfo.offset
	dataMap := make(map[int]*LogHandlerBus, 1<<6) // key: target.no
	for rows.Scan() {
		// 保证本次读取内容小于 fileSize
		if readSize > fileSize {
			break
		}
		data := rows.Bytes()
		readSize += int64(len(data))
		targe, ok := p.parse(fileInfo.Handler, data)
		if !ok {
			continue
		}
		plg.Info("target:", base.ToString(targe))
		if _, ok := dataMap[targe.No]; !ok {
			dataMap[targe.No] = &LogHandlerBus{LogPath: fileInfo.FileName(), Ext: fileInfo.Handler.Ext, buf: new(bytes.Buffer), tos: targe.To}
		} else {
			plg.Info("data:", string(data))
			dataMap[targe.No].buf.WriteString(string(data) + "\n")
		}
	}
	p.writer(dataMap)

	// 保存偏移量
	fileInfo.offset = fileSize
	p.taskPool.Submit(func() {
		fileInfo.saveOffset(fileSize)
	})
}

// cleanOffset 清理已保存的 offset
func (p *PsLog) cleanOffset(fileInfo *FileInfo) {
	if p.preCleanOffset {
		fileInfo.offset = 0
		fileInfo.putContent(fileInfo.offsetFilename(), "0")
		p.preCleanOffset = false
	}
}

// parse 需要处理
func (p *PsLog) parse(h *Handler, row []byte) (*Target, bool) {
	if h.targets.Null() {
		return nil, false
	}
	target, ok := h.targets.GetTarget(row)
	if !ok {
		return nil, false
	}
	return target, !target.excludes.Search(row)
}

// writer 写入目标, 默认同步处理
func (p *PsLog) writer(dataMap map[int]*LogHandlerBus) {
	if len(dataMap) == 0 {
		return
	}

	plg.Infof("dataMap: %+v", base.ToString(dataMap))
	for _, data := range dataMap {
		if data.skip() {
			continue
		}
		for _, to := range data.tos {
			if p.async2Tos { // 异步
				p.taskPool.Submit(func() {
					to.WriteTo(data)
				})
				continue
			}
			to.WriteTo(data)
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
	tmpLogMap := p.cloneLogMap(true)
	deleteKeys := make([]string, 0, len(tmpLogMap))
	for path, fileInfo := range tmpLogMap {
		if fileInfo.IsExpire(t) {
			deleteKeys = append(deleteKeys, path)
		}
	}
	for _, path := range deleteKeys {
		delete(tmpLogMap, path)
	}

	p.rwMu.Lock()
	p.logMap = tmpLogMap
	p.rwMu.Unlock()
}

func (p *PsLog) cloneLogMap(depth ...bool) map[string]*FileInfo {
	defaultDepth := false
	if len(depth) > 0 {
		defaultDepth = depth[0]
	}
	p.rwMu.RLock()
	defer p.rwMu.RUnlock()
	if defaultDepth {
		tmpLogMap := make(map[string]*FileInfo, len(p.logMap))
		for k, v := range p.logMap {
			tmpLogMap[k] = v
		}
		return tmpLogMap
	}
	tmp := p.logMap
	return tmp
}

// List 返回待处理的内容
// 格式:
// ---------------------------
// |  PATH |  TAIL | OFFSET  |
// ---------------------------
// |  xxxx |  true | 100     |
// ---------------------------
func (p *PsLog) List() string {
	header := []string{"PATH", "TAIL", "OFFSET", "TARGETS"}
	buffer := new(bytes.Buffer)
	buffer.WriteByte('\n')

	table := tw.NewWriter(buffer)
	table.SetHeader(header)
	table.SetRowLine(true)
	table.SetCenterSeparator("|")
	for k, v := range p.cloneLogMap() {
		data := []string{
			k,
			base.ToString(v.Handler.Tail),
			base.ToString(v.offset),
			v.Handler.getTargetDump(),
		}
		table.Append(data)
	}
	table.Render()
	return buffer.String()
}
